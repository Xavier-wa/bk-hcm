## 1. 删除旧函数

- [x] 1.1 在 `cmd/woa-server/logics/plan/penalty.go` 中完全删除 `CreatePenaltyBaseFromTicket` 函数定义（暂未完成，按用户要求保留旧函数）
- [x] 1.2 查找并更新所有调用 `CreatePenaltyBaseFromTicket` 的地方，改为调用 `CreatePenaltyBaseFromResPlanDemand`
- [x] 1.3 删除与订单快照相关的所有辅助函数和逻辑

## 2. 新增 `CreatePenaltyBaseFromResPlanDemand` 函数

- [x] 2.1 在 `cmd/woa-server/logics/plan/penalty.go` 中新增 `CreatePenaltyBaseFromResPlanDemand` 函数
- [x] 2.2 实现从 `res_plan_demand` 表查询预测数据逻辑：
    - 通过 `ListResPlanDemand` API 查询
    - 过滤条件：`expect_time` 在目标周内、且 `plan_type = 'in_plan'`
    - 移除"回顾过去 2 个月"限制
- [x] 2.3 实现数据聚合逻辑：
    - 按 `bk_biz_id + area_name + device_family` 维度聚合
    - 累加 `cpu_core`
    - 生成 `DemandPenaltyBaseKey` 作为聚合 Key
- [x] 2.4 实现写入数据库逻辑：
    - 先删除该年周的所有旧数据（避免重复）
    - 批量插入新计算的数据
- [x] 2.5 实现补偿机制（补算上周数据）：
    - 系统每周一启动时，检查上周的罚金基数是否已生成
    - 如果未生成，则补算上周的罚金基数（从 `res_plan_demand` 表查询）
    - 补算逻辑与正常计算逻辑一致
- [x] 2.6 支持分页查询（防大数据量）

## 3. `ListResPlanDemand` API 调用封装

- [x] 3.1 检查是否已存在 `ListResPlanDemand` API 调用封装
    - **结果**：已存在，位于 `pkg/client/data-service/global/resource_plan.go` 第 46-52 行
    - **请求参数**：`*rpproto.ResPlanDemandListReq`（包含 `Filter` 和 `Page` 字段）
    - **响应结果**：`*rpproto.ResPlanDemandListResult`（包含 `Details` 和 `Count` 字段）
    - **分页支持**：已支持（通过 `Page.Start` 和 `Page.Limit`）
- [x] 3.2 无需新增封装（直接使用现有实现）
- [x] 3.3 请求参数结构体已定义（`rpproto.ResPlanDemandListReq`）
- [x] 3.4 响应结构体已定义（`rpproto.ResPlanDemandListResult`）
- [x] 3.5 API 调用方法已实现，支持分页查询

## 4. 更新 `generatePenaltyBase` 函数

- [x] 4.1 在 `cmd/woa-server/logics/plan/penalty.go` 中找到 `generatePenaltyBase` 函数
- [x] 4.2 更新 `generatePenaltyBase` 函数中的调用点，改为调用 `CreatePenaltyBaseFromResPlanDemand`
- [x] 4.3 确保补偿机制（补算上周数据）也使用新函数

## 5. 更新 `CalcPenaltyBase` 函数

- [x] 5.1 在 `cmd/woa-server/logics/plan/penalty.go` 中找到 `CalcPenaltyBase` 函数
- [x] 5.2 更新 `CalcPenaltyBase` 函数中的调用点，改为调用 `CreatePenaltyBaseFromResPlanDemand`

## 6. 单元测试

- [x] 6.1 为 `CreatePenaltyBaseFromResPlanDemand` 和 `calcPenaltyBaseCoreByResPlanDemand` 函数编写单元测试
    - 测试文件：`cmd/woa-server/logics/plan/penalty_test.go`
    - 测试函数（已完成）：
        - `TestCalcPenaltyBaseCoreByResPlanDemand`：测试核心聚合逻辑（模拟）
        - `TestCreatePenaltyBaseFromResPlanDemand_FilterLogic`：测试过滤逻辑
- [x] 6.2 测试数据过滤逻辑（`expect_time` 在目标周内、`plan_type = 'in_plan'`）
- [x] 6.3 测试数据聚合逻辑（按业务ID + 大区 + 机型族累加 CPU 核心数）
