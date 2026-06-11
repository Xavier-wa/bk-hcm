## Context

### 现有架构

`agent-server` 使用 `trpc-agent-go` 图引擎实现 ReAct 工作流。当前主图拓扑如下：

```
START → intent_recognition → ConditionalEdge(by intent)
  ├─ host_apply → llm → ConditionalEdge(by tool_calls)
  │     ├─ human_confirm → hitl → llm
  │     ├─ other_tool_calls → tool → llm
  │     └─ no tool_calls → fallback (interrupt) → llm
  └─ 其他意图 → fallback (interrupt) → intent_recognition
```

- `BuildGraph(mdl, skillRepo, toolset, toolProxy, agentName, modelCfg, promptStore) (*graph.Graph, error)` — 构建并编译主图
- `NewGraphAgent(name, compiledGraph, saver) (agent.Agent, error)` — 封装为 GraphAgent
- `MCPToolSet.TS []tool.ToolSet` — 所有 MCP toolset 的平铺列表，无场景区分

`resource_query` 意图当前直接路由 `fallback`，返回"功能建设中"固定文本。

### 关键约束

- 框架通过 `sg.AddSubgraphNode(nodeID, opts...)` 注册嵌套子图；父 Agent 须通过 `graphagent.WithSubAgents([]agent.Agent{childGA})` 关联子 Agent，否则框架无法找到子图的执行上下文
- 子图 fallback 的 `graph.Interrupt` 会向上传播到 runner 层（nested interrupt），需确认框架版本支持

---

## Goals / Non-Goals

**Goals：**
- resource_query 意图能路由到独立 subgraph，执行完整 ReAct 循环
- subgraph 使用独立 prompt key（`resource_query_system_prompt`）
- MCPToolSet 支持 `scenes` 字段，实现不同意图工具集隔离
- host_apply 现有行为 100% 向后兼容

**Non-Goals：**
- resource_query 的 prompt 文案内容（开发阶段另行确定）
- resource_query 专属 MCP tool server 的搭建
- resource_query 会话结束后的状态清理逻辑

---

## Decisions

### D-1：使用框架嵌套图（AddSubgraphNode）而非主图扁平扩展

**选择**：框架嵌套图（`AddSubgraphNode` + `graphagent.WithSubAgents`）

**备选**：在主图中直接添加 resource_query 专属的 llm2 / tool2 / fallback2 节点，通过命名区分

**理由**：
- 封装性更好，子图状态隔离，不污染主图消息历史
- 框架原生支持，示例代码已有完整演示（`examples/graph/subgraph/main.go`）
- 未来新增其他意图的 subgraph 时，扩展模式一致

**代价**：`BuildGraph` 需返回子 Agent 列表，`NewGraphAgent` 需接收并透传，调用侧需适配

---

### D-2：BuildGraph 签名变更为返回子 Agent 列表，并新增 saver 参数

**变更前**：`BuildGraph(...) (*graph.Graph, error)`

**变更后**：`BuildGraph(..., rqToolProxy *toolproxy.ToolProxy, saver graph.CheckpointSaver) (*graph.Graph, []trpcagent.Agent, error)`

**理由**：
- 子图的 `graphagent.GraphAgent` 必须在 `graphagent.WithSubAgents` 时传入，而子图在 `BuildGraph` 内部构建，外部（`runtime.go`）无法独立获取。将子 Agent 列表作为返回值是最直接的耦合方式。
- **子图 GraphAgent 必须配置 `graphagent.WithCheckpointSaver(saver)`**，否则框架的 `latestInterruptedCheckpointID` 拿不到子图 checkpoint，nested resume 退化失效（见 D-6）。由于子图在 `BuildGraph` 内部封装为 GraphAgent，saver 必须作为 `BuildGraph` 参数传入。原本 saver 仅在 `NewGraphAgent`（`BuildGraph` 之后）使用，现需提前传入。

**备选**：`BuildGraph` 仅返回子图 `*graph.Graph`，由 `runtime.go`/`NewGraphAgent` 用 saver 统一封装并 `WithSubAgents` 注册 → saver 归属集中，但 `BuildGraph` 需暴露子图原始 Graph，调用侧职责变重。本次选择前者（saver 入参），与"BuildGraph 返回 []Agent"的方向一致。

**注意**：子图 GraphAgent 的 `name` 必须与主图 `AddSubgraphNode("resource_query", ...)` 的 nodeID 完全一致（框架按 `Info().Name` 匹配子 Agent），即子 GraphAgent 名字固定为 `"resource_query"`。

---

### D-3：MCPToolSet.Scenes 语义：空 = 通用

**`AgentMCPToolSet.Scenes []string` 字段语义**：
- 为空（`nil` 或 `[]`）→ 该 toolset 适用于**所有场景**（向后兼容）
- 非空 → 仅适用于列举的场景

**`FilterByScene(scene string) *MCPToolSet` 实现**：
- 遍历 `MCPToolSet.TS`，但因为 `tool.ToolSet` 是接口（没有直接携带 scenes 信息），需要在 `buildOneMCPToolSet` 时将 cfg 中的 scenes 信息附加到包装层

