## 1. 数据库迁移

- [x] 1.1 新增 SQL 迁移脚本 `scripts/sql/9999_*_aiagent_session_bk_biz_id.sql`：`ALTER TABLE aiagent_session ADD COLUMN bk_biz_id BIGINT NOT NULL DEFAULT -1`，并创建索引 `idx_app_user_biz(app_name, user, bk_biz_id)`

## 2. 表结构与 DAO 层

- [x] 2.1 扩展 `pkg/dal/table/aiagent/session.go`：`SessionTable` 增加 `BkBizID int64` 字段及 `SessionColumnDescriptor` 列定义
- [x] 2.2 扩展 `pkg/dal/dao/aiagent/session.go`：INSERT/SELECT/UPDATE 语句包含 `bk_biz_id` 列；Create 默认 `-1`
- [x] 2.3 确认 `session_code` 生成逻辑未引入 `bk_biz_id`（回归检查）

## 3. data-service API 类型与 Handler

- [x] 3.1 扩展 `pkg/api/data-service/aiagent/session.go`：Create 请求增加可选 `BkBizID`，响应 details 增加 `BkBizID`
- [x] 3.2 扩展 `cmd/data-service/service/aiagent/session/` Create handler：未传 `bk_biz_id` 时默认 `constant.UnassignedBiz`；显式传入 `0` 时按 `0` 写入
- [x] 3.3 确认 List/Update 链路响应包含 `bk_biz_id`，filter 支持按 `bk_biz_id` 查询；升级后历史数据均为 `-1`

## 4. data-service Client

- [x] 4.1 扩展 `pkg/client/data-service/aiagent/session.go`：Create/List 请求响应类型同步 `BkBizID` 字段

## 5. IAM 权限点

- [x] 5.1 在 `pkg/iam/sys/types.go` 新增 `BizAgentAssistant` ActionID 及显示名「业务-智能体助手」
- [x] 5.2 在 `pkg/iam/meta/resource.go` 新增 `BizAgentAssistant` ResourceType
- [x] 5.3 在 `pkg/iam/sys/initial_actions.go` 注册 Action（`RelatedResourceTypes: bizResource`，`RelatedActions: [BizAccess]`）
- [x] 5.4 在 `cmd/auth-server/service/auth/adaptor.go` 与 `gen_id.go` 实现 `genBizAgentAssistantResource`，按 `BizID` 生成 CMDB 业务资源实例
- [x] 5.5 （如需要）在 `pkg/iam/sys/initial_action_groups.go` 将新 Action 加入对应权限组

## 6. agent-server 平台 API 语义扩展

- [x] 6.1 扩展 `pkg/api/agent-server/session/` 类型：`ListSessionsResult.Details` 增加 `BkBizID`；`CreateSessionResp` 不返回 `BkBizID`
- [x] 6.2 修改 `cmd/agent-server/service/session/create.go`：Create 请求传入 `bk_biz_id = constant.UnassignedBiz`；响应不带回 `BkBizID`
- [x] 6.3 确认 `cmd/agent-server/service/session/query.go` List 不主动追加 `bk_biz_id` 过滤，客户端 filter 仍按现有机制合并，响应 details 含 `BkBizID`

## 7. agent-server 业务维度 API

- [x] 7.1 在 `cmd/agent-server/service/session/session.go` 注册 biz 路由：`bizH.Path("/bizs/{bk_biz_id}")` 下挂载 create/list/update/delete
- [x] 7.2 实现 `BizCreateSession`：解析 path `bk_biz_id`（校验 `> 0`）、IAM `BizAgentAssistant + Create` 鉴权、写入 path 中的 `bk_biz_id`
- [x] 7.3 实现 `BizListSessions`：IAM `BizAgentAssistant + Find` 鉴权，强制 filter `user=当前用户 AND bk_biz_id=path值`
- [x] 7.4 无业务权限时返回 `PermissionDenied`，错误信息包含 `bk_biz_id`
- [x] 7.5 实现 `BizUpdateSession`：解析 path `bk_biz_id` 与 `session_code`，IAM `BizAgentAssistant + Update` 鉴权，校验会话归属当前用户且 `bk_biz_id` 与 path 一致后更新会话名称
- [x] 7.6 实现 `BizDeleteSession`：解析 path `bk_biz_id` 与 `session_code`，IAM `BizAgentAssistant + Delete` 鉴权，校验会话归属当前用户且 `bk_biz_id` 与 path 一致后删除会话

