# api-server-mcp-to-a2a-bridge Specification

## Purpose
TBD - created by archiving change api-server-mcp-a2a-bridge. Update Purpose after archive.
## Requirements
### Requirement: send_message tool handler 触发 A2A 流式调用

api-server SHALL 实现 `send_message` 工具的 handler，在收到 MCP `tools/call(send_message, arguments)` 时：

1. 从 `arguments.text` 读取用户输入文本（必填）；
2. 从 `arguments.contextId` 读取会话 ID（可选，无则由 agent-server 生成）；
3. 从 `arguments.bk_biz_id` / `arguments.model_name` 读取业务参数（可选，按需透传）；
4. 调用 `trpc-a2a-go.A2AClient.StreamMessage(ctx, A2AMessage{...})` 发起流式调用；
5. 持续读取 A2A `TaskStatusUpdateEvent` / `TaskArtifactUpdateEvent`，转换为 MCP `notifications/progress` 通过 `trpc-mcp-go.Server.SendNotification` 推送给客户端；
6. 在 A2A 任务 `final=true` 时，将累积的 artifact 文本作为 `CallToolResult{content:[{type:"text", text:"..."}]}` 返回。

handler MUST NOT 调用阻塞型 A2A 接口（如 `SendMessage` 非流式版本），必须使用流式 API。

#### Scenario: 文本对话端到端打通

- **WHEN** OpenClaw 发起 `tools/call(send_message, arguments={"text":"列出所有CVM"})`
- **THEN** api-server SHALL 立即通过 `A2AClient.StreamMessage` 向 agent-server 发起流式调用
- **AND** SHALL 在 A2A `working` 状态推送 `notifications/progress` 给客户端
- **AND** SHALL 在 A2A `completed` 状态返回 `CallToolResult` 含累积的 artifact 文本

#### Scenario: 缺 text 参数校验失败

- **WHEN** 客户端发起 `tools/call(send_message, arguments={"contextId":"ctx-1"})`
- **THEN** handler SHALL 在调用 `A2AClient.StreamMessage` 之前返回 `-32602 Invalid params`
- **AND** SHALL 不发起任何 A2A 调用

### Requirement: contextId 透传与会话连续性

handler SHALL 严格透传 `arguments.contextId` 到 A2A `Message.contextId`，agent-server 据此关联会话历史。

当 `arguments.contextId` 为空时，handler SHALL 让 `A2AClient.StreamMessage` 自行生成新的 `contextId`，并 SHALL 通过 MCP `notifications/progress` 或 `CallToolResult.content` 中的元数据回传给客户端，便于客户端在下一次调用中继续传入。

handler MUST NOT 在 api-server 侧自行维护 `contextId` ↔ 用户 / 会话 的映射表（owner = agent-server）。

#### Scenario: 首次调用生成 contextId 并回传

- **WHEN** 客户端首次调用 `tools/call(send_message, arguments={"text":"你好"})`（未提供 contextId）
- **THEN** A2A 响应 SHALL 携带新 `contextId=C1`
- **AND** `CallToolResult` 或 `notifications/progress` SHALL 把 `C1` 回传给客户端

#### Scenario: 复用 contextId 维持会话

- **WHEN** 客户端在第二次调用传入 `arguments.contextId=C1`
- **THEN** api-server SHALL 在 A2A 请求中 `Message.contextId` 字段填入 `C1`
- **AND** agent-server SHALL 能基于 `C1` 关联到前一次会话历史

### Requirement: progressToken ↔ taskId 映射

handler SHALL 维护进程内并发安全的映射表 `map[progressToken]taskId`：

- 在 A2A `StreamMessage` 返回首个 `taskId` 时，SHALL 写入 `(progressToken, taskId)` 条目；
- 在 A2A 任务终态（`completed` / `failed` / `canceled`）或 `tools/call` 返回时，SHALL 删除该条目；
- 映射表 SHALL 设置 max size（默认 10000）与 entry TTL（默认 30 分钟）兜底防泄漏。

`progressToken` 来源于客户端 `tools/call` 请求的 `params._meta.progressToken`，由 `trpc-mcp-go` 框架解析。

#### Scenario: 映射创建与删除

- **WHEN** 客户端发起 `tools/call(send_message)` 含 `_meta.progressToken=P1`，A2A 返回 `taskId=T1`
- **THEN** api-server SHALL 在映射表中存入 `P1→T1`
- **WHEN** A2A 任务进入 `completed` 状态
- **THEN** api-server SHALL 从映射表中删除 `P1→T1` 条目

#### Scenario: 取消请求按映射查 taskId

- **WHEN** 客户端发送 `notifications/cancelled with params.requestId=P1`
- **THEN** api-server SHALL 从映射表查到 `T1` 并调用 `A2AClient.CancelTask(T1)`

#### Scenario: 映射表溢出兜底

- **WHEN** 映射表条目数超过 `max size`
- **THEN** api-server SHALL 按 LRU 或 FIFO 淘汰最旧条目并记录 warn 日志
- **AND** SHALL 不阻塞新请求处理

### Requirement: A2A 流式事件转换为 MCP notifications/progress

handler 在 A2A 流式调用过程中，SHALL 将事件转换为 MCP `notifications/progress`：

