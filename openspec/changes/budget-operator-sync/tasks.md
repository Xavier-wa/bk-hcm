# 预算提报人同步 - 任务清单

## 1. API 层（woa-server）- 已完成

### 1.1 请求/响应协议定义

- [x] 1.1.1 在 `cmd/woa-server/logics/plan/types.go` 中添加 `BudgetOperatorSyncReq`
  - 字段：`StartTime`, `EndTime`（string，YYYY-MM-DD 格式）
  - 添加校验（必填、日期格式）
  - 文件位置：定义在 `cmd/woa-server/logics/plan/demand.go` 中

- [x] 1.1.2 在 `cmd/woa-server/logics/plan/types.go` 中添加 `BudgetOperatorSyncResp`
  - 字段：`TotalCount`, `ProcessedCount`, `SuccessCount`, `FailedCount`, `FailedDemandIds`, `SkippedCount`, `SkippedDemandIds`
  - 文件位置：定义在 `cmd/woa-server/logics/plan/types.go` 中

- [x] 1.1.3 在 `cmd/woa-server/service/plan/budget_operator_sync.go` 中注册 API 路由
  - 路径：`/plans/resources/demands/budget_operator/sync/by_time`
  - 方法：POST
  - 处理器：`SyncBudgetOperatorByTime`
  - 文件位置：`cmd/woa-server/service/plan/budget_operator_sync.go`

### 1.2 服务实现

- [x] 1.2.1 在 `cmd/woa-server/logics/plan/demand.go` 中实现 `SyncBudgetOperatorByTime` 方法
  - 作为 `Controller` 结构体的方法
  - 实现完整的业务逻辑：参数校验、单据查询、分组、批量处理、FinOps API 调用、提报人选择、幂等更新、响应聚合
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 1893-1935 行

- [x] 1.2.2 在 `cmd/woa-server/logics/plan/demand.go` 中实现参数校验
  - 时间格式校验（YYYY-MM-DD）
  - 开始时间 <= 结束时间校验
  - 文件位置：`cmd/woa-server/service/plan/budget_operator_sync.go`

- [x] 1.2.3 实现单据查询逻辑
  - 通过 `pkg/client/data-service/global/resource_plan.go` 的 `ResourcePlanClient.ListResPlanDemand` 调用 data-service
  - 筛选条件：`creator = 'hcm-backend-admin' AND expect_time BETWEEN start_time AND end_time`
  - 排序：`expect_time ASC`
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 2025-2069 行（`collectBudgetOperatorDemands` 方法）

- [x] 1.2.4 实现分组逻辑
  - 按 `OpProductID + 年月` 分组
  - 使用 `[]*budgetDemand` 指针切片避免内存拷贝
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 1937-1955 行（`groupBudgetDemands` 方法）

- [x] 1.2.5 实现批量处理
  - 先收集所有需要更新的 demand IDs，再按 100 条/批批量提交
  - 子批次间隔 200ms
  - 优化：同一 candidate 的 demands 汇总后一次性批量提交，减少 data-service 调用次数
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 2090-2156 行（`processBudgetDemands` 方法）

- [x] 1.2.6 集成 FinOps API 客户端
  - 直接使用 `pkg/thirdparty/api-gateway/finops` 包的 `GetBudgetDeclarationOperator` 方法
  - 根据分组中的 `OpProductID` 和 `Year` 构造批量请求参数
  - 批量查询多个 OpProductID 和年份组合
  - 处理 API 响应解析（从 `Composition.Creators` 或 `Committers` 获取候选人列表）
  - 实现重试逻辑（3 次，间隔 1s）
  - 文件位置：
    - `cmd/woa-server/logics/plan/demand.go` 1938-2007 行（`batchFetchBudgetOperatorCandidates` 方法）
    - `pkg/thirdparty/api-gateway/finops/finops.go` 111-222 行（`GetBudgetDeclarationOperator` 方法和参数/响应定义）

- [x] 1.2.7 实现提报人选择逻辑
  - 按提报人 ID 去重
  - 取第一个非空候选人
  - 过滤空字符串和 backend 用户
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 2055-2072 行（`deduplicateBudgetOperatorCandidates` 方法）

- [x] 1.2.8 实现幂等更新
  - 通过 data-service 客户端调用 `ResPlanDemandBatchUpdateCreator`
  - 条件：`id IN (...) AND creator = :backend`
  - **重要**：移除了原计划中的 `creator != :candidate_operator` 条件，因为它与 SQL 三值逻辑冲突，会导致 creator 为 NULL 的记录无法更新
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 2090-2155 行（`processBudgetDemands` 方法）

