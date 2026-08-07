## MODIFIED Requirements

### Requirement: 裁撤总览查本地表聚合

裁撤总览接口 `POST /api/v1/woa/dissolve/table/list` SHALL 仅查询 `recycle_host_info`，按 `bk_biz_id` 分组聚合统计裁撤进度，并附一行「合计」。查询 SHALL 恒定附加 `is_ignore=false` 与 `project_id NOT IN listExcludedProjectIDs` 过滤，支持按 `project_ids`/`group_ids`/`bk_biz_ids`/`operators`/`regions`/`expect_abolish_times` 可选过滤；已申领 CPU 核数仍从 `device_info` 统计（机房裁撤需求类型、已交付、统计起始时间之后）。聚合 SHALL 由 woa-server 通过 data-service 列表接口分页拉取后在内存完成，不新增聚合接口。

#### Scenario: 按业务统计进度

- **WHEN** 调用总览接口
- **THEN** 系统 SHALL 返回每个业务的 `origin_host_count`/`origin_cpu_core`（命中条件全部记录）、`current_host_count`/`current_cpu_core`（`abolish_phase != complete`）、`delivered_cpu_core`、`progress = (origin_host_count - current_host_count) / origin_host_count`，并附「合计」行；合计行 `bk_biz_id` SHALL 为 `-1`）

#### Scenario: 恒定过滤忽略主机与排除项目

- **WHEN** 执行总览统计
- **THEN** 查询条件 SHALL 恒定包含 `is_ignore=false` 且 `project_id NOT IN listExcludedProjectIDs`

#### Scenario: 按裁撤截止时间过滤

- **WHEN** 请求传入非空 `expect_abolish_times` 数组
- **THEN** 系统 SHALL 追加 `expect_abolish_time IN (...)` 过滤，仅统计命中这些截止时间的主机

### Requirement: 裁撤明细查本地表

裁撤明细接口 `POST /api/v1/woa/dissolve/host/detail/list` SHALL 分页查询 `recycle_host_info` 并返回全字段（含 `expect_abolish_time` 等新增字段），恒定附加 `is_ignore=false` 与 `project_id NOT IN listExcludedProjectIDs`，支持 `bk_biz_ids`/`project_ids`/`group_ids`/`operators`/`modules`/`inner_ips`/`asset_ids`/`status`/`expect_abolish_times` 过滤；`operators` 过滤 SHALL 使用 JSON 命中匹配（任一负责人命中即返回）。

#### Scenario: operators JSON 命中过滤

- **WHEN** 请求传入 `operators` 过滤
- **THEN** 系统 SHALL 对 JSON 数组字段 `operators` 做命中过滤，返回负责人列表包含指定值的主机

#### Scenario: 响应返回裁撤截止时间

- **WHEN** 调用明细接口
- **THEN** 每条明细 SHALL 返回 `expect_abolish_time` 字段

#### Scenario: 按裁撤截止时间过滤

- **WHEN** 请求传入非空 `expect_abolish_times` 数组
- **THEN** 系统 SHALL 追加 `expect_abolish_time IN (...)` 过滤，仅返回命中这些截止时间的主机

### Requirement: 查询导出的裁撤主机明细分页 JSON

查询导出的裁撤主机明细接口 `POST /api/v1/woa/dissolve/host/detail/export/list` SHALL 复用明细查询逻辑，以分页 JSON 返回（单页上限 5000，不导出 Excel），并在明细全部参数基础上新增可选 `snapshot_date`；请求 SHALL 支持 `expect_abolish_times` 过滤并透传至明细查询，响应 SHALL 返回 `expect_abolish_time` 字段；当传入 `snapshot_date` 时 SHALL 按该日期定位 ES 快照索引，按固资号补充主机性能/属性扩展字段后合并返回。

#### Scenario: 不传 snapshot_date

- **WHEN** 导出请求未传 `snapshot_date`
- **THEN** 系统 SHALL 仅返回 `recycle_host_info` 的主机集合（分页，上限 5000），含 `expect_abolish_time` 字段

#### Scenario: 传 snapshot_date 补充 ES 扩展字段

- **WHEN** 导出请求传入 `snapshot_date`
- **THEN** 系统 SHALL 按 `snapshot_date` 定位 ES 快照索引，按固资号补充性能/属性扩展字段后合并分页返回

#### Scenario: 按裁撤截止时间过滤

- **WHEN** 导出请求传入非空 `expect_abolish_times` 数组
- **THEN** 系统 SHALL 将其透传至明细查询，追加 `expect_abolish_time IN (...)` 过滤

## ADDED Requirements

### Requirement: 查询裁撤截止时间列表接口

系统 SHALL 提供 woa-server 接口 `POST /api/v1/woa/dissolve/expect_abolish_time/list`，返回 `recycle_host_info` 中按 `expect_abolish_time` 去重、升序排序、去空后的全量截止时间数组。请求 SHALL 支持可选 `filter` 过滤表达式（可按 `project_id`/`bk_biz_id`/`group_id`/`region`/`abolish_phase`/`expect_abolish_time` 等字段过滤）；查询 SHALL 恒定附加 `is_ignore=false` 与 `project_id NOT IN listExcludedProjectIDs`，与总览/明细查询口径一致。接口鉴权 SHALL 与其余 scr 视角裁撤接口一致（服务请求-机房裁撤菜单粒度）。去重查询 SHALL 经 data-service 完成，woa-server MUST NOT 直连 DB。

#### Scenario: 返回去重升序截止时间

- **WHEN** 调用该接口且未传 `filter`
- **THEN** 系统 SHALL 经 data-service 对 `recycle_host_info`（`is_ignore=false`、排除 `listExcludedProjectIDs`、`expect_abolish_time != ''`）按 `expect_abolish_time` 去重升序，返回全量截止时间字符串数组

#### Scenario: 按 filter 过滤

- **WHEN** 请求传入非空 `filter`（如 `project_id IN (...)`）
- **THEN** 系统 SHALL 将 `filter` 以 AND 追加到恒定条件后再去重升序返回

#### Scenario: 无数据返回空数组

- **WHEN** 命中记录中不存在非空 `expect_abolish_time`
- **THEN** 系统 SHALL 返回空数组
