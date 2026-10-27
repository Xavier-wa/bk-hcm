## Why

在海垒（HCM）IaC 插件化改造架构中，控制面（`cloud-server`、`iac-server`、`task-server`）负责解析用户配置、构建并调度 DAG，而 `hc-service` 负责执行单个节点。不同插件的启动参数截然不同（如云插件需 `secret_id`/`secret_key`，CMDB 三方插件需 `access_token`/`bk_tenant_id`），且北向调用链路严禁传递明文密钥或 `launch_config`，控制面仅传递插件名（`plugin`）与插件实例 ID（`plugin_instance_id`）。

目前仓库已落地插件注册表（`iac_plugin`）、插件 Schema 管理（`iac_plugin_schema` 与 `SchemaManager`）、插件实例管理与 `attrs`/`secret_attrs` 同构拆分合并（`cmd/hc-service/logics/plugin/instance` 与 `data-service` 加解密）以及南向 HTTP 客户端与 RSA-OAEP 加密组件（`pkg/thirdparty/api-gateway/plugin`），但缺少统一的 **`hc-service` 北向协议 HTTP 入口（`Northbound`）**、**插件与实例路由器（`Router`）**、**漂移检测与支持多层嵌套的 `proposed_state` 合成引擎（`StateComposer`）** 以及 **三方 HTTP 插件（`http-plugin`）路由执行闭环**，导致上游控制面无法通过统一的 `(plugin, plugin_instance_id, type_name)` 契约寻址插件、解析实例凭证并驱动三方插件运行时。

## What Changes

### 本期包含（Scope In）

1. **北向协议 11 个 HTTP 接口契约、公共模型/枚举下沉、Service Handler 与 Client SDK（对应 Wave 1 子需求 `1069995598139293292`）**：
   - 在 `pkg/criteria/enumor/iac_plugin.go` 定义北向操作类型枚举 `IaCPluginOperationType` 与任务状态枚举 `IaCPluginTaskStatus`；在 `pkg/api/core/plugin/iac_plugin.go` 下沉公共运行时模型（`PathStep`、`Diagnostic`、`HealthData`、`Progress`、`ListItem`、`ListData`、`MinListLimit`/`MaxListLimit`），并在 `pkg/thirdparty/api-gateway/plugin/types.go` 通过类型别名复用，消除北向 API 对 `thirdparty` 包的反向依赖；
   - 在 `pkg/api/hc-service/iac-plugin/northbound.go` 定义统一挂载于 `/api/v1/hc/iac_plugins/{health|schema|execute|result|stop|plan|validate|import|refresh|read_data|list}` 的 11 个北向接口请求/响应模型与前置参数校验（`Validate()`）；
   - 在 `pkg/client/hc-service/iac-plugin/client.go` 扩展北向 11 个接口的类型安全 Client SDK 方法；
   - 在 `cmd/hc-service/service/iac-plugin` 注册 11 个北向 HTTP 路由与 Handler 解码/校验层，并通过 `capability.Capability` 注入 `PluginRouter *router.Service`。
2. **`hc-service` 插件路由器（`Router`）、实例归属强校验与无实例接口实现（对应 Wave 2 子需求 `1069995598139293636`）**：
   - 在 `cmd/hc-service/logics/iac-plugin/router` 按 `plugin` 查询 `iac_plugin` 表并通过 `iacplugin.Latest(records)` 校验最高版本记录处于 `state == "enabled"`；
   - 调用 `SchemaManager.GetSchema` 获取活跃 Schema，校验 `type_name` 存在性（区分 `resource_schemas` 与 `data_source_schemas`）及当前操作对应的 `capabilities` 能力位（`apply`、`async_apply`、`stop && async_apply`、`plan`、`validate`、`import`、`refresh`、`read_data`、`list`）；
   - 复用 `plugininst.PluginInstanceManager.GetPluginInstance` 读取实例并将普通属性树 `attrs` 与已解密敏感属性树 `secret_attrs` 同构深度合并还原为明文 `Properties`，同时强校验 `inst.PluginName == plugin`（严禁跨插件混用实例套取凭证）；
   - 实现北向无实例接口 `GetSchema`（只读进程内 `SchemaCache` 与 `data-service` `iac_plugin_schema` 表，支持可选 `plugin_version`，触发南向 HTTP `/schema` 次数为 0）与 `Health`（调用 `httpPlugin.Health` 南向探活返回 `plugin_version`）；
   - 按 `protocol` 将 `http` 协议插件的运行态请求连同 `*RouteContext` 分发至 `*httpplugin.Service`（对暂未对接的 `tf_provider` 协议返回 `errf.Aborted`）。
