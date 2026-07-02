## Context

OBS 消费 HCM 上报的账单数据进行成本核算。前序变更 `obs-bills-add-city-resclass` 已建立 OBS 上报链路的字段填充模式：`account_bill_item（分账后）→ obs/sync/sync_{vendor}.go convert 函数 → obs_{vendor}_bills`，并已落地 `CityId`、`ResClassId` 两字段、`global_config` 配置加载（`gpu_lookup.go`）以及 `IsAIBillItem` 关键词识别。

本期在该基础上增量新增 `GpuCardCategory`、`APIBrandName` 两字段，复用既有 convert 流程与 global_config 加载机制。关键约束：6 月账单 7 月 2 日出账前必须上线并覆盖 6 月同步；改造 `aws_gpu_instance_types` 配置不得破坏前序 `ResClassId` 的 GPU 判定。

## Goals / Non-Goals

**Goals:**
- 三张 OBS 账单表新增 `GpuCardCategory`、`APIBrandName` 字段并随上报填充。
- AWS 按实例类型映射填充 GPU 卡型；GCP 按 L1 显式卡型关键词 + L2 实例族前缀两层规则识别卡型并据此改造 `isGcpGPU` 判定；三厂商按关键词识别 API 厂商并归并。
- 配置改造同时支持「是否 GPU」与「卡型获取」，对 AWS `ResClassId` 判定保持兼容。

**Non-Goals:**
- 不填充华为的 GPU 卡型（字段保留留空，规则后续提供）。
- GCP 不识别 T4（N1 手动挂接的 `nvidia-tesla-t4`），不处理 L3（`SSD backed Local Storage` 等需结合同账单实例族间接判断的场景）。
- 不存储/上报 GPU 卡数（card count）。
- 不修改 `account_bill_item` 分账表结构，不改 OBS 平台侧。
- 不回填历史已上报数据（存量两字段为空字符串）。

## Decisions

### 决策 1：复用并改造 `aws_gpu_instance_types`，由数组改为对象

**选择**：将 `global_config` 中 `aws_gpu_instance_types` 的 `config_value` 由 JSON 数组（`["g5.xlarge", ...]`）改为 JSON 对象（`{"g5.xlarge": "A10G", ...}`）。`loadAwsGpuInstanceTypes` 返回类型由 `map[string]struct{}` 改为 `map[string]string`（实例类型→卡型）。`isAwsGPU` 改判 `if _, ok := awsGpuMap[productInstanceType]; ok`；卡型则直接取 map value。

**备选**：新增独立 config_key（如 `aws_gpu_instance_card_types`）专存卡型映射，保留原数组判 GPU。

**理由**：单一配置同时承载「判 GPU + 取卡型」，避免两份配置数据不一致；运营只维护一处。改造后 GPU 判定语义不变（键集合 == 原数组元素），对 `ResClassId` 完全兼容。代价是需在上线时同步刷新该配置数据（见迁移计划）。

### 决策 2：新增「返回命中品牌」的匹配函数，复用现有正则思路

**选择**：在 `pkg/criteria/enumor/bill.go` 新增一个返回命中品牌字符串的函数（如 `MatchAPIBrandName(str string) string`）。沿用 `aiBillItemRegexp` 的「忽略大小写 + 词边界」匹配规则，但用 `FindStringSubmatch` 取代 `MatchString` 以捕获命中的具体关键词（关键词位于第 2 个捕获组），再做归并：`veo`/`imagen`/`lyria` → `gemini`，其余取原值；未命中返回空字符串。正则只返回字符串中位置最靠前的一处命中，故一条文本命中多个品牌词时取首个（见决策 3）。

**备选**：在调用方逐个 `strings.Contains` 匹配关键词。

**理由**：集中在 enumor 层与现有 `IsAIBillItem` 同源，规则一致、便于维护；词边界匹配避免子串误判（如 `claudexx`）。`IsAIBillItem` 保持不变，新增函数独立承载品牌识别。

