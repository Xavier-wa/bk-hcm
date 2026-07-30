## Context

主机申领 Agent 采用主图 + `host_apply` 子图的 Graph 结构。子图内含 `account_select`、`llm`、`hitl`、`tool`、`after_tool_hitl` 节点，带 `WithCheckpointSaver` 持久化每轮 checkpoint。前端在中断恢复时经 `forwardedProps.resumeValue` 回传结构化选择（账号、方案、提单参数），该值经 `service.go: tryPrepareAutoResume` 写入 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`（常量 `"forwarded_resume_value"`，`pkg/criteria/constant/aiagent.go:252`），同时作为 resume command 注入。

设计理念（用户坚持保留）：`StateKeyCommand`（resume command）= 用户输入框自由文本；`forwardedProps.resumeValue` / `StateKeyForwardedResumeValue` = 前端结构化协议。二者应**按字段来源区分**，而非按内容（JSON 嗅探）区分。

MR 3191 把 `after_tool_hitl` 改为只信 `graph.Interrupt` 返回的 `resumeValue` + JSON 嗅探，抹掉了来源区分，并引入「自由文本是 JSON 则误判为结构化」的误判。本次方案希望恢复来源区分，并以「消费后清除 `state` 中的 key」消除脏数据。

**脏数据根因（已确认）**：`mergeInitialStateNonInternal`（`trpc-agent-go/graph`）合并规则为 **checkpoint 优先、请求态补位**：恢复时先从 checkpoint 还原 `state`，再把本次请求的 `inv.RunOptions.RuntimeState` merge 进去；`StateKeyForwardedResumeValue` 非 `_` 前缀，不跳过，若 checkpoint 已有旧值则**保留旧值、忽略本次新值**。`after_tool_hitl → llm` 是循环边，同轮多次中断，旧 checkpoint 持久化的旧值压制新值 → 第二次恢复读到第一次选择（数据错误）。

**关键约束（已结合 v1.8.1 与 `tryPrepareAutoResume` 核实并修正）**：HITL 的每一次中断恢复都是一次新的 HTTP Run，`service.go: tryPrepareAutoResume` 在**每次 Run** 时重新执行、重建 `inv.RunOptions.RuntimeState`（含 `StateKeyForwardedResumeValue` 与 resume command）。因此 `after_tool_hitl` 作为子图**内部循环节点**读 `inv.RunOptions.RuntimeState` 同样可靠——与 `account_select`/`hitl`（子图 entry 节点）一致，均不脏。`after_tool_hitl → llm` 循环边虽发生在单次 Run 的 executor 内部、不重新跑 `tryPrepareAutoResume`，但循环边处理的仍是同一轮 choice，前端不会在循环边内部重发 forwarded，故 RuntimeState 不变恰是正确行为。

**最终方案（已定稿）**：把「消费后清除不可靠的 `{nil}` StateDelta」替换为「**入口剥键 + 读请求级 RuntimeState**」——在 `makeSubgraphInputMapper` 处 `delete(child, StateKeyForwardedResumeValue)`（项目已有同类 `delete(child, CfgKeyCheckpointID)` 模式），从源头阻止该 key 进入子图持久化 state；`after_tool_hitl` 改读 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`。这等价达成「消费后清除」目标（子图 state 永远不含旧值，每次恢复经 `mergeInitialStateNonInternal` 用当轮 RuntimeState 补位），但实现于框架真正支持 `delete` 的层级，规避了「节点返回值 `{nil}` 不能删 key」的框架陷阱（见 Risks）。

## Goals / Non-Goals

**Goals:**
- 恢复 `after_tool_hitl` 的「来源区分」设计：forwarded 结构化（写 `state[StateKeyForwardedResumeValue]`）→ 优先；自由文本（走 `resumeValue`）→ 回退。
- 通过消费后清除 `state` 中的 `StateKeyForwardedResumeValue`，消除子图循环边同轮多次中断恢复的脏数据。
- 保持 `account_select`/`hitl` 现有 `inv.RunOptions.RuntimeState` 通道不变（已验证不脏）。

