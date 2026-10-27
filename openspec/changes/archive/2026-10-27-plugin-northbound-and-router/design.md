## Context

在海垒（HCM）IaC 插件化架构中，控制面服务（`cloud-server`、`iac-server`、`task-server`）负责解析用户配置并调度 DAG，而 `hc-service` 负责单节点插件执行。北向调用链路严禁传递任何明文凭证或 `launch_config`，控制面仅传递插件名（`plugin`）、插件实例 ID（`plugin_instance_id`）与类型名（`type_name`）。

当前代码库已落地的基础设施：
1. **插件注册表与公共规则**：`pkg/api/core/plugin/iac_plugin.go`、`pkg/iacplugin/iacplugin.go`（`Latest(records)`、`ParseVersion`）、`pkg/iacplugin/http.go`（`JoinPluginURL`、`ValidateAPIPath`、`ValidateCrypto`）以及 `cmd/hc-service/logics/iac-plugin/store/store.go`（`store.List`）。
2. **插件 Schema 管理与缓存**：`cmd/hc-service/logics/plugin/schema` 已提供 `SchemaManager.GetSchema(kt, coreplugin.PluginInfo)`，内置 `SchemaCache`（支持进程内懒加载、`singleflight` 防击穿、滑动 TTL 与 `schema_hash` 轻量比对）。
3. **插件实例管理与同构合并**：`cmd/hc-service/logics/plugin/instance` 已提供 `PluginInstanceManager.GetPluginInstance(kt, id)`，内部查询 `data-service` `IacPluginInstance`（其中 `SecretAttrs` 的标量叶子已由 `data-service` 自动 AES 解密），并通过 `mergeInstanceProperties(raw.Attrs, raw.SecretAttrs)` 将同构的普通属性树与敏感属性树深度合并还原为完整明文 `PluginInstance.Properties`。
4. **南向 HTTP 客户端与 RSA-OAEP 加密器**：`pkg/thirdparty/api-gateway/plugin` 已提供 `NewLaunchConfigEncryptor`（3072 位以上 RSA-OAEP with SHA-256 加密生成 Base64 `launch_config`）与 `plugin.NewClient`（11 个南向 HTTP 接口客户端）。
5. **`hc-service` HTTP 插件服务与路由挂载**：`cmd/hc-service/logics/iac-plugin/http-plugin` 已承载 HTTP 插件注册/升级/修改/巡检并维护 `pluginCliMap sync.Map`；`cmd/hc-service/service/service.go` 中 `ws.Path("/api/v1/hc")`，`cmd/hc-service/service/iac-plugin/service.go` 通过 `rest.NewHandler()` 注册 `/iac_plugins/...` 子路径，并在 `pkg/client/hc-service/client.go` 中以 `/api/v1/hc` 为前缀构造 `IaCPlugin` Client。

在此基础上，本变更补齐 `hc-service` 北向 11 个标准接口、公共运行时模型/枚举下沉、Client SDK、插件与实例路由器（`router.Service`）、支持多层嵌套块的漂移检测与 `proposed_state` 状态合成引擎以及三方 HTTP 插件加密执行通道（`httpplugin.Service`）。

## Goals / Non-Goals

