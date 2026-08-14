## ADDED Requirements

### Requirement: MCP JSON-RPC 2.0 报文解析

api-server 北向 MCP ingress SHALL 通过 `trpc-mcp-go.NewServer(...)` 构建 MCP Server 实例，复用 `trpc-mcp-go` 的 JSON-RPC 2.0 报文解析能力（含 request / notification 区分、按 `method` 路由的 dispatch table）。

api-server MUST NOT 自行实现 JSON-RPC 2.0 编解码（避免重复造轮子且与 `trpc-mcp-go` 行为漂移）。

#### Scenario: 标准 MCP 请求被正确解析

- **WHEN** MCP 客户端发送合法 JSON-RPC 2.0 请求（`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}`）
- **THEN** `trpc-mcp-go.Server` SHALL 解析报文并 dispatch 到 `initialize` 处理器
- **AND** api-server 自身代码 SHALL 不包含 JSON-RPC 解析逻辑

#### Scenario: 非法 JSON 返回标准错误

- **WHEN** MCP 客户端发送非法 JSON 报文
- **THEN** 响应 SHALL 包含 JSON-RPC 标准错误码 `-32700`（Parse error）
- **AND** 响应 SHALL 由 `trpc-mcp-go` 框架生成（api-server 不参与构造错误体）

### Requirement: initialize 方法本地应答

api-server SHALL 通过 `trpc-mcp-go.NewServer("hcm-agent", "<version>")` 注册 server name 与 version，框架 SHALL 在 `initialize` 请求时返回包含 `protocolVersion`、`serverInfo`、`capabilities.tools` 的标准响应。

`serverInfo.name` 字段 SHALL 为 `hcm-agent`；`capabilities.tools` 字段 SHALL 至少声明 `listChanged: false`（工具列表运行期固定）。

#### Scenario: initialize 返回合法 server info

- **WHEN** MCP 客户端发送 `initialize` 请求
- **THEN** 响应 `result.serverInfo.name` SHALL 等于 `hcm-agent`
- **AND** 响应 `result.capabilities.tools` SHALL 存在且为对象类型
- **AND** 响应 `result.protocolVersion` SHALL 为 MCP 当前支持的协议版本字符串

#### Scenario: notifications/initialized 不返回响应

- **WHEN** MCP 客户端发送 `notifications/initialized` 通知（无 `id` 字段）
- **THEN** api-server SHALL 不返回任何 HTTP 响应体（JSON-RPC notification 语义）
- **AND** 服务端会话 SHALL 标记为 ready，允许后续 `tools/list` 等请求

### Requirement: tools/list 注册聚合工具 send_message

api-server 北向 MCP ingress SHALL 通过 `trpc-mcp-go.Server.RegisterTool` 注册**唯一**工具 `send_message`，title 为 `HCM Agent`，inputSchema 包含字段 `text`（必填，string）、`contextId`（可选，string）、`bk_biz_id`（可选，integer）、`model_name`（可选，string）。

所有 `{mcp_server_name}` 路径下的 `tools/list` 请求 SHALL 返回相同结果：单元素列表 `[send_message]`。

`tools/list` 响应 MUST NOT 包含任何 HCM 业务工具（如 `list_cvms` 等） —— 这些工具仅在南向 MCP 上暴露。

#### Scenario: tools/list 返回 send_message

- **WHEN** MCP 客户端在 `initialize` 后发送 `tools/list`
- **THEN** 响应 `result.tools` SHALL 是长度为 1 的数组
- **AND** 该工具 `name` SHALL 等于 `send_message`
- **AND** 该工具 `inputSchema.properties` SHALL 包含 `text`、`contextId`、`bk_biz_id`、`model_name` 四个键
- **AND** `inputSchema.required` SHALL 包含 `text`

#### Scenario: 不同 mcp_server_name 返回相同工具列表

- **WHEN** 客户端分别向 `.../bk-hcm-devhk-tcloud-ziyan-cvm/mcp/` 与 `.../bk-hcm-finops/mcp/` 发送 `tools/list`
- **THEN** 两个响应的 `result.tools` SHALL 完全相同

### Requirement: ping 方法本地 pong