**实现方案**：
在 `BuildMCPToolSets` 时，为每个 toolset 额外记录其对应的 `Scenes`，不修改 `tool.ToolSet` 接口。在 `MCPToolSet` 上维护并行的 `scenes [][]string` 与 `TS []tool.ToolSet`。`FilterByScene` 按 index 匹配，返回新的 `MCPToolSet`（仅含符合条件的 toolset + 对应 scenes）。

---

### D-4：resource_query subgraph 与主图共享 modelCallbacks，使用独立 prompt key

resource_query subgraph 的 llm 节点复用与主图 `host_apply` 完全相同的 `modelCb`（logger、历史工具结果过滤、prompt 注入、skill 注入、时间注入），但使用独立的 system prompt 和 instruction：
- system prompt key：`resource_query_system_prompt`（常量 `ResourceQuerySystemPromptKey`）
- instruction key：`resource_query_instruction_prompt`（常量 `ResourceQueryInstructionKey`）
- 实现方式：新增 `resolveStaticPromptByKey(store *prompt.Store, systemKey, instructionKey string) string`，原 `resolveStaticPrompt` 保持不变（方案 A，向后兼容）
- resource_query 不 fallback 到 `cc.AgentServer().Prompt`，未配置时降级为空 prompt，节点正常运行

---

### D-5：resource_query subgraph 使用独立的 toolProxy，由 FilterByScene 构建

**背景**：`toolProxy` 的工具范围由构建时传入的 `MCPToolSet` 决定（`NewToolProxy` 接受 `*MCPToolSet`，`Build()` 时从中加载工具建索引）。

**选择**：在 `runtime.go` 的 `buildToolLoading` 中额外构建第二个 `*toolproxy.ToolProxy` 实例（`rqToolProxy`），以 `mcpToolSets.FilterByScene("resource_query")` 作为工具来源。通过 `BuildGraph` 的新参数 `rqToolProxy *toolproxy.ToolProxy` 传入。

**语义**：
- 主图 `toolProxy`：保持现状，从完整 `mcpToolSets` 构建（向后兼容 host_apply）
- resource_query 子图 `rqToolProxy`：从 `FilterByScene("resource_query")` 构建，仅包含 scenes 为空或含 `"resource_query"` 的 toolset

**代价**：`buildToolLoading` 和 `buildToolProxy` 需适配双 toolProxy 构建；`BuildGraph` 签名新增 `rqToolProxy` 参数；`newAGUIRunner` 中调用侧适配。

**toolProxy 模式下 FilterByScene 语义**：FilterByScene 决定哪些 toolset 能被 rqToolProxy 搜索到，非 proxy 模式下同样生效（直接过滤 `toolset.TS`）。

---

### D-6：子图复用主图 fallback，"等待下一条消息"走主图、"人工确认"走子图内 hitl（混合机制）

**关键判定**：resource_query **不自带 fallback 节点**，而是复用主图的 `fallback` 节点来实现"投递回复 + interrupt 等待下一条消息"。子图仅含 `llm / hitl / tool` 三节点，`llm` 无 tool_calls 时直接 finish（`SetFinishPoint`），把控制权交还主图。

**子图拓扑**：

```
子图 (resource_query):
  START → llm ─conditional(by tool_calls)─
                 ├ 仅 human_confirm → hitl → llm
                 ├ 其他工具         → tool → llm
                 └ 无 tool_calls    → END (finish，交还主图)
  finish point = llm
```

**主图拓扑变更**：

```
intent_recognition ─resource_query→ "resource_query"(子图节点) ─AddEdge→ fallback (interrupt)
fallback ─post-routing─ resource_query→ "resource_query"(子图节点)   ← 会话保持
```

**两种 interrupt 机制并存（混合）**：

| 场景 | 触发节点 | 机制 | resume 路径 |
|------|---------|------|------------|
| 等待用户下一条消息（llm 给出最终答复、无 tool_calls） | **主图** `fallback` | 单图内 interrupt（与 host_apply 同款） | 主图 fallback → `makePostFallbackRoutingFunc` → 路由回 `"resource_query"` 子图节点，子图带完整历史重新执行 |
| 任务中人工确认（llm 调用 `human_confirm`） | **子图内** `hitl` | 框架 nested interrupt（`hitl.makeHITLNode` 自身 `graph.Interrupt`） | runner resume 父图 checkpoint → 框架经 `StateKeySubgraphInterrupt` 转发子图 checkpoint+ResumeMap → 子图 hitl 节点续跑 |

> 即使 fallback 复用主图，**子图含 hitl 节点仍会产生 nested interrupt**（`hitl/node.go` 第 68 行调 `graph.Interrupt`）。因此 nested interrupt 能力与子图 checkpoint saver（见 D-2）**仍不可省**。