**Goals:**
- **Wave 1（契约、公共模型下沉与 SDK 基线）**：在 `pkg/criteria/enumor/iac_plugin.go` 与 `pkg/api/core/plugin/iac_plugin.go` 下沉操作类型/任务状态枚举及公共运行时模型，在 `pkg/api/hc-service/iac-plugin` 与 `pkg/client/hc-service/iac-plugin` 中完整定义并暴露挂载于 `/api/v1/hc/iac_plugins/{health|schema|execute|result|stop|plan|validate|import|refresh|read_data|list}` 的 11 个北向接口 DTO、参数校验、Client SDK 方法及 HTTP Handler 路由绑定。
- **Wave 2A（插件与实例路由器门控）**：在 `cmd/hc-service/logics/iac-plugin/router` 实现 `router.Service`，完成 `iac_plugin` 最高版本 `enabled` 状态校验、活跃 `PluginSchema` 的 `type_name` 与 `capabilities` 能力位门控、复用 `PluginInstanceManager.GetPluginInstance` 完成实例同构合并与跨插件归属强校验、北向无实例接口 `GetSchema`（零南向调用）与 `Health`（南向探活）以及按 `protocol` 路由分发。
- **Wave 2B（多层嵌套状态合成引擎与三方插件加密执行通道）**：在 `cmd/hc-service/logics/iac-plugin/http-plugin` 实现 `Execute`/`Plan` 前置静态校验（`ValidateConfigAndRecompute`）、前置 `Refresh` 漂移检测状态机（外部已删除时的删除短路与更新转重建）、对照 `*tfjson.SchemaBlock`（支持顶层 `Attributes`、SDKv2 `NestedBlocks` 与 Plugin Framework `AttributeNestedType`）的 4 类属性递归 `proposed_state` 合成与多层级/通配路径 `recompute: []string` 显式置 `null` 重算引擎，以及 RSA-OAEP `launch_config` 加密器缓存、南向 `plugin.Client` 并发缓存复用与外层/内层状态分离映射。
- 满足性能与安全红线：路由 + 同构合并 + 3072 位 RSA-OAEP 加密 P99 < 10ms（1,000 次 Benchmark）、北向 `GetSchema` 南向 HTTP 调用 0 次、50 协程 `-race` 并发安全、全链路日志与错误消息明文凭证出现次数 0。

**Non-Goals:**
- 不包含 `InternalPlugin` 本地 `tf_provider` 插件通道对接与执行（`go-plugin` 进程管理、msgpack 编解码、gRPC 调用，由兄弟需求 `1069995598138774196` 负责）。
- 不包含 HCL 语法解析与 DAG 编排调度（由 `iac-server` / `task-server` 负责）。
- 不修改现有数据库表结构，不新增配置文件项。

## Architecture & Module Layout

```
Control Plane (cloud-server / iac-server / task-server)
   │  pkg/client/hc-service/iac-plugin.Client (11 methods, zero secret in payload)
   ▼
cmd/hc-service/service/iac-plugin (HTTP Handlers: /api/v1/hc/iac_plugins/*)
   │  1. Decode JSON / Query params → errf.DecodeRequestFailed (2000004)
   │  2. req.Validate()             → errf.InvalidParameter (2000001)
   ▼
cmd/hc-service/logics/iac-plugin/router (*router.Service)
   ├── GetSchema(kt, req) ──► SchemaManager.GetSchema (SchemaCache + DB, 0 Southbound HTTP)
   ├── Health(kt, req)    ──► store.List + iacplugin.Latest ──► httpPlugin.Health
   └── Resolve & Dispatch (9 runtime POST APIs):
         1. store.List + iacplugin.Latest             → verify state == "enabled"
         2. SchemaManager.GetSchema                   → verify type_name & capabilities bit
         3. instanceMgr.GetPluginInstance(instanceID) → merge attrs & secret_attrs + verify inst.PluginName == req.Plugin
         4. Switch protocol == "http"                 → httpPlugin.<Operation>
   ▼
cmd/hc-service/logics/iac-plugin/http-plugin (*httpplugin.Service)
   ├── EncryptLaunchConfig:
   │     ├── Parse HTTPExtension (api_path, crypto_type, crypto_config) + ValidateAPIPath
   │     └── getOrCreateEncryptor (encryptorMap sync.Map) → Encrypt(properties) → Base64 launch_config
   ├── Execute / Plan (prepareStateChange):
   │     ├── ValidateConfigAndRecompute(resSchema.Block, config, recompute)
   │     ├── EncryptLaunchConfig + getPluginClient (pluginCliMap sync.Map)
   │     ├── If prior_state != nil && capabilities.refresh: call Southbound Refresh
   │     │     ├── refreshed == nil && Delete → short-circuit succeeded (state=nil)
   │     │     ├── refreshed == nil && Update → baselinePrior = nil (convert to Create)
   │     │     └── refreshed != nil           → baselinePrior = refreshed, refreshedState = refreshed
   │     ├── ComposeProposedState(resSchema.Block, baselinePrior, config, recompute)
   │     └── Call Southbound /apply or /plan (preserve refreshedState if apply fails with nil state)
   └── GetResult / Stop / Validate / Import / Refresh / ReadData / List:
         └── prepareCall + Call Southbound Client & map to Northbound response
```

### 包路径与文件划分

