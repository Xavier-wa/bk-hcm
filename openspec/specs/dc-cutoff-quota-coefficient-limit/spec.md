# dc-cutoff-quota-coefficient-limit

## Purpose

定义机房裁撤场景下的配额系数与业务偏移机制，将原有的 1:1 等额申领模式改造为可配置的全局配额系数 + 业务偏移额度的灵活模式。

## Requirements

### Requirement: 可申请额度计算公式

系统 SHALL 使用以下公式计算业务可申请额度：
```
可申请额度 = max(0, 裁撤原始核数 × 配额系数 + 业务偏移额度 - 已交付核数)
```

#### Scenario: 正常计算可申请额度
- **WHEN** 业务裁撤原始核数为 1000，配额系数为 65（百分比），偏移额度为 50（increase），已交付 100 核
- **THEN** 系统 SHALL 计算可申请额度为 max(0, 1000 × 65/100 + 50 - 100) = 600 核

#### Scenario: 计算结果为负数时返回 0
- **WHEN** 计算结果为负数
- **THEN** 系统 SHALL 返回可申请额度为 0

#### Scenario: 偏移类型为 decrease 时作为负数计算
- **WHEN** 偏移类型为 "decrease"
- **THEN** 系统 SHALL 将偏移值作为负数参与计算
- **WHEN** 偏移类型为 "increase"
- **THEN** 系统 SHALL 将偏移值作为正数参与计算

### Requirement: 配额系数配置

系统 SHALL 在 `global_config` 表中存储配额系数，`config_type = 'res_dissolve'`，`config_key = 'dissolve_quota_coefficient'`。

#### Scenario: 未配置时使用默认值
- **WHEN** 配额系数未配置
- **THEN** 系统 SHALL 使用默认值 65（百分比）

#### Scenario: 配额系数范围校验
- **WHEN** 传入的配额系数超出 1 ~ 100 范围
- **THEN** 系统 SHALL 返回 `InvalidParameter` 错误

#### Scenario: 配额系数即时生效
- **WHEN** 配额系数被更新
- **THEN** 新值 SHALL 立即对后续所有额度计算生效

### Requirement: 偏移上限控制

系统 SHALL 使用以下公式计算偏移上限，确保最大可申请额度不超过原始裁撤总量：
```
偏移上限 = 原始裁撤核数 × (100 - 配额系数) / 100
```

#### Scenario: 偏移在上限内
- **WHEN** 原始裁撤 1000 核，配额系数 65，偏移 300
- **THEN** 系统 SHALL 允许该偏移，因为 300 ≤ 1000 × 35/100 = 350

#### Scenario: 偏移超过上限
- **WHEN** 原始裁撤 1000 核，配额系数 65，偏移 400
- **THEN** 系统 SHALL 拒绝该偏移并返回错误，因为 400 > 1000 × 35/100 = 350

### Requirement: 偏移数据存储

系统 SHALL 在 `global_config` 表中以 JSON 格式存储各业务的偏移额度配置，`config_type = 'res_dissolve'`，`config_key = 'dissolve_quota_offsets'`。

#### Scenario: 偏移 JSON 结构
- **WHEN** 存储偏移配置
- **THEN** JSON 结构 SHALL 如下：
```json
[
    {"bk_biz_id": 100001, "offset": 50, "type": "increase", "memo": "特殊业务需求"},
    {"bk_biz_id": 100002, "offset": 30, "type": "decrease", "memo": "额度回收"}
]
```

#### Scenario: 业务无偏移记录
- **WHEN** 业务在偏移配置中无记录
- **THEN** 系统 SHALL 将偏移值视为 0

### Requirement: 获取配置接口

系统 SHALL 提供 `GET /api/v1/woa/dissolve/config` 接口返回当前裁撤配置。

#### Scenario: 成功获取配置
- **WHEN** 调用 GET 请求
- **THEN** 系统 SHALL 返回包含 `host_apply_time`、`approval_limit`、`quota_coefficient`、`quota_offsets` 的配置信息

#### Scenario: 响应结构
- **WHEN** 获取配置
- **THEN** 响应 SHALL 遵循以下结构：
```json
{
    "code": 0,
    "data": {
        "host_apply_time": "2026-01-01T00:00:00Z",
        "approval_limit": 80,
        "quota_coefficient": 65,
        "quota_offsets": [
            {"bk_biz_id": 100001, "offset": 50, "type": "increase", "memo": "特殊业务需求"}
        ]
    }
}
```

### Requirement: 更新配置接口（全量覆盖）

系统 SHALL 提供 `PUT /api/v1/woa/dissolve/config` 接口对裁撤配置进行全量覆盖更新。

#### Scenario: 成功更新配置
- **WHEN** 发送有效的 PUT 请求，包含所有配置字段
- **THEN** 系统 SHALL 全量覆盖配置并返回成功

#### Scenario: 更新时校验配额系数
- **WHEN** `quota_coefficient` 超出 1 ~ 100 范围
- **THEN** 系统 SHALL 返回 `InvalidParameter` 错误

#### Scenario: 清空所有偏移
- **WHEN** `quota_offsets` 设置为空数组 `[]`
- **THEN** 系统 SHALL 清空所有业务的偏移配置

#### Scenario: 操作日志记录
- **WHEN** 配置更新成功
- **THEN** 系统 SHALL 记录操作日志，包含操作人、操作时间、请求 ID 和配置内容，便于审计追溯

