## ADDED Requirements

### Requirement: IntentType enumeration is defined
系统 SHALL 在 `pkg/criteria/enumor/aiagent.go` 中定义 `IntentType string` 类型，包含三个枚举值：
- `IntentTypeHostApply IntentType = "host_apply"`
- `IntentTypeResourceQuery IntentType = "resource_query"`
- `IntentTypeChat IntentType = "chat"`

`IntentType` 必须提供 `Validate() error` 方法，当值不在枚举集合中时返回错误。

#### Scenario: Valid intent type passes validation
- **WHEN** 调用 `IntentTypeHostApply.Validate()`
- **THEN** 返回 nil

#### Scenario: Unknown intent type fails validation
- **WHEN** 调用 `IntentType("unknown").Validate()`
- **THEN** 返回非 nil 错误

---

### Requirement: StateKeyIntent constant is defined
系统 SHALL 在 `pkg/criteria/constant/aiagent.go` 中新增常量 `StateKeyIntent = "intent"`，用于在 `graph.State` 中存取意图识别结果。

#### Scenario: Constant value is correct
- **WHEN** 读取 `constant.StateKeyIntent`
- **THEN** 其字符串值为 `"intent"`

---

### Requirement: Intent recognition node classifies user intent with conversation context
系统 SHALL 提供 `MakeIntentRecognitionNode(mdl trpcmodel.Model, cfg cc.AgentIntentConfig) graph.NodeFunc`，返回的节点函数在被 Graph 执行时，必须：
1. 从 `state[graph.StateKeyMessages]` 中过滤掉 ToolCall/ToolResult 消息，取最近 `cfg.ContextWindowSize` 条作为历史上下文
2. 读取 `state[constant.StateKeyIntent]`（上一轮意图），注入到系统消息末尾作为延续提示
3. 以意图分类 Prompt（含延续提示）为系统消息、历史消息为上下文，调用 `mdl.GenerateContent`
4. 消费响应 channel，将各 chunk 的 `Content` 拼接得到完整响应（忽略 `ReasoningContent`）
5. 对响应内容做 `strings.TrimSpace`，与枚举值精确比较，确定 `IntentType`
6. 将 `IntentType` 字符串写入 `graph.State[constant.StateKeyIntent]` 并返回

#### Scenario: Successfully classifies host_apply intent
- **WHEN** 用户最新消息语义为主机申领（如"帮我申请一台主机"），LLM 返回 `"host_apply"`
- **THEN** 节点返回 `graph.State{constant.StateKeyIntent: "host_apply"}`，无错误

#### Scenario: Successfully classifies resource_query intent
- **WHEN** 用户最新消息语义为资源查询（如"查看业务下的主机"），LLM 返回 `"resource_query"`
- **THEN** 节点返回 `graph.State{constant.StateKeyIntent: "resource_query"}`，无错误

#### Scenario: Successfully classifies chat intent
- **WHEN** 用户最新消息为闲聊（如"你好"），LLM 返回 `"chat"`
- **THEN** 节点返回 `graph.State{constant.StateKeyIntent: "chat"}`，无错误

#### Scenario: Previous intent is injected into system message for context continuation
- **WHEN** `state[constant.StateKeyIntent]` 为 `"host_apply"`（上一轮已识别意图），且用户发送模糊消息
- **THEN** 系统消息末尾包含上一轮意图的提示信息，LLM 可据此判断是否延续意图

#### Scenario: ToolCall and ToolResult messages are excluded from context window
- **WHEN** `state[graph.StateKeyMessages]` 包含 ToolCall/ToolResult 消息
- **THEN** 这些消息被过滤，不计入 `contextWindowSize` 限制，也不传给意图识别 LLM

---

### Requirement: Intent recognition node falls back to chat on LLM failure
系统 SHALL 在以下情况下将意图设为 `IntentTypeChat` 并**正常返回**（不返回 error）：
- `mdl.GenerateContent` 返回 error
- 响应 channel 中的 `Response.Error` 非 nil
- 响应 `Content` 经 TrimSpace 后不属于任何已知 `IntentType`

#### Scenario: LLM call error triggers fallback
- **WHEN** `mdl.GenerateContent` 返回非 nil 错误
- **THEN** 节点返回 `graph.State{constant.StateKeyIntent: "chat"}`，节点本身返回 nil error，并记录 Warn 日志

#### Scenario: Unknown LLM response triggers fallback
- **WHEN** LLM 响应内容为无法识别的字符串（如 `"intent: host_apply"`）
- **THEN** 节点返回 `graph.State{constant.StateKeyIntent: "chat"}`，节点本身返回 nil error，并记录 Warn 日志

---

### Requirement: Intent recognition node handles missing user message gracefully
系统 SHALL 在 `state[graph.StateKeyMessages]` 为空或其中不含任何 `RoleUser` 消息时，将意图设为 `IntentTypeChat` 并正常返回。

#### Scenario: Empty messages state falls back to chat
- **WHEN** `state[graph.StateKeyMessages]` 为空切片
- **THEN** 节点返回 `graph.State{constant.StateKeyIntent: "chat"}`，无错误

---

### Requirement: AgentIntentConfig is validated at startup
系统 SHALL 在 `AgentIntentConfig.Validate()` 中检查 `IntentPrompt`：当 `intentPromptFile` 未配置或对应文件为空时，返回非 nil 错误，使 agent-server 启动失败。

#### Scenario: Missing intent prompt file causes startup failure
- **WHEN** `intentPromptFile` 配置为空字符串
- **THEN** `AgentIntentConfig.Validate()` 返回非 nil 错误

#### Scenario: Valid config passes validation
- **WHEN** `intentPromptFile` 指向一个非空文件
- **THEN** `AgentIntentConfig.Validate()` 返回 nil

---

### Requirement: Intent recognition supports configurable model
系统 SHALL 在 `AgentIntentConfig` 中支持 `modelName` 字段：当 `modelName` 非空且在 `modelsMap` 中存在时，使用该模型进行意图识别；否则使用 `defaultMdl`。

#### Scenario: Empty modelName uses default model
- **WHEN** `AgentIntentConfig.ModelName` 为空字符串
- **THEN** 意图识别节点使用 Graph 的 `defaultMdl`

#### Scenario: Configured modelName uses specified model
- **WHEN** `AgentIntentConfig.ModelName` 为已注册模型名
- **THEN** 意图识别节点使用对应的独立模型实例

---

### Requirement: Graph entry point is updated to intent_recognition
`BuildGraph` 函数 SHALL 将 Graph 入口点设为 `"intent_recognition"` 节点，并添加 `"intent_recognition" → "llm"` 边，使意图识别节点在每次执行时先于 llm 节点运行。

#### Scenario: intent_recognition runs before llm
- **WHEN** 用户发送消息触发 Graph 执行
- **THEN** `intent_recognition` 节点先于 `llm` 节点执行，且执行完成后 `state[constant.StateKeyIntent]` 已被写入

#### Scenario: Existing llm → ConditionalEdge routing is unchanged
- **WHEN** `intent_recognition` 节点执行完毕后 `llm` 节点执行
- **THEN** `llm → ConditionalEdge → hitl/tool/fallback` 的路由逻辑与重构前保持一致
