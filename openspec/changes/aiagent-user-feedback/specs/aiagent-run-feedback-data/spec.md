## ADDED Requirements

### Requirement: aiagent_run_feedback 表结构

系统 SHALL 在 MySQL 中提供 `aiagent_run_feedback` 表，引擎 InnoDB，字符集 utf8mb4。建表脚本 MUST 落在 `scripts/sql/9999_<date>_aiagent_run_feedback.sql`。系统 MUST NOT 增加压制表 `aiagent_feedback_pref`。系统 MUST NOT 在本表存储 `session_code`。

表 MUST 包含：

- `id` VARCHAR(64) 主键
- `run_id` VARCHAR(64) NOT NULL，唯一
- `session_id` VARCHAR(64) NOT NULL（= `aiagent_session.id` = 框架 threadId）
- `user` VARCHAR(64) NOT NULL（提交用户，不可变）
- `bk_biz_id` BIGINT NOT NULL
- `tags` JSON，可空（字符串数组）
- `reaction` VARCHAR(16) NOT NULL：`like` / `dislike`
- `comment` VARCHAR(500)，可空
- `creator` / `reviser` / `created_at` / `updated_at`

索引 MUST 包含：`uk_run_id`、`idx_session_id`、`idx_user_created_at(user, created_at)`、`idx_reaction_created_at(reaction, created_at)`、`idx_updated_at`。

#### Scenario: 建表包含唯一约束与差评索引

- **GIVEN** 迁移脚本已执行
- **WHEN** 检查 `aiagent_run_feedback`
- **THEN** 表 MUST 存在且含 `uk_run_id`、`idx_reaction_created_at` 与 `idx_updated_at`
- **THEN** 表 MUST NOT 含 `session_code` 列

#### Scenario: 同一 run_id 不能插入两行

- **GIVEN** 已存在 `run_id=r1` 的一行
- **WHEN** 再插入相同 `run_id`
- **THEN** 数据库 MUST 因唯一约束拒绝

### Requirement: data-service 内部反馈 CRUD

data-service SHALL 提供内部接口：创建、按 id 更新、列表、按 filter 批量删除。除 data-service 外，系统 MUST NOT 直连 `aiagent_run_feedback`。列表 MUST 支持按 `run_id`、`session_id`、`user`、`bk_biz_id`、`reaction`、`created_at`、`updated_at` 过滤，以及 `tags` 的 `json_contains`。列表 MUST 支持按 `updated_at` / `created_at` 排序。更新 MUST NOT 允许改 `run_id`、`session_id`、`user`、`creator`。

#### Scenario: 创建反馈行

- **WHEN** data-service 收到合法创建请求（含 `run_id`、`session_id`、`user`、`bk_biz_id`、`reaction`）
- **THEN** MUST 插入一行并返回新 `id`

#### Scenario: 按 run_id 列出单行

- **GIVEN** 表中存在 `run_id=r1`
- **WHEN** 列表过滤 `run_id=r1`
- **THEN** details MUST 恰好 1 条且 `run_id=r1`

#### Scenario: 按 tags json_contains 过滤

- **GIVEN** 一行 tags 含 `factual_error`，另一行不含
- **WHEN** 列表过滤 `tags json_contains factual_error`
- **THEN** details MUST 仅含前者

#### Scenario: 禁止改提交用户

- **GIVEN** 已有反馈行 `user=alice`
- **WHEN** 更新请求携带新的 `user`
- **THEN** MUST 返回参数非法或忽略该字段且库中 `user` 仍为 `alice`
