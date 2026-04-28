# Spec: obs-bill-res-class-id

## Purpose

OBS 账单资源类型分类 ID 字段：在三张 OBS 账单表中新增 `ResClassId` 字段，通过判断账单项是 GPU 还是 CPU 资源，映射为各云厂商对应的枚举值后上报。GPU 判断规则按厂商分别配置，GPU 机型配置存入 `global_config`（config_type="account_bill"）可运营管理，同时扩充 AI 账单关键词以提升 GPU 识别覆盖率。

## ADDED Requirements

### Requirement: res_class_id 枚举映射
系统 SHALL 为 ResClassID 定义枚举值，各厂商 CPU/GPU 对应值如下：AWS CPU=451，AWS GPU=6311；GCP CPU=601，GCP GPU=6312；HuaWei CPU=1244，HuaWei GPU=6315。OBS 上报时按 vendor + isGPU 查表填充 ResClassId 字段。

#### Scenario: AWS CPU 账单 ResClassId
- **GIVEN** 一条 AWS 账单不满足任何 GPU 判断条件
- **WHEN** 执行 OBS 上报
- **THEN** obs_aws_bills 中该记录的 ResClassId=451

#### Scenario: AWS GPU 账单 ResClassId
- **GIVEN** 一条 AWS 账单满足 GPU 判断条件
- **WHEN** 执行 OBS 上报
- **THEN** obs_aws_bills 中该记录的 ResClassId=6311

#### Scenario: GCP CPU/GPU ResClassId
- **GIVEN** 一条 GCP 账单，isGPU 判断结果为 false
- **WHEN** 执行 OBS 上报
- **THEN** ResClassId=601；若 isGPU 为 true，则 ResClassId=6312

#### Scenario: HuaWei CPU/GPU ResClassId
- **GIVEN** 一条 HuaWei 账单，isGPU 判断结果为 false
- **WHEN** 执行 OBS 上报
- **THEN** ResClassId=1244；若 isGPU 为 true，则 ResClassId=6315

---

### Requirement: AWS GPU 账单识别
AWS 账单 SHALL 按以下优先顺序判断是否为 GPU：
1. `line_item_product_code` 等于 "AmazonSageMaker" → GPU
2. `product_instance_type` 在 global_config 的 AWS GPU 机型列表（config_type="account_bill"，config_key="aws_gpu_instance_types"）中 → GPU
3. `hc_product_name` 以 `_HCM_AI_` 前缀开头（已在 dailysplit 阶段标记）→ GPU
4. 以上均不满足 → CPU

#### Scenario: SageMaker 账单为 GPU
- **GIVEN** AWS 账单的 line_item_product_code="AmazonSageMaker"
- **WHEN** 执行 OBS 上报的 isGPU 判断
- **THEN** 判定为 GPU

#### Scenario: GPU 机型列表匹配
- **GIVEN** global_config 中 aws_gpu_instance_types=["p3.2xlarge","p4d.24xlarge"]，账单的 product_instance_type="p3.2xlarge"
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 GPU

#### Scenario: AI 前缀标记为 GPU
- **GIVEN** 账单 hc_product_name 以 "_HCM_AI_" 开头
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 GPU

#### Scenario: 普通 EC2 账单为 CPU
- **GIVEN** AWS 账单 line_item_product_code="AmazonEC2"，product_instance_type="t3.micro"，hc_product_name 无 AI 前缀
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 CPU

#### Scenario: GPU 机型配置不存在时默认 CPU
- **GIVEN** global_config 中不存在 aws_gpu_instance_types 配置记录（Details 为空）
- **WHEN** 执行 isGPU 判断，且其他条件均不满足
- **THEN** 判定为 CPU（静默使用空机型集合，不记录 Warn 日志）

---

### Requirement: GCP GPU 账单识别
GCP 账单 SHALL 按以下规则判断是否为 GPU：
1. `SkuDescription`（不区分大小写）包含 "calendar mode" → GPU
2. `hc_product_name` 以 `_HCM_AI_` 前缀开头 → GPU
3. 以上均不满足 → CPU

#### Scenario: SKU 描述含 calendar mode 为 GPU
- **GIVEN** GCP 账单的 SkuDescription="Preemptible NVIDIA T4 GPU Calendar Mode"（含 "calendar mode"，大小写不限）
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 GPU

