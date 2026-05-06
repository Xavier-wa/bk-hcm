/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
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
 SQLVER=0073,HCMVER=v1.8.11.8

 Notes:
 新增 account_bill_region_city_rel 地域-城市映射表
*/
START TRANSACTION;

CREATE TABLE IF NOT EXISTS `account_bill_region_city_rel`
(
    `id`         varchar(64)  NOT NULL,
    `region`     varchar(128) NOT NULL COMMENT '云厂商 region 标识',
    `vendor`     varchar(64)  NOT NULL COMMENT '云厂商',
    `city_id`    int(11)      NOT NULL DEFAULT 0 COMMENT '城市ID',
    `creator`    varchar(64)  NOT NULL DEFAULT '',
    `reviser`    varchar(64)  NOT NULL DEFAULT '',
    `created_at` datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_vendor_region` (`vendor`, `region`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4 COMMENT ='账单地域-城市映射表';

insert into id_generator(`resource`, `max_id`)
values ('account_bill_region_city_rel', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v1.8.11.8' as `hcm_ver`,
       '0073'   as `sql_ver`;

COMMIT;
