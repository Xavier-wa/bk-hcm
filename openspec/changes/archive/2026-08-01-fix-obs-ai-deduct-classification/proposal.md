## Why

AI 扣减（`AIDeduct`）条目在同步到 OBS 时，因 `HcProductName` 被改写为字面量 `AIDeduct`，导致品牌/GPU 判定输入丢失，`ResClassId` 错误回落到 CPU（外租服务），与对应原始账单的 API/GPU 归类不一致。反馈明确不影响实际总成本，但核算展示正负分属不同资源类型；需让扣减条目与原始账单使用同一套 OBS 归类与字段填充口径。

## What Changes

- 修正 AWS/GCP OBS 同步链路：对 AI 扣减条目按与原始账单一致的规则计算 `ResClassId`（API > GPU > CPU），并填充 `APIBrandName`、`GpuCardCategory`
- 禁止「凡 `AIDeduct` 一律 API-OFS」；须覆盖 API 类与 GPU 卡类型 AI 扣减
- 识别失败时走与原始账单相同的降级路径（不强制 API），不阻断整批同步
- 不回刷历史已同步账期；成本冲销口径不变
- 补充单元测试覆盖 API/GPU 扣减归类场景

## Capabilities

### New Capabilities
- `obs-ai-deduct-classification`: AI 扣减条目 OBS 归类对齐——AWS/GCP 同步时对 `AIDeduct` 使用与原始账单同口径的资源分类与品牌/卡型字段填充

### Modified Capabilities

（无；前序 `obs-bills-add-gpu-card-api-brand` 等能力定义原始账单填充行为，本期新增扣减对齐能力，不修改其既有 requirement 文本）

## Impact

- **代码**：`cmd/task-server/logics/action/obs/sync/sync_aws.go`、`sync_gcp.go`、`gpu_lookup.go`（及对应 `_test.go`）；必要时轻量调整 `aws_ai_deduct.go` / `gcp_ai_deduct.go`（仅当 OBS convert 侧无法从 extension 还原判定输入时）
- **数据**：OBS AWS/GCP 账单表 `ResClassId` / `APIBrandName` / `GpuCardCategory` 对 AI 扣减条目的填充结果
- **API / 对外接口**：无
- **依赖系统**：OBS 账单库（既有同步写入）
- **范围外**：历史回刷、华为、OBS 报表改造、AI 扣减业务规则本身
