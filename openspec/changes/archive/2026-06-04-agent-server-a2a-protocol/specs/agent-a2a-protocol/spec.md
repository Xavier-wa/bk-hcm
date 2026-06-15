## ADDED Requirements

### Requirement: A2A 协议端点挂载

`agent-server` SHALL 在启用 A2A 时，向其 HTTP 根 ServeMux 挂载以下端点：

- A2A JSON-RPC 入口：`POST /api/v1/agent/a2a`
- AgentCard（A2A v0.2.2 规范）：`GET /api/v1/agent/.well-known/agent-card.json`
- AgentCard（兼容旧规范别名）：`GET /api/v1/agent/.well-known/agent.json`

端点前缀 SHALL 由配置 `a2a.basePath` 控制，默认值为 `/api/v1/agent`，与现有 AG-UI 端点前缀保持一致。

#### Scenario: 启用 A2A 时挂载标准 A2A 端点

- **WHEN** `agent_server.yaml` 中配置 `a2a.enable: true` 且未显式设置 `a2a.basePath`
- **THEN** agent-server 启动后，`POST /api/v1/agent/a2a` 能接收 A2A JSON-RPC 请求并返回符合 A2A v0.2.2 规范的响应
- **AND** `GET /api/v1/agent/.well-known/agent-card.json` 返回 200 与 AgentCard JSON
- **AND** `GET /api/v1/agent/.well-known/agent.json` 返回与 `agent-card.json` 完全相同的 AgentCard JSON

#### Scenario: 自定义 basePath

- **WHEN** 配置 `a2a.basePath: "/api/v1/agent"` 或其它合法路径前缀
- **THEN** A2A JSON-RPC 入口与 AgentCard 端点 SHALL 全部以该 basePath 为前缀挂载
- **AND** 默认的 `/a2a/` 兜底路径 SHALL 不再被挂载

#### Scenario: 未启用 A2A 时端点不存在

- **WHEN** `a2a.enable: false`（或未配置 `a2a` 段）
- **THEN** A2A JSON-RPC 入口与 AgentCard 端点均 SHALL 不被挂载
- **AND** 对这些路径的访问 SHALL 返回 404
- **AND** agent-server 启动时 SHALL NOT 加载 A2A Server、SHALL NOT 构建 AgentCard

### Requirement: A2A Server 复用 Runner

`agent-server` 的 A2A Server SHALL 复用 `runTime.AGUIRunner`（同一 `runner.Runner` 实例），SHALL NOT 为 A2A 协议构造新的 Runner、Agent、Model、ToolSet、Session 或 Memory 实例。

#### Scenario: A2A 与 AG-UI 共享 Runner

- **WHEN** A2A 启用且 agent-server 启动完成
- **THEN** A2A Server 通过 `agoa2a.WithRunner(rt.AGUIRunner)`（或等价 API）传入与 AG-UI 同一 Runner 实例
- **AND** A2A 链路触发的工具调用 SHALL 与 AG-UI 使用同一 MCP ToolSet 配置
- **AND** A2A 链路触发的 session/memory 持久化 SHALL 写入与 AG-UI 同一存储后端

### Requirement: AgentCard 静态描述

`agent-server` SHALL 在启动时从 `cc.AgentServer().A2A.Card` 一次性构建 AgentCard，构建后 SHALL 在运行期保持不变。AgentCard SHALL NOT 在请求处理路径上动态从 `SkillManager.Repository` 或其它内部组件读取内容。

AgentCard 必须包含：
- `name`（来自 `card.name`，未配置时默认 `"HCM Agent"`）
- `description`（来自 `card.description`，未配置时默认 `"BlueKing Hybrid Cloud Management AI Agent"`）
- `version`（来自 `card.version`，未配置时默认 `"1.0.0"`）
- `url`（来自 `card.url`，可为空字符串）
- `capabilities.streaming: true`
- `defaultInputModes: ["text"]`
- `defaultOutputModes: ["text"]`
- `skills`：至少一项 `AgentSkill`，每项包含 `id`、`name`、`tags`（A2A v0.2.2 规范必填）

#### Scenario: 配置完整提供 AgentCard 描述

- **WHEN** `a2a.card` 配置了 `name`、`description`、`version`、`url` 与至少一个 `skills` 条目
- **THEN** AgentCard JSON 响应 SHALL 完整反映配置内容
- **AND** 两次连续请求 AgentCard 端点 SHALL 返回字节级完全相同的内容

#### Scenario: 配置未提供 skills 时使用内置占位

- **WHEN** `a2a.enable: true` 且 `a2a.card.skills` 为空或未配置
- **THEN** AgentCard.skills SHALL 包含至少一项内置占位 skill（id 固定为 `hcm-agent`，name 固定为 `"HCM Agent"`，tags 至少包含 `"hcm"`、`"agent"`）
- **AND** AgentCard JSON 响应 SHALL 通过 A2A v0.2.2 规范校验（skills 字段非空）

#### Scenario: 部分配置使用默认值

- **WHEN** `a2a.card.name` 未配置或为空字符串
- **THEN** AgentCard.name SHALL 使用内置默认值 `"HCM Agent"`
- **AND** 其它已配置字段 SHALL 保持配置原值

### Requirement: 基于 contextId 的连续对话

