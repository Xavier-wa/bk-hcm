# 机房裁撤配额系数与偏移管理 - 提案

## Why

机房裁撤场景由原 1:1 等额申领模式运行，存在以下问题：

1. **资源释放规模不可控**：缺乏全局配额系数管控，无法灵活调整整体资源释放比例
2. **业务差异化诉求无法满足**：特殊业务需要额外的额度支持，但缺少偏移量配置机制
3. **配置管理分散**：配额相关配置分布在多处，缺乏统一管理入口

## What Changes

- 新增全局配额系数（`quota_coefficient`），支持 1 ~ 100 范围配置（百分比），默认值 65
- 新增业务偏移额度配置（`quota_offsets`），支持为特殊业务配置正/负偏移量（数组格式存储）
- 新增偏移上限控制，偏移量不能超过 `原始裁撤核数 × (100 - 配额系数) / 100`
- 可申请额度计算公式改为：`max(0, 裁撤原始核数 × 配额系数/100 + 业务偏移额度 - 已交付核数)`
- 复用 `global_config` 表存储配额系数和偏移配置（JSON 数组格式）
- 额度汇总接口仅返回 `available_quota` 字段，前端直接使用后端计算结果

## Capabilities

### New Capabilities

- `dissolve-quota-coefficient`: 机房裁撤全局配额系数配置，控制资源释放比例
- `dissolve-quota-offset`: 业务偏移额度配置，支持为特殊业务配置额外额度
- `dissolve-config-api`: 统一配置管理接口（GET/PUT），支持配额系数和偏移配置的读写
- `dissolve-offset-api`: 单业务偏移修改接口（`PUT /api/v1/woa/dissolve/quota/offset/{bk_biz_id}`）

### Modified Capabilities

- `dissolve-quota-calculation`: 额度计算逻辑改造，使用新公式计算可申请额度
- `dissolve-host-apply-validation`: 主机申请校验逻辑改造，基于新公式进行校验

## Impact

- **数据库层**：复用 `global_config` 表，新增 `dissolve_quota_coefficient` 和 `dissolve_quota_offsets` 配置项
- **枚举层**：`pkg/criteria/enumor/` 新增配置键常量、偏移类型枚举 `DissolveQuotaOffsetType`
- **类型定义**：`cmd/woa-server/types/dissolve/` 新增配置结构体、请求/响应结构体
- **服务层 (woa-server)**：扩展配置管理接口、新增单业务偏移修改接口
- **逻辑层 (woa-server)**：改造额度计算逻辑、主机申请校验逻辑
- **不影响**：已交付额度不受系数影响，仅限制后续可申请额度