**Non-Goals:**
- 不改为「统一走 `resumeValue` + JSON 嗅探」（即不维持 MR 现状）。
- 不修改 `makeSubgraphInputMapper`（保持透传，因 `after_tool_hitl` 需从 `state` 读该 key）。
- 不改动前端协议（`forwardedProps.resumeValue` 字段不变）。
- 不删除 `StateKeyForwardedResumeValue` 这个 key（`account_select`/`hitl` 仍依赖它）。

## Decisions

### Decision 1: `makeSubgraphInputMapper` 入口剥键，从源头阻止 `StateKeyForwardedResumeValue` 进入子图持久化 state

在 `graph_build.go` 的 `makeSubgraphInputMapper`（紧邻已有的 `delete(child, graph.CfgKeyCheckpointID)`）增加：
```go
// 子图状态里不持久化 forwarded_resume_value：该值每轮由父图 tryPrepareAutoResume
// 重建进 inv.RunOptions.RuntimeState，子图节点应读 RuntimeState 而非子图 state。
// 否则旧 checkpoint 会持久化旧值，经 mergeInitialStateNonInternal 的 checkpoint 优先
// 规则压制当轮新值，导致循环边同轮多次恢复读到历史选择（脏读）。
delete(child, constant.StateKeyForwardedResumeValue)
```

**Rationale**: `mergeInitialStateNonInternal` 的合并语义为 **checkpoint 优先、请求态补位**（`exists(key)` 判定、不看值）。只要子图 state 里出现过该 key，其旧值就会压制 `tryPrepareAutoResume` 补入的当轮新值。在入口 `delete` 保证子图 state **永远不含该 key** → 每次恢复 `restored` 缺该 key → `mergeInitialStateNonInternal` 用当轮 `RuntimeState` 补位 → 读到本轮值，脏读根因被**从源头切断**。`delete(state, key)` 是 v1.8.1 框架唯一支持「删除 key」的层级（节点返回值 `{key: nil}` 不能删 key，见 Risks），项目可插手的正是 input mapper。

### Decision 2: `after_tool_hitl` 改读 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`，恢复来源区分

在 `aftertool/node.go` 的 `doInterrupt` 中恢复「优先读 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`（forwarded 结构化）→ 未命中回退 `graph.Interrupt` 自由文本 `resumeValue`」：
- 命中（非空字符串）→ 发 `after_tool_hitl.resume_forwarded` 事件（`emitForwardedResumeEvent`），以该值作为用户选择并入消息；
- 未命中 → 回退 `graph.Interrupt` 返回的 `resumeValue`，校验为非空字符串（否则报错终止）；
- **移除当前的「JSON 嗅探」分支**（`json.Unmarshal` 判断 choice 是否 JSON 并据此发事件的逻辑）。

**Rationale**: 恢复设计理念（来源区分：command=自由输入 / forwarded=结构化协议）。`after_tool_hitl` 读 `inv.RunOptions.RuntimeState` 在子图恢复时可靠——HITL 每次中断恢复都是新 HTTP Run，`tryPrepareAutoResume` 每次重建 RuntimeState（含当轮 forwarded 与 resume command）。这与 `account_select`/`hitl` 读 RuntimeState 不脏同因。移除 JSON 嗅探后，自由文本即使是合法 JSON 也走 `resumeValue` 自由文本通道，不再误发 `resume_forwarded` 事件。

**Alternatives considered**:
- 维持 MR 现状（统一 `resumeValue` + JSON 嗅探）：抹掉来源区分，且自由文本是 JSON 会误发 `resume_forwarded` 事件；否决（违背设计理念）。
- 「消费后返回 `graph.State{key: nil}` 清除」：已证实在 v1.8.1 **无效**（`{key: nil}` 使 key 仍 exists、`mergeInitialStateNonInternal` 压制新值、脏读复活）；否决（见 Risks）。

### Decision 3: 取消「消费后清除」机制，不再依赖脆弱的 StateDelta 删除

本方案**不要求** `after_tool_hitl` 返回清除该 key 的 StateDelta。脏读根因（子图 state 持久化旧值）已被 Decision 1 的入口 `delete` 从源头切断，无需每次恢复后清理。

**Rationale**: 「读完即清」是一种易遗漏的全局纪律——任何读 `state[StateKeyForwardedResumeValue]` 的中断节点若漏清即复活脏读。改为「入口剥键 + 读 RuntimeState」后，该 key 根本不在子图 state 流转，**无清理负担**，且 `account_select`/`hitl` 读的是 RuntimeState 那份、从不受影响。

### Decision 4: `account_select`/`hitl` 维持读 `RuntimeState`，不改动

`account_select`（`resolveSelectedAccountID`）与 `hitl` 继续从 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]` 读取（它们本就读 RuntimeState 而非子图 state，与本次入口剥键不冲突）。`accountID` 的对话上下文保持走独立的 session state 持久化链路，不受剥键影响（见 Risks「已排除」）。

