# 罚金基数从预算导入数据计算 - 设计文档

## 1. 背景与动机

### 1.1 当前问题

当前罚金基数（`demand_penalty_base`）是从**订单快照**计算的：
- 函数：`CreatePenaltyBaseFromTicket`、`calcPenaltyBaseCoreByTicket`
- 数据来源：从历史订单（`res_plan_ticket`）还原预测总量
- 限制：只考虑 2 个月内的单据（`ticketEnd.AddDate(0, -2, 0)`）

### 1.2 目标

改为从 **`res_plan_demand` 表（预算导入数据）** 计算罚金基数：
- 新函数：`CreatePenaltyBaseFromResPlanDemand`、`calcPenaltyBaseCoreByResPlanDemand`
- 数据来源：从 `res_plan_demand` 表查询预测数据
- 优势：
    - 数据更准确（直接从预算导入数据计算，不需要从订单还原）
    - 无时间限制（移除"回顾过去 2 个月"限制）
    - 逻辑更简单（不需要解析订单快照）

## 2. 方案设计

### 2.1 数据结构

#### 2.1.1 `res_plan_demand` 表（输入）

| 字段 | 类型 | 说明 |
|------|------|------|
| `bk_biz_id` | int64 | 业务 ID |
| `area_name` | string | 大区名称 |
| `device_family` | string | 机型族 |
| `cpu_core` | *int64 | CPU 核心数（指针类型） |
| `expect_time` | string | 期望时间（格式：YYYY-MM-DD） |
| `plan_type` | string | 计划类型（`in_plan` / `out_plan`） |

#### 2.1.2 `demand_penalty_base` 表（输出）

| 字段 | 类型 | 说明 |
|------|------|------|
| `year` | int | 年 |
| `year_week` | int | 年周 |
| `bk_biz_id` | int64 | 业务 ID |
| `area_name` | string | 大区名称 |
| `device_family` | string | 机型族 |
| `cpu_core` | *int64 | CPU 核心数（罚金基数） |

### 2.2 核心逻辑

#### 2.2.1 数据查询

通过 `ListResPlanDemand` API 查询 `res_plan_demand` 表：

```go
// 构建查询过滤条件
// 注意：expect_time 在数据库中为数值类型（时间戳），需先通过 ConvStrTimeToInt 将日期字符串转为整数
startExpTime, err := times.ConvStrTimeToInt(timeRange.Start, constant.DateLayout)
if err != nil {
    return nil, nil, err
}
endExpTime, err := times.ConvStrTimeToInt(timeRange.End, constant.DateLayout)
if err != nil {
    return nil, nil, err
}

listRules := []*filter.AtomRule{
    tools.RuleGreaterThanEqual("expect_time", startExpTime),
    tools.RuleLessThanEqual("expect_time", endExpTime),
    tools.RuleEqual("plan_type", enumor.PlanTypeCodeInPlan),
}
if len(bkBizIDs) > 0 {
    listRules = append(listRules, tools.RuleIn("bk_biz_id", bkBizIDs))
}
listFilter := tools.ExpressionAnd(listRules...)
```

#### 2.2.2 数据过滤

- **条件 1**：`expect_time` 在时间范围内（`startTime <= expect_time <= endTime`）
- **条件 2**：`plan_type = 'in_plan'`（只统计预测内的数据）

#### 2.2.3 数据聚合

按 `bk_biz_id + area_name + device_family` 维度聚合，累加 `cpu_core`：

```go
key := ptypes.DemandPenaltyBaseKey{
    BkBizID:      detail.BkBizID,
    AreaName:     detail.AreaName,
    DeviceFamily: detail.DeviceFamily,
}
// detail.CpuCore 是指针类型，需要解引用
cpuCore := int64(0)
if detail.CpuCore != nil {
    cpuCore = *detail.CpuCore
}
baseCoreMap[key] += cpuCore
```

#### 2.2.4 业务组织关系查询

在聚合数据的同时，查询每个业务的组织关系（每个业务只查询一次）：

```go
// 查询业务组织关系（每个业务只查询一次）
for _, detail := range result.Details {
    if _, exists := bizOrgRelMap[detail.BkBizID]; exists {
        continue
    }
    bizOrgRel, err := c.bizLogics.GetBizOrgRel(kt, detail.BkBizID)
    if err != nil {
        logs.Errorf("failed to get biz org rel, err: %v, bk_biz_id: %d, rid: %s", err, detail.BkBizID, kt.Rid)
        return nil, nil, err
    }
    bizOrgRelMap[detail.BkBizID] = *bizOrgRel
}
```

#### 2.2.5 数据写入

