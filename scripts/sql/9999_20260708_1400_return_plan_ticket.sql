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
    1. 新增退回计划主单据表 return_plan_ticket
    2. 新增退回计划子单据表 return_plan_sub_ticket（与 CRP 退回计划单一一对应）
*/

START TRANSACTION;

-- 1. 退回计划主单据表
create table if not exists `return_plan_ticket`
(
    `id`                varchar(64)   not null comment '主键',
    `type`              varchar(64)   not null comment '单据类型：add/adjust/cancel',
    `details`           json          not null comment '退回计划条目列表',
    `applicant`         varchar(64)   not null comment '提单人',
    `bk_biz_id`         bigint        not null comment '业务ID',
    `bk_biz_name`       varchar(64)   not null comment '业务名称',
    `op_product_id`     bigint        not null comment '运营产品ID',
    `op_product_name`   varchar(64)   not null comment '运营产品名称',
    `plan_product_id`   bigint        not null comment '规划产品ID',
    `plan_product_name` varchar(64)   not null comment '规划产品名称',
    `virtual_dept_id`   bigint        not null comment '虚拟部门ID',
    `virtual_dept_name` varchar(64)   not null comment '虚拟部门名称',
    `status`            varchar(64)   not null comment '单据状态：init/auditing/rejected/partial_rejected/revoked/done/failed/partial_failed/terminated',
    `message`           varchar(2048) not null default '' comment '单据处理信息，失败时聚合各失败子单原因',
    `remark`            varchar(1024) not null default '' comment '退回计划说明',
    `submitted_at`      datetime      not null comment '提单时间',
    `creator`           varchar(64)   not null comment '创建人',
    `reviser`           varchar(64)   not null comment '更新人',
    `created_at`        timestamp     not null default current_timestamp comment '该记录创建的时间',
    `updated_at`        timestamp     not null default current_timestamp on update current_timestamp comment '该记录更新的时间',
    primary key (`id`),
    index `idx_bk_biz_id` (`bk_biz_id`),
    index `idx_status` (`status`)
    ) engine = innodb
    default charset = utf8mb4
    collate utf8mb4_bin comment ='退回计划主单据表';

-- 2. 退回计划子单据表
create table if not exists `return_plan_sub_ticket`
(
    `id`                 varchar(64)   not null comment '主键',
    `ticket_id`          varchar(64)   not null comment '父单据ID',
    `sub_type`           varchar(64)   not null comment '子单类型：add(CRP新增单)/cancel(CRP删除单)',
    `sub_details`        json          not null comment '该子单对应的退回计划条目',
    `bk_biz_id`          bigint        not null comment '业务ID',
    `bk_biz_name`        varchar(64)   not null comment '业务名称',
    `op_product_id`      bigint        not null comment '运营产品ID',
    `op_product_name`    varchar(64)   not null comment '运营产品名称',
    `plan_product_id`    bigint        not null comment '规划产品ID',
    `plan_product_name`  varchar(64)   not null comment '规划产品名称',
    `virtual_dept_id`    bigint        not null comment '虚拟部门ID',
    `virtual_dept_name`  varchar(64)   not null comment '虚拟部门名称',
    `obs_project`        varchar(64)   not null comment '项目类型（拆单维度）',
    `res_pool_name`      varchar(64)   not null comment '资源池（拆单维度）',
    `status`             varchar(64)   not null comment '子单状态：init/auditing/rejected/revoked/invalid/done/failed/terminated',
    `crp_sn`             varchar(64)   not null default '' comment 'CRP退回计划单号',
    `crp_url`            varchar(255)  not null default '' comment 'CRP退回计划单链接',
    `message`            varchar(512)  not null default '' comment '子单处理信息，失败时返回失败原因',
    `submitted_at`       datetime      not null comment '提单时间',
    `creator`            varchar(64)   not null comment '创建人',
    `reviser`            varchar(64)   not null comment '更新人',
    `created_at`         timestamp     not null default current_timestamp comment '该记录创建的时间',
    `updated_at`         timestamp     not null default current_timestamp on update current_timestamp comment '该记录更新的时间',
    primary key (`id`),
    index `idx_ticket_id` (`ticket_id`),
    index `idx_bk_biz_id` (`bk_biz_id`),
    index `idx_status` (`status`)
    ) engine = innodb
    default charset = utf8mb4
    collate utf8mb4_bin comment ='退回计划子单据表';

-- 3. id_generator 注册
insert into id_generator(`resource`, `max_id`)
values ('return_plan_ticket', '0'),
       ('return_plan_sub_ticket', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT;
