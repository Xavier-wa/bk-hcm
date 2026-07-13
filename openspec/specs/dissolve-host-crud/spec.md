# dissolve-host-crud

## Purpose

裁撤主机表 CRUD 能力：通过 data-service 暴露 `recycle_host_info` 表的批量创建/更新/删除与列表查询接口，并提供 client 封装供 woa-server 经微服务调用，禁止上层直连 DB。

## Requirements

### Requirement: 裁撤主机表 CRUD 接口

系统 SHALL 通过 data-service 暴露 `recycle_host_info` 表的批量创建、批量更新、列表查询、批量删除四个 HTTP 接口，供上层服务（woa-server）调用。`recycle_host_info` SHALL 包含字段 `project_id`、`region`、`bk_biz_id`、`group_id`、`operators`（JSON 数组）、`cpu_core`、`is_ignore`，创建/更新接口 SHALL 支持写入这些字段，列表接口 SHALL 支持按这些字段过滤（`operators` 用 JSON 命中匹配）与返回。系统 MUST NOT 依赖 `asset_id` 唯一约束，允许同一 `asset_id` 在不同 `project_id` 下并存。

#### Scenario: 批量创建裁撤主机记录

- **WHEN** 调用 `POST /dissolve/recycle_hosts/batch/create`，传入包含 asset_id、inner_ip、module、abolish_phase、project_name、project_id、region、bk_biz_id、group_id、operators、cpu_core、is_ignore 的主机列表
- **THEN** 系统 SHALL 为每条记录生成唯一 ID，在事务中批量插入 `recycle_host_info` 表，返回创建的 ID 列表，且不因相同 asset_id 报唯一约束冲突

#### Scenario: 列表查询裁撤主机

- **WHEN** 调用 `POST /dissolve/recycle_hosts/list`，传入 filter 表达式和分页参数
- **THEN** 系统 SHALL 按 filter 条件查询 `recycle_host_info` 表，支持按 asset_id、project_id、bk_biz_id、group_id、region、abolish_phase、is_ignore 过滤及 operators JSON 命中过滤，返回符合条件的记录列表或记录总数（count 模式）

#### Scenario: 批量更新裁撤主机记录

- **WHEN** 调用 `PATCH /dissolve/recycle_hosts/batch`，传入 filter 表达式和更新字段（含新增字段）
- **THEN** 系统 SHALL 在事务中按 filter 条件更新 `recycle_host_info` 表中匹配的记录

#### Scenario: 批量删除裁撤主机记录

- **WHEN** 调用 `DELETE /dissolve/recycle_hosts/batch`，传入 filter 表达式
- **THEN** 系统 SHALL 在事务中按 filter 条件删除 `recycle_host_info` 表中匹配的记录

### Requirement: 裁撤主机 client 封装

系统 SHALL 在 `pkg/client/data-service/tcloud-ziyan/` 提供 `DissolveClient`，封装上述四个 data-service 接口的调用，并注册到 `Client` 结构体中，使 woa-server 可通过 `clientSet.DataService().TCloudZiyan.Dissolve` 链式调用。woa-server 的裁撤同步、CRUD、`IsDissolveHost` 判定、scheduler 固资校验 SHALL 全部经此 client 完成，MUST NOT 直连 `pkg/dal/dao`。

#### Scenario: woa-server 通过 client 查询裁撤主机

- **WHEN** woa-server 业务逻辑调用 `clientSet.DataService().TCloudZiyan.Dissolve.ListRecycleHost(kt, req)`
- **THEN** client SHALL 发送 HTTP 请求到 data-service 的 `/dissolve/recycle_hosts/list` 端点，并将响应反序列化后返回

#### Scenario: woa-server 同步与判定走 client

- **WHEN** woa-server 执行裁撤同步写库、`IsDissolveHost` 判定或 scheduler 固资校验
- **THEN** 相关读写 SHALL 经 `DataService().TCloudZiyan.Dissolve` client，而非直连 `pkg/dal/dao`
