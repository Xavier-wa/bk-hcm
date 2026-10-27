# plugin-state-composer-thirdparty Specification

## Purpose
定义 HCM `hc-service` 在 `cmd/hc-service/logics/iac-plugin/http-plugin` 中实现的静态配置前置校验、`Refresh` 漂移检测状态机（含外部已删除时的删除短路与更新自动转重建）、对照 `*tfjson.SchemaBlock`（含顶层属性、`NestedBlocks` 与 `AttributeNestedType`）的递归 `proposed_state` 状态合成引擎（支持 4 类属性规则与多层级/通配路径 `recompute: []string` 显式置 `null` 重算），以及三方 HTTP 插件 RSA-OAEP `launch_config` 加密执行通道与外层/内层状态分离映射规范。

## Requirements
### Requirement: Execute 与 Plan 前置静态校验、Refresh 漂移检测与短路/重建转换（规则 R-004）
系统 SHALL 在 `httpplugin.Service` 的 `Execute` 与 `Plan` 处理过程中（`prepareStateChange`），先通过 `ValidateConfigAndRecompute` 对照资源 Schema 静态校验 `config` 与 `recompute`，随后当入参 `prior_state != nil` 且插件 `capabilities.refresh == true` 时调用南向 `Refresh` 获取外部系统最新状态 `refreshed_state`：
- 若 `config` 或 `recompute` 违反资源 Schema 约束，系统 MUST 在发起南向 `Refresh` 前直接返回 `errf.InvalidParameter`（`code=2000001`）；
- 若前置 `Refresh` 发生网关通信错误（外层非 0），系统 MUST 直接返回 `errf.Aborted`（`code=2000006, data=nil`），不执行后续变更；
- 若 `refreshed_state == nil`（外部对象已被手动删除）且本次操作为**删除**（`config == nil`），系统 MUST 短路跳过后续插件 `Apply` / `Plan` 调用，直接返回成功（`Execute` 返回 `status="succeeded", state=nil`；`Plan` 返回 `state=nil, replace_paths=[]`）；
- 若 `refreshed_state == nil`（外部对象已被手动删除）且本次操作为**更新**（`config != nil`），系统 MUST 将基准 `baselinePrior` 置为 `nil`，自动转为**创建（Create）**流程；
- 若 `refreshed_state != nil`，系统 MUST 以 `refreshed_state` 替换入参 `prior_state` 作为基准状态参与后续 `proposed_state` 合成与南向调用。

#### Scenario: 删除操作前置 Refresh 发现外部对象已不存在时短路返回成功
- **WHEN** 调用北向 `Execute`（或 `Plan`）执行删除（`prior_state != nil, config == nil`），且前置南向 `Refresh` 返回 `state: nil`
- **THEN** `hc-service` 不再调用南向 `Apply`（或 `Plan`），直接返回外层 `code=0`，其中 `Execute` 的 `data.status="succeeded", data.state=nil`，`Plan` 的 `data.state=nil, data.replace_paths=[]`

#### Scenario: 更新操作前置 Refresh 发现外部对象已不存在时自动转为创建
- **WHEN** 调用北向 `Execute`（或 `Plan`）执行更新（`prior_state != nil, config != nil`），且前置南向 `Refresh` 返回 `state: nil`
- **THEN** `hc-service` 将基准 `baselinePrior` 置为 `nil`，按创建规则合成 `proposed_state`，并向南向插件发起 `prior_state=nil` 的创建请求

### Requirement: 对照资源 Schema（含多层嵌套块与嵌套属性）合成 proposed_state 与路径级 recompute 重算引擎（规则 R-005）
系统 SHALL 在 `cmd/hc-service/logics/iac-plugin/http-plugin/composer.go` 中对照目标资源的 `*tfjson.SchemaBlock`（统一支持顶层 `Attributes`、SDKv2 `NestedBlocks` 以及 Plugin Framework `AttributeNestedType`）递归校验用户 `config` 与 `recompute` 列表，并合成 `proposed_state`：
- 当 `config == nil`（删除）时，校验 `recompute` 合法后返回 `proposed_state = nil`；
- **递归配置校验（`validateBlockConfig`）**：
  - 若任意层级 `config` 中包含既不在当前 `Block.Attributes` 也不在 `Block.NestedBlocks` 中声明的字段，或包含纯 `computed`（`computed=true, required=false, optional=false`）且值非 `nil` 的属性，或嵌套块值容器类型与 `NestingMode` 不匹配（`Single`/`Group` 须为 `map[string]any`，`Map` 须为 `map[string]map[string]any`，`List`/`Set` 须为 `[]map[string]any`），系统 MUST 返回 `errf.InvalidParameter`（`code=2000001`）拒绝请求；
