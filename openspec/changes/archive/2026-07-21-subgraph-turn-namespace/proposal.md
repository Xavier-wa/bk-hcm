## Why

主机申领（`host_apply`）等子图通过 `makeSubgraphInputMapper` 以"进入子图时的消息条数" `host_apply_<len(msgs)>` 作为 checkpoint namespace，用来区分同一会话内不同轮次，避免下一轮误 resume 到上一轮的完成态（`__end__`）。该标识是**脆弱的**：同一会话内若两轮进入子图时父图消息条数恰好相等（Ns 碰撞），子图会命中旧 namespace 的 checkpoint 而非从入口重跑，导致消息被重复 merge、tool_call_id 重复，触发 Bedrock 的 "duplicate Ids" 报错。

根因不是 lineage（当前 `service.go:745` 已把 lineage 绑定为稳定 `input.ThreadID`），而是轮次标识使用了会回踩的消息长度。需要一种严格单调递增的轮次序号替代 `len(msgs)` 作为 namespace。

## What Changes

- 在 graph state / RuntimeState 中维护一个按"会话 + 子图"递增的轮次序号 `turn`（首次进入子图 +1，同轮重入不 +1）。
- `makeSubgraphInputMapper` 的子图 namespace 由 `host_apply_<len(msgs)>` 改为 `host_apply_<turn>`，与消息内容解耦。
- 复用框架已有的"中断恢复用中断时记录的 namespace 覆盖"机制（`applyCheckpointResumeFields`）保证单 Run 内 hitl 重跑仍走原 namespace。
- 同步更新 `makeSubgraphInputMapper` 顶部注释中对 namespace 设计意图的描述。
- output mapper 的 `parentLen` 计算基准（`len(parentMsgs)`）保持不变，无需改动。

## Capabilities

### New Capabilities
- `subgraph-turn-namespace`: 子图 checkpoint namespace 改用严格单调递增的轮次序号（turn）代替消息条数（len(msgs)），根治 Ns 碰撞导致的陈旧 checkpoint 误命中与消息重复问题。

### Modified Capabilities
<!-- 本变更只改 namespace 生成方式，不涉及对外的 spec 级行为变化，留空 -->

## Impact

- 受影响代码：`cmd/agent-server/logics/agent/graph_build.go`（`makeSubgraphInputMapper`、`logSubgraphInput` 调用与注释）。
- 依赖：框架 `trpc-agent-go@v1.8.1` 的 `applyCheckpointResumeFields`（中断恢复时按 `subgraphInterrupt` 记录的 ns 覆盖）行为不变；`CfgKeyLineageID = ThreadID` 机制不变。
- 不改动：父图 resume 链路、checkpoint 删除逻辑（明确**不**采用 `DeleteLineage`，避免整条会话 checkpoint 被误删）、output mapper delta 计算。
- 风险：turn 必须持久化进 state（否则 resume 后归零重新碰撞）；单 Run 内同轮多次进入（循环边 / hitl 重跑）不能 +1。
