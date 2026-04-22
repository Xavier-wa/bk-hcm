## ADDED Requirements

### Requirement: sessionCode 预处理中间件

系统 SHALL 提供 `sessionCodeMiddleware`，应用于 `/api/v1/agent/agui`、`/api/v1/agent/cancel` 和 `/api/v1/agent/history` 路由。中间件 MUST 执行以下操作：
1. 从请求体解析 `sessionCode` 字段
2. 通过 sessionResolver 查询 data-service 获取对应的 `thread_id`
3. 生成 `runId`（UUID 格式）
4. 将 `threadId` 和 `runId` 写回请求体，删除 `sessionCode` 字段
5. 将改写后的请求转发给下游 handler

#### Scenario: 正常解析 sessionCode

- **WHEN** 客户端发送 `/agui` 请求，请求体包含 `{ "sessionCode": "a1b2...-2026031914", "messages": [...] }`
- **THEN** 中间件 MUST 解析 sessionCode 获取 threadId
- **THEN** 中间件 MUST 生成 UUID 格式的 runId
- **THEN** 下游 handler 收到的请求体 MUST 包含 `threadId` 和 `runId`，不包含 `sessionCode`

#### Scenario: sessionCode 缺失

- **WHEN** 请求体不包含 `sessionCode` 或值为空
- **THEN** 中间件 MUST 返回 HTTP 400 错误

#### Scenario: sessionCode 无效

- **WHEN** sessionCode 在数据库中不存在
- **THEN** 中间件 MUST 返回 HTTP 400 错误

### Requirement: sessionResolver 缓存

系统 SHALL 提供 `sessionResolver` 组件，负责 sessionCode → threadId 的映射查询。该组件 MUST 支持本地缓存：
- 缓存策略：带 TTL 的 LRU 缓存
- 缓存容量上限：10000 条
- TTL：30 分钟
- 缓存未命中时通过 data-service 查询

#### Scenario: 缓存命中

- **WHEN** 同一 sessionCode 第二次被解析
- **THEN** sessionResolver MUST 从本地缓存返回 threadId，不调用 data-service

#### Scenario: 缓存未命中

- **WHEN** sessionCode 首次被解析
- **THEN** sessionResolver MUST 调用 data-service 查询，获取 threadId 后写入缓存并返回

#### Scenario: 缓存 TTL 过期

- **WHEN** 缓存条目超过 30 分钟未被刷新
- **THEN** 下次查询 MUST 重新从 data-service 获取

### Requirement: 消息计数自动更新

对 `/agui` 路径的请求，中间件 SHALL 在 SSE 流结束（`next.ServeHTTP` 返回）后，异步递增该会话的 `session_content_count`。

#### Scenario: agui 请求完成后递增计数

- **WHEN** `/agui` 请求的 SSE 流正常结束
- **THEN** 系统 MUST 启动异步 goroutine 调用 data-service `incr-content-count` 接口
- **THEN** 异步操作的超时时间 MUST 为 5 秒

#### Scenario: 计数更新失败不影响主流程

- **WHEN** data-service 计数递增调用失败
- **THEN** 系统 MUST 仅记录日志，不影响已完成的 SSE 响应

#### Scenario: 非 agui 路径不触发计数

- **WHEN** `/history` 请求经过中间件处理
- **THEN** 系统 MUST NOT 触发消息计数更新

### Requirement: 路由注册改造

agent-server 的路由注册 SHALL 为 `/agui`、`/cancel` 和 `/history` 路径套上 `sessionCodeMiddleware`，同时注册新的 `/sessions/*` 会话管理路由。context-stats 接口不走 sessionCodeMiddleware，而是在 handler 内部通过 sessionResolver 完成 sessionCode → threadId 转换。

#### Scenario: 中间件正确挂载

- **WHEN** agent-server 启动并注册路由
- **THEN** `/agui`、`/cancel` 和 `/history` 路由 MUST 经过 sessionCodeMiddleware 处理
- **THEN** `/sessions/create`、`/sessions/list`、`/sessions/{session_code}`(PATCH/DELETE) 路由 MUST 直接映射到对应 handler
- **THEN** `/sessions/{session_code}/context-stats` 路由 MUST NOT 经过 sessionCodeMiddleware，而是由 handler 内部通过 sessionResolver 处理
