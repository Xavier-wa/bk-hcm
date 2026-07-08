# ziyan-device-type-sync Specification

## Purpose

定义自研云（TCloudZiyan）机型同步能力规范。系统通过 CRP `QueryCvmTypeList` 接口按可用区获取「可填报需求预测」的机型列表，再由 `QueryCvmInstanceType` 补全机型详情，组装并入库到 `device_type` 表，作为自研云机型数据的权威来源。

## Requirements

### Requirement: 机型列表数据源

自研云（TCloudZiyan）机型同步 SHALL 使用 CRP `QueryCvmTypeList` 接口获取「可填报需求预测」的机型列表，不再使用 `GetInstanceTypeInfo`。

#### Scenario: 使用 QueryCvmTypeList 获取机型列表

- **GIVEN** 自研云机型同步任务已触发（vendor=tcloud_ziyan，指定 region）
- **WHEN** 同步流程按可用区获取机型列表
- **THEN** 系统 SHALL 调用 `QueryCvmTypeList` 并从响应 `result`（`[]CvmTypeItem`）中提取 `cvmInstanceModel` 作为机型规格名称列表，不再调用 `GetInstanceTypeInfo`

#### Scenario: 机型列表为空跳过可用区

- **WHEN** 某可用区调用 `QueryCvmTypeList` 后收集到的机型规格名称列表为空
- **THEN** 系统 SHALL 跳过该可用区、不写入空数据，并继续处理其他可用区，不阻断整体同步

#### Scenario: 接口调用失败终止同步

- **WHEN** `QueryCvmTypeList` 调用出现 HTTP 错误或返回业务错误码（`error.code` 非 0）
- **THEN** 系统 SHALL 记录 `logs.Errorf` 告警并立即返回错误、终止本次同步

### Requirement: 可用区中文名过滤与编码归属

自研云机型同步 SHALL 按可用区循环调用 `QueryCvmTypeList`，以可用区**中文名**作为 `zoneNames` 过滤条件（CRP 接口口径），而 `device_type` 表 `zone` 字段 SHALL 继续存储可用区**编码**，保持入库口径不变。

#### Scenario: zoneNames 传入可用区中文名

- **GIVEN** 某地域存在多个可用区，每个可用区有编码（如 `ap-shanghai-1`）和中文名（如 `上海一区`）
- **WHEN** 同步流程按可用区循环调用 `QueryCvmTypeList`
- **THEN** 系统 SHALL 在 `zoneNames` 参数中传入当前可用区的中文名，实现机型按可用区维度过滤

#### Scenario: device_type 入库归属可用区编码

- **WHEN** 某可用区查询到的机型组装为 `device_type` 记录并入库
- **THEN** 系统 SHALL 将该记录的 `zone` 字段设置为当前可用区的**编码**（与 zone 表 `name` 字段一致），使 `cloud_id`（region+zone+device_type）组成口径保持不变

#### Scenario: 各机型正确归属所属可用区

- **GIVEN** 某地域存在多个可用区
- **WHEN** 完成同步
- **THEN** 各机型 SHALL 正确归属到其所属可用区，`device_type` 表 region+zone 维度数据完整

### Requirement: 部门过滤参数

自研云机型同步调用 `QueryCvmTypeList` 时 SHALL 传入部门名称 `deptName=CvmLaunchDeptName`（"IEG技术运营部"）。

#### Scenario: deptName 传入 IEG 技术运营部

- **WHEN** 同步流程调用 `QueryCvmTypeList`
- **THEN** 系统 SHALL 在 `deptName` 参数中传入常量 `CvmLaunchDeptName`（"IEG技术运营部"）

### Requirement: 机型详情查询保留

机型的详细规格信息 SHALL 继续由 CRP `QueryCvmInstanceType` 接口提供，该调用逻辑及字段映射保持不变，确保 `device_type` 各字段不缺失。

#### Scenario: 保留 QueryCvmInstanceType 查询详情

- **WHEN** 收集到机型规格名称列表后需要查询机型详细信息
- **THEN** 系统 SHALL 调用 `QueryCvmInstanceType`（入参为机型规格名称列表）获取详情，并按既有字段映射填充 `device_type` 的 CPU/内存/GPU/核心类型/机型代际等字段

#### Scenario: 详情查询失败终止同步

- **WHEN** `QueryCvmInstanceType` 调用出现 HTTP 错误或返回业务错误码（`error.code` 非 0）
- **THEN** 系统 SHALL 记录 `logs.Errorf` 告警并立即返回错误、终止本次同步

### Requirement: QueryCvmTypeList 可选过滤参数扩展

CRP 客户端 `QueryCvmTypeListParams` SHALL 新增 `regionNames` / `cityNames` / `zoneNames` 三个可选字段，向后兼容，不影响既有调用方。

#### Scenario: 新增可选过滤字段

- **WHEN** 构造 `QueryCvmTypeListParams`
- **THEN** 结构体 SHALL 提供 `RegionNames []string`（`json:"regionNames,omitempty"`）、`CityNames []string`（`json:"cityNames,omitempty"`）、`ZoneNames []string`（`json:"zoneNames,omitempty"`）三个可选字段

#### Scenario: 既有调用方不受影响

- **WHEN** 既有调用方（如 woa-server 机型族映射同步）以 `&QueryCvmTypeListParams{}` 调用且不设置新增字段
- **THEN** 新增字段因 `omitempty` 不出现在请求体中，CRP 调用行为 SHALL 与扩展前保持一致
