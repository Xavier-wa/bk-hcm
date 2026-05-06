## ADDED Requirements

### Requirement: 按产品类型分组查询 AWS SP 已覆盖用量

系统 SHALL 提供按 `(line_item_product_code, product_instance_type, product_product_name, line_item_currency_code)` 分组查询 AWS Savings Plans 已覆盖用量（`SavingsPlanCoveredUsage`）的能力，返回每组的 SP net effective cost 合计，仅支持 AWS 云厂商。

查询 SHALL 支持以下过滤条件：
- 根账号云 ID（payer account）
- SP ARN 前缀（LIKE 过滤）
- 账单年份、月份
- 起始日/截止日（基于 `line_item_usage_start_date`）

#### Scenario: 查询按实例类型正确分组
- **WHEN** SP 覆盖了 GPU 实例类型（如 `p3.2xlarge`）和非 GPU 实例类型（如 `c5.xlarge`）
- **THEN** 返回结果包含两条独立记录，各自携带对应的 `line_item_product_code`、`product_instance_type` 及对应的 `sp_net_cost` 合计

#### Scenario: SP ARN 前缀过滤生效
- **WHEN** 请求中配置了 `sp_arn_prefix`
- **THEN** 仅返回 ARN 以该前缀开头的 SP 覆盖用量记录

#### Scenario: 返回结果为空时处理
- **WHEN** 指定条件下无 `SavingsPlanCoveredUsage` 记录
- **THEN** 返回空列表，不报错

---

### Requirement: SP 反转月任务 Pull 阶段按类型生成多条 RawBillItem

AWS SP 月任务的 Pull 阶段 SHALL 为每个 `(product_code, instance_type)` 组合生成独立的 `RawBillItem`，不再生成单一聚合条目。

每条 `RawBillItem` SHALL 满足：
- `HcProductCode` 和 `HcProductName` 保持为 `"SavingsPlanCostReverse"`（用于系统内路由）
- `Extension.LineItemProductCode` 设置为真实的云产品码（如 `"AmazonEC2"`）
- `Extension.ProductInstanceType` 设置为真实的实例类型（如 `"p3.2xlarge"`）
- `BillCost` 为该组合对应 SP net cost 的负值（反转）

#### Scenario: GPU 实例类型对应条目携带正确字段
- **GIVEN** SP 覆盖用量中包含 `product_instance_type = "p3.2xlarge"` 的记录
- **WHEN** Pull 阶段执行完毕
- **THEN** 生成的 RawBillItem 中 `Extension.ProductInstanceType = "p3.2xlarge"` 且 `Extension.LineItemProductCode` 为对应产品码

#### Scenario: 所有条目 BillCost 之和等于原总量的负值
- **WHEN** Pull 阶段执行完毕
- **THEN** 所有返回的 `RawBillItem.BillCost` 之和等于 SP 覆盖用量 net cost 总和的负值，账单金额守恒

---

### Requirement: SP 反转月任务 Split 阶段为每条 RawBillItem 生成独立 BillItem

`splitSpReverseExpense` SHALL 为每条 `RawBillItem` 生成一条独立的 `BillItemCreateReq`，而非将所有 rawItem 合并为单一条目。

每条 `BillItemCreateReq` SHALL 满足：
- `Extension.LineItemProductCode` 透传自对应 `RawBillItem` 的 Extension
- `Extension.ProductInstanceType` 透传自对应 `RawBillItem` 的 Extension
- `Cost` 等于该 `RawBillItem` 的 `BillCost`
- `MainAccountID`、`ProductID`、`BkBizID` 均来自 SP 账号对应的 summary 查询

#### Scenario: GPU 实例类型的 BillItem 可被 OBS sync 正确识别
- **GIVEN** RawBillItem 的 `Extension.ProductInstanceType = "p3.2xlarge"` 且该类型在 GPU 实例集合中
- **WHEN** OBS 同步阶段调用 `isAwsGPU(Extension.LineItemProductCode, Extension.ProductInstanceType, HcProductName, awsGpuSet)`
- **THEN** 函数返回 `true`，该条目被标记为 GPU 资源

#### Scenario: 非 GPU 实例类型的 BillItem 不被误判
- **GIVEN** RawBillItem 的 `Extension.ProductInstanceType = "c5.xlarge"` 且该类型不在 GPU 实例集合中
- **WHEN** OBS 同步阶段调用 `isAwsGPU`
- **THEN** 函数返回 `false`，该条目被标记为非 GPU 资源
