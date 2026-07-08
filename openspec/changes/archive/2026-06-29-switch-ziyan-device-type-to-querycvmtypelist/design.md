## Context

自研云机型同步入口 `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` 当前在 `listDeviceTypeFromCloud` 中按可用区循环：先调 `GetInstanceTypeInfo`（DeptId + Zone 编码）拿机型规格列表，再调 `QueryCvmInstanceType` 补详情，最后组装 `device_type` 入库。`GetInstanceTypeInfo` 只返回「已填报需求预测」的机型，导致数据缺失。

CRP 新接口 `queryCvmTypeList` 的客户端方法 `QueryCvmTypeList` 已在归档变更 `2026-05-25-add-crp-device-type-sync` 中实现并落地（`pkg/thirdparty/cvmapi/`），返回「可填报需求预测」的全量机型，本次仅需切换数据源并补齐过滤参数。

约束：
- `device_type` 表 `zone` 字段参与 `cloud_id`（region+zone+device_type）拼装，被 `RemoveDeletedFromCloud` 用于云上比对，口径不可随意变更，否则首次同步会触发全量删除重建。
- `listZones` 当前返回 `zone.Name`（= 可用区编码），同时用于过滤参数与入库归属两处。

## Goals / Non-Goals

**Goals:**
- 自研云机型列表数据源切换为 `QueryCvmTypeList`，修复机型缺失。
- 按可用区中文名过滤（CRP 口径），机型详情查询逻辑保持不变。
- `device_type.zone` 入库口径保持编码不变，避免历史数据翻动。
- `QueryCvmTypeListParams` 补齐 region/city/zone 过滤能力，向后兼容。

**Non-Goals:**
- 不调整非自研云 vendor 的机型同步。
- 不变更 `device_type` 表结构与机型详情字段映射规则。
- 不删除 `GetInstanceTypeInfo` 客户端方法（保留通用 API 封装）。
- 不改动 woa-server `crp-device-type-sync`（机型族映射）同步链路。

## Decisions

### 决策 1：`listZones` 返回编码 + 中文名两份，而非只返回中文名

CRP `queryCvmTypeList` 的 `zoneNames` 过滤吃**中文名**（如 `上海一区`，见需求接口契约示例），但 `device_type.zone` 必须继续存**编码**以保持 `cloud_id` 稳定。因此 `listZones` 需同时提供两个值。

- 方案：`listZones` 返回 `[]corezone.BaseZone`（复用 `hcm/pkg/api/core/cloud/zone` 已有类型，其 `Name`=编码、`NameCn`=中文名两者都已存在），无需新增局部结构体。
- 循环内：`zoneNames` 传 `zone.NameCn`，组装 `device_type.Zone` 时传 `zone.Name`。
- **备选（已否决）**：① 只返回 `NameCn` 一份 → 会使 `device_type.zone` 变中文名、`cloud_id` 组成变化，首次同步全量删除重建，否决；② 新增局部结构体 `zoneInfo{Name, NameCn}` → `BaseZone` 已完整覆盖所需字段，重复定义无必要，否决。

### 决策 2：`deptName` 传 `CvmLaunchDeptName`

调用 `QueryCvmTypeList` 时 `deptName` 传常量 `CvmLaunchDeptName`（`pkg/thirdparty/cvmapi/constvar.go`，"IEG技术运营部"）。

- 这与早期需求文档「deptName 传空」相反，已与用户确认改为传部门名；需求文档 R-002 / AC-002 / 澄清记录同步更正。

### 决策 3：从 `QueryCvmTypeList` 响应收集 `CvmInstanceModel`

`QueryCvmTypeListResp.Result` 为 `[]CvmTypeItem`，其 `CvmInstanceModel`（实例规格）等价于旧 `InstanceTypeInfoItem.CvmInstanceModel`，作为 `QueryCvmInstanceType` 的入参列表。收集逻辑直接平移，过滤空字符串。

### 决策 4：注释修正

`QueryCvmTypeList` 接口/实现/参数结构体注释由「查询CVM机型与物理机机型族映射列表」改为「查询「可填报需求预测」的CVM机型列表（含物理机机型族等映射信息）」，更贴合接口本质（映射只是返回字段之一）。

## Risks / Trade-offs

- [CRP `zoneNames` 实际口径与中文名不一致] → 上线前与 CRP 联调验证 `上海一区` 形态可命中机型；若不符按实际口径调整 `listZones` 取值。
- [`deptName` 传部门名收窄机型范围，与"最大化覆盖"初衷相悖] → 已与用户确认按部门名传，联调验证机型数据与 CRP 系统展示一致（AC-004）。
- [新增可选字段影响既有调用方] → 三字段均 `omitempty`，既有 `&QueryCvmTypeListParams{}` 调用不变；通过既有调用方场景校验。
- [首次同步数据翻动] → 因 `zone` 仍存编码、`cloud_id` 组成不变，预期无翻动；切换前后机型集合差异属正常修复结果。

## Migration Plan

1. 扩展 `QueryCvmTypeListParams` 字段 + 注释修正（纯增量，向后兼容）。
2. 改造 `device_type.go`：`listZones` 双值返回 + `listDeviceTypeFromCloud` 切换数据源。
3. 同步更正需求文档。
4. 联调验证 AC-001~AC-005，重点对比 HCM `device_type` 与 CRP 系统展示一致性。
5. 回滚策略：改动集中在自研云同步单一链路，回退代码即恢复旧数据源（旧逻辑无破坏性变更）。

## Open Questions

- 无（Q-001/Q-002 口径问题已在方案中明确为：zoneNames 传中文名、deptName 传 IEG技术运营部，待联调最终确认）。
