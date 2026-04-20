/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

/*
    SQLVER=9999,HCMVER=v9.9.9

    Notes:
    1. 新增 ziyan_cvm_apply_order 表
    2. 新增 ziyan_cvm_apply_suborder 表
    3. 新增 ziyan_cvm_apply_step 表
    4. 新增 ziyan_cvm_generate_record 表
    5. 新增 ziyan_cvm_apply_init_task 表
    6. 新增 ziyan_cvm_device_info 表
    7. 新增 ziyan_cvm_deliver_record 表
    8. 新增 ziyan_cvm_modify_record 表
*/

START TRANSACTION;

CREATE TABLE `ziyan_cvm_apply_order` (
    `order_id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '申请单ID',
    `product_type` VARCHAR(64) NOT NULL COMMENT '生产类型(business:业务生产 admin:管理员生产, 默认：business)',
  	`itsm_ticket_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'ITSM工单ID',
    `stage` VARCHAR(32) NOT NULL COMMENT '阶段(AUDIT:审核中 RUNNING:运行中 DONE:已完成 SUSPEND:已暂停)',
    `bk_biz_id` BIGINT NOT NULL COMMENT '业务ID',
    `bk_username` VARCHAR(64) NOT NULL COMMENT '申请人',
    `follower` JSON COMMENT '关注人列表(JSON数组)',
    `enable_notice` TINYINT(1) DEFAULT 0 COMMENT '是否启用通知(0:否 1:是)',
    `require_type` TINYINT NOT NULL COMMENT '需求类型(1:常规 2:春保等)',
    `expect_time` DATETIME DEFAULT NULL COMMENT '期望交付时间',
    `remark` varchar(255) COMMENT '备注',

    -- 子单信息存储为 JSON
    `suborders` JSON NOT NULL COMMENT '子单列表(JSON数组)',
    -- 旧的子单信息存储为 JSON
    `old_suborders` JSON NOT NULL COMMENT '旧的子单列表(JSON数组)',

    `creator` VARCHAR(64) NOT NULL COMMENT '创建人',
    `reviser` VARCHAR(64) DEFAULT '' COMMENT '修改人',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`order_id`),
    KEY `idx_product_type_bk_biz_id_stage` (`product_type`, `bk_biz_id`, `stage`)
) ENGINE=InnoDB AUTO_INCREMENT=100000 DEFAULT CHARSET=utf8mb4 COMMENT='CVM申请单主单表';

