# resource-query-subgraph

## ADDED Requirements

### Requirement: resource_query 意图路由到独立 subgraph 节点

当 intent_recognition 节点识别到用户意图为 `resource_query` 时，主图的条件边 SHALL 将执行流路由到 `resource_query` subgraph 节点，而非路由到 fallback 节点。

#### Scenario: resource_query 意图路由到 subgraph

- **WHEN** intent_recognition 节点运行，`StateKeyIntent` 被写入 `"resource_query"`
- **THEN** 条件边路由到 `resource_query` subgraph 节点，不路由到 `fallback`

#### Scenario: host_apply 意图路由不受影响

- **WHEN** intent_recognition 节点运行，`StateKeyIntent` 被写入 `"host_apply"`
- **THEN** 条件边路由到 `llm` 节点（host_apply 行为与变更前完全一致）

#### Scenario: 其他/未知意图仍路由到 fallback

- **WHEN** intent_recognition 节点运行，`StateKeyIntent` 被写入 `"chat"` 或其他未支持意图
- **THEN** 条件边路由到 `fallback` 节点

---

### Requirement: resource_query subgraph 实现 ReAct 循环并复用主图 fallback

resource_query subgraph 内部 SHALL 仅包含 llm、hitl、tool 三个节点（**不自带 fallback**），实现 ReAct 工作流循环；llm 无 tool_calls 时 SHALL finish 交还主图，由主图 fallback 节点投递回复并 interrupt 等待下一条消息。

#### Scenario: LLM 决定调用普通工具时路由到 tool 节点

- **WHEN** resource_query subgraph 中 llm 节点响应包含非 `human_confirm` 的 tool_calls
- **THEN** 条件边路由到 tool 节点执行工具调用，tool 节点执行完毕后路由回 llm

#### Scenario: LLM 决定调用 human_confirm 时路由到 hitl 节点（子图内 nested interrupt）

- **WHEN** resource_query subgraph 中 llm 节点响应的 tool_calls 仅包含 `human_confirm`
- **THEN** 条件边路由到 hitl 节点，hitl 节点在子图内发起 nested interrupt 等待人工确认，恢复后路由回 llm

#### Scenario: LLM 无工具调用时子图 finish 交还主图 fallback

- **WHEN** resource_query subgraph 中 llm 节点响应不包含 tool_calls
- **THEN** 子图 finish，控制权经 `resource_query → fallback` 边交还主图 fallback 节点，由主图 fallback 投递回复并 interrupt 等待用户下一条消息

---

### Requirement: 主图 fallback 中断后 resource_query session 保持

当 resource_query 子图 finish 后进入主图 fallback 节点并 interrupt，用户发送下一条消息时，主图的 post-fallback 条件边 SHALL 按 `StateKeyIntent` 路由回 `"resource_query"` 子图节点，不重新走 intent_recognition；`buildFallbackResumeDelta` SHALL 不清空 resource_query intent。

#### Scenario: resource_query 意图下主图 fallback 恢复路由回子图节点

- **WHEN** `StateKeyIntent` 为 `"resource_query"`，主图 fallback 节点 interrupt 后用户发送新消息
- **THEN** post-fallback 条件边路由到 `"resource_query"` 子图节点，跳过 intent_recognition，`StateKeyIntent` 保持为 `"resource_query"`（不被清空）

#### Scenario: 非 resource_query/host_apply 意图下主图 fallback 恢复重走 intent_recognition

- **WHEN** `StateKeyIntent` 为 `"chat"` 或其他意图，主图 fallback interrupt 后用户发送新消息
- **THEN** post-fallback 条件边路由到 `intent_recognition`，`StateKeyIntent` 被清空

#### Scenario: 任务中人工确认走子图内 hitl nested interrupt

- **WHEN** resource_query 子图 llm 调用 `human_confirm`，子图 hitl 节点 nested interrupt 后用户发送确认消息
- **THEN** runner resume 父图 checkpoint，框架经 `StateKeySubgraphInterrupt` 转发子图 checkpoint + ResumeMap，执行从子图 hitl 节点继续，不经过主图 fallback / intent_recognition

---

### Requirement: resource_query subgraph 使用独立 prompt key

resource_query subgraph 的 llm 节点 SHALL 使用 `resource_query_system_prompt` 与 `resource_query_instruction_prompt` 两个 prompt key，与主图 host_apply 的 `system_prompt` / `instruction_prompt` 相互独立。两个 key 均为可选：未配置时各自降级为空字符串，节点正常运行。

#### Scenario: resource_query_system_prompt 已配置时使用对应 prompt

- **WHEN** promptStore 中存在 `resource_query_system_prompt` key 的内容
- **THEN** resource_query subgraph 的 llm 节点使用该内容作为系统提示词

#### Scenario: resource_query_system_prompt 未配置时降级为空 prompt

- **WHEN** promptStore 中不存在 `resource_query_system_prompt` key，或内容为空
- **THEN** resource_query subgraph 的 llm 节点使用空 system prompt，节点正常运行不报错

