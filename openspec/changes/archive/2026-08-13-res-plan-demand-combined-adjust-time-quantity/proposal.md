## Why

当前 `AdjustRPDemandReq` 要求每个条目都必须带 `DemandID`，且 `AdjustRPDemandReqElem.Validate()` 对所有条目强校验 `original_info + updated_info`，导致两个受限场景无法支撑：

1. **改期与改量无法同批传入**：update 类型被限制为仅能改其一，无法表达「改期同时改量」（如 10@原期 → 9@8 月）。
2. **纯新增无法混入批次**：纯新增（无 `demand_id`）被 `DemandID` 必填 + 归属校验 + `ExamineDemandClass`（只能从 DB 已有需求推导 `demand_class`）卡死；且整批纯新增时因没有可查的 DB 需求，无法判定该批次是 CVM 还是 CA。

本次改动支撑「改期+改量同时传」「混批纯新增」，并让整批纯新增批次通过请求级 `demand_class` 直接确定 CVM/CA，使提单、锁定、拆单全链路对纯新增与既有需求混批正确工作。

## What Changes

- `cmd/woa-server/types/plan/demand.go`
  - `AdjustRPDemandReq` 新增 `DemandClass`：仅当整批均为纯新增（无 `demand_id`）时必填。
  - `AdjustRPDemandReqElem.DemandID` 改为非必填（`struct tag` 加 `omitempty`）。
  - `AdjustRPDemandReqElem.Validate()`：
    - 纯新增（`demand_id` 为空）：只校验 `updated_info`；`original_info` 可空。
    - 修改已有（`demand_id` 非空）：`demand_id + original_info + updated_info` 必填。
    - `update`：允许日期与数量同时变更。
    - `delay`：仍校验 `expect_time`；本期文档不暴露 `delay_os`。

- `cmd/woa-server/service/plan/demand_update.go`
  - `validateAdjustResPlan()`：纯新增无 `original_info` 时，跳过滚服等基于原需求的校验；滚服限制本身不变。

- `cmd/woa-server/logics/plan/demand_adjust.go`
  - `AdjustBizResPlanDemand()`：过滤空 `demand_id` 后再做归属校验、`ExamineDemandClass`。
    - 混批：`demand_class` 由有 ID 条目查 DB；全纯新增批次用请求 `demand_class`。
    - 校验整批 `demand_class` 一致（CVM/CA 不可混）。
    - 纯新增不参与 lock。
  - `constructAdjustReq()`：
    - 有 `demand_id`：按 `adjust_type` 走 `constructUpdateDemands` / `constructDelayDemands`。
    - 纯新增（无 `demand_id`）：`updated_info` → `[]CreateResPlanDemandReq`，调用 `buildDemandsFromCreateReq()`（`Original=nil`）。
    - lock 仅遍历有 `Original` 的 demand。

- `cmd/woa-server/logics/plan/types.go`
  - `CreateResPlanTicketReq.Validate()`：调整主单允许部分 demand 满足 `Original == nil && Updated != nil`（纯新增）。

- `cmd/woa-server/logics/plan/splitter/adjust.go`
  - 调整 `getDemandsWithoutTransfer()` 路由：

    | 条件 | 子单 |
    | --- | --- |
    | 仅改期、数量不变（同机型，非跨年） | 延期 |
    | 改期且改量（含场景 1：10→9@8） | 常规 adjust（调增/调减） |
    | 换机型 或 仅改量不改期 | 常规 adjust |
    | `delay` 类型提单（整单延期） | 延期 |

  - 纯新增（`Original==nil`）追加子单已由 `splitAdjustDemandsToAddAndDelete()` 支持，提单打通后自动生效，无需改 `add.go` / `sub_ticket.go`。

**说明**：CRP 接口层面虽提供部分延期（部分 `delay_os`）能力，但页面未暴露，本期保持页面行为一致，不暴露 `delay_os`。

## Capabilities

### New Capabilities
<!-- 无新增能力，复用既有 res-plan-demand-combined-adjust 能力并扩展其 Requirement -->

### Modified Capabilities
- `res-plan-demand-combined-adjust`：调整单需求扩展支持「改期+改量同时传」「混批纯新增」「全纯新增批次以请求 `demand_class` 确定 CVM/CA」；拆单路由与 CRP 延期/修改区分一致，纯新增追加子单复用既有拆单逻辑。

## Impact

- 入参契约：`AdjustRPDemandReq` 新增 `DemandClass`；`AdjustRPDemandReqElem.DemandID` 由必填改为可选。
- 校验逻辑：`validateAdjustResPlan` / `AdjustRPDemandReqElem.Validate` / `CreateResPlanTicketReq.Validate` 放宽纯新增语义，纯新增不再被归属校验/滚服校验误拦。
- 提单逻辑：`AdjustBizResPlanDemand` / `constructAdjustReq` 支持混批与纯新增；lock 仅覆盖含 `Original` 的 demand。
- 拆单逻辑：`getDemandsWithoutTransfer` 路由调整，改期+改量走 adjust、仅改期走 delay，与 CRP 延期/修改区分一致。
- 不涉及 DB schema / data-service 接口 / 离线统计变更（`demand_class` 字段已存在于 `res_plan_demand` 与 `res_plan_ticket`）。
- 不暴露 `delay_os`（部分延期），页面行为与 CRP 页面保持一致。
