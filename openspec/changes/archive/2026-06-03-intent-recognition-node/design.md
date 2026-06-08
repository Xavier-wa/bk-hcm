## Context

当前 GraphAgent（`enumor.AgentModeGraph`）使用单一 ReAct Graph 处理所有用户请求，Graph 拓扑为：

```
START → llm(LLMNode) → ConditionalEdge → hitl / tool / fallback → END
```

`BuildGraph` 函数接收 `trpcmodel.Model`、`skill.Repository`、工具集、Prompt 配置等参数，生成编译后的 `*graph.Graph`。`graph.State` 是 `map[string]any` 类型，框架通过 `graph.MessagesStateSchema()` 提供消息列表的 append reducer，其余 key 默认使用覆盖（replace）语义。

目标：在 `START` 和 `llm` 之间插入一个 `intent_recognition` 函数节点，该节点对用户最新消息进行意图分类，将结果写入 State，供本子需求后续的"ReAct节点动态适配"读取。

## Goals / Non-Goals

**Goals:**
- 新增 `intent_recognition` 节点，直接调用 LLM 对用户最新消息进行三分类
- 将分类结果以 `StateKeyIntent`（`"intent"`）为 key 写入 Graph State
- 分类失败（LLM 调用失败 / 返回无法解析的结果）时 fallback 到 `chat`
- 定义 `IntentType` 枚举，在常量文件中统一管理意图类别标识符
- 修改 `BuildGraph` 使新节点成为 Graph 入口点

**Non-Goals:**
- 基于意图结果路由到不同子流程（由"ReAct节点动态适配"负责）
- 为意图识别使用独立的轻量模型（当前阶段复用 `defaultMdl`）
- 修改 LLM 节点、hitl、tool、fallback 的行为

## Decisions

### 1. 节点位置：顺序插入而非条件分支

**决策**：`intent_recognition → llm`（顺序边），不在本需求中引入基于意图的条件路由。

**原因**：本子需求的职责边界明确——只负责写入 intent，不负责基于 intent 做路由。路由逻辑由"ReAct节点动态适配"在 `llm` 节点的 callback 或独立的条件边中实现。保持职责分离可以降低本子需求的测试复杂度，也不会影响现有 hitl/tool/fallback 分支的回归。

**备选**：在本需求中同时实现条件路由 → 增加协调成本，且路由所需的差异化工具集/Prompt 尚未就绪。

---

### 2. LLM 调用方式：直接调用 Model.GenerateContent，传入最近 N 条消息

**决策**：在 `intent_recognition` 节点内部，直接调用 `trpcmodel.Model.GenerateContent(ctx, req)` 发起一次独立的 LLM 请求。请求消息包含：
1. 意图分类 System Prompt
2. `state[graph.StateKeyMessages]` 中**最近 N 条消息**（N 可配置，默认 5）
3. 当前 `state[constant.StateKeyIntent]`（上一轮意图，可能为空）注入到 System Prompt 末尾作为上下文提示

**原因**：
- 仅传最新一条用户消息时，多轮对话中的模糊消息（如"16G内存够吗"）会被错误分类。传入最近 N 条消息后，LLM 能自然理解对话上下文，意图延续问题不需要额外逻辑
- 注入 `previousIntent` 作为 Prompt 提示，处理极端模糊输入的最后保障（"继续"、"好的"等）
- `graph.NodeFunc` 是 `func(ctx context.Context, state graph.State) (any, error)` 签名，不能驱动框架 LLMNode；直接调用 `GenerateContent` 更轻量
- `ctx` 中已由 service 层注入 `BKUsername`/`BKTicket`，model 的 middleware 会自动读取，鉴权透明

**消息窗口**：从 `state[graph.StateKeyMessages]` 中取最后 N 条（N 通过 `AgentIntentConfig.ContextWindowSize` 配置，默认 5），不含 ToolCall/ToolResult 消息（这类消息对意图分类无帮助）。

**实现细节**：
```
systemMsg := buildIntentSystemMsg(intentPrompt, previousIntent)
historyMsgs := extractRecentMessages(messages, contextWindowSize)
req := &model.Request{
    Messages: append([]model.Message{model.NewSystemMessage(systemMsg)}, historyMsgs...),
    GenerationConfig: model.GenerationConfig{Stream: false, MaxTokens: &intentMaxTokens},
}
respCh, err := mdl.GenerateContent(ctx, req)
// drain channel, accumulate Content
```

**注意**：部分思考型模型（DeepSeek-R1、Hunyuan2-Thinking）会在 `ReasoningContent` 字段返回推理过程，实际分类结果在 `Content` 字段。解析时只读 `Content`，对其做 `strings.TrimSpace` 后与枚举值比较。

---

