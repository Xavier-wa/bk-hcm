## Why

当前 HCM 平台在从 CRP 同步机型数据时，未包含 GPU 卡数信息。随着 GPU 机型在业务中的广泛使用，用户需要在查询机型详情时获取 GPU 卡数信息，以便进行资源规划和成本核算。本变更将在机型表中增加 `gpu_amount` 字段，并在同步流程中从 CRP 接口获取 GPU 卡数信息保存到数据库，同时更新相关查询接口以返回该字段。

## What Changes

1. **数据库表结构变更**：在 `device_type` 表中新增 `gpu_amount` 字段，用于存储 GPU 卡数量
2. **同步逻辑增强**：修改机型同步逻辑，在调用 `QueryCvmInstanceType` 接口时获取 `gpuAmount` 字段并保存到机型表
3. **API 接口扩展**：所有查询机型明细的接口增加 `gpu_amount` 返回字段，包括：
   - `ListDeviceType` - 查询机型列表
   - `ListDistinctDeviceType` - 查询去重机型列表
   - woa-server 的 `/config/createmany/config/cvm/device` 接口
4. **接口文档更新**：同步更新相关接口文档，新增 `gpu_amount` 字段说明

## Capabilities

### New Capabilities
- `gpu-amount-sync`: 机型 GPU 卡数同步与查询能力，支持从 CRP 同步 GPU 卡数并在各查询接口中返回

### Modified Capabilities
- 无（此变更为纯数据字段扩展，不涉及现有接口行为变更）

## Impact

### 受影响的服务
- `hc-service`: 修改机型同步逻辑，从 CRP 接口获取 GPU 卡数
- `data-service`: 修改机型表的创建、更新、查询逻辑
- `cloud-server`: 机型查询接口返回字段扩展
- `web-server`: 机型查询接口返回字段扩展
- `woa-server`: `/config/createmany/config/cvm/device` 接口返回字段扩展

### 受影响的文件
- 数据表定义：`pkg/dal/table/cloud/device-type/device_type.go`
- 核心模型：`pkg/api/core/cloud/device-type/tcloud_ziyan.go`
- API 协议：`pkg/api/data-service/cloud/device_type.go`
- 同步逻辑：`cmd/hc-service/service/sync/tcloud-ziyan/device_type.go`
- 同步客户端：`cmd/hc-service/logics/res-sync/ziyan/device_type.go`
- 数据服务查询：`cmd/data-service/service/cloud/device-type/query.go`
- woa-server 配置接口：`cmd/woa-server/service/config/device.go`
- 接口文档：`docs/api-docs/web-server/docs/scr/meta/list_device_type.md`
- 接口文档：`docs/api-docs/web-server/docs/scr/config-manage/create_config_cvm_device.md`

### 依赖的第三方接口
- CRP `QueryCvmInstanceType` 接口（已包含 `gpuAmount` 字段，位于 `pkg/thirdparty/cvmapi/cvmapi_response.go:781`）

### 兼容性说明
- 数据库变更：新增字段，向后兼容
- API 变更：新增返回字段，向后兼容
- 同步逻辑：新增字段同步，不影响现有字段同步