3. **漂移检测（`Refresh`）与对照 Schema 递归合成 `proposed_state` 引擎（对应 Wave 2 子需求 `1069995598139293839`）**：
   - 在 `cmd/hc-service/logics/iac-plugin/http-plugin/executor.go` 的 `Execute` 与 `Plan` 中，先通过 `ValidateConfigAndRecompute` 静态校验用户 `config` 与 `recompute`；随后当 `prior_state != nil` 且插件支持 `refresh` 时调用南向 `Refresh` 获取外部最新状态 `refreshed_state`：若 `refreshed_state == nil` 且为删除（`config == nil`）则短路直接返回成功；若 `refreshed_state == nil` 且为更新（`config != nil`）则将 `baselinePrior` 置 `nil` 自动转为创建流程；若 `refreshed_state != nil` 则以其作为后续基准 `prior_state`；
   - 在 `cmd/hc-service/logics/iac-plugin/http-plugin/composer.go` 对照资源 `*tfjson.SchemaBlock`（含顶层 `Attributes`、SDKv2 `NestedBlocks` 及 Plugin Framework `AttributeNestedType`）递归校验并合成 `proposed_state`：拦截任意层级未知字段、非 `nil` 的纯 `computed` 属性、容器类型不匹配以及 `recompute` 中未知或非 `optional + computed` 的属性路径；支持 `recompute: []string` 传入顶层属性、Single/Group 子属性、List/Set 精确下标（如 `data_disks[0].kms_key_id`）或无下标通配路径（如 `data_disks.kms_key_id`）、Map 动态 key 路径（如 `labels_map.env.tag_id`）或省略动态 key 通配路径（如 `labels_map.tag_id`），在更新时将未指定/为 `nil` 的 `optional + computed` 属性显式置为 `nil` 触发重算，未列入 `recompute` 时默认沿用刷新后的 `prior_state` 对应值（其中 List 嵌套块按下标对齐保留已有元素属性、新增元素按创建规则置 `nil`）。
4. **三方 HTTP 插件（`http-plugin`）完整加密执行链路（对应 Wave 2 子需求 `1069995598139293839`）**：
   - 在 `cmd/hc-service/logics/iac-plugin/http-plugin/executor.go` 从 `iac_plugin.extension` 解析 `HTTPExtension`（`api_path`、`crypto_type`、`crypto_config`），通过 `encryptorMap sync.Map` 缓存复用 `plugin.LaunchConfigEncryptor`，将实例明文 `properties` 经 3072 位 RSA-OAEP（SHA-256）加密为 Base64 `launch_config`（前置拦截超出 `keyBytes - 66` 明文上限）；
   - 复用 `pluginCliMap sync.Map` 并发安全缓存南向 `plugin.Client`，完成全部 9 个运行态南向接口调用与北向响应映射，严格分离外层网关通信错误（`code=2000006, data=null`）与内部任务失败/配置诊断（`code=0, data.status="failed"` 回传漂移或部分落地 `state`，`Validate` 回传 `data.diagnostics`）。
5. **完整的单元测试与性能基准测试覆盖**：
   - 覆盖 11 个接口参数校验与零敏感字段反射测试、Client SDK `httptest` 契约测试、`Router` 状态/Schema/Capabilities/跨插件归属门控测试、漂移检测短路/转创建、`proposed_state` 顶层与多层嵌套块合成及 `recompute` 正反例、三方插件全链路 Mock 测试、50 协程 `-race` 并发安全测试、敏感凭证零日志泄露测试（`SK_SUPER_SECRET_12345` 出现次数为 0）及 `BenchmarkNorthboundRouteAndEncrypt`（1,000 次路由+同构合并+RSA 加密 P99 < 10ms）。

### 本期不包含（Scope Out）

