## 1. API 协议（pkg/api/woa-server/cvm_apply_recommend.go）

- [x] 1.1 新增请求结构体 `ApplyRecommendByPlanReq`（必填 `limit`；可选 A 类 `require_type`/`region`/`device_type`/`image_id`、B 类 `zone`/`res_assign`/`replicas`），实现 `Validate()`（含 `RequireType`/`ResAssign` 非空校验）；注：与接口1 `ApplyRecommendByStaticReq` 不同，不含 `bk_username` 字段
- [x] 1.2 复用既有 `ApplyRecommendByStaticResp` / `ApplyRecommendItem` / `ApplyRecommendSuborder` 作为响应结构（不新增）；`ApplyRecommendItem.Source` 字段补充 `omitempty` tag

## 2. 库存校验 helper 抽取（cmd/woa-server/service/task/recommend.go）

- [x] 2.1 将 `queryCapacitySatisfied` / `filterCandidatesByCapacity` 抽取为以 `capacityTriple{requireType, region, deviceType}` 三元组为输入的共用形态，保留 `buildCapacityKey` / `NotNeedVerifyCapacity` 跳过与 zone 联动逻辑
- [x] 2.2 确认接口1 `GetBizApplyRecommendByStatic` 改用共用 helper 后行为与 SQL 不变

## 3. 预测匹配 Logics（cmd/woa-server/service/task/recommend.go）

- [x] 3.1 调 `planLogics.GetProdResRemainPoolMatch(kt, bkBizID, requireType, "")` 取预测余量池
- [x] 3.2 确定 region 集合：入参 region 非空取该 region，否则遍历池内所有 `RegionID` 去重
- [x] 3.3 预测内 `GetPlanTypeAvlDeviceTypesV2(PlanTypeCodeInPlan, ...)` 逐 region 收集 `Available` 候选与 `RemainCore`
- [x] 3.4 全局回退：预测内全空时整体改 `PlanTypeCodeOutPlan` 重跑，命中候选计费模式置 `POSTPAID_BY_HOUR`，否则 `PREPAID`
- [x] 3.5 device_type 过滤（A 类）+ 余量门槛 `RemainCore ≥ applyNum × cpuCore`（`cpuCore` 取自 `GetAllDeviceTypeMap`）

## 4. image_id 回查 helper（cmd/woa-server/service/task/recommend.go）

- [x] 4.1 实现 `(require_type, region, device_type)` → 最近 image_id 回查：`ZiyanCvmApplySuborder.List` 过滤三元组 + `image_id != ''`，按 `created_at` 倒序取首条
- [x] 4.2 image_id 优先级：入参 → 回查 → 默认 `cvmapi.DftImageID`；按候选三元组去重回查并缓存

## 5. 接口实现 Handler（cmd/woa-server/service/task/recommend.go）

- [x] 5.1 新增 Handler `GetBizApplyRecommendByPlan`：解析路径 `bk_biz_id`、解码与校验请求、业务访问鉴权（`Biz`/`Access`）
- [x] 5.2 参数归一：`require_type` 未传归一 `RequireTypeRegular(1)`；`applyNum` 取入参/默认配置；`limit` 取入参
- [x] 5.3 串联：预测匹配 → device_type/余量门槛过滤 → 库存校验（共用 helper）→ image_id 与默认值补全 → 按 `RemainCore` 倒序去重（key `region|device_type`）取前 limit 组装方案
- [x] 5.4 边界：A 类过滤无匹配 / 预测或库存全不足 → 返回空 `Items`

## 6. 路由注册（cmd/woa-server/service/task/service.go）

- [x] 6.1 在 `bizService` 注册 `POST /apply/recommend/by_plan` → `s.GetBizApplyRecommendByPlan`

## 7. 验证与文档

- [x] 7.1 `go build` / `go vet` 通过
- [ ] 7.2 自测：未传 zone / 传 zone / A 类过滤无匹配 / 预测内命中 PREPAID / 预测内全空回退预测外 POSTPAID / 余量门槛剔除 / 库存不足剔除 / image_id 入参与回查与默认 等场景（需运行环境）
- [x] 7.3 回归确认接口1 `GetBizApplyRecommendByStatic` 行为不变（库存 helper 抽取后等价）
- [x] 7.4 新增接口文档 `docs/api-docs/api-server/docs/zh/get_biz_apply_recommend_by_plan.md`（版本 v9.9.9）
