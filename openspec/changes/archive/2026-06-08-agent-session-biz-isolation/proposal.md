## Why

HCM AI 助手（agent-server Graph 模式主机申领 workflow）当前会话管理仅按 user、threadId、app 三个维度组织，多业务用户切换业务时会话列表无法隔离，且 Agent 首轮需反复询问 `bk_biz_id`。在已有业务会话层（`aiagent_session`）基础上引入 `bk_biz_id` 绑定，可在创建/列表时按业务隔离，并通过 `session_code` 自动注入业务上下文，提升主机申领流程效率。

## What Changes

- `aiagent_session` 表新增 `bk_biz_id` 列（BIGINT NOT NULL DEFAULT -1）及联合索引 `idx_app_user_biz`（`app_name`, `user`, `bk_biz_id`）；`session_code` 生成规则不变
- data-service aiagent session CRUD 链路支持读写 `bk_biz_id`；存量数据迁移后默认 `-1`（`constant.UnassignedBiz`）
- agent-server 新增业务维度会话 API：
  - `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create`
  - `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list`（强制过滤当前用户 + path 中的 `bk_biz_id`）
- 保留平台维度原 API（`POST /sessions/create`、`POST /sessions/list`）：列表返回用户全部业务会话；创建写入 `bk_biz_id = -1`
- 扩展 `sessionCodeMiddleware` / `Resolver`：解析 `session_code` 后读取 `bk_biz_id`，当值 > 0 时由 middleware 透传业务上下文，RunOptionResolver 注入 Graph State，`BeforeModel` callback 注入 System Prompt；`/agui`、`/history`、`/cancel` 请求体不增加 `bk_biz_id` 参数
- 新增 IAM 业务资源权限点「业务-智能体助手」，业务 API 鉴权使用该权限 + 对应 `bk_biz_id` 资源实例
- 新增 SQL 迁移脚本与 API 文档（`docs/api-docs/web-server/docs/service/agent/`）

## Capabilities

### New Capabilities

- `aiagent-session-biz-data`：`aiagent_session` 表 `bk_biz_id` 字段、索引、DDL 迁移，以及 data-service/DAO/client 全链路读写支持
- `aiagent-session-biz-api`：agent-server 业务维度与平台维度会话 create/list API、IAM 权限点注册与鉴权、API 文档
- `aiagent-session-biz-context`：`sessionCodeMiddleware` / `Resolver` 扩展，有效业务会话下将 `bk_biz_id` 注入 Graph State 与 System Prompt

### Modified Capabilities

（无。既有 `aiagent-session-*` 能力仅存在于已归档变更 `2026-04-20-agent-session-management`，未纳入 `openspec/specs/` 主 spec；本次为在其之上的增量能力，不修改现有主 spec。）

## Impact

- **数据库**：新增迁移脚本，修改 `aiagent_session` 表结构
- **data-service**：`pkg/dal/table/aiagent`、`pkg/dal/dao/aiagent`、`cmd/data-service` aiagent session handler 扩展 `bk_biz_id`
- **agent-server**：`cmd/agent-server/service/session/` 新增业务路由；`cmd/agent-server/service/service.go` 扩展 middleware 与 RunOptionResolver；`cmd/agent-server/service/session/middleware.go` 扩展 Resolver 缓存；`cmd/agent-server/logics/agent/graph_build.go` 增加 BeforeModel 业务上下文注入
- **pkg/api**：data-service 与 agent-server session 请求/响应类型扩展
- **pkg/client**：data-service aiagent session client 扩展
- **pkg/iam**：新增业务资源 Action（如 `biz_agent_assistant`）、`meta.ResourceType`、auth-server adaptor
- **docs/api-docs**：在 `web-server/docs/service/agent/` 新增业务维度会话 API 文档
- **前端**：本变更不交付前端；前端后续对接 `/bizs/{bk_biz_id}/sessions/*`（独立需求单）
- **兼容性**：非 breaking；原 `/sessions/*` 与 `/agui`/`/history`/`/cancel` 接口路径与 `session_code` 格式保持不变
