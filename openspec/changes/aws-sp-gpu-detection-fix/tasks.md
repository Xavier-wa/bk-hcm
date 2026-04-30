## 1. Adaptor 层：新增 SP 已覆盖用量按类型分组查询

- [x] 1.1 在 `pkg/adaptor/types/bill/bill.go` 中新增 `AwsRootSpCoveredUsageByTypeOpt` 结构体（含 `PayerCloudID`、`SpArnPrefix`、`Year`、`Month`、`StartDay`、`EndDay` 字段）
- [x] 1.2 在 `pkg/adaptor/types/bill/bill.go` 中新增 `AwsSpCoveredUsageByType` 结构体（含 `LineItemProductCode`、`ProductInstanceType`、`ProductProductName`、`LineItemCurrencyCode`、`SpNetCost` 字段）
- [x] 1.3 在 `pkg/adaptor/aws/bill.go` 中新增常量 `AwsSPCoveredUsageByTypeSQL`（按 product_code/instance_type/product_name/currency 分组的聚合 SQL）
- [x] 1.4 在 `pkg/adaptor/aws/bill.go` 中实现 `GetRootSpCoveredUsageByType` 方法，调用 Athena 查询并将结果解析为 `[]AwsSpCoveredUsageByType`

## 2. HC Service API 层：新增请求/响应类型

- [x] 2.1 在 `pkg/api/hc-service/bill/aws.go` 中新增 `AwsRootSpCoveredUsageByTypeReq` 请求结构体（含 `RootAccountID`、`SpArnPrefix`、`Year`、`Month`、`StartDay`、`EndDay` 以及校验 tag）
- [x] 2.2 在 `pkg/api/hc-service/bill/aws.go` 中新增 `AwsSpCoveredUsageByTypeItem` 响应条目结构体

## 3. HC Service 实现层：新增 Handler 和路由

- [x] 3.1 在 `cmd/hc-service/service/bill/aws_get.go` 中新增 `AwsGetRootSpCoveredUsageByType` Handler，完成参数解码、校验、adaptor 调用及结果返回
- [x] 3.2 在 `cmd/hc-service/service/bill/bill.go` 的 `InitBillService` 中注册新路由（GET `/vendors/aws/root_account_bills/sp_covered_usage_by_type`）

## 4. HC Service Client 层：新增调用方法

- [x] 4.1 在 `pkg/client/hc-service/aws/bill.go` 中新增 `GetRootAccountSpCoveredUsageByType` 方法，通过 `common.Request` 调用新路由

## 5. Task Server：修改 Pull 阶段

- [x] 5.1 修改 `aws_savings_plans.go` 的 `Pull` 方法：将 `GetRootAccountSpTotalUsage` 调用替换为 `GetRootAccountSpCoveredUsageByType`
- [x] 5.2 在 `Pull` 方法中，为每个返回条目构造 `RawBillItem`，将真实 `product_code`/`instance_type`/`product_name` 写入 Extension 的 `line_item_product_code`/`product_instance_type`/`product_product_name` 字段，`BillCost` 设置为 `spNetCost.Neg()`
- [x] 5.3 确保 `RawBillItem.HcProductCode` 和 `HcProductName` 保持为 `constant.AwsSavingsPlansCostCodeReverse`

## 6. Task Server：修改 Split 阶段

- [x] 6.1 修改 `aws.go` 中的 `convAwsBillItemExtension` 函数，新增 `productInstanceType` 参数并将其写入 `billcore.AwsRawBillItem.ProductInstanceType`
- [x] 6.2 修改 `aws_savings_plans.go` 的 `splitSpReverseExpense` 方法：改为循环处理每条 rawItem，为每条生成独立的 `BillItemCreateReq`（调用更新后的 `convAwsBillItemExtension`）
- [x] 6.3 在 `splitSpReverseExpense` 中，从每条 rawItem 的 Extension 中解析出 `line_item_product_code` 和 `product_instance_type`，透传给 `convAwsBillItemExtension`
