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
    SQLVER=0091,HCMVER=v1.9.3.1

    Notes:
    1. 新增 aiagent_run_eval 模型评估结果表，含 user / bk_biz_id，与 aiagent_run 对应，供按用户和业务筛选
*/

START TRANSACTION;

CREATE TABLE IF NOT EXISTS `aiagent_run_eval`
(
    `id`               VARCHAR(64)  NOT NULL COMMENT '主键，由id_generator生成',
    `run_id`           VARCHAR(64)  NOT NULL COMMENT '被评轮，等于 aiagent_run.run_id，唯一',
    `session_id`       VARCHAR(128) NOT NULL COMMENT 'aiagent_session.id（=thread_id），按会话聚合；对外会话键在账本 session_code',
    `user`             VARCHAR(64)  NOT NULL COMMENT '发起用户，与 aiagent_run.user 对应',
    `bk_biz_id`        BIGINT       NOT NULL COMMENT '业务ID，与 aiagent_run.bk_biz_id 对应',
    `scene`            VARCHAR(32)  NOT NULL COMMENT '场景，与 aiagent_run.scene 对应',
    `query`            TEXT         NOT NULL COMMENT '本轮第一条用户文本，与 aiagent_run.query 对应',
    `start_run_id`     VARCHAR(64)  NOT NULL COMMENT '阶段二最早一轮；等于 run_id 表示只评本轮',
    `process_score`    SMALLINT     NOT NULL COMMENT '过程分 0-100，代码计算',
    `outcome_score`    SMALLINT     NOT NULL COMMENT '结果分 0-100，代码计算',
    `quality_score`    SMALLINT     NOT NULL COMMENT '质量分 0-100，代码计算',
    `redlines`         JSON         NOT NULL COMMENT '语义红线字符串数组，无红线为[]',
    `reason_code`      VARCHAR(64)  NOT NULL COMMENT '问题分类闭集',
    `rubric_version`   VARCHAR(32)  NOT NULL COMMENT '评分标准版本',
    `eval_result`      JSON         NOT NULL COMMENT '4.4 整包含百分制 summary briefs',
    `context_snapshot` JSON         NOT NULL COMMENT '窗口成员 run_id/role/brief，不含 items',
    `eval_trace`       JSON         NOT NULL COMMENT '阶段一/二提示词与模型原文，排障用',
    `creator`          VARCHAR(64)  NOT NULL COMMENT '创建者',
    `reviser`          VARCHAR(64)  NOT NULL COMMENT '更新者',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '评估完成时间',
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_run_id` (`run_id`),
    KEY `idx_created_at` (`created_at`),
    KEY `idx_quality_created_at` (`quality_score`, `created_at`),
    KEY `idx_session_id` (`session_id`),
    KEY `idx_start_run_id` (`start_run_id`),
    KEY `idx_reason_code` (`reason_code`),
    KEY `idx_user_created_at` (`user`, `created_at`),
    KEY `idx_bk_biz_id_created_at` (`bk_biz_id`, `created_at`),
    KEY `idx_scene_created_at` (`scene`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4 COMMENT ='aiagent模型评估结果';

insert into id_generator(`resource`, `max_id`)
values ('aiagent_run_eval', '0');

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v1.9.3.1' as `hcm_ver`, '0091' as `sql_ver`;

COMMIT;
