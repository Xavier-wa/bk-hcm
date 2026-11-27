# Spec Delta

## Purpose

提供 IaC 仓库主表（`iac_repository`）、Spec 版本记录表（`iac_spec`）与版本目录和文件节点表（`iac_spec_node`）的持久化存储、DAO 与 `data-service` 批量数据访问接口，为后续仓库管理、Spec 三态分批上传、逐层懒加载浏览及 State 归属校验提供统一数据底座。

## ADDED Requirements

### Requirement: 仓库主表 iac_repository 持久化与唯一约束
系统 SHALL 在 `iac_repository` 表中持久化存储 IaC 仓库元数据，启用多租户 `tenant_id` 隔离，并以 `(tenant_id, bk_biz_id, name)` 作为唯一键；`latest_version` 与 `last_exec_version` 默认为空字符串 `""`。数据服务层 MUST 提供单数路由的批量创建、批量更新、批量删除与列表查询接口（`/iac_repository/batch/create`、`/iac_repository/batch/update`、`/iac_repository/batch`、`/iac_repository/list`）。

#### Scenario: 创建合法仓库记录成功
- **WHEN** 调用批量创建接口写入包含合法 `bk_biz_id`（`> 0`）与 `name` 的仓库记录
- **THEN** 系统生成主键 `id` 并落库，`latest_version` 与 `last_exec_version` 默认保存为空字符串 `""`

#### Scenario: 同租户同业务重名仓库写入被拒绝
- **WHEN** 同一 `(tenant_id, bk_biz_id)` 下已存在相同 `name` 的仓库记录，再次批量创建同名仓库
- **THEN** 系统返回数据重复错误（`RecordDuplicated`），不覆盖既有记录

#### Scenario: 更新仓库信息不允许修改 bk_biz_id
- **WHEN** 调用批量更新接口更新仓库的 `name`、`memo`、`latest_version` 或 `last_exec_version`
- **THEN** 系统仅更新允许变更的列，不可变列 `bk_biz_id` 保持原值不变

### Requirement: 版本记录表 iac_spec 持久化与三态约束
系统 SHALL 在 `iac_spec` 表中按 `(repository_id, version)` 唯一键持久化存储 Spec 版本元数据，启用多租户 `tenant_id` 隔离，将 `status` 限定为 `uploading`、`uploaded` 或 `upload_failed`，`last_run_id` 默认为空字符串 `""`。数据服务层 MUST 提供单数路由的批量创建、批量更新、批量删除与列表查询接口（`/iac_spec/batch/create`、`/iac_spec/batch/update`、`/iac_spec/batch`、`/iac_spec/list`）。

#### Scenario: 创建合法版本记录成功
- **WHEN** 调用批量创建接口写入包含合法 `bk_biz_id`、`repository_id`、`version` 与 `status`（`uploading`/`uploaded`/`upload_failed`）的记录
- **THEN** 系统生成主键 `id` 并成功落库，`last_run_id` 默认为空字符串 `""`

#### Scenario: 同仓库重复版本号或非法状态被拒绝
- **WHEN** 同一 `repository_id` 下已存在相同 `version` 的记录，或传入非法 `status` 枚举值
- **THEN** 重复版本号返回 `RecordDuplicated`，非法状态返回 `InvalidParameter`

### Requirement: 目录与文件节点表 iac_spec_node 持久化与投影查询
系统 SHALL 在 `iac_spec_node` 表中持久化存储 Spec 版本的目录与 `.tf` 文件节点，启用多租户 `tenant_id` 隔离，将 `node_type` 限定为 `dir` 或 `file`、`level` 限定为非负整数；目录节点（`dir`）的 `content` 为 `NULL`、`file_size` 为 `0`，文件节点（`file`）的 `children_node_ids` 为 `NULL`。数据服务层 MUST 提供单数路由的批量创建、批量更新、批量删除与支持 `fields` 投影的列表查询接口（`/iac_spec_node/batch/create`、`/iac_spec_node/batch/update`、`/iac_spec_node/batch`、`/iac_spec_node/list`）。

#### Scenario: 批量写入目录与文件节点成功
- **WHEN** 调用批量创建接口写入 `level = 0` 根目录节点（`node_type = "dir"`、`parent_id = ""`、`children_node_ids` 非空、`content = NULL`）及文件节点（`node_type = "file"`、`content` 非空、`children_node_ids = NULL`）
- **THEN** 系统批量生成节点主键 `id` 并成功落库

#### Scenario: 列表查询支持按 fields 投影跳过大字段
- **WHEN** 调用 `/iac_spec_node/list` 且在请求 `fields` 中排除 `content` 列
- **THEN** 系统仅查询并返回指定列，不加载 `content` 大文本内容

#### Scenario: 非法 node_type 或超限批量请求被拒绝
- **WHEN** 传入非 `dir`/`file` 的 `node_type`、负值 `level`，或单次批量操作条数超过系统上限
- **THEN** 系统返回参数校验错误（`InvalidParameter`），拒绝写入数据库