CREATE TABLE `ziyan_cvm_apply_suborder` (
    `suborder_id` VARCHAR(64) NOT NULL COMMENT '主键ID',

    -- 订单基本信息
    `order_id` BIGINT NOT NULL COMMENT '主单ID',
    `bk_biz_id` BIGINT NOT NULL COMMENT '业务ID',
    `bk_username` VARCHAR(64) NOT NULL COMMENT '申请人',
    `follower` JSON COMMENT '关注人列表(JSON数组)',
    `auditor` VARCHAR(64) DEFAULT '' COMMENT '审批人',
  	`source` VARCHAR(64) NOT NULL COMMENT '来源(business:业务 purchase_to_resource_pool:资源池)',
	`product_type` VARCHAR(64) NOT NULL COMMENT '生产类型(business:业务生产 admin:管理员生产, 默认：business)',

    -- 需求信息
    `require_type` TINYINT NOT NULL COMMENT '需求类型(常规项目、滚服项目、小额绿通等)',
    `expect_time` DATETIME DEFAULT NULL COMMENT '期望交付时间',
    `resource_type` VARCHAR(32) NOT NULL COMMENT '资源类型(QCLOUDCVM/IDCDVM/PM等)',
    `anti_affinity_level` VARCHAR(64) NOT NULL DEFAULT 'ANTI_NONE' COMMENT '反亲和级别',
  	`enable_disk_check` BOOL COMMENT '是否检查磁盘',
  	`obs_project` VARCHAR(64) DEFAULT '' COMMENT 'OBS项目名称(常规项目、滚服项目等)',
	`description` VARCHAR(255) COMMENT '主单备注',
  	`remark` VARCHAR(255) COMMENT '子单备注',
    
    -- Spec 规格信息
  	`region` VARCHAR(64) NOT NULL COMMENT '地域',
    `zone` VARCHAR(64) DEFAULT '' COMMENT '可用区',
    `device_group` VARCHAR(128) DEFAULT '' COMMENT '机型族',
  	`device_size` VARCHAR(64) DEFAULT '' COMMENT '机型核心类型(小核心、中核心、大核心)',
    `device_type` VARCHAR(64) NOT NULL COMMENT '机型',
    `image_id` VARCHAR(64) DEFAULT '' COMMENT '镜像ID',
    `image` VARCHAR(128) DEFAULT '' COMMENT '镜像名称',
    `disk_size` INT DEFAULT 0 COMMENT '磁盘大小(GB)',
    `disk_type` VARCHAR(64) DEFAULT '' COMMENT '磁盘类型(CLOUD_PREMIUM/CLOUD_SSD等)',
    `network_type` VARCHAR(64) DEFAULT '' COMMENT '网络类型',
    `vpc` VARCHAR(64) DEFAULT '' COMMENT 'VPC ID',
    `subnet` VARCHAR(64) DEFAULT '' COMMENT '子网ID',
    `os_type` VARCHAR(64) DEFAULT '' COMMENT '操作系统类型',
    `raid_type` VARCHAR(64) DEFAULT '' COMMENT 'RAID类型',
    `isp` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '运营商',
  	`failed_zone_ids` JSON DEFAULT NULL COMMENT '记录报错的可用区, JSON数组',
  	`charge_type` VARCHAR(64) DEFAULT '' COMMENT '计费模式(计费模式：PREPAID包年包月，POSTPAID_BY_HOUR按量计费)',
  	`charge_months` INT DEFAULT 0 COMMENT '计费时长，单位：月',
  	`inherit_instance_id` VARCHAR(64) DEFAULT '' COMMENT '被继承云主机实例ID',
    `bk_asset_id` VARCHAR(64) DEFAULT '' COMMENT '被继承固资编号',
  	`res_assign` TINYINT DEFAULT 0 COMMENT '资源分配方式（1表示“有资源区域优先”、2表示“分Campus生产”）',
    `cpu_thread_switch` INT DEFAULT 0 COMMENT 'CPU超线程(0:默认 1:关闭 2:开启)',
  	`system_disk` JSON NOT NULL COMMENT '系统盘, JSON对象',
  	`data_disk` JSON DEFAULT NULL COMMENT '数据盘, JSON数组',
	`zones` JSON NOT NULL COMMENT '多可用区，JSON数组',

  	-- UpgradeCVMSpec cvm升降配规格信息
  	`upgrade_cvm_list` JSON DEFAULT NULL COMMENT 'cvm升降配列表，JSON数组',
  
    -- 订单状态信息
    `stage` VARCHAR(32) NOT NULL COMMENT '阶段(AUDIT:审核中 RUNNING:运行中 DONE:已完成 SUSPEND:已暂停)',
    `status` VARCHAR(32) NOT NULL COMMENT '状态(PENDING:待处理 MATCHING:匹配中 DONE:已完成 FAILED:失败 TERMINATE:终止等)',
    `retry_time` INT DEFAULT 0 COMMENT '重试次数',
	`modify_time` INT DEFAULT 0 COMMENT '修改次数',
  	`applied_core` INT DEFAULT 0 COMMENT '需求核心数',
  	`delivered_core` INT DEFAULT 0 COMMENT '交付核心数',
  	`plan_expend_group` JSON DEFAULT NULL COMMENT '预测使用记录, JSON数组',
    
    -- 数量统计信息
    `origin_num` INT DEFAULT 0 COMMENT '原始总数量',
  	`total_num` INT DEFAULT 0 COMMENT '需求总数量',
    `success_num` INT DEFAULT 0 COMMENT '成功数量',
    `pending_num` INT DEFAULT 0 COMMENT '待处理数量',
  	`failed_num` INT DEFAULT 0 COMMENT '失败数量',
    
    -- 审计字段
    `creator` VARCHAR(64) NOT NULL COMMENT '创建人',
    `reviser` VARCHAR(64) DEFAULT '' COMMENT '修改人',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`suborder_id`),
    KEY `idx_bk_biz_id_suborder_id` (`bk_biz_id`,`suborder_id`),
    KEY `idx_stage_status_created_at_bk_biz_id` (`stage`, `status`, `created_at`, `bk_biz_id`),
  	KEY `idx_product_type_region_zone` (`product_type`,`region`, `zone`),
    KEY `idx_created_at_bk_biz_id` (`created_at`, `bk_biz_id`) COMMENT '统计查询优化索引',
    KEY `idx_created_at_source_stage_status_bk_biz_id` (`created_at`, `source`, `stage`, `status`, `bk_biz_id`) COMMENT '结单率统计覆盖索引',
    KEY `idx_created_at_source_stage_success_total_bk_biz_id` (`created_at`, `source`, `stage`, `success_num`, `total_num`, `bk_biz_id`) COMMENT '交付率统计覆盖索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='CVM申请子单表';

