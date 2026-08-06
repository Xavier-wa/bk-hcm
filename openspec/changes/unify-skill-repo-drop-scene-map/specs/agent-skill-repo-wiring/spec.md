## ADDED Requirements

### Requirement: 全进程单一 skill 仓库实例

agent-server SHALL 在整个进程内只持有一个 skill 仓库实例，即 `skill.Manager.Repository`（由 `BuildFSRepo` 基于 `tools.skills.root` 与 `extraDirs` 构造的 `FSRepository`）。系统 MUST NOT 按场景（`enumor.IntentType`）、按 subagent 或按会话创建或映射出多个 skill 仓库实例。

BKAIDev 同步对该实例的 `Refresh()` 热更新（见 `agent-skill-sync`）MUST 对所有 subagent 同时生效。

#### Scenario: 所有 subagent 共享同一仓库实例

- **GIVEN** agent-server 以 graph 模式启动，主图注册了 host_apply 与 resource_query 两个 subagent
- **WHEN** 系统构建这两个子图
- **THEN** 两个子图的 skill 工具（`skill_load` / `skill_list_docs` / `skill_select_docs`）与 prompt 注入回调 MUST 收到同一个 `skillpkg.Repository` 实例
- **AND** 该实例 MUST 是 `skill.Manager.Repository`，中间 MUST NOT 存在按场景选择仓库的映射结构

#### Scenario: 新增场景无需注册即可获得 skill 能力

- **GIVEN** 主图新增了一个 subagent（例如新的业务场景子图）
- **WHEN** 该子图按现有方式接收 skill 仓库参数
- **THEN** 它 MUST 直接获得与既有子图相同的仓库实例
- **AND** 系统 MUST NOT 要求开发者在任何场景注册表中额外登记该场景

#### Scenario: skill 热更新对所有 subagent 同时可见

- **GIVEN** BKAIDev 定时同步安装了一个新 skill 并调用了 `Refresh()`
- **WHEN** 任意 subagent 在下一轮对话中列出可用 skill
- **THEN** 所有 subagent MUST 都能看到该新 skill，不存在某个场景仓库未刷新的情况

### Requirement: skill 仓库缺失时启动失败

agent-server 在构建 AGUI runner 时 SHALL 校验 skill 仓库可用性：当 `skill.Manager` 为 nil 或其 `Repository` 为 nil 时，系统 MUST 记录 error 日志并返回错误终止启动。系统 MUST NOT 以「仓库为 nil」的降级状态继续提供对话服务。

该校验 MUST 在按运行模式（graph / LLM）分支之前统一执行一次，两种模式 MUST 使用同一个仓库实例。

#### Scenario: 仓库为 nil 时启动失败

- **GIVEN** skill 配置中 `root` 与 `extraDirs` 均为空，`BuildFSRepo` 返回 nil 仓库
- **WHEN** 系统构建 AGUI runner
- **THEN** 系统 MUST 输出包含 rid 之外上下文的 error 日志并返回错误
- **AND** agent-server MUST 启动失败，而不是启动成功但所有场景都没有 skill 能力

#### Scenario: 仓库正常时两种运行模式取同一实例

- **GIVEN** skill 仓库构造成功
- **WHEN** agent-server 分别以 graph 模式和 LLM 模式启动
- **THEN** graph 模式的子图与 LLM 模式的 agent MUST 都使用 `skill.Manager.Repository`
- **AND** 系统 MUST NOT 在任一分支内重复做 nil 判空降级

### Requirement: skill 仓库与工具代理的场景隔离策略相互独立

skill 仓库的「不按场景隔离」SHALL NOT 影响 `toolproxy.ToolProxies` 的按场景隔离。工具代理按场景裁剪可见工具集是既有且必要的行为，MUST 保持不变。

#### Scenario: 子图仍使用各自场景的工具代理

- **GIVEN** 主图构建 host_apply 与 resource_query 子图
- **WHEN** 系统注入工具代理
- **THEN** 每个子图 MUST 仍通过 `proxies.SceneProxy(<该场景>)` 获得其独有的 `ToolProxy`
- **AND** 同时 MUST 共享同一个 skill 仓库实例
