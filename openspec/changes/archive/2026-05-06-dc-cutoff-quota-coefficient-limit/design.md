# 机房裁撤配额系数与偏移管理 - 设计文档

## Context

机房裁撤场景支持业务从裁撤机房申领等额资源，原有模式为 1:1 等额申领。新增全局配额系数管控整体资源释放规模，同时支持为特殊业务配置额度偏移量，在统一规则基础上灵活补差，满足差异化业务诉求。

**涉及服务**：woa-server（服务层）、data-service（资源层，复用 global_config）

## Goals / Non-Goals

**Goals:**
- 支持全局配额系数配置（1 ~ 100 百分比），控制资源释放比例
- 支持业务级偏移额度配置，满足特殊业务差异化诉求
- 偏移上限控制，防止超额申请
- 配置即时生效，无需关联裁撤周期
- 全量更新时记录操作日志，方便回溯

**Non-Goals:**
- 不对存量已交付额度进行追溯调整
- 不支持按裁撤周期自动清理偏移配置（由管理员手动处理）

## Decisions

### 1. 数据存储：复用 global_config 表

**决策**：复用现有 `global_config` 表存储配额系数和偏移配置，不新建表。

**理由**：
- `global_config` 表已有成熟的配置管理机制
- 配额系数和偏移配置本质上是全局配置项
- 减少数据库 schema 变更，降低维护成本

**存储结构**：
```sql
-- 配额系数：config_type = 'res_dissolve', config_key = 'dissolve_quota_coefficient'
-- config_value = '65'

-- 偏移配置：config_type = 'res_dissolve', config_key = 'dissolve_quota_offsets'
-- config_value = '[{"bk_biz_id": 100001, "offset": 50, "type": "increase", "memo": "特殊业务需求"}, ...]'
```

### 2. 偏移类型：使用 type 字段区分正负

**决策**：偏移值使用正整数 `offset` 配合 `type` 字段（increase/decrease）区分方向。

**理由**：
- 语义更清晰，避免负数带来的理解歧义
- 与调账（BillAdjustment）等现有功能保持一致的设计风格
- 便于前端展示和校验

**备选**：直接使用有符号整数表示偏移（正数=增加，负数=减少），因语义不够直观而放弃。

### 3. 操作日志：日志记录而非建表

**决策**：全量覆盖更新时通过日志打印记录操作人、操作时间、配置内容，不新建审计表。

**理由**：
- 需求仅要求"方便回溯"，日志记录足以满足
- 避免额外的数据库表维护成本
- 可通过日志系统进行检索和查询

### 4. 偏移上限：基于配额系数动态计算

**决策**：偏移上限 = 原始裁撤核数 × (100 - 配额系数) / 100

**理由**：
- 确保最大可申请额度不超过原始裁撤总量
- 配额系数调整时，偏移上限自动联动

**示例**：原始裁撤 1000 核，系数 65
- 基础额度 = 1000 × 65/100 = 650 核
- 偏移上限 = 1000 × 35/100 = 350 核
- 最大可申请 = 650 + 350 = 1000 核

## API Design

### 获取配置

```
GET /api/v1/woa/dissolve/config
```

**响应**：
```json
{
    "code": 0,
    "data": {
        "host_apply_time": "2026-01-01T00:00:00Z",
        "approval_limit": 80,
        "quota_coefficient": 65,
        "quota_offsets": [
            {"bk_biz_id": 100001, "offset": 50, "type": "increase", "memo": "特殊业务需求"},
            {"bk_biz_id": 100002, "offset": 30, "type": "decrease", "memo": "额度回收"}
        ]
    }
}
```

### 更新配置（全量覆盖）

```
PUT /api/v1/woa/dissolve/config
```

**请求**：
```json
{
    "host_apply_time": "2026-01-01T00:00:00Z",
    "approval_limit": 80,
    "quota_coefficient": 65,
    "quota_offsets": [
        {"bk_biz_id": 100001, "offset": 50, "type": "increase", "memo": "特殊业务需求"}
    ]
}
```

**校验规则**：
- `quota_coefficient`：1 ~ 100（百分比），超出范围返回 `InvalidParameter`

### 单业务偏移修改（仅后端接口）