| 模块路径 | 文件 | 核心职责 |
|---|---|---|
| `pkg/criteria/enumor/` | `iac_plugin.go` | 定义 `IaCPluginOperationType`（9 个北向运行态操作枚举）与 `IaCPluginTaskStatus`（`running`/`succeeded`/`failed`）及 `Validate()` |
| `pkg/api/core/plugin/` | `iac_plugin.go` | 定义北向与南向共用的 `MinListLimit`/`MaxListLimit`、`PathStep`、`Diagnostic`、`HealthData`、`Progress`、`ListItem`、`ListData` |
| `pkg/thirdparty/api-gateway/plugin/` | `types.go`, `client_test.go` | 通过类型别名复用 `coreplugin` 公共模型与 `enumor.IaCPluginTaskStatus` |
| `pkg/api/hc-service/iac-plugin/` | `northbound.go`, `northbound_test.go` | 北向 11 个接口的 Req/Result DTO、`Validate()` 参数校验与零密钥反射单测 |
| `pkg/client/hc-service/iac-plugin/` | `client.go`, `client_test.go` | 北向 11 个接口的 `hc-service` Client SDK 方法与 `httptest` 契约单测 |
| `cmd/hc-service/logics/iac-plugin/router/` | `types.go`, `router.go`, `router_test.go` | `RouteContext`、`router.Service`、状态/Schema/能力位门控、复用 `PluginInstanceManager` 读取合并实例与跨插件归属校验、`GetSchema`/`Health` 及 9 个运行态接口协议分发与单测 |
| `cmd/hc-service/logics/iac-plugin/http-plugin/` | `service.go`, `composer.go`, `executor.go`, `executor_test.go`, `benchmark_test.go` | 支持多层嵌套块的 `ValidateConfigAndRecompute` 与 `ComposeProposedState`（含路径级/通配 `recompute`）、前置 `Refresh` 漂移状态机、RSA-OAEP `launch_config` 加密缓存与南向接口调用映射、并发/零泄露单测与 Benchmark |
| `cmd/hc-service/service/iac-plugin/` | `service.go`, `northbound.go`, `northbound_test.go` | 注册 `/iac_plugins/{health\|schema\|execute\|result\|stop\|plan\|validate\|import\|refresh\|read_data\|list}` 路由与 Handler 单测 |
| `cmd/hc-service/service/capability/` & `cmd/hc-service/service/` | `capability.go`, `iac_plugin.go`, `service.go` | 初始化 `router.Service`（注入 `httpPlugin`）并挂载至 `capability.Capability.PluginRouter` |

## Detailed Interface & Algorithm Design

### 1. 公共枚举、模型与北向 API DTO（`pkg/criteria/enumor/iac_plugin.go`、`pkg/api/core/plugin/iac_plugin.go`、`pkg/api/hc-service/iac-plugin/northbound.go`）

- **操作类型与任务状态枚举（`pkg/criteria/enumor/iac_plugin.go`）**：
  - `IaCPluginOperationType`：`execute`、`result`、`stop`、`plan`、`validate`、`import`、`refresh`、`read_data`、`list`。
  - `IaCPluginTaskStatus`：`running`、`succeeded`、`failed`。
- **无实例请求（Query 参数绑定/校验）**：
  - `HealthReq{Plugin string}`：调用 `validatePluginName(req.Plugin)` 校验插件名正则；响应 `HealthResult` 为 `coreplugin.HealthData` 类型别名。
  - `GetSchemaReq{Plugin string, PluginVersion string}`：校验 `Plugin` 符合正则；当 `PluginVersion != ""` 时调用 `iacplugin.ParseVersion(req.PluginVersion)` 校验三段式 semver；响应直接返回 `*coreplugin.PluginSchema`。
- **需实例公共内嵌结构 `CommonInstanceReq`**：
  - 字段：`Plugin string`、`PluginInstanceID string`、`TypeName string`。
  - `Validate()`：校验 `validatePluginName(req.Plugin)`，且 `PluginInstanceID`、`TypeName` 均非空白。
