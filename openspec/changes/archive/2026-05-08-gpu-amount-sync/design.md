## Context

### 当前系统状态

HCM 平台通过 `hc-service` 服务从 CRP（Cloud Resource Platform）同步机型数据。同步流程如下：

1. **获取可用区列表**：根据地域查询该地域下的所有可用区
2. **获取机型列表**：调用 CRP `GetInstanceTypeInfo` 接口获取可用区下的机型名称列表
3. **获取机型详情**：调用 CRP `QueryCvmInstanceType` 接口获取机型的详细配置信息（包括 CPU、内存、GPU 等）
4. **数据同步**：将获取到的机型数据写入 `device_type` 表

当前 `device_type` 表结构已包含 CPU 核心数（`cpu_core`）和内存大小（`memory`）字段，但未包含 GPU 卡数字段。CRP 的 `QueryCvmInstanceType` 接口已返回 `gpuAmount` 字段（见 `pkg/thirdparty/cvmapi/cvmapi_response.go:781`），但同步逻辑未将其保存到数据库。

### 相关接口现状

| 接口 | 服务 | 路径 | 当前返回字段 |
|------|------|------|-------------|
| ListDeviceType | data-service | /device_types/list | device_type, device_class, device_family, core_type, cpu_core, memory, technical_class, ... |
| ListDistinctDeviceType | data-service | /device_types/distinct | 同上 |
| GetDeviceType | woa-server | /config/findmany/config/cvm/devicetype | 同上 |
| GetCvmDeviceDetail | woa-server | /config/findmany/config/cvm/device | 同上 |
| CreateManyDevice | woa-server | /config/createmany/config/cvm/device | 创建接口，需支持 gpu_amount 入参 |

## Goals / Non-Goals

**Goals:**
1. 在 `device_type` 表中新增 `gpu_amount` 字段，类型为 `float64`，默认值为 0
2. 修改机型同步逻辑，从 CRP `QueryCvmInstanceType` 接口获取 `gpuAmount` 并保存到数据库
3. 修改机型差异比较逻辑，将 `gpu_amount` 纳入变更检测
4. 扩展所有机型查询接口的返回结果，增加 `gpu_amount` 字段
5. 更新相关接口文档，添加 `gpu_amount` 字段说明

**Non-Goals:**
1. 不修改 CRP 接口调用方式（仅使用已有字段）
2. 不修改机型同步的触发逻辑和频率
3. 不修改机型数据的其他字段
4. 不涉及前端页面修改（仅后端接口返回字段扩展）
5. 不添加 GPU 类型的存储（如需要可后续扩展）

## Decisions

### 1. 字段类型选择

**决策**：`gpu_amount` 字段使用 `float64` 类型

**理由**：
- CRP 接口返回的 `GPUAmount` 为 `float64` 类型（`pkg/thirdparty/cvmapi/cvmapi_response.go:781`）
- 保留 CRP 原始精度，避免截断导致的数据丢失
- 业务层可根据需要自行处理小数逻辑

**替代方案**：使用 `int64` 类型 - 被拒绝，因为 CRP 返回的是 `float64`，截断可能丢失精度

### 2. 默认值策略

**决策**：`gpu_amount` 默认值为 0

**理由**：
- 非 GPU 机型的 GPU 卡数为 0
- 与现有字段（`cpu_core`、`memory`）的默认值策略一致
- 0 值在业务上有明确含义（无 GPU）

### 3. 同步逻辑变更范围

**决策**：修改以下三个位置以支持 GPU 卡数同步

1. **数据构建**（`cmd/hc-service/service/sync/tcloud-ziyan/device_type.go:188-203`）
   - 在构建 `coredevicetype.DeviceType` 时添加 `GpuAmount` 字段
   
2. **差异检测**（`cmd/hc-service/logics/res-sync/ziyan/device_type.go:119-157`）
   - 在 `isDeviceTypeChanged` 函数中增加 `gpu_amount` 字段比较
   
3. **创建/更新操作**（`cmd/hc-service/logics/res-sync/ziyan/device_type.go:159-219`）
   - 在 `createDeviceType` 和 `updateDeviceType` 函数中包含 `gpu_amount` 字段

### 4. 接口扩展策略

**决策**：采用最小侵入式扩展，仅添加返回字段

**修改文件清单**：

| 文件 | 修改内容 |
|------|----------|
| `pkg/dal/table/cloud/device-type/device_type.go` | 表结构添加 `gpu_amount` 字段 |
| `pkg/api/core/cloud/device-type/tcloud_ziyan.go` | `DeviceType` 和 `DistinctDeviceType` 结构体添加 `GpuAmount` 字段 |
| `pkg/api/data-service/cloud/device_type.go` | `DeviceTypeCreate` 和 `DeviceTypeUpdate` 结构体添加 `GpuAmount` 字段 |
| `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` | 同步时从 CRP 响应中提取 `gpuAmount` |
| `cmd/hc-service/logics/res-sync/ziyan/device_type.go` | 差异检测、创建、更新逻辑包含 `gpu_amount` |
| `cmd/data-service/service/cloud/device-type/query.go` | 转换函数包含 `gpu_amount` |
| `docs/api-docs/web-server/docs/scr/meta/list_device_type.md` | 接口文档添加 `gpu_amount` 说明 |
| `docs/api-docs/web-server/docs/scr/config-manage/create_config_cvm_device.md` | 接口文档添加 `gpu_amount` 说明 |

## Risks / Trade-offs

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 历史数据无 GPU 卡数 | 已同步的机型在数据库中 `gpu_amount` 为 0 或 NULL | 通过重新同步或增量更新补充数据；业务层需知晓 0 表示未设置或无 GPU |
| CRP 接口返回异常值 | 如果 CRP 返回负数或极大值，可能导致数据异常 | 添加校验逻辑，确保 `gpu_amount >= 0`；超出合理范围时记录警告日志 |
| 接口返回字段增加导致客户端解析问题 | 部分老旧客户端可能对新字段敏感 | 此变更为新增字段，JSON 解析通常忽略未知字段；已在 Non-Goals 中明确不涉及前端修改 |

## Migration Plan

### 数据库迁移

```sql
-- 在 device_type 表添加 gpu_amount 字段
ALTER TABLE device_type ADD COLUMN gpu_amount DOUBLE NOT NULL DEFAULT 0 COMMENT 'GPU卡数' AFTER memory;
```

### 部署步骤

1. **数据库变更**：执行上述 SQL，添加 `gpu_amount` 字段
2. **服务部署**：
   - 部署 `data-service`：支持新字段的读写
   - 部署 `hc-service`：支持同步 GPU 卡数
   - 部署 `woa-server`：支持返回 GPU 卡数
3. **数据同步**：触发一次机型全量同步，补充历史数据的 GPU 卡数
4. **文档更新**：合并接口文档更新

### 回滚策略

- 数据库字段添加为不可逆操作，但字段有默认值，不影响旧代码运行
- 如需紧急回滚，可部署旧版本服务，新字段将被忽略（不读取、不写入）

## Open Questions

1. **历史数据处理**：是否需要编写脚本对历史机型数据进行批量更新？还是依赖下一次全量同步？
   - 建议：依赖下一次全量同步，简化部署流程

2. **GPU 类型存储**：当前仅存储 GPU 卡数，是否需要存储 GPU 类型（如 V100、A100）？
   - 建议：当前需求不涉及，如需可在后续迭代中扩展（CRP 接口已返回 `gpuType` 字段）