### Requirement: 单业务偏移修改接口

系统 SHALL 提供 `PUT /api/v1/woa/dissolve/quota/offset/{bk_biz_id}` 接口，支持单独修改某个业务的偏移额度。

#### Scenario: 成功修改单业务偏移
- **WHEN** 传入有效的路径参数 `bk_biz_id` 和请求体 `offset`、`type`、`memo`
- **THEN** 系统 SHALL 更新该业务的偏移配置，返回修改前后的偏移值

#### Scenario: 请求结构
- **WHEN** 发送请求
- **THEN** 路径参数 SHALL 包含：
  - `bk_biz_id` (int64, 必需): 业务 ID
- **AND** 请求体 SHALL 包含以下字段：
  - `offset` (int64, 必需): 偏移值，必须大于等于 0
  - `type` (string, 必需): "increase" 或 "decrease"
  - `memo` (string, 可选, 最大 512 字符): 调整原因

#### Scenario: 成功响应结构
- **WHEN** 偏移更新成功
- **THEN** 响应 SHALL 遵循以下结构：
```json
{
    "code": 0,
    "data": {
        "bk_biz_id": 100001,
        "before_offset": 0,
        "after_offset": 50
    }
}
```

#### Scenario: 偏移超过上限
- **WHEN** 偏移值超过该业务的计算上限
- **THEN** 系统 SHALL 返回 `InvalidParameter` 错误

#### Scenario: 备注长度校验
- **WHEN** memo 超过 512 字符
- **THEN** 系统 SHALL 返回 `InvalidParameter` 错误

### Requirement: 类型定义

系统 SHALL 在 `cmd/woa-server/types/dissolve/` 定义以下类型：

#### Scenario: QuotaOffsetItem 结构体
- **WHEN** 定义偏移项
- **THEN** 结构体 SHALL 包含：
  - `BkBizID` (int64): 业务 ID
  - `Offset` (int64): 偏移值
  - `Type` (string): "increase" 或 "decrease"
  - `Memo` (string): 调整原因

#### Scenario: Config 响应结构体
- **WHEN** 定义配置响应
- **THEN** 结构体 SHALL 包含：
  - `HostApplyTime` (*time.Time)
  - `ApprovalLimit` (*float64)
  - `QuotaCoefficient` (*float64)
  - `QuotaOffsets` ([]QuotaOffsetItem): 偏移配置数组

#### Scenario: UpdateQuotaOffsetReq 请求结构体
- **WHEN** 定义单业务偏移修改请求
- **THEN** 结构体 SHALL 包含：
  - `Offset` (int64, 必需): 偏移值，必须大于等于 0
  - `Type` (string, 必需): "increase" 或 "decrease"
  - `Memo` (string, 可选): 调整原因

#### Scenario: UpdateQuotaOffsetResp 响应结构体
- **WHEN** 定义单业务偏移修改响应
- **THEN** 结构体 SHALL 包含：
  - `BkBizID` (int64)
  - `BeforeOffset` (int64)
  - `AfterOffset` (int64)

### Requirement: 额度汇总逻辑改造

系统 SHALL 修改额度汇总逻辑，使用新公式计算可申请额度。

#### Scenario: 汇总响应包含新字段
- **WHEN** 调用额度汇总接口（`POST /api/v1/woa/dissolve/cpu_core/summary` 或 `POST /api/v1/woa/bizs/{bk_biz_id}/dissolve/cpu_core/summary`）
- **THEN** 系统 SHALL 返回 `available_quota` 字段（后端计算好的可申请额度）
- **AND** 前端直接使用 `available_quota` 字段显示可申领额度，无需自行计算
- **NOTE** `quota_coefficient` 和 `quota_offset` 字段不在响应中返回，仅在后端内部计算使用

#### Scenario: 配额系数对所有业务统一生效
- **WHEN** 计算任意业务的可申请额度
- **THEN** 全局配额系数 SHALL 统一应用

### Requirement: 主机申请校验逻辑改造

系统 SHALL 修改主机申请校验逻辑，使用新的可申请额度计算方式进行校验。

#### Scenario: 申请在可用额度内
- **WHEN** 主机申请数量在计算的可申请额度内
- **THEN** 系统 SHALL 允许申请

#### Scenario: 申请超过可用额度
- **WHEN** 主机申请数量超过计算的可申请额度
- **THEN** 系统 SHALL 拒绝申请并返回包含可申请额度详情的错误信息

### Requirement: 路由注册

系统 SHALL 在 woa-server 路由配置中注册新的 API 路由 `PUT /api/v1/woa/dissolve/quota/offset/{bk_biz_id}`。

#### Scenario: 路由可访问
- **WHEN** 服务启动
- **THEN** 单业务偏移修改的 PUT 端点 SHALL 可访问

## Compatibility

### Requirement: 向后兼容性

#### Scenario: 未配置系统使用默认配额系数
- **WHEN** 配额系数未配置
- **THEN** 系统 SHALL 使用默认值 65（百分比），不影响现有功能

#### Scenario: 已交付额度不追溯调整
- **WHEN** 配额系数生效
- **THEN** 已交付额度 SHALL 不受影响，仅限制后续可申请额度

#### Scenario: 现有接口响应扩展
- **WHEN** 调用现有的配置 GET 接口
- **THEN** 响应 SHALL 包含新增字段（`quota_coefficient`、`quota_offsets`），同时保留现有字段
