## 1. 枚举与基础类型

- [x] 1.1 在 `pkg/criteria/enumor/bill.go` 中新增 `BillAdjustmentResClass` 枚举类型，定义 `BillAdjustmentResClassCPU = "cpu"` 和 `BillAdjustmentResClassGPU = "gpu"` 常量，实现 `Validate()` 方法
- [x] 1.2 在 `pkg/criteria/enumor/bill.go` 中新增 `BillAdjustmentResClassNameMap`，映射 `cpu→"CPU"`，`gpu→"GPU"`

## 2. 数据库层

- [x] 2.1 在 `pkg/dal/table/bill/billadjustmentitem.go` 中，`AccountBillAdjustmentItemColumnDescriptor` 追加 `res_class` 列描述符
- [x] 2.2 在 `pkg/dal/table/bill/billadjustmentitem.go` 中，`AccountBillAdjustmentItem` 结构体新增 `ResClass enumor.BillAdjustmentResClass` 字段（`db:"res_class"`）
- [x] 2.3 编写 SQL 迁移文件，为 `account_bill_adjustment_item` 表添加 `res_class` 列（`VARCHAR(32) NOT NULL DEFAULT ''`）

## 3. Core Model

- [x] 3.1 在 `pkg/api/core/bill/billIadjustment.go` 中，`AdjustmentItem` 结构体新增 `ResClass enumor.BillAdjustmentResClass` 字段

## 4. Data-Service API 与服务层

- [x] 4.1 在 `pkg/api/data-service/bill/billadjustmentitem.go` 中，`BillAdjustmentItemCreateReq` 新增 `ResClass enumor.BillAdjustmentResClass` 字段
- [x] 4.2 在 `pkg/api/data-service/bill/billadjustmentitem.go` 中，`BillAdjustmentItemUpdateReq` 新增 `ResClass enumor.BillAdjustmentResClass` 字段（可选）
- [x] 4.3 在 `cmd/data-service/service/bill/billadjustmentitem/create.go` 中，将 `req.ResClass` 映射到 DB 结构体 `ResClass` 字段
- [x] 4.4 在 `cmd/data-service/service/bill/billadjustmentitem/update.go` 中，将 `req.ResClass` 映射到 DB 更新结构体

## 5. Account-Server API 与服务层

- [x] 5.1 在 `pkg/api/account-server/bill/billadjustmentitem.go` 中，`BillAdjustmentItemCreateReq` 新增 `ResClass enumor.BillAdjustmentResClass` 字段（`validate:"required"`）
- [x] 5.2 在 `pkg/api/account-server/bill/billadjustmentitem.go` 中，`BillAdjustmentItemUpdateReq` 新增 `ResClass enumor.BillAdjustmentResClass` 字段（可选）
- [x] 5.3 在 `cmd/account-server/service/bill/billadjustment/bill_adjustment.go` 中，`convBillAdjustmentCreate()` 透传 `ResClass` 到 data-service 请求
- [x] 5.4 在 `cmd/account-server/service/bill/billadjustment/bill_adjustment.go` 中，`UpdateBillAdjustmentItem()` 透传 `ResClass` 到 data-service 请求

## 6. Excel 导出

- [x] 6.1 在 `cmd/account-server/logics/bill/export/billadjustment.go` 中，`BillAdjustmentTable` 新增 `ResClass string` 字段（`header:"资源类型"`）
- [x] 6.2 在 `cmd/account-server/service/bill/billadjustment/export.go` 中，`toRawData()` 函数填充 `ResClass` 字段值（使用 `enumor.BillAdjustmentResClassNameMap[detail.ResClass]`）

## 7. OBS 同步层

- [x] 7.1 在 `cmd/task-server/logics/action/obs/sync/sync_adjustment.go` 中，`convertAws()` 根据 `adj.ResClass` 计算 `isGPU` 并设置 OBS 条目的 `ResClassId`（调用 `enumor.GetOBSResClassID(adj.Vendor, isGPU)`）；同步根据 `mainAccount.Site` 设置 `CityId`
- [x] 7.2 在 `cmd/task-server/logics/action/obs/sync/sync_adjustment.go` 中，`convertHuawei()` 同样设置 `ResClassId`；同步根据 `mainAccount.Site` 设置 `CityId`
- [x] 7.3 在 `cmd/task-server/logics/action/obs/sync/sync_adjustment.go` 中，`convertGcp()` 同样设置 `ResClassId`；新增 `mainAccount.Site` 判断分支并设置 `CityId`（GCP 此前无此逻辑）
