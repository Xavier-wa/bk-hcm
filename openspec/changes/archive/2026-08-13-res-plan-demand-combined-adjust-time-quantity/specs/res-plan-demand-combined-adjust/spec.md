## MODIFIED Requirements

### Requirement: 调整主单入参与校验
系统 SHALL 在 `AdjustRPDemandReq` 提供可选的请求级 `DemandClass`（`demand_class`，`omitempty`）：仅当整批调整后所有条目 `demand_id` 均为空（全纯新增）时，该字段由调用方必填以确定 CVM/CA；混批时由有 ID 条目查 DB 推导，请求级 `demand_class` 不生效。`AdjustRPDemandReqElem.DemandID` SHALL 改为非必填（`omitempty`）。`AdjustRPDemandReqElem.Validate()` SHALL 按条目分流：纯新增（`demand_id` 为空）仅校验 `updated_info`，`original_info` 可空；修改已有（`demand_id` 非空）强校验 `demand_id + original_info + updated_info` 必填。`update` 类型 SHALL 允许期望时间（`expect_time`）与资源数量同时变更；`delay` 类型 SHALL 仍校验 `expect_time`，且不在此暴露部分延期（`delay_os`）。

#### Scenario: 全纯新增批次以请求 demand_class 确定 CVM/CA
- **WHEN** 整批条目 `demand_id` 均为空且请求携带 `demand_class`
- **THEN** 系统以请求级 `demand_class` 确定整批 CVM/CA，并允许 `original_info` 为空

#### Scenario: 修改已有需求强校验原信息
- **WHEN** 条目 `demand_id` 非空但 `original_info` 或 `updated_info` 为空
- **THEN** 校验失败返回 `InvalidParameter`

#### Scenario: 改期同时改量
- **WHEN** `adjust_type=update` 且 `updated_info` 同时变更 `expect_time` 与资源数量
- **THEN** 校验通过，不再互斥

### Requirement: 调整主单提单与构造
系统 SHALL 在 `AdjustBizResPlanDemand` 中先过滤空 `demand_id` 再做业务归属校验（`AreAllDemandBelongToBiz`）与 `demand_class` 推导（`ExamineDemandClass`）：混批时由有 ID 条目查 DB，全纯新增批次回退到请求级 `DemandClass`；整批 `demand_class` 必须一致（CVM/CA 不可混）。纯新增条目 SHALL NOT 参与 lock。系统在 `constructAdjustReq` 中：有 `demand_id` 的条目按 `adjust_type` 走 `constructUpdateDemands` / `constructDelayDemands`；纯新增（无 `demand_id`）条目将其 `updated_info` 转为 `CreateResPlanDemandReq` 并调用 `buildDemandsFromCreateReq()` 产出 `Original==nil && Updated!=nil` 的需求；lock 遍历 SHALL 仅覆盖 `demand.Original != nil` 的条目。

#### Scenario: 混批归属校验跳过纯新增
- **WHEN** 批次同时含「有 demand_id 的既有需求」与「无 demand_id 的纯新增」
- **THEN** 仅对既有需求做归属校验与 DB 推导 demand_class，纯新增不参与

#### Scenario: 整批 demand_class 不一致被拒
- **WHEN** 批次内条目推导出的 demand_class 不一致（如部分 CVM、部分 CA）
- **THEN** 返回 `InvalidParameter`，不再继续提单

#### Scenario: 纯新增不触发 lock
- **WHEN** 批次含纯新增条目
- **THEN** lock 仅锁定含 Original 的既有需求，纯新增条目不被锁定

#### Scenario: 纯新增经创建请求构造
- **WHEN** 存在无 `demand_id` 的条目
- **THEN** 系统用其 `updated_info` 经 `buildDemandsFromCreateReq` 生成 `Original==nil` 的需求并纳入提单

### Requirement: 调整主单拆单路由
系统 SHALL 在 `getDemandsWithoutTransfer` 中对「改期类」需求按变更内容路由子单：仅当 **仅改期且数量不变（同机型、非跨年）** 时归入延期子单；「改期且改量（含 10→9@8）」「换机型」「仅改量不改期」均归入常规 adjust（调增/调减）子单；`delay` 类型提单（整单延期）归入延期子单。纯新增（`Original==nil`）追加子单 SHALL 复用既有 `splitAdjustDemandsToAddAndDelete()` 逻辑，无需改动 `add.go` / `sub_ticket.go`。拆单路由 SHALL 与 CRP 延期/修改区分口径一致。

#### Scenario: 仅改期走延期
- **WHEN** 需求仅变更 `expect_time`、数量与机型不变、且非跨年
- **THEN** 归入延期子单

#### Scenario: 改期且改量走 adjust
- **WHEN** 需求同时变更 `expect_time` 与数量（如 10→9@8）
- **THEN** 归入常规 adjust（调增/调减）子单

#### Scenario: 换机型或仅改量走 adjust
- **WHEN** 需求换机型或仅改量不改期
- **THEN** 归入常规 adjust 子单

#### Scenario: 纯新增追加子单复用既有逻辑
- **WHEN** 需求 `Original==nil`（纯新增）
- **THEN** 由 `splitAdjustDemandsToAddAndDelete` 归为追加子单，无需单独改动 add/sub_ticket
