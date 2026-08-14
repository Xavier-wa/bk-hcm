## Requirements

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

### Requirement: Intent recognition node classifies user intent with conversation context

意图识别 SHALL 以纯函数而非 graph 节点的形式提供：`intent.Classify(ctx context.Context, mdl trpcmodel.Model, promptStore *prompt.Store, messages []trpcmodel.Message, contextWindowSize int) enumor.IntentType`。该函数在被调用时必须：

1. 从 `messages` 中过滤掉 ToolCall/ToolResult 消息，取最近 `contextWindowSize` 条作为历史上下文
2. 以意图分类 Prompt（`constant.IntentRecognitionPromptKey`）为系统消息、历史消息为上下文，调用 `mdl.GenerateContent`
3. 消费响应 channel，将各 chunk 的 `Content` 拼接得到完整响应（忽略 `ReasoningContent`）
4. 对响应内容做 `strings.TrimSpace`，与枚举值精确比较，确定 `IntentType`
5. **返回** `IntentType`，SHALL NOT 写入任何 graph state

调用方（`scene_dispatch`）SHALL 把返回值作为局部值消费。系统 SHALL NOT 再提供 `MakeIntentRecognitionNode` 构造器，主图 SHALL NOT 注册 `intent_recognition` 节点。

#### Scenario: Successfully classifies host_apply intent

- **WHEN** 用户最新消息语义为主机申领（如"帮我申请一台主机"），LLM 返回 `"host_apply"`
- **THEN** `Classify` 返回 `IntentTypeHostApply`

#### Scenario: Successfully classifies resource_query intent

- **WHEN** 用户最新消息语义为资源查询（如"查看业务下的主机"），LLM 返回 `"resource_query"`
- **THEN** `Classify` 返回 `IntentTypeResourceQuery`

#### Scenario: Successfully classifies chat intent

- **WHEN** 用户最新消息为闲聊（如"你好"），LLM 返回 `"chat"`
- **THEN** `Classify` 返回 `IntentTypeChat`

#### Scenario: ToolCall and ToolResult messages are excluded from context window

- **WHEN** `messages` 包含 ToolCall/ToolResult 消息
- **THEN** 这些消息被过滤，不计入 `contextWindowSize` 限制，也不传给意图识别 LLM

#### Scenario: Classification result is not persisted to graph state

- **WHEN** `Classify` 返回任意分类结果
- **THEN** graph state 中不存在承载该结果的键，调用方以局部值消费

---

### Requirement: Intent recognition node falls back to chat on LLM failure

`intent.Classify` SHALL 在以下情况下返回 `IntentTypeChat` 而非向调用方抛出错误：

- `mdl.GenerateContent` 返回 error
- 响应 channel 中的 `Response.Error` 非 nil
- 响应 `Content` 经 TrimSpace 后不属于任何已知 `IntentType`
- `promptStore` 为 nil 或分类 Prompt 缺失

由于调用方按决策矩阵把 `chat` 视作「不受支持场景」，对已有受支持 `session_tag` 的会话，这一降级的效果即为「保持原场景不切换」。

#### Scenario: LLM call error triggers fallback

- **WHEN** `mdl.GenerateContent` 返回非 nil 错误
- **THEN** `Classify` 返回 `IntentTypeChat` 并记录 Error 日志，不向调用方返回错误

#### Scenario: Unknown LLM response triggers fallback

- **WHEN** LLM 响应内容为无法识别的字符串（如 `"intent: host_apply"`）
- **THEN** `Classify` 返回 `IntentTypeChat` 并记录 Warn 日志

#### Scenario: Missing prompt does not break the run

- **WHEN** `promptStore` 为 nil 或分类 Prompt 未配置
- **THEN** `Classify` 返回 `IntentTypeChat` 并记录 Error 日志，当前 run 继续按原场景执行

---

### Requirement: Intent recognition node handles missing user message gracefully

`intent.Classify` SHALL 在 `messages` 为空或其中不含任何 `RoleUser` 消息时直接返回 `IntentTypeChat`，SHALL NOT 发起 LLM 调用。

#### Scenario: Empty messages falls back to chat without LLM call

- **WHEN** `messages` 为空切片
- **THEN** `Classify` 返回 `IntentTypeChat`，且未调用 `mdl.GenerateContent`

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

### Requirement: Intent classification latency is observable

由于意图识别不再是独立 graph 节点，框架的 node start/complete 事件不再自动为其打点。系统 SHALL 显式记录每次分类调用的耗时与结果，使「每轮固定一次分类调用」的延迟成本可在生产环境度量。

#### Scenario: 分类调用产生可度量的耗时记录

- **WHEN** `scene_dispatch` 调用 `intent.Classify` 完成一次分类
- **THEN** 系统记录该次调用的耗时、分类结果与 rid
