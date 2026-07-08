## T-01: 新增常量

- [x] 1.1 在 `pkg/criteria/constant/aiagent.go` 的 `prompt-key` 常量块中新增 `ResourceQuerySystemPromptKey = "resource_query_system_prompt"`
- [x] 1.2 在 `pkg/criteria/constant/aiagent.go` 的 `prompt-key` 常量块中新增 `ResourceQueryInstructionKey = "resource_query_instruction_prompt"`

## T-02: AgentMCPToolSet 支持 Scenes 字段

- [x] 2.1 在 `pkg/cc/service.go` 的 `AgentMCPToolSet` 结构体中新增 `Scenes []string \`yaml:"scenes"\`` 字段
- [x] 2.2 在 `AgentMCPToolSet.Validate()` 中遍历 `Scenes`，每个值必须是 `"host_apply"` 或 `"resource_query"`，未知值返回 error；`Scenes` 为空时校验通过（向后兼容）

## T-03: MCPToolSet 新增 FilterByScene

- [x] 3.1 在 `cmd/agent-server/logics/tool/tool.go` 的 `MCPToolSet` 结构体中新增 `scenes [][]string` 字段（与 `TS []tool.ToolSet` 按 index 对齐）
- [x] 3.2 在 `BuildMCPToolSets`（或 `buildOneMCPToolSet`）中，将 `AgentMCPToolSet.Scenes` 填入 `MCPToolSet.scenes`，保证 index 与 `TS` 对齐
- [x] 3.3 实现 `(m *MCPToolSet) FilterByScene(scene string) *MCPToolSet`：遍历 `m.scenes`，scenes 为空或含 `scene` 的 toolset 保留，返回新实例，不修改原对象

## T-04: runtime.go 构建 resource_query 专属 toolProxy

- [x] 4.1 在 `cmd/agent-server/logics/runtime.go` 的 `toolLoadingSetup` 结构体中新增 `rqToolProxy *toolproxy.ToolProxy` 字段
- [x] 4.2 在 `buildToolLoading` 的 toolProxy 激活分支中，额外调用 `buildToolProxy(clientSet, tpCfg, mcpToolSets.FilterByScene("resource_query"), aidevGW)`，将结果赋给 `setup.rqToolProxy`（失败策略与主 toolProxy 一致）
- [x] 4.3 `Runtime` 结构体新增 `rqToolProxy *toolproxy.ToolProxy` 字段，在 `Close()` 中调用 `rqToolProxy.StopRefresh()`（与主 toolProxy 对称）
- [x] 4.4 `newAGUIRunner` 函数签名新增 `rqToolProxy *toolproxy.ToolProxy` 参数，透传给 `agent.BuildGraph`；`New()` 中调用侧传入 `toolSetup.rqToolProxy`

## T-05: BuildGraph 签名变更及 resource_query subgraph 构建

