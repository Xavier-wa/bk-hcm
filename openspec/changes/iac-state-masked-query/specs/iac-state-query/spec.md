# Spec Delta

## Purpose

在 `iac-server` 中提供仓库维度的受管实例状态只读查询能力（`ListState` 摘要分页列表与 `GetState` 脱敏详情），确保导入暂存态隔离、跨租户与跨仓库访问隔离，并基于插件 Schema 对敏感属性执行 Fail-Closed 递归脱敏与内部私有字段剥离。

## ADDED Requirements

### Requirement: 分页查询仓库受管状态摘要列表
系统 SHALL 在 `iac-server` 提供 `POST /api/v1/iac/repository/{repository_id}/state/list`（`ListState`）只读接口，先校验 `repository_id` 在 `iac_repository` 中存在且归属当前请求租户（`tenant_id`），再按 `repository_id` 隔离查询并默认强制过滤 `import_from_spec_id == ""`，分页返回受管实例摘要列表且 MUST NOT 返回 `state`、`private` 或 `extra` 字段。

#### Scenario: 仓库不存在或跨租户返回 404
- **WHEN** 路径参数 `repository_id` 在 `iac_repository` 中不存在或归属其他租户，客户端调用 `ListState`
- **THEN** 系统返回资源不存在错误（`RecordNotFound` / HTTP 404）

#### Scenario: 正常分页返回受管状态摘要
- **WHEN** 客户端以合法 `repository_id` 与分页参数调用 `ListState`
- **THEN** 系统返回 `details` 列表，每条摘要包含 `id`、`repository_id`、`block_id`、`kind`、`has_current`、`plugin`、`plugin_instance_id`、`type_name`、`deposed_count`、`deposed_keys`、`last_flow_id` 与 `updated_at`，且不包含 `state`、`private` 或 `extra`

#### Scenario: Current 为空但 Deposed 非空时的摘要提取
- **WHEN** 某条已生效记录的 `current` 为 `NULL` 且 `deposed` 非空（如含 `k-1`）
- **THEN** 摘要中 `has_current` 为 `false`，`deposed_count` 为 `deposed` 键数量，`deposed_keys` 为排序后的键列表，`plugin_instance_id` 取行级字段，`plugin` 与 `type_name` 从 `deposed` 中的旧实例提取

#### Scenario: 暂存态与跨仓库跨租户记录不可见
- **WHEN** 仓库中存在 `import_from_spec_id != ""` 的暂存记录，或其他仓库/其他租户的状态记录
- **THEN** `ListState` 响应中不包含这些记录；当客户端在过滤条件中尝试覆盖 `repository_id` 或 `import_from_spec_id` 时，服务端强制条件仍生效，不越权返回

### Requirement: 查询单个受管实例状态详情并剥离私有字段
系统 SHALL 在 `iac-server` 提供 `GET /api/v1/iac/repository/{repository_id}/state/{state_id}`（`GetState`）只读接口，校验 `repository_id` 归属当前租户且记录同时匹配当前 `tenant_id`、路径参数 `repository_id`、主键 `state_id` 与 `import_from_spec_id == ""`，返回包含 `current` 与 `deposed` 的实例详情，并 MUST 从 `current` 及所有 `deposed` 实例中剥离 `private` 字段；`extra` 字段原样返回不参与敏感脱敏。

#### Scenario: 查询合法已生效实例详情成功
- **WHEN** 目标 `state_id` 属于当前租户与指定 `repository_id` 且 `import_from_spec_id == ""`，客户端调用 `GetState`
- **THEN** 系统返回完整详情（含 `id`、`repository_id`、`block_id`、`kind`、`plugin_instance_id`、`last_flow_id`、`current`、`deposed`），响应 JSON 中 `current` 与各 `deposed` 实例均不包含 `private` 字段，`extra` 原样保留

#### Scenario: 记录不存在、跨仓库/租户或处于暂存态返回 404
- **WHEN** `repository_id` 或 `state_id` 不存在、归属其他 `repository_id` 或其他 `tenant_id`、或处于 `import_from_spec_id != ""` 暂存态
- **THEN** `GetState` 返回资源不存在错误（`RecordNotFound` / HTTP 404）

### Requirement: 基于插件 Schema 的敏感属性递归脱敏
`GetState` 在返回 `current` 与 `deposed` 实例前 SHALL 根据每个实例的 `plugin` 查询 `iac_plugin` 中状态为 `enabled` 的插件版本，再按 `(plugin_name, plugin_version)` 查询插件 Schema 获取对应资源或数据源的 Schema 定义，递归将 `state` 中所有标记为 `sensitive = true` 的属性值替换为固定占位符 `"(sensitive value)"`。

#### Scenario: 顶层与嵌套敏感属性被全量脱敏
- **WHEN** 受管实例 `current.state` 或 `deposed[key].state` 包含顶层 `sensitive = true` 字段（如 `password`）及嵌套块或嵌套对象中的 `sensitive = true` 字段
- **THEN** `GetState` 响应中所有敏感属性值均被替换为 `"(sensitive value)"`，非敏感属性与 `extra` 保持原值不变

#### Scenario: Data Source 按 data_source_schemas 脱敏
- **WHEN** 受管实例记录的 `kind` 为 `data`
- **THEN** 系统从插件 Schema 的 `data_source_schemas[type_name]` 读取字段敏感标记并对 `state` 执行递归脱敏

#### Scenario: 含多个 Deposed 实例时逐一脱敏
- **WHEN** 记录同时包含 `current` 与一条或多条 `deposed` 实例
- **THEN** 系统对 `current` 及每条 `deposed` 实例分别按其 `plugin` 与 `type_name` 执行敏感属性脱敏与 `private` 剥离

### Requirement: 插件 Schema 不可用时执行 Fail-Closed 拦截
当 `GetState` 无法获取目标实例对应的权威插件 Schema 时，系统 MUST 中止响应并返回错误，严禁降级返回任何未脱敏明文状态。

#### Scenario: 插件无 enabled 版本或未注册时拒绝返回
- **WHEN** 实例的 `plugin` 在 `iac_plugin` 中不存在或没有 `state = "enabled"` 的版本记录
- **THEN** `GetState` 返回错误，响应体中不包含任何 `state` 明文内容

#### Scenario: 插件 Schema 缺失、类型未定义或查询失败时拒绝返回
- **WHEN** 插件 Schema 查无 `(plugin_name, plugin_version)` 记录、Schema 中缺少目标 `type_name` 定义、或下游查询报错
- **THEN** `GetState` 返回错误，绝不返回未脱敏的 `current` 或 `deposed` 明文

### Requirement: 对外接口只读约束
`iac-server` 对外关于 `iac_state` 的路由 SHALL 仅暴露只读查询接口（`ListState` 与 `GetState`），不对外暴露直接写入、覆盖或删除状态记录的 HTTP 端点。

#### Scenario: 仅注册只读查询路由
- **WHEN** `iac-server` 初始化并挂载 State 服务路由
- **THEN** 仅注册 `POST /api/v1/iac/repository/{repository_id}/state/list` 与 `GET /api/v1/iac/repository/{repository_id}/state/{state_id}` 两个只读端点
