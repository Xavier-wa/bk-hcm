# bill-obs-sync-mode Specification

## Purpose
TBD - created by syncing change add-bill-adjustment-only-sync-mode. Update Purpose after archive.

## Requirements
### Requirement: 同步模式入参

创建 OBS 账单同步记录接口 `POST /api/v1/account/bills/sync_records` SHALL 支持可选入参 `sync_mode`，取值为 `full` 或 `adjustment_only`，缺省按 `full` 处理。非法取值 MUST 返回参数校验错误。

#### Scenario: 缺省 sync_mode 按 full 处理

- **WHEN** 调用方创建同步记录且未传 `sync_mode`
- **THEN** 系统按 `full` 模式创建同步记录，`account_bill_sync_record.sync_mode` 落库为 `full`

#### Scenario: 显式传入 adjustment_only

- **WHEN** 调用方创建同步记录且 `sync_mode=adjustment_only`
- **THEN** 系统按 `adjustment_only` 模式创建同步记录，`sync_mode` 落库为 `adjustment_only`

#### Scenario: 非法 sync_mode 被拒绝

- **WHEN** 调用方传入 `sync_mode` 为 `full`/`adjustment_only` 之外的值
- **THEN** 系统返回参数校验失败（InvalidParameter），不创建同步记录

### Requirement: 同步模式准入门槛一致

`adjustment_only` 模式 SHALL 沿用与 `full` 模式完全相同的准入校验与副作用：要求该 `vendor+year+month` 下全部一级账单汇总状态为 `confirmed`，采集含调账金额的 `cost/rmb_cost`，并触发 `BatchSyncBillSummaryRoot` 重新核算。

#### Scenario: 存在未确认的一级汇总时拒绝创建

- **WHEN** 以 `adjustment_only` 创建同步记录，但该月存在非 `confirmed` 状态的一级账单汇总
- **THEN** 系统拒绝创建并返回错误，与 `full` 模式行为一致

#### Scenario: 校验通过后触发重新核算

- **WHEN** 以 `adjustment_only` 创建同步记录且全部一级汇总为 `confirmed`
- **THEN** 系统采集 `cost/rmb_cost`（含 `adjustment_cost`）落库，并调用 `BatchSyncBillSummaryRoot`

### Requirement: adjustment_only 调度跳过明细推送

在 `adjustment_only` 模式下，`SyncController` 调度 SHALL 展开二级账号明细并对每个明细执行 `COUNT` 计数后直接置为 `synced`，跳过创建明细推送 Flow（`FlowObsSyncBillItem`），随后复用现有调账同步 Flow 与 OBS 通知。`full` 模式行为保持不变。

#### Scenario: adjustment_only 不创建明细推送 Flow

- **WHEN** `SyncController` 处理 `sync_mode=adjustment_only` 的同步记录
- **THEN** 每个二级账号明细经 `COUNT` 计数后直接标记为 `synced`，不创建 `FlowObsSyncBillItem` 明细推送任务

#### Scenario: 明细完成后执行调账同步并通知 OBS

- **WHEN** `adjustment_only` 记录的全部明细已标记 `synced`
- **THEN** 系统执行调账同步 Flow（`FlowObsSyncAdjustment`），成功后调用 `NotifyRePull`，最终将记录状态置为 `synced`

#### Scenario: OBS 上报口径与 full 一致

- **WHEN** `adjustment_only` 记录调用 `NotifyRePull`
- **THEN** 上报的 `total` 为各明细 `COUNT` 计数之和，`sum` 取同步记录的 `cost/rmb_cost`，与 `full` 模式口径一致

#### Scenario: full 模式行为不变

- **WHEN** `SyncController` 处理 `sync_mode=full`（含存量缺省）的同步记录
- **THEN** 系统按原有两阶段流程逐账号分批推送明细后再同步调账

### Requirement: sync_mode 全链路透传与兼容

`sync_mode` SHALL 在 account-server 入参、data-service 创建入参与落库、core 结构体、表结构体之间透传。`account_bill_sync_record` 表 MUST 新增 `sync_mode` 列，默认值 `'full'`，保证存量数据与存量调用方零影响。

#### Scenario: 存量数据视为 full

- **WHEN** 读取升级前已存在的 `account_bill_sync_record` 记录
- **THEN** 其 `sync_mode` 为默认值 `full`，调度按 `full` 模式处理
