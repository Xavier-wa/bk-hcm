## 1. 枚举与常量定义

- [x] 1.1 在 `pkg/criteria/enumor/aiagent.go` 中新增 `IntentType string` 类型，定义 `IntentTypeHostApply`、`IntentTypeResourceQuery`、`IntentTypeChat` 三个枚举值，并实现 `Validate() error` 方法
- [x] 1.2 在 `pkg/criteria/constant/aiagent.go` 中新增 `StateKeyIntent = "intent"` 常量

## 2. 意图识别专属配置结构

- [x] 2.1 在 `pkg/cc/service.go` 中新增 `AgentIntentConfig` 结构体（位于 `AgentAGUI` 内，yaml tag: `intent`），包含：
- `IntentRecognitionPromptFile string`（yaml: `intentRecognitionPromptFile`，必填，空时 `Validate()` 返回错误）
  - `IntentPrompt string`（不暴露 yaml，由 `trySetDefault()` 加载）
  - `ModelName string`（yaml: `modelName`，可选，为空时复用 defaultMdl）
  - `ContextWindowSize int`（yaml: `contextWindowSize`，默认 5，意图识别传入的最近消息数量）
- [x] 2.2 实现 `AgentIntentConfig.trySetDefault()`：调用 `loadPromptFile(IntentPromptFile)` 赋值 `IntentPrompt`；`ContextWindowSize` 为 0 时设为默认值 5
- [x] 2.3 实现 `AgentIntentConfig.Validate()`：`IntentPrompt == ""` 时返回错误（文件未配置或内容为空）
- [x] 2.4 在 `AgentAGUI.trySetDefault()` 和 `AgentAGUI.Validate()` 中调用 `Intent.trySetDefault()` 和 `Intent.Validate()`
- [x] 2.5 新建 `cmd/agent-server/etc/prompts/intent_recognition_prompt.md`，内容为意图分类系统 Prompt（包含三类意图的定义、示例，以及"当前上下文意图"动态占位符说明）
- [x] 2.6 在 `cmd/agent-server/etc/agent_server.yaml` 的 `agui` 配置段中添加 `intent` 配置示例（`intentPromptFile`、`contextWindowSize`、`modelName`）

## 3. 意图识别节点实现

- [x] 3.1 新建 `cmd/agent-server/logics/agent/intent/node.go`，实现 `MakeIntentRecognitionNode(mdl trpcmodel.Model, cfg cc.AgentIntentConfig) graph.NodeFunc`：
  - 从 `state[graph.StateKeyMessages]` 中过滤掉 ToolCall/ToolResult 消息后，取最近 `cfg.ContextWindowSize` 条作为历史上下文
  - 如无任何用户消息，直接 fallback 到 `chat`
  - 读取 `state[constant.StateKeyIntent]`（上一轮意图，可为空），注入到系统消息末尾作为延续提示
  - 构造 `model.Request`（系统消息 + 历史消息，`Stream: false`，MaxTokens 适当限制如 16）
  - 调用 `mdl.GenerateContent(ctx, req)` 并 drain channel 拼接 `Content`（忽略 `ReasoningContent`）
  - 对响应做 `strings.TrimSpace`，与枚举值精确比较，无法识别时 fallback 到 `chat`（记录 Warn 日志）
  - 返回 `graph.State{constant.StateKeyIntent: string(intent)}`；LLM 调用失败同样 fallback 到 `chat`（记录 Warn 日志，不返回 error）

## 4. Graph 结构变更

- [x] 4.1 在 `runtime.go` 的 `newAGUIRunner` 中，根据 `aguiCfg.Intent.ModelName` 从 `modelsMap` 查找意图识别模型（为空时使用 `defaultMdl`），并将模型和 `aguiCfg.Intent` 传入 `agent.BuildGraph`
- [x] 4.2 修改 `BuildGraph` 函数签名，新增 `intentMdl trpcmodel.Model` 和 `intentCfg cc.AgentIntentConfig` 参数
- [x] 4.3 在 `BuildGraph` 函数体中：
  - 注册节点：`stateGraph.AddNode("intent_recognition", intent.MakeIntentRecognitionNode(intentMdl, intentCfg))`
  - 修改入口点：`stateGraph.SetEntryPoint("intent_recognition")`（原为 `"llm"`）
  - 添加边：`stateGraph.AddEdge("intent_recognition", "llm")`
  - 更新函数注释中的 Graph 拓扑说明

## 5. 单元测试

- [x] 5.1 在 `cmd/agent-server/logics/agent/intent/` 新增 `node_test.go`，使用 mock model 测试：
  - LLM 返回 `"host_apply"` → state intent 为 `host_apply`
  - LLM 返回 `"resource_query"` → state intent 为 `resource_query`
  - LLM 返回 `"chat"` → state intent 为 `chat`
  - LLM 返回无法识别字符串 → fallback 到 `chat`，节点 error 为 nil
  - LLM 调用返回 error → fallback 到 `chat`，节点 error 为 nil
  - messages 为空 → fallback 到 `chat`，节点 error 为 nil
  - 有 previousIntent（`host_apply`）时，系统消息中包含延续提示
  - ToolCall/ToolResult 消息被过滤，不计入 contextWindowSize
- [x] 5.2 在 `pkg/criteria/enumor/` 补充 `IntentType.Validate()` 的测试用例
