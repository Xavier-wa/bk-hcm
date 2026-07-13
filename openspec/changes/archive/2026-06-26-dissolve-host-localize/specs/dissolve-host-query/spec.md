## ADDED Requirements

### Requirement: 裁撤总览查本地表聚合

裁撤总览接口 `POST /api/v1/woa/dissolve/table/list` SHALL 仅查询 `recycle_host_info`，按 `bk_biz_id` 分组聚合统计裁撤进度，并附一行「合计」。查询 SHALL 恒定附加 `is_ignore=false` 与 `project_id NOT IN listExcludedProjectIDs` 过滤，支持按 `project_ids`/`group_ids`/`bk_biz_ids`/`operators`/`regions` 可选过滤；已申领 CPU 核数仍从 `device_info` 统计（机房裁撤需求类型、已交付、统计起始时间之后）。聚合 SHALL 由 woa-server 通过 data-service 列表接口分页拉取后在内存完成，不新增聚合接口。

#### Scenario: 按业务统计进度

- **WHEN** 调用总览接口
- **THEN** 系统 SHALL 返回每个业务的 `origin_host_count`/`origin_cpu_core`（命中条件全部记录）、`current_host_count`/`current_cpu_core`（`abolish_phase != complete`）、`delivered_cpu_core`、`progress = (origin_host_count - current_host_count) / origin_host_count`，并附「合计」行

#### Scenario: 恒定过滤忽略主机与排除项目

- **WHEN** 执行总览统计
- **THEN** 查询条件 SHALL 恒定包含 `is_ignore=false` 且 `project_id NOT IN listExcludedProjectIDs`

### Requirement: 裁撤明细查本地表

裁撤明细接口 `POST /api/v1/woa/dissolve/host/detail/list` SHALL 分页查询 `recycle_host_info` 并返回全字段（含新增字段），恒定附加 `is_ignore=false` 与 `project_id NOT IN listExcludedProjectIDs`，支持 `bk_biz_ids`/`project_ids`/`group_ids`/`operators`/`modules`/`inner_ips`/`asset_ids`/`status` 过滤；`operators` 过滤 SHALL 使用 JSON 命中匹配（任一负责人命中即返回）。

#### Scenario: operators JSON 命中过滤

- **WHEN** 请求传入 `operators` 过滤
- **THEN** 系统 SHALL 对 JSON 数组字段 `operators` 做命中过滤，返回负责人列表包含指定值的主机

### Requirement: 裁撤状态四态两态转换

明细查询 SHALL 在请求与响应两处转换裁撤状态：请求参数 `status=complete` 映射为 DB `abolish_phase = complete`，请求参数 `status=incomplete` 映射为 DB `abolish_phase IN (incomplete, bsiComplete, retain)`；响应时 DB 四态 SHALL 归并回 `status` 两态 `complete`/`incomplete` 返回，`retain` MUST NOT 遗漏。

#### Scenario: 请求 incomplete 覆盖三态

- **WHEN** 明细请求 `status = incomplete`
- **THEN** 系统 SHALL 查询 DB `abolish_phase IN (incomplete, bsiComplete, retain)` 的记录

#### Scenario: 响应归并为两态

- **WHEN** 返回的记录 DB `abolish_phase` 为 `bsiComplete` 或 `retain`
- **THEN** 系统 SHALL 将其 `status` 归并为 `incomplete` 返回

### Requirement: 查询导出的裁撤主机明细分页 JSON

查询导出的裁撤主机明细接口 `POST /api/v1/woa/dissolve/host/detail/export/list` SHALL 复用明细查询逻辑，以分页 JSON 返回（单页上限 5000，不导出 Excel），并在明细全部参数基础上新增可选 `snapshot_date`；当传入 `snapshot_date` 时 SHALL 按该日期定位 ES 快照索引，按固资号补充主机性能/属性扩展字段后合并返回。

#### Scenario: 不传 snapshot_date

- **WHEN** 导出请求未传 `snapshot_date`
- **THEN** 系统 SHALL 仅返回 `recycle_host_info` 的主机集合（分页，上限 5000）

#### Scenario: 传 snapshot_date 补充 ES 扩展字段

- **WHEN** 导出请求传入 `snapshot_date`
- **THEN** 系统 SHALL 按 `snapshot_date` 定位 ES 快照索引，按固资号补充性能/属性扩展字段后合并分页返回

### Requirement: CPU 配额汇总改本地表来源

CPU 配额汇总接口 `POST /api/v1/woa/dissolve/cpu_core/summary` SHALL 将「需裁撤总核数」改为查询 `recycle_host_info`（`is_ignore=false`、排除 `listExcludedProjectIDs`）按业务的原始 `sum(cpu_core)`；已申领核数、配额系数、业务偏移、可申请额度等逻辑保持不变。

#### Scenario: total_core 来自本地表

- **WHEN** 调用配额汇总接口
- **THEN** `total_core` SHALL 取自 `recycle_host_info` 按业务的原始 `sum(cpu_core)`，而非 ES 原始快照

## REMOVED Requirements

### Requirement: 原始与当前主机分离查询

**Reason**: 总览/明细改为统一查询 `recycle_host_info`，不再区分 ES 原始快照与 CC 实时当前两条链路。

**Migration**: 调用方改用 `POST /dissolve/table/list`（总览）与 `POST /dissolve/host/detail/list`（明细）；`/dissolve/host/origin/list` 与 `/dissolve/host/current/list` 接口下线。
