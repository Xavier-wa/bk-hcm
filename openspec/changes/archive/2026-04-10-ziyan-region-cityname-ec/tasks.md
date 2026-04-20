## 1. CRP 接口封装

- [ ] 1.1 在 `pkg/thirdparty/cvmapi/constvar.go` 中添加 `CvmQueryZoneCityListMethod = "queryZoneCityList"` 方法常量
- [ ] 1.2 在 `pkg/thirdparty/cvmapi/cvmapi_request.go` 中添加 `QueryZoneCityListReq` 和 `QueryZoneCityListParams` 请求结构体
- [ ] 1.3 在 `pkg/thirdparty/cvmapi/cvmapi_response.go` 中添加 `QueryZoneCityListResp` 响应结构体和 `ZoneCityInfo` 数据模型
- [ ] 1.4 在 `pkg/thirdparty/cvmapi/cvmapi.go` 中实现 `QueryZoneCityList` 方法（路径 `/yunti-demand/external`）
- [ ] 1.5 在 `pkg/thirdparty/cvmapi/cvmapi.go` 中添加 `CVMClientInterface` 接口方法声明
- [ ] 1.6 在 `pkg/thirdparty/cvmapi/constvar.go` 中添加 `NewQueryZoneCityListReq` 请求构造函数

## 2. CRP 客户端依赖传递链改造

- [ ] 2.1 修改 `cmd/hc-service/service/service.go:191`，将 `s.crpCli` 传递给 `ressync.NewClient`
- [ ] 2.2 修改 `cmd/hc-service/logics/res-sync/client.go`，在 `client` 结构体添加 `crpCli` 字段，更新 `NewClient` 函数签名接收 `crpCli`
- [ ] 2.3 修改 `cmd/hc-service/logics/res-sync/client.go` 的 `TCloudZiyan` 方法，将 `crpCli` 传递给 `ziyan.NewClient`
- [ ] 2.4 修改 `cmd/hc-service/logics/res-sync/ziyan/client.go`，在 `client` 结构体添加 `crpCli` 字段，更新 `NewClient` 函数签名接收 `crpCli`

## 3. 地域同步逻辑修改

- [ ] 3.1 在 `cmd/hc-service/logics/res-sync/ziyan/region.go` 中新增 `getRegionCityMapFromCRP` 方法，一次性调用 CRP 接口获取全量 region-city 映射，构建 `map[regionID]cityName`
- [ ] 3.2 修改 `Region()` 方法入口，在 diff 之前调用 `getRegionCityMapFromCRP` 获取映射 map
- [ ] 3.3 修改 `createRegion` 函数，新增 `regionCityMap` 参数，优先使用 CRP 返回的 city_name，失败时回退到 `extractAreaAndCityName`
- [ ] 3.4 修改 `updateRegion` 函数，新增 `regionCityMap` 参数，优先使用 CRP 返回的 city_name，失败时回退到 `extractAreaAndCityName`
- [ ] 3.5 修改 `isRegionChange` 函数，新增 `regionCityMap` 参数，使用 CRP 返回的 city_name 进行对比（避免因 city_name 来源不同导致每次同步都触发 update）
- [ ] 3.6 添加 CRP 接口调用失败时的错误日志记录和降级处理逻辑（map 为 nil 时所有函数自动回退）

## 4. 代码审查与测试

- [ ] 4.1 运行单元测试 `go test ./pkg/thirdparty/cvmapi/... -v` 确保现有测试通过
- [ ] 4.2 运行单元测试 `go test ./cmd/hc-service/logics/res-sync/ziyan/... -v` 确保现有测试通过
- [ ] 4.3 代码审查确保错误处理符合规范（无 `_` 忽略错误）
- [ ] 4.4 检查代码注释是否符合中文注释规范

## 5. 集成验证

- [ ] 5.1 在测试环境部署并触发地域同步任务
- [ ] 5.2 验证数据库中自研云地域数据的 city_name 字段已包含 EC 后缀
- [ ] 5.3 验证 CRP 接口调用失败时回退逻辑正常工作
- [ ] 5.4 检查日志中是否有 CRP 接口调用相关的错误/警告信息
