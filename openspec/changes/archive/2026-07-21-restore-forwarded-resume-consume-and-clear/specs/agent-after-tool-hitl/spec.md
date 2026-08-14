## MODIFIED Requirements

### Requirement: after_tool_hitl 恢复后回到 llm

`after_tool_hitl` 触发中断并经 resume 后 SHALL 将用户选择并入消息（复用 `message.BuildFallbackResumeDelta`）并路由回 `llm` 继续当前任务；`after_tool_hitl` → `llm` SHALL 为循环边。

`after_tool_hitl` SHALL 优先采用前端经 `forwardedProps` 传入、存于 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]` 的结构化回复（该值由 `service.go: tryPrepareAutoResume` 在**每次 HTTP Run** 重建进 RuntimeState，语义为前端本轮结构化协议）：命中（非空字符串）时 SHALL 以其作为用户选择并发自定义事件 `after_tool_hitl.resume_forwarded`（payload `{value}`）、忽略 `graph.Interrupt` 返回的自由文本 resume 值；未命中时 SHALL 回退使用 `graph.Interrupt` 返回的自由文本 resume 值，且该值 SHALL 为非空字符串（否则报错终止）。

子图输入映射 `makeSubgraphInputMapper` SHALL 在构建子图 `state` 时 `delete(child, StateKeyForwardedResumeValue)`，使该 key **不进入**子图持久化 `state`。由此，子图 `after_tool_hitl → llm` 循环边同轮多次中断恢复时，持久化 checkpoint 不含旧值，`mergeInitialStateNonInternal` 的 checkpoint 优先规则不会压制本轮新值；每次恢复经 `mergeInitialStateNonInternal` 用本轮 `tryPrepareAutoResume` 写入的 `inv.RunOptions.RuntimeState` 新值补位，`after_tool_hitl` 读到当轮值，避免脏读。

#### Scenario: 前端结构化回复优先恢复

- **WHEN** 前端经 `forwardedProps` 传入了非空结构化回复（写入 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`）
- **THEN** `after_tool_hitl` 以该结构化值作为用户选择、发 `after_tool_hitl.resume_forwarded` 事件，并将其并入消息回到 `llm`

#### Scenario: 无结构化回复时用自由文本恢复

- **WHEN** `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]` 未命中（本轮无 forwarded），仅有 `graph.Interrupt` 返回的自由文本值
- **THEN** `after_tool_hitl` 校验其为非空字符串后并入消息，流程回到 `llm` 继续

#### Scenario: 入口剥键避免同轮二次中断脏读

- **WHEN** 子图 `after_tool_hitl → llm` 同轮内发生多次中断恢复（如先 by_static 选 A、再 by_plan 选 B），且子图 `state` 已不含 `StateKeyForwardedResumeValue`（入口剥键）
- **THEN** 每次恢复的 checkpoint 均不含该 key，`mergeInitialStateNonInternal` 用本轮 `tryPrepareAutoResume` 写入的当轮值（B）补位，`after_tool_hitl` 读到 B（而非第一次的 A），不误用历史选择