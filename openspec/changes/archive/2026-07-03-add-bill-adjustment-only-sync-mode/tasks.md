## 1. 枚举

- [x] 1.1 `pkg/criteria/enumor/bill.go` 新增 `BillSyncMode` 类型与常量 `BillSyncModeFull`（`full`）、`BillSyncModeAdjustmentOnly`（`adjustment_only`），并实现 `Validate()`

## 2. 数据模型

- [x] 2.1 `pkg/dal/table/bill/billsyncrecord.go`：`AccountBillSyncRecord` 增 `SyncMode` 字段（`db:"sync_mode"`），`AccountBillSyncRecordColumnDescriptor` 增 `sync_mode` 列（`enumor.String`）
- [x] 2.2 `pkg/api/core/bill/billsyncrecord.go`：`SyncRecord` 增 `SyncMode enumor.BillSyncMode`

## 3. data-service

- [x] 3.1 `pkg/api/data-service/bill/billsyncrecord.go`：`BillSyncRecordCreateReq` 增 `SyncMode`（omitempty）
- [x] 3.2 `cmd/data-service/service/bill/billsyncrecord/create.go`：构造 `AccountBillSyncRecord` 时映射 `SyncMode`（缺省回退 `full`）

## 4. account-server 接口

- [x] 4.1 `pkg/api/account-server/bill/billsyncrecord.go`：`BillSyncRecordCreateReq` 增 `SyncMode`（omitempty），`Validate()` 中校验 `SyncMode`（缺省视为 `full`）
- [x] 4.2 `cmd/account-server/service/bill/billsyncrecord/sync.go`：创建 `dsbill.BillSyncRecordCreateReq` 时透传 `SyncMode`（缺省 `full`），其余逻辑不变

## 5. 调度（核心）

- [x] 5.1 `cmd/account-server/logics/bill/sync_controller.go`：`advanceSyncItems` 将 `syncRecord.SyncMode` 透传给 `handleSyncRecordDetailItem`
- [x] 5.2 `handleSyncRecordDetailItem` 在 `stateNew` 分支按模式处理：`adjustment_only` 时 `setTotal` 后直接置 `stateSynced`（跳过明细 Flow），`full` 维持原 `stateSyncing`
- [x] 5.3 确认 `handleAdjustment` / `notifyObs` / `initSyncItem` 无需改动，adjustment_only 复用

## 6. DDL

- [x] 6.1 新增 `scripts/sql/9999_20260804_1147_account_bill_sync_record_sync_mode.sql`：`ALTER TABLE account_bill_sync_record ADD COLUMN sync_mode varchar(32) NOT NULL DEFAULT 'full'`，`SQLVER=9999`、`HCMVER=v9.9.9`，并更新 `hcm_version` 视图

## 7. 文档

- [x] 7.1 `docs/api-docs/web-server/docs/resource/bill/sync_bill_record.md` 更新创建同步记录接口，补充 `sync_mode` 字段说明，版本 v9.9.9+

## 8. 验证

- [x] 8.1 `go build ./cmd/... ./pkg/...` 编译通过
- [x] 8.2 `openspec validate add-bill-adjustment-only-sync-mode --strict` 通过
