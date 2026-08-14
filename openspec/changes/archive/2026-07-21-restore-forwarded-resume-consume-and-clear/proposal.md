## Why

MR 3191 为支持「主机申领子图（host_apply）」重构了 `after_tool_hitl` 恢复逻辑：把前端结构化回复 `StateKeyForwardedResumeValue` 同时写进 `inv.RunOptions.RuntimeState` **和** resume command，并让节点改为**只信任 `graph.Interrupt` 返回的 `resumeValue`**，用「是否为 JSON」嗅探区分结构化 vs 自由文本。

这造成了与设计理念的不一致：`StateKeyCommand`（resume command）= 用户自由输入；`forwardedProps.resumeValue` / `StateKeyForwardedResumeValue` = 前端结构化协议（选择方案、选账号）。MR 后把二者统一塞进 `resumeValue` 通道、靠 JSON 嗅探分流，实际上**抹掉了字段来源层面的区分**；且自由文本若恰好是合法 JSON，会被误判为结构化选择并发 `after_tool_hitl.resume_forwarded` 事件。

`agent-after-tool-hitl` spec 仍约定「恢复时优先采用 `StateKeyForwardedResumeValue` 的结构化回复，否则回退自由文本」。本次改动希望**恢复这一区分设计**（command=自由输入 / forwarded=结构化协议），同时通过「**消费后把 `state` 中的 `StateKeyForwardedResumeValue` 清除**」来避免子图循环边同轮多次中断恢复时的脏数据——而非像 MR 那样退化为 JSON 嗅探。

脏数据的根因（已在历史分析中确认）：`StateKeyForwardedResumeValue` 经 `mergeInitialStateNonInternal` 被合进**持久化的 graph `state`**，且合并规则是 **checkpoint 优先**——`after_tool_hitl → llm` 循环边同轮多次中断时，旧 checkpoint 里持久化的旧值会压制 `tryPrepareAutoResume` 本次写入的新值，导致第二次恢复读到第一次的选择。`account_select`/`hitl` 读的是 `inv.RunOptions.RuntimeState`（请求级、每 resume 由父图重新 invoke 重建、**不脏**），但 `after_tool_hitl` 是子图**内部循环节点**，不满足该前提，必须走另一条干净通道。

## What Changes

- **恢复区分语义**：`after_tool_hitl`（`doInterrupt`）恢复「优先读 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`（forwarded 结构化）→ 未命中回退 `graph.Interrupt` 自由文本 `resumeValue`」逻辑；命中 forwarded 时发 `after_tool_hitl.resume_forwarded` 自定义事件、以该值作为用户选择；未命中时回退自由文本且校验非空。移除当前的「JSON 嗅探」分支。
- **入口剥键（取代「消费后清除」）**：在 `makeSubgraphInputMapper` 处 `delete(child, StateKeyForwardedResumeValue)`，从源头阻止该 key 进入子图持久化 `state`。由于 `mergeInitialStateNonInternal` 的合并语义为 checkpoint 优先、请求态补位（`exists(key)` 判定），子图 state 永远不含该 key → 每次恢复经 `mergeInitialStateNonInternal` 用当轮 `tryPrepareAutoResume` 重建的 `RuntimeState` 补位 → 读到本轮值，从而消除脏读。**这等价于「消费后清除」的目标，但实现于框架真正支持 `delete` 的层级**——v1.8.1 已证实节点返回值 `{key: nil}` 无法删除 key（key 仍 exists、脏读复活），故不再采用「消费后返回 `{nil}`」写法。
- **读请求级 RuntimeState**：`after_tool_hitl` 改读 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`。HITL 每次中断恢复都是新 HTTP Run，`tryPrepareAutoResume` 每次重建 RuntimeState（含当轮 forwarded 与 resume command），故子图循环节点读 RuntimeState 同样可靠，与 `account_select`/`hitl` 一致。
- **双通道注入维持**：`tryPrepareAutoResume` 维持现状——`forwarded` 同时写入 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]`（供 `account_select`/`hitl`/`after_tool_hitl` 读，请求级不脏）与 resume command（供 `after_tool_hitl` 的 `resumeValue` 自由文本回退）。

## Capabilities

### New Capabilities
<!-- 无新增能力 -->

### Modified Capabilities
- `agent-after-tool-hitl`: 修改 after_tool_hitl 恢复语义——恢复「优先 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]` 结构化回复、否则回退自由文本」的规范行为，并配套在子图入口 `makeSubgraphInputMapper` 剥离该 key（避免循环边同轮多次中断恢复脏读）；不再依赖「消费后返回 `{nil}` 删除 key」（框架不支持）。

## Impact

- **Affected code**:
  - `cmd/agent-server/logics/agent/graph_build.go`：`makeSubgraphInputMapper` 增加 `delete(child, constant.StateKeyForwardedResumeValue)`（紧邻已有 `delete(child, graph.CfgKeyCheckpointID)`）。
  - `cmd/agent-server/logics/agent/aftertool/node.go`：`doInterrupt` 恢复「读 `inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]` 优先、否则 `resumeValue`」逻辑，移除 JSON 嗅探；不再返回清除该 key 的 delta。
  - `cmd/agent-server/service/service.go`：`tryPrepareAutoResume` 维持双通道（行为不变，仅注释对齐语义）。
- **APIs**：AGUI 事件 `after_tool_hitl.resume_forwarded` 恢复为「命中 forwarded 结构化值时才发」（与 spec 一致），不再因自由文本是 JSON 而误发。
- **Dependencies**：依赖 trpc-agent-go `mergeInitialStateNonInternal` 的「checkpoint 优先、缺 key 时请求态补位」语义，以及 `tryPrepareAutoResume` 每次 Run 重建 `RuntimeState` 的事实。
- **Systems**：主机申领子图（host_apply）的 `after_tool_hitl` 中断恢复链路。