#### Scenario: resource_query_instruction_prompt 已配置时追加到系统提示词

- **WHEN** promptStore 中存在 `resource_query_instruction_prompt` key 的内容
- **THEN** resource_query subgraph 的 llm 节点将该内容拼接在 system prompt 之后，作为完整指令传入 LLM

#### Scenario: resource_query_instruction_prompt 未配置时不影响 prompt 构建

- **WHEN** promptStore 中不存在 `resource_query_instruction_prompt` key，或内容为空
- **THEN** resource_query subgraph 的 llm 节点仅使用 system prompt（或空字符串），节点正常运行不报错

---

### Requirement: resource_query subgraph 仅加载对应场景的 MCP toolset

resource_query subgraph 的 llm 节点和 tool 节点 SHALL 仅使用 `scenes` 字段包含 `"resource_query"` 或 `scenes` 为空（通用）的 MCP toolset，不加载仅标注 `"host_apply"` 的 toolset。

#### Scenario: resource_query subgraph 不加载仅 host_apply 场景的 toolset

- **WHEN** MCPToolSet 配置中某 toolset 的 `scenes` 仅包含 `"host_apply"`，resource_query subgraph 加载 toolset
- **THEN** 该 toolset 不被加载到 resource_query subgraph 的 llm/tool 节点

#### Scenario: 通用 toolset（scenes 为空）对所有场景均可用

- **WHEN** MCPToolSet 配置中某 toolset 的 `scenes` 为空
- **THEN** 该 toolset 对 host_apply 和 resource_query 场景均可用

#### Scenario: 明确标注 resource_query 的 toolset 被正确加载

- **WHEN** MCPToolSet 配置中某 toolset 的 `scenes` 包含 `"resource_query"`
- **THEN** 该 toolset 被加载到 resource_query subgraph 的 llm/tool 节点

---

### Requirement: AgentMCPToolSet 支持 scenes 字段配置与枚举校验

`AgentMCPToolSet` 结构体 SHALL 新增 `Scenes []string` 字段（yaml tag: `scenes`），`Validate()` 方法 SHALL 校验 `Scenes` 中每个值必须是已知场景枚举（`host_apply` 或 `resource_query`），未知值 SHALL 导致校验失败并报错。

#### Scenario: 合法 scenes 值通过校验

- **WHEN** `AgentMCPToolSet.Scenes` 为 `["host_apply", "resource_query"]` 或其子集或为空
- **THEN** `Validate()` 返回 nil

#### Scenario: 未知 scenes 值校验失败

- **WHEN** `AgentMCPToolSet.Scenes` 包含 `"unknown_scene"` 等未知值
- **THEN** `Validate()` 返回非 nil 错误，服务启动失败

---

### Requirement: MCPToolSet 提供 FilterByScene 方法

`MCPToolSet` SHALL 提供 `FilterByScene(scene string) *MCPToolSet` 方法，返回仅包含适用于指定 scene 的 toolset 的新实例，不修改原始对象。

#### Scenario: FilterByScene 过滤出匹配场景的 toolset

- **WHEN** 调用 `toolset.FilterByScene("resource_query")`
- **THEN** 返回新的 `*MCPToolSet`，其 `TS` 仅包含 `scenes` 为空或包含 `"resource_query"` 的 toolset

#### Scenario: FilterByScene 不修改原始对象

- **WHEN** 调用 `toolset.FilterByScene("resource_query")` 后
- **THEN** 原始 `MCPToolSet` 的 `TS` 内容不变

---

### Requirement: BuildGraph 签名变更以支持 subgraph 注册

`BuildGraph` 函数 SHALL 新增 `saver graph.CheckpointSaver` 参数并额外返回 `[]trpcagent.Agent`（子 Agent 列表），其中包含 resource_query subgraph 封装成的 `*graphagent.GraphAgent`（名字为 `"resource_query"`，并通过 `graphagent.WithCheckpointSaver(saver)` 配置 saver），供调用方传入 `NewGraphAgent`。

#### Scenario: BuildGraph 成功构建主图及子 Agent 列表

- **WHEN** `BuildGraph` 被调用且所有依赖正常
- **THEN** 返回编译后的主图 `*graph.Graph`、子 Agent 列表 `[]trpcagent.Agent`（至少包含名为 `"resource_query"`、已配置 checkpoint saver 的 subgraph GraphAgent）、nil error

---

### Requirement: NewGraphAgent 支持接收并注册子 Agent

`NewGraphAgent` 函数 SHALL 接收 `subAgents []trpcagent.Agent` 参数，并在构建 `graphagent.GraphAgent` 时通过 `graphagent.WithSubAgents(subAgents)` 注册，使框架能找到 subgraph 的执行上下文。

#### Scenario: 传入非空 subAgents 时正确注册

- **WHEN** `NewGraphAgent` 被调用，`subAgents` 包含 resource_query subgraph 的 GraphAgent
- **THEN** 返回的 `agent.Agent` 内部已注册该 subAgent，subgraph 节点可被框架路由执行
