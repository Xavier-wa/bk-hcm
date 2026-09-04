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
    SQLVER=9999,HCMVER=v9.9.9.9

    Notes:
    1. 新增 aiagent_run_feedback 表，用于存储用户对单轮 Agent 对话的点赞/点踩反馈
    2. 向 global_config 预置 agent_feedback_tag 的 like / dislike 两份标签文案；id 从 id_generator.global_config.max_id 递增并回写 max_id
*/

START TRANSACTION;

CREATE TABLE IF NOT EXISTS `aiagent_run_feedback`
(
    `id`         VARCHAR(64)  NOT NULL COMMENT '反馈ID，由id_generator生成',
    `run_id`     VARCHAR(64)  NOT NULL COMMENT '本轮对话的run ID，一个run最多一条反馈',
    `session_id` VARCHAR(64)  NOT NULL COMMENT '所属会话ID，等于aiagent_session.id',
    `user`       VARCHAR(64)  NOT NULL COMMENT '提交反馈的用户，不可变',
    `bk_biz_id`  BIGINT       NOT NULL COMMENT '所属业务ID',
    `scene`      VARCHAR(32)  NOT NULL COMMENT '场景，与 aiagent_run.scene 对应，不可变',
    `query`      TEXT         NOT NULL COMMENT '本轮第一条用户文本，与 aiagent_run.query 对应，不可变',
    `tags`       JSON                  DEFAULT NULL COMMENT '归因标签数组',
    `reaction`   VARCHAR(16)  NOT NULL COMMENT '反馈态度：like / dislike',
    `comment`    VARCHAR(500)          DEFAULT '' COMMENT '用户手输文本',
    `creator`    VARCHAR(64)  NOT NULL COMMENT '创建者',
    `reviser`    VARCHAR(64)  NOT NULL COMMENT '更新者',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_run_id` (`run_id`),
    KEY `idx_session_id` (`session_id`),
    KEY `idx_user_created_at` (`user`, `created_at`),
    KEY `idx_reaction_created_at` (`reaction`, `created_at`),
    KEY `idx_updated_at` (`updated_at`),
    KEY `idx_scene_created_at` (`scene`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4 COMMENT ='Agent对话反馈表';

insert into id_generator(`resource`, `max_id`)
values ('aiagent_run_feedback', '0');

-- 从 id_generator.global_config.max_id 递增申请 2 个 8 位 36 进制 ID，插入后回写 max_id，
-- 与应用层 id_generator.Batch 语义一致，避免与后续 API 写入撞号。
SELECT `max_id` INTO @gc_max_id
FROM `id_generator`
WHERE `resource` = 'global_config' FOR UPDATE;
SET @gc_max_dec = CONV(@gc_max_id, 36, 10);
SET @gc_id_like = LPAD(LOWER(CONV(@gc_max_dec + 1, 10, 36)), 8, '0');
SET @gc_id_dislike = LPAD(LOWER(CONV(@gc_max_dec + 2, 10, 36)), 8, '0');

-- like tags: 回答准确、内容完整、专业清晰、解决了问题、格式清晰、其他
-- dislike tags: 事实错误、推理错误、内容不完整、内容不专业、计算错误、违法有害、
--   格式错误、乱码错误、内容重复、需画图但生成文本、工具调用失败、答非所问、其他
INSERT INTO `global_config` (`id`, `config_key`, `config_value`, `config_type`,
    `memo`, `creator`, `reviser`)
VALUES
(@gc_id_like, 'like',
 '{"accurate":"回答准确","complete":"内容完整","professional":"专业清晰","solved":"解决了问题","well_formatted":"格式清晰","other":"其他"}',
 'agent_feedback_tag', 'HCM Agent like tags', 'system', 'system'),
(@gc_id_dislike, 'dislike',
 '{"factual_error":"事实错误","reasoning_error":"推理错误","incomplete":"内容不完整","unprofessional":"内容不专业","calculation_error":"计算错误","harmful":"违法有害","format_error":"格式错误","garbled":"乱码错误","duplicated":"内容重复","chart_expected":"需画图但生成文本","tool_call_failed":"工具调用失败","off_topic":"答非所问","other":"其他"}',
 'agent_feedback_tag', 'HCM Agent dislike tags', 'system', 'system');

UPDATE `id_generator`
SET `max_id` = @gc_id_dislike
WHERE `resource` = 'global_config';

CREATE OR REPLACE VIEW `hcm_version`(`hcm_ver`, `sql_ver`) AS
SELECT 'v9.9.9.9' as `hcm_ver`, '9999' as `sql_ver`;

COMMIT;
