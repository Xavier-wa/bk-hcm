## 1. CRP 客户端参数扩展与注释修正

- [x] 1.1 在 `pkg/thirdparty/cvmapi/cvmapi_request.go` 的 `QueryCvmTypeListParams` 新增 `RegionNames []string`（`json:"regionNames,omitempty"`）、`CityNames []string`（`json:"cityNames,omitempty"`）、`ZoneNames []string`（`json:"zoneNames,omitempty"`）三个可选字段，并为每个字段补充「选填」中文注释
- [x] 1.2 修正 `QueryCvmTypeListReq` / `QueryCvmTypeListParams` 结构体注释为「查询「可填报需求预测」的CVM机型列表（含物理机机型族等映射信息）」相关口径
- [x] 1.3 修正 `pkg/thirdparty/cvmapi/cvmapi.go` 中 `CVMClientInterface` 接口定义处 `QueryCvmTypeList` 方法注释
- [x] 1.4 修正 `pkg/thirdparty/cvmapi/cvmapi.go` 中 `cvmApi.QueryCvmTypeList` 实现处方法注释

## 2. 自研云机型同步数据源切换

- [x] 2.1 在 `cmd/hc-service/service/sync/tcloud-ziyan/device_type.go` 将 `listZones` 返回值由 `[]string` 改为 `[]corezone.BaseZone`（复用 `hcm/pkg/api/core/cloud/zone` 已有类型，同时持有编码 `Name` 与中文名 `NameCn`），直接 `append(zones, result.Details...)`
- [x] 2.2 调整 `Next` 中对 `listZones` 返回值的传递，使 `listDeviceTypeFromCloud` 接收 `[]corezone.BaseZone`
- [x] 2.3 在 `listDeviceTypeFromCloud` 中将每个可用区的机型列表查询由 `GetInstanceTypeInfo` 替换为 `QueryCvmTypeList`，参数 `DeptName=cvmapi.CvmLaunchDeptName`、`ZoneNames=[]string{zone.NameCn}`
- [x] 2.4 从 `QueryCvmTypeList` 响应 `Result`（`[]CvmTypeItem`）中收集非空 `CvmInstanceModel` 作为机型规格名称列表；列表为空则 `continue` 跳过该可用区
- [x] 2.5 保留 `QueryCvmInstanceType` 详情查询逻辑及字段映射不变；组装 `device_type` 时 `Zone` 字段使用可用区编码（`zone.Name`）
- [x] 2.6 校验错误处理：`QueryCvmTypeList` HTTP/业务错误码失败时 `logs.Errorf` 并返回 error 终止同步（含 rid）

## 3. 文档更正

- [x] 3.1 更正 `docs/reqs/CRP机型接口对接.md` 中 R-002、AC-002、澄清记录第 1 轮 Q2 答复：由「deptName 传空」改为「deptName 传 IEG技术运营部（CvmLaunchDeptName）」
- [x] 3.2 在 `docs/reqs/CRP机型接口对接.md` 末尾补充「实现位置」章节，列出新增/修改文件

## 4. 自测与校验

- [x] 4.1 `go build ./...` 编译通过，`goimports -w` 格式化相关文件
- [x] 4.2 确认既有调用方（woa-server `sync_device_type_physical_rel`）以 `&QueryCvmTypeListParams{}` 调用不受新增字段影响（编译 + 行为不变）
- [ ] 4.3 联调验证：自研云机型同步后对比 HCM `device_type` 与 CRP 系统展示一致，无机型缺失（AC-001~AC-005）— 运行时验证
- [x] 4.4 运行 `openspec validate switch-ziyan-device-type-to-querycvmtypelist --strict` 校验通过