- **9 个 POST 请求与响应结构**：
  - `ExecuteReq` / `PlanReq`：内嵌 `CommonInstanceReq`，含 `PriorState map[string]any`、`Config map[string]any`、`Recompute []string`、`PriorPrivate *string`、`PriorIdentity map[string]any`。
    - `Validate()`：`PriorState == nil && Config == nil` 时返回错误；遍历 `Recompute` 校验元素非空白字符串；若 `PriorPrivate != nil && len(cvt.PtrToVal(PriorPrivate)) > 0`，校验不能为纯空白且调用 `base64.StdEncoding.DecodeString` 校验合法标准 Base64。
    - 创建/更新/删除语义由 `PriorState` 与 `Config` 是否为 `nil` 直接决定（`PriorState == nil && Config != nil` 为创建，`PriorState != nil && Config != nil` 为更新，`PriorState != nil && Config == nil` 为删除）。
  - `GetResultReq` / `StopReq`：内嵌 `CommonInstanceReq`，含 `ApplyID string`（必填非空白）。
  - `ValidateReq` / `ReadDataReq`：内嵌 `CommonInstanceReq`，含 `Config map[string]any`（必填 `!= nil`）。
  - `ImportReq`：内嵌 `CommonInstanceReq`，含 `ImportID string`（必填非空白）。
  - `RefreshReq`：内嵌 `CommonInstanceReq`，含 `State map[string]any`（必填 `len(State) > 0`）、`Private *string`（可选合法 Base64）、`Identity map[string]any`。
  - `ListReq`：内嵌 `CommonInstanceReq`，含 `Config map[string]any`、`Limit int`（必填 `coreplugin.MinListLimit(1) <= Limit <= coreplugin.MaxListLimit(500)`）、`PageToken string`。
  - 响应结构：`ExecuteResult`（含 `Status enumor.IaCPluginTaskStatus`、`Message string`、`ApplyID string`、`State map[string]any`、`Private *string`、`Identity map[string]any`、`Progress *coreplugin.Progress`、`Diagnostics []coreplugin.Diagnostic`）、`PlanResult`（含 `State`、`ReplacePaths`、`Diagnostics`）、`ValidateResult`（含 `Diagnostics`）、`StateResult`（含 `State`、`Private`、`Identity`）、`ReadDataResult`（含 `State`）、`ListResult = coreplugin.ListData`。

### 2. Client SDK 路由映射（`pkg/client/hc-service/iac-plugin/client.go`）

`pkg/client/hc-service/client.go` 中 `IaCPlugin` 客户端已配置基础路径 `/api/v1/hc`，因此 `iacplugin.Client` 内部相对路径统一为 `/iac_plugins/...`：
- `Health(kt, pluginName)`：通过 `requestGetWithParams` 发送 `GET /iac_plugins/health`（Query 参数 `plugin`）
- `GetSchema(kt, pluginName, pluginVersion)`：通过 `requestGetWithParams` 发送 `GET /iac_plugins/schema`（Query 参数 `plugin` 及非空时的 `plugin_version`）
- 其余 9 个 `POST` 方法（`Execute`、`GetResult`、`Stop`、`Plan`、`Validate`、`Import`、`Refresh`、`ReadData`、`List`）：通过 `common.Request[Req, Resp]` 发送 `POST /iac_plugins/{execute|result|stop|plan|validate|import|refresh|read_data|list}`

### 3. 路由器门控、实例同构合并复用与分发（`cmd/hc-service/logics/iac-plugin/router/router.go`）

`router.Service.Resolve(kt, pluginName, instanceID, typeName, op)` 严格按以下顺序执行前置门控：
1. **插件启用态门控（`getEnabledLatestPlugin`）**：调用 `store.List(kt, s.dataCli, pluginName)` + `iacplugin.Latest(records)`：
   - `latest == nil` → `errf.Newf(errf.RecordNotFound, "iac plugin %s not found", pluginName)`；
   - `latest.State != enumor.IaCPluginStateEnabled` → `errf.Newf(errf.Aborted, "iac plugin %s@%s is not enabled, current state: %s", pluginName, latest.PluginVersion, latest.State)`。
2. **Schema 类型与能力位门控（`validateSchemaTypeAndCapability`）**：调用 `s.schemaMgr.GetSchema(kt, coreplugin.PluginInfo{Name: pluginName, Version: latestPlugin.PluginVersion})`：
   - 类型校验：`IaCPluginOperationReadData` 要求 `schema.DataSourceSchemas[typeName] != nil`；`IaCPluginOperationValidate` 要求存在于 `ResourceSchemas` 或 `DataSourceSchemas`；其余操作要求 `schema.ResourceSchemas[typeName] != nil`；不满足则返回 `errf.InvalidParameter`。
   - 能力位校验：检查 `schema.Capabilities` 中对应布尔位（其中 `IaCPluginOperationStop` 要求 `cap.Stop && cap.AsyncApply`）；不满足则返回 `errf.Aborted`。