1. **清理旧数据**：先删除该年周的所有旧数据（避免重复）
   ```go
   deleteReq := &dataservice.BatchDeleteReq{
       Filter: tools.ExpressionAnd(
           tools.RuleEqual("year", baseYearWeek.Year),
           tools.RuleEqual("year_week", baseYearWeek.YearWeek),
       ),
   }
   err = c.client.DataService().Global.ResourcePlan.DeleteDemandPenaltyBase(kt, deleteReq)
   ```

2. **批量插入新数据**：
   ```go
   // 调用 createDemandPenaltyBase 函数，传入 baseCoreMap 和 bizOrgRelMap
   createIDs, err := c.createDemandPenaltyBase(kt, baseYearWeek, baseCoreMap, bizOrgRelMap)
   ```

   `createDemandPenaltyBase` 函数会将聚合结果转换为 `rpproto.DemandPenaltyBaseCreate` 结构体，并批量插入数据库。

### 2.3 补偿机制

系统每周一启动时，检查上周的罚金基数是否已生成：
- 如果未生成，则补算上周的罚金基数（从 `res_plan_demand` 表查询）
- 补算逻辑与正常计算逻辑一致

### 2.4 分页查询

支持分页查询，防止大数据量导致内存溢出：
```go
pageSize := 500
for pageStart := 0; ; pageStart += pageSize {
    listReq := &rpproto.ResPlanDemandListReq{
        ListReq: core.ListReq{
            Filter: listFilter,
            Page: &core.BasePage{
                Start: uint32(pageStart),
                Limit: uint(pageSize),
            },
        },
    }
    result, err := c.client.DataService().Global.ResourcePlan.ListResPlanDemand(kt, listReq)
    // ...
    if len(result.Details) < pageSize {
        break // 最后一页
    }
}
```

## 3. 影响分析

### 3.1 正向影响

1. **数据准确性提升**：直接从预算导入数据计算，不需要从订单还原
2. **无时间限制**：移除"回顾过去 2 个月"限制，可以统计更长时间范围内的数据
3. **逻辑简化**：不需要解析订单快照，逻辑更清晰
4. **性能提升**：从单个表查询，不需要关联多个表

### 3.2 潜在风险

1. **数据迁移**：历史数据需要重新计算（或者保留旧数据）
2. **兼容性**：需要确保新逻辑与旧逻辑计算结果一致（对比测试）
3. **异常处理**：查询失败时需要正确处理（不写表、记录错误日志）

### 3.3 依赖关系

- **上游**：`ListResPlanDemand` API（已存在）
- **下游**：`demand_penalty_base` 表（已存在）
- **配置**：无

## 4. 测试计划

### 4.1 单元测试

| 测试函数 | 测试内容 | 状态 |
|---------|---------|------|
| `TestCalcPenaltyBaseCoreByResPlanDemand` | 测试核心聚合逻辑（模拟） | ✅ 已完成 |
| `TestCreatePenaltyBaseFromResPlanDemand_FilterLogic` | 测试过滤逻辑 | ✅ 已完成 |
| `TestCreatePenaltyBaseFromResPlanDemand_Flow` | 测试完整流程（模拟核心逻辑） | ✅ 已完成 |

### 4.2 集成测试

| 测试场景 | 验证内容 |
|---------|---------|
| 新逻辑计算结果与旧逻辑一致 | 对比测试 |
| 罚金基数提前 13 周生成 | 时间逻辑 |
| 补偿机制正常工作 | 补算上周数据 |
| 分页查询正常工作 | 大数据量 |
| 异常处理正确 | 查询失败时不写表、记录错误日志 |

## 5. 实施计划

| 阶段 | 任务 | 状态 |
|------|------|------|
| 1. 删除旧函数 | 删除 `CreatePenaltyBaseFromTicket` 和 `calcPenaltyBaseCoreByTicket` | ✅ 已完成 |
| 2. 新增函数 | 新增 `CreatePenaltyBaseFromResPlanDemand` 和 `calcPenaltyBaseCoreByResPlanDemand` | ✅ 已完成 |
| 3. API 调用封装 | 检查 `ListResPlanDemand` API 调用封装 | ✅ 已完成（已存在） |
| 4. 更新调用点 | 更新 `generatePenaltyBase` 和 `CalcPenaltyBase` 函数中的调用点 | ✅ 已完成 |
| 5. 单元测试 | 编写单元测试 | ✅ 已完成（基础版本） |

## 6. 附录

### 6.1 相关文件

| 文件 | 说明 |
|------|------|
| `cmd/woa-server/logics/plan/penalty.go` | 罚金计算逻辑（主文件） |
| `cmd/woa-server/logics/plan/penalty_test.go` | 单元测试 |
| `pkg/client/data-service/global/resource_plan.go` | `ListResPlanDemand` API 调用封装 |
| `pkg/dal/table/types/res_plan_demand.go` | `res_plan_demand` 表结构体定义 |