#### Scenario: SKU 描述大小写不敏感
- **GIVEN** GCP 账单的 SkuDescription="NVIDIA A100 CALENDAR MODE"
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 GPU

#### Scenario: GCP AI 前缀账单为 GPU
- **GIVEN** GCP 账单 hc_product_name 以 "_HCM_AI_" 开头
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 GPU

#### Scenario: 普通 GCP 账单为 CPU
- **GIVEN** GCP 账单 SkuDescription 不含 "calendar mode"，hc_product_name 无 AI 前缀
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 CPU

---

### Requirement: HuaWei GPU 账单识别
HuaWei 账单 SHALL 通过 `product_spec_desc` 字段提取规格前缀来判断是否为 GPU：取 product_spec_desc 按 "|" 分割后的第一段，再取**第一个点（`.`）之前**的部分作为机型前缀（大小写不敏感匹配），若该前缀在 global_config 的华为 GPU 前缀列表（config_type="account_bill"，config_key="huawei_gpu_instance_prefixes"）中，则为 GPU；否则为 CPU。`product_spec_desc` 为 nil 或空时直接判定为 CPU。若 product_spec_desc 不含点号，则整个第一段作为前缀。

#### Scenario: 规格前缀匹配 GPU 列表
- **GIVEN** global_config 中 huawei_gpu_instance_prefixes=["p1","p2v","g6"]，账单的 product_spec_desc="p2v.2xlarge.4|GPU实例P2v"
- **WHEN** 执行 isGPU 判断
- **THEN** 取第一段 "p2v.2xlarge.4"，取第一个点之前的部分得到前缀 "p2v"，匹配 GPU 列表，判定为 GPU

#### Scenario: 带竖线分隔符的规格描述
- **GIVEN** product_spec_desc="g6v|高性能GPU实例G6v系列"
- **WHEN** 执行 isGPU 判断
- **THEN** 取第一段 "g6v"，无点号则整段作为前缀 "g6v"，判定为 GPU

#### Scenario: 普通规格为 CPU
- **GIVEN** product_spec_desc="s6.2xlarge.4|通用计算"，GPU 前缀列表中无 "s6"
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 CPU

#### Scenario: product_spec_desc 为空时为 CPU
- **GIVEN** HuaWei 账单的 product_spec_desc 为 nil 或空字符串
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 CPU，不报错

#### Scenario: GPU 前缀配置不存在时默认 CPU
- **GIVEN** global_config 中不存在 huawei_gpu_instance_prefixes 配置记录（Details 为空）
- **WHEN** 执行 isGPU 判断
- **THEN** 判定为 CPU（静默使用空前缀列表，不记录 Warn 日志）

---

### Requirement: AI 账单关键词扩充
`IsAIBillItem` 函数 SHALL 在现有关键词（gemini、claude）基础上新增以下关键词：kimi、jina、veo、imagen、lyria（均不区分大小写匹配），对所有厂商统一生效。新关键词在 dailysplit 阶段触发，符合任一关键词的 hc_product_name 将被打上 `_HCM_AI_` 前缀，后续 OBS 上报时据此判断为 GPU。

#### Scenario: kimi 关键词匹配
- **GIVEN** hc_product_name 中包含 "Kimi"（大写开头）
- **WHEN** 调用 IsAIBillItem
- **THEN** 返回 true

#### Scenario: jina 关键词匹配
- **GIVEN** hc_product_name="jina-embedding-v3"
- **WHEN** 调用 IsAIBillItem
- **THEN** 返回 true

#### Scenario: Veo 关键词匹配（GCP）
- **GIVEN** SkuDescription="Veo 2 Video Generation"
- **WHEN** 调用 IsAIBillItem
- **THEN** 返回 true

#### Scenario: Imagen 关键词匹配（GCP）
- **GIVEN** SkuDescription="Imagen 3 Fast Generate"
- **WHEN** 调用 IsAIBillItem
- **THEN** 返回 true

#### Scenario: Lyria 关键词匹配（GCP）
- **GIVEN** SkuDescription="Lyria Music Generation API"
- **WHEN** 调用 IsAIBillItem
- **THEN** 返回 true

#### Scenario: 不含关键词时返回 false
- **GIVEN** hc_product_name="AmazonEC2 t3.micro"
- **WHEN** 调用 IsAIBillItem
- **THEN** 返回 false