3. **实例读取、同构深度合并与归属强校验（`getInstanceProperties`）**：调用 `s.instanceMgr.GetPluginInstance(kt, instanceID)`：
   - `PluginInstanceManager.GetPluginInstance` 内部从 `data-service` 读取 `IacPluginInstance`（不存在时返回 `errf.RecordNotFound`），并通过 `mergeInstanceProperties(raw.Attrs, raw.SecretAttrs)` 将同构的普通属性树 `Attrs` 与已解密敏感属性树 `SecretAttrs` 递归深度合并（支持顶层属性、嵌套 `map[string]any` 同名键递归合并、`[]any` 按下标逐元素递归合并）还原为 `inst.Properties`；
   - 强校验 `inst.PluginName != pluginName` → 返回 `errf.Newf(errf.InvalidParameter, "plugin instance %s belongs to plugin %s, cannot be used by plugin %s", instanceID, inst.PluginName, pluginName)`。
4. **协议分发**：当 `routeCtx.Plugin.Protocol == enumor.IaCPluginProtocolHTTP` 时，调用 `s.httpPlugin` 对应方法；当为 `tf_provider` 或其他协议时返回 `errf.Aborted`。

### 4. 漂移检测与支持多层嵌套的 `proposed_state` 状态合成引擎（`cmd/hc-service/logics/iac-plugin/http-plugin/`）

#### 4.1 前置静态校验与 `Refresh` 漂移检测状态机（`executor.go`：`prepareStateChange`）
在 `Execute` 和 `Plan` 中：
1. 先校验 `schema.ResourceSchemas[typeName].Block` 非空，并调用 `ValidateConfigAndRecompute(resSchema.Block, config, recompute)` 对用户 `config` 与 `recompute` 执行前置静态校验；若不合法立即返回 `errf.InvalidParameter`，不触发加密与南向请求。
2. 调用 `prepareCall` 完成 `launch_config` RSA-OAEP 加密并获取南向 `plugin.Client`。
3. 若 `priorState != nil` 且 `schema.Capabilities.Refresh` 为 `true`：
   - 调用南向 `cli.Refresh(kt, &plugin.RefreshReq{TypeName: typeName, LaunchConfig: launchConfig, State: priorState})`；
   - 若南向 `Refresh` 返回 error（通信/网关失败） → 直接返回 `errf.Newf(errf.Aborted, ...)`；
   - 若 `refreshData == nil || refreshData.State == nil`（外部对象已不存在）：
     - 当 `config == nil`（原本是删除） → **短路完成（`shortCircuit: true`）**：`Execute` 直接返回 `&ExecuteResult{Status: enumor.IaCPluginTaskStatusSucceeded, State: nil}`，`Plan` 直接返回 `&PlanResult{State: nil, ReplacePaths: []string{}}`；
     - 当 `config != nil`（原本是更新） → **转为重建（创建）**：将 `baselinePrior = nil`，后续按创建规则合成 `proposed_state` 并向南向传递 `PriorState: nil`；
   - 若 `refreshData != nil && refreshData.State != nil` → 令 `baselinePrior = refreshData.State`，并记录 `refreshedState = refreshData.State` 供后续 `Apply` 失败且未返回新状态时回填 `data.state`。
4. 调用 `ComposeProposedState(resSchema.Block, baselinePrior, config, recompute)` 合成 `proposed_state`。

