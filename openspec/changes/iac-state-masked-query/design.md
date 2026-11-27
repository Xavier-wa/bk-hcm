# Design

## Context

`bk-hcm` 已具备 `iac_plugin`、`iac_plugin_instance`、`iac_plugin_schema` 三张表的 `data-service` 批量 CRUD 能力，且 `cmd/iac-server` 服务骨架（路径前缀 `/api/v1/iac`）已就绪。本变更落地 `iac_repository`、`iac_spec`、`iac_spec_node`、`iac_state` 四张表的持久化与 `data-service` 批量接口，并在其上实现 `pkg/iacstate` 无状态状态管理器与 `iac-server` 对外只读脱敏查询接口（动机见 `proposal.md`，行为契约见 `specs/`）。

## Goals / Non-Goals

**Goals:**
- 落地 `iac_repository`、`iac_spec`、`iac_spec_node`、`iac_state` 四张 MySQL 表结构、DAO、`data-service` 单数路由批量接口及对应客户端。
- 在 `pkg/iacstate` 提供无状态 `StateManager` 公共结构体，供 `iac-server` 与 `task-server` 统一调用，完整实现 `current` 与 `deposed` 实例状态流转、依赖状态提取、`import.` 去前缀、半成品写回与暂存态隔离。
- 在 `cmd/iac-server` 实现 `ListState` 与 `GetState` 只读接口（含 `iac_repository` 存在性校验），并通过 `iac_plugin`（`enabled` 版本）与 `hc-service` 的插件 Schema 查询接口实现 Fail-Closed 递归脱敏与 `private` 剥离。
- 通过合理索引与 `hc-service` 既有 `SchemaCache`，保证单仓库 2000 条记录、50 并发下 `ListState` 与 `GetState` 的 P99 响应时间低于 500ms。

**Non-Goals:**
- 不包含 `iac-server` 侧的仓库 CRUD 接口（`CreateRepository`/`UpdateRepository`/`ListRepository`/`DeleteRepository`）与 Spec 上传/浏览/执行接口（`UploadSpec`/`ListSpec`/`GetSpec`/`ListSpecNodes`/`GetSpecNodeFile`/`ExecuteSpec`）。
- 不包含 `iac_run` 建表、Plan/Apply DAG 编排、异步 Flow 调度与中断恢复逻辑。
- 不包含空状态仓库初始导入 `.tfstate` 逻辑（本变更仅预留 `import_from_spec_id` 列与默认过滤语义）。
- `iac_state` 表不冗余 `bk_biz_id` 列，业务归属统一维护在 `iac_repository.bk_biz_id`。

## Decisions

### 1. 四张核心表结构与多租户隔离
- **决策**：仓库实体统一使用全称 `repository`（不缩写为 `repo`），在单份 SQL 脚本 `scripts/sql/9999_20261126_iac_repository_spec_state.sql` 中创建四张表并初始化对应 `id_generator` 记录，均在 `pkg/dal/table/table.go` 注册并开启 `EnableTenant: true`：
  - `iac_repository`：主键 `id`、`bk_biz_id`、`name`、`latest_version`、`last_exec_version`、`memo`、`tenant_id` 及审计列；`UNIQUE KEY idx_uk_tenant_biz_name (tenant_id, bk_biz_id, name)`。
  - `iac_spec`：主键 `id`、`bk_biz_id`、`repository_id`、`version`、`status`（`uploading`/`uploaded`/`upload_failed`）、`last_run_id`、`memo`、`tenant_id` 及审计列；`UNIQUE KEY idx_uk_repository_version (repository_id, version)`、`KEY idx_tenant_biz_repository (tenant_id, bk_biz_id, repository_id)`。
  - `iac_spec_node`：主键 `id`、`repository_id`、`spec_id`、`parent_id`、`level`、`node_type`（`dir`/`file`）、`node_name`、`children_node_ids`（`json DEFAULT NULL`）、`file_size`、`content`（`mediumtext DEFAULT NULL`）、`tenant_id` 及审计列；`KEY idx_spec_level (spec_id, level)`、`KEY idx_repository_id (repository_id)`。
  - `iac_state`：主键 `id`、`repository_id`、`block_id`、`plugin_instance_id`、`kind`（`resource`/`data`）、`current`（`json DEFAULT NULL`）、`deposed`（`json NOT NULL`）、`last_flow_id`、`import_from_spec_id`、`tenant_id` 及审计列；`UNIQUE KEY idx_uk_repository_block (repository_id, block_id)`、`KEY idx_tenant_repository (tenant_id, repository_id)`、`KEY idx_repository_import_spec (repository_id, import_from_spec_id)`。
- **备选方案与取舍**：不把 `bk_biz_id` 冗余到 `iac_state` 中，而是同步落库 `iac_repository`，由 `iac_repository` 作为业务归属单一事实源；同时 `iac_spec` 与 `iac_spec_node` 一并落库，保证仓库与版本数据层完整对齐。

