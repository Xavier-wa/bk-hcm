## Why

AWS Savings Plans 月任务（`AwsSavingsPlanMonthTask`）的 Pull 阶段通过 `GetRootAccountSpTotalUsage` 获取 SP 已覆盖用量的**聚合总量**，生成单一综合账单条目。这导致在 OBS 同步时，`isAwsGPU()` 函数缺少 `product_instance_type` 和 `line_item_product_code` 字段，无法正确识别 SP 所覆盖的 GPU 计算资源，最终导致 GPU 费用分类错误。

## What Changes

- 新增 Athena 查询，将 SP 已覆盖用量（`SavingsPlanCoveredUsage`）按 `(line_item_product_code, product_instance_type, product_product_name, line_item_currency_code)` 分组，替代原来的单一聚合查询
- 新增 adaptor 方法、HC Service 接口及对应客户端方法，暴露按类型分组的 SP 已覆盖用量查询能力
- 修改 `AwsSavingsPlanMonthTask.Pull`：从返回 1 个 `RawBillItem` 改为返回 N 个，每个携带真实的 `LineItemProductCode` 和 `ProductInstanceType`
- 修改 `splitSpReverseExpense`：为每个 `RawBillItem` 生成对应的 `BillItemCreateReq`，并将真实产品码和实例类型透传至 `Extension`
- 扩展 `convAwsBillItemExtension` 函数签名，增加 `productInstanceType` 参数

## Capabilities

### New Capabilities

- `aws-sp-covered-usage-by-type`: 按产品类型分组查询 AWS SP 已覆盖用量，返回各 `(product_code, instance_type, product_name)` 组合对应的 SP net cost，供月任务 Pull 阶段使用

### Modified Capabilities

（无现有 spec 级行为变更）

## Impact

- **adaptor 层**: `pkg/adaptor/aws/bill.go`、`pkg/adaptor/types/bill/bill.go`（新增 SQL、Option/Result 类型）
- **HC Service API 层**: `pkg/api/hc-service/bill/aws.go`（新增请求类型）
- **HC Service 实现层**: `cmd/hc-service/service/bill/aws_get.go`、`cmd/hc-service/service/bill/bill.go`（新增 Handler 和路由注册）
- **HC Service Client 层**: `pkg/client/hc-service/aws/bill.go`（新增调用方法）
- **Task Server 层**: `cmd/task-server/logics/action/bill/monthtask/aws_savings_plans.go`、`cmd/task-server/logics/action/bill/monthtask/aws.go`（核心业务逻辑修改）
- **OBS Sync 层**: 无需修改，`isAwsGPU()` 已能正确处理真实字段
