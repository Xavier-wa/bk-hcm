## 1. Proto 定义

- [x] 1.1 在 `pkg/api/woa-server/cvm_apply_recommend.go` 新增 `ApplyRecommendSplitSubOrderReq`：必填 require_type / region / zone / device_type / image_id / res_assign / replicas / system_disk / data_disk，可选 `OccupiedSuborders []*ApplyRecommendSuborder`
- [x] 1.2 实现 `ApplyRecommendSplitSubOrderReq.Validate()`：struct tag 校验 + require_type/res_assign 枚举校验 + occupied 子单 require_type 一致性校验
- [x] 1.3 新增 `ApplyRecommendSplitSubOrderResp`：扁平 `{ Suborders []*ApplyRecommendSuborder }`
- [x] 1.4 在 `pkg/thirdparty/cvmapi/cvmapi_request.go` 新增 `ChargeType.ToPlanType()`（按量计费→预测外，其余→预测内）作为计费↔预测映射的统一入口；`cmd/woa-server/logics/plan/types.go` 的 `GetPlanTypeByChargeType` 重构为复用该方法

## 2. 库存数值查询（升级布尔为数值上限）

- [x] 2.1 在 `service/task/recommend.go` 新增 `querySplitCapacityLimit`：按 `(require_type, region, device_type)` 查 `device_capacity` 返回各 zone 容量上限 map（同一 zone 多行取最大 capacity）；zone=all 查 region 下所有 zone、具体 zone 仅查该 zone
- [x] 2.2 支持扣减 `occupiedStock`（按 require_type/region/device_type/zone 聚合台数）：zone=all 占用为浮动占用统一扣、具体 zone 占用只扣对应 zone，扣减后截断 0 得有效库存上限

## 3. 增量拆分占用预处理

- [x] 3.1 构建 `occupiedCore` 族级映射：`(region, TechnicalClass, CoreType, 内/外)` → 核数，族信息取自 `GetAllDeviceTypeMap`，预测内/外由子单 charge_type 经 `ChargeType.ToPlanType()` 推导（裁撤经 `normalizePlanType` 合并到同一 `PlanTypeCodeIgnore` key）
- [x] 3.2 构建 `occupiedStock` 机型级映射：`(require_type, region, device_type, zone)` → 台数
- [x] 3.3 占用子单 require_type 一致性校验（不一致返回 InvalidParameter）；占用子单机型未知/禁用（cpu_core 取不到）直接返回错误

## 4. 预测余量与数量分配

- [x] 4.1 仅对 `NeedVerifyResPlan` 类型调用 `GetProdResRemainPoolMatch` + `GetPlanTypeAvlDeviceTypesV2` 取本机型预测内/外 RemainCore
- [x] 4.2 扣减族级 occupiedCore（截断 0），按 `余量核数 / cpuCore` 向下取整得 nPrepaid / nPostpaid
- [x] 4.3 裁撤(3) 内外合并为单池只算一次 nPool；非预测类型 nPrepaid = replicas
- [x] 4.4 数量分配：takePrepaid = min(replicas, nPrepaid)、takePostpaid = min(replicas-takePrepaid, nPostpaid)
- [x] 4.5 用有效库存上限按「预测内 → 预测外」顺序对总量封顶（绿通跳过库存）

## 5. Handler 与组装

- [x] 5.1 新增 `GetBizApplyRecommendSplitSubOrder` Handler：解析 bk_biz_id、解码校验、业务访问鉴权（`Biz`/`Access`）
- [x] 5.2 串联 Logics：占用预处理 → 预测余量 → 库存封顶 → 数量分配 → 组装子单
- [x] 5.3 组装子单 A(PREPAID, takePrepaid) / B(POSTPAID, takePostpaid)，丢弃数量 ≤ 0 子单；可分配总量 < 1 或请求机型 cpu_core ≤ 0 返回空；不回显 occupied
- [x] 5.4 在 `service/task/service.go` 的 `bizService` 注册路由 `POST /apply/recommend/split_suborder`

## 6. 接口文档与验证

- [x] 6.1 在 `docs/api-docs/api-server/docs/zh/get_biz_apply_recommend_split_suborder.md` 新增接口文档（版本 v9.9.9+，含请求/响应示例与 occupied_suborders 增量场景）
- [x] 6.2 `go build ./...` 编译通过，`goimports -w` 格式化
- [x] 6.3 自测覆盖：常规预测内外拆 2 子单、绿通跳库存、裁撤单池、滚服不校验预测、occupied 族级扣减、zone=all 各 zone 之和封顶、可分配 < 1 返回空
