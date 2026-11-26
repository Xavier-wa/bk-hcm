# Spec Delta

## Purpose

提供 IaC 受管实例状态（`iac_state`）的持久化存储、批量数据访问接口与无状态状态管理器（`StateManager`），统一管理仓库级受管资源的 `current` 与 `deposed` 状态流转、依赖状态读取、先建后删替换、销毁清理与导入暂存态隔离。

## ADDED Requirements

### Requirement: 仓库级状态记录持久化与唯一性约束
系统 SHALL 在 `iac_state` 表中按 `(repository_id, block_id)` 唯一键持久化存储受管实例状态记录，启用多租户 `tenant_id` 隔离，并将 `kind` 限定为 `resource` 或 `data`、`deposed` 默认为 `{}`、`import_from_spec_id` 默认为空字符串 `""`。数据服务层 MUST 提供单数路由的批量创建、批量更新、批量删除与列表查询接口。

#### Scenario: 创建合法状态记录成功
- **WHEN** 调用批量创建接口写入包含合法 `repository_id`、`block_id`、`plugin_instance_id`、`kind`（`resource` 或 `data`）与 `current` 的状态记录
- **THEN** 系统生成主键 `id` 并成功落库，`deposed` 默认保存为空 JSON 对象 `{}`，`import_from_spec_id` 默认为空字符串 `""`

#### Scenario: 同仓库重复 block_id 写入被拒绝
- **WHEN** 同一 `repository_id` 下已存在相同 `block_id` 的状态记录，再次执行批量创建写入该 `(repository_id, block_id)`
- **THEN** 系统返回数据重复错误（`RecordDuplicated`），不覆盖既有记录

#### Scenario: 非法 kind 或超限批量请求被拒绝
- **WHEN** 传入 `kind` 非 `resource`/`data` 的记录，或单次批量操作条数超过系统上限
- **THEN** 系统返回参数校验错误（`InvalidParameter`），拒绝写入数据库

### Requirement: 查询仓库已生效受管状态
状态管理器 SHALL 提供 `GetRepositoryStates(kt, repositoryID, blockIDs)` 方法，查询指定仓库下所有已正式生效（`import_from_spec_id == ""`）的状态记录，并以 `map[block_id]*StateRecord` 形式返回；当无匹配记录时 MUST 返回 `nil`。

#### Scenario: 查询仓库全量已生效状态并过滤暂存态
- **WHEN** 仓库中存在正式生效记录（`import_from_spec_id == ""`）与导入暂存记录（`import_from_spec_id == "spec-001"`），调用 `GetRepositoryStates(kt, repositoryID, nil)`
- **THEN** 仅返回 `import_from_spec_id == ""` 的记录字典，暂存记录不被返回；当暂存记录的 `import_from_spec_id` 清空为 `""` 后可被正常查出

#### Scenario: 按 blockIDs 列表定向过滤查询
- **WHEN** 传入非空 `blockIDs` 切片调用 `GetRepositoryStates`
- **THEN** 仅返回同时满足 `repository_id`、`import_from_spec_id == ""` 且 `block_id` 位于 `blockIDs` 中的状态记录

#### Scenario: 仓库无已生效记录返回 nil
- **WHEN** 目标仓库不存在任何 `import_from_spec_id == ""` 的状态记录
- **THEN** `GetRepositoryStates` 返回 `(nil, nil)`

### Requirement: 批量获取上游依赖实例状态
状态管理器 SHALL 提供 `GetDependencyStates(kt, repositoryID, depBlockIDs)` 方法，批量读取指定上游 `block_id` 列表对应已生效记录的 `Current.State`（忽略 `Deposed` 中的待销毁旧实例）并返回 `map[block_id]map[string]any`。

#### Scenario: 全部上游依赖存在且 Current 非空
- **WHEN** `depBlockIDs` 中的每个 `block_id` 在目标仓库均有已生效记录且 `Current != nil`（即使其 `Deposed` 非空）
- **THEN** 系统返回每个 `block_id` 对应的 `Current.State` 字典，不返回任何 `Deposed` 中的旧实例内容

#### Scenario: 任一依赖缺失或 Current 为空时报错
- **WHEN** `depBlockIDs` 中存在未落库、处于暂存态或 `Current == nil` 的 `block_id`
- **THEN** 系统返回错误，且错误信息 MUST 明确包含缺失的 `block_id`

