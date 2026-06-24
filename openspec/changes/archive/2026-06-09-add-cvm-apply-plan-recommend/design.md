## Context

接口1（`GetBizApplyRecommendByStatic`，已归档 `add-cvm-apply-static-recommend`）基于静态推荐表给出单子单方案。接口2 是其孪生接口：**候选来源换成实时预测余量池**，其余（库存校验、可用区联动、默认值补全、单子单方案结构）尽量复用。

技术方案出处：iWiki 4021537729「接口2: 预测余量 + 库存情况推荐」。

依赖前置（已就绪，不在本变更范围）：
- 预测余量能力 `planLogics.GetProdResRemainPoolMatch` / `GetPlanTypeAvlDeviceTypesV2` / `GetAllDeviceTypeMap` 已具备（现有 `GetCvmChargeTypeDeviceTypeV2` 即基于此）。
- `service` struct 已注入 `planLogics`（`service.go` 中 `planLogics: c.PlanController`），**接口2 无需新增注入**（区别于技术方案改动清单的描述）。
- 库存静态表 `device_capacity`、子单表 `ziyan_cvm_apply_suborder`（含 `image_id`）、配置 `cc.ApplyRecommend.DefaultApplyNum`、默认镜像 `cvmapi.DftImageID` 均已就绪。

## Goals / Non-Goals

**Goals:**
- 新增业务视角接口 `POST /bizs/{bk_biz_id}/task/apply/recommend/by_plan`，返回若干单子单推荐方案。
- 复用既有预测能力，不重写预测逻辑；复用接口1 的库存校验与方案组装资产。
- 计费模式随命中预测池来源（预测内 PREPAID / 预测外 POSTPAID_BY_HOUR），预测内全空时全局回退预测外。

**Non-Goals:**
- 预测余量能力本身的重写。
- 静态推荐表查询、离线统计改造、SQL 迁移、刷历史脚本、新建 data-service 接口。
- MCP 工具化、Graph 编排、Skill 沉淀（父需求其他子项）。
- 修改既有 `GetBizApplyRecommendTop` / `GetBizApplyRecommendByStatic` / `GetCvmChargeTypeDeviceTypeV2` 行为。

## Decisions

### 决策 1：落点 service/task，路由 by_plan
新接口全部实现放在 `cmd/woa-server/service/task/recommend.go`（Handler、预测匹配 Logics、image_id 回查 helper 均在此文件），与接口1 同包同文件，`bizService` 注册 `POST /apply/recommend/by_plan`。
- 路由名取 `by_plan`：对齐技术方案接口契约表（`recommend/by_plan`）与接口1 的 `by_static_recommend` 命名风格。技术方案「改动清单」里出现的 `forecast_plan` 系为规避「与接口1 `recommend/plan` 冲突」的早期措辞，但接口1 实际落地为 `by_static_recommend`，不存在冲突，故采用契约表的 `by_plan`。
- 备选：放 `service/cvm`（本地需求文档措辞）。因接口1 已在 service/task 且需复用其库存校验私有 helper，选 service/task。

### 决策 2：预测匹配（复用 GetCvmChargeTypeDeviceTypeV2 的调用范式）
1. `planLogics.GetProdResRemainPoolMatch(kt, bkBizID, requireType, "")` → `prodMaxAvailable` 池（按 `ResPlanPoolKeyV2{RegionID, DeviceType, ...}` 聚合）。
2. region 集合：入参 `region` 非空 → 仅该 region；否则遍历 `prodMaxAvailable` 池内出现的所有 `RegionID` 去重。
3. 预测内：对每个 region 调 `GetPlanTypeAvlDeviceTypesV2(PlanTypeCodeInPlan, req{BkBizID, RequireType, Region}, prodMaxAvailable)` → `[]DeviceTypeAvailable{DeviceType, Available, RemainCore}`，收集 `Available==true` 的候选。
4. **全局回退**：所有 region 预测内均无候选时，整体改用 `PlanTypeCodeOutPlan` 重跑步骤 3，命中候选计费模式置 `POSTPAID_BY_HOUR`；否则计费模式为 `PREPAID`。计费↔预测内外映射复用 `GetPlanTypeByChargeType`。
- 备选：按 region 各自回退。技术方案表述为「预测内无候选则回退预测外」，按整体语义采用全局回退（用户已确认）。