- [x] 5.1 `BuildGraph` 签名改为返回 `(*graph.Graph, []trpcagent.Agent, error)`，并新增参数 `rqToolProxy *toolproxy.ToolProxy`（位于 `toolProxy` 之后）与 `saver graph.CheckpointSaver`（用于封装子图 GraphAgent，见 5.4）
- [x] 5.2 新增 `resolveStaticPromptByKey(store *prompt.Store, systemKey, instructionKey string) string`：从 store 按 key 读取 system prompt 和 instruction 并合并，store 为 nil 或 key 不存在时降级为空字符串；原 `resolveStaticPrompt` 函数保持不变
- [x] 5.3 新增 `buildResourceQuerySubgraph(mdl, skillRepo, toolset, rqToolProxy, agentName, modelCfg, promptStore)`：构建子图（**仅 llm/hitl/tool 三节点，不含 fallback**），llm 节点使用 `resolveStaticPromptByKey(promptStore, ResourceQuerySystemPromptKey, ResourceQueryInstructionKey)`，llmOpts/toolOpts 使用 `rqToolProxy`（判断逻辑同 `genLLMNodeOptions`/`genToolNodeOptions`），路由复用 `makeRoutingFunc()` 但**无 tool_calls 时路由到 `graph.End`（finish），交还主图**（不进子图 fallback）；`hitl`→`llm`、`tool`→`llm` 回边，`SetFinishPoint("llm")`，返回 `(*graph.Graph, error)`
- [x] 5.4 将 `buildResourceQuerySubgraph` 返回的子图封装为 `*graphagent.GraphAgent`，名字固定为 `"resource_query"`（须与 5.5 的 nodeID 一致），并通过 `graphagent.WithCheckpointSaver(saver)` 配置 checkpoint saver（saver 来自 5.1 新增参数，子图 hitl nested resume 必需，缺失会导致退化失效），作为子 Agent 纳入 `BuildGraph` 的返回值 `[]trpcagent.Agent`
- [x] 5.5 在主图中调用 `stateGraph.AddSubgraphNode("resource_query", ...)` 注册子图节点（按需配置 input/output mapping 打通消息回流，见 5.9），主图 `AddConditionalEdges("intent_recognition", ...)` 路由 map 新增 `"resource_query": "resource_query"`，并新增静态边 `stateGraph.AddEdge("resource_query", "fallback")`（子图 finish 后进入主图 fallback 等待下一条消息）
- [x] 5.6 `makeIntentRoutingFunc` 新增分支：`IntentTypeResourceQuery → "resource_query"`
- [x] 5.7 `makePostFallbackRoutingFunc` 新增分支：`IntentTypeResourceQuery → "resource_query"`，并将主图 `AddConditionalEdges("fallback", ...)` 路由 map 新增 `"resource_query": "resource_query"`（会话保持：主图 fallback resume 后路由回子图节点）
- [x] 5.8 修复 `buildFallbackResumeDelta` 中 `clearingUnsupported` 条件：在 `!= enumor.IntentTypeHostApply` 基础上增加 `&& intentType != enumor.IntentTypeResourceQuery`（否则 resource_query resume 时 intent 被清空、会话保持失效）
- [x] 5.9 打通子图与主图消息回流：确认子图入参携带完整对话历史（默认 `include_contents=all`），并在 `AddSubgraphNode` 上按需配置 `WithSubgraphOutputMapper`，把子图 finish 时新增的 assistant 回复合并回主图 `StateKeyMessages`，使主图 fallback 的 `resolveFallbackLastResp` 能读到子图回复（开发时实测验证）

## T-06: NewGraphAgent 签名变更，注册子 Agent

- [x] 6.1 `NewGraphAgent` 签名新增参数 `subAgents []trpcagent.Agent`（`cmd/agent-server/logics/agent/agent_graph.go`）
- [x] 6.2 当 `subAgents` 非空时，通过 `graphagent.WithSubAgents(subAgents)` 传入 `graphagent.New`

## T-07: runtime.go 调用侧适配

- [x] 7.1 `newAGUIRunner` 中 graph 模式调用 `agent.BuildGraph` 时传入 `saver`（与 `NewGraphAgent` 使用同一 saver 实例），接收第三个返回值 `subAgents`，并传入 `agent.NewGraphAgent(... , subAgents)`

## T-08: 更新测试文件

- [x] 8.1 更新 `cmd/agent-server/logics/agent/graph_build_test.go`：适配 `BuildGraph` 新签名（接收第三个返回值 `subAgents`），现有测试用例继续通过

## T-09: 验证 nested interrupt resume（框架能力已确认）

> 框架 `v1.8.1` 已确认支持 nested interrupt + resume（`graph/subgraph_test.go` 的 `TestSubgraph_NestedInterruptResume` / `TestSubgraph_MultiLevelNestedInterruptResume`）。Q-002 已解决，无需退化为主图扁平扩展。

- [x] 9.1 参考 `cmd/agent-server/examples/graph/nested_interrupt/main.go`（含 interrupt 节点的子图示例），确认本项目 runner（`runtime.go`/aguirunner）现有的 interrupt 检测与 resume 逻辑对"父图 agent node 上抛的 nested interrupt"同样生效：resource_query 子图内 hitl 节点调用 `human_confirm` 产生 nested interrupt 后，下一条用户消息能 resume 回子图内部 hitl 节点继续执行，且不重跑主图 intent_recognition、不经过主图 fallback