### 3. State 扩展：自定义 key，无需修改 Schema

**决策**：定义 `constant.StateKeyIntent = "intent"` 作为 State key，直接写入 `graph.State`。不修改 `graph.MessagesStateSchema()`。

**原因**：`graph.State` 是 `map[string]any`，非 messages key 的字段默认使用覆盖语义，满足每轮覆盖意图的需求。不引入新的 schema 类型可以减少对框架的依赖。

---

### 4. 枚举定义位置

**决策**：在 `pkg/criteria/enumor/aiagent.go` 新增 `IntentType string` 类型及三个枚举值（`IntentTypeHostApply`, `IntentTypeResourceQuery`, `IntentTypeChat`），在 `pkg/criteria/constant/aiagent.go` 新增 `StateKeyIntent`。

**原因**：与现有 `AgentMode`、`GraphCheckpointBackend` 等保持一致，便于后续需求复用。

---

### 5. 意图识别模型配置：预留独立模型配置项

**决策**：当前阶段意图识别复用 `defaultMdl`（主 Agent 模型），但在 `cc` 配置中预留 `AgentIntentConfig` 结构，使运维可以在不修改代码的情况下为意图识别指定独立（更轻量）的模型。

**配置结构**（新增在 `AgentAGUI` 下）：
```yaml
intent:
  modelName: ""               # 为空时复用 defaultMdl（当前阶段）
  contextWindowSize: 5        # 传入意图识别的最近消息数量，默认 5
  intentPromptFile: "..."     # 意图分类 Prompt 文件路径（必填，空时启动报错）
```

`BuildGraph` 接收 `AgentIntentConfig`，由配置决定使用哪个 model。当 `modelName` 为空时传入 `defaultMdl`，非空时从 `modelsMap` 中查找。

**原因**：预留扩展点，使意图识别可以独立选用更快、更便宜的轻量模型，降低每轮额外延迟。

---

### 6. 包结构：新建 `agent/intent` 包

**决策**：新建 `cmd/agent-server/logics/agent/intent/` 包，包含：
- `node.go`：`MakeIntentRecognitionNode(mdl trpcmodel.Model, intentPrompt string) graph.NodeFunc`

Prompt 内容通过参数注入（来自 `cc.AgentPromptConfig.IntentPrompt`），不在包内定义常量。

**原因**：与 `agent/hitl` 包的组织方式一致；Prompt 由外部文件管理，节点包只关注调用逻辑。

---

### 7. Prompt 文件化 + 启动强校验

**决策**：意图识别 Prompt 存放在 `cmd/agent-server/etc/prompts/intent_recognition_prompt.md`。`intentRecognitionPromptFile` 为**必填**配置项，未配置时 `AgentIntentConfig.Validate()` 返回错误，启动直接失败。

**加载链路**：
1. 在 `AgentIntentConfig` 中定义 `IntentPromptFile string`（yaml）和 `IntentPrompt string`（内存）
2. `trySetDefault()` 调用 `loadPromptFile(IntentPromptFile)` 赋值给 `IntentPrompt`
3. `Validate()` 中检查 `IntentPrompt == ""` 时返回错误（文件路径未配置或文件为空均报错）
4. `runtime.go` 的 `newAGUIRunner` 将 `aguiCfg.Intent`（整个配置）传入 `BuildGraph`

**原因**：意图分类 Prompt 是功能正确性的前提，启动时发现比运行时 fallback 更安全，避免上线后悄悄退化为全 chat 意图。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|----------|
| 意图识别准确率受模型能力影响，分类错误导致错误路由 | Prompt 中提供清晰的分类示例；失败 fallback 到 `chat`；后续可通过评测持续优化 Prompt |
| 每次用户消息都增加一次 LLM 调用，首次响应延迟略有增加 | 意图识别请求无工具、无历史消息、MaxTokens 较小（约 16），延迟可控；当前阶段无 SLA 硬指标 |
| 思考型模型（DeepSeek-R1 等）在 `Content` 前可能有 `<think>...</think>` 前缀残留 | 使用 `strings.TrimSpace` + 与枚举值精确比较，不匹配则 fallback |
| 节点直接调用 `GenerateContent` 绕过框架的 model callback 体系（日志、token filter 等） | 意图识别调用是独立轻量请求，不需要 HITL/history filter 等中间件；日志在节点内手动记录 |

## Migration Plan

本次变更仅在 `enumor.AgentModeGraph` 模式下生效（`BuildGraph` 函数路径），LLMAgent 模式（默认 `enumor.AgentModeAgent`）不受影响。无数据库迁移，无 API 变更，无外部依赖新增。回滚方式：恢复 `graph_build.go` 中的入口点和边配置即可。

## Open Questions

暂无未决问题，所有决策已在上文 Decisions 中明确。
