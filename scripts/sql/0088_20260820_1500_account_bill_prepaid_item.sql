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
    SQLVER=0088,HCMVER=v1.9.2.16

    Notes:
    1. 新增预付费账单主表 account_bill_prepaid_item
    2. account_bill_adjustment_item 新增 source / source_id / push_status / push_fail_reason / settle_state 五字段，
       以及 idx_source_id(source_id)，支撑按预付费主单点查 / IN 查询派生调账
    3. 刷取存量 account_bill_adjustment_item 里 push_status 和 settle_state 字段
*/

-- STEP 1: 建表与 DDL
START TRANSACTION;

CREATE TABLE IF NOT EXISTS `account_bill_prepaid_item`
(
    `id`                    VARCHAR(64)     NOT NULL COMMENT '预付费账单ID',
    `uuid`                  VARCHAR(255)    NOT NULL COMMENT '外部唯一标识',
    `order_year`            BIGINT(1)       NOT NULL COMMENT '订单年份',
    `order_month`           TINYINT(1)      NOT NULL COMMENT '订单月份',
    `vendor`                VARCHAR(16)     NOT NULL COMMENT '云厂商',
    `root_account_id`       VARCHAR(64)     NOT NULL COMMENT '一级账号ID',
    `main_account_id`       VARCHAR(64)     NOT NULL COMMENT '二级账号ID',
    `root_account_cloud_id` VARCHAR(255)             DEFAULT '' COMMENT '一级账号云上ID',
    `main_account_cloud_id` VARCHAR(255)             DEFAULT '' COMMENT '二级账号云上ID',
    `product_id`            BIGINT(1)                DEFAULT -1 COMMENT '运营产品ID',
    `resource_id`           VARCHAR(255)             DEFAULT '' COMMENT '资源ID',
    `invoice_id`            VARCHAR(255)             DEFAULT '' COMMENT '发票ID',
    `gpu_type`              VARCHAR(64)              DEFAULT '' COMMENT 'GPU型号',
    `device_num`            INT(1)                   DEFAULT 0 COMMENT '数量(台)',
    `card_num`              INT(1)                   DEFAULT 0 COMMENT '数量(卡)',
    `product_name`          VARCHAR(255)             DEFAULT '' COMMENT '产品名称',
    `product_spec`          VARCHAR(255)             DEFAULT '' COMMENT '产品规格',
    `region`                VARCHAR(255)             DEFAULT '' COMMENT '地域',
    `usage_start_at`        VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '使用开始时间',
    `usage_end_at`          VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '使用结束时间',
    `order_at`              VARCHAR(64)     NOT NULL COMMENT '订单时间',
    `currency`              VARCHAR(16)     NOT NULL COMMENT '币种',
    `cost`                  DECIMAL(38, 10) NOT NULL COMMENT '优惠后总价(不含税)',
    `rmb_cost`              DECIMAL(38, 10) NOT NULL COMMENT '优惠后总价(不含税)人民币金额，由服务端按币种派生',
    `settle_state`          VARCHAR(32)     NOT NULL DEFAULT 'unsettled' COMMENT '定账状态：unsettled/settled',
    `creator`               VARCHAR(64)     NOT NULL COMMENT '创建者',
    `reviser`               VARCHAR(64)     NOT NULL COMMENT '更新者',
    `created_at`            TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`            TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid_order_month` (`uuid`, `order_year`, `order_month`),
    KEY `idx_main_account_id` (`main_account_id`),
    KEY `idx_order_year_month` (`order_year`, `order_month`),
    KEY `idx_settle_state` (`settle_state`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4 COMMENT ='预付费账单主表';

INSERT INTO id_generator(`resource`, `max_id`)
VALUES ('account_bill_prepaid_item', '0')
ON DUPLICATE KEY UPDATE `resource` = `resource`;

ALTER TABLE `account_bill_adjustment_item`
    ADD COLUMN `source` VARCHAR(32) NOT NULL DEFAULT 'manual' COMMENT '来源：manual 人工录入 / prepaid 预付费生成' AFTER `state`,
    ADD COLUMN `source_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '预付费账单ID，人工录入为空' AFTER `source`,
    ADD COLUMN `push_status` VARCHAR(32) NOT NULL DEFAULT 'unpushed' COMMENT '推送状态：unpushed/pushing/pushed/failed' AFTER `source_id`,
    ADD COLUMN `push_fail_reason` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '推送失败原因' AFTER `push_status`,
    ADD COLUMN `settle_state` VARCHAR(32) NOT NULL DEFAULT 'unsettled' COMMENT '定账状态：unsettled/settled' AFTER `push_fail_reason`,
    ADD KEY `idx_source_id` (`source_id`);

UPDATE `account_bill_adjustment_item`
SET `push_status`  = 'pushed',
    `settle_state` = 'settled'
WHERE `source` = 'manual'
  AND `state` = 'confirmed';

COMMIT;

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v1.9.2.16' AS `hcm_ver`, '0088' AS `sql_ver`;
