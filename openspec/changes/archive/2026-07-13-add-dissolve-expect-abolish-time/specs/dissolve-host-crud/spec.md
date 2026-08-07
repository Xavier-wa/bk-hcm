## MODIFIED Requirements

### Requirement: 裁撤主机表 CRUD 接口

系统 SHALL 通过 data-service 暴露 `recycle_host_info` 表的批量创建、批量更新、列表查询、批量删除四个 HTTP 接口，供上层服务（woa-server）调用。`recycle_host_info` SHALL 包含字段 `project_id`、`region`、`bk_biz_id`、`group_id`、`operators`（JSON 数组）、`cpu_core`、`is_ignore`、`expect_abolish_time`（裁撤截止时间，字符串，格式 `yyyy-MM-dd`，默认空串），创建/更新接口 SHALL 支持写入这些字段，列表接口 SHALL 支持按这些字段过滤（`operators` 用 JSON 命中匹配）与返回。系统 MUST NOT 依赖 `asset_id` 唯一约束，允许同一 `asset_id` 在不同 `project_id` 下并存。

#### Scenario: 批量创建裁撤主机记录

- **WHEN** 调用 `POST /dissolve/recycle_hosts/batch/create`，传入包含 asset_id、inner_ip、module、abolish_phase、project_name、project_id、region、bk_biz_id、group_id、operators、cpu_core、is_ignore、expect_abolish_time 的主机列表
- **THEN** 系统 SHALL 为每条记录生成唯一 ID，在事务中批量插入 `recycle_host_info` 表，返回创建的 ID 列表，且不因相同 asset_id 报唯一约束冲突

#### Scenario: 列表查询裁撤主机

- **WHEN** 调用 `POST /dissolve/recycle_hosts/list`，传入 filter 表达式和分页参数
- **THEN** 系统 SHALL 按 filter 条件查询 `recycle_host_info` 表，支持按 asset_id、project_id、bk_biz_id、group_id、region、abolish_phase、is_ignore、expect_abolish_time 过滤及 operators JSON 命中过滤，返回符合条件的记录列表或记录总数（count 模式）

#### Scenario: 批量更新裁撤主机记录

- **WHEN** 调用 `PATCH /dissolve/recycle_hosts/batch`，传入 filter 表达式和更新字段（含 expect_abolish_time 等新增字段）
- **THEN** 系统 SHALL 在事务中按 filter 条件更新 `recycle_host_info` 表中匹配的记录

#### Scenario: 批量删除裁撤主机记录

- **WHEN** 调用 `DELETE /dissolve/recycle_hosts/batch`，传入 filter 表达式
- **THEN** 系统 SHALL 在事务中按 filter 条件删除 `recycle_host_info` 表中匹配的记录

## ADDED Requirements

### Requirement: 裁撤截止时间去重查询接口

系统 SHALL 通过 data-service 暴露按 `expect_abolish_time` 去重的查询接口，对 `recycle_host_info` 按 `expect_abolish_time` 分组去重、升序排序、全量返回非空的截止时间列表，供上层服务（woa-server）调用。查询 SHALL 支持传入 filter 表达式，并恒定排除 `expect_abolish_time` 为空串的记录。该接口 SHALL 在 `pkg/client/data-service/tcloud-ziyan/` 的 `DissolveClient` 提供对应封装。

#### Scenario: 按截止时间去重升序返回

- **WHEN** 调用该 data-service 接口，传入 filter 表达式（如 `project_id IN (...)`、`is_ignore=false`）
- **THEN** 系统 SHALL 执行 `GROUP BY expect_abolish_time` 且过滤 `expect_abolish_time != ''`，按 `expect_abolish_time` 升序返回去重后的全量截止时间字符串数组

#### Scenario: 无命中数据

- **WHEN** filter 命中的记录中不存在非空 `expect_abolish_time`
- **THEN** 系统 SHALL 返回空数组
