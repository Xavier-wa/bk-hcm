## ADDED Requirements

### Requirement: OBS 账单表新增 GpuCardCategory 字段

系统 SHALL 在 `obs_aws_bills`、`obs_gcp_bills`、`obs_huawei_bills` 三张 OBS 账单表中新增 `GpuCardCategory`（GPU 卡型，字符串）字段，并在每次 OBS 上报时随账单一并写入。该字段为字符串类型，允许为空，默认值为空字符串，不影响存量数据。

#### Scenario: 三张表均具备 GpuCardCategory 字段

- **WHEN** 执行数据库迁移后
- **THEN** `obs_aws_bills`、`obs_gcp_bills`、`obs_huawei_bills` 三张表均存在 `GpuCardCategory` 字段，类型为字符串，默认值为空字符串

#### Scenario: 上报时字段随账单写入

- **WHEN** OBS 同步任务处理任一厂商账单项并上报
- **THEN** 上报到 OBS 的账单数据携带 `GpuCardCategory` 字段

### Requirement: aws_gpu_instance_types 配置改造为实例类型到卡型映射

系统 SHALL 将 `global_config` 中 `aws_gpu_instance_types` 配置的 `config_value` 由「GPU 实例类型列表（JSON 数组）」改造为「实例类型 → 卡型 的映射（JSON 对象）」，使其同时具备「判断实例类型是否为 GPU」与「获取实例类型对应卡型」两种能力。改造后 AWS GPU 资源的 `ResClassId` 判定结果 MUST 与改造前保持一致。

#### Scenario: 配置为合法 JSON 对象

- **WHEN** 从 `global_config` 读取 `aws_gpu_instance_types` 配置
- **THEN** 其 `config_value` 为合法 JSON 对象，键为实例类型、值为卡型字符串

#### Scenario: GPU 判定逻辑兼容

- **WHEN** 某 AWS 账单项的实例类型存在于映射键中
- **THEN** 该账单项被判定为 GPU 资源，`ResClassId` 取 AWS GPU 分类 ID（与改造前一致）

#### Scenario: 配置缺失时降级

- **WHEN** `global_config` 中不存在 `aws_gpu_instance_types` 配置
- **THEN** 视为空映射，所有账单项 `GpuCardCategory` 留空且不阻断上报流程

### Requirement: AWS 账单按实例类型填充 GPU 卡型

系统 SHALL 在 AWS OBS 上报时，取账单项的实例类型（`ProductInstanceType`）在映射表中查找：命中则将 `GpuCardCategory` 填为对应卡型；未命中（含 SageMaker、`_HCM_AI_` 前缀等"判定为 GPU 但无具体实例类型映射"的情况）则 `GpuCardCategory` 留空。`GpuCardCategory` MUST 仅存卡型字符串，不含卡数。

#### Scenario: 命中实例类型映射

- **WHEN** 一条 AWS GPU 账单项实例类型为 `g5.12xlarge`，且映射表中 `g5.12xlarge` → `A10G`
- **THEN** 该账单的 `GpuCardCategory` 为 `A10G`

#### Scenario: 命中 L4 卡型

- **WHEN** 一条 AWS GPU 账单项实例类型为 `g6.xlarge`，且映射表中 `g6.xlarge` → `L4`
- **THEN** 该账单的 `GpuCardCategory` 为 `L4`

#### Scenario: 判定为 GPU 但无实例类型映射

- **WHEN** 一条 AWS 账单项被判定为 GPU（如 SageMaker 或 `_HCM_AI_` 前缀）但实例类型未命中映射
- **THEN** 该账单的 `GpuCardCategory` 为空字符串

#### Scenario: 非 GPU 账单项留空

- **WHEN** 一条非 GPU 的 AWS 账单项上报
- **THEN** 该账单的 `GpuCardCategory` 为空字符串

### Requirement: GCP 按两层规则识别填充 GpuCardCategory

系统 SHALL 在 GCP OBS 上报时，对账单项的 `SkuDescription` 按「L1 显式卡型关键词」与「L2 实例族前缀」两层规则识别 GPU 卡型并填充 `GpuCardCategory`：先匹配 L1，未命中再匹配 L2，均未命中则留空。卡型取值 MUST 为短名（`H200`/`H100`/`A100`/`L4`/`RTX6000PRO`/`TPU7x`/`V100`/`P100`/`P4`/`K80`），仅存卡型字符串、不含卡数。本期 MUST NOT 识别 T4，MUST NOT 处理 L3（`SSD backed Local Storage` 等需结合同账单实例族间接判断的场景）。

L1 显式卡型关键词规则 SHALL 硬编码（与 AI 关键词识别同源），采用忽略大小写 + 词边界匹配，覆盖：`H200`→`H200`、`H100`→`H100`、`A100`/`Tesla A100`→`A100`、`L4`→`L4`、`RTX 6000`/`RTX Pro 6000`→`RTX6000PRO`、`TPU7x`→`TPU7x`、`V100`→`V100`、`P100`→`P100`、`P4`→`P4`、`K80`→`K80`。