### Decision 5: `service.go` 双通道维持，注释对齐

`tryPrepareAutoResume` 维持 `forwarded` 同时写入 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`（给 `account_select`/`hitl`/`after_tool_hitl` 读 RuntimeState）与 resume command（给 `after_tool_hitl` 的 `resumeValue` 自由文本回退）。仅调整注释，明确该 key 经 RuntimeState（请求级、每 Run 重建）服务于各中断节点，不再经子图持久化 state。

## Risks / Trade-offs

- **[Risk][已规避] 节点返回 `graph.State{key: nil}` 不能清除 key（v1.8.1）**：`mergeInitialStateNonInternal`（executor.go:696）用 `exists(key)` 判定（不看值），checkpoint 优先、initial 补位；`StateSchema.ApplyUpdate`（state.go:297）默认 `result[key] = deepCopyAny(nil)`；`createCheckpointFromState`/`safeClone` 不跳过 nil。即「消费后返回 `{key: nil}`」会让 key 仍 **exists 且值为 nil**，下次恢复 `exists(key)` 为真、当轮新值被压制 → 脏读复活。框架内部真正删除 key 用 `delete(state, key)`（如 `applyCheckpointResumeFields` 的 `delete(child, CfgKeyCheckpointID)`），但**节点返回值层无删除机制**。
  → **本方案如何规避**：最终方案**不采用**「消费后 `{nil}` 清除」。改为在 `makeSubgraphInputMapper` 处 `delete(child, StateKeyForwardedResumeValue)`（框架支持的删除层级），从源头阻止该 key 进入子图持久化 state；节点读 `inv.RunOptions.RuntimeState`（每 Run 重建）而非子图 state。脏读根因被入口 `delete` 切断，无需任何「返回值删除」机制。
- **[Risk][已排除] 清除 `forwarded_resume_value` 不会丢失 accountID（对话上下文保持不受影响）**：`accountID` 的留存走与 `forwarded_resume_value` 完全独立的两条链路：① 会话级持久 `PersistStateToService({SessionSelectedAccountIDStateKey: accountID})`（写入 session state，非 graph checkpoint state）；② `account_select` 复用时读 `inv.Session.GetState(SessionSelectedAccountIDStateKey)`（account_select.go:274，**不读 forwarded key**），LLM 占位符注入读 `RuntimeState[SessionAccountIDTempKey]`（callback.go:201，**不读 forwarded key**）。三 key 字面值互不相同（`aiagent.go:252` 等），清除只针对 `forwarded_resume_value` 单一 key，**不会触碰 `SessionAccountIDTempKey` / `SessionSelectedAccountIDStateKey`**，故进入 account_select 时仍能从 session state 复用已选账号、无需重选。
  → **强约束**：清除动作必须**精确到 `constant.StateKeyForwardedResumeValue` 这一个 key**，严禁写成「清空 graph.State」或「清除所有 resume 相关 key」，否则会误删同处 graph state 的 `SessionAccountIDTempKey` 而破坏 LLM 占位符注入。

- **[Trade-off vs 当前 resumeValue 方案]**：当前已落地的 `resumeValue` 方案把「本次操作」绑在 resume command 上，框架保证确定性，**零状态机依赖、无需消费后清除**。本方案（方案1）恢复了设计理念上的来源区分，但代价是：① 回退 MR 3191 对 `after_tool_hitl` 的改动；② 引入对「checkpoint 清除 + merge 补位」机制的依赖（循环边脆弱性）；③ 所有读 `state[StateKeyForwardedResumeValue]` 的中断节点都需遵守「读完即清」，任一遗漏即复活脏读（`after_tool_hitl` 是唯一读 state 这份的节点，循环边内每次都清即可，风险可控；`account_select`/`hitl` 读的是 RuntimeState 那份，不受影响）。
  → **建议**：实施前与用户确认「恢复来源区分的设计价值」是否高于「resumeValue 方案的零状态机依赖简单性」。本方案按用户决策落地。

- **[Trade-off] 自由文本为 JSON 时不再误判**：移除 JSON 嗅探后，自由文本即使是合法 JSON，也因其未写入 `state[StateKeyForwardedResumeValue]`，会走 `resumeValue` 自由文本通道，不再误发 `resume_forwarded` 事件——行为更正确。

## Migration Plan

1. 修改 `aftertool/node.go`：`doInterrupt` 恢复「读 `state[StateKeyForwardedResumeValue]` 优先、否则 `resumeValue`」逻辑，移除 JSON 嗅探，消费后返回清除该 key 的 delta（`nil` 或空串）。
2. 调整 `service.go` 注释对齐语义（行为不变）。
3. 补充单测：覆盖「有 forwarded → 发事件并采用」「无 forwarded → 自由文本回退」「消费后下一 checkpoint state 不含旧 forwarded（同轮二次中断读到本轮值）」三类场景。

**Rollback**: 改动局部且独立，回退 `aftertool/node.go` 即可恢复 MR 现状；无数据迁移依赖。

## Open Questions

### #1 [已解决] `graph.State{key: nil}` 不能清除 key → 改用入口剥键 + 读 RuntimeState

在 trpc-agent-go **v1.8.1** 已核实：`mergeInitialStateNonInternal`（executor.go:696）用 `exists(key)` 判定；`StateSchema.ApplyUpdate`（state.go:297）默认 `result[key] = deepCopyAny(nil)`；`createCheckpointFromState`/`safeClone` 不跳过 nil；框架仅在 `delete(state, key)` 处真正删 key（如 input mapper 的 `delete(child, CfgKeyCheckpointID)`），节点返回值层无删除机制。

**结论与采用**：原始「消费后返回 `{key: nil}`」无效，已弃用。最终方案采用「入口剥键（`makeSubgraphInputMapper` 处 `delete(child, StateKeyForwardedResumeValue)`）+ 节点读 `inv.RunOptions.RuntimeState`」——等价达成「消费后清除」目标，但实现于框架真正支持 `delete` 的层级。

### #2 [已解决] 子图循环边同轮多次恢复时，`inv.RunOptions.RuntimeState` 是否被重新合入

**答案：可靠，方向 A 对循环节点成立。** 事实链：HITL 的每一次中断恢复都是一次新的 HTTP Run（`service.go: tryPrepareAutoResume` 在每次 Run 时重新执行、重建 `inv.RunOptions.RuntimeState`，含当轮 `StateKeyForwardedResumeValue` 与 resume command）。`after_tool_hitl → llm` 循环边虽在单次 Run 的 executor 内部、不重新跑 `tryPrepareAutoResume`，但循环边处理的仍是同一轮 choice，前端不会在循环边内部重发 forwarded，故 RuntimeState 不变恰是正确行为。每次子图恢复（跨 Run）的 `initial` 都含当轮 RuntimeState，配合 Decision 1 的入口 `delete` 保证 `restored` 缺该 key，`mergeInitialStateNonInternal` 用当轮值补位 → `after_tool_hitl` 读 RuntimeState 对循环节点同样可靠，与 `account_select`/`hitl` 一致。

### #3 存量 checkpoint 清理

存量的、已固化旧 `forwarded_resume_value` 的子图 checkpoint 是否需要一次性清理？（倾向否：新 namespace 按消息条数隔离，且新写入的检查点不含该 key，旧数据不影响新轮次恢复。）
