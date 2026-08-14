## 1. 常量与状态键定义

- [x] 1.1 在 `constant` 包新增子图轮次序号 state key 常量（`StateKeySubgraphTurnPrefix`），与 `StateKeyForwardedResumeValue` 等并列集中管理；并新增 `SessionRidStateKey` 供 runOptionResolver 把 rid 注入 RuntimeState 供 mapper 日志串联。
- [x] 1.2 在 `graph_build.go` 中定义按 `nodePrefix` 隔离的 turn key 辅助函数 `subgraphTurnKey(nodePrefix)` 与 `subgraphTurnFromState(state, nodePrefix)`（兼容 int/float64 反序列化），保证 `host_apply` 与 `resource_query` 使用独立命名空间。

## 2. InputMapper 改造

- [x] 2.1 修改 `makeSubgraphInputMapper`：从父图 state 读取当前 `turn`（缺失默认 0），`turn++` 后写入 child state 的 turn key，并据此生成 `child[graph.CfgKeyCheckpointNS] = fmt.Sprintf("%s_%d", nodePrefix, turn)`，替换原 `len(msgs)` 逻辑。
- [x] 2.2 turn 写入 child state 随子图 checkpoint 持久化（JSON 序列化，存放在非内部 key `subgraph_turn_<prefix>`，不被 `isFrameworkInternalStateKey` 过滤掉），并经 output mapper 回填父图保证跨轮递增。
- [x] 2.3 更新 `makeSubgraphInputMapper` 顶部注释：将"进入子图时的消息条数"改为"严格单调递增的轮次序号 turn"，并说明同轮重入由框架 `applyCheckpointResumeFields` 用中断记录的 ns 覆盖、不重复 +1。

## 3. 日志与可观测性

- [x] 3.1 调整 `logSubgraphInput` 调用参数：传入 `rid`、`turn` 与生成的 ns，确保 `[hcm graph trace]` 日志可打印 `checkpoint_ns=<prefix>_<turn> turn=<n> rid: <rid>` 便于串联验证；`output` 侧日志同步补充 `rid` 与 `parent_turn`。
- [x] 3.2 确认 `logSubgraphOutputMerge` / `logSubgraphOutputSkip` 等输出侧日志的 `parentLen` 基准（`len(parentMsgs)`）无需改动；ns 变更不影响 delta 计算（delta 仍按 `len(decoded) > parentLen` 切片）。

## 4. 编译与验证

- [x] 4.1 相关包 `go build` / `go vet` 通过（agent、service 包 vet 干净）；`go test ./cmd/agent-server/logics/agent/ -run Subgraph|MakeSubgraph` 全部 PASS。注：完整二进制 `go build ./cmd/agent-server/...` 因环境缺 sqlite3 CGO 链接库报 `_sqlite3_*` undefined（仓库既有环境问题，与本次改动无关）。
- [x] 4.2 新增单测 `TestMakeSubgraphInputMapperUsesTurnNamespace`：验证同一会话两轮父图消息数相同但最终 ns 为 `host_apply_1` / `host_apply_2`（严格递增、不碰撞）。
- [x] 4.3 新增单测 `TestMakeSubgraphOutputMapperFeedsTurnBackToParent`：验证 output mapper 将子图最终态的 turn 经 RawStateDelta 回填父图，使下一轮 input mapper 续递增；hitl 中断恢复路径 ns 由 `applyCheckpointResumeFields` 覆盖为原 `host_apply_<turn>`，turn 不重复 +1（框架既有行为，未改动）。