L2 实例族前缀规则 SHALL 由 `global_config`（`config_type=account_bill`、`config_key=gcp_gpu_instance_prefixes`）承载，`config_value` 为「实例族前缀 → 短卡型名」JSON 对象（如 `G4` → `RTX6000PRO`）；匹配采用忽略大小写 + 词边界，命中多个前缀时取前缀字符串最长者对应的卡型。

#### Scenario: L1 显式卡型关键词命中

- **WHEN** 一条 GCP 账单项 `SkuDescription` 为 `Nvidia L4 GPU running in Frankfurt`
- **THEN** 该账单的 `GpuCardCategory` 为 `L4`

#### Scenario: RTX 6000 系列映射为 RTX6000PRO

- **WHEN** 一条 GCP 账单项 `SkuDescription` 为 `NVIDIA RTX Pro 6000 GPU with no zonal redundancy in us-central1` 或 `RTX 6000 96GB running in Delhi`
- **THEN** 该账单的 `GpuCardCategory` 为 `RTX6000PRO`

#### Scenario: L1 识别历史卡型

- **WHEN** 一条 GCP 账单项 `SkuDescription` 含 `V100`/`P100`/`P4`/`K80` 之一
- **THEN** 该账单的 `GpuCardCategory` 为对应短名（`V100`/`P100`/`P4`/`K80`）

#### Scenario: L2 实例族前缀命中

- **WHEN** 一条 GCP 账单项 `SkuDescription` 为 `A2 Instance Core running in Americas`，且 `gcp_gpu_instance_prefixes` 中 `A2` → `A100`
- **THEN** 该账单的 `GpuCardCategory` 为 `A100`

#### Scenario: A3Ultra 与 A3 歧义按最长前缀消解

- **WHEN** 一条 GCP 账单项 `SkuDescription` 为 `DWS Defined Duration A3 Ultra Instance Core`，且配置中同时存在 `A3` → `H100` 与 `A3 Ultra` → `H200`
- **THEN** 该账单的 `GpuCardCategory` 为 `H200`（取最长命中前缀 `A3 Ultra`）

#### Scenario: 本期不识别 T4

- **WHEN** 一条 GCP 账单项 `SkuDescription` 含 `T4`（如 N1 手动挂接 `nvidia-tesla-t4`）
- **THEN** 该账单的 `GpuCardCategory` 为空字符串

#### Scenario: L2 配置缺失时仅 L1 生效

- **WHEN** `global_config` 中不存在 `gcp_gpu_instance_prefixes` 配置，且某 GCP 账单项 `SkuDescription` 仅含实例族前缀（无 L1 显式卡型关键词）
- **THEN** 该账单的 `GpuCardCategory` 为空字符串，且不阻断上报流程

#### Scenario: 未命中任何规则留空

- **WHEN** 一条 GCP 账单项 `SkuDescription` 不含任何 L1 关键词或 L2 前缀
- **THEN** 该账单的 `GpuCardCategory` 为空字符串

### Requirement: GCP isGcpGPU 判定改为卡型识别与 AI 识别

系统 SHALL 将 GCP 账单项的 GPU 判定（`isGcpGPU`）改为满足以下任一条件即判定为 GPU：(a) 卡型识别（L1∪L2）命中，(b) `HcProductName` 含 `_HCM_AI_` 前缀，(c) `SkuDescription` 命中 AI 关键词（兜底）。系统 MUST 移除原「`SkuDescription` 含 `calendar mode`」判定标准，并删除不再使用的 `GcpCalendarMode` 常量。`isGcpGPU` 的结果用于计算 `ResClassId`。

#### Scenario: 卡型命中判定为 GPU

- **WHEN** 一条 GCP 账单项卡型识别命中（如 `SkuDescription` 含 `A3Ultra`）
- **THEN** `isGcpGPU` 为 true，`ResClassId` 取 GCP GPU 分类 ID

#### Scenario: AI 前缀命中判定为 GPU

- **WHEN** 一条 GCP 账单项 `HcProductName` 含 `_HCM_AI_` 前缀
- **THEN** `isGcpGPU` 为 true

#### Scenario: 不再以 calendar mode 判定

- **WHEN** 一条 GCP 账单项 `SkuDescription` 含 `calendar mode`，但卡型识别未命中且非 AI
- **THEN** `isGcpGPU` 为 false，`ResClassId` 取 GCP CPU 分类 ID

### Requirement: 华为本期 GpuCardCategory 留空

系统 SHALL 在本期对华为账单项的 `GpuCardCategory` 统一留空（字段保留，后续补充卡型映射规则）。

#### Scenario: 华为账单卡型留空

- **WHEN** 任意华为账单项上报
- **THEN** 该账单的 `GpuCardCategory` 为空字符串