#### 4.2 多层嵌套 `ComposeProposedState` 与路径级 `recompute` 合成算法（`composer.go`）
输入：`block *tfjson.SchemaBlock`、`priorState map[string]any`（刷新后的基准状态，创建时为 `nil`）、`config map[string]any`、`recompute []string`。
1. **构建并校验 `recomputeSet`（`buildRecomputeSet` / `findAttributeFromParts`）**：
   - 支持点分与数组下标路径在 `SchemaBlock` 树（含 `NestedBlocks` 与 `AttributeNestedType`）中递归寻址目标 `*tfjson.SchemaAttribute`：
     - 顶层属性：`"bk_module_id"`；
     - Single/Group 嵌套块或 `AttributeNestedType`：`"system_disk.disk_type"`、`"network.private_ip"`；
     - List/Set 嵌套块：支持精确下标 `"data_disks[0].kms_key_id"` 或无下标通配 `"data_disks.kms_key_id"`（校验当前块 `NestingMode` 必须为列表/集合模式，下标必须为非负整数）；
     - Map 嵌套块：支持带运行时动态 key 路径 `"labels_map.env.tag_id"`（跳过动态字典 key `"env"` 在子 Block 中继续查找）或省略动态 key 的通配路径 `"labels_map.tag_id"`；
     - 多层嵌套块：如 `"forward_balancer_ids[0].target_attribute[1].weight"`；
   - 目标属性必须存在且满足 `attr.Optional && attr.Computed`，否则立即返回 `errf.InvalidParameter`。
2. **若 `config == nil`（删除）**：直接返回 `nil, nil`。
3. **递归校验 `config`（`validateBlockConfig` / `validateNestedBlockConfig`）**：
   - 在每一层 `SchemaBlock` 中，若字段既不在 `Attributes` 也不在 `NestedBlocks` 中 → 返回 `errf.InvalidParameter`（含完整字段路径）；
   - 若字段属于 `Attributes` 且为纯 `computed`（`attr.Computed && !attr.Required && !attr.Optional`）且值非 `nil` → 返回 `errf.InvalidParameter`；
   - 若字段为 `NestedBlocks` 或带 `AttributeNestedType` 的属性且值非 `nil`：按 `NestingMode` 校验容器类型（`Single`/`Group` 要求 `map[string]any`，`Map` 要求 `map[string]map[string]any`，`List`/`Set` 要求 `[]map[string]any`）并向下递归校验每个子块。
4. **递归合成每个层级 `proposed_state`（`composeBlockState`）**：
   - 遍历 `block.Attributes`：
     - 若 `attr.AttributeNestedType != nil`（`composeNestedAttrValue`）：当 `config` 未传或为 `nil` 时按外层属性自身的 `Optional`/`Computed` 规则处理（如 `optional+computed` 沿用 `priorState[name]` 整棵子树）；当 `config` 传入非 `nil` 子对象时，转为虚拟 `SchemaBlockType` 与 `priorState[name]` 对齐递归合成；
     - 普通叶子属性（`composeAttributeValue`）：
       - **纯 `computed`**：创建时（`priorState == nil`）置 `nil`；更新时沿用 `priorState[name]`；
       - **`optional + computed`**：若 `config` 显式指定了非 `nil` 值则取 `config[name]`；若未指定或为 `nil`，创建时置 `nil`，更新时通过 `shouldRecomputeField(fieldPath, recomputeSet)` 判断是否命中精确路径或剥离数组下标后的通配路径（`stripAllSliceIndices`）：命中则**显式置为 `nil`**（触发插件重算），否则**沿用 `priorState[name]` 原值**；
       - **`required` 或纯 `optional`**：取 `config[name]`（未传则为 `nil`）。
   - 遍历 `block.NestedBlocks`（`composeNestedBlockValue`）：当 `config` 中存在该子块时，取出 `priorState[name]` 按 `NestingMode` 对齐递归合成：
     - `Single` / `Group`：`priorMap` 与 `cfgMap` 一对一对齐；
     - `Map`：按字典 key 对齐 `priorOuter[k]` 与 `cfgOuter[k]`，并通过 `deriveMapItemRecomputeSet` 将省略动态 key 的通配 `recompute` 路径展开为当前字典项的具体路径；
     - `List` / `Set`：按下标 `i` 对齐 `priorSlice[i]` 与 `cfgSlice[i]`（当 `i < len(priorSlice)` 时传入 `priorSlice[i]` 保留旧元素内部 `computed`/`optional+computed` 字段；当 `i >= len(priorSlice)` 新增元素时 `priorItem = nil` 按创建规则置 `nil`）。

## Decisions

