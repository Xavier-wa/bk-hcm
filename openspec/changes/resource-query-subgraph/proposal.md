## Why

当前 agent-server 图架构仅支持 `host_apply` 意图的 ReAct 工作流，对于 `resource_query` 意图只返回"功能建设中"的固定提示，无法实际处理用户的资源查询请求。本期在现有主图基础上引入 resource_query subgraph 节点，补全查询能力，并通过场景标签机制实现不同意图对应的 MCP 工具集隔离。

## What Changes

- **新增** `resource_query` subgraph：仅含 `llm / hitl / tool` 三节点（**不自带 fallback**），llm 无 tool_calls 时 finish 交还主图、由主图 fallback 投递回复并 interrupt；hitl 调用 `human_confirm` 时在子图内 nested interrupt
- **修改** `BuildGraph` 函数签名：新增 `rqToolProxy` 与 `saver` 参数，额外返回 `[]trpcagent.Agent`（子 Agent 列表），并在主图中通过 `AddSubgraphNode` 注册 resource_query 节点；子图 GraphAgent 在 `BuildGraph` 内用 saver 封装（`WithCheckpointSaver`），名字固定为 `"resource_query"`
- **修改** `NewGraphAgent` 函数签名：新增 `subAgents []trpcagent.Agent` 参数，透传给 `graphagent.WithSubAgents`
- **修改** `makeIntentRoutingFunc`：新增 `IntentTypeResourceQuery` → `"resource_query"` 路由分支；主图新增静态边 `resource_query → fallback`（子图 finish 后由主图 fallback 投递回复并 interrupt 等待下一条消息）
- **修改** `makePostFallbackRoutingFunc`：新增 `IntentTypeResourceQuery → "resource_query"`，主图 fallback 路由 map 新增 `"resource_query"`（会话保持，复用主图 fallback 单图 resume 机制）
- **修改** `buildFallbackResumeDelta`：`clearingUnsupported` 条件排除 `resource_query`，避免 resume 时误清 intent
- **新增** `ResourceQuerySystemPromptKey` 常量（`resource_query_system_prompt`）
- **新增** `AgentMCPToolSet.Scenes []string` 字段及枚举校验，支持 `host_apply` / `resource_query` 两种场景标签
- **新增** `MCPToolSet.FilterByScene(scene string) *MCPToolSet` 方法，按场景过滤工具集
- **修改** `runtime.go` 中 `BuildGraph` / `NewGraphAgent` 调用侧，适配新签名

## Capabilities

### New Capabilities

- `resource-query-subgraph`：resource_query 场景的子图构建、路由接入与场景工具隔离的完整能力，包含子图节点结构、路由规则变更、prompt key 扩展、MCPToolSet scene 过滤

### Modified Capabilities

（无独立的已有 spec 需要打 delta，本期为纯新增能力）

## Impact

**受影响代码**：
- `pkg/criteria/constant/aiagent.go` — 新增常量
- `pkg/cc/service.go` — `AgentMCPToolSet` 新增字段及 `Validate` 修改
- `cmd/agent-server/logics/tool/tool.go` — `MCPToolSet` 新增 `FilterByScene`
- `cmd/agent-server/logics/agent/graph_build.go` — 签名变更、新增 subgraph 构建函数、路由函数修改
- `cmd/agent-server/logics/agent/agent_graph.go` — `NewGraphAgent` 签名变更
- `cmd/agent-server/logics/runtime.go` — 调用侧适配

**外部依赖**：
- `trpc-agent-go` 框架：需使用 `AddSubgraphNode` + `graphagent.WithSubAgents` API，须确认框架版本支持 nested interrupt propagation

**兼容性**：
- `AgentMCPToolSet.Scenes` 为空时语义为"适用所有场景"，向后兼容现有配置
- `host_apply` 现有行为不变（`FilterByScene("host_apply")` 对空 scenes 的 toolset 返回全部）
