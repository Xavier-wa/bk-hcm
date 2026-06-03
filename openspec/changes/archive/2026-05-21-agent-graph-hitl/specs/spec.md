## ADDED Requirements

### Requirement: HITL Graph 拓扑扩展

ReAct Graph SHALL 在现有 3 节点（`llm`、`tool`、`fallback`）基础上引入第 4 个 `hitl` 节点，实现三向条件边路由（`hitl`/`tool`/`fallback`）和 HITL 循环回边（`hitl` → `llm`）。

#### Scenario: LLM 调用 human_confirm 路由到 hitl 节点
- **WHEN** `llm` 节点输出包含 `tool_calls` 且首个调用为 `human_confirm`
- **THEN** 条件边 SHALL 路由到 `hitl` 节点

#### Scenario: LLM 调用其他工具路由到 tool 节点
- **WHEN** `llm` 节点输出包含 `tool_calls` 且所有调用均不为 `human_confirm`
- **THEN** 条件边 SHALL 路由到 `tool` 节点

#### Scenario: LLM 无工具调用路由到 fallback 节点
- **WHEN** `llm` 节点输出不包含 `tool_calls`
- **THEN** 条件边 SHALL 路由到 `fallback` 节点

#### Scenario: hitl 节点执行后回到 llm 节点
- **WHEN** `hitl` 节点执行完成（用户选择已注入 messages）
- **THEN** 图执行 SHALL 从 `hitl` → `llm` 继续

---

### Requirement: HITL 人工确认工具

`human_confirm` 工具 SHALL 注册为 LLM 可调用的特殊工具，通过 Tool Callbacks 的 `BeforeTool` 阶段拦截并调用 `graph.Interrupt` 触发中断。

#### Scenario: human_confirm 工具声明
- **WHEN** 系统初始化 Graph
- **THEN** `human_confirm` 工具 SHALL 注册到工具集，包含 `question`（string）和 `options`（array，可选）参数

#### Scenario: BeforeTool 拦截 human_confirm
- **WHEN** LLM 调用 `human_confirm` 工具
- **THEN** `BeforeTool` 回调 SHALL 拦截该调用，不执行实际业务逻辑

#### Scenario: 首次调用触发 Interrupt
- **WHEN** `BeforeTool` 回调拦截 `human_confirm` 且无 resume value
- **THEN** 回调 SHALL 调用 `graph.Interrupt(ctx, state, "human_confirm", interruptValue)` 抛出 `InterruptError`

#### Scenario: Resume 后返回用户选择
- **WHEN** Runner Resume 时传入 `Command.ResumeMap{"human_confirm": userInput}`
- **THEN** `graph.Interrupt` SHALL 返回 `userInput`，回调 SHALL 将用户选择包装为工具结果返回

---

### Requirement: HITL 中断恢复机制

`hitl` 节点内 SHALL 实现 `graph.Interrupt` 的幂等调用、checkpoint 保存/恢复、ResumeMap 用户选择传递。

#### Scenario: Interrupt 首次调用保存 checkpoint
- **WHEN** `graph.Interrupt` 首次被调用（无 UsedInterrupts 记录）
- **THEN** 框架 SHALL 保存 checkpoint，抛出 `InterruptError`，SSE 流结束

#### Scenario: Interrupt 幂等返回 resume value
- **WHEN** `graph.Interrupt` 再次被执行（Resume 后）
- **THEN** 框架 SHALL 从 `state[StateKeyResumeMap][key]` 读取值并返回，不再中断

#### Scenario: Checkpoint 恢复继续执行
- **WHEN** Runner 传入 `CfgKeyCheckpointID` 和 `StateKeyCommand.ResumeMap`
- **THEN** 框架 SHALL 从指定 checkpoint 恢复，重新执行 `hitl` 节点

#### Scenario: ResumeMap 传递用户输入
- **WHEN** Runner 构造 `Command{ResumeMap: map[string]any{"human_confirm": userInput}}`
- **THEN** `graph.Interrupt` SHALL 正确识别 key 并返回对应的 userInput

---

### Requirement: HITL 消息注入

`hitl` 节点恢复后 SHALL 将用户选择结果追加为 `role=user` 的 message，确保 LLM 能看到用户输入并继续推理。

#### Scenario: 用户选择追加为 user message
- **WHEN** `hitl` 节点从 tool result 提取到用户选择
- **THEN** 节点 SHALL 构造 `model.Message{Role: RoleUser, Content: userChoice}` 并追加到 `StateKeyMessages`

#### Scenario: LLM 看到用户选择后继续推理
- **WHEN** `hitl` → `llm` 边触发，`llm` 节点执行
- **THEN** LLM 的 messages 上下文 SHALL 包含用户选择作为最后一条 user message

#### Scenario: 消息顺序正确
- **WHEN** HITL 流程完成（human_confirm 调用 → 中断 → 恢复 → hitl 节点）
- **THEN** messages 顺序 SHALL 为：... → assistant (tool_call) → tool (result) → user (choice) → assistant (response)

---

## MODIFIED Requirements

无

## REMOVED Requirements

无