### 2. DAO、`data-service` 批量接口与 `current` 显式置 `NULL` 机制
- **决策**：四张表统一遵循 `bk-hcm` 规范：DAO 层提供 `BatchCreateWithTx`、`UpdateWithTx`、`DeleteWithTx`、`List`；`data-service` 层统一注册单数路由批量接口（`/iac_repository/...`、`/iac_spec/...`、`/iac_spec_node/...`、`/iac_state/...`）并在 `Validate()` 中校验 `len(Items) <= constant.BatchOperationMaxLimit`；`iac_repository` 更新时忽略不可变列 `bk_biz_id`；`iac_spec_node` 列表查询支持 `fields` 投影跳过 `content`。
- **`iac_state.current` 显式置 `NULL` 与查询返回闭环方案（零侵入 ORM 框架）**：
  1. **辅助方法（`pkg/dal/table/types`）**：在既有 `types.JsonField` 上补充只读方法 `IsNull() bool`（判断 `f == "null"`），不改动 `Value()` 与 `RearrangeSQLDataWithOption`，零影响存量表。
  2. **表模型与更新 DTO**：`IacStateTable.Current` 与 `IacStateUpdateField.Current` 统一沿用全仓标准的 `types.JsonField`：
     - `""`（未传）：`RearrangeSQLDataWithOption` 视为空值自动跳过，不修改 `current`；
     - `"null"`（显式清空）：`RearrangeSQLDataWithOption` 正常生成 `current = :current`，`IacStateDao.UpdateWithTx` 检测到 `model.Current.IsNull()` 时将 `toUpdate["current"] = nil`，由 `sqlx` 绑定写入真正的 SQL `NULL`；
     - `"{...}"`（有效对象）：正常覆写 JSON 对象。
     - `Deposed` 为 `json NOT NULL`，清空时传 `types.JsonField("{}")` 走常规非空更新即可；`ImportFromSpecID` 使用 `*string` + `AddBlankedFields("import_from_spec_id")`（与全仓 `Memo *string` 一致）支持显式清空为 `""`。
  3. **查询返回 DTO（`coreiac.IacState`）**：`Current` 使用 `*types.JsonField`（避免 `types.JsonField("")` 序列化为 `"{}"`）：`List` 转换时若 `len(row.Current) == 0 || row.Current.IsNull()` 则置为 `nil`（序列化为 `"current": null`），否则取 `&row.Current`；`iac_spec_node` 的可空列 `children_node_ids` 与 `content` 在查询返回 DTO 中同样使用指针保持 SQL `NULL` 语义。

### 3. `pkg/iacstate` 无状态 `StateManager` 结构体封装
- **决策**：在 `pkg/iacstate` 中定义 `StateRecord`、`StateObject`、`UpsertAppliedReq`、`RemoveDestroyedReq`，并直接导出无状态结构体 `StateManager` 与构造函数 `NewStateManager(dataCli *dataservice.Client) *StateManager`（遵循 Go “接收接口、返回结构体”习惯，不预先导出大接口；各调用方如需单测 mock，在调用方包内按需定义最小消费接口即可）。内部仅持有 `*dataservice.Client`（无本地状态）：
  - `GetRepositoryStates`：固定附加 `import_from_spec_id = ""` 过滤条件；当 `blockIDs` 为空时按页循环拉取直至读完该仓库全部已生效记录；当 `blockIDs` 非空时按批次切片（每批 ≤ `constant.BatchOperationMaxLimit`）以 `IN` 条件查询并汇总；结果为空返回 `(nil, nil)`。
  - `GetDependencyStates`：复用 `GetRepositoryStates(kt, repositoryID, depBlockIDs)`，逐一检查每个 `depBlockID` 是否存在且 `Current != nil`；若存在缺失则汇总缺失的 `block_id` 返回明确错误；否则将 `Current.State` 反序列化为 `map[string]any` 返回（完全忽略 `Deposed`）。
  - `UpsertAppliedState`：先校验参数；若 `BlockID` 以 `"import."` 开头，剥离前缀并将 `Kind` 强制设为 `"resource"`；按 `(repository_id, block_id)` 查询既有记录：
    - 不存在：构造 `deposed = "{}"` 调用 `BatchCreate`；
    - 已存在且 `DeposedKey == ""`：覆写 `current`，保留原 `deposed`，更新 `plugin_instance_id` 与 `last_flow_id`；
    - 已存在且 `DeposedKey != ""`：若原 `current != nil`，将其移入 `deposed[DeposedKey]`，新 `Object` 写入 `current`，更新 `plugin_instance_id` 与 `last_flow_id`。
  - `RemoveDestroyedState`：查询目标 `(repository_id, block_id)` 生效记录（不存在时幂等返回 `nil`）：
    - `DeposedKey != ""`：删除 `deposed[DeposedKey]`；若此时 `current == nil && len(deposed) == 0` 则 `BatchDelete` 删整行，否则 `BatchUpdate` 更新 `deposed` 与 `last_flow_id`；
    - `DeposedKey == ""`：若 `len(deposed) == 0` 则 `BatchDelete` 删整行，否则 `BatchUpdate` 传 `Current: types.JsonField("null")` 显式置 `current = NULL` 并更新 `last_flow_id`。