## 8. Resolver 与 sessionCodeMiddleware 扩展

- [x] 8.1 定义 `SessionMeta{ThreadID, BkBizID, User}`，扩展 `Resolver` 为 `ResolveMeta`（或扩展 `Resolve` 返回值），LRU 缓存 value 改为 struct
- [x] 8.2 修改 `cmd/agent-server/service/service.go` 中 `sessionCodeMiddleware`：调用 `ResolveMeta`，无效 sessionCode 返回 400；校验 `SessionMeta.User == cts/kt.User`，不匹配时返回 `PermissionDenied`
- [x] 8.3 当 `/agui` 且 `bk_biz_id > 0` 时通过 `r.WithContext(ctx)` 与 agent-server 私有 context key 透传 `bk_biz_id`，不使用 header，不信任客户端传入的业务上下文字段
- [x] 8.4 确认 middleware 仅负责解析、归属校验、改写 body 与透传业务上下文，不直接写入 Graph/session state
- [x] 8.5 修改 `GetContextStats`：使用 `ResolveMeta` 解析 `session_code`，显式校验会话 `user` 与当前用户一致，不匹配时返回 `PermissionDenied`

## 9. Graph State 与 Prompt 注入

- [x] 9.1 扩展 `makeRunOptionResolver`：从请求 context 读取 `bk_biz_id`，当 `> 0` 时写入 `runtimeState["bk_biz_id"]`
- [x] 9.2 更新 `cmd/agent-server/etc/prompts/instruction.md`：增加 `{temp:session_bk_biz_id?}` 占位符及「有效业务会话不得询问业务 ID」约束句
- [x] 9.3 在 `cmd/agent-server/logics/agent/graph_build.go` 的 `BeforeModel` 链路新增业务上下文注入 callback：模型调用前写入 `temp:session_bk_biz_id` 并注入 Prompt 上下文
- [x] 9.4 验证 `/agui`、`/history`、`/cancel` 请求体仍仅需 `sessionCode`，无 `bk_biz_id` 参数

## 10. 接口文档

- [x] 10.1 新增 `docs/api-docs/web-server/docs/biz/agent/biz_create_session.md`（`POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create`）
- [x] 10.2 新增 `docs/api-docs/web-server/docs/biz/agent/biz_list_session.md`（`POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list`）
- [x] 10.3 更新 `docs/api-docs/web-server/docs/service/agent/create_session.md` 与 `list_session.md`，补充平台 API 语义说明；仅 list 响应包含 `bk_biz_id`
- [x] 10.4 新增 `docs/api-docs/web-server/docs/biz/agent/biz_update_session.md`（`PATCH /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}`）
- [x] 10.5 新增 `docs/api-docs/web-server/docs/biz/agent/biz_delete_session.md`（`DELETE /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}`）

## 11. 验收与自测

- [ ] 11.1 AC-001 ~ AC-004：业务 create/list/update/delete、IAM 拒绝、平台 list 跨业务
- [ ] 11.2 AC-005 ~ AC-007：`/agui` 平台权限门槛不变、State/Prompt 注入、`-1` 会话行为不变、存量会话可访问
- [ ] 11.3 AC-008：session_code 生成公式回归
- [ ] 11.4 AC-S01：跨用户 `/agui`、`/history`、`/cancel`、`context_stats`、更新、删除拒绝
- [ ] 11.5 AC-P01：单业务 100 条会话 list P99 < 500ms（测试环境）
