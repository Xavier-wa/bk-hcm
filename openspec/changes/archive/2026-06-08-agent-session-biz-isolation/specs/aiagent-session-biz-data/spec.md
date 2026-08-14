## ADDED Requirements

### Requirement: aiagent_session 表 bk_biz_id 字段

系统 SHALL 在 `aiagent_session` 表新增 `bk_biz_id` 字段：`BIGINT NOT NULL DEFAULT -1`，表示会话所属蓝鲸业务 ID；`-1` 表示未分配业务（`constant.UnassignedBiz`）。

#### Scenario: DDL 迁移成功

- **WHEN** 执行迁移脚本
- **THEN** `aiagent_session` 表 MUST 包含 `bk_biz_id` 列，默认值为 `-1`，引擎 InnoDB，字符集 utf8mb4

#### Scenario: 存量数据默认值

- **WHEN** 迁移完成且表中存在历史会话记录
- **THEN** 所有存量记录的 `bk_biz_id` MUST 为 `-1`

### Requirement: aiagent_session 业务列表索引

系统 SHALL 新增联合索引 `idx_app_user_biz`（`app_name`, `user`, `bk_biz_id`），用于优化按业务维度的会话列表查询。现有 `idx_app_user` 索引 MUST 保留。

#### Scenario: 索引创建

- **WHEN** 执行 DDL 迁移脚本
- **THEN** 表 MUST 存在 `idx_app_user_biz(app_name, user, bk_biz_id)` 索引

### Requirement: session_code 生成规则不变

data-service 创建会话时 `session_code` 生成规则 MUST 保持不变：`md5(app_name + user + thread_id) + "-" + YYYYMMDDHH`；`bk_biz_id` MUST NOT 参与 `session_code` 计算。

#### Scenario: 业务会话 session_code 与改造前公式一致

- **WHEN** 以 `app_name="hcm-agent"`、`user="alice"` 创建 `bk_biz_id=123` 的会话
- **THEN** 生成的 `session_code` MUST 等于 `md5(app_name + user + thread_id) + "-" + YYYYMMDDHH`，与相同 app/user/thread 但无 bk_biz_id 列时的公式一致

### Requirement: data-service 创建会话支持 bk_biz_id

data-service `POST /api/v1/data/aiagent/sessions/create` SHALL 接受可选请求字段 `bk_biz_id`（int64）。未传时 MUST 默认 `-1`；传入时 MUST 原样持久化至 `aiagent_session.bk_biz_id`，其中显式传入 `0` MUST 按 `0` 处理，不做特殊转换。

#### Scenario: 创建业务会话记录

- **WHEN** 请求体包含 `bk_biz_id: 123` 及其他必填字段
- **THEN** 系统 MUST 在事务内创建记录，且 `bk_biz_id=123`

#### Scenario: 未传 bk_biz_id 时默认未分配

- **WHEN** 请求体不包含 `bk_biz_id`
- **THEN** 系统 MUST 写入 `bk_biz_id=-1`

#### Scenario: 显式传入 0 时按 0 写入

- **WHEN** 请求体显式包含 `bk_biz_id: 0`
- **THEN** 系统 MUST 写入 `bk_biz_id=0`

### Requirement: data-service 列表与查询返回 bk_biz_id

data-service 会话 List/查询接口 SHALL 在响应 `details` 中包含 `bk_biz_id` 字段；SHALL 支持按 `bk_biz_id` 字段过滤。

#### Scenario: 按 bk_biz_id 过滤列表

- **WHEN** List 请求 filter 包含 `bk_biz_id = 123`
- **THEN** 响应 details MUST 仅包含 `bk_biz_id=123` 的记录

#### Scenario: 单条查询返回 bk_biz_id

- **WHEN** 按 `session_code` 查询到一条会话
- **THEN** 响应 MUST 包含该记录的 `bk_biz_id` 值

### Requirement: DAO 与表结构定义扩展

`pkg/dal/table/aiagent`、`pkg/dal/dao/aiagent`、`pkg/api/data-service/aiagent` 及 `pkg/client/data-service/aiagent` SHALL 贯通 `bk_biz_id` 字段的读写，INSERT/SELECT/UPDATE 语句 MUST 包含该列。

#### Scenario: DAO 创建写入 bk_biz_id

- **WHEN** DAO `CreateWithTx` 接收 `SessionTable.BkBizID=456`
- **THEN** INSERT 语句 MUST 将 `bk_biz_id=456` 写入数据库
