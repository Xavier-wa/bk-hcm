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
    1. 裁撤主机表新增项目、地域、业务、组织、负责人、CPU、忽略等字段
    2. 删除裁撤主机表固资号唯一索引，改为项目+固资号组合索引，并补充查询辅助索引
    3. 下线裁撤模块表
*/

START TRANSACTION;

-- 1. 裁撤主机表新增字段
alter table recycle_host_info
    add column `project_id` int          not null default 0 comment '裁撤项目ID',
    add column `region`     varchar(64)   not null default '' comment '地域ID',
    add column `bk_biz_id`  bigint        not null default 0 comment '业务ID',
    add column `group_id`   bigint        not null default 0 comment '组织ID',
    add column `operators`  json          comment '负责人列表',
    add column `cpu_core`   int           not null default 0 comment 'CPU核心数',
    add column `is_ignore`  tinyint(1)    not null default 0 comment '是否忽略该主机',
    add column `device_type` varchar(128) not null default '' comment '机型';

-- 2. 删除固资号唯一索引，新增组合索引与查询辅助索引
alter table recycle_host_info
    drop index `idx_uk_asset_id`,
    add index `idx_project_id_asset_id` (`project_id`, `asset_id`),
    add index `idx_bk_biz_id` (`bk_biz_id`),
    add index `idx_abolish_phase` (`abolish_phase`),
    add index `idx_is_ignore` (`is_ignore`);

-- 3. 下线裁撤模块表
drop table if exists `recycle_module_info`;

delete from id_generator where `resource` = 'recycle_module_info';

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT
