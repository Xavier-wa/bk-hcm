## Why

自研云 CVM 机型主数据已同步 GPU 卡数（`gpu_amount`），但未同步 GPU 卡类型（如 A100、V100）。资源运营在机型管理、容量规划和申领核对时只能看到“有几张卡”，无法区分卡型号，存在线下对照错配风险。同时，主动登记创建页已能填卡数，但不能提交卡类型和技术分类资源量，手工补录的 GPU 机型规格不完整。CRP `queryCvmInstanceType` 已返回 `gpuType`，本期将其落入 `device_type.gpu_type`，并打通主动登记与查询消费路径。

## What Changes

- 在 `device_type` 表新增 `gpu_type`（字符串，最大 64，默认空字符串），全链路（DAL / Core / data-service 创建更新查询）透传该字段
- 自研云机型同步从 CRP `QueryCvmInstanceType.gpuType` 写入/更新 `gpu_type`；空值仍同步成功；仅卡类型变化也必须更新
- 主动登记创建接口允许可选提交 `gpu_type`、`tech_class_res_amt`；创建页以可搜索下拉录入卡类型（选项外允许手输），并补齐技术分类资源量录入
- 机型查询接口返回 `gpu_type`；机型配置页增加卡类型筛选；已有规格详情可展示卡类型
- 更新相关接口文档（创建入参、列表/查询出参）
- 历史数据不做离线回填，依赖上线后常规同步补齐

本期不包含：CVM 申领按卡类型筛选/推荐、其他云厂商、后端强制卡类型枚举、配置列表加列、独立详情页、批量编辑卡类型。

## Capabilities

### New Capabilities
- `device-type-gpu-type`: 自研云机型 GPU 卡类型主数据、同步、主动登记、查询返回、配置页筛选与规格详情展示

### Modified Capabilities
- `ziyan-device-type-sync`: 自研云机型详情映射增加 `gpu_type`；不得因卡类型为空额外跳过机型

## Impact

### 受影响的服务
- `data-service`：机型表读写与查询返回 `gpu_type`
- `hc-service`：自研云机型同步写入、差异检测、创建/更新
- `woa-server`：主动登记创建与机型查询透传新字段（结构体透传，逻辑基本不变）
- `web-server`：接口文档；前端机型配置创建页、筛选、规格详情展示

### 受影响的文件（预期）
- 表结构：`pkg/dal/table/cloud/device-type/device_type.go`
- Core：`pkg/api/core/cloud/device-type/tcloud_ziyan.go`
- API：`pkg/api/data-service/cloud/device_type.go`
- 同步构建：`cmd/hc-service/service/sync/tcloud-ziyan/device_type.go`
- 同步 diff/写库：`cmd/hc-service/logics/res-sync/ziyan/device_type.go` 及测试
- 数据服务：`cmd/data-service/service/cloud/device-type/create.go`、`update.go`
- 前端：`front/src/views/ziyanScr/cvm-model/CreateDevice/index.tsx`、`front/src/views/ziyanScr/cvm-model/index.tsx`、申领规格详情相关组件
- 文档：`docs/api-docs/web-server/docs/scr/config-manage/create_config_cvm_device.md`、`list_config_cvm_device.md`、`docs/api-docs/web-server/docs/scr/meta/list_device_type.md` 等
- 迁移：`scripts/sql/` 新增 `9999_*_device_type_gpu_type.sql`（`SQLVER=9999`、`HCMVER=v9.9.9.9`，出包前再替换真实编号）

### 依赖
- CRP `QueryCvmInstanceType` 已有 `gpuType`（`pkg/thirdparty/cvmapi/cvmapi_response.go`），本期只消费不改 CRP 客户端契约
- `tech_class_res_amt` 后端创建/更新结构已存在，本期补文档、创建页与入参透传

### 兼容性
- 非 BREAKING：新增可选字段与返回字段；旧调用方不传 `gpu_type` 时落库为空，行为与现在一致
- 权限点不变（平台-CVM机型）
