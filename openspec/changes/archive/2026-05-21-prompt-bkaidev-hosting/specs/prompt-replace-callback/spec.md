## ADDED Requirements

### Requirement: MakePromptReplaceCallback 在每次 LLM 调用前替换系统消息
`MakePromptReplaceCallback(store *PromptStore) model.BeforeModelCallbackStructured` SHALL 返回一个 callback，
在 `args.Request.Messages` 中找到 `Role == RoleSystem` 的消息并将其 `Content` 替换为 `store.SystemPrompt()` 的最新值。
如果 messages 中不存在 system message，则在头部插入一条新的 system message。

该 callback SHALL 与已有的 `MakeSkillInjectWithModelCallback` 和 `MakeTimeInjectCallback` 组合使用，
注册顺序为：PromptReplace → SkillInject → TimeInject（prompt replace 先执行，为其他 callback 提供基础系统消息）。

#### Scenario: GraphAgent 每次 LLM 调用前替换系统消息
- **WHEN** GraphAgent 的 LLM 节点触发 BeforeModel callback
- **THEN** `msgs[0].Content` 被替换为 `PromptStore.SystemPrompt()` 的当前值

#### Scenario: 远程同步后下一次 LLM 调用使用新 prompt
- **WHEN** Syncer 更新了 `PromptStore` 中的 system prompt
- **THEN** 该 session 的下一次 LLM 调用使用新 prompt，当前正在进行的请求不受影响

#### Scenario: PromptStore 为空时不替换系统消息
- **WHEN** `store.SystemPrompt()` 返回空字符串（未配置 systemPromptID 且未设置文件）
- **THEN** 保持 messages 不变，不插入空 system message

### Requirement: LLMAgent 同样注入 PromptReplaceCallback
LLMAgent（`agent_llm.go`）中的 `modelCb.BeforeModel` SHALL 也注册 `MakePromptReplaceCallback`，
使其与 GraphAgent 保持相同的 prompt 热更新能力。

LLMAgent 现有的 `WithGlobalInstruction(systemPrompt)` 和 `WithInstruction(instruction)` 传入初始构建时的内容（从 cc 配置读取），仍然保留；
PromptReplaceCallback 在运行时覆盖 system message，优先级更高。

#### Scenario: LLMAgent 运行时使用 PromptStore 的最新 system prompt
- **WHEN** LLMAgent 在 BeforeModel callback 中收到 PromptReplaceCallback
- **THEN** system message 被替换为 `PromptStore.SystemPrompt()`，覆盖构建时的静态值

### Requirement: Instruction 通过 BeforeModel callback 追加到系统消息
当 `PromptStore.Instruction()` 非空时，`MakePromptReplaceCallback` SHALL 在替换 system message 后，
将 instruction 内容追加到 system message 末尾（以 `\n\n` 分隔），模拟 LLMAgent `WithInstruction` 的行为。

#### Scenario: 同时配置了 system prompt 和 instruction 时合并内容
- **WHEN** `PromptStore.SystemPrompt()` 和 `PromptStore.Instruction()` 均非空
- **THEN** system message content = systemPrompt + "\n\n" + instruction