CREATE TABLE `ziyan_cvm_apply_step` (
    `id` VARCHAR(64) NOT NULL COMMENT '主键ID',
    
    -- 关联信息
    `suborder_id` VARCHAR(64) NOT NULL COMMENT '子单ID(格式: order_id-序号)',
    `step_id` INT NOT NULL COMMENT '步骤ID(1:生成 2:匹配 3:初始化等)',
    `step_name` VARCHAR(64) NOT NULL COMMENT '步骤名称(生成/匹配/初始化/交付等)',
    
    -- 步骤状态
    `status` TINYINT DEFAULT 0 COMMENT '状态(0:成功 1:失败 2:运行中 3:部分成功等)',
    `message` TEXT COMMENT '状态消息/错误信息',
    
    -- 数量统计
    `total_num` INT DEFAULT 0 COMMENT '总数量',
    `success_num` INT DEFAULT 0 COMMENT '成功数量',
    `failed_num` INT DEFAULT 0 COMMENT '失败数量',
    `running_num` INT DEFAULT 0 COMMENT '运行中数量',
    
    -- 时间信息
    `start_at` DATETIME DEFAULT NULL COMMENT '开始时间',
    `end_at` DATETIME DEFAULT NULL COMMENT '结束时间',
    
    -- 审计字段
    `creator` VARCHAR(64) NOT NULL COMMENT '创建人',
    `reviser` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '修改人',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_suborder_step_name` (`suborder_id`, `step_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='申请单步骤记录表';

CREATE TABLE `ziyan_cvm_generate_record` (
    `generate_id` VARCHAR(64) NOT NULL COMMENT '主键ID, 生产ID(同一子单可能有多次生产)',
    
    -- 关联信息
    `suborder_id` VARCHAR(64) NOT NULL COMMENT '子单ID(格式: order_id-序号)',
    `generate_type` VARCHAR(32) DEFAULT '' COMMENT '生产类型(QCLOUDCVM/PM等)',
    
    -- 任务信息
    `task_id` VARCHAR(128) DEFAULT '' COMMENT '生成任务ID(如云梯任务ID)',
    `task_link` VARCHAR(512) DEFAULT '' COMMENT '生成任务链接',
    `request_info` TEXT COMMENT '请求信息',
    
    -- 任务状态
    `status` TINYINT NOT NULL DEFAULT -1 COMMENT '状态(-1:默认 0:成功 1:失败 2:运行中等)',
    `is_matched` TINYINT(1) DEFAULT 0 COMMENT '是否已匹配(0:否 1:是)',
    `is_manual_matched` TINYINT(1) DEFAULT 0 COMMENT '是否手工匹配(0:否 1:是)',
    `message` TEXT COMMENT '状态消息/错误信息',
    
    -- 数量统计
    `total_num` INT DEFAULT 0 COMMENT '总数量',
    `success_num` INT DEFAULT 0 COMMENT '成功数量',
    `success_list` JSON COMMENT '成功列表(IP列表等)',
    
    -- 时间信息
    `start_at` DATETIME DEFAULT NULL COMMENT '开始时间',
    `end_at` DATETIME DEFAULT NULL COMMENT '结束时间',
    
    -- 审计字段
    `creator` VARCHAR(64) NOT NULL COMMENT '创建人',
    `reviser` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '修改人',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`suborder_id`, `generate_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='申请单生产任务记录表';

CREATE TABLE `ziyan_cvm_apply_init_task` (
    `id` VARCHAR(64) NOT NULL COMMENT '主键ID',
    
    -- 关联信息
    `suborder_id` VARCHAR(64) NOT NULL COMMENT '子单ID',
    `ip` VARCHAR(64) NOT NULL COMMENT '设备IP地址',
    
    -- 任务信息
    `task_id` VARCHAR(128) DEFAULT '' COMMENT '初始化任务ID(如SCR任务ID)',
    `task_link` VARCHAR(512) DEFAULT '' COMMENT '初始化任务链接',
    
    -- 任务状态
    `status` TINYINT NOT NULL DEFAULT -1 COMMENT '状态(-1:默认 0:成功 1:失败 2:运行中等)',
    `message` TEXT COMMENT '状态消息/错误信息',
    
    -- 时间信息
    `start_at` DATETIME DEFAULT NULL COMMENT '开始时间',
    `end_at` DATETIME DEFAULT NULL COMMENT '结束时间',

    -- 审计字段
    `creator` VARCHAR(64) NOT NULL COMMENT '创建人',
    `reviser` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '修改人',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`id`),
    KEY `idx_suborder_id_ip` (`suborder_id`, `ip`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='申请单初始化任务记录表';

CREATE TABLE `ziyan_cvm_device_info` (
    `id` VARCHAR(64) NOT NULL COMMENT '主键ID',
    `order_id` BIGINT NOT NULL COMMENT '主单ID',
    `suborder_id` VARCHAR(64) NOT NULL COMMENT '子单ID',
    `generate_id` VARCHAR(64) NOT NULL COMMENT '生产ID',
    `bk_biz_id` BIGINT NOT NULL COMMENT '业务ID',
    `bk_username` VARCHAR(64) NOT NULL COMMENT '申请人',
    `bk_host_id` BIGINT DEFAULT 0 COMMENT '主机ID',
    `ip` VARCHAR(64) NOT NULL COMMENT 'IP地址',
    `asset_id` VARCHAR(64) NOT NULL COMMENT '资产ID',
    `instance_id` VARCHAR(64) DEFAULT NULL COMMENT '实例ID',
    `require_type` TINYINT NOT NULL COMMENT '需求类型',
    `resource_type` VARCHAR(32) NOT NULL COMMENT '资源类型(QCLOUDCVM/PM等)',
    `device_type` VARCHAR(64) NOT NULL COMMENT '设备类型/机型',
    `description` TEXT COMMENT '描述',
    `remark` VARCHAR(255) DEFAULT '' COMMENT '备注',
    `zone_name` VARCHAR(128) DEFAULT '' COMMENT '可用区名称(从bkcc获取并写入)',
    `zone_id` BIGINT DEFAULT 0 COMMENT '可用区ID(从bkcc获取并写入)',
    `cloud_zone` VARCHAR(64) DEFAULT '' COMMENT '云可用区',
    `cloud_region` VARCHAR(64) DEFAULT '' COMMENT '云地域',
    `module_name` VARCHAR(128) DEFAULT '' COMMENT '模块名称(从bkcc获取并写入)',
    `rack_id` VARCHAR(64) DEFAULT '' COMMENT '机架ID(从bkcc获取并写入)',
    `is_matched` TINYINT(1) DEFAULT 0 COMMENT '是否已匹配(0:否 1:是)',
    `is_checked` TINYINT(1) DEFAULT 0 COMMENT '是否已检查(0:否 1:是)',
    `is_inited` TINYINT(1) DEFAULT 0 COMMENT '是否已初始化(0:否 1:是)',
    `is_delivered` TINYINT(1) DEFAULT 0 COMMENT '是否已交付(0:否 1:是)',
    `deliverer` VARCHAR(64) DEFAULT '' COMMENT '交付人/交付系统',
    `generate_task_id` VARCHAR(128) DEFAULT '' COMMENT '生成任务ID',
    `generate_task_link` VARCHAR(512) DEFAULT '' COMMENT '生成任务链接',
    `init_task_id` VARCHAR(128) DEFAULT '' COMMENT '初始化任务ID',
    `init_task_link` VARCHAR(512) DEFAULT '' COMMENT '初始化任务链接',
    `is_manual_matched` TINYINT(1) DEFAULT 0 COMMENT '是否手动匹配(0:否 1:是)',
    `owner_ip` VARCHAR(64) DEFAULT '' COMMENT '所属母机IP',
    `creator` VARCHAR(64) NOT NULL COMMENT '创建人',
    `reviser` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '修改人',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_suborder_id_generate_ip_asset` (`suborder_id`, `generate_id`, `ip`, `asset_id`),
    KEY `idx_bk_biz_id_suborder_id` (`bk_biz_id`,`suborder_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='设备交付记录表';

CREATE TABLE `ziyan_cvm_deliver_record` (
    `id` VARCHAR(64) NOT NULL COMMENT '自增主键',
    `suborder_id` VARCHAR(64) NOT NULL COMMENT '子订单ID',
    `ip` VARCHAR(45) NOT NULL COMMENT 'IP地址',
    `asset_id` VARCHAR(64) NOT NULL COMMENT '固资号',
    `status` TINYINT NOT NULL DEFAULT -1 COMMENT '状态（-1:默认 0:成功 1:失败 2:处理中）',
    `message` VARCHAR(512) DEFAULT '' COMMENT '状态消息',
    `deliverer` VARCHAR(32) DEFAULT '' COMMENT '交付方式',
    `generate_task_id` VARCHAR(64) DEFAULT '' COMMENT '生产任务ID',
    `generate_task_link` VARCHAR(512) DEFAULT '' COMMENT '生产任务链接',
    `init_task_id` VARCHAR(64) DEFAULT '' COMMENT '初始化任务ID',
    `init_task_link` VARCHAR(512) DEFAULT '' COMMENT '初始化任务链接',
    `is_manual_matched` TINYINT(1) DEFAULT 0 COMMENT '是否手动匹配（0-否, 1-是）',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `start_at` TIMESTAMP DEFAULT NULL COMMENT '开始时间',
    `end_at` TIMESTAMP DEFAULT NULL COMMENT '结束时间',
 
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_suborder_id_ip` (`suborder_id`, `ip`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='CVM设备记录表';

CREATE TABLE `ziyan_cvm_modify_record` (
    `id` VARCHAR(64) NOT NULL COMMENT '自增主键',
    `suborder_id` VARCHAR(64) NOT NULL COMMENT '子订单ID',
    `bk_username` VARCHAR(64) NOT NULL COMMENT '蓝鲸用户名',

    -- 修改前数据（pre_data）
    `pre_total_num` INT DEFAULT NULL COMMENT '修改前-总数量',
    `pre_replicas` INT DEFAULT NULL COMMENT '修改前-副本数',
    `pre_region` VARCHAR(64) DEFAULT NULL COMMENT '修改前-地域',
    `pre_zone` VARCHAR(64) DEFAULT NULL COMMENT '修改前-可用区',
    `pre_device_type` VARCHAR(64) DEFAULT NULL COMMENT '修改前-机型',
    `pre_image_id` VARCHAR(64) DEFAULT NULL COMMENT '修改前-镜像ID',
    `pre_disk_size` INT DEFAULT NULL COMMENT '修改前-磁盘大小',
    `pre_disk_type` VARCHAR(32) DEFAULT NULL COMMENT '修改前-磁盘类型',
    `pre_network_type` VARCHAR(32) DEFAULT NULL COMMENT '修改前-网络类型',
    `pre_vpc` VARCHAR(64) DEFAULT NULL COMMENT '修改前-VPC',
    `pre_subnet` VARCHAR(64) DEFAULT NULL COMMENT '修改前-子网',
    `pre_system_disk_type` VARCHAR(32) DEFAULT NULL COMMENT '修改前-系统盘类型',
    `pre_system_disk_size` INT DEFAULT NULL COMMENT '修改前-系统盘大小',
    `pre_system_disk_num` INT DEFAULT NULL COMMENT '修改前-系统盘数量',
    `pre_data_disk` JSON DEFAULT NULL COMMENT '修改前-数据盘配置（JSON数组）',
    `pre_zones` JSON DEFAULT NULL COMMENT '修改前-可用区列表（JSON数组）',
    `pre_res_assign` TINYINT DEFAULT NULL COMMENT '修改前-资源分配方式',
    `pre_bk_asset_id` VARCHAR(64) DEFAULT '' COMMENT '修改前-被继承固资编号',
    `pre_inherit_instance_id` VARCHAR(64) DEFAULT '' COMMENT '修改前-被继承云主机实例ID',
    
    -- 修改后数据（cur_data）
    `cur_total_num` INT DEFAULT NULL COMMENT '修改后-总数量',
    `cur_replicas` INT DEFAULT NULL COMMENT '修改后-副本数',
    `cur_region` VARCHAR(64) DEFAULT NULL COMMENT '修改后-地域',
    `cur_zone` VARCHAR(64) DEFAULT NULL COMMENT '修改后-可用区',
    `cur_device_type` VARCHAR(64) DEFAULT NULL COMMENT '修改后-机型',
    `cur_image_id` VARCHAR(64) DEFAULT NULL COMMENT '修改后-镜像ID',
    `cur_disk_size` INT DEFAULT NULL COMMENT '修改后-磁盘大小',
    `cur_disk_type` VARCHAR(32) DEFAULT NULL COMMENT '修改后-磁盘类型',
    `cur_network_type` VARCHAR(32) DEFAULT NULL COMMENT '修改后-网络类型',
    `cur_vpc` VARCHAR(64) DEFAULT NULL COMMENT '修改后-VPC',
    `cur_subnet` VARCHAR(64) DEFAULT NULL COMMENT '修改后-子网',
    `cur_system_disk_type` VARCHAR(32) DEFAULT NULL COMMENT '修改后-系统盘类型',
    `cur_system_disk_size` INT DEFAULT NULL COMMENT '修改后-系统盘大小',
    `cur_system_disk_num` INT DEFAULT NULL COMMENT '修改后-系统盘数量',
    `cur_data_disk` JSON DEFAULT NULL COMMENT '修改后-数据盘配置（JSON数组）',
    `cur_zones` JSON DEFAULT NULL COMMENT '修改后-可用区列表（JSON数组）',
    `cur_res_assign` TINYINT DEFAULT NULL COMMENT '修改后-资源分配方式',
    `cur_bk_asset_id` VARCHAR(64) DEFAULT '' COMMENT '修改后-被继承固资编号',
    `cur_inherit_instance_id` VARCHAR(64) DEFAULT '' COMMENT '修改后-被继承云主机实例ID',

    `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态：0-待审批, 1-已审批, 2-审批失败, 3-已拒绝, 4-审批超时/作废',
    `approver` VARCHAR(64) DEFAULT NULL COMMENT '审批人',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间（毫秒精度）',
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间（毫秒精度）',

    PRIMARY KEY (`id`),
    KEY `idx_suborder_id` (`suborder_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='变更记录表';

insert into id_generator(`resource`, `max_id`) values 
('ziyan_cvm_apply_step', '0'),
('ziyan_cvm_generate_record', '0'),
('ziyan_cvm_apply_init_task', '0'),
('ziyan_cvm_device_info', '0'),
('ziyan_cvm_deliver_record', '0'),
('ziyan_cvm_modify_record', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT;
