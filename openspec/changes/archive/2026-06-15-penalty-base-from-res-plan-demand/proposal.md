## Why

当前罚金基数（`demand_penalty_base`）是从**订单快照**计算的：
- 函数：`CreatePenaltyBaseFromTicket`、`calcPenaltyBaseCoreByTicket`
- 数据来源：从历史订单（`res_plan_ticket`）还原预测总量
- 限制：只考虑 2 个月内的单据（`ticketEnd.AddDate(0, -2, 0)`）

这种方式存在以下问题：
1. **数据不准确**：需要从订单还原预测总量，可能存在误差
2. **时间限制**：只能统计最近 2 个月的数据
3. **逻辑复杂**：需要解析订单快照，逻辑不够清晰

## What Changes

改为从 **`res_plan_demand` 表（预算导入数据）** 计算罚金基数：

- 新增函数 `CreatePenaltyBaseFromResPlanDemand`：负责从 `res_plan_demand` 表计算罚金基数，先清理对应周期的旧数据，再写入新聚合数据
- 新增函数 `calcPenaltyBaseCoreByResPlanDemand`：核心逻辑函数，实现过滤、分页查询、数据聚合
- 更新调用点：将 `generatePenaltyBase`、`CalcPenaltyBase` 中原有调用 `CreatePenaltyBaseFromTicket` 的代码，替换为调用 `CreatePenaltyBaseFromResPlanDemand`

### 数据过滤条件

1. `expect_time` 在时间范围内（`startTime <= expect_time <= endTime`）
2. `plan_type = 'in_plan'`（只统计预测内的数据）

### 数据聚合逻辑

按 `bk_biz_id + area_name + device_family` 维度聚合，累加 `cpu_core`：
- `cpu_core` 字段为指针类型（`*int64`），需要正确解引用（如果指针为 nil，则使用 0）
- 同时查询每个业务的组织关系（`BizOrgRel`），包括 `BkBizName`、`OpProductID`、`OpProductName`、`PlanProductID`、`PlanProductName`、`VirtualDeptID`、`VirtualDeptName`
- 每个业务只查询一次（去重）

### 分页查询

支持分页查询（每页 500 条），防止大数据量导致内存溢出

## Capabilities

### New Capabilities

- `CreatePenaltyBaseFromResPlanDemand`：从 `res_plan_demand` 表计算罚金基数
- `calcPenaltyBaseCoreByResPlanDemand`：核心计算逻辑，包括数据查询、过滤、聚合

### Modified Capabilities

- `generatePenaltyBase`：更新调用点，改为调用 `CreatePenaltyBaseFromResPlanDemand`
- `CalcPenaltyBase`：更新调用点，改为调用 `CreatePenaltyBaseFromResPlanDemand`

## Impact

### 修改文件

- `cmd/woa-server/logics/plan/penalty.go`：新增函数、更新调用点
- `cmd/woa-server/logics/plan/penalty_test.go`：新增单元测试
- `openspec/changes/archive/2026-06-15-penalty-base-from-res-plan-demand/`：新增 openspec 提案文档

### 数据库变更

- 无数据库 schema 变更
- 数据来源从 `res_plan_ticket` 表改为 `res_plan_demand` 表

### API 变更

- 无 API 变更

### 兼容性

- 保留旧函数 `CreatePenaltyBaseFromTicket` 和 `calcPenaltyBaseCoreByTicket`，确保向后兼容
- 新逻辑与旧逻辑计算结果应一致（需对比测试）

### 性能影响

- **性能提升**：从单个表查询，不需要关联多个表
- **无时间限制**：移除"回顾过去 2 个月"限制，可以统计更长时间范围内的数据