- **多层级与通配路径 `recompute` 校验（`buildRecomputeSet`）**：
  - 支持顶层属性（如 `"bk_module_id"`）、Single/Group 嵌套块或 `AttributeNestedType` 子属性（如 `"system_disk.disk_type"`、`"network.private_ip"`）、List/Set 嵌套块精确下标路径（如 `"data_disks[0].kms_key_id"`）或无下标通配路径（如 `"data_disks.kms_key_id"`）、Map 嵌套块带动态 key 路径（如 `"labels_map.env.tag_id"`）或省略动态 key 通配路径（如 `"labels_map.tag_id"`）以及多层嵌套组合路径；
  - 若 `recompute` 路径语法非法（如非列表块带下标、下标非数字）、目标属性在 Schema 中不存在，或目标属性不满足 `optional=true && computed=true`，系统 MUST 返回 `errf.InvalidParameter`（`code=2000001`）拒绝请求；
- **叶子属性与嵌套块递归合成（`composeBlockState`）**：
  - 对 `required` 或纯 `optional`（`computed=false`）属性：取 `config` 中的值（`config` 未指定则为 `nil`）；
  - 对纯 `computed`（`computed=true, required=false, optional=false`）属性：创建时置为 `nil`，更新时沿用刷新后的 `prior_state` 对应值；
  - 对 `optional + computed`（`optional=true && computed=true`）属性：若用户在 `config` 中显式指定了非 `nil` 值则取 `config` 中的值；若用户在 `config` 中未指定（或为 `nil`），创建时置为 `nil`（交由插件赋默认值），更新时若该属性路径命中 `recompute`（含精确路径或通配路径匹配）则显式置为 `nil`（触发插件重算），否则**沿用刷新后的 `prior_state` 原值（保持现状）**；
  - 对嵌套属性（`AttributeNestedType`）与嵌套块（`NestedBlocks`）：Single/Group 按对象一对一对齐递归合成；Map 按字典 key 对齐递归合成；List/Set 按数组下标 `i` 对齐递归合成（当 `i < len(priorSlice)` 时对齐旧元素保留未改动 `computed`/`optional+computed` 子属性，当 `i >= len(priorSlice)` 新增元素时 `priorItem=nil` 按创建规则置 `nil`）。

#### Scenario: 更新时合成顶层与多层嵌套块属性并按精确/通配 recompute 路径显式置 null 重算
- **WHEN** ① 顶层更新场景：资源 Schema 含 `required` 字段 `bk_biz_id`、纯 `optional` 字段 `operator`、`optional+computed` 字段 `bk_cloud_id` 与 `bk_module_id`、纯 `computed` 字段 `bk_host_id`，刷新后 `prior_state={"bk_biz_id":213, "bk_cloud_id":0, "bk_module_id":5001, "operator":"old", "bk_host_id":10086}`，入参 `config={"bk_biz_id":213}` 且 `recompute=["bk_module_id"]`；或 ② 嵌套块更新场景：含 `AttributeNestedType`（`network`）、Single 块（`system_disk`）、二层 List 块（`forward_balancer_ids[].target_attribute[]`，含已有元素修改与末尾新增元素）及 Map 块（`labels_map`），并传入 `recompute=["forward_balancer_ids[0].kms_key_id", "labels_map.env.tag_id"]`
- **THEN** ① 顶层合成的 `proposed_state` 为 `{"bk_biz_id":213, "bk_cloud_id":0, "bk_module_id":nil, "operator":nil, "bk_host_id":10086}`；② 嵌套块已有元素保留未改动 `computed`/`optional+computed` 子字段、命中 `recompute` 的子字段置为 `nil`、List 末尾新增元素的 `computed`/`optional+computed` 未指定子字段置为 `nil`