### 决策 3：余量门槛 + device_type 过滤
- 机型核数 `cpuCore = GetAllDeviceTypeMap()[deviceType].CpuCore`。
- 余量硬门槛：保留 `RemainCore ≥ applyNum × cpuCore` 的候选；`cpuCore ≤ 0` 视为无法计算，剔除。
- A 类 `device_type` 入参非空时，仅保留该机型候选；A 类传了但无任何匹配候选 → 直接返回空。

### 决策 4：库存校验抽取为共用 helper
- 将接口1 现有的 `queryCapacitySatisfied` / `filterCandidatesByCapacity` / `buildCapacityKey` 逻辑抽取为不依赖 `staticRecommendCandidate` 具体类型的共用形态（以 `(requireType, region, deviceType)` 三元组为输入），接口1/2 共用。
- 校验规则与接口1 一致：`capacity ≥ applyNum`；未传 zone → region 下任一 zone 满足即保留、不回填具体 zone；传了 zone → 按该 zone 校验、不足剔除；`require_type.NotNeedVerifyCapacity()` 跳过校验。
- 备选：接口2 复制一份。为避免重复逻辑、保证两接口库存语义一致，选择抽取共用。

### 决策 5：image_id 补全（接口2 特有）
优先级：入参 `image_id` → 历史子单回查 → 默认 `cvmapi.DftImageID`。
- 历史子单回查：`ZiyanCvmApplySuborder.List`，过滤 `(require_type, region, device_type)`、`image_id != ''`，按 `created_at` 倒序取最近一条的 `image_id`。
- 为减少 N 次查询，按候选去重的 `(require_type, region, device_type)` 批量/逐项回查并缓存（候选数 ≤ limit，量级可控）。

### 决策 6：候选去重、排序与组装
- 去重 key：`region|device_type`（同一 region 同机型只保留余量最高一条）。
- 排序：按 `RemainCore` 倒序；取前 `limit` 个。
- 复用接口1 的响应结构 `ApplyRecommendByStaticResp` / `ApplyRecommendItem` / `ApplyRecommendSuborder`；`source` 字段对接口2 无 user/biz 语义，置空或预测来源标识（保持结构一致，值留空）。
- 默认值：计费模式随预测池来源；可用区/res_assign 联动同接口1；系统盘 `CLOUD_PREMIUM/100G/1`、数据盘 `CLOUD_PREMIUM/500G/1`（复用 `constant.Recommend*` 常量）。

### 决策 7：请求结构体
新增 `ApplyRecommendByPlanReq`，与 `ApplyRecommendByStaticReq` 基本一致但有一处差异：**不含 `bk_username` 字段**（接口2 通过 session/header 获取调用方身份，无需显式传入）。必填 `limit`；A 类 `require_type/region/device_type/image_id`；B 类 `zone/res_assign/replicas`；`require_type` 未传时归一为 `RequireTypeRegular(1)`。

## Risks / Trade-offs

- [预测余量为实时计算，P99 未定（Q-001）] → 本期接口与 `GetCvmChargeTypeDeviceTypeV2` 同源，性能特征一致；指标待后续压测确认。
- [全局回退 vs 按 region 回退语义差异] → 已与用户确认采用全局回退；如后续需要按 region 细化，回退逻辑集中在一处便于调整。
- [image_id 回查多次查库] → 候选数 ≤ limit（≤20），按三元组去重后回查量级可控；命中入参或默认时不查库。
- [抽取库存 helper 影响接口1] → 抽取后接口1 调用保持等价；通过对照确认接口1 行为不变。
- [余量仅作门槛与排序不返回] → 响应不含余量字段，符合技术方案边界条件。

## Open Questions

- Q-001：推荐/试算接口 P99 响应时间上限指标未明确（沿用父需求未解决问题，不阻塞本变更）。
