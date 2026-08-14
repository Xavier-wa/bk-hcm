## Why

当前 GraphAgent 中所有用户消息都直接进入同一个 ReAct 流程（`START → llm → ...`），没有意图区分，导致不同业务场景（主机申领/资源查询/闲聊）的处理路径完全相同，无法为后续针对不同意图提供差异化工具集和 Prompt 做基础。本子需求为 Graph 添加意图识别节点，作为多意图路由架构的第一步。

## What Changes

- 新增 `intent_recognition` Graph 节点：在 `START` 与现有 `llm` 节点之间插入，通过独立的 LLM 调用对用户最新消息进行三分类（`host_apply` / `resource_query` / `chat`）
- 扩展 Graph State：在 `pkg/criteria/constant/aiagent.go` 中新增 `StateKeyIntent` 常量（值 `"intent"`），作为意图识别结果的状态键
- 新增 `IntentType` 枚举：在 `pkg/criteria/enumor/aiagent.go` 中定义 `IntentType` 及其三个枚举值，附带 `Validate()` 方法
- 新增意图识别节点包：`cmd/agent-server/logics/agent/intent/`，包含节点构造函数与 Prompt 模板
- 修改 `BuildGraph`：将入口点从 `llm` 改为 `intent_recognition`，`intent_recognition → llm` 顺序连接，后续 llm 路由逻辑不变

> 本子需求**不包含**基于意图路由到不同子流程的实现（由「ReAct节点动态适配」子需求负责）。当前阶段意图结果写入 State 后，llm 节点继续以原有方式执行。

## Capabilities

### New Capabilities

- `intent-recognition-node`: Graph 意图识别节点，在 ReAct 主循环前对用户意图分类，将结果存入 Graph State 供后续节点消费

### Modified Capabilities

（无现有 spec 涉及 Graph 节点结构变更，故无 Modified Capabilities）

## Impact

**受影响代码**：
- `pkg/criteria/constant/aiagent.go`：新增 `StateKeyIntent` 常量
- `pkg/criteria/enumor/aiagent.go`：新增 `IntentType` 枚举及 `Validate()`
- `cmd/agent-server/logics/agent/intent/`：新建包（`node.go`、`prompt.go`）
- `cmd/agent-server/logics/agent/graph_build.go`：修改 `BuildGraph` 函数，插入意图识别节点

**不受影响**：
- LLMAgent（非 Graph 模式）路径不变
- 现有 hitl、tool、fallback 节点逻辑不变
- 所有 Service / Handler 层代码不变

**依赖**：
- `trpc.group/trpc-go/trpc-agent-go/model` 提供 `Model.Generate` 能力（意图识别节点内直接调用 LLM）
- 与「ReAct节点动态适配」子需求之间通过 `StateKeyIntent` 解耦，本需求仅负责写入，适配需求负责读取
