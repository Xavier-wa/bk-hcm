## 1. 枚举扩展

- [x] 1.1 在 `pkg/criteria/enumor/global_config.go` 中新增 `GlobalConfigTypeCvmApply GlobalConfigType = "cvm_apply"` 枚举值
- [x] 1.2 在同文件中新增 `GlobalConfigKeyCvmApply` 类型及 `GlobalConfigKeyCvmApplyLoadTestSubnet = "load_test_subnet"` 常量

## 2. 类型定义

- [x] 2.1 新建 `cmd/woa-server/types/config/load_test_subnet.go`，定义 `LoadTestSubnetConfig = map[string]map[string][]string`
- [x] 2.2 在同文件中定义 `UpsertLoadTestSubnetReq`，实现 `Validate()` 方法（校验 JSON 格式，不校验子网 ID 有效性）

## 3. 业务逻辑层

- [x] 3.1 新建 `cmd/woa-server/logics/config/load_test_subnet.go`，定义 `LoadTestSubnetIf` 接口（含 `UpsertConfig` 和 `ListConfig` 方法）
- [x] 3.2 在同文件中实现 `loadTestSubnet` struct，复用 `client.DataService().Global.GlobalConfig` 的 List/BatchCreate/BatchUpdate 接口
- [x] 3.3 修改 `cmd/woa-server/logics/config/logics.go`，在 `Logics` 接口和 `logics` struct 中注册 `LoadTestSubnet() LoadTestSubnetIf`，在 `New()` 函数中初始化

## 4. HTTP Handler 层

- [x] 4.1 新建 `cmd/woa-server/service/config/load_test_subnet.go`，实现 `ListLoadTestSubnets` handler（无鉴权，直接调用 logics）
- [x] 4.2 在同文件中实现 `UpsertLoadTestSubnets` handler（鉴权：`meta.GlobalConfig + meta.Create`，decode→validate→auth→logics）
- [x] 4.3 修改 `cmd/woa-server/service/config/service.go`，新增 `initLoadTestSubnet(h)` 方法，注册以下两条路由：
  - `GET /config/load_test_subnets` → `ListLoadTestSubnets`
  - `POST /config/load_test_subnets/upsert` → `UpsertLoadTestSubnets`
- [x] 4.4 在 `service.go` 的 `InitService` 函数中调用 `s.initLoadTestSubnet(h)`

## 5. 接口文档

- [x] 5.1 新建 `docs/api-docs/web-server/docs/scr/config-manage/list_load_test_subnets.md`，按现有文档格式编写 GET 接口文档（版本 v9.9.9+，无需权限）
- [x] 5.2 新建 `docs/api-docs/web-server/docs/scr/config-manage/upsert_load_test_subnets.md`，按现有文档格式编写 POST 接口文档（版本 v9.9.9+，需 GlobalConfig.Create 权限）