api-server SHALL 通过 `trpc-mcp-go` 内置支持响应 `ping` 请求，返回空对象 `result: {}`，用于客户端心跳检测。

#### Scenario: ping 返回 pong

- **WHEN** 客户端发送 `{"jsonrpc":"2.0","id":N,"method":"ping"}`
- **THEN** 响应 SHALL 为 `{"jsonrpc":"2.0","id":N,"result":{}}`
- **AND** 响应延迟 SHALL 不超过 100ms（本地应答）

### Requirement: notifications/cancelled 触发 A2A tasks/cancel

api-server SHALL 在 `trpc-mcp-go.Server` 上注册 `notifications/cancelled` 处理器，从 `params.requestId`（即客户端 `tools/call` 的 `progressToken`）查找对应的 A2A `taskId`（参见 `api-server-mcp-to-a2a-bridge` capability），并调用 `trpc-a2a-go.A2AClient.CancelTask(taskId)` 取消下游 agent 任务。

如果 `requestId` 在映射表中找不到（说明任务已结束或从未存在），SHALL 仅记录 warn 日志，不报错（符合 JSON-RPC notification 语义）。

#### Scenario: 取消正在进行的 tools/call

- **WHEN** 客户端先发起 `tools/call(send_message)` 拿到 `progressToken=P1`，关联到 agent 任务 `taskId=T1`
- **AND** 客户端发送 `notifications/cancelled` with `params.requestId=P1`
- **THEN** api-server SHALL 调用 `A2AClient.CancelTask(T1)` 取消 agent 侧任务
- **AND** 原 `tools/call` SHALL 返回 `CallToolResult{isError:true, content:[{text:"任务已取消"}]}`

#### Scenario: 取消未知 progressToken

- **WHEN** 客户端发送 `notifications/cancelled` with `params.requestId=unknown_id`
- **THEN** api-server SHALL 记录 `warn` 日志（含 `rid` 与未知 progressToken）
- **AND** SHALL 不返回错误响应（notification 语义）

### Requirement: prompts/list 与 resources/list 返回空

api-server 第一阶段 SHALL 对 `prompts/list` 与 `resources/list` 方法返回空列表（`result.prompts: []` / `result.resources: []`），由 `trpc-mcp-go` 内置默认行为提供。

api-server MUST NOT 在本变更中实现 prompts / resources 功能。

#### Scenario: prompts/list 返回空数组

- **WHEN** 客户端发送 `prompts/list`
- **THEN** 响应 SHALL 为 `result: {prompts: []}` 或 `result: {prompts: [], nextCursor: null}`

#### Scenario: resources/list 返回空数组

- **WHEN** 客户端发送 `resources/list`
- **THEN** 响应 SHALL 为 `result: {resources: []}` 或 `result: {resources: [], nextCursor: null}`

### Requirement: 标准错误码映射

api-server 在 MCP 方法处理失败时 SHALL 返回 JSON-RPC 标准错误码：

| 错误场景 | JSON-RPC code | 触发示例 |
|---|---|---|
| 报文非法 JSON | -32700 Parse error | 客户端发送畸形 JSON |
| 缺少 method / id / params | -32600 Invalid Request | JSON 合法但缺关键字段 |
| 调用未注册的 method | -32601 Method not found | `tools/call` 调用不存在的工具 |
| 参数 schema 校验失败 | -32602 Invalid params | `send_message` 调用缺 `text` 字段 |
| 上游异常（如 agent-server 不可达） | -32603 Internal error | A2A client 调用失败 |

错误响应体由 `trpc-mcp-go` 框架按 JSON-RPC 2.0 规范生成，api-server 仅在 tool handler 中通过返回 `error` 触发对应错误码。

#### Scenario: 调用未知工具返回 -32601

- **WHEN** 客户端发送 `tools/call(unknown_tool)`
- **THEN** 响应 `error.code` SHALL 等于 `-32601`
- **AND** `error.message` SHALL 包含 `unknown_tool`

#### Scenario: send_message 缺 text 返回 -32602

- **WHEN** 客户端发送 `tools/call(send_message, arguments={"contextId":"ctx-1"})`（缺 `text`）
- **THEN** 响应 `error.code` SHALL 等于 `-32602`
- **AND** `error.message` SHALL 提示 `text is required` 或同等含义
