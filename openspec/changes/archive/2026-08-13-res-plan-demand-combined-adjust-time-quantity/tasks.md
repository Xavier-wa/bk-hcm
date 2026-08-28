## 0. 枚举（pkg/criteria/enumor/woa_ziyan_resplan.go）

- [x] 0.1 新增 `RPDemandAdjustTypeAdd = "add"`，`RPDemandAdjustType.Validate()` 允许 `add`

## 1. 入参与校验（types/plan/demand.go）

- [x] 1.1 `AdjustRPDemandReq` 新增 `DemandClass enumor.DemandClass json:"demand_class" validate:"omitempty"`（仅整批均为 `adjust_type=add` 且无 `demand_id` 时由 logics 层判定必填）
- [x] 1.2 `AdjustRPDemandReqElem.DemandID` 改为 `validate:"omitempty"`（`add` 类型不传；`update`/`delay` 必填）
- [x] 1.3 `AdjustRPDemandReqElem.Validate()` 按 `adjust_type` 分流：
  - **`add`**：仅校验 `updated_info`；`demand_id`、`original_info` 必须为空
  - **`update`**：强校验 `demand_id + original_info + updated_info`；允许 `expect_time` 与资源量同时变更
  - **`delay`**：强校验 `demand_id + expect_time`；`original_info`/`updated_info` 可不传（后端按 ID 查库）；**不在此暴露 `delay_os`**
- [x] 1.4 `update`/`delay` 与 `add` 互斥校验：`add` 不得带 `demand_id`；`update`/`delay` 不得 `demand_id` 为空

## 2. 提单校验放宽（service/plan/demand_update.go）

- [x] 2.1 `validateAdjustResPlan()`：对 `adjust_type=add` 或 `original_info==nil` 的条目跳过滚服等基于原需求的校验；既有需求条目的滚服限制不变

## 3. 提单与构造（logics/plan/demand_adjust.go）

- [x] 3.1 `AdjustBizResPlanDemand()`：先过滤空 `demand_id` 再做 `AreAllDemandBelongToBiz` / `ExamineDemandClass`；整批均为 `add` 时使用请求级 `DemandClass`；整批 `add` 且 `demand_class` 为空 → `InvalidParameter`
- [x] 3.2 校验整批 `demand_class` 一致（CVM/CA 不可混）；混批时忽略请求级 `demand_class`，以 DB 推导为准
- [x] 3.3 纯新增（`add`）条目不参与 lock（lock 仅含 `Original` 的 demand）
- [x] 3.4 `constructAdjustReq()` 按 `adjust_type` 分流：
  - **`add`**：`updated_info` → `[]CreateResPlanDemandReq`，调用 `buildDemandsFromCreateReq()`（`Original=nil`）
  - **`update`** → `constructUpdateDemands()`
  - **`delay`** → `constructDelayDemands()`
  - lock 遍历仅覆盖 `demand.Original != nil`

## 4. 主单落库校验（logics/plan/types.go）

- [x] 4.1 `CreateResPlanTicketReq.Validate()`：调整主单允许部分 demand 满足 `Original == nil && Updated != nil`（纯新增）；非法全 `Updated==nil` 组合仍拒

## 5. 拆单路由（logics/plan/splitter/adjust.go）

- [x] 5.1 `getDemandsWithoutTransfer()` 路由调整：
  - 仅改期且数量不变（同机型、**非跨年**）→ **延期**
  - 改期且改量（含 10→9@8）/ 换机型 / 仅改量不改期 → **常规 adjust（调增/调减）**
  - **跨年改期**仍走 adjust（沿用现有额度逻辑）
  - `delay` 类型提单（整单延期）→ **延期**
- [x] 5.2 确认纯新增（`Original==nil`）追加子单已由 `splitAdjustDemandsToAddAndDelete()` 支持，`add.go` / `sub_ticket.go` 无需改动

## 6. 单元测试

- [x] 6.1 `cmd/woa-server/types/plan/demand_test.go`：`add`/`update`/`delay` 分流校验
- [x] 6.2 `cmd/woa-server/logics/plan/demand_adjust_test.go`：ID 收集、lock 过滤、`resolveAdjustDemandClass`
- [x] 6.3 `cmd/woa-server/logics/plan/splitter/adjust_test.go`：仅改期→delay、改期改量→adjust、纯新增→add
- [x] 6.4 `cmd/woa-server/types/plan/demand_test.go`：提交语义校验

## 7. 编译与自测

- [x] 7.1 `go build ./...` 编译通过；`goimports -w` 格式化相关文件
- [x] 7.2 自测：改期+改量、`add` 纯新增、混批（delay/update + add）、整批 `add` + `demand_class`、拆单路由（delay vs adjust）

## 9. 提交语义校验（F-001 后端兜底，不完全信任前端 adjust_type）

- [x] 9.1 在 `demand.go` 增加语义校验：对比 `original_info` / `updated_info` / `expect_time` 识别时间变更 vs 资源变更
- [x] 9.2 `update`：`original` 与 `updated` 无差异 → 拒绝；**仅改期** → 拒绝并提示应使用 `delay`；改期+改量或仅改量 → 通过
- [x] 9.3 `delay`：不得携带 `updated_info`；若传 `original_info` 则 `expect_time` 必须与原期望时间不同
- [x] 9.4 `AdjustRPDemandReqElem.Validate()` 末尾调用语义校验
- [x] 9.5 单测：`demand_test.go` 覆盖上述场景
