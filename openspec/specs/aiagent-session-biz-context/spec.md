## Requirements

### Requirement: Resolver 解析会话元数据

`session.Resolver` SHALL 支持通过 `session_code` 解析会话元数据，至少包含 `thread_id`、`bk_biz_id` 与 `user`。SHALL 使用 LRU 缓存（容量 10000、TTL 30 分钟）缓存元数据，避免重复查询 data-service。

#### Scenario: 缓存命中返回元数据

- **WHEN** 同一 `session_code` 在 TTL 内第二次解析
- **THEN** Resolver MUST 从缓存返回 `thread_id`、`bk_biz_id` 与 `user`，MUST NOT 再次调用 data-service

#### Scenario: session_code 不存在

- **WHEN** `session_code` 在数据库中不存在
- **THEN** Resolver MUST 返回 `RecordNotFound` 类错误

### Requirement: sessionCodeMiddleware 解析 session_code

`sessionCodeMiddleware`（覆盖 `/agui`、`/cancel`、`/history`）SHALL 调用 Resolver 解析 `sessionCode`，校验会话 `user` 与当前登录用户一致，并将 `threadId` 与 `runId` 写回请求体；当解析到 `bk_biz_id > 0` 且请求为 `/agui` 时，SHALL 通过 `r.WithContext(ctx)` 与私有 context key 透传 `bk_biz_id`，供下游 RunOptionResolver 与 BeforeModel callback 使用。`session_code` 无效时 MUST 返回 HTTP 400。

#### Scenario: 成功解析并改写请求体

- **WHEN** 请求体包含有效 `sessionCode`
- **THEN** middleware MUST 删除 `sessionCode`、写入 `threadId` 与 `runId`，并转发至 agui handler
- **THEN** 若会话 `bk_biz_id > 0` 且请求为 `/agui`，middleware MUST 通过请求 context 透传该业务 ID 给下游模型运行链路

#### Scenario: 无效 sessionCode 拒绝

- **WHEN** `sessionCode` 无法解析
- **THEN** middleware MUST 返回 HTTP 400

#### Scenario: sessionCode 归属用户不匹配

- **WHEN** `sessionCode` 可解析，但解析出的 `user` 与当前登录用户不一致
- **THEN** middleware MUST 拒绝访问并返回权限错误

#### Scenario: 不接受客户端伪造业务上下文

- **WHEN** 客户端在请求 header 或请求体中传入自定义 `bk_biz_id`
- **THEN** middleware 与 RunOptionResolver MUST 仅信任 Resolver 解析出的会话元数据

### Requirement: 有效业务会话 Graph State 注入

当请求 context 透传的 `bk_biz_id > 0` 时，`makeRunOptionResolver`（或等效 Run 配置入口）SHALL 通过 `agent.WithRuntimeState` 将 `bk_biz_id` 写入 Graph 运行时 State，供 Function 节点（如 `fetch_plans`）直接消费。

#### Scenario: 业务会话 agui 注入 State

- **WHEN** 通过业务 API 创建且 `bk_biz_id=123` 的 `session_code` 用于 `POST /api/v1/agent/agui`
- **THEN** Graph Run 的 runtime State MUST 包含 `bk_biz_id=123`（或等效 int64 值）

#### Scenario: 未分配业务不注入 State

- **WHEN** 会话 `bk_biz_id=-1` 的 `session_code` 用于 `/agui`
- **THEN** runtime State MUST NOT 包含有效业务 `bk_biz_id` 注入，行为与改造前一致

### Requirement: 有效业务会话 Prompt 注入

当 `bk_biz_id > 0` 时，系统 SHALL 在 Agent 执行前将业务 ID 注入 Instruction/System Prompt 上下文，使 LLM 知晓当前会话所属业务；SHALL 在 `BeforeModel` callback（`cmd/agent-server/logics/agent/graph_build.go` 注册链路）中通过 session `temp:` 命名空间占位符（如 `{temp:session_bk_biz_id?}`）或等效机制实现。Instruction MUST 包含约束：有效业务会话首轮 MUST NOT 向用户询问业务 ID。

#### Scenario: 业务会话 Prompt 含业务 ID

- **WHEN** `bk_biz_id=123` 的会话发起首轮 `/agui` 对话
- **THEN** 注入后的 Prompt/Instruction MUST 包含业务 ID `123` 的明确说明

#### Scenario: 首轮不询问业务 ID

- **WHEN** `bk_biz_id=123` 的会话发起首轮 `/agui` 对话
- **THEN** Agent 首轮回复 MUST NOT 向用户询问业务 ID

#### Scenario: 未分配业务 Prompt 不变

- **WHEN** `bk_biz_id=-1` 的会话发起 `/agui` 对话
- **THEN** 系统 MUST NOT 注入业务 ID 至 Prompt，行为与改造前一致

### Requirement: BeforeModel 写入 temp 状态

当 `bk_biz_id > 0` 且请求为 `/agui` 时，模型调用前的 `BeforeModel` callback SHALL 通过当前 Graph/session 上下文写入 session 级 `temp:session_bk_biz_id` 状态，供 Instruction 占位符展开；`sessionCodeMiddleware` MUST NOT 直接写入 session state。当请求为 `/history`、`/cancel` 或 `bk_biz_id = -1` 时 MUST NOT 写入。

#### Scenario: agui 模型调用前写入 temp 命名空间

- **WHEN** 有效业务会话的 `/agui` 请求进入模型调用前 callback
- **THEN** 对应 session 的 `temp:session_bk_biz_id` MUST 被设置为 path/记录中的 `bk_biz_id` 字符串值

#### Scenario: history 与 cancel 不写入 temp 命名空间

- **WHEN** 有效业务会话的 `/history` 或 `/cancel` 请求经 middleware 处理
- **THEN** 系统 MUST NOT 写入 `temp:session_bk_biz_id`

### Requirement: 业务列表查询性能

单用户单业务会话列表查询（≤200 条）在测试环境基准下 P99 响应时间 MUST < 500ms；依赖 `idx_app_user_biz` 索引保障。

#### Scenario: 100 条会话列表性能

- **WHEN** 单用户单业务存在 100 条会话，调用业务 list API
- **THEN** P99 响应时间 MUST < 500ms（测试环境基准）

### Requirement: context_stats 会话归属显式校验

`GetContextStats` SHALL 使用 Resolver 解析 `session_code` 会话元数据，并显式校验解析出的 `user` 与当前登录用户一致；不一致时 MUST 返回权限错误，且 MUST NOT 读取或返回该会话上下文统计。

#### Scenario: 当前用户访问自己的 context_stats

- **WHEN** 用户 A 使用属于用户 A 的 `session_code` 调用 `/sessions/{session_code}/context_stats`
- **THEN** 系统 MUST 通过归属校验并返回该会话上下文统计

#### Scenario: 当前用户访问他人的 context_stats

- **WHEN** 用户 B 使用属于用户 A 的 `session_code` 调用 `/sessions/{session_code}/context_stats`
- **THEN** 系统 MUST 返回 `PermissionDenied` 或等效未授权错误
- **THEN** 系统 MUST NOT 读取或返回用户 A 的会话上下文统计
