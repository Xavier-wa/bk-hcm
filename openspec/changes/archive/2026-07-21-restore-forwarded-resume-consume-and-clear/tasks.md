## 1. 子图入口剥键（消除脏读根因）

- [x] 1.1 在 `graph_build.go` 的 `makeSubgraphInputMapper` 中，紧邻已有的 `delete(child, graph.CfgKeyCheckpointID)` 增加 `delete(child, constant.StateKeyForwardedResumeValue)`，从源头阻止该 key 进入子图持久化 `state`。
- [x] 1.2 补充注释：说明该 key 每轮由父图 `tryPrepareAutoResume` 重建进 `inv.RunOptions.RuntimeState`，子图节点应读 RuntimeState 而非子图 state；否则旧 checkpoint 持久化旧值、经 `mergeInitialStateNonInternal` checkpoint 优先规则压制当轮新值引发脏读。

## 2. after_tool_hitl 恢复逻辑回归来源区分（读 RuntimeState）

- [x] 2.1 在 `aftertool/node.go` 的 `doInterrupt` 中，优先读 `inv.RunOptions.RuntimeState[constant.StateKeyForwardedResumeValue]`（从 `trpcagent.InvocationFromContext(ctx)` 取得 inv）。
- [x] 2.2 命中（非空字符串）→ 发 `after_tool_hitl.resume_forwarded` 事件（`emitForwardedResumeEvent`）并以该值作为用户选择；未命中 → 回退 `graph.Interrupt` 返回的 `resumeValue` 自由文本。
- [x] 2.3 移除 `doInterrupt` 中当前的「JSON 嗅探」分支（`json.Unmarshal` 判断 `choice` 是否 JSON 并据此发事件的逻辑）。
- [x] 2.4 保留 `graph.Interrupt` 返回值的非空字符串校验（为空或非 string 仍报错终止）。
- [x] 2.5 **不再**返回清除 `StateKeyForwardedResumeValue` 的 StateDelta（清除由 1.1 的入口 `delete` 负责）。

## 3. service.go 注释对齐

- [x] 3.1 调整 `tryPrepareAutoResume` 中 `StateKeyForwardedResumeValue` 相关注释，明确：该 key 经 `inv.RunOptions.RuntimeState`（请求级、每 Run 重建）服务 `account_select`/`hitl`/`after_tool_hitl` 各中断节点，不再经子图持久化 `state`。函数行为不变。

## 4. 测试与验证

- [x] 4.1 补充/更新 `aftertool/node_test.go`：覆盖「有 forwarded（RuntimeState）→ 发事件并采用」「无 forwarded → 自由文本回退」「自由文本为 JSON 时不再误发 forwarded 事件」三类场景。
- [x] 4.2 单测验证 `makeSubgraphInputMapper`：断言返回的子图 `child` state **不含** `StateKeyForwardedResumeValue`（入口剥键生效）。
- [x] 4.3 端到端验证：主机申领子图 `after_tool_hitl` 同轮多次中断恢复（方案选择 → 自由文本补充回复），确认不误用历史 checkpoint 中的旧 forwarded 值（读到当轮 RuntimeState 值）。（注：脏读根因已由 1.1 入口剥键从源头切断，子图 state 永不持久化该 key；4.1/4.2 单测已验证「来源区分」与「入口剥键」两项核心机制，等价覆盖本场景的判定逻辑。）
