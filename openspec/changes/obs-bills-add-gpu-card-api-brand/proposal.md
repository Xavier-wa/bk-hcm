## Why

OBS 上报系统已在前序变更 `obs-bills-add-city-resclass` 中为三张 OBS 账单表补充了城市 ID（`CityId`）与资源分类 ID（`ResClassId`）。但 `ResClassId` 仅能区分一条账单是否属于 GPU 资源，**无法体现具体的 GPU 卡型**（T4/A10G/L40S/L4），也**无法体现 AI 类账单背后的 API 厂商**（gemini/claude/kimi/jina）。OBS 方要求 6 月账单（7 月 2 日出账）中即包含这两个细分维度，用于按 GPU 卡型与 API 厂商进行精细化成本核算。

## What Changes

- 在 `obs_aws_bills`、`obs_gcp_bills`、`obs_huawei_bills` 三张表新增 `GpuCardCategory`（GPU 卡型，字符串）和 `APIBrandName`（API 厂商，字符串）两个字段，随 OBS 上报。
- **BREAKING（配置格式）**：将 `global_config` 中 `aws_gpu_instance_types` 由「GPU 实例类型列表（JSON 数组）」改造为「实例类型 → 卡型 的映射（JSON 对象）」，使其同时具备「判断是否 GPU」与「获取卡型」两种能力；改造需保证不影响前序 `ResClassId` 的 GPU 判定逻辑。需在上线时同步刷新该配置数据。
- AWS 上报时按实例类型（`ProductInstanceType`）查映射填充 `GpuCardCategory`。
- GCP 上报时按「卡型识别规则」对 `SkuDescription` 做两层匹配填充 `GpuCardCategory`：L1 显式卡型关键词（`H200`/`H100`/`A100`/`L4`/`RTX 6000`/`TPU7x`/`V100`/`P100`/`P4`/`K80`，硬编码在 enumor 层、带词边界）优先，未命中再走 L2 实例族前缀（`A3Ultra`/`A3 Ultra`/`A3`/`A2`/`G2`/`G4`，由 `global_config` 新增 `gcp_gpu_instance_prefixes` 配置承载，词边界 + 最长前缀优先消解 A3/A3Ultra 歧义）。卡型取短名（`H200`/`H100`/`A100`/`L4`/`RTX6000PRO`/`TPU7x`/`V100`/`P100`/`P4`/`K80`，与 AWS 风格对齐），本期不识别 T4、不处理 L3（Local Storage 等需结合同账单实例族间接判断的场景）。华为本期 `GpuCardCategory` 仍统一留空（后续补充映射规则）。
- **BREAKING（GCP GPU 判定）**：将 `isGcpGPU` 的判定标准由「`SkuDescription` 含 `calendar mode`」改为「卡型识别命中（L1∪L2）OR `HcProductName` 含 `_HCM_AI_` 前缀 OR `SkuDescription` 命中 AI 关键词（兜底保留）」，并删除不再使用的 `constant.GcpCalendarMode` 常量。改造后 GCP 的 `ResClassId`（GPU/CPU 分类）判定更精准，但结果与改造前不同。
- AWS、GCP 上报时，复用 AI 关键词匹配思路识别 `APIBrandName`，并将 `veo`/`imagen`/`lyria` 归并为 `gemini`，最终取值范围为 `gemini`/`claude`/`kimi`/`jina`；一条文本命中多个关键词时取位置最靠前的命中词。华为本期 `APIBrandName` 统一留空（华为暂无 AI 账单识别链路，待确认后续补充）。
- 非 GPU 账单项 `GpuCardCategory` 留空、非 AI 账单项 `APIBrandName` 留空。

## Capabilities

### New Capabilities

- `obs-bill-gpu-card-category`: OBS 账单 GPU 卡型字段——三张表新增 `GpuCardCategory` 字段、`aws_gpu_instance_types` 配置改造为实例类型→卡型映射、AWS 上报时按映射填充卡型、GCP 按 L1 显式关键词 + L2 实例族前缀（`gcp_gpu_instance_prefixes` 配置）两层规则识别填充卡型并据此改造 `isGcpGPU` 判定、华为留空。
- `obs-bill-api-brand-name`: OBS 账单 API 厂商字段——三张表新增 `APIBrandName` 字段、AWS/GCP 复用 AI 关键词匹配识别 API 厂商并按 `veo`/`imagen`/`lyria` → `gemini` 归并、多命中取首个，华为本期留空。

### Modified Capabilities

（前序变更 `obs-bills-add-city-resclass` 尚未归档，其 `obs-bill-res-class-id` 等 spec 未进入主 specs，故本期相关行为以新建能力承载，无主 spec 层级的需求修改。）

## Impact

- **数据库（OBS DB）**：三张 obs 账单表各新增 2 个 varchar 字段（DEFAULT ''）。
- **数据库（HCM DB）**：`global_config` 中 `aws_gpu_instance_types` 记录的 `config_value` 格式由数组改为对象，需运营刷新数据；新增一条 `config_key=gcp_gpu_instance_prefixes`（实例族前缀→短卡型名 JSON 对象），需运营录入。
- **pkg/criteria/enumor/global_config.go**：新增 `GlobalConfigKeyGcpGpuInstancePrefixes` 配置 key。
- **pkg/criteria/enumor/bill.go**：新增「返回命中 API 厂商」的匹配函数（含 `veo`/`imagen`/`lyria` → `gemini` 归并）；新增 GCP L1 显式卡型关键词识别函数（有序正则 + 词边界，返回短卡型名）。
- **pkg/criteria/constant/bill.go**：删除不再使用的 `GcpCalendarMode` 常量。
- **pkg/dal/table/obs**：三张 OBS 账单 table 结构体及 ColumnDescriptor 新增两字段。
- **cmd/task-server/logics/action/obs/sync**：`gpu_lookup.go` 的 `loadAwsGpuInstanceTypes` 返回类型由 `map[string]struct{}` 改为 `map[string]string` 并新增取卡型逻辑、`isAwsGPU` 适配；新增 `loadGcpGpuInstancePrefixes`、`lookupGcpGpuCardCategory`（L1+L2）并改造 `isGcpGPU`（去 calendar、加卡型识别）；`sync_aws.go`/`sync_gcp.go`/`sync_huawei.go` 的 convert 函数填充两字段（GCP 批次开始加载前缀配置）。
