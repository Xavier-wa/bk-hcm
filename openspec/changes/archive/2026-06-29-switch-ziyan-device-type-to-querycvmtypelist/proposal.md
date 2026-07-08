## Why

自研云（TCloudZiyan）机型同步当前通过 CRP `GetInstanceTypeInfo` 获取机型列表，该接口仅返回「已填报需求预测」的机型，存在数据缺失、与 CRP 系统展示不一致的问题。CRP 已封装 `queryCvmTypeList` 接口返回「可填报需求预测」的全量机型，需要将同步数据源切换过去以修复缺失。

## What Changes

- 自研云机型同步「获取机型列表」的数据源从 `GetInstanceTypeInfo` 切换为 `QueryCvmTypeList`；机型详情查询 `QueryCvmInstanceType` 逻辑保持不变。
- `QueryCvmTypeListParams` 新增 `regionNames` / `cityNames` / `zoneNames` 三个可选过滤字段（向后兼容，不影响既有调用方）。
- 同步时按可用区循环，`zoneNames` 传入当前可用区的**中文名**（CRP 接口口径），`deptName` 传 `CvmLaunchDeptName="IEG技术运营部"`。
- `device_type` 表 `zone` 字段口径保持不变（仍存可用区**编码**），中文名仅用于接口过滤。
- 修正 `QueryCvmTypeList` 接口/参数注释，准确描述其为「查询可填报需求预测的 CVM 机型列表」。

## Capabilities

### New Capabilities

- `ziyan-device-type-sync`: 自研云（TCloudZiyan）机型同步流程的数据源与可用区归属规范，覆盖机型列表数据源切换、可用区中文名过滤、机型详情查询保留、入库字段口径等行为。

### Modified Capabilities

<!-- 无：crp-device-type-sync 是 woa-server 机型族映射同步能力，与本次自研云机型同步为不同调用链；新增的可选参数向后兼容，不改变其既有行为。 -->

## Impact

- **代码**：
  - `pkg/thirdparty/cvmapi/cvmapi_request.go`：`QueryCvmTypeListParams` 新增 3 个可选字段。
  - `pkg/thirdparty/cvmapi/cvmapi.go`：`QueryCvmTypeList` 接口/实现方法注释修正。
  - `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go`：`listZones` 返回类型改为 `corezone.BaseZone`（同时持有编码 `Name` 与中文名 `NameCn`）；`listDeviceTypeFromCloud` 切换数据源为 `QueryCvmTypeList`。
- **外部依赖**：依赖 CRP `queryCvmTypeList` 接口可用且数据完整。
- **数据**：上线后首次同步因 `cloud_id` 组成不变（zone 仍存编码），不产生历史数据翻动。
- **文档**：`docs/reqs/CRP机型接口对接.md` 中 R-002 / AC-002 / 澄清记录由「deptName 传空」更正为「deptName 传 IEG技术运营部」。
