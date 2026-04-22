# dissolve-host-crud

## Requirements

### Requirement: 裁撤主机表 CRUD 接口

系统 SHALL 通过 data-service 暴露 `recycle_host_info` 表的批量创建、批量更新、列表查询、批量删除四个 HTTP 接口，供上层服务（woa-server）调用。

#### Scenario: 批量创建裁撤主机记录

- **WHEN** 调用 `POST /dissolve/recycle_hosts/batch/create`，传入包含 asset_id、inner_ip、module、abolish_phase、project_name 的主机列表
- **THEN** 系统 SHALL 为每条记录生成唯一 ID，在事务中批量插入 `recycle_host_info` 表，返回创建的 ID 列表

#### Scenario: 列表查询裁撤主机

- **WHEN** 调用 `POST /dissolve/recycle_hosts/list`，传入 filter 表达式和分页参数
- **THEN** 系统 SHALL 按 filter 条件查询 `recycle_host_info` 表，支持按 asset_id 精确匹配，返回符合条件的记录列表或记录总数（count 模式）

#### Scenario: 批量更新裁撤主机记录

- **WHEN** 调用 `PATCH /dissolve/recycle_hosts/batch`，传入 filter 表达式和更新字段
- **THEN** 系统 SHALL 在事务中按 filter 条件更新 `recycle_host_info` 表中匹配的记录

#### Scenario: 批量删除裁撤主机记录

- **WHEN** 调用 `DELETE /dissolve/recycle_hosts/batch`，传入 filter 表达式
- **THEN** 系统 SHALL 在事务中按 filter 条件删除 `recycle_host_info` 表中匹配的记录

### Requirement: 裁撤主机 client 封装

系统 SHALL 在 `pkg/client/data-service/tcloud-ziyan/` 提供 `DissolveClient`，封装上述四个 data-service 接口的调用，并注册到 `Client` 结构体中，使 woa-server 可通过 `clientSet.DataService().TCloudZiyan.Dissolve` 链式调用。

#### Scenario: woa-server 通过 client 查询裁撤主机

- **WHEN** woa-server 业务逻辑调用 `clientSet.DataService().TCloudZiyan.Dissolve.ListRecycleHost(kt, req)`
- **THEN** client SHALL 发送 HTTP 请求到 data-service 的 `/dissolve/recycle_hosts/list` 端点，并将响应反序列化后返回
