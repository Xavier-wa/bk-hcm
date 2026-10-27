# plugin-northbound-contract-sdk Specification

## Purpose
定义 HCM `hc-service` 北向协议 11 个标准 HTTP 接口（`/api/v1/hc/iac_plugins/{health|schema|execute|result|stop|plan|validate|import|refresh|read_data|list}`）的请求/响应数据模型、前置参数校验、三态变更语义、`pkg/criteria/enumor` 与 `pkg/api/core/plugin` 公共枚举/运行时模型下沉，以及 `pkg/client/hc-service/iac-plugin` Client SDK 规范。

## Requirements
### Requirement: 北向协议 11 个标准 HTTP 接口路由与请求契约
系统 SHALL 在 `hc-service` 的 `/api/v1/hc/iac_plugins/` 路径下暴露全部 11 个北向协议 HTTP 接口，并在 `pkg/api/hc-service/iac-plugin/northbound.go` 中定义对应的请求与响应数据结构，确保北向请求体中 **零明文密钥与零 `launch_config` 传输**：
- `GET /api/v1/hc/iac_plugins/health`：Query 参数 `plugin`（必填，复用 `validatePluginName` 校验插件名正则 `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`），响应 `data` 为 `HealthResult`（即 `coreplugin.HealthData`，`{ "plugin_version": string }`）；
- `GET /api/v1/hc/iac_plugins/schema`：Query 参数 `plugin`（必填，符合插件名正则）、`plugin_version`（可选，非空时通过 `iacplugin.ParseVersion` 校验三段式 semver 格式），响应 `data` 为完整 `*coreplugin.PluginSchema`；
- 其余 9 个 `POST` 接口（`execute`、`result`、`stop`、`plan`、`validate`、`import`、`refresh`、`read_data`、`list`）均内嵌 `CommonInstanceReq`，包含公共必填字段：`plugin`（符合插件名正则）、`plugin_instance_id`（非空白字符串）、`type_name`（非空白字符串），且请求结构体 MUST NOT 包含 `launch_config`、`secret_attrs`、`secret_key`、`secret_id`、`access_token`、`properties` 等任何凭证字段。

#### Scenario: 合法的北向公共寻址参数与零密钥结构校验通过
- **WHEN** 调用北向任意接口，传入符合 `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$` 的 `plugin`（以及需实例接口的非空 `plugin_instance_id` 和 `type_name`），且请求体中不包含任何明文凭证或 `launch_config`
- **THEN** 公共参数校验返回 `nil`，允许进入后续业务路由流程

#### Scenario: 缺失必填公共字段或非法插件名格式被前置拦截
- **WHEN** `plugin` 为空或包含 `/`、空格等不满足 `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$` 的字符，或 POST 接口缺失 `plugin_instance_id` / `type_name`，或 `GetSchema` 传入非法的 `plugin_version`
- **THEN** Handler 在前置校验阶段直接返回 `errf.InvalidParameter`（`code=2000001`），不查询数据库且不调用下游插件

### Requirement: Execute 与 Plan 请求参数校验及三态变更语义（规则 R-003）
系统 SHALL 为 `POST /api/v1/hc/iac_plugins/execute` 与 `POST /api/v1/hc/iac_plugins/plan` 提供请求参数校验与创建/更新/删除三态变更语义：
- 请求支持 `prior_state`（`map[string]any`）、`config`（`map[string]any`）、`recompute`（`[]string`，可选，指定更新时在 `config` 中未指定/为 `null` 且需在 `proposed_state` 中显式置为 `null` 重算的 `optional + computed` 属性路径列表）、`prior_private`（`*string`，可选标准 Base64）、`prior_identity`（`map[string]any`，可选）；
- 按 `prior_state` 与 `config` 是否为 `nil` 表达三态变更语义：`prior_state == nil && config != nil` 为**创建（Create）**；`prior_state != nil && config != nil` 为**更新（Update）**；`prior_state != nil && config == nil` 为**删除（Delete）**；
- 当 `prior_state == nil && config == nil` 时，系统 MUST 判定为非法参数并直接拒绝；
- 当 `recompute` 包含空或纯空白字符串元素，或 `prior_private` 非空但为纯空白或不是合法标准 Base64 字符串时，系统 MUST 直接拒绝。

#### Scenario: 创建、更新、删除三种合法状态组合校验通过
- **WHEN** 调用 `ExecuteReq.Validate()` 或 `PlanReq.Validate()` 并传入：① 创建（`prior_state=nil, config={"bk_biz_id": 213}`）；② 更新（`prior_state={"bk_biz_id": 213}, config={"bk_biz_id": 213}, recompute=["bk_module_id"]`）；③ 删除（`prior_state={"bk_biz_id": 213}, config=nil`）
- **THEN** `Validate()` 均返回 `nil`

#### Scenario: prior_state 与 config 同时为 nil 或可选字段格式非法时前置拒绝
- **WHEN** 调用北向 `Execute` 或 `Plan` 时传入 `prior_state=nil && config=nil`，或 `recompute` 含空/空白字符串 `""`，或 `prior_private` 为非法 Base64 字符串
- **THEN** Handler 直接返回 `errf.InvalidParameter`（`code=2000001`），不查询数据库且不调用下游插件

### Requirement: GetResult、Stop、Validate、Import、Refresh、ReadData 与 List 接口参数校验
系统 SHALL 对其余 7 个北向 POST 接口执行严格的前置参数校验：
- `POST /api/v1/hc/iac_plugins/result` 与 `POST /api/v1/hc/iac_plugins/stop`：`apply_id` MUST 为非空白字符串；
- `POST /api/v1/hc/iac_plugins/validate` 与 `POST /api/v1/hc/iac_plugins/read_data`：`config` MUST 为非 `nil` 的 `map[string]any`；
- `POST /api/v1/hc/iac_plugins/import`：`import_id` MUST 为非空白字符串；
- `POST /api/v1/hc/iac_plugins/refresh`：`state` MUST 为非空 `map[string]any`（`len(state) > 0`），可选 `private` 非空时 MUST 为合法标准 Base64 字符串；
- `POST /api/v1/hc/iac_plugins/list`：`limit` MUST 满足 `coreplugin.MinListLimit(1) <= limit <= coreplugin.MaxListLimit(500)`，支持可选 `config` 过滤条件与 `page_token` 游标（首页为 `""`）。

#### Scenario: List 分页 limit 边界校验与其余接口必填项校验拦截
- **WHEN** 调用 `List` 传入 `limit=0` 或 `limit=501`，或调用 `GetResult`/`Stop` 传入空 `apply_id`，或调用 `Import` 传入空 `import_id`，或调用 `Refresh` 传入空 `state`，或调用 `Validate`/`ReadData` 传入 `config=nil`
- **THEN** Handler 在前置校验阶段 100% 拦截并返回 `errf.InvalidParameter`（`code=2000001`）；若请求体 JSON 语法错误则返回 `errf.DecodeRequestFailed`（`code=2000004`）

### Requirement: 公共运行时模型/枚举下沉、北向统一响应包络与 hc-service Client SDK
系统 SHALL 遵循 HCM 内部标准响应 `{ "code": int, "message": string, "data": any }` 封装全部 11 个北向接口响应，并在 `pkg/client/hc-service/iac-plugin` 中提供全部 11 个北向接口的类型安全 Client SDK 方法（`Health`、`GetSchema`、`Execute`、`GetResult`、`Stop`、`Plan`、`Validate`、`Import`、`Refresh`、`ReadData`、`List`）：
- 在 `pkg/criteria/enumor/iac_plugin.go` 中定义 `IaCPluginOperationType`（`execute`/`result`/`stop`/`plan`/`validate`/`import`/`refresh`/`read_data`/`list`）与 `IaCPluginTaskStatus`（`running`/`succeeded`/`failed`）；
- 在 `pkg/api/core/plugin/iac_plugin.go` 中定义 `PathStep`、`Diagnostic`、`HealthData`、`Progress`、`ListItem`、`ListData`、`MinListLimit`/`MaxListLimit`，由北向 `pkg/api/hc-service/iac-plugin` 与南向 `pkg/thirdparty/api-gateway/plugin` 共同复用，保持 100% JSON 兼容且消除 `pkg/api` 对 `pkg/thirdparty` 的反向依赖。

#### Scenario: 通过 Client SDK 调用全部 11 个北向接口契约一致
- **WHEN** 控制面调用方通过 `pkg/client/hc-service/iac-plugin.Client` 依次调用 `Health`、`GetSchema`、`Execute`、`GetResult`、`Stop`、`Plan`、`Validate`、`Import`、`Refresh`、`ReadData`、`List`
- **THEN** 客户端发起的 HTTP 方法（`GET`/`POST`）、URL 路径（`/api/v1/hc/iac_plugins/*`）、Query 参数、请求体序列化与响应体反序列化 100% 符合北向协议契约
