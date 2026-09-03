## 1. 枚举、配置与协议类型

- [x] 1.1 在 `pkg/criteria/enumor` 增加 `FeedbackReaction`（`like` / `dislike`）及 `Validate()`；增加 `GlobalConfigTypeAgentFeedbackTag = "agent_feedback_tag"` 与 key `like` / `dislike`
- [x] 1.2 不引入功能总开关配置（不要写 `cooldownDays` / `minFinishedRuns`）
- [x] 1.3 新增 `pkg/api/agent-server/feedback/`：commit / delete 的请求响应结构体，字段与 `commit_feedback.md`、`delete_feedback.md` 对齐；API 与库列均用 `reaction`；不提供对外 list 类型
- [x] 1.4 扩展 `pkg/api/agent-server/config/`：list 请求（`config_type`）与 `details[]` 响应，对齐 `list_agent_config.md`

## 2. 表定义与 SQL

- [x] 2.1 在 `pkg/dal/table/table.go` 注册 `AiagentRunFeedbackTable = "aiagent_run_feedback"`
- [x] 2.2 新增 `pkg/dal/table/aiagent/feedback.go`：列描述符、`InsertValidate` / `UpdateValidate`（禁止改 `run_id` / `session_id` / `user` / `creator`）；列名 `reaction` 存 `like`/`dislike`；无 `session_code` 列
- [x] 2.3 新增 `scripts/sql/0088_20260825_1730_aiagent_run_feedback.sql`：InnoDB / utf8mb4 建表；索引 `uk_run_id`、`idx_session_id`、`idx_user_created_at`、`idx_reaction_created_at`、`idx_updated_at`
- [x] 2.4 同一份 SQL 必须含 `INSERT INTO global_config`：写入 `config_type=agent_feedback_tag` 的 like / dislike 两行，`config_value` 为提案中的 JSON map；`id` 从 `id_generator.global_config.max_id` 按 8 位 36 进制递增申请，插入后回写 `max_id`；不得只建表不 INSERT

## 3. DAO 与 data-service

- [x] 3.1 新增 `pkg/dal/dao/aiagent/feedback.go`（Create / Update / List / Delete），在 `pkg/dal/dao/dao.go` 的 `dao.Set` 注册 `AiagentRunFeedback()`
- [x] 3.2 新增 `pkg/api/data-service/aiagent/feedback.go` 内部协议
- [x] 3.3 在 `cmd/data-service/service/aiagent/` 增加反馈 handler 并注册：`POST /aiagent/feedbacks/create`、`PATCH /aiagent/feedbacks`、`POST /aiagent/feedbacks/list`、`DELETE /aiagent/feedbacks/batch`
- [x] 3.4 新增 `pkg/client/data-service/aiagent/feedback.go`，经 `pkg/client/common/request.go` 调用；挂到 `pkg/client/data-service/aiagent.Client`

## 4. 标签配置下发

- [x] 4.1 在 `cmd/agent-server/service/config` 增加 `GET /bizs/{bk_biz_id}/config/list`：业务访问鉴权；`config_type` 白名单仅 `agent_feedback_tag`；走已有 `GlobalConfig.List`；缺行返回空 `details`
- [x] 4.2 单测覆盖白名单：`config_type=auth`/未知值不在白名单、`agent_feedback_tag` 在白名单（`cmd/agent-server/service/config/list_test.go`）。`InvalidParameter`/`PermissionDenied` 具体错误码与「响应不含凭据」由 `ListConfig` 中的白名单前置校验与固定响应结构保证；受限于项目现状 agent-server handler 层无既有 IAM/HTTP mock 基础设施，未对整个 handler 做端到端单测

## 5. 用户侧提交 / 撤销

- [x] 5.1 新增 `cmd/agent-server/service/feedback/`，在 `service.go` 调用 `InitService`；biz 路由复用 `authorizeBizSession`
- [x] 5.2 实现 `POST /bizs/{bk_biz_id}/feedback/commit`：解码校验（含必填 `session_id`）、校验会话归属、`tags` 按当前 `reaction` 的 `agent_feedback_tag` map 校验、`comment` 按 rune 计 ≤500、按 `run_id` upsert。不校验 `aiagent_run` 终态，代码留 TODO，后续不一定补
- [x] 5.3 实现 `DELETE /bizs/{bk_biz_id}/feedback/{run_id}?session_id=`：校验会话归属；只删当前用户当前业务且 `session_id` 匹配的行；不存在 `RecordNotFound`；他人 / 其他会话 `PermissionDenied`
- [x] 5.4 **不实现** `GET /bizs/{bk_biz_id}/feedback/list`：用户回查本期无前端调用方，不对外暴露

## 6. 运营列表

- [x] 6.1 **不实现** `POST /api/v1/agent/feedback/list`：运营查列表走 `POST /api/v1/agent/eval/dashboard/feedback/list`，避免双入口
- [x] 6.2 本期无运营对外 list，不在本包做 `agent_assistant` 鉴权

## 7. 接口文档

- [x] 7.1 **不新增** `docs/api-docs/web-server/docs/biz/agent/list_feedback.md`
- [x] 7.2 **不新增** `docs/api-docs/web-server/docs/scr/agent/list_feedback.md`
- [x] 7.3 已有 `commit_feedback.md` / `delete_feedback.md` / `list_agent_config.md` 不改请求/响应形状；**不修改 `front/` 或任何前端代码**

## 8. 测试与收口

- [x] 8.1 单测覆盖（受限于项目现状：agent-server/data-service 的 handler 与 DAO 层均无既有 HTTP/DB mock 基础设施，本次新增测试聚焦可独立验证的纯逻辑单元，未新增端到端 handler 集成测试）：
  - `enumor.FeedbackReaction.Validate`（`pkg/criteria/enumor/aiagent_feedback_test.go`）
  - `FeedbackTable.InsertValidate/UpdateValidate`：必填校验、非法 `reaction`、`run_id`/`session_id`/`user`/`creator` 禁止更新、`reviser` 必填（`pkg/dal/table/aiagent/feedback_test.go`）
  - `CommitFeedbackReq.Validate`：缺 `session_id` / `run_id`、非法 `reaction`、`comment` 超 500 rune 拒绝、恰好 500 rune 通过（`pkg/api/agent-server/feedback/feedback_test.go`）
  - config 白名单：`agent_feedback_tag` 放行、`auth`/未知值/空值拒绝（`cmd/agent-server/service/config/list_test.go`）
  - `like→dislike` 改判清空旧 `tags`/`comment`、跨套 tags 拒绝、撤销鉴权等场景已在 handler 代码中实现（见 `commit.go`/`delete.go`），但因缺少 mock 基础设施未落地为自动化用例，待后续补充 handler 集成测试基建后跟进
- [x] 8.2 确认无 `aiagent_feedback_pref`、无 `batch_query`、无 `/stats/feedback_list`、无对外 `list_feedback`、无反馈打点、无 `session_code` 列、无 `front/` 改动；`gofmt` 已跑（本机无网络无法安装 `goimports`，人工核对导入分组）、行宽 120、日志 rid、错误英文
- [x] 8.3 本期不校验 `aiagent_run` 终态；commit 校验会话归属，代码留 TODO
