## Why

接口1（离线偏好叠加库存）已交付，但「预测内资源优先消费」场景无法覆盖：离线静态推荐表只反映历史偏好，不感知业务当月实时预测余量。本变更新增接口2，在实时预测余量之上叠加库存校验与默认值补全，把预测余量机型补全为「可直接下单的单子单完整方案」，供主机申领 AI Agent 优先消费预测内资源、快速完成申领。

## What Changes

- 新增业务视角接口 `POST /bizs/{bk_biz_id}/task/apply/recommend/by_plan`：候选来源为**实时预测余量**（不读静态推荐表），叠加库存校验 + 默认值补全，返回若干推荐方案，每方案含 1 个子单。
- 复用既有预测能力 `planLogics.GetProdResRemainPoolMatch` / `GetPlanTypeAvlDeviceTypesV2` / `GetAllDeviceTypeMap`，不重写预测逻辑（`service` 已注入 `planLogics`，无需新增注入）。
- 计费模式与预测内/外联动：默认包年包月（PREPAID）匹配预测内池；**所有 region 预测内均无候选时全局回退预测外池**，计费模式转按量计费（POSTPAID_BY_HOUR）。
- 余量硬门槛：候选保留条件 `预测余量 ≥ 申请数量 × 机型核数`；余量仅用于门槛与排序，不在响应中返回。
- 抽取接口1 现有库存校验逻辑为共用 helper，接口2 复用静态表 `device_capacity` 校验与可用区联动规则。
- image_id 补全：入参 → 历史子单按 `(require_type, region, device_type)` 回查最近 image_id → 默认 `cvmapi.DftImageID`。
- 不含：image_id 字段扩展与回填（已就绪）、离线统计改造、静态推荐表查询、新建 data-service 接口、MCP 工具化 / Graph 编排 / Skill 沉淀（父需求其他子项）。

## Capabilities

### New Capabilities
<!-- 无新增独立 capability，复用现有推荐域 -->

### Modified Capabilities
- `apply-recommend`: 新增「预测余量叠加库存的在线单子单推荐查询接口」相关需求（在现有离线推荐 + 接口1 之上叠加实时预测余量来源的方案组装）。

## Impact

- 受影响 specs：`apply-recommend`
- 受影响代码：
  - `pkg/api/woa-server/cvm_apply_recommend.go`（新增 `ApplyRecommendByPlanReq` 及其 `Validate()`；复用既有 `ApplyRecommendItem` / `ApplyRecommendSuborder` / `ApplyRecommendByStaticResp`）
  - `cmd/woa-server/service/task/recommend.go`（新增 Handler + 预测匹配 Logics + image_id 回查 helper；抽取库存校验为共用 helper 供接口1/2 复用）
  - `cmd/woa-server/service/task/service.go`（`bizService` 注册路由 `/apply/recommend/by_plan`）
  - `docs/api-docs/api-server/docs/zh/get_biz_apply_recommend_by_plan.md`（接口文档，版本 v9.9.9）
- 复用（无需改动）：`planLogics`（`c.PlanController` 已注入）、`cc.ApplyRecommend.DefaultApplyNum`（已存在）、`cvmapi.DftImageID`、`DeviceCapacity.List`、`ZiyanCvmApplySuborder.List`。
- 不涉及：SQL 迁移、离线统计、刷历史脚本、配置新增。