- [x] 1.2.9 实现响应聚合
  - 统计 total、processed、success、failed、skipped 数量
  - 收集 failed 和 skipped 的 demand ID
  - FinOps 调用失败时返回当前已累计的响应结果
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 1893-1935 行（`SyncBudgetOperatorByTime` 方法）


## 2. 数据层（data-service）- 已完成

### 2.1 API 协议增强

- [x] 2.1.1 在 `pkg/api/data-service/resource-plan/res_plan_demand.go` 中添加 `ResPlanDemandBatchUpdateCreatorReq`
  - 字段：`IDs` ([]string, validate: required,min=1,max=1000), `Creator` (string, validate: required), `Reviser` (string, validate: required)
  - 文件位置：161-165 行

- [x] 2.1.2 在 `pkg/api/data-service/resource-plan/res_plan_demand.go` 中添加 `ResPlanDemandBatchUpdateCreatorResp`
  - 字段：`UpdatedCount` (int64)
  - 文件位置：173-175 行

- [x] 2.1.3 在 `cmd/data-service/service/resource-plan/res-plan-demand/service.go` 的 `InitService` 函数中注册路由
  - 路径：`/res_plans/res_plan_demands/update_creator`
  - 方法：PATCH
  - 处理器：`BatchUpdateDemandCreator
  - 文件位置：`cmd/data-service/service/resource-plan/res-plan-demand/service.go` 52 行

### 2.2 DAO 层

- [x] 2.2.1 在 `pkg/dal/dao/resource-plan/res_plan_demand.go` 中添加 `UpdateDemandCreator` 方法
  - 使用 filter 框架构建 WHERE 条件：`id IN (...) AND creator = :backend`
  - 更新 `creator` 和 `reviser` 字段
  - 返回实际更新的记录数
  - 文件位置：`pkg/dal/dao/resource-plan/res_plan_demand.go` 140-186 行

- [x] 2.2.2 添加接口定义到 `ResPlanDemandInterface`
  - 方法签名：`UpdateDemandCreator(kt *kit.Kit, demandIDs []string, creator, reviser string) (int64, error)`
  - 文件位置：`pkg/dal/dao/resource-plan/res_plan_demand.go` 55 行

### 2.3 Data-service Service 层

- [x] 2.3.1 在 `cmd/data-service/service/resource-plan/res-plan-demand/update.go` 中实现 `UpdateDemandCreator` 方法
  - 调用 DAO 层的 `UpdateDemandCreator` 方法
  - 返回更新记录数
  - 文件位置：`cmd/data-service/service/resource-plan/res-plan-demand/update.go` 227-247 行

### 2.4 Data-service 客户端

- [x] 2.4.1 在 `pkg/client/data-service/global/resource_plan.go` 中添加 `ResPlanDemandBatchUpdateCreator` 方法
  - 调用 data-service 的 `/res_plans/res_plan_demands/update_creator` 接口
  - 文件位置：`pkg/client/data-service/global/resource_plan.go` 69-74 行

## 3. 重要说明

### 3.1 SQL 三值逻辑修复

**原计划条件（有 bug）：**
```sql
WHERE id = :id AND (creator IS NULL OR creator = :backend) AND creator != :candidate_operator
```

**问题：**
- `NULL != value` 在 SQL 三值逻辑中结果为 `NULL`（非 TRUE），导致 creator 为 NULL 的记录永远无法被更新
- 这会造成数据不一致：查询到的 NULL creator 记录被收集进入更新流程，但在 DB 层被静默丢弃

**实际实现（已修复）：**
```sql
WHERE id IN (...) AND creator = :backend
```

**变更说明：**
- 移除了 `creator IS NULL OR` 部分，只更新 `creator = 'hcm-backend-admin'` 的记录
- 移除了 `creator != :candidate_operator` 条件，避免三值逻辑问题

**业务逻辑影响：**
- 只会更新 backend 创建的记录（包含空字符串和 backend 用户）
- 已被业务用户设置 creator 的记录不会被覆盖（符合预期）
- logics 层在 `processBudgetDemands` 方法（2100-2111 行）处理了当前 creator 不为 backend 的情况

### 3.2 使用 filter 框架

根据 Code Review 意见，DAO 层更新方法已改用项目标准 filter 框架，避免手动拼接 SQL：
- 使用 `tools.ContainersExpression` 构建批量 ID 条件
- 使用 `tools.EqualExpression` 构建等值条件
- 使用 `filter.Expression` 组合多个条件
- 文件位置：`pkg/dal/dao/resource-plan/res_plan_demand.go` 149-159 行

### 3.3 筛选条件实现

**woa-server 层的 filter 构建：**
```go
listFilter := tools.ExpressionAnd(
    tools.RuleEqual("creator", constant.BackendOperationUserKey),
    tools.RuleGreaterThanEqual("expect_time", times.ConvTimeToCompactInt(start)),
    tools.RuleLessThan("expect_time", times.ConvTimeToCompactInt(end.AddDate(0, 0, 1))),
)
```

**说明：**
- 使用 `RuleEqual` 方法只匹配 `BackendOperationUserKey`，与 DAO 层 WHERE 条件对齐
- 空字符串 creator 的记录不再收集，避免发送到 DAO 后被静默跳过导致统计不准确
- 使用 `expect_time` 字段进行时间范围查询
- 使用 `times.ConvTimeToCompactInt` 将时间转换为紧凑整数格式（YYYYMMDD）

### 3.4 批量优化

**优化点：**
1. **分组查询**：按 `OpProductID + 年月` 分组，避免重复调用 FinOps API
2. **批量查询**：`batchFetchBudgetOperatorCandidates` 方法中一次性查询所有分组的预算提报人，避免 N 次单独调用
3. **提报人缓存**：批量查询后构建 `candidateCache`，按 `opID-year` key 缓存候选人
4. **批次处理**：先收集所有需要更新的 IDs，再按 100 条/批批量提交，200ms 间隔，避免数据库压力过大
5. **重试机制**：FinOps API 调用失败时自动重试 3 次，间隔 1s
6. **指针切片优化**：`demandGroup.Items` 使用 `[]*budgetDemand` 指针切片，避免 range + append 时的结构体拷贝

## 4. 异常处理与日志

- [x] 4.1 添加 FinOps API 失败时的正确异常处理
  - 失败时返回错误码，并在 `data` 中携带当前已累计的处理结果
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 1913-1917 行

- [x] 4.2 添加跳过单据的日志（无候选人）
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 1922-1928 行（无候选人分支）

- [x] 4.3 添加更新失败日志
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 2138-2143 行（err 分支）

- [x] 4.4 添加批量进度结构化日志
  - 文件位置：`cmd/woa-server/logics/plan/demand.go` 多处 Info/Infof 日志

## 5. 质量保障

### 5.1 单元测试

- [x] 5.1.1 测试提报人选择逻辑（去重、取第一个非空）
  - 测试函数：`TestDeduplicateBudgetOperatorCandidates`
  - 覆盖场景：空列表、单个空列表、过滤空字符串、过滤backend用户、去重、去除前后空格、多个列表合并去重、综合场景
  - 文件位置：`cmd/woa-server/logics/plan/demand_test.go`
- [x] 5.1.2 测试分组逻辑（同一 OpProductID 跨月场景）
  - 测试函数：`TestGroupBudgetDemands`、`TestGroupBudgetDemands_PointerStability`
  - 覆盖场景：空列表、单个需求、同一OpProductID同一月份、同一OpProductID跨月份、不同OpProductID同一月份、跨年场景、复杂场景（多产品多月份）
  - 额外测试：指针稳定性验证（确保使用指针切片避免内存拷贝）
  - 文件位置：`cmd/woa-server/logics/plan/demand_test.go`
- [x] 5.1.3 测试参数校验（时间格式错误、start > end）
  - 测试函数：`TestBudgetOperatorSyncReqValidate`、`TestBudgetOperatorSyncReqTimeRange`
  - 覆盖场景：有效请求、相同日期、缺少start_time、缺少end_time、start_time格式错误、end_time格式错误、start_time晚于end_time、无效日期（2月30日、13月）、正常时间范围、跨年时间范围
  - 文件位置：`cmd/woa-server/types/plan/demand_test.go`
- [ ] 5.1.4 测试批量拆分（100条/批，200ms间隔）

### 5.2 集成测试

- [ ] 5.2.1 测试灰度执行（特定月份）
- [ ] 5.2.2 测试全量执行（全年）
- [ ] 5.2.3 测试 FinOps API 失败处理
- [ ] 5.2.4 测试空候选人处理
- [ ] 5.2.5 测试 SQL 三值逻辑修复（验证 backend 记录能被正确更新）
- [ ] 5.2.6 测试分组逻辑（OpProductID + 年月）

### 5.3 性能测试

- [ ] 5.3.1 测试单批次（100条）处理耗时
- [ ] 5.3.2 测试 1000 条批量处理总耗时
- [ ] 5.3.3 测试数据库锁争用

## 6. 监控

- [ ] 6.1 添加 API 监控指标（调用量、成功率、响应时间）
- [ ] 6.2 添加数据监控（creator 字段为 'hcm-backend-admin' 或空的记录数量）
- [ ] 6.3 配置告警：
  - FinOps API 调用失败
  - 批量更新失败率 > 1%
  - 数据库锁等待超时
  - 更新记录数为 0 但查询结果不为空的异常情况
