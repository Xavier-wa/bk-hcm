## Why

账单调整（BillAdjustment）功能目前缺少资源类型维度，无法区分调账明细属于 CPU 还是 GPU 资源。这导致调账数据同步到 OBS 时 `ResClassId` 无法正确设置，OBS 侧无法按资源类型（CPU/GPU）进行分类统计和核算。

## What Changes

- 在账单调整表（`account_bill_adjustment_item`）新增 `res_class` 字段，枚举值：`cpu` / `gpu`
- 创建调账时该字段必填，区分调账费用归属的资源类型
- 存量数据 `res_class` 为空时，OBS 同步逻辑默认视为 CPU
- 调账明细的 Excel 导出新增"资源类型"列
- OBS 同步逻辑（AWS / 华为 / GCP）根据 `res_class` 正确设置 `ResClassId`
- Zenlayer OBS 表无 `ResClassId` 字段，无需处理

## Capabilities

### New Capabilities

- `bill-adjustment-res-class`: 账单调整资源类型（CPU/GPU）字段，覆盖枚举定义、DB 字段、创建/更新 API、导出展示、OBS 同步

### Modified Capabilities

（无需修改现有 spec）

## Impact

- **数据库**：`account_bill_adjustment_item` 表新增 `res_class` 列（DDL 变更）
- **枚举层**：`pkg/criteria/enumor/bill.go` 新增 `BillAdjustmentResClass` 类型
- **Core Model**：`pkg/api/core/bill/billIadjustment.go` 新增字段
- **Data-Service**：创建/更新 API 的 Req 结构体变更，create/update 服务层透传
- **Account-Server**：创建（必填）、更新（可选）API 新增字段，convBillAdjustmentCreate 透传
- **Export**：导出表头新增"资源类型"列，`toRawData` 填充中文名
- **OBS 同步**：`sync_adjustment.go` 中 AWS/华为/GCP 三个 convert 函数新增 ResClassId 设置逻辑；同步修复三个函数的 CityId 设置（GCP 额外新增 site 判断分支）
- **不影响**：汇总统计（sum.go）无需按 CPU/GPU 分组
