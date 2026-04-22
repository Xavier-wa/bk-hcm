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
    1. 新增 aiagent_session 表
*/

START TRANSACTION;

CREATE TABLE IF NOT EXISTS `aiagent_session`
(
    `id`                    VARCHAR(64)  NOT NULL COMMENT '会话ID，由id_generator生成的8位36进制字符串',
    `session_code`          VARCHAR(128) NOT NULL COMMENT '对外会话标识，格式: {md5}-{YYYYMMDDHH}',
    `session_name`          VARCHAR(255) NOT NULL DEFAULT '' COMMENT '用户可编辑的会话名称',
    `app_name`              VARCHAR(64)  NOT NULL COMMENT '应用名称，参与session_code生成',
    `user`                  VARCHAR(64)  NOT NULL COMMENT '会话所属用户',
    `thread_id`             VARCHAR(64)  NOT NULL COMMENT '框架threadId，值等于id',
    `is_temporary`          TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否为临时会话',
    `session_content_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '会话消息计数',
    `extension`             JSON                  DEFAULT NULL COMMENT '扩展字段',
    `creator`               VARCHAR(64)  NOT NULL COMMENT '创建者',
    `reviser`               VARCHAR(64)  NOT NULL COMMENT '更新者',
    `created_at`            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_session_code` (`session_code`),
    KEY `idx_app_user` (`app_name`, `user`),
    KEY `idx_creator` (`creator`),
    KEY `idx_created_at` (`created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4 COMMENT ='aiagent会话表';

insert into id_generator(`resource`, `max_id`)
values ('aiagent_session', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT;
