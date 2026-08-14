## Context

当前 agent-server 的会话由 `trpc-agent-go` 框架内部管理，用户通过 `threadId` 标识会话。该 `threadId` 是框架内部概念，直接暴露给前端缺乏可读性，用户无法自定义会话名称或管理多个对话。

本设计在 `trpc-agent-go` 框架 session 之上引入一层业务会话管理，遵循 HCM 项目的分层架构（agent-server → data-service → MySQL），通过 `aiagent_session` 表记录会话元数据，对外暴露 `session_code` 作为唯一标识。

相关利益方：
- 前端团队：需要配合将 `/agui`、`/cancel` 和 `/history` 请求体中的 `threadId` 替换为 `sessionCode`，同时将 `/sessions/{thread_id}/context-stats` 路径参数改为 `sessionCode`
- agent-server：新增会话管理 API 和预处理中间件，改造 context-stats 接口
- data-service：新增 aiagent session 模块

## Goals / Non-Goals

**Goals:**

- 提供完整的会话 CRUD 管理能力（创建、重命名、列表查询、删除）
- 对外使用可读的 `session_code`（含 md5 和时间戳）替代框架内部 `threadId`
- 通过中间件透明改造现有 `/agui`、`/cancel` 和 `/history` 接口，保持框架层无感知
- 每次对话完成后自动更新会话消息计数
- sessionCode → threadId 映射支持本地缓存，减少 data-service 调用

**Non-Goals:**

- 不实现会话归档 / 软删除（后续扩展）
- 不实现会话分享功能
- 不实现临时会话自动清理定时任务
- 不修改 `trpc-agent-go` 框架本身的 session 管理逻辑

## Decisions

### 1. 分层架构：agent-server 不直接操作数据库

**决策**: 所有 DB 操作通过 data-service 完成，agent-server 通过 client 调用 data-service 接口。

**理由**: 这是 HCM 项目的标准架构规范。data-service 统一管理数据访问层，agent-server 作为对外入口只负责业务编排。

**替代方案**: agent-server 直接操作数据库 → 违反项目规范，不利于数据访问统一治理。

### 2. session_code 生成策略

**决策**: `session_code = md5(app_name + user + thread_id) + "-" + YYYYMMDDHH`，由 data-service 在事务内自动生成。`app_name` 作为字段存储在 `aiagent_session` 表中。

**理由**:
- md5 保证唯一性（thread_id 本身全局唯一）
- YYYYMMDDHH 后缀提升可读性，方便按时间段筛选
- 在 data-service 事务内生成，保证 id/thread_id/session_code 的一致性
- `app_name` 持久化到表中，使 data-service 具备按应用维度查询和管理会话的能力

**替代方案**: UUID 作为 session_code → 不可读；纯时间戳 → 无法保证唯一性。

### 3. thread_id 复用 id 字段

**决策**: `aiagent_session.thread_id` 的值等于 `aiagent_session.id`（由 id_generator 生成的 8 位 36 进制字符串）。

**理由**: 简化映射关系，避免额外的 ID 分配逻辑。id_generator 生成的 ID 全局唯一，可直接作为框架的 threadId 使用。

### 4. 中间件方式改造现有接口

**决策**: 在路由层为 `/agui`、`/cancel` 和 `/history` 增加 `sessionCodeMiddleware`，拦截请求体中的 `sessionCode`，解析为 `threadId` 和 `runId` 后写回请求体。

**理由**:
- 对 agui handler 完全透明，无需修改框架层代码
- 复用已有的 agui 处理逻辑
- 关注点分离，会话解析逻辑独立于业务逻辑

**替代方案**: 在 agui handler 内部处理 sessionCode → 侵入框架代码，耦合严重。

### 5. sessionResolver 本地缓存

**决策**: 使用带 TTL 的 LRU 缓存，容量上限 10000 条，TTL 30 分钟。

**理由**: sessionCode → threadId 的映射创建后不可变，天然适合缓存。本地缓存避免每次请求都调用 data-service，减少延迟。TTL 兜底防止进程间数据不一致。

### 6. context-stats 接口改造方式

**决策**: context-stats 接口（`GET /sessions/{session_code}/context-stats`）在 handler 内部通过 `sessionResolver` 完成 sessionCode → threadId 的转换，不走 `sessionCodeMiddleware`。

**理由**:
- context-stats 的标识符在 URL 路径参数中，而 sessionCodeMiddleware 设计为解析请求 body
- context-stats 只有一个接口，在 handler 内部调一次 resolver 即可，无需为此扩展中间件
- 复用同一个 `sessionResolver` 实例（含 LRU 缓存），保持一致性

**替代方案**: 扩展中间件支持 path 参数解析 → 过度设计，且需适配 go-restful 的中间件机制。

### 7. 消息计数异步更新

**决策**: `/agui` 请求的 SSE 流结束后，在独立 goroutine 中异步调用 data-service 递增 `session_content_count`。

**理由**: 计数为辅助信息，不应影响主流程的响应时间和可靠性。SSE `ServeHTTP` 会阻塞直到流结束，返回后即表示本次 Run 已完成，可安全触发计数更新。

## Risks / Trade-offs

- **[Breaking Change]** /agui、/cancel、/history 接口请求体格式变更及 /sessions/{thread_id}/context-stats 路径参数变更 → 前后端同步上线，不设过渡期
- **[缓存一致性]** 进程间缓存不同步 → TTL 兜底（30 分钟），且映射不可变，实际不一致窗口仅在会话删除场景
- **[计数不精确]** 异步计数更新可能因 goroutine 失败丢失 → 仅记日志不重试，允许轻微不准确
- **[缓存内存占用]** 10000 条缓存的内存开销 → 每条约 200 字节，总计约 2MB，可接受
