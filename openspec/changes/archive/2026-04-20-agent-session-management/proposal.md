## Why

当前 agent-server 的会话由 `trpc-agent-go` 框架内部管理，直接将框架 `threadId` 暴露给前端，导致用户无法自定义会话名称、无法管理多个对话、缺少可读的会话标识。需要引入一层业务会话管理，提供完整的会话生命周期管理能力。

## What Changes

- 新增 `aiagent_session` 数据库表，存储会话元数据（session_code、session_name、app_name、user、thread_id 等）
- 新增 data-service 内部 CRUD 接口：创建、更新、列表查询、批量删除、消息计数递增
- 新增 agent-server 对外会话管理 API：创建会话、更新会话名称、查询会话列表、删除会话
- **BREAKING**: 改造现有 `/agui`、`/cancel` 和 `/history` 接口，客户端从传 `threadId` 改为传 `sessionCode`，由中间件自动解析转换
- **BREAKING**: 改造现有 `/sessions/{thread_id}/context-stats` 接口，路径参数从 `threadId` 改为 `sessionCode`，handler 内部完成转换
- 新增 `sessionCodeMiddleware` 预处理中间件，负责 sessionCode → threadId 映射及 runId 自动生成
- 每次 `/agui` 请求完成后异步递增会话消息计数

## Capabilities

### New Capabilities

- `aiagent-session-data`: aiagent_session 表定义（含 app_name 字段）、DAO 层 CRUD 实现、data-service 接口（创建/更新/列表/删除/计数递增）
- `aiagent-session-api`: agent-server 对外会话管理 API（创建/更新/列表/删除）、context-stats 接口改造（sessionCode→threadId）及 data-service client 封装
- `aiagent-session-middleware`: sessionCode 预处理中间件，改造 /agui、/cancel 和 /history 接口，自动完成 sessionCode→threadId 映射、runId 生成及消息计数更新

### Modified Capabilities

（无需修改现有 spec，本次变更均为新增能力）

## Impact

- **数据库**: 新增 `aiagent_session` 表及迁移脚本
- **data-service**: 新增 aiagent session 模块（handler、DAO、路由注册）
- **agent-server**: 新增会话管理 API 路由、新增 sessionCode 中间件、修改现有 /agui、/cancel 和 /history 路由注册方式、改造 context-stats 接口支持 sessionCode
- **pkg/client**: 新增 data-service aiagent session client 模块
- **pkg/dal**: 新增表定义和 DAO 层
- **pkg/api**: 新增 data-service aiagent session 请求/响应类型
- **前端**: 需配合改造，/agui、/cancel 和 /history 请求体从 threadId 改为 sessionCode（breaking change）