**主图路由需恢复的改动**（与早期"纯 nested"判断相反，本方案需要这些）：
- `makeIntentRoutingFunc` 新增 `IntentTypeResourceQuery → "resource_query"`。
- 主图新增静态边 `AddEdge("resource_query", "fallback")`（子图 finish 后进入主图 fallback 等待）。
- `makePostFallbackRoutingFunc` 新增 `IntentTypeResourceQuery → "resource_query"`，主图 `AddConditionalEdges("fallback", ...)` 路由 map 新增 `"resource_query": "resource_query"`。
- `buildFallbackResumeDelta` 的 `clearingUnsupported` 条件必须排除 `resource_query`（同 host_apply），否则 resume 时 intent 被清空 → 重走 intent_recognition，会话保持失效（无编译错误，仅运行时暴露）：

```go
// 变更后
clearingUnsupported := intentStr != "" &&
    intentType != enumor.IntentTypeHostApply &&
    intentType != enumor.IntentTypeResourceQuery
```

**子图消息回流（必须打通）**：主图 fallback 的 `resolveFallbackLastResp` / `buildFallbackResumeDelta` 读取的是**主图** `StateKeyMessages`。因此子图与主图之间的消息必须双向贯通：
- **入**：主图把完整对话历史传入子图（默认 `include_contents=all`，子图 llm 才能看到上下文）。
- **出**：子图 finish 时把新产生的 assistant 回复合并回主图 `StateKeyMessages`（按需配置 `WithSubgraphOutputMapper`，否则主图 fallback 看不到子图回复、历史断裂）。
- llm 节点本身在生成时已流式 emit assistant 文本事件（AGUI），主图 fallback 的 `shouldEmitFallbackResponse` 在消息尾为 assistant 时返回 false，不会重复 emit。

**前置条件**：子图 GraphAgent 必须配置 checkpoint saver（见 D-2），用于 hitl nested resume。

**runner 层验证点**：① 主图 fallback 的单图 resume（host_apply 已在用，复用即可）；② 子图 hitl 的 nested resume——框架示例 `examples/graph/nested_interrupt/main.go` 按此模式 resume，预期可复用。开发时需实测子图消息回流（出/入 mapping）是否符合预期。

---

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| ~~框架 nested interrupt 稳定性未验证~~（已验证，仍用于 hitl） | 框架 `v1.8.1` 已支持 nested interrupt + resume，源码 `graph/state_graph.go`（`setSubgraphInterruptState`/`applySubgraphResumeForAgentNode`）+ 测试 `graph/subgraph_test.go`（三层嵌套通过）。子图 hitl 仍走 nested 机制（见 D-6），仍需在 runner 层确认 hitl nested resume 触发链路 |
| 子图 GraphAgent 漏配 checkpoint saver | hitl nested resume 会退化失效（`latestInterruptedCheckpointID` 取不到子 checkpoint），无编译错误；通过 D-2 强制 saver 入参 + 封装处统一 `WithCheckpointSaver` 规避 |
| 子图消息未正确回流主图 | 主图 fallback 读不到子图 assistant 回复 / 历史断裂；开发时实测 subgraph 入/出 mapping（默认 include_contents=all 入、`WithSubgraphOutputMapper` 出），见 D-6 |
| `buildFallbackResumeDelta` 的 `clearingUnsupported` 条件漏改 | resource_query resume 时 intent 被清空、会话保持失效；在 D-6 明确列出变更后代码，review 重点检查此处 |
| `BuildGraph` 签名变更影响调用侧 | 仅 `runtime.go` 一处调用，改动范围可控；变更后需同步更新测试文件 `graph_build_test.go` |
| `FilterByScene` 需要并行维护 TS 与 scenes 列表 | 结构简单（两个同步的切片），`BuildMCPToolSets` 内构建时保证 index 对齐 |
| `rqToolProxy` 独立构建增加启动时间 | 与主图 toolProxy 构建逻辑复用 `buildToolProxy`，仅多一次 Build 调用；嵌入向量索引构建是瓶颈，但二者可串行接受 |
| resource_query prompt 未配置时节点空跑 | 降级为空 prompt，节点正常运行，不影响主流程 |

---

## Migration Plan

1. 配置文件：现有 `mcp.scenes` 字段缺失时默认为空（全场景），无需修改存量配置
2. 服务重启：`BuildGraph` 在服务启动时执行，resource_query subgraph 构建失败会导致整体启动失败（与 `host_apply` 主图编译行为一致）
3. 回滚：回滚本次代码变更后，重启服务即可恢复

---

## Open Questions

| ID | 问题 | 负责人 | 状态 |
|----|------|--------|------|
| Q-001 | `resource_query_system_prompt` 的具体文案内容 | pandafyang | 开发阶段确认 |
| Q-002 | 框架版本是否支持 nested interrupt（子图 hitl interrupt 向上传播到 runner 并 resume 回子图） | pandafyang | ✅ 已解决：`v1.8.1` 支持，见 `graph/subgraph_test.go`；剩余仅 runner 层 hitl nested resume 触发链路 + 子图消息回流待开发时实测 |
