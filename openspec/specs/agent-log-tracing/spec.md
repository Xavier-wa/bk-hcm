# Spec: agent-log-tracing

## Purpose

定义 agent-server 的日志 rid 追踪能力。通过在 LLM 调用、工具执行、Memory 操作、Skill 图回调、Embedding 中间件、动态工具索引/过滤等模块的日志中统一注入 rid，并将 rid 同步写入请求 context 与 HTTP 响应头，实现跨模块调用链串联，便于日志聚合系统按 rid 过滤完整调用链。

## Requirements

### Requirement: 日志必须包含 rid 标识

所有 Agent 相关日志（LLM 调用、工具执行、Memory 操作、Skill 图回调、Embedding 中间件等）MUST 包含 rid（request id）标识，格式为 `rid: <uuid>`，放置在日志消息末尾，用于串联同一请求的调用链路。

#### Scenario: LLM 调用日志包含 rid

- **WHEN** Agent 执行 LLM 推理请求（包括请求发送和响应接收）
- **THEN** 日志中必须包含 `rid: <uuid>` 字段

#### Scenario: Tool 执行日志包含 rid

- **WHEN** Agent 调用工具（包括 BeforeTool 和 AfterTool callback）
- **THEN** 日志中必须包含 `rid: <uuid>` 字段

#### Scenario: Memory 操作日志包含 rid

- **WHEN** Agent 执行 Memory 增删改查操作
- **THEN** 日志中必须包含 `rid: <uuid>` 字段

#### Scenario: MCP HTTP 请求日志包含 rid

- **WHEN** Agent 通过 MCP 协议调用外部工具
- **THEN** HTTP 请求和响应日志中必须包含 `rid: <uuid>` 字段

#### Scenario: Skill 图回调与 Prompt 注入日志包含 rid

- **WHEN** Skill 图节点 AfterNodeCallback 触发，或 BeforeModel 阶段注入 skill prompt
- **THEN** 相关日志必须包含 `rid: <uuid>` 字段

#### Scenario: Embedding HTTP 调用错误日志包含 rid

- **WHEN** Embedding 客户端的 HTTP 中间件捕获到 transport error 或 HTTP 4xx/5xx
- **THEN** 错误日志中必须包含 `rid: <uuid>` 字段

#### Scenario: 动态工具索引/过滤日志包含 rid

- **WHEN** `LazyToolIndex.ensureBuild` 构建索引、`EmbeddingIndex.Search` 搜索工具、或 `MakeDynamicToolFilter` 过滤工具
- **THEN** 相关日志必须包含 `rid: <uuid>` 字段

### Requirement: rid 必须从 context 中提取并复用通用函数

所有 callback 和中间件 MUST 通过 `rest.RidFromContext(ctx)` 从 context 中提取 rid，而非生成新的 rid，确保同一请求的 rid 一致性，并避免在各模块重复实现提取逻辑。

#### Scenario: Callback 从 context 提取 rid

- **WHEN** Model/Tool/Memory callback 被调用且 context 中存在 rid
- **THEN** 使用 context 中的 rid 而非生成新 rid

#### Scenario: HTTP 入口将 rid 注入 context

- **WHEN** 请求进入 `bkapiContextMiddleware` 且 header `X-Bkapi-Request-Id` 已存在或被中间件兜底生成
- **THEN** rid 必须通过 `context.WithValue(ctx, constant.RidKey, rid)` 注入 context，使下游 callback 可通过 `rest.RidFromContext(ctx)` 获取

#### Scenario: Context 中无 rid 时的降级处理

- **WHEN** Callback 从 context 提取 rid 失败（context 为 nil、rid 不存在或类型错误）
- **THEN** `rest.RidFromContext` 返回空字符串，日志中 `rid: ` 字段为空，但不影响业务逻辑执行

### Requirement: rid 必须回写到响应头

HTTP 响应头 MUST 包含 `X-Bkapi-Request-Id`，并与当前请求 context 中的 rid 一致，以便客户端 / 网关侧关联调用链。

#### Scenario: 普通响应包含 rid 响应头

- **WHEN** Agent HTTP 请求正常返回（包括 2xx 和错误状态码）
- **THEN** 响应头中必须包含 `X-Bkapi-Request-Id`，其值与日志中的 rid 完全一致

#### Scenario: 流式响应包含 rid 响应头

- **WHEN** Agent 通过 SSE/AG-UI 返回流式响应，首次调用 `Write` 时还未显式调用 `WriteHeader`
- **THEN** 包装的 `ResponseWriter` 必须在首次 `Write` 前自动将 rid 写入响应头

### Requirement: 支持跨模块调用链追踪

通过统一的 rid 标识，系统 MUST 支持在日志中过滤和串联同一请求在不同模块（LLM → Tool → Memory → Skill → Embedding）的所有日志，形成完整调用链。

#### Scenario: 单次 Agent 调用的日志可串联

- **WHEN** 用户发起一次 Agent 请求，依次触发 LLM 推理、工具调用、Memory 查询
- **THEN** 所有相关日志包含相同的 rid，可通过 `grep "rid: <uuid>"` 过滤出完整调用链

#### Scenario: 并发请求的日志可区分

- **WHEN** 多个用户并发发起 Agent 请求
- **THEN** 每个请求的 rid 唯一，不同请求的日志可通过 rid 完全隔离
