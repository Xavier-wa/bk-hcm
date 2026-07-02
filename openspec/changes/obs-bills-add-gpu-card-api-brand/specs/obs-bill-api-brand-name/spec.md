## ADDED Requirements

### Requirement: OBS 账单表新增 APIBrandName 字段

系统 SHALL 在 `obs_aws_bills`、`obs_gcp_bills`、`obs_huawei_bills` 三张 OBS 账单表中新增 `APIBrandName`（API 厂商，字符串）字段，并在每次 OBS 上报时随账单一并写入。该字段为字符串类型，允许为空，默认值为空字符串，不影响存量数据。

#### Scenario: 三张表均具备 APIBrandName 字段

- **WHEN** 执行数据库迁移后
- **THEN** `obs_aws_bills`、`obs_gcp_bills`、`obs_huawei_bills` 三张表均存在 `APIBrandName` 字段，类型为字符串，默认值为空字符串

#### Scenario: 上报时字段随账单写入

- **WHEN** OBS 同步任务处理任一厂商账单项并上报
- **THEN** 上报到 OBS 的账单数据携带 `APIBrandName` 字段

### Requirement: 按 AI 关键词识别 API 厂商

系统 SHALL 复用 AI 账单关键词匹配思路（忽略大小写、带词边界），在账单明细文本字段上匹配 `BillItemAIFlag` 关键词以识别 API 厂商，并返回命中的品牌。识别对 AWS、GCP 两厂商生效。各厂商使用的文本字段为：AWS 取 `hc_product_name`；GCP 优先取 `hc_product_name`，命中为空时兜底取 `sku_description`（与 `isGcpGPU` 的双路径判定保持一致，避免「被判为 AI/GPU 但品牌为空」的口径不一致）。

#### Scenario: 命中单一品牌关键词

- **WHEN** 账单明细文本含 `claude`、`kimi` 或 `jina`
- **THEN** `APIBrandName` 分别为 `claude`、`kimi`、`jina`

#### Scenario: 大小写混合仍能识别

- **WHEN** 账单明细文本含大小写混合的关键词（如 `Gemini`、`VEO`）
- **THEN** 仍能正确匹配并归类（忽略大小写）

#### Scenario: 命中多个关键词取首个

- **WHEN** 账单明细文本同时命中多个品牌关键词
- **THEN** `APIBrandName` 取文本中位置最靠前的命中词（归并规则照常适用）

#### Scenario: 未命中任何关键词

- **WHEN** 账单明细文本不含任何 AI 关键词
- **THEN** `APIBrandName` 为空字符串

#### Scenario: GCP 品牌字段优先 hc_product_name 兜底 sku_description

- **WHEN** 一条 GCP 账单项 `hc_product_name` 含品牌关键词
- **THEN** `APIBrandName` 取自 `hc_product_name` 的命中结果，不再读取 `sku_description`

#### Scenario: GCP hc_product_name 未命中时回退 sku_description

- **WHEN** 一条 GCP 账单项 `hc_product_name` 不含品牌关键词，但 `sku_description` 含品牌关键词（如 Credit 条目）
- **THEN** `APIBrandName` 取自 `sku_description` 的命中结果

### Requirement: veo/imagen/lyria 归并为 gemini

系统 SHALL 在识别 API 厂商时，将命中的 `veo`、`imagen`、`lyria` 关键词统一归并为 `gemini`；其余命中关键词保持原值。最终 `APIBrandName` 取值范围（小写）SHALL 限定为 `gemini`、`claude`、`kimi`、`jina`。

#### Scenario: gemini 关键词直接归类

- **WHEN** 账单明细文本含 `gemini`
- **THEN** `APIBrandName` 为 `gemini`

#### Scenario: veo/imagen/lyria 归并

- **WHEN** 账单明细文本含 `veo`、`imagen` 或 `lyria`
- **THEN** `APIBrandName` 统一归并为 `gemini`

### Requirement: 华为账单本期 APIBrandName 留空

系统 SHALL 在本期对华为账单项的 `APIBrandName` 统一留空（华为当前无 AI 账单识别链路、不打 `_HCM_AI_` 前缀，字段保留，待确认华为存在 AI 账单及其文本字段后再补充识别规则）。

#### Scenario: 华为账单项 APIBrandName 留空

- **WHEN** 任意华为账单项上报
- **THEN** 该账单的 `APIBrandName` 为空字符串