1. **`InternalPlugin` 本地 `tf_provider` 插件通道对接与执行**：包括官方 Terraform Provider 的 `go-plugin` 进程拉起、握手、日志落盘、`DynamicValue` msgpack 编解码、进程池 FIFO 调度及 `InternalPlugin` 本地 gRPC 协议调用（由兄弟需求 `1069995598138774196` 负责）；
2. **HCL 语法解析与 DAG 编排调度**：由 `iac-server` 与 `task-server` 负责；
3. **三方插件的注册、升级、修改、启停与后台巡检**：已由需求 `1069995598138774194` 实现并合入；
4. **南向 HTTP 客户端底层通信与 RSA-OAEP 基础加密库**：已由需求 `1069995598138774184` 实现并合入 `pkg/thirdparty/api-gateway/plugin`；
5. **插件 Schema 同步入库与 `SchemaCache` 底层实现**：已由需求 `1069995598139220814` 实现并合入 `cmd/hc-service/logics/plugin/schema`；
6. **`hc-service` 插件实例管理 CRUD 与属性拆分合并底层实现**：已落位于 `cmd/hc-service/logics/plugin/instance`，本变更在 `Router` 中直接复用 `PluginInstanceManager.GetPluginInstance`。

## Capabilities

### New Capabilities
- `plugin-northbound-contract-sdk`: 北向协议 11 个标准 HTTP 接口（`/api/v1/hc/iac_plugins/{health|schema|execute|result|stop|plan|validate|import|refresh|read_data|list}`）的请求/响应数据模型、前置参数校验、`pkg/criteria/enumor` 与 `pkg/api/core/plugin` 公共枚举/模型下沉、`pkg/client/hc-service/iac-plugin` Client SDK 以及 `cmd/hc-service/service/iac-plugin` 路由绑定。
- `plugin-instance-router`: `hc-service` 插件与实例路由器（`cmd/hc-service/logics/iac-plugin/router`），提供插件最高版本 `enabled` 状态校验、活跃 Schema 类型（区分资源与数据源）与 9 项 `capabilities` 能力位门控、基于 `PluginInstanceManager.GetPluginInstance` 的实例读取（含 `attrs` 与已解密 `secret_attrs` 同构树深度合并）与跨插件归属强校验、北向无实例接口 `GetSchema`（零南向调用）与 `Health`（南向探活），以及按协议路由分发。
- `plugin-state-composer-thirdparty`: `hc-service` 在 `cmd/hc-service/logics/iac-plugin/http-plugin` 中实现的静态配置前置校验、`Refresh` 漂移检测状态机（含外部已删除时的删除短路与更新自动转重建）、对照 `*tfjson.SchemaBlock`（含顶层属性、`NestedBlocks` 与 `AttributeNestedType`）的递归 `proposed_state` 状态合成引擎（支持 4 类属性规则与多层级/通配路径 `recompute: []string` 显式置 `null` 重算），以及三方 HTTP 插件 RSA-OAEP `launch_config` 加密执行通道与外层/内层状态分离映射。

### Modified Capabilities

## Impact

- `pkg/criteria/enumor/iac_plugin.go` & `pkg/api/core/plugin/iac_plugin.go` & `pkg/thirdparty/api-gateway/plugin/types.go` — 下沉 `IaCPluginOperationType`、`IaCPluginTaskStatus` 枚举与 `Diagnostic`、`PathStep`、`Progress`、`HealthData`、`ListData` 等核心运行时模型
- `pkg/api/hc-service/iac-plugin/` — 新增北向 11 个接口的请求/响应结构体、参数校验及单元测试（`northbound.go`、`northbound_test.go`）
- `pkg/client/hc-service/iac-plugin/` — 扩展北向 11 个接口的 Client SDK 方法及单元测试（`client.go`、`client_test.go`）
- `cmd/hc-service/logics/iac-plugin/router/` — 新增 `RouteContext`、插件状态与 Schema/Capabilities 门控、复用 `PluginInstanceManager` 的实例读取与归属强校验、`GetSchema`/`Health` 及 9 个运行态接口路由分发与单元测试（`types.go`、`router.go`、`router_test.go`）
- `cmd/hc-service/logics/iac-plugin/http-plugin/` — 扩展前置 `Refresh` 漂移检测、支持多层嵌套块/嵌套属性的 `ComposeProposedState` 状态合成引擎（含路径级 `recompute` 控制）、RSA-OAEP `launch_config` 加密缓存与南向 9 个运行态接口 + `Health` 调用映射、并发/零泄露测试及性能 Benchmark（`service.go`、`composer.go`、`executor.go`、`executor_test.go`、`benchmark_test.go`）
- `cmd/hc-service/service/iac-plugin/` & `cmd/hc-service/service/capability/` & `cmd/hc-service/service/` — 初始化 `router.Service` 并注册北向 11 个 HTTP 路由 Handler 及单元测试
