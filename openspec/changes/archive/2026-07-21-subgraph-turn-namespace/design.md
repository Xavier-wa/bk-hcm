## Context

主机申领（`host_apply`）等子图在 `cmd/agent-server/logics/agent/graph_build.go` 的 `makeSubgraphInputMapper` 中，以"进入子图时的父图消息条数"作为 checkpoint namespace（`host_apply_<len(parent.messages)>`），意图是让同一会话（lineage = ThreadID）内每轮进入子图落到不同的 ns，从而下一轮不会误 resume 到上一轮留下的完成态（`__end__`）。

问题：`len(msgs)` 不是严格的轮次计数器。同一会话内若两轮进入子图时父图消息条数恰好相等（Ns 碰撞），子图 executor 用 `(ThreadID, host_apply_<N>, 空 checkpoint_id)` 取最新 checkpoint 时会命中上一轮的完成态或陈旧历史，导致消息被重复 merge、tool_call_id 重复，触发 Bedrock "duplicate Ids" 报错。

框架现状已具备的两个前提（无需改动）：
- `service.go:745` 已把 `graph.CfgKeyLineageID = input.ThreadID` 经 `WithRuntimeState` 展开进父图 initial state，lineage 跨 Run 稳定。
- 子图中断恢复由 `buildChildStateForAgentNode`（`state_graph.go:2520`）先跑 inputMapper，再用 `applyCheckpointResumeFields`（`state_graph.go:2466`）以 `subgraphInterrupt` 记录的 `childCheckpointNS` 覆盖 ns，保证单 Run 内 hitl 重跑仍走原 ns。

## Goals / Non-Goals

**Goals:**
- 用严格单调递增的轮次序号 `turn` 替代 `len(msgs)` 生成子图 namespace，彻底消除 Ns 碰撞。
- 复用框架已有的中断恢复 ns 覆盖机制，保证单 Run 内同轮重入（循环边 / hitl）不重复 +1。
- 不改动 output mapper 的 delta 计算基准（`len(parentMsgs)`）。

**Non-Goals:**
- 不改动 lineage（已是稳定 ThreadID）。
- 不引入 `DeleteLineage`/`DeleteCheckpoint` 清理完成态（框架仅提供整条 lineage 级删除，会误删同会话其他进行中的中断，方案 B 已否决）。
- 不改动父图 resume 链路、checkpoint 存储实现、其他子图（resource_query 等）以外的逻辑。

## Decisions

### 决策 1：轮次序号 `turn` 的存储位置
`turn` 必须随 checkpoint 持久化，否则子图恢复后状态里的 turn 归零会重新碰撞。将其存入子图 child state（经 `makeSubgraphInputMapper` 写入，随子图 checkpoint 一起序列化）。恢复时由 `applyCheckpointResumeFields` 用记录的 ns 覆盖，turn 的实际值不依赖恢复路径读取，只需保证"每次全新进入 +1"。

按 `<nodePrefix>` 区分不同子图（如 `host_apply` / `resource_query`），ns 形如 `host_apply_<turn>`。

### 决策 2：`turn` 的递增边界
在 `makeSubgraphInputMapper` 内：从父图 state 读当前 `turn`（首次为 0），`turn++` 后写入 child state 并生成 ns。
依据框架 `buildChildStateForAgentNode` 的调用顺序（先 inputMapper 后 ns 覆盖）：
- **全新轮次进入**：inputMapper 执行 → turn +1 → ns = `host_apply_<turn>`。
- **同轮重入**（同 Run 内 hitl 重跑，或新 Run resume 到子图节点）：`applyCheckpointResumeFields` 用 `subgraphInterrupt.childCheckpointNS` 覆盖 ns，inputMapper 虽执行但并非恢复路径的 ns 来源，且 ns 被覆盖回原始值 → 不出现重复 ns。
- 因此"inputMapper 内 +1"在单次全新进入语义下是安全的，不会因框架重跑 inputMapper 导致同轮 +1 多次（因为 ns 最终以中断记录为准）。

### 决策 3：命名常量
新增 state key 常量，如 `StateKeySubgraphTurn`（与 `constant` 包现有命名一致）。注释明确该值经子图 checkpoint 持久化、仅用于 namespace 生成。

### 备选方案（已否决）
- **方案 B（固定 ns + DeleteLineage）**：框架仅 `DeleteLineage(lineageID)` 按整条 ThreadID 删除，会清空同会话所有子图所有轮次 checkpoint，误伤其他进行中的中断 → 否决。
- **方案 C（ns 固定为子图名，靠父图驱动）**：复活"下一轮直接 resume 到 __end__"的已知 bug → 否决。

## Risks / Trade-offs

- **[Risk] turn 未持久化 → resume 后归零重新碰撞。**
  Mitigation：turn 写入 child state，随子图 checkpoint JSON 序列化；恢复路径 ns 由 `subgraphInterrupt.childCheckpointNS` 覆盖，不依赖恢复时重读 turn。
- **[Risk] 单 Run 内同轮多次进入（循环边 / hitl）导致 turn 多次 +1。**
  Mitigation：框架 `applyCheckpointResumeFields` 在同轮重入时用中断记录的 ns 覆盖，最终 ns 取记录的 `host_apply_<turn>` 而非 inputMapper 新算值；仅"从未被中断记录的、真正的全新进入"会让新 turn 生效。
- **[Risk] 多子图共用同一 turn 计数导致交叉。**
  Mitigation：ns 以 `<nodePrefix>` 区分，各子图独立 ns 空间；turn 计数按 nodePrefix 隔离存储（如 key 带上 prefix 或各自独立 key）。
- **[Trade-off] 旧 checkpoint 中 ns 为 `host_apply_<len>` 格式，升级后新 ns 为 `host_apply_<turn>`，天然不冲突；旧完成态 checkpoint 不会再被命中，无迁移兼容问题。**

## Migration Plan

- 纯逻辑变更，无数据迁移。旧 `host_apply_<len>` checkpoint 因 ns 格式变化自然失效，不影响新流程。
- 回滚：回退 `makeSubgraphInputMapper` 的 ns 生成即可恢复原 `len(msgs)` 行为。
- 验证：借助已有 `[hcm graph trace]` 日志，观察 `logSubgraphInput` 打印的 `checkpointNS` 由 `<prefix>_<len>` 变为 `<prefix>_<turn>`，且同一会话两轮进入 turn 严格递增、不重复。

## Open Questions

- turn 是否需要在 `constant` 包集中定义（与 `StateKeyForwardedResumeValue` 等并列），还是就近定义在 `graph_build.go`？倾向 `constant` 包集中管理。
