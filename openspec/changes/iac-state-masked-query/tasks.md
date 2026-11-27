# Tasks

## 1. 数据库表、枚举常量与 DAL 层（`iac_repository` / `iac_spec` / `iac_spec_node` / `iac_state`）

- [x] 1.1 新增 `scripts/sql/9999_20261126_iac_repository_spec_state.sql`（含 `iac_repository`、`iac_spec`、`iac_spec_node`、`iac_state` 建表、全部唯一键/普通索引与 `id_generator` 初始化记录），校验 SQL 语法及表结构字段与 `design.md` 一致
- [x] 1.2 在 `pkg/criteria/enumor/` 新增 `IacSpecStatus`（`uploading`/`uploaded`/`upload_failed`）、`IacSpecNodeType`（`dir`/`file`）与 `IacStateKind`（`resource`/`data`）枚举及 `Validate()`，在 `pkg/criteria/constant/` 新增脱敏占位符 `"(sensitive value)"` 与 `import.` 前缀常量，运行对应包 `go test` 验证通过
- [x] 1.3 在 `pkg/dal/table/types/types.go` 为 `JsonField` 补充只读方法 `IsNull()`，在 `pkg/dal/table/table.go` 注册 `IacRepositoryTable`、`IacSpecTable`、`IacSpecNodeTable`、`IacStateTable`（均启用 `EnableTenant: true`）并在 `pkg/dal/table/iac/` 实现四张表模型、列定义及 `InsertValidate`/`UpdateValidate`，编写表模型单元测试并运行 `go test ./pkg/dal/table/...` 通过
- [x] 1.4 在 `pkg/dal/dao/iac/` 实现 `IacRepository`、`IacSpec`、`IacSpecNode`、`IacState` 四个 DAO（均含 `BatchCreateWithTx`、`UpdateWithTx`、`DeleteWithTx`、`List`，其中 `IacState.UpdateWithTx` 在 `model.Current.IsNull()` 时将 `toUpdate["current"] = nil` 写入 SQL `NULL`，并通过 `AddBlankedFields("import_from_spec_id")` 支持清空暂存标记）并注册到 `dao.Set`，运行 `go test ./pkg/dal/dao/...` 验证通过

## 2. `data-service` 批量接口与客户端

- [x] 2.1 在 `pkg/api/core/iac/` 与 `pkg/api/data-service/iac/` 定义 `iac_repository`、`iac_spec`、`iac_spec_node`、`iac_state` 核心 DTO（`coreiac.IacState.Current`、`IacSpecNode.ChildrenNodeIDs`/`Content` 使用指针保持 SQL `NULL` 语义）及批量创建/更新/删除/列表请求与响应结构体（含 `Validate()` 与 `constant.BatchOperationMaxLimit` 校验），运行对应 API 包 `go test` 验证通过
- [x] 2.2 在 `cmd/data-service/service/iac/` 注册四组单数路由批量接口（`/iac_repository/...`、`/iac_spec/...`、`/iac_spec_node/...`、`/iac_state/...`）并挂入 `data-service` 路由初始化，编写单元测试覆盖批量创建、更新（含 `Current: "null"` 置 SQL `NULL`、`iac_repository` 不改 `bk_biz_id`）、删除与 `fields` 投影列表查询，运行 `go test ./cmd/data-service/...` 验证通过
- [x] 2.3 在 `pkg/client/data-service/global/` 新增 `IacRepositoryClient`、`IacSpecClient`、`IacSpecNodeClient`、`IacStateClient` 并挂载到 `global.Client`，运行 `go build ./pkg/client/data-service/...` 验证编译通过

## 3. `pkg/iacstate` 无状态状态管理器

- [x] 3.1 在 `pkg/iacstate/` 定义 `StateRecord`（含 `CurrentOrNil`）、`StateObject`、`UpsertAppliedReq`、`RemoveDestroyedReq` 及无状态结构体 `StateManager` 与 `NewStateManager`，确保 JSON 序列化字段与设计契约一致并通过 `go build ./pkg/iacstate/...`
- [x] 3.2 实现 `StateManager` 的 `GetRepositoryStates` 与 `GetDependencyStates`（自动分页拉取、默认过滤 `import_from_spec_id == ""`、无匹配返回 `nil`、依赖缺失或 `Current == nil` 返回含缺失 `block_id` 的错误），编写单元测试覆盖全量/定向查询、暂存态隔离与依赖缺失场景并运行 `go test -v ./pkg/iacstate/...` 通过
- [x] 3.3 实现 `StateManager` 的 `UpsertAppliedState` 与 `RemoveDestroyedState`（覆盖常规写回、先建后删 `DeposedKey` 轮转、`import.` 前缀剥离并置 `kind="resource"`、半成品失败写回、按 `DeposedKey` 精准清理、`current` 与 `deposed` 均空删整行及 `Deposed` 非空时传 `Current: types.JsonField("null")` 置 `current=NULL`），编写完整状态机单元测试并运行 `go test -race -v ./pkg/iacstate/...` 通过

## 4. `hc-service` Schema 查询接口与 `iac-server` 只读脱敏查询接口

- [x] 4.1 在 `pkg/api/hc-service/`、`cmd/hc-service/service/` 与 `pkg/client/hc-service/` 基于既有 `SchemaManager.GetSchema` 暴露并封装插件 Schema 内部查询接口及客户端，编写接口单元测试并运行 `go test ./cmd/hc-service/... ./pkg/client/hc-service/...` 通过
- [x] 4.2 在 `pkg/api/iac-server/` 定义 `ListState` 与 `GetState` 请求与响应结构体（`GetState` 实例状态响应不包含 `private` 字段，`extra` 原样保留），运行 `go build ./pkg/api/iac-server/...` 验证编译通过
- [x] 4.3 在 `cmd/iac-server/logics/` 实现插件 Schema 定位（先查 `iac_plugin` `enabled` 版本再调用 `hc-service` 获取 Schema，不做本地缓存）以及基于 `tfjson.SchemaBlock` 的递归敏感属性脱敏器（覆盖顶层属性、`nested_type`、`block_types` 与 `resource`/`data` 双模式，Schema 不可用时 Fail-Closed），编写单元测试并运行 `go test -race -v ./cmd/iac-server/logics/...` 通过
- [x] 4.4 在 `cmd/iac-server/service/` 实现 `ListState`（`POST /api/v1/iac/repository/{repository_id}/state/list`）与 `GetState`（`GET /api/v1/iac/repository/{repository_id}/state/{state_id}`）Handler 并注册至 `/api/v1/iac` 路由（含 `iac_repository` 存在性校验），补充 Handler 单元测试（覆盖仓库不存在 404、摘要字段提取、`current == nil` 降级取 `deposed`、暂存态与跨仓库/租户隔离、脱敏与 Fail-Closed 拦截），运行 `go test -race -v ./cmd/iac-server/...` 通过

## 5. 联调构建与规范校验

- [x] 5.1 运行 `go build ./cmd/data-service/... ./cmd/hc-service/... ./cmd/iac-server/... ./pkg/iacstate/...` 与 `go vet ./cmd/data-service/... ./cmd/hc-service/... ./cmd/iac-server/... ./pkg/iacstate/...` 确认全链路编译与静态检查零报错
- [ ] 5.2 在单仓库 2000 条 `iac_state` 记录的测试环境下对 `ListState` 与 `GetState` 执行 50 并发压测 60 秒，验证 P99 响应时间均低于 500ms 且错误率为 0%
