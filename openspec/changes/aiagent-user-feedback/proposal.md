## Why

HCM Agent 整页聊天与浮窗助手目前没有点赞 / 点踩入口，产品只能靠零散口头反馈迭代；模型评估也缺少用户校准基准。对话结束后在每条 Agent 回复下方提供常驻轻量反馈，并把结果按 run 持久化，才能支撑后续差评切片与评估交叉。

用户侧接口文档已落盘（commit `9679e1d2ab3f2e32d82f4a6bb71eb41e52c79c08`：`commit_feedback.md` / `delete_feedback.md` / `list_agent_config.md`）。本期只做后端：按该契约实现 agent-server / data-service 的提交与撤销。**不改 `front/` 或任何前端代码。** 对外 list 不在本期：用户回查与运营原始明细均不下发；运营看板列表由兄弟变更 `aiagent-model-eval` 的 `POST /api/v1/agent/eval/dashboard/feedback/list` 承担。

## What Changes

- 新增 `aiagent_run_feedback` 表：一 run 一行，赞踩互斥；API 与库列均用 `reaction=like/dislike`。data-service 内部 list 的 `filter` 直接透传。
- 用户侧接口（前缀 `/api/v1/agent/bizs/{bk_biz_id}`，业务访问，只操作自己的数据）：
  - `POST /feedback/commit`：按 `run_id` upsert；点面板「提交」才写库；请求体必传 `session_id`、`run_id`、`reaction`。
  - `DELETE /feedback/{run_id}?session_id=`：撤销，校验会话归属。
  - `GET /config/list?config_type=`：白名单下发 `global_config`；本期只放行 `agent_feedback_tag`。
- 本期 **不** 提供 `GET /feedback/list`（用户回查）与 `POST /api/v1/agent/feedback/list`（运营原始明细）。运营看板查列表走 `POST /api/v1/agent/eval/dashboard/feedback/list`。
- 标签闭集与中文文案预置到 `global_config`。迁移 SQL **必须**含 `INSERT INTO global_config`，写入 `config_type=agent_feedback_tag` 的两行（`config_key=like` / `dislike`，`config_value` 为英文 key → 中文文案的 JSON map）。提交校验与 `GET /config/list` 都读这两行，不在代码里写死 tag 闭集。`id` 必须从 `id_generator` 中 `resource=global_config` 的 `max_id` 按 8 位 36 进制递增申请（插 2 行则 `max_id+1` / `max_id+2`），插入后把该 resource 的 `max_id` 更新为本次最大值，与应用层 `id_generator.Batch` 一致，禁止手写固定 id。示例：

```sql
SELECT `max_id` INTO @gc_max_id
FROM `id_generator` WHERE `resource` = 'global_config' FOR UPDATE;
SET @gc_max_dec = CONV(@gc_max_id, 36, 10);
SET @gc_id_like = LPAD(LOWER(CONV(@gc_max_dec + 1, 10, 36)), 8, '0');
SET @gc_id_dislike = LPAD(LOWER(CONV(@gc_max_dec + 2, 10, 36)), 8, '0');
INSERT INTO `global_config` (`id`, `config_key`, `config_value`, `config_type`,
    `memo`, `creator`, `reviser`)
VALUES
(@gc_id_like, 'like',
 '{"accurate":"回答准确","complete":"内容完整","professional":"专业清晰",
   "solved":"解决了问题","well_formatted":"格式清晰","other":"其他"}',
 'agent_feedback_tag', 'HCM Agent like tags', 'system', 'system'),
(@gc_id_dislike, 'dislike',
 '{"factual_error":"事实错误","reasoning_error":"推理错误","incomplete":"内容不完整",
   "unprofessional":"内容不专业","calculation_error":"计算错误","harmful":"违法有害",
   "format_error":"格式错误","garbled":"乱码错误","duplicated":"内容重复",
   "chart_expected":"需画图但生成文本","other":"其他"}',
 'agent_feedback_tag', 'HCM Agent dislike tags', 'system', 'system');
UPDATE `id_generator` SET `max_id` = @gc_id_dislike WHERE `resource` = 'global_config';
```
- 不引入总开关配置、频控、压制表、`cooldownDays`、`minFinishedRuns`。
- 已有三份用户侧文档（commit / delete / config list）以现稿为契约，不改请求/响应形状。不新增 `list_feedback.md`。

## Capabilities

### New Capabilities

- `aiagent-run-feedback-data`：`aiagent_run_feedback` 表、DAO、data-service 内部创建/更新/删除/列表接口
- `aiagent-run-feedback-api`：agent-server 用户侧提交 / 撤销；本期不依赖 `aiagent_run` 终态校验，不提供对外 list
- `aiagent-feedback-tag-config`：`GET /config/list` 白名单、迁移 SQL `INSERT INTO global_config` 预置 like / dislike 两行

### Modified Capabilities

- （无）现有 `aiagent-session-biz-*` / `aiagent-run-*` 需求不变；本变更只读取会话与 run 账本。

## Impact

- **分层**：Service 层 `agent-server`（对外入口）+ Resource 层 `data-service`（唯一写库方）。agent-server **禁止**直连反馈表。
- **数据库**：新增 `aiagent_run_feedback` 建表语句；同一份（或紧随其后的）迁移 SQL 必须 `INSERT INTO global_config` 写入 `agent_feedback_tag` 的 like / dislike 两行，不能只建表不下发标签。`id` 从 `id_generator.global_config.max_id` 递增申请并回写 `max_id`。
- **依赖**：本期 commit 不查 `aiagent_run`。账本表由兄弟变更落地后，如产品需要再补终态校验。
- **IAM**：用户侧走「业务访问」。本期无运营对外 list，不新增权限点；看板 list 的 `agent_assistant` 由兄弟变更承担。
- **pkg**：`pkg/dal`、`pkg/api`、`pkg/client`、`pkg/cc`、`pkg/criteria/enumor`。
- **文档**：`docs/api-docs/web-server/docs/biz/agent/`（用户侧 commit / delete / config list）。
- **不涉及**：**任何前端代码**（含 `front/`）、弹窗频控 / 压制表、反馈打点、`session_code` 入库、把 `global_config` 做成通用配置后门。已有三份用户侧接口文档的请求/响应形状不改。
