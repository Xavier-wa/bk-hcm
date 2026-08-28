## Why

资源预测通配当前按技术分类 + 核心类型判断两条机型能否共用预测额度。GPU 机型常同属 GPU 类，但卡类型（如 A100、V100）不同，预测不能通用。按现口径会把不同卡型误判为可通配，造成额度错占。机型主数据即将由依赖单补齐 `gpu_type`，本期在通配判定中消费该字段：先认 GPU 类，再按卡类型决定能否通配。

## What Changes

- 扩展资源预测通配判定：GPU 类且双方都有真实卡类型时，按 `gpu_type` 是否相同决定通配（可跨机型族）；不同卡类型不得通配
- GPU 类机型必须判断卡类型，禁止回退技术分类 + 核心类型；卡类型不是真实卡（空或「无」）时该机型不可通配：仅当前目标机型如此才报错阻断；池中脏 GPU、预测列表、可用机型列表只告警并退出通配
- 仅双方都非 GPU 类时，才保持现网技术分类 + 核心类型通配
- 申领扣减、回收、升降配校验、预测列表展开继续走同一套 `IsDeviceMatched`，不新开入口、不新增对外接口

本期不包含：卡类型别名合并、改造 CRP「小模型分卡管理」、非 CVM/非自研云通配、依赖单未上线前的过渡兼容、新权限点。

## Capabilities

### New Capabilities
- `res-plan-device-match`: 资源预测通配判定增加 GPU 卡类型维度，覆盖 GPU 类识别、真实卡比较、GPU 禁止回退、非真实卡按入口报错或告警，以及四处业务入口同一口径

### Modified Capabilities
- （无。现网通配判定没有独立 OpenSpec 基线 spec，本期以新 capability 描述完整规则）

## Impact

### 受影响的服务
- `woa-server`：资源预测通配判定 `IsDeviceMatched` 及申领扣减、回收、升降配校验、预测列表展开调用方

### 受影响的文件（预期）
- `cmd/woa-server/logics/plan/device_types.go`：通配判定规则
- `cmd/woa-server/logics/plan/device_types_test.go`：新增表驱动用例（预计新建）
- `cmd/woa-server/logics/plan/demand.go`、`demand_aggregate.go`、`verify.go`：确认四处入口仍只走 `IsDeviceMatched`；池构建 / 列表 / 可用机型对脏 GPU 降级为告警 + 不通配，不整次失败

### 依赖
- TAPD `1069995598136810399` / 变更 `device-type-gpu-type-sync`：机型主数据与 `DistinctDeviceType` 已有 `gpu_type`
- 机型缓存 `deviceTypesMap.GetDeviceTypes` 需能读到 `GpuType`、`TechnicalClass`、`DeviceFamily`、`CoreType`

### 兼容性
- 非 BREAKING：无新接口、无改请求响应字段；通配结果集合变化属于预期业务变化
- 同名机型仍视为可通配
- 权限点不变
