## ADDED Requirements

### Requirement: 配额系数配置管理

系统 SHALL 支持在 `global_config` 表中存储和管理机房裁撤配额系数，`config_type = 'res_dissolve'`，`config_key = 'dissolve_quota_coefficient'`。

#### Scenario: 获取配额系数
- **WHEN** 调用 `GET /api/v1/woa/dissolve/config`
- **THEN** 系统 SHALL 返回当前配置的配额系数
- **AND** 若未配置则返回默认值 `enumor.DefaultQuotaCoefficient`（65，百分比）

#### Scenario: 更新配额系数
- **WHEN** 调用 `PUT /api/v1/woa/dissolve/config` 并传入 `quota_coefficient`
- **THEN** 系统 SHALL 校验值在 1 ~ 100 范围内
- **AND** 更新成功后立即生效

#### Scenario: 配额系数校验失败
- **WHEN** 传入的 `quota_coefficient` 超出 1 ~ 100 范围
- **THEN** 系统 SHALL 返回 `InvalidParameter` 错误

### Requirement: 业务偏移额度配置管理

系统 SHALL 支持在 `global_config` 表中以 JSON 格式存储各业务的偏移额度配置，`config_type = 'res_dissolve'`，`config_key = 'dissolve_quota_offsets'`。

#### Scenario: 获取偏移配置
- **WHEN** 调用 `GET /api/v1/woa/dissolve/config`
- **THEN** 系统 SHALL 返回 `quota_offsets` 字段，包含所有业务的偏移配置
- **AND** 每个业务配置包含 `offset`、`type`、`memo` 字段

#### Scenario: 全量更新偏移配置
- **WHEN** 调用 `PUT /api/v1/woa/dissolve/config` 并传入 `quota_offsets`
- **THEN** 系统 SHALL 全量覆盖现有偏移配置
- **AND** 记录操作日志（操作人、操作时间、配置内容）

#### Scenario: 清空偏移配置
- **WHEN** 调用 `PUT /api/v1/woa/dissolve/config` 并传入空的 `quota_offsets: []`
- **THEN** 系统 SHALL 清空所有业务的偏移配置

### Requirement: 单业务偏移修改接口

系统 SHALL 提供 `PUT /api/v1/woa/dissolve/quota/offset/{bk_biz_id}` 接口，支持单独修改某个业务的偏移额度。

#### Scenario: 成功修改单业务偏移
- **WHEN** 调用接口传入 `offset`（指针类型 `*int64`，必须 >= 0）、`type`、`memo`
- **THEN** 系统 SHALL 更新该业务的偏移配置
- **AND** 返回修改前后的偏移值

#### Scenario: 偏移值为0有效
- **WHEN** 传入的 `offset` 为 0
- **THEN** 系统 SHALL 允许该操作（使用指针类型支持传入 0 值）

#### Scenario: 偏移超过上限
- **WHEN** 传入的 `offset` 超过偏移上限（原始裁撤核数 × (100 - 配额系数) / 100）
- **THEN** 系统 SHALL 返回 `InvalidParameter` 错误

#### Scenario: 备注长度校验
- **WHEN** 传入的 `memo` 超过 512 字符
- **THEN** 系统 SHALL 返回 `InvalidParameter` 错误

### Requirement: 可申请额度计算公式

系统 SHALL 使用新公式计算业务可申请额度：`可申请额度 = max(0, 裁撤原始核数 × 配额系数/100 + 业务偏移额度 - 已交付核数)`。

#### Scenario: 正常计算可申请额度
- **WHEN** 业务裁撤原始核数为 1000，配额系数为 65（百分比），偏移额度为 50（increase），已交付 100 核
- **THEN** 系统 SHALL 计算可申请额度为 max(0, 1000 × 65/100 + 50 - 100) = 600 核

#### Scenario: 计算结果为负数
- **WHEN** 计算结果为负数
- **THEN** 系统 SHALL 返回可申请额度为 0

#### Scenario: 偏移类型为 decrease
- **WHEN** 偏移类型为 "decrease"
- **THEN** 系统 SHALL 将偏移值作为负数参与计算

#### Scenario: 业务无偏移记录
- **WHEN** 业务在 `quota_offsets` 中无记录
- **THEN** 系统 SHALL 将偏移值视为 0

### Requirement: 偏移上限控制

系统 SHALL 使用公式 `偏移上限 = 原始裁撤核数 × (100 - 配额系数) / 100` 计算偏移上限，确保最大可申请额度不超过原始裁撤总量。

#### Scenario: 偏移在上限内
- **WHEN** 原始裁撤 1000 核，配额系数 65，偏移 300
- **THEN** 系统 SHALL 允许该偏移，因为 300 ≤ 1000 × 35/100 = 350

#### Scenario: 偏移超过上限
- **WHEN** 原始裁撤 1000 核，配额系数 65，偏移 400
- **THEN** 系统 SHALL 拒绝该偏移，因为 400 > 1000 × 35/100 = 350

## MODIFIED Requirements

### Requirement: 额度汇总逻辑改造

额度汇总逻辑 SHALL 使用新公式计算并返回可申请额度。

#### Scenario: 汇总响应包含新字段
- **WHEN** 调用额度汇总接口（`POST /api/v1/woa/dissolve/cpu_core/summary` 或 `POST /api/v1/woa/bizs/{bk_biz_id}/dissolve/cpu_core/summary`）
- **THEN** 系统 SHALL 返回 `available_quota` 字段（后端计算好的可申请额度）
- **AND** 前端直接使用 `available_quota` 字段显示可申领额度，无需自行计算
- **NOTE** `quota_coefficient` 和 `quota_offset` 字段不在响应中返回，仅在后端内部计算使用

### Requirement: 主机申请校验逻辑改造

主机申请校验逻辑 SHALL 使用新公式计算可申请额度进行校验。

#### Scenario: 申请在可用额度内
- **WHEN** 主机申请数量在计算的可申请额度内
- **THEN** 系统 SHALL 允许申请

#### Scenario: 申请超过可用额度
- **WHEN** 主机申请数量超过计算的可申请额度
- **THEN** 系统 SHALL 拒绝申请

### Requirement: 审批阈值判断逻辑改造

审批阈值判断逻辑 SHALL 基于配额额度计算已交付比例。

#### Scenario: 审批阈值计算公式
- **WHEN** 判断是否需要审批
- **THEN** 系统 SHALL 使用公式：`配额额度 = 裁撤原始核数 × 配额系数 / 100 + 业务偏移额度`
- **AND** 计算 `已交付比例 = 已交付核数 / 配额额度 × 100`
- **AND** 当 `已交付比例 >= approval_limit` 时需要审批

#### Scenario: 配额额度为零或负数
- **WHEN** 计算的配额额度 <= 0
- **THEN** 系统 SHALL 返回错误，拒绝处理
- **AND** 返回包含可申请额度详情的错误信息

### Requirement: 常量与枚举定义

系统 SHALL 在 `pkg/criteria/enumor/global_config.go` 中定义配额相关常量和枚举类型，避免硬编码。

#### Scenario: 偏移类型枚举
- **WHEN** 校验偏移类型
- **THEN** 系统 SHALL 使用 `enumor.QuotaOffsetType` 类型及其 `Validate()` 方法
- **AND** 支持的值为 `QuotaOffsetTypeIncrease = "increase"` 和 `QuotaOffsetTypeDecrease = "decrease"`

#### Scenario: 默认配额系数常量
- **WHEN** 配额系数未配置
- **THEN** 系统 SHALL 使用 `enumor.DefaultQuotaCoefficient`（值为 65）作为默认值