### 4. `GetState` 插件 Schema 定位（经 `hc-service` 获取）与递归脱敏
- **决策**：
  - **经 `hc-service` 获取 Schema（复用 `SchemaManager` 缓存）**：单个实例对象（`StateObject`）仅记录 `plugin`（插件名）与 `type_name`。在 `cmd/hc-service` 基于既有 `SchemaManager.GetSchema` 暴露内部查询接口，并在 `pkg/client/hc-service` 封装对应客户端方法。`iac-server` 调用 `data-service` `IaCPlugin.List`（过滤 `plugin_name = plugin AND state = "enabled"`）获取当前可用 `plugin_version`（查无 `enabled` 记录或下游异常直接返回错误），随后调用 `hc-service` 接口获取 `*coreplugin.PluginSchema`（命中 `hc-service` 进程内 `SchemaCache`，`iac-server` 侧暂不重复维护本地缓存，后续按压测或线上表现再按需扩展）。
  - **递归脱敏算法**：根据 `record.Kind` 分别从 `ProviderSchema.ResourceSchemas[obj.TypeName]`（`resource`）或 `ProviderSchema.DataSourceSchemas[obj.TypeName]`（`data`）获取 `*tfjson.Schema`，未找到则 Fail-Closed 报错。以 `schema.Block` 递归遍历反序列化后的 `state` JSON 树：
    1. 遍历 `Block.Attributes`：若 `attr.Sensitive == true`，且目标键存在于当前对象中，直接将其值替换为常量 `"(sensitive value)"`；若非敏感但 `attr.AttributeNestedType != nil`，按 `NestingMode`（`single` / `list` / `set` / `map`）递归处理子对象的 `Attributes`。
    2. 遍历 `Block.NestedBlocks`：按 `NestingMode`（`single` / `group` / `list` / `set` / `map`）递归调用 `maskByBlock` 处理每个子块对象。
    3. 返回 DTO 中不包含 `private` 字段，`extra` 字段不进入脱敏器直接原样透传。
  - **Fail-Closed 保障**：脱敏器在任意步骤（查询插件、调用 `hc-service` 获取 Schema、反序列化 JSON、找不到 `type_name`）出错时立即中止并返回错误，响应体中不输出任何实例明文。

### 5. `ListState` / `GetState` 仓库归属校验、边界过滤与确定性摘要
- **决策**：
  - `ListState` 与 `GetState` 入口均先调用 `data-service` `IacRepository.List` 校验 `id == {path.repository_id}` 在当前租户下存在；查无记录返回 `RecordNotFound`（HTTP 404）。
  - `ListState` 在调用 `data-service` `IacState.List` 前，将 `repository_id == {path.repository_id}` 与 `import_from_spec_id == ""` 通过 `AND` 规则硬性包裹在调用方传入的 `Filter` 之外，杜绝过滤条件绕过。
  - 摘要转换时：`has_current` 为 `record.Current != nil`；`deposed_keys` 对 `record.Deposed` 键做字典序升序排序，`deposed_count = len(deposed_keys)`；`plugin_instance_id` 取行字段 `record.PluginInstanceID`；`plugin` 与 `type_name` 优先取 `record.Current`，若 `Current == nil` 且 `len(deposed_keys) > 0` 则取 `record.Deposed[deposed_keys[0]]`。

## Risks / Trade-offs

- **[并发读-改-写同一 `(repository_id, block_id)`]** → 同一仓库的 Apply 任务流由编排器按拓扑依赖调度，同一 `block_id` 的 `#apply` 与 `#destroy` 不会并发执行；数据库唯一索引 `idx_uk_repository_block` 防止重复行插入。
- **[`ListState` 读取 JSON 列带宽开销]** → 依托 `idx_repository_import_spec` 索引与标准分页上限（单页 ≤ 500）控制扫描范围；若未来单实例状态 JSON 体积进一步增大，可在 `data-service` 侧补充 SQL JSON 函数投影摘要字段，无需改动对外接口契约。
- **[`GetState` 经 `hc-service` 跨服务拉取大体积 Schema]** → `hc-service` 侧直接命中进程内 `SchemaCache` 避免 DB 压力；若后续 50 并发压测显示跨服务传输或 JSON 反序列化成为瓶颈，再在 `iac-server` 侧按需补充短 TTL 缓存。

## Migration Plan

1. 执行新增 SQL 脚本 `scripts/sql/9999_20261126_iac_repository_spec_state.sql`，创建 `iac_repository`、`iac_spec`、`iac_spec_node`、`iac_state` 四张表并初始化对应 `id_generator` 记录。
2. 先发布 `data-service`（承载四张表的批量 CRUD 接口）与 `hc-service`（暴露插件 Schema 查询内部接口），再发布 `iac-server`（承载 `ListState` / `GetState` 只读接口）。
3. 回滚方案：本变更全为新增表与新增接口，无既有表结构变更；回滚时下线新路由即可，不影响存量业务。
