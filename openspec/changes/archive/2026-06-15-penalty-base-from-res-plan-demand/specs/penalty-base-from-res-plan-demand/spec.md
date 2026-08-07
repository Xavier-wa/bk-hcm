## NEW Requirements

### Requirement: 从 res_plan_demand 表计算罚金基数

系统 SHALL 从 `res_plan_demand` 表（预算导入数据）计算罚金基数（`demand_penalty_base`），替代原有的从订单快照计算的方式。

#### Scenario: 正常计算罚金基数

- **WHEN** 调用 `CreatePenaltyBaseFromResPlanDemand` 函数
- **THEN** 系统从 `res_plan_demand` 表查询预测数据，按 `bk_biz_id + area_name + device_family` 维度聚合 CPU 核心数，并写入 `demand_penalty_base` 表

#### Scenario: 数据过滤 - expect_time 在时间范围内

- **WHEN** 查询 `res_plan_demand` 表时
- **THEN** 只返回 `expect_time` 在目标时间范围内的记录（`startTime <= expect_time <= endTime`）

#### Scenario: 数据过滤 - plan_type 为 in_plan

- **WHEN** 查询 `res_plan_demand` 表时
- **THEN** 只返回 `plan_type = 'in_plan'` 的记录（只统计预测内的数据）

### Requirement: 数据聚合逻辑

系统 SHALL 按 `bk_biz_id + area_name + device_family` 维度聚合 CPU 核心数，生成罚金基数。

#### Scenario: 按多维度聚合

- **WHEN** 有多个 `res_plan_demand` 记录具有相同的 `bk_biz_id`、`area_name`、`device_family`
- **THEN** 系统将这些记录的 `cpu_core` 累加，作为一个聚合值

#### Scenario: 生成聚合 Key

- **WHEN** 聚合数据时
- **THEN** 系统使用 `ptypes.DemandPenaltyBaseKey` 结构体作为聚合 Key，包含 `BkBizID`、`AreaName`、`DeviceFamily` 字段

#### Scenario: 处理 CpuCore 指针类型

- **WHEN** `res_plan_demand` 记录的 `cpu_core` 字段为指针类型
- **THEN** 系统正确解引用指针，获取实际的 CPU 核心数（如果指针为 nil，则使用 0）

#### Scenario: 查询业务组织关系

- **WHEN** 聚合数据时
- **THEN** 系统同时查询每个业务的组织关系（`BizOrgRel`），包括 `BkBizName`、`OpProductID`、`OpProductName`、`PlanProductID`、`PlanProductName`、`VirtualDeptID`、`VirtualDeptName`
- **AND** 每个业务只查询一次（去重）

### Requirement: 数据写入逻辑

系统 SHALL 先删除该年周的所有旧数据，再批量插入新计算的数据，避免重复。

#### Scenario: 清理旧数据

- **WHEN** 写入新数据前
- **THEN** 系统先删除 `demand_penalty_base` 表中该年周（`year` 和 `year_week`）的所有旧数据

#### Scenario: 批量插入新数据

- **WHEN** 聚合计算完成后
- **THEN** 系统将聚合结果批量插入 `demand_penalty_base` 表

### Requirement: 分页查询支持

系统 SHALL 支持分页查询 `res_plan_demand` 表，防止大数据量导致内存溢出。

#### Scenario: 分页查询大数据量

- **WHEN** `res_plan_demand` 表数据量超过每页限制（500 条）
- **THEN** 系统分多次查询，每次查询一页数据，直到查询完所有数据

#### Scenario: 最后一页判断

- **WHEN** 某次查询返回的记录数小于每页限制
- **THEN** 系统判断为最后一页，停止查询

### Requirement: 更新调用点

系统 SHALL 更新 `generatePenaltyBase` 和 `CalcPenaltyBase` 函数中的调用点，改为调用 `CreatePenaltyBaseFromResPlanDemand`。

#### Scenario: 更新 generatePenaltyBase 调用点

- **WHEN** 系统执行 `generatePenaltyBase` 函数
- **THEN** 调用 `CreatePenaltyBaseFromResPlanDemand` 计算罚金基数（原调用 `CreatePenaltyBaseFromTicket` 的代码被替换）

#### Scenario: 更新 CalcPenaltyBase 调用点

- **WHEN** 系统执行 `CalcPenaltyBase` 函数
- **THEN** 调用 `CreatePenaltyBaseFromResPlanDemand` 计算罚金基数（原调用 `CreatePenaltyBaseFromTicket` 的代码被替换）

### Requirement: 保留旧函数

系统 SHALL 保留旧函数 `CreatePenaltyBaseFromTicket` 和 `calcPenaltyBaseCoreByTicket`，不删除（按用户要求）。

#### Scenario: 旧函数仍然存在

- **WHEN** 查看 `penalty.go` 文件
- **THEN** `CreatePenaltyBaseFromTicket` 和 `calcPenaltyBaseCoreByTicket` 函数仍然存在，未被删除

## MODIFIED Requirements

### Requirement: 移除时间限制

系统 SHALL 移除"回顾过去 2 个月"限制，可以统计更长时间范围内的数据。

#### Scenario: 无时间限制

- **WHEN** 从 `res_plan_demand` 表计算罚金基数时
- **THEN** 不考虑"回顾过去 2 个月"限制，可以统计任意时间范围内的数据（只要 `expect_time` 在目标时间范围内）