### 1. 北向运行态执行直接合入 `http-plugin` 包，实例同构合并直接复用 `plugin/instance` 包
- **原因**：`cmd/hc-service/logics/iac-plugin/http-plugin` 已维护 HTTP 插件的 `parseExtension` 与 `pluginCliMap sync.Map` 客户端池，而 `cmd/hc-service/logics/plugin/instance` 已实现 `PluginInstanceManager.GetPluginInstance` 及 `attrs`/`secret_attrs` 同构树深度合并。将 HTTP 插件的北向运行态方法（`Health`、`Execute`、`Plan` 等）与 `composer.go` 直接合入 `http-plugin`，并在 `router` 中直接复用 `PluginInstanceManager` 与 `*httpplugin.Service`，既消除了跨包重复代码（无需单独维护 `router/merge.go` 或冗余的 `ThirdPartyExecutor` 接口包装），又保持了 `router`（协议无关门控与分发）与 `http-plugin`（HTTP 协议生命期管理与运行态执行）的清晰边界。

### 2. 公共运行时模型与枚举下沉至 `pkg/api/core/plugin` 与 `pkg/criteria/enumor`
- **原因**：`Diagnostic`、`PathStep`、`Progress`、`HealthData`、`ListData`、`IaCPluginTaskStatus` 同时被北向 API（`pkg/api/hc-service/iac-plugin`）与南向客户端（`pkg/thirdparty/api-gateway/plugin`）使用。将其下沉至 `pkg/api/core/plugin` 与 `pkg/criteria/enumor`，由 `thirdparty` 通过类型别名复用，符合 HCM 分层规范，避免 `pkg/api` 反向依赖 `pkg/thirdparty`。

### 3. `httpplugin.Service` 缓存复用南向 `plugin.Client` 与 `LaunchConfigEncryptor`
- **原因**：非功能需求要求在 `SchemaCache` 命中时，单次路由 + 实例同构合并 + 3072 位 RSA-OAEP 加密 P99 耗时 < 10ms，且支持 50 协程并发安全。
- **做法**：在 `httpplugin.Service` 中通过 `pluginCliMap sync.Map` 按 `api_path` 缓存 `plugin.Client`，并通过 `encryptorMap sync.Map` 按 `crypto_type + ":" + public_key` 缓存 `plugin.LaunchConfigEncryptor` 实例；由于 `RSAOaep.EncryptToBase64` 仅读取公钥参数与 `crypto/rand.Reader`，天然无状态且并发安全。

### 4. 外层调用错误与内部任务失败/漂移状态严格分离
- **原因**：北向协议要求当三方网关不可达或返回 `result=false`/`code!=0` 时，北向返回外层 `errf.Aborted`（`code=2000006, data=nil`），防止控制面误覆盖已保存的 `prior_state`；而当南向 `Apply` 正常响应 `result=true, code=0, data.status="failed"` 时，外层 `code` 必须为 `0`，且若前置 `Refresh` 已刷出漂移后的 `refreshed_state` 而 `Apply` 失败未返回新 `state`，必须将 `refreshed_state` 填入 `data.state` 交回控制面保存。
- **做法**：在 `httpplugin.Service.Execute` 中保存前置 `Refresh` 的 `plan.refreshedState`；当 `applyData.Status == enumor.IaCPluginTaskStatusFailed && state == nil && plan.refreshedState != nil` 时，显式将 `state = plan.refreshedState`。

## Risks / Trade-offs

- **[Risk] 明文 `properties` 在日志或错误信息中意外泄露** → **Mitigation**：`PluginInstanceManager.GetPluginInstance` 产出的 `Properties` 仅存在于内存局部变量并直接传入 `encryptor.Encrypt(properties)`；日志只允许记录 `plugin`、`plugin_instance_id`、`type_name`、`rid`，严禁对 `RouteContext`、`IacPluginInstance` 或 `properties` 使用 `%+v`/`%v` 打印；编写专项安全单测注入 `"SK_SUPER_SECRET_12345"` 断言日志与错误响应中出现次数为 0。
- **[Risk] 实例 `properties` 序列化后超出 3072 位 RSA-OAEP 明文上限（`keyBytes - 66 = 318` 字节）** → **Mitigation**：捕获 `encryptor.Encrypt` 返回的明文超长错误，转换为 `errf.InvalidParameter`（`2000001`）立即返回（错误消息仅包含字节长度与上限，不包含明文内容），不发起南向 HTTP 请求。
