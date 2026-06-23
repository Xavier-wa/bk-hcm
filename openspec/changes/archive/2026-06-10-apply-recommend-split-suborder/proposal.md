## Why

接口1/2（`by_static_recommend` / `by_plan`）只能产出「单子单、数量取入参」的推荐方案，无法把一个确定方案按计费模式拆成可直接下单的主单 + 多子单。在「Agent 给出方案 → 用户确认 → 提单」的闭环中，缺少「先按预测内/预测外余量与库存把申请数量拆分试算、再提单」的细化环节。接口3 补齐这一拆单试算能力（纯试算、不落库）。

## What Changes

- 新增业务视角拆单试算接口 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/split_suborder`，落在 `service/task`，与接口1/2 同族；调用前经业务访问鉴权（`Biz`/`Access`）。
- 入参为一个确定方案的全部下单参数（需求类型、地域、可用区、机型、镜像 image_id、资源分配方式、申请数量、系统盘、数据盘，均必填），可选 `occupied_suborders`（已占用子单数组，支持增量拆分）。
- 以**计费模式**为唯一拆分轴：预测内余量 → 包年包月（PREPAID）子单，预测内装不下的部分溢出预测外余量 → 按量计费（POSTPAID_BY_HOUR）子单；一个入参组合最多拆 2 个子单。
- 按需求类型差异化校验：常规(1)/春保(2)/短租(9) 校验预测内外 + 库存；机房裁撤(3) 预测内外合并为单池只算一次、1 子单 PREPAID + 库存；滚服(6)/春保资源池(8) 不校验预测、1 子单 PREPAID + 库存；小额绿通(7) 不校验预测、1 子单 PREPAID + 跳过库存。
- `occupied_suborders` 支持增量拆分：先扣减已占用的**预测余量（机型族级共享）**与**库存（机型级，按 zone 区分 `zone=all` 浮动占用与具体 zone 占用）**，再基于剩余资源计算本次增量子单；仅返回新算出的增量子单，不回显传入的占用子单。占用子单机型无效（cpu_core 取不到）直接返回错误。
- 复用既有能力（`planLogics.GetProdResRemainPoolMatch` / `GetPlanTypeAvlDeviceTypesV2` / `GetAllDeviceTypeMap`、`DeviceCapacity.List`、`RequireType.NeedVerifyResPlan` / `NotNeedVerifyCapacity`），**不新增 data-service 接口、不改 DB、不改离线统计、不落库**。
- 计费模式 ↔ 预测内/外映射统一收敛到新增的 `cvmapi.ChargeType.ToPlanType()`（按量计费→预测外，其余→预测内），`cmd/woa-server/logics/plan/types.go` 的 `GetPlanTypeByChargeType` 重构为复用该方法，避免 switch 散落多处。

## Capabilities

### New Capabilities
<!-- 无新增能力，接口3 作为既有能力的新增 Requirement -->

### Modified Capabilities
- `apply-recommend`: 新增「主机申请单据拆分试算」相关 Requirement（拆单接口契约、按计费模式拆分、需求类型差异化校验、库存数值封顶、增量拆分占用扣减），与既有离线推荐 / 接口1 / 接口2 的 Requirement 并存、互不改动。

## Impact

- 新增 proto：`pkg/api/woa-server`（`ApplyRecommendSplitSubOrderReq` / `ApplyRecommendSplitSubOrderResp`，复用 `ApplyRecommendSuborder` 作为子单与占用结构）。
- 新增实现：`cmd/woa-server/service/task` 的 `GetBizApplyRecommendSplitSubOrder` Handler + Logics，注册进现有 `bizService`（路由 `POST /apply/recommend/split_suborder`）；`s.planLogics` 已由接口2 注入，无需改 `InitService`。
- 新增/重构公共能力：`pkg/thirdparty/cvmapi` 新增 `ChargeType.ToPlanType()`，`cmd/woa-server/logics/plan/types.go` 的 `GetPlanTypeByChargeType` 重构为复用该方法。
- 新增接口文档：`docs/api-docs/api-server/docs/zh/get_biz_apply_recommend_split_suborder.md`（版本 v9.9.9+）。
- 不影响既有提单/落库链路（纯试算）。
