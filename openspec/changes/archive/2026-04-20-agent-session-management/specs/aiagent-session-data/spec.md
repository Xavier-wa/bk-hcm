## ADDED Requirements

### Requirement: aiagent_session 表结构定义

系统 SHALL 在 MySQL 中提供 `aiagent_session` 表，包含以下字段：
- `id` (VARCHAR(64), PK)：由 id_generator 生成的 8 位 36 进制字符串
- `session_code` (VARCHAR(128), UNIQUE, NOT NULL)：对外会话标识，格式 `{md5}-{YYYYMMDDHH}`
- `session_name` (VARCHAR(255), NOT NULL, DEFAULT '')：用户可编辑的会话名称
- `app_name` (VARCHAR(64), NOT NULL)：应用名称，参与 session_code 生成，支持按应用维度管理会话
- `user` (VARCHAR(64), NOT NULL)：会话所属用户
- `thread_id` (VARCHAR(128), NOT NULL)：框架 threadId，值等于 `id`
- `is_temporary` (TINYINT(1), NOT NULL, DEFAULT 0)：是否为临时会话
- `session_content_count` (INT UNSIGNED, NOT NULL, DEFAULT 0)：会话消息计数
- `extensions` (JSON, NULL)：扩展字段
- `creator` (VARCHAR(64), NOT NULL)：创建者
- `revisor` (VARCHAR(64), NOT NULL)：最近修改者
- `created_at` (TIMESTAMP(6), NOT NULL)：创建时间
- `updated_at` (TIMESTAMP(6), NOT NULL)：更新时间

索引：`uk_session_code`(UNIQUE)、`idx_app_user`(app_name, user)、`idx_creator`、`idx_created_at`。

#### Scenario: 表结构正确创建

- **WHEN** 执行 DDL 迁移脚本
- **THEN** aiagent_session 表 MUST 包含上述所有字段及索引（含 app_name 字段和 idx_app_user 联合索引），引擎为 InnoDB，字符集为 utf8mb4

### Requirement: session_code 自动生成

data-service 在创建会话记录时 SHALL 在同一事务内自动生成 `session_code`，规则为 `md5(app_name + user + thread_id) + "-" + YYYYMMDDHH`，其中 `thread_id` 等于 `id`。

#### Scenario: 创建会话时自动生成 session_code

- **WHEN** data-service 收到创建会话请求，包含 app_name="agent-server"、user="pandafyang"
- **THEN** 系统 MUST 生成 id（8 位 36 进制），将 id 赋值给 thread_id，计算 session_code = md5("agent-server" + "pandafyang" + id) + "-" + 当前时间的 YYYYMMDDHH 格式
- **THEN** 响应 MUST 返回 id、session_code、thread_id

### Requirement: data-service 创建会话接口

data-service SHALL 提供 `POST /api/v1/data/aiagent/sessions/create` 接口创建会话记录。请求体包含 `app_name`、`user`、`session_name`、`is_temporary`、`extensions`、`creator`。

#### Scenario: 成功创建会话

- **WHEN** 收到合法创建请求
- **THEN** 系统 MUST 在事务内生成 id、thread_id、session_code 并写入数据库
- **THEN** 响应 MUST 返回 `{ id, session_code, thread_id }`

#### Scenario: 缺少必填字段

- **WHEN** 请求体缺少 app_name 或 user
- **THEN** 系统 MUST 返回错误响应

### Requirement: data-service 更新会话接口

data-service SHALL 提供 `PATCH /api/v1/data/aiagent/sessions` 接口更新会话。仅允许修改 `session_name`，请求体包含 `id`、`session_name`、`revisor`。

#### Scenario: 成功更新会话名称

- **WHEN** 收到包含有效 id 和 session_name 的更新请求
- **THEN** 系统 MUST 更新 session_name 和 revisor 字段

#### Scenario: 更新不存在的会话

- **WHEN** 请求中的 id 不存在
- **THEN** 系统 MUST 返回未找到错误

### Requirement: data-service 列表查询接口

data-service SHALL 提供 `POST /api/v1/data/aiagent/sessions/list` 接口，支持 HCM 标准 filter + page 模式查询会话列表。

#### Scenario: 按用户查询会话列表

- **WHEN** 收到 filter 条件 `creator = "pandafyang"`，page 参数 `start=0, limit=20, sort=created_at, order=DESC`
- **THEN** 系统 MUST 返回该用户的会话列表（包含 count 和 details），按 created_at 降序排列

#### Scenario: 空结果查询

- **WHEN** filter 条件匹配不到任何记录
- **THEN** 系统 MUST 返回 `{ count: 0, details: [] }`

### Requirement: data-service 批量删除接口

data-service SHALL 提供 `DELETE /api/v1/data/aiagent/sessions/batch` 接口，通过 filter 条件批量删除会话记录。

#### Scenario: 成功批量删除

- **WHEN** 收到 filter 条件 `id in ["0000abcd", "0000abce"]`
- **THEN** 系统 MUST 删除匹配的记录

### Requirement: data-service 消息计数递增接口

data-service SHALL 提供 `PATCH /api/v1/data/aiagent/sessions/incr-content-count` 接口，通过 `session_code` 原子递增 `session_content_count` 字段。

#### Scenario: 成功递增消息计数

- **WHEN** 收到包含有效 session_code 的请求
- **THEN** 系统 MUST 执行 `SET session_content_count = session_content_count + 1`，利用 SQL 原子操作避免并发问题

#### Scenario: session_code 不存在

- **WHEN** 请求中的 session_code 不存在
- **THEN** 系统 MUST 返回未找到错误

### Requirement: DAO 层 CRUD 实现

pkg/dal/dao/aiagent/ 目录 SHALL 提供 `AiagentSession` DAO 接口与实现，包含 Create、Update、List、Delete、IncrContentCount 方法。DAO 实现 MUST 遵循 HCM 项目已有的 DAO 模式。

#### Scenario: DAO Create 方法

- **WHEN** 调用 Create 方法传入合法的 session 数据
- **THEN** MUST 写入数据库并返回完整记录

#### Scenario: DAO List 方法支持分页

- **WHEN** 调用 List 方法传入 filter 和 page 参数
- **THEN** MUST 返回匹配记录列表及总数

### Requirement: 表名常量注册

系统 SHALL 在 `pkg/dal/table/table.go` 中新增 `AiagentSessionTable Name = "aiagent_session"` 常量，并在 `pkg/dal/dao/dao.go` 的 `dao.Set` 中注册 `AiagentSession()` DAO。

#### Scenario: 表名常量可引用

- **WHEN** 代码引用 `table.AiagentSessionTable`
- **THEN** MUST 返回 `"aiagent_session"` 字符串
