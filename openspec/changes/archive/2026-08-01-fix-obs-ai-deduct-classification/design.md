## Context

原始账单 OBS 同步已按 `MatchAPIBrandName` + `isAwsGPU`/`isGcpGPU` + `GetOBSResClassIDByType`（API > GPU > CPU）填充 `ResClassId` / `APIBrandName` / `GpuCardCategory`。AI 扣减月任务将 `HcProductCode/Name` 覆盖为 `AIDeduct` 并取反金额，extension 仍保留云上明细。当前 convert 仅用 `HcProductName` 做品牌与 AI 前缀判定，扣减条目落入 CPU 分类。

需求文档：`docs/reqs/API扣减归类.md`（TAPD 1069995598136192578）。

## Goals / Non-Goals

**Goals:**
- AWS/GCP AI 扣减条目 OBS 归类与字段填充与对应原始账单同口径
- 覆盖 API 类与 GPU 卡类型 AI 扣减；禁止凡 AIDeduct 一律 API
- 识别失败走与原始相同降级；不阻断整批同步；成本不变；不回刷历史

**Non-Goals:**
- 历史账期强制回刷
- 修改 AI 扣减业务规则（取反、账号分摊）
- 华为、OBS 平台报表
- 修改原始非扣减账单归类规则

## Decisions

### 决策 1：在 OBS convert 侧还原判定输入，不改月任务产品码

**选择**：保持 `AIDeduct` 产品码（避免被当成原始账单重复参与 AI 扣减拉取）；在 `convertAwsBill` / `convertGcpBill` 识别 `AIDeduct` 后，从 extension 文本与 AI 血缘信号还原品牌/GPU 判定输入。

**备选**：月任务保留带 `_HCM_AI_` 的 `HcProductName` —— 可能破坏「防被当成原始账单」的意图，波及更大。

**理由**：改动面小、与现有月任务契约兼容，extension 已含 `product_product_name` / `sku_description` 等可识别文本。

### 决策 2：AWS 品牌与 GPU 信号还原规则

对 `HcProductCode` 或 `HcProductName` 为 `AIDeduct` 的条目：

1. **APIBrandName**：依次对 `ProductProductName`、`LineItemLineItemDescription`、原 `HcProductName` 调用 `MatchAPIBrandName`，取首个非空
2. **isGPU 用的 hcProductName**：使用 `constant.BillItemAIPrefix` 占位，恢复原始链路「AI 前缀 ⇒ isGPU」分支（扣减源单拉取条件即为 `_HCM_AI_` 前缀）
3. **GpuCardCategory**：仍走既有 `lookupAwsGpuCardCategory(ProductProductName, ProductInstanceType, awsGpuMap)`
4. **ResClassId**：`GetOBSResClassIDByType(vendor, isGPU, apiBrandName != "")`，优先级不变

### 决策 3：GCP 品牌与 GPU 信号还原规则

对 `AIDeduct` 条目：

1. **APIBrandName**：`resolveGcpAPIBrandName` 在 `HcProductName` 未命中时已兜底 `SkuDescription`；若 `HcProductName` 为 `AIDeduct`，优先对 `SkuDescription` 匹配，再尝试 extension 内其他可读描述字段（与现有双路径一致）
2. **isGPU**：若 `HcProductName` 为 `AIDeduct`，按 AI 前缀占位参与 `isGcpGPU` 判定（与 AWS 对称）；卡型仍由 `lookupGcpGpuCardCategory(SkuDescription, ...)` 填充
3. **ResClassId**：同 `GetOBSResClassIDByType`

### 决策 4：降级与可观测性

品牌与卡型均无法识别时：不强制 API；若 AI 血缘使 `isGPU=true` 则落 GPU，否则落 CPU（与原始「有 AI 前缀无品牌 → GPU」边角一致）。打 `Warnf` 日志（含 main_account、产品码），不 return 错误。

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| extension 缺失导致仍归 CPU | Warn 日志；源单既有 AI 前缀血缘时用前缀占位抬升 isGPU |
| 误把非 AI 的 AIDeduct 当 AI | 产品码仅月任务写入，无其他写入方 |
| GCP 无 extension 的 HCM 生成路径 | 依赖 SkuDescription/兜底；单测覆盖 |

## Migration Plan

- 上线后对新同步/重同步账期生效
- 不主动回刷历史；发布说明注明历史月可能仍有展示差异
- 回滚：还原 convert 判定逻辑即可

## Open Questions

无（澄清阶段已闭合）