### 决策 3：API 厂商识别仅对 AWS/GCP 生效，字段与现有 AI 识别同源；多命中取首个

**选择**：
- AWS：`item.HcProductName`（与 `dailysplit/aws.go` 打 `_HCM_AI_` 前缀、`isAwsGPU` 中 AI 前缀判断同源；品牌关键词嵌于前缀之后，正则的非字母词边界仍能命中）。AWS 打前缀的条件即 `IsAIBillItem(HcProductName)`，故凡被判为 AI 的 AWS 条目，`HcProductName` 必含品牌词，品牌匹配必然非空。
- GCP：**优先匹配 `HcProductName`，命中为空则兜底匹配 `SkuDescription`**，与 `isGcpGPU` 的双路径判定保持一致——`isGcpGPU` 先看 `HcProductName` 的 `_HCM_AI_` 前缀（前缀基于 `IsAIBillItem(HcProductName)` 打），再兜底 `IsAIBillItem(SkuDescription)`（捞 Credit 等 `HcProductName` 不含前缀的条目）。品牌匹配采用同序回退，避免出现「被判为 AI/GPU 但品牌为空」的口径不一致。
- 华为：本期不识别，`APIBrandName` 统一留空（华为无 AI 识别链路，详见决策 6）。
- 一条文本命中多个品牌关键词时，取正则匹配到的首个（位置最靠前）命中词。

**理由**：复用各厂商已验证的 AI 文本来源，减少新增字段透传成本与误判面；GCP 品牌匹配的字段顺序与 `isGcpGPU` 对称，保证「是 AI/GPU ⇔ 有品牌」自洽。业务上单条账单同时含多品牌词属罕见，取首个实现最简单且行为可预期。

### 决策 4：卡型仅取卡型字符串，不含卡数

**选择**：即便运营配置或原始映射中存在卡数信息，`GpuCardCategory` 仅存卡型（如 `A10G`）。

**理由**：需求明确本期不需要卡数；保持字段语义单一，便于 OBS 侧按卡型聚合。

### 决策 5：字段填充点沿用 convert 函数，批量初始化配置

**选择**：在各 `doSyncXxxBillItem` 批次开始时一次性加载配置（`loadAwsGpuInstanceTypes` 已是批量加载），convert 函数内逐条填充两字段；不在逐条账单时查 DB。

**理由**：与前序 `CityId`/`ResClassId` 填充模式完全一致，单批最大 500 条，避免逐条查询。

### 决策 6：华为本期 API 厂商留空

**选择**：华为账单项 `APIBrandName` 本期统一留空，与 `GpuCardCategory` 的华为处理对称。

**理由**：华为当前完全没有 AI 账单识别链路——`dailysplit` 不对华为调 `IsAIBillItem`、不打 `_HCM_AI_` 前缀，`isHuaweiGPU` 仅按机型前缀判断。强行加品牌识别属全新行为，且在确认华为是否存在 AI 账单及其文本字段前无法验证。保留字段、留空，待需求方确认后再补充。

### 决策 7：GCP 卡型本期改为按规则识别填充（推翻原「GCP 留空」）

**选择**：GCP `GpuCardCategory` 由「本期留空」改为按卡型识别规则（见决策 8/9）对 `SkuDescription` 匹配填充；卡型取**短名**（`H200`/`H100`/`A100`/`L4`/`RTX6000PRO`/`TPU7x`/`V100`/`P100`/`P4`/`K80`），与 AWS（`T4`/`A10G`/`L4`/`L40S`）风格对齐，便于 OBS 侧跨厂商按卡型聚合。

**备选**：沿用原决策本期 GCP 留空；或卡型用全名（`NVIDIA H200 141GB`）。

**理由**：需求方已提供 GCP 卡型识别规则（`docs/reqs/GCP_GPU_SKU_提取结果.md` 第十部分），且要求基于卡型识别改造 `isGcpGPU`，故必须先实现卡型识别。短名与 AWS 统一口径，避免 OBS 侧维护两套卡型命名。

