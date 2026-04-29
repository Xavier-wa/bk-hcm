## Context

账单调整（BillAdjustment）是平台手动录入调账费用的核心功能，支持对二级账号的账单进行增减调整。调账明细在确认后会通过 OBS 同步任务将数据同步至 OBS 数据库，OBS 中通过 `ResClassId` 字段区分 CPU/GPU 资源类型进行分类统计。

**当前问题**：账单调整表缺少资源类型字段，调账明细同步 OBS 时 `ResClassId` 始终为 0（未设置），导致 OBS 无法按 CPU/GPU 分类核算调账数据。

**涉及厂商**：AWS / 华为云 / GCP（均有 `ResClassId` 字段）；Zenlayer OBS 表无此字段，不受影响。

## Goals / Non-Goals

**Goals:**
- 账单调整明细支持记录资源类型（CPU / GPU）
- 创建调账时 `res_class` 必填
- OBS 同步时根据 `res_class` 正确设置 `ResClassId`
- OBS 同步时根据主账号站点正确设置 `CityId`（顺带修复）
- Excel 导出展示资源类型列
- 存量数据空值默认视为 CPU（兼容）

**Non-Goals:**
- 不按 CPU/GPU 分组修改汇总统计（sum.go）
- 不修改 Zenlayer OBS 同步逻辑（表无 ResClassId 字段）
- 不对存量数据做数据迁移（空值在运行时按 CPU 处理）

## Decisions

### 1. 字段命名：`res_class` 而非 `res_type`

**决策**：DB 列名 `res_class`，Go 字段名 `ResClass`，枚举类型 `BillAdjustmentResClass`。

**理由**：与现有 OBS 表字段 `ResClassId` 保持语义一致，明确表达"资源分类"含义，避免与已有的 `type`（调账方向：increase/decrease）混淆。

### 2. 新增独立枚举字段，而非扩展 `type`

**决策**：新增独立的 `res_class` 字段，不扩展现有 `BillAdjustmentType`。

**理由**：`type`（增/减方向）与 `res_class`（资源类型）是两个正交维度，合并会导致枚举值爆炸（4 个值），且破坏现有 `GetCost()` 逻辑及存量数据。

**备选**：扩展 type 为 `increase_cpu/increase_gpu/decrease_cpu/decrease_gpu`，因破坏性大而放弃。

### 3. 创建时必填，存量数据兼容为 CPU

**决策**：
- DB 列 `DEFAULT ''`（允许空字符串，兼容存量数据）
- 新建接口 `validate:"required"`（强制前端/调用方填写）
- OBS 同步时 `res_class == ""` 视为 CPU

**理由**：DB 层不设 `NOT NULL` 约束，保证存量数据不受 schema 变更影响；业务层通过 API 校验保证新数据质量；运行时的 CPU 默认值符合历史数据的实际情况（历史调账大部分为 CPU 资源）。

### 4. OBS 同步：复用 `GetOBSResClassID`

**决策**：在 `sync_adjustment.go` 的 `convertAws/Huawei/Gcp` 中，直接调用已有的 `enumor.GetOBSResClassID(vendor, isGPU)` 函数。

**理由**：该函数已封装了各厂商 CPU/GPU 的 ResClassId 映射，无需重复实现。

### 5. OBS 同步：同步修复 CityId 设置

**决策**：在 `sync_adjustment.go` 的 `convertAws/Huawei/Gcp` 中，同步根据主账号 `site` 字段设置 `CityId`（国内站 → `OBSDefaultCityIDChina`，国际站 → `OBSDefaultCityIDOverseas`）。

**理由**：历史代码三个 convert 函数均未设置 `CityId`（AWS/华为已有 site 判断但未赋值，GCP 甚至无 site 判断），此次顺带修复，避免 OBS 侧城市归类缺失问题。GCP 新增 site 判断逻辑与 AWS/华为保持一致。

## Risks / Trade-offs

- **存量数据默认 CPU** → 如有已存在的 GPU 调账数据（通过旧接口录入），OBS 同步时会错误归类为 CPU。缓解：通过排查存量数据或人工确认后修正 res_class 值。
- **Zenlayer 无 ResClassId** → Zenlayer 调账可以填写 res_class（展示用），但同步 OBS 时不会体现。这是 Zenlayer OBS 表的结构限制，接受此行为。
- **必填影响调用方** → 所有创建调账的客户端需同步更新请求参数。通过 API validate 报错引导调用方适配。
