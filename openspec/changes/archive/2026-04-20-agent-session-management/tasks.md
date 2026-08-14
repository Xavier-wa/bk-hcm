## 1. 数据库与表定义

- [x] 1.1 在 `pkg/dal/table/table.go` 中新增 `AiagentSessionTable Name = "aiagent_session"` 常量
- [x] 1.2 新建 `pkg/dal/table/aiagent/session.go`，定义 `AiagentSession` 表结构体（含 `app_name` 字段）、列描述符、Validate 方法
- [x] 1.3 在 SQL 迁移目录新增 `9999_aiagent_session.sql` 建表脚本（DDL 含所有字段和索引，包括 `app_name` 字段及 `idx_app_user(app_name, user)` 联合索引）

## 2. DAO 层实现

- [x] 2.1 新建 `pkg/dal/dao/aiagent/session.go`，实现 `AiagentSession` DAO 接口（Create、Update、List、Delete、IncrContentCount）
- [x] 2.2 在 `pkg/dal/dao/dao.go` 的 `dao.Set` 中注册 `AiagentSession()` DAO

## 3. data-service API 类型定义

- [x] 3.1 新建 `pkg/api/data-service/aiagent/session.go`，定义请求/响应类型（CreateReq、UpdateReq、ListReq、DeleteReq、IncrContentCountReq 及对应响应）

## 4. data-service Handler 实现

- [x] 4.1 新建 `cmd/data-service/service/aiagent/session.go`，实现 CRUD handlers（Create、Update、List、BatchDelete、IncrContentCount）
- [x] 4.2 在 `cmd/data-service/service/service.go` 中调用 `aiagent.InitService(capability)` 注册路由

## 5. data-service Client 封装

- [x] 5.1 新建 `pkg/client/data-service/aiagent/session.go`，封装 Session client 接口（Create、Update、List、Delete、IncrContentCount），通过 `pkg/client/common/request.go` 方法进行 HTTP 请求
- [x] 5.2 在 `pkg/client/data-service/client.go` 中挂载 `Aiagent` 子模块

## 6. agent-server 会话管理 API

- [x] 6.1 新建 `cmd/agent-server/service/session_api.go`，实现对外会话管理接口（CreateSession、UpdateSession、ListSessions、DeleteSession）
- [x] 6.2 在 `cmd/agent-server/service/service.go` 或 `context_stats.go` 中注册 `/api/v1/agent/sessions/*` 路由
- [x] 6.3 改造 `context_stats.go` 中的 `GetContextStats` handler，路径参数从 `{thread_id}` 改为 `{session_code}`，handler 内部通过 `sessionResolver` 完成 sessionCode → threadId 转换

## 7. sessionCode 预处理中间件

- [x] 7.1 新建 `cmd/agent-server/service/session_middleware.go`，实现 `sessionCodeMiddleware`（解析 sessionCode、查询 threadId、生成 runId、改写请求体）
- [x] 7.2 实现 `sessionResolver`（含 LRU 缓存，容量 10000，TTL 30 分钟）
- [x] 7.3 实现消息计数异步更新逻辑（仅对 /agui 路径，SSE 结束后 goroutine 调用 incr-content-count）
- [x] 7.4 在 `cmd/agent-server/service/service.go` 中为 `/agui`、`/cancel` 和 `/history` 路由挂载 `sessionCodeMiddleware`

## 8. 接口文档沉淀

- [x] 8.1 新建 `docs/api-docs/agent-server/docs/create_session.md`，编写创建会话接口文档（POST /api/v1/agent/sessions/create），版本 v9.9.9，遵循现有文档格式（描述、URL、输入参数、调用示例、响应示例、响应参数说明）
- [x] 8.2 新建 `docs/api-docs/agent-server/docs/update_session.md`，编写更新会话接口文档（PATCH /api/v1/agent/sessions/{session_code}）
- [x] 8.3 新建 `docs/api-docs/agent-server/docs/list_session.md`，编写查询会话列表接口文档（POST /api/v1/agent/sessions/list），包含 filter + page 参数说明及 rules 表达式说明
- [x] 8.4 新建 `docs/api-docs/agent-server/docs/delete_session.md`，编写删除会话接口文档（DELETE /api/v1/agent/sessions/{session_code}）
