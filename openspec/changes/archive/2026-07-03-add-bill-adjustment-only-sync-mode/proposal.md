# Change: OBS 账单同步新增「只同步调账」模式

## Why

对外（OBS）账单同步固定两阶段串行执行：先把 `account_bill_item` 明细全量推送到 OBS 独立库（耗时大头），再推送已确认调账。当仅调账数据变化需重新对外推送时，只能整月明细全量重推，耗时过长。调账同步链路本身已是独立、幂等的 Flow/Action，具备单独运行能力。

## What Changes

- 复用现有 `POST /api/v1/account/bills/sync_records` 接口，新增 `sync_mode` 入参（`full` 默认 / `adjustment_only`），不新增接口。
- `adjustment_only` 模式沿用与 `full` 完全相同的准入校验（全部一级汇总须 `confirmed`）与 `BatchSyncBillSummaryRoot` 副作用，仅在调度阶段跳过明细推送 Flow：逐二级账号仅做 `COUNT` 计数后直接置 `synced`，随后复用现有调账同步 Flow 与 `notifyObs`。
- 新增 `BillSyncMode` 枚举（含 `Validate()`）。
- `account_bill_sync_record` 表新增 `sync_mode` 列（默认 `'full'`），并在 account-server 入参、data-service 创建入参与落库、core 结构体、表结构体全链路透传。
- 更新运营管理视角接口文档，补充 `sync_mode` 字段（版本 v9.9.9+）。
- 本次不改前端，仅后端支持 `sync_mode` 入参（前端后续单独处理）。

## Impact

- Affected specs: `bill-obs-sync-mode`（新增 capability）
- Affected code:
  - `pkg/criteria/enumor/bill.go`（新增 `BillSyncMode`）
  - `pkg/api/account-server/bill/billsyncrecord.go`（`BillSyncRecordCreateReq` 增 `SyncMode`）
  - `cmd/account-server/service/bill/billsyncrecord/sync.go`（创建记录透传 `SyncMode`）
  - `cmd/account-server/logics/bill/sync_controller.go`（调度按模式分流，核心）
  - `pkg/api/data-service/bill/billsyncrecord.go`、`cmd/data-service/service/bill/billsyncrecord/create.go`（透传 + 落库）
  - `pkg/dal/table/bill/billsyncrecord.go`、`pkg/api/core/bill/billsyncrecord.go`（数据模型透传）
  - `scripts/sql/9999_*.sql`（DDL，`account_bill_sync_record` 增 `sync_mode`）
  - `docs/api-docs/web-server/docs/scr/`（接口文档）