| A2A 事件 | MCP notification |
|---|---|
| `TaskStatusUpdateEvent{state: working, message}` | `notifications/progress{ progressToken, progress, message }` |
| `TaskArtifactUpdateEvent{artifact}` | `notifications/progress{ progressToken, message: <artifact-text-chunk> }` |
| `TaskStatusUpdateEvent{state: completed, final: true}` | 终止流，返回 `CallToolResult` |
| `TaskStatusUpdateEvent{state: failed, final: true}` | 返回 `CallToolResult{isError:true, content:[{text: errMsg}]}` |
| `TaskStatusUpdateEvent{state: canceled, final: true}` | 返回 `CallToolResult{isError:true, content:[{text:"任务已取消"}]}` |

handler SHALL 通过 `trpc-mcp-go.Server.SendNotification` 发送 progress 通知（依赖 streamable HTTP 的服务端推送能力）。

#### Scenario: 流式 artifact 增量推送

- **WHEN** A2A agent 通过 5 个 `TaskArtifactUpdateEvent` 增量返回 artifact 文本
- **THEN** api-server SHALL 向客户端推送 5 次 `notifications/progress`
- **AND** 每次 progress 的 `message` 字段 SHALL 包含对应 artifact 增量文本

#### Scenario: 失败状态返回 isError

- **WHEN** A2A agent 返回 `TaskStatusUpdateEvent{state: failed, final: true, message: "模型调用超时"}`
- **THEN** `tools/call` 最终响应 SHALL 为 `CallToolResult{isError: true, content: [{type:"text", text: "模型调用超时"}]}`

### Requirement: agent-server 实例获取与 A2A 客户端复用

api-server SHALL 在启动阶段初始化**单例** A2A 调用器，agent-server 实例列表 SHALL 通过 etcd 服务发现 `discovery.NewAPIDiscovery(cc.AgentServerName, dis)` 获取（与 `cmd/web-server/service/proxy.go` 同源），不在 yaml 中手填 URL。

每次 `tools/call(send_message)` 时，api-server SHALL 调用 `APIDiscovery.GetServers()` 拿到当前可用 agent-server 实例 URL 列表，按内置轮询策略选取一个实例 `<host>:<port>`，并拼接固定路径段 `constant.A2ABasePathDefault + constant.A2AJSONRPCSubPath`（即 `/api/v1/agent/a2a`）构造完整 A2A JSON-RPC URL。

`trpc-a2a-go.A2AClient` 的 HTTP 传输层 SHALL 启用 keep-alive，连接池大小可配置（默认 100，对应 `ApiServerSetting.MCP.Bridge.MaxIdleConnsPerHost`）。

`A2AClient` 调用超时 SHALL 可配置（默认 connect=5s，read=300s 适配长会话；对应 `ApiServerSetting.MCP.Bridge.ConnectTimeout` / `ReadTimeout`）。

api-server **MUST NOT** 在 yaml 中要求手填 `agentServerA2AURL` 字段（已废弃）。

#### Scenario: 实例通过 etcd 服务发现获取

- **WHEN** `tools/call(send_message)` 被触发
- **THEN** handler SHALL 调用 `APIDiscovery.GetServers()` 获取 agent-server 实例列表
- **AND** SHALL 不从 yaml 读取硬编码的 agent-server URL

#### Scenario: 单例 A2A client 共享 HTTP 传输

- **WHEN** api-server 启动
- **THEN** SHALL 仅创建一个 HTTP transport / client 实例（含 keep-alive 连接池）
- **AND** 所有 `send_message` handler 调用 SHALL 共享同一 HTTP transport

#### Scenario: 长会话不超时

- **WHEN** A2A 任务持续 60 秒
- **THEN** `A2AClient.StreamMessage` 调用 SHALL 不因 read timeout 提前中断

#### Scenario: agent-server 扩容自动生效

- **WHEN** agent-server 实例扩容（新实例向 etcd 注册）
- **THEN** api-server 后续 `tools/call` 调用 SHALL 在轮询中包含新实例
- **AND** SHALL 不需要修改 api-server yaml 或重启 api-server

### Requirement: 桥接错误日志与 metrics

handler SHALL 在以下场景输出结构化日志，**必须**包含 `rid`（取自 ctx）、`bk_username`、`mcp_server_name`、`progressToken`、`taskId`（如已建立）：

- `Infof` —— `tools/call` 收到 / `tools/call` 完成 / A2A 任务创建 / A2A 任务终态
- `Warnf` —— A2A 流中可恢复异常（如某个 artifact 解析失败但继续）/ 映射表溢出
- `Errorf` —— A2A 客户端调用失败 / handler 中断退出

handler SHALL 通过项目现有 metrics 工具（如 `pkg/metrics`）暴露下列指标：

- `bk_hcm_api_server_mcp_tools_call_total{tool, mcp_server_name, status}` —— Counter
- `bk_hcm_api_server_mcp_tools_call_duration_seconds{tool, mcp_server_name}` —— Histogram
- `bk_hcm_api_server_mcp_bridge_active_tasks` —— Gauge（映射表当前条目数）

#### Scenario: 调用成功日志含 rid

- **WHEN** `tools/call(send_message)` 成功完成
- **THEN** 日志 SHALL 至少输出一条 `Infof` 含 `rid`、`bk_username`、`mcp_server_name`、`progressToken`、`taskId`

#### Scenario: A2A 失败日志输出 Errorf

- **WHEN** `A2AClient.StreamMessage` 因网络异常返回 error
- **THEN** handler SHALL 输出 `Errorf` 日志含 `err: %v, rid: %s`
- **AND** 不 panic，按 `Internal error` 返回 MCP 响应