### 决策 8：卡型识别两层规则——L1 硬编码、L2 走 global_config

**选择**：仅取文档第十部分的前两层（去掉 T4、不处理 L3）：
- **L1 显式卡型关键词**（最高优先级）：`H200`→`H200`、`H100`→`H100`、`A100`(含 `Tesla A100`)→`A100`、`L4`→`L4`、`RTX (Pro )?6000`→`RTX6000PRO`、`TPU7x`→`TPU7x`、`V100`→`V100`、`P100`→`P100`、`P4`→`P4`、`K80`→`K80`。硬编码在 `pkg/criteria/enumor`（与 `IsAIBillItem` 的正则同源），统一用**词边界**匹配（如 `(^|[^a-z0-9])kw($|[^a-z0-9])`），避免 `L4`、`A100`、`P4` 等子串误命中（如 `A1000`）。按有序列表逐项匹配，命中即返回。
- **L2 实例族前缀**：`A3Ultra`/`A3 Ultra`/`A3`/`A2`/`G2`/`G4` → 短卡型名（`G4`→`RTX6000PRO`）。由 `global_config` 新增 `config_key=gcp_gpu_instance_prefixes`（`config_type=account_bill`）承载，`config_value` 为「实例族前缀 → 短卡型名」JSON 对象，与 `aws_gpu_instance_types` 对象风格一致，便于运营在不发版的前提下扩展实例族。

匹配顺序：**先 L1 后 L2**，L1 命中即定型，未命中再走 L2。

**备选**：L1+L2 全部硬编码；或 L1+L2 全部走 global_config（含词边界正则）。

**理由**：L1 含词边界/正则语义（`L4` 词边界、`RTX (Pro )?6000`），纯配置表达成本高且易错，硬编码在 enumor 层与既有 AI 正则一致、可单测覆盖；L2 实例族是相对规整的 token 且会随 GCP 机型演进，配置化（`global_config`）满足需求点「实例族映射前缀等规则通过 global_config 配置」，运营可独立维护。

### 决策 9：L2 前缀匹配用「词边界 + 最长前缀优先」消解 A3/A3Ultra 歧义

**选择**：`gcp_gpu_instance_prefixes` 用无序 JSON 对象（不引入有序数组），代码匹配时对每个前缀做词边界匹配，命中多个时取**前缀字符串最长**者的卡型。

```
"DWS ... A3Ultra Core"   → 命中 A3Ultra → H200（A3 因词边界不命中连写 a3ultra）
"A3 Ultra Instance Ram"  → 命中 A3、A3 Ultra，取最长 A3 Ultra → H200
"A3 High Instance Core"  → 仅命中 A3 → H100
```

**备选**：配置改为有序数组，按数组顺序匹配（A3Ultra/A3 Ultra 排在 A3 前）。

**理由**：`A3`（H100，非 Ultra）与 `A3Ultra`/`A3 Ultra`（H200）的区分本质是「更具体前缀优先」，最长前缀优先与之等价，且保持配置为无序对象（与 AWS 配置风格统一、运营无需关心顺序）。配置缺失/解析失败时 L2 整体不命中（仅 L1 生效），不阻断上报，与既有降级策略一致。

### 决策 10：改造 isGcpGPU——去除 calendar mode，改为卡型识别 + AI

**选择**：`isGcpGPU` 判定链改为：① `lookupGcpGpuCardCategory(SkuDescription)` 非空（L1∪L2 命中）→ GPU；② `HcProductName` 含 `_HCM_AI_` 前缀 → GPU；③ `IsAIBillItem(SkuDescription)` 兜底 → GPU。删除原「`SkuDescription` 含 `calendar mode`」判定，并清理仅此处引用的 `constant.GcpCalendarMode` 常量。

