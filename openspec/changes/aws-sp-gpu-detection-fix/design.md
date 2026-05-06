## Context

AWS Savings Plans 月任务处理分两个阶段：
1. **Pull**：从 Athena 拉取 SP 相关账单数据，生成 `RawBillItem` 列表
2. **Split**：将 `RawBillItem` 分配给对应的二级账号，生成 `BillItemCreateReq`

后续 OBS 同步阶段（`doSyncAwsBillItem`）读取 BillItem 的 `Extension.LineItemProductCode` 和 `Extension.ProductInstanceType` 来判断是否 GPU 资源（`isAwsGPU()`）。

当前问题：Pull 阶段调用 `GetRootAccountSpTotalUsage`，其底层 SQL 无 GROUP BY，将所有 `SavingsPlanCoveredUsage` 行聚合为单一总量，丢失了各行的 `product_instance_type`/`line_item_product_code` 信息。最终生成的 BillItem Extension 中这两个字段为空，`isAwsGPU()` 永远返回 `false`。

业务约束：
- `spArnPrefix` 在实际部署中始终配置，查询结果行数可控
- GPU/非GPU 费用必须分开，不能合并为同一条目
- `HcProductCode` 字段用于系统内路由过滤（`GetHcProductCodes()` 返回 SP 专属常量），不能改变

## Goals / Non-Goals

**Goals:**
- SP reverse 账单条目携带真实的 `Extension.LineItemProductCode` 和 `Extension.ProductInstanceType`，使 OBS sync 的 GPU 识别正确工作
- GPU 与非 GPU 的 SP 反转费用分为独立条目上报
- 不破坏现有 `HcProductCode = "SavingsPlanCostReverse"` 的路由逻辑

**Non-Goals:**
- 修改 OBS sync 层的 `isAwsGPU()` 逻辑
- 修改其他云厂商或其他 Month Task 类型

## Decisions

### 决策 1：新增专用 Athena SQL 而非复用 `AwsListRootBillItems`

**选择**：新增 `GetRootSpCoveredUsageByType` adaptor 方法，使用专用 SQL

**理由**：
- `AwsListRootBillItems` 底层使用 `QueryRootBillGroupBySQL`，其 GROUP BY 包含 `identity_line_item_id`（行级主键），聚合粒度过细，SP 覆盖同一实例类型的多条记录不会合并
- SP ARN 前缀过滤需要 `LIKE` 条件，现有 `FieldsMap` 只支持 `IN`，需额外改造
- 专用 SQL 仅 SELECT 5 个字段，查询开销更小

**新 SQL**：
```sql
SELECT
    line_item_product_code,
    product_instance_type,
    product_product_name,
    line_item_currency_code,
    SUM(savings_plan_net_savings_plan_effective_cost) AS sp_net_cost
FROM {db}.{table}
WHERE line_item_line_item_type = 'SavingsPlanCoveredUsage'
  AND bill_payer_account_id = '{payerCloudID}'
  AND year = '{year}' AND month = '{month}'
  AND date(line_item_usage_start_date) >= date '{year}-{month}-{startDay}'
  AND date(line_item_usage_start_date) <= date '{year}-{month}-{endDay}'
  AND savings_plan_savings_plan_a_r_n LIKE '{spArnPrefix}%'
GROUP BY line_item_product_code, product_instance_type,
         product_product_name, line_item_currency_code
```

**备选方案**：扩展 `AwsListRootBillItems` 加 `LikeFieldsMap` 字段 → 改动现有接口影响面大，且聚合粒度仍不匹配需求，故放弃

---

### 决策 2：`HcProductCode` 保持 `"SavingsPlanCostReverse"`，真实字段通过 Extension 透传

**选择**：`RawBillItem.HcProductCode` 和 `HcProductName` 保持 `AwsSavingsPlansCostCodeReverse`；真实的 `product_code` 和 `instance_type` 写入 `Extension.LineItemProductCode` / `Extension.ProductInstanceType`

**理由**：
- `HcProductCode` 是 Split 阶段过滤 `RawBillItem` 的依据（`GetHcProductCodes()`），改变它会导致这批条目无法被正确识别和处理
- OBS sync 的 `isAwsGPU()` 读取的是 `item.Extension`（即 `AwsRawBillItem` 结构体），与 `HcProductCode` 无关
- 两个字段职责分离：`HcProductCode` 用于内部路由，`Extension` 字段用于云端数据透传

---

### 决策 3：Pull 返回 N 个 RawBillItem，Split 返回 N 个 BillItem

**选择**：Pull 为每个 `(product_code, instance_type)` 组合创建一个 `RawBillItem`；`splitSpReverseExpense` 为每个 rawItem 创建一个 `BillItemCreateReq`

**理由**：
- GPU 和非 GPU 需要分开，必须是独立条目
- 各条目的 `Cost` 均为 SP net cost 的负值（反转），各条目之和等于原总量，数据完整性不变
- Split 中 SP 账号的 `ProductID`、`BkBizID` 来源于同一个 summary 查询，N 个条目均使用同一来源，无需额外查询

---

### 决策 4：扩展 `convAwsBillItemExtension` 签名

**选择**：为 `convAwsBillItemExtension` 新增 `productInstanceType` 参数，写入 `billcore.AwsRawBillItem.ProductInstanceType`

**理由**：
- `AwsRawBillItem` 已有 `ProductInstanceType` 字段（`json:"product_instance_type,omitempty"`），无需新增结构体
- 统一在已有函数扩展，避免引入新的帮助函数增加复杂度

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| SP 覆盖产品类型过多，N 值较大（如覆盖数百种实例类型）导致查询/存储压力增加 | `spArnPrefix` 总是配置，实际上限制了 SP 范围；通常一个 SP ARN 覆盖的实例类型在几十个量级内，可接受 |
| 现有数据（已入库的 SP reverse BillItem）`ProductInstanceType` 仍为空 | 此为历史数据问题，新任务执行后的数据将自动修正；历史数据可通过重新执行月任务补录 |
| `sp_net_cost` 分组求和浮点精度问题 | 使用 `shopspring/decimal` 解析，与现有代码一致，精度可控 |