`agent-server` A2A Server SHALL 将 A2A `message.contextId` 作为 Runner 的 `threadId` 注入，同一 `contextId` 在同一 agent-server 进程或同一 session 后端下 SHALL 命中同一 session/thread，实现多轮连续对话上下文承接。

#### Scenario: 同一 contextId 连续多轮对话承接上下文

- **GIVEN** A2A 启用且 `aguiSessionSvc` 已配置 MySQL 后端
- **WHEN** 客户端先后发送两次 `message/stream` 请求，body 中 `params.message.contextId` 为同一字符串 `ctx-demo-1`
- **THEN** 第二轮请求处理时 Runner SHALL 能读取到第一轮的历史消息
- **AND** 两轮请求 SHALL 落在同一 `threadId`（值等于 `ctx-demo-1`）上

#### Scenario: 不同 contextId 隔离会话

- **WHEN** 两次 `message/stream` 请求使用不同的 `contextId`（如 `ctx-A` 与 `ctx-B`）
- **THEN** 两次请求 SHALL 落在不同的 `threadId` 上
- **AND** 两次请求的对话历史 SHALL 彼此不可见

#### Scenario: 首轮请求未提供 contextId 时由协议层生成

- **WHEN** A2A 请求 body 中 `params.message.contextId` 缺失
- **THEN** A2A Server SHALL 由底层 `trpc-a2a-go` 协议层自动生成符合 `ctx-<uuid>` 格式的新 `contextId`
- **AND** 该 `contextId` SHALL 在 A2A 响应事件流中返回给调用方，供后续轮次复用

### Requirement: A2A 链路中间件

`agent-server` A2A Handler SHALL 套用以下中间件链（从外到内）：

1. `mcpCallerOriginMiddleware`：校验 `X-Bkhcm-Caller-Source` header
2. `agentAuthMiddleware`：复用 AG-UI 现有 IAM 鉴权（`AgentAssistant.Find`）
3. `bkapiContextMiddleware`：复用 AG-UI 现有 BK 用户信息注入（读取 `X-Bkapi-User-Name` 写入 context）
4. `readinessMiddleware`：复用 AG-UI 现有就绪门控（skill/prompt 初始同步完成前返回 503）

中间件 SHALL 按上述顺序包装 A2A Handler，A2A Handler SHALL 在 readinessMiddleware 之内。

#### Scenario: 鉴权失败拦截

- **WHEN** 请求带有效 `X-Bkapi-User-Name` 但调用方在 IAM 中不具备 `AgentAssistant.Find` 权限
- **THEN** A2A 端点 SHALL 返回 HTTP 403，且 A2A Handler SHALL NOT 被执行

#### Scenario: 服务未就绪拦截

- **WHEN** agent-server 的 skill/prompt 初始同步尚未完成（`Readiness.IsReady()` 为 false）
- **THEN** A2A 端点 SHALL 返回 HTTP 503 + `UnHealthy` code
- **AND** AgentCard 端点同样受此中间件保护

#### Scenario: 来源校验

- **GIVEN** `a2a.enforceCallerOrigin: true`
- **WHEN** 请求未携带 `X-Bkhcm-Caller-Source: api-server` header
- **THEN** A2A 端点 SHALL 返回 HTTP 403 并 SHALL NOT 进入后续中间件

#### Scenario: 来源校验未强制时仅告警

- **GIVEN** `a2a.enforceCallerOrigin: false`（默认）
- **WHEN** 请求未携带 `X-Bkhcm-Caller-Source: api-server` header
- **THEN** A2A 端点 SHALL 正常处理请求
- **AND** agent-server SHALL 记录一条 warn 级别日志，包含 `X-Forwarded-For` 或 `RemoteAddr`、`X-Bkapi-Request-Id`

### Requirement: 与 AG-UI 链路隔离

A2A 协议入口 SHALL 与现有 AG-UI 链路完全独立：

- AG-UI 现有端点（`/api/v1/agent/agui`、`/api/v1/agent/cancel`、`/api/v1/agent/history`）、中间件链（`sessionCodeMiddleware`、`agentAuthMiddleware`、`bkapiContextMiddleware`、`readinessMiddleware` 的现有包装顺序）、`session_code → threadId` 解析逻辑 SHALL NOT 被本变更修改。
- AG-UI 默认 MCP ToolSet 类型（`type: "bkaidev"`）的代码分支 SHALL NOT 被本变更修改。
- A2A 启用与否 SHALL NOT 影响 AG-UI 的任何运行时行为或性能特征。

#### Scenario: 启用 A2A 不破坏 AG-UI 链路

- **GIVEN** 既有 AG-UI 端到端测试套件 + `a2a.enable: true`
- **WHEN** 启动 agent-server 并跑完 AG-UI 端到端用例
- **THEN** AG-UI 用例 SHALL 全部通过（无回归）
- **AND** AG-UI 请求 SHALL 不会路由进 A2A Handler

#### Scenario: 关闭 A2A 时 AG-UI 行为完全不变

- **GIVEN** `a2a.enable: false`（默认值）
- **WHEN** 启动 agent-server
- **THEN** agent-server SHALL 以与本变更合入前完全一致的方式响应所有 AG-UI 请求
- **AND** agent-server 启动日志中 SHALL NOT 包含 A2A 相关挂载日志