#### Scenario: 顶层或嵌套块中 config 包含未知/纯 computed 字段或 recompute 路径非法时报错拒绝
- **WHEN** 调用 `Execute` 或 `Plan` 时：① 在顶层或嵌套块 `config` 中传入纯 `computed` 字段（如 `{"bk_host_id": 10086}`、`{"system_disk": {"disk_size": 50, "disk_id": "d-1"}}`）、未知字段或错误容器类型；或 ② 在 `recompute` 中传入 `required`/纯 `computed`/未知字段路径，或在 Single 块上使用数组下标（如 `"system_disk[0].disk_type"`）
- **THEN** 状态合成引擎直接返回 `errf.InvalidParameter`（`code=2000001`），不向南向插件发起变更请求

### Requirement: 三方 HTTP 插件 launch_config 加密与 9 个运行态接口调用映射（规则 R-006）
系统 SHALL 在 `httpplugin.Service`（`cmd/hc-service/logics/iac-plugin/http-plugin/executor.go`）中完成 `HTTPExtension` 解析、RSA-OAEP `launch_config` 加密缓存、南向 `plugin.Client` 并发复用、9 个运行态接口调用以及外层通信状态与内部业务状态的严格分离：
- 解析 `iac_plugin.extension` 中的 `coreplugin.HTTPExtension`（`api_path`、`crypto_type`、`crypto_config`），校验 `api_path`，通过 `encryptorMap sync.Map` 缓存复用 `plugin.LaunchConfigEncryptor` 将实例明文 `properties` 经 RSA-OAEP（SHA-256）加密为 Base64 字符串 `launch_config`；若明文 JSON 长度超过 RSA 公钥上限（`keyBytes - 66`），系统 MUST 返回 `errf.InvalidParameter`（`code=2000001`）且不发起南向 HTTP 请求；
- `Execute` 与 `Plan` 仅向南向 `/apply` 与 `/plan` 传递 `type_name`、`launch_config`、刷新后的 `prior_state` 与合成后的 `proposed_state`（不传 `config`、`recompute`、`private`、`identity`）；`Execute`、`GetResult`、`Stop`、`Import`、`Refresh` 映射回北向响应时 `private` 与 `identity` 置为 `nil`；
- 当南向 HTTP 调用返回网关/通信错误（`result=false` 或 `code!=0`）时，系统 MUST 返回 `errf.Aborted`（外层 `code=2000006, data=nil`）；
- 当 `Execute` / `GetResult` / `Stop` 任务执行失败（`data.status="failed"`）时，北向响应外层 `code` MUST 仍为 `0`，保留 `data.message`；若南向返回了部分落地 `state` 则返回该 `state`，若南向返回的 `state == nil` 但 `Execute` 前置 `Refresh` 已获取到最新状态 `refreshed_state`，则 `data.state` MUST 返回 `refreshed_state`；
- 当 `Validate` 配置校验不通过返回 `diagnostics` 时，北向响应外层 `code` MUST 仍为 `0`，问题列表写入 `data.diagnostics`；
- 全链路在任何日志级别与错误消息中 MUST NOT 打印明文 `properties` 属性值，并在 `SchemaCache` 命中场景下满足单次路由 + 同构合并 + 3072 位 RSA-OAEP 加密 P99 耗时 < 10ms 及多协程并发安全。

#### Scenario: 三方插件 RSA-OAEP 加密 launch_config 与运行态接口全链路闭环
- **WHEN** `RouteContext` 携带合法 3072 位 RSA 公钥与合并后的明文 `properties`，调用 `httpplugin.Service` 的 `Execute`、`Plan`、`GetResult`、`Stop`、`Validate`、`Import`、`Refresh`、`ReadData`、`List`
- **THEN** `httpplugin.Service` 将 `properties` 加密为 `launch_config`（可用对应私钥解密还原且日志中敏感值出现次数为 0），调用南向客户端并返回外层 `code=0` 的北向标准响应；当 `properties` 超出 `keyBytes - 66` 时前置返回 `errf.InvalidParameter`（`2000001`）

#### Scenario: Execute 任务失败时回传前置 Refresh 漂移状态且与网关通信失败隔离
- **WHEN** ① `Execute` 更新时前置 `Refresh` 刷新得到漂移状态 `S_refreshed`，随后南向 `Apply` 返回 `result=true, code=0, data={status:"failed", state:nil, message:"apply err"}`；或 ② 南向网关不可达返回 `result=false, code=50300`
- **THEN** ① 北向返回外层 `code=0`，`data.status="failed"`、`data.message="apply err"` 且 `data.state` 等于 `S_refreshed`；② 北向返回外层 `code=2000006`（`errf.Aborted`）且 `data=nil`
