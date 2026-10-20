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
    SQLVER=9999,HCMVER=v9.9.9

    TODO(temp_delete): 临时 iac_plugin_temp_delete 建表脚本，用于 Schema 同步自测，合并插件注册分支后删除。
    对齐《插件注册与升级》设计方案 §2.1 表结构，extension 见 §2.2 / §2.3
*/

START TRANSACTION;

-- 临时注册表 iac_plugin_temp_delete：与正式 iac_plugin 区分表名避免测试环境冲突。
CREATE TABLE IF NOT EXISTS `iac_plugin_temp_delete`
(
    `id`             varchar(64)  NOT NULL COMMENT '主键',
    `plugin_name`    varchar(64)  NOT NULL COMMENT '插件名',
    `plugin_version` varchar(64)  NOT NULL COMMENT '插件版本',
    `protocol`       varchar(32)  NOT NULL COMMENT '插件协议：tf_provider / http',
    `state`          varchar(32)  NOT NULL COMMENT '状态：init / enabled / disabled / exception',
    `message`        varchar(255) NOT NULL DEFAULT '' COMMENT '系统写入的状态说明，exception 时写明原因',
    `extension`      json         NOT NULL COMMENT '协议相关信息，结构随 protocol 不同',
    `memo`           varchar(255)          DEFAULT '' COMMENT '备注',
    `creator`        varchar(64)  NOT NULL COMMENT '创建者',
    `reviser`        varchar(64)  NOT NULL COMMENT '更新者',
    `created_at`     timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`     timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_uk_plugin_name_version` (`plugin_name`, `plugin_version`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin COMMENT ='IaC 插件临时测试注册表（合并后删除）';

INSERT INTO id_generator(`resource`, `max_id`)
VALUES ('iac_plugin_temp_delete', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT;
