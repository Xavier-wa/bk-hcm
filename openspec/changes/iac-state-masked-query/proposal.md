# Proposal

## Why

IaC 编排引擎在 Plan 对账与 Apply 执行阶段需要持久化记录云上真实受管资源现状（通过 `current` 保存当前生效实例、`deposed` 暂存待销毁旧实例，以支持先建后删替换与中断补删，并通过 `import_from_spec_id` 隔离导入暂存态），同时业务运维与研发人员需要安全、只读地查询受管资源列表与脱敏后的单实例状态详情，避免明文密钥泄漏。本变更同步落地 `iac_repository`、`iac_spec`、`iac_spec_node` 与 `iac_state` 四张核心表的存储与 `data-service` 批量接口，并在其上交付 `pkg/iacstate` 公共状态管理器与 `iac-server` 只读脱敏查询接口，为上层仓库管理、Spec 上传、DAG 编排与 Apply 执行提供完整数据底座。

## What Changes

- 新增 `iac_repository`、`iac_spec`、`iac_spec_node`、`iac_state` 四张 MySQL 表结构及 `id_generator` 初始化脚本（全部启用多租户 `tenant_id` 隔离；`iac_state` 严格按方案建表，不冗余 `bk_biz_id` 列）。
- 在 `data-service` 层新增上述四张表的表模型、DAO（`BatchCreateWithTx`、`UpdateWithTx`、`DeleteWithTx`、`List`）、单数路由批量接口（`/iac_repository/...`、`/iac_spec/...`、`/iac_spec_node/...`、`/iac_state/...`）与对应的 `data-service` 客户端封装（`IacRepositoryClient`、`IacSpecClient`、`IacSpecNodeClient`、`IacStateClient`）。
- 新增公共包 `pkg/iacstate`，定义 `StateRecord`、`StateObject` 核心领域模型与无状态结构体 `StateManager`（通过 `NewStateManager` 构造），提供 `GetRepositoryStates`、`GetDependencyStates`、`UpsertAppliedState`、`RemoveDestroyedState` 四个标准方法，统一收口 `current`/`deposed` 状态流转、`import.` 前缀剥离、半成品失败写回与暂存态默认过滤逻辑。
- 在 `cmd/iac-server` 新增两个仓库维度只读 State 查询接口：
  - `POST /api/v1/iac/repository/{repository_id}/state/list`（`ListState`）：校验 `iac_repository` 存在性与租户归属后，分页返回受管实例摘要（不含 `state`、`private`、`extra`），默认过滤暂存态。
  - `GET /api/v1/iac/repository/{repository_id}/state/{state_id}`（`GetState`）：校验 `iac_repository` 与记录归属后，返回脱敏后的完整 `current` 与 `deposed`，通过查询 `iac_plugin` 的 `enabled` 版本定位 `iac_plugin_schema`，递归将 `sensitive = true` 属性替换为 `"(sensitive value)"` 并剥离 `private` 字段；插件或 Schema 不可用时执行 Fail-Closed 拦截，绝不返回明文。

## Capabilities

### New Capabilities
- `iac-repository-spec-store`: 管理 `iac_repository`、`iac_spec`、`iac_spec_node` 三表的持久化存储（表结构、DAO、`data-service` 单数路由批量接口与客户端）。
- `iac-state-manager`: 管理 `iac_state` 持久化存储（表结构、DAO、`data-service` 批量接口与客户端）及 `pkg/iacstate.StateManager` 状态读写状态机（`current`/`deposed` 状态流转、先建后删、孤儿销毁与遗忘、`import.` 去前缀、半成品写回与暂存态隔离）。
- `iac-state-query`: 提供 `iac-server` 对外只读 State 摘要列表查询（`ListState`）与基于插件 Schema 的 Fail-Closed 敏感字段递归脱敏详情查询（`GetState`）。

### Modified Capabilities
- （无：本变更不修改任何既有能力的需求规范）

## Impact

- **数据库与持久化层**：
  - `scripts/sql/9999_20261126_iac_repository_spec_state.sql`（新增 `iac_repository`、`iac_spec`、`iac_spec_node`、`iac_state` 建表与 `id_generator` 初始化 SQL）
  - `pkg/dal/table/table.go`、`pkg/dal/table/iac/`、`pkg/dal/dao/iac/`、`pkg/dal/dao/`、`pkg/dal/dao/types/`
- **枚举与常量**：
  - `pkg/criteria/enumor/`（新增 `IacStateKind`、`IacSpecStatus`、`IacSpecNodeType` 枚举及 `Validate()`）
  - `pkg/criteria/constant/`（新增敏感值脱敏占位符常量 `"(sensitive value)"`、`import.` 前缀常量等）
- **API 协议与客户端**：
  - `pkg/api/core/iac/`、`pkg/api/data-service/iac/`、`pkg/api/hc-service/`、`pkg/api/iac-server/`
  - `pkg/client/data-service/global/`（新增 `IacRepositoryClient`、`IacSpecClient`、`IacSpecNodeClient`、`IacStateClient`）
  - `pkg/client/hc-service/`（新增插件 Schema 查询客户端方法）
- **公共状态管理库**：
  - `pkg/iacstate/`（核心模型、`StateManager` 无状态结构体实现、单元测试）
- **微服务路由与逻辑**：
  - `cmd/data-service/service/iac/`（注册 `iac_repository`、`iac_spec`、`iac_spec_node`、`iac_state` 批量 CRUD 与列表查询接口）
  - `cmd/hc-service/service/`（基于既有 `SchemaManager` 暴露插件 Schema 查询内部接口）
  - `cmd/iac-server/service/`（注册 `ListState` / `GetState` 路由及经 `hc-service` 获取 Schema 的脱敏查询逻辑）
- **依赖影响**：
  - 复用既有 `iac_plugin`、`iac_plugin_schema` 的 `data-service` 客户端与 `github.com/hashicorp/terraform-json` 的 `ProviderSchema` / `SchemaBlock` 类型，不新增外部第三方依赖。
