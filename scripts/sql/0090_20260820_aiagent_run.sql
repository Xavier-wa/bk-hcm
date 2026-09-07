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
    SQLVER=0090,HCMVER=v1.9.3.1

    Notes:
    1. 新增 aiagent_run 对话轮次账本
*/

START TRANSACTION;

CREATE TABLE IF NOT EXISTS `aiagent_run`
(
    `id`           VARCHAR(64)  NOT NULL COMMENT '主键，由id_generator生成',
    `run_id`       VARCHAR(64)  NOT NULL COMMENT 'AG-UI runId，唯一',
    `session_code` VARCHAR(128) NOT NULL COMMENT '所属会话对外ID',
    `user`         VARCHAR(64)  NOT NULL COMMENT '发起用户，创建后不变',
    `bk_biz_id`    BIGINT       NOT NULL COMMENT '业务ID',
    `scene`        VARCHAR(32)  NOT NULL COMMENT 'host_apply/resource_query/chat/unsupported',
    `status`       VARCHAR(32)  NOT NULL DEFAULT 'running' COMMENT 'running/finished/error/cancel/unknown',
    `reason`       VARCHAR(256) NOT NULL DEFAULT '' COMMENT '附加原因摘要',
    `query`        TEXT         NOT NULL COMMENT '本轮第一条用户文本',
    `transcript`   JSON                  DEFAULT NULL COMMENT '本轮对话时间线 items[]',
    `creator`      VARCHAR(64)  NOT NULL COMMENT '创建者',
    `reviser`      VARCHAR(64)  NOT NULL COMMENT '更新者',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间，兼作开始时间',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间，兼作结束时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_run_id` (`run_id`),
    KEY `idx_updated_at_status` (`updated_at`, `status`),
    KEY `idx_status_created_at` (`status`, `created_at`),
    KEY `idx_session_code_created_at` (`session_code`, `created_at`),
    KEY `idx_user_updated_at` (`user`, `updated_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4 COMMENT ='aiagent对话轮次账本';

insert into id_generator(`resource`, `max_id`)
values ('aiagent_run', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v1.9.3.1' as `hcm_ver`, '0090' as `sql_ver`;

COMMIT;