**理由**：`calendar mode` 仅是计费模式的粗粒度代理（见文档 10.5，影响金额不影响卡型），过宽。改用精确的卡型识别 + AI 兜底，判定更准。GPU 的 calendar 预留 SKU 描述通常含 `A3Ultra`/`G4` 等族名，会被 L2 覆盖，删除 calendar 判定的漏判风险低。

## Risks / Trade-offs

- **[Risk] 配置格式切换期间新旧不兼容** → 上线顺序：先刷新 `aws_gpu_instance_types` 为对象格式，再部署改造后的 task-server；`loadAwsGpuInstanceTypes` 解析失败时记录 Warnf 并返回错误，避免按错误格式静默跑空。解析失败采用「硬失败」（中断整批同步），而非软降级——账单同步异常后有定时任务重新触发，部署窗口短暂的格式不匹配只会让该批次重试，不会导致上报最终挂掉，故无需为部署窗口竞态引入兼容双格式的过渡逻辑。
- **[Risk] 配置缺失导致全部卡型留空** → 视为空映射 + Warnf 日志，不阻断上报（卡型留空、GPU 判定退化为 false，与前序一致行为）。
- **[Risk] 华为是否存在 AI 账单未确定** → 本期华为 `APIBrandName` 一律留空（决策 6），不引入识别逻辑；待需求方确认华为存在 AI 账单及其文本字段后再补充，不影响 AWS/GCP。
- **[Risk] 存量数据两字段为空** → OBS 侧需知悉空字符串表示历史无数据。
- **[Risk] GCP 去除 calendar mode 判定的漏判** → 描述里无任何 L1/L2 token 且非 AI 的「calendar mode」SKU 不再判为 GPU。实际 GPU 的 calendar 预留 SKU 描述含 `A3Ultra`/`G4` 等族名，被 L2 覆盖，漏判风险低（决策 10）。
- **[Risk] `gcp_gpu_instance_prefixes` 配置缺失** → L2 整体不命中，仅 L1 生效，卡型可能留空、`isGcpGPU` 退化为仅 AI 判定，不阻断上报；需运营在上线前录入该配置。
- **[Trade-off] 改造共享配置** → 需运营在上线窗口刷新数据，增加一步部署协同，但换取配置单一来源。

## Migration Plan

1. 执行 SQL 迁移：三张 OBS 账单表 `ALTER ADD COLUMN GpuCardCategory`、`APIBrandName`（varchar，DEFAULT ''，不影响存量读写）。
2. 在 `global_config` 中将 `aws_gpu_instance_types` 的 `config_value` 刷新为「实例类型→卡型」JSON 对象。
3. 在 `global_config` 中新增 `config_type=account_bill`、`config_key=gcp_gpu_instance_prefixes` 记录，`config_value` 为「实例族前缀→短卡型名」JSON 对象（如 `{"A3Ultra":"H200","A3 Ultra":"H200","A3":"H100","A2":"A100","G2":"L4","G4":"RTX PRO 6000"}`）。
4. 部署 task-server（含改造后的 `gpu_lookup.go` 与三厂商 convert 逻辑）。
5. 触发/等待 6 月账单同步，验证两字段按规则填充。

**回滚**：task-server 回滚不影响已写入数据；若需回滚配置格式，需同时回滚 task-server 与 `aws_gpu_instance_types` 配置（两者格式强耦合）；`gcp_gpu_instance_prefixes` 缺失仅导致 L2 不命中，不阻断上报，可不回滚。SQL 字段可 `ALTER DROP`（需评估 OBS 侧是否已依赖新字段）。

## Open Questions

- **Q-001（已决议）**：华为账单 API 厂商识别本期不做，`APIBrandName` 统一留空（决策 6）。华为当前未接入 AI 账单识别链路；后续若需支持，需先确认华为是否存在 AI 账单及其品牌关键词所在文本字段（`product_spec_desc` 或 `hc_product_name`）。
