## Context

OBS 账单同步由 `SyncController`（account-server，master 节点 10s 轮询）驱动，处理 `account_bill_sync_record`：
1. `initSyncItem`：按 `vendor+year+month` 展开所有二级账号到 `detail`（每项一个 `SyncRecordDetailItem`）。
2. 逐 item：`stateNew → setTotal`（`COUNT`）→ `stateSyncing → doSubSyncTask`（按 5 万/批创建 `FlowObsSyncBillItem` 推送明细）→ `synced`。
3. 全部 item `synced` 后 `handleAdjustment`（`FlowObsSyncAdjustment` + `SyncAdjustmentAction`，按 `set_index="adjustment"+year+month` 先 clean 再重写，幂等）。
4. `notifyObs`：累加 `detail.Total` 得 `totalCount`，`sum` 取 `syncRecord.Cost/RMBCost`，调用 `NotifyRePull`。

调账通路与云上明细通路独立，调账 Flow/Action 已完全具备独立运行能力。

## Goals / Non-Goals

- Goals：新增 `adjustment_only` 模式，跳过明细全量推送 Flow，仅重推调账，大幅缩短耗时；对 `full` 模式与存量调用/数据零影响。
- Non-Goals：不新增接口、不新增云适配/Action、不改调账 Flow/Action、不改 `notifyObs` 上报口径、本次不改前端。

## Decisions

- **Decision: 入口参数 + 调度分支**，复用现有接口与调账链路。改动集中在 `sync_mode` 全链路透传与 `SyncController` 分流。
- **Decision: 准入门槛与 full 完全一致**。`adjustment_only` 仍执行 `collectAllBillSummaryRoot`（要求全部一级汇总 `confirmed`，采集含 `AdjustmentCost` 的 cost）与 `BatchSyncBillSummaryRoot`，不放宽状态。
- **Decision: 保留明细计数以对齐 notify 口径**。`adjustment_only` 仍 `initSyncItem` 展开 detail 并对每个 item 执行 `setTotal`（`COUNT`，仅计数不搬数据），随后**直接置 `synced`**，跳过 `doSubSyncTask`/`createSyncBillItemFlow`。`notifyObs` 原样复用（`totalCount` 由 `detail.Total` 累加、`sum` 取 cost）。
- **Decision: 模式透传方式**。`SyncRecord.SyncMode` 由 controller 读取；`advanceSyncItems` 已持有 `syncRecord`，将 `syncRecord.SyncMode` 透传给 `handleSyncRecordDetailItem`，在 `stateNew` 分支按模式决定 `setTotal` 后是置 `stateSyncing`（full）还是 `stateSynced`（adjustment_only）。
- **Decision: DDL 默认值 `'full'`**，存量数据自动等价 full，平滑兼容。`sync_mode` 仅创建时写入，创建后不更新，故 update 链路无需改动。
- Alternatives considered:
  - 新增独立「只同步调账」接口 —— 冗余，调账链路已可复用，弃用。
  - `adjustment_only` 跳过 `COUNT` 计数 —— 会导致 `notifyObs` 的 `total` 口径与 full 不一致，弃用。

## Risks / Trade-offs

- **并发名额**：`advanceSyncItems` 仅在 `new→syncing` 时占用并发名额。`adjustment_only` 走 `new→synced`，`syncingCount` 恒为 0，所有 item 一个 tick 内跑完，不受并发上限限制。
- **COUNT 开销**：二级账号极多时 `COUNT` 次数较多，但远低于明细全量搬运（5 万/批写 OBS）成本。
- **调账 clean 幂等**：调账 Flow 先删后写，可安全重复执行。

## Migration Plan

1. 上线 DDL：`account_bill_sync_record` 增 `sync_mode varchar` 默认 `'full'`（存量行自动补 `'full'`）。
2. 后端发布：缺省 `sync_mode` 按 `full`，行为与现状一致。
3. 回滚：`sync_mode` 列可保留（默认 full 无副作用）；代码回滚后旧逻辑忽略该列。