### Requirement: Apply 执行写回与先建后删替换流转
状态管理器 SHALL 提供 `UpsertAppliedState(kt, req)` 方法，根据 `DeposedKey` 控制直接覆写 `current` 或将原 `current` 移入 `deposed[DeposedKey]` 后写入新 `current`，并在行级更新 `plugin_instance_id` 与 `last_flow_id`，扩展信息 `extra` 随对应 `StateObject` 独立保存。

#### Scenario: 常规新增或原地覆盖 Current
- **WHEN** `DeposedKey` 为空字符串且记录不存在时调用 `UpsertAppliedState`
- **THEN** 系统新增一行状态记录，写入 `Current`、`PluginInstanceID`、`LastFlowID` 并将 `Deposed` 初始化为 `{}`
- **WHEN** `DeposedKey` 为空字符串且记录已存在时调用 `UpsertAppliedState`
- **THEN** 系统覆写该行 `Current` 并更新 `PluginInstanceID` 与 `LastFlowID`，保留既有 `Deposed` 内容不变

#### Scenario: 先建后删替换将原 Current 移入 Deposed
- **WHEN** `block_id` 已有生效实例 `ins-old`，以非空 `DeposedKey="k-1"` 与新实例 `ins-new` 调用 `UpsertAppliedState`
- **THEN** 原 `Current`（`ins-old`）移入 `Deposed["k-1"]`，新实例 `ins-new` 写入 `Current`，并更新行级 `PluginInstanceID` 与 `LastFlowID`

### Requirement: Import 前缀剥离与半成品状态落盘
状态管理器在 `UpsertAppliedState` 中 SHALL 自动处理 `import.` 地址前缀剥离，并支持将执行失败但已产生非空云上状态的半成品实例安全落盘。

#### Scenario: Import 节点自动剥离前缀并按 resource 落库
- **WHEN** 调用 `UpsertAppliedState` 时传入以 `"import."` 开头的 `BlockID`（如 `"import.tencentcloud_vpc.existing"`）
- **THEN** 系统剥离 `"import."` 前缀，以目标地址 `"tencentcloud_vpc.existing"` 且 `Kind = "resource"` 写入 `iac_state`

#### Scenario: 半成品失败状态写回
- **WHEN** 上游节点执行失败但返回了非空 `StateObject`（`state != null`）并调用 `UpsertAppliedState`
- **THEN** 系统按与常规成功写回一致的规则持久化该半成品状态，供后续重试或清理复用

### Requirement: 销毁与遗忘状态清理
状态管理器 SHALL 提供 `RemoveDestroyedState(kt, req)` 方法，在资源销毁或数据源遗忘后按 `DeposedKey` 精确清理目标实例状态，并在 `current` 与 `deposed` 均为空时删除整行记录。

#### Scenario: 按 DeposedKey 移除待销毁旧实例
- **WHEN** 记录同时存在 `Current` 与 `Deposed["k-1"]`，以 `DeposedKey="k-1"` 调用 `RemoveDestroyedState`
- **THEN** 系统仅移除 `Deposed["k-1"]`，`Current` 与其余 `Deposed` 条目保持不变，并更新 `LastFlowID`

#### Scenario: 移除最后一个 Deposed 且 Current 为空时删除整行
- **WHEN** 记录的 `Current == nil` 且 `Deposed` 仅含 `"k-1"`，以 `DeposedKey="k-1"` 调用 `RemoveDestroyedState`
- **THEN** 系统彻底删除该行 `iac_state` 记录

#### Scenario: 销毁当前实例且 Deposed 为空时删除整行
- **WHEN** 记录的 `Deposed` 为空字典 `{}`，以空 `DeposedKey` 调用 `RemoveDestroyedState`
- **THEN** 系统彻底删除该行 `iac_state` 记录

#### Scenario: 销毁当前实例但仍有残留 Deposed 时保留行记录
- **WHEN** 记录的 `Deposed` 非空，以空 `DeposedKey` 调用 `RemoveDestroyedState`
- **THEN** 系统仅将该行 `Current` 置为 `NULL` 并更新 `LastFlowID`，保留 `Deposed` 中的残留待销毁旧实例

### Requirement: 状态管理器参数校验
状态管理器在执行读写操作前 MUST 校验入参合法性，拒绝空关键标识或非法枚举值。

#### Scenario: 缺少必填参数或非法枚举被拦截
- **WHEN** `repositoryID`、`BlockID`、`PluginInstanceID`、`FlowID` 或 `Object` 为空，或 `Kind` 非 `resource`/`data`
- **THEN** 系统返回 `InvalidParameter`（`2000001`）错误，不发起下游数据库写操作