```
PUT /api/v1/woa/dissolve/quota/offset/{bk_biz_id}
```

**请求**：
```json
{
    "offset": 50,
    "type": "increase",
    "memo": "特殊业务需求"
}
```

**响应（成功）**：
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

**校验规则**：
- `offset`：必填，必须 >= 0（使用指针类型 `*int64` 以支持传入 0 值）
- `type`：必填，枚举值 `increase` 或 `decrease`
- `memo`：可选，最大 512 字符

### 审批阈值判断逻辑

审批阈值的判断基于**配额额度**计算已交付比例：

```
配额额度 = 裁撤原始核数 × 配额系数 / 100 + 业务偏移额度
已交付比例 = 已交付核数 / 配额额度 × 100
```

当 `已交付比例 >= approval_limit` 时，需要审批。

## Data Structures

```go
// pkg/criteria/enumor/global_config.go

// DissolveQuotaOffsetType 偏移类型
type DissolveQuotaOffsetType string

const (
    DissolveQuotaOffsetTypeIncrease DissolveQuotaOffsetType = "increase"
    DissolveQuotaOffsetTypeDecrease DissolveQuotaOffsetType = "decrease"
)

func (t DissolveQuotaOffsetType) Validate() error {
    if t != DissolveQuotaOffsetTypeIncrease && t != DissolveQuotaOffsetTypeDecrease {
        return fmt.Errorf("invalid dissolve quota offset type: %s", t)
    }
    return nil
}

// DefaultDissolveQuotaCoefficient 默认配额系数（百分比，范围1-100）
const DefaultDissolveQuotaCoefficient float64 = 65
```

```go
// cmd/woa-server/types/dissolve/types.go

// QuotaOffsetItem 单个业务偏移配置
type QuotaOffsetItem struct {
    BkBizID int64                          `json:"bk_biz_id"` // 业务ID
    Offset  int64                          `json:"offset"`    // 偏移值
    Type    enumor.DissolveQuotaOffsetType `json:"type"`      // 调整类型：increase=调增，decrease=调减
    Memo    string                         `json:"memo"`      // 调整原因
}

// Config 配置响应结构体
type Config struct {
    HostApplyTime    *time.Time        `json:"host_apply_time"`
    ApprovalLimit    *float64          `json:"approval_limit"`
    QuotaCoefficient *float64          `json:"quota_coefficient,omitempty"`
    QuotaOffsets     []QuotaOffsetItem `json:"quota_offsets,omitempty"`
}

// UpsertConfigReq 更新配置请求
type UpsertConfigReq struct {
    HostApplyTime    *time.Time        `json:"host_apply_time" validate:"omitempty"`
    ApprovalLimit    *float64          `json:"approval_limit" validate:"omitempty"`
    QuotaCoefficient *float64          `json:"quota_coefficient" validate:"omitempty"`
    QuotaOffsets     []QuotaOffsetItem `json:"quota_offsets" validate:"omitempty"`
}

// UpdateDissolveQuotaOffsetReq 单业务偏移修改请求（bk_biz_id 在路径参数中）
type UpdateDissolveQuotaOffsetReq struct {
    Offset *int64                         `json:"offset" validate:"required,min=0"`
    Type   enumor.DissolveQuotaOffsetType `json:"type" validate:"required"`
    Memo   string                         `json:"memo" validate:"max=512"`
}

// UpdateDissolveQuotaOffsetResp 单业务偏移修改响应
type UpdateDissolveQuotaOffsetResp struct {
    BkBizID      int64 `json:"bk_biz_id"`
    BeforeOffset int64 `json:"before_offset"`
    AfterOffset  int64 `json:"after_offset"`
}
```

## Risks / Trade-offs

- **配额系数默认值 `enumor.DefaultDissolveQuotaCoefficient`（65%）** → 未配置时自动使用，可能与业务预期不符。缓解：上线前与业务确认默认值是否合适。
- **偏移配置不自动清理** → 新裁撤周期开始时需管理员手动删除偏移配置。缓解：在管理页面提示管理员处理。
- **偏移上限依赖实时查询裁撤核数** → 若查询性能差会影响接口响应。缓解：可考虑缓存或异步校验。
