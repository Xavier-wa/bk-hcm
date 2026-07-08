## Why

机房裁撤的总览/明细/导出/配额查询当前在运行时实时聚合「本地表 + CC + ES 快照」三方数据，响应慢、依赖重、易出错；组织维度依赖人工维护的「裁撤模块」与裁撤系统真实维度「裁撤项目」不一致；裁撤项目、黑名单等配置写死在配置文件，调整需改代码重启；且 woa-server 越权直连 DB 操作 `recycle_host_info`，违反「仅 data-service 操作 DB」约定。本次将查询数据在同步阶段一次性落库，查询只读本地表，并以裁撤项目取代裁撤模块、配置动态化、读写收敛到 data-service。

## What Changes

- **同步阶段一次性落库**：同步任务把主机的业务、组织、负责人、CPU、地域、裁撤项目等字段全部补齐并写入 `recycle_host_info`（CPU 核数 `cpu_core` 取自裁撤系统 `cpuLogicCoreNum`；`bk_biz_id`/`group_id`/`operators` 走「CC 优先、ES 兜底」）。
- **查询直接查本地表**：总览/明细/明细导出/配额汇总改为只查 `recycle_host_info`（已申领核数仍查 `device_info`），不再运行时聚合 CC/ES。
- **裁撤项目取代裁撤模块**：组织维度由 `project_id` 承载，diff 比对键由 `asset_id` 改为 `(project_id, asset_id)`。**BREAKING** 下线 `recycle_module_info` 表及全部模块相关表/DAO/接口（`/dissolve/recycled_module/*`）。
- **去除固资号唯一约束**：删除 `recycle_host_info` 的 `idx_uk_asset_id`，允许同一固资号在不同项目并存。**BREAKING**
- **配置动态化**：裁撤项目改存 `global_config`（`dissolve_project`，支持「裁撤周期」数组）；新增 `ignore_biz` 忽略业务列表（命中则主机 `is_ignore=true`）；`listExcludedProjectNames`（`[]string`）改名为 `listExcludedProjectIDs`（`[]int`）。**BREAKING** 删除顶层 `blacklist` 配置与 `resourceDissolve.projectIDs`。
- **读写收敛 data-service**：woa-server 对 `recycle_host_info` 的全部增删改查统一走 data-service client。
- **新增接口**：`GET /dissolve/projects`（透传裁撤系统项目列表）；裁撤系统客户端新增 `ListProjects` 方法。
- **接口调整**：`GET /dissolve/config` 与 `PUT /dissolve/config/upsert` 增加 `dissolve_projects` 读写；明细导出改为分页 JSON（上限 5000）+ `snapshot_date` 走 ES。**BREAKING** 删除 `/dissolve/host/origin/list`、`/dissolve/host/current/list`。

## Capabilities

### New Capabilities

- `dissolve-host-sync`: 裁撤主机同步任务——按裁撤项目从裁撤系统拉取设备，补全业务/组织/负责人/CPU/地域字段（CC 优先 ES 兜底）、计算 is_ignore、按 `(project_id, asset_id)` diff，并经 data-service 落库。
- `dissolve-host-query`: 裁撤查询能力——总览、明细、明细导出、CPU 配额汇总仅查 `recycle_host_info`，含裁撤状态四态↔两态转换、`is_ignore=false` 与 `project_id NOT IN listExcludedProjectIDs` 恒定过滤。
- `dissolve-project-config`: 裁撤项目动态配置——`global_config` 存取 `dissolve_project`、裁撤配置接口扩展 `dissolve_projects`、裁撤系统项目列表透传接口及客户端 `ListProjects` 方法。

### Modified Capabilities

- `dissolve-host-crud`: `recycle_host_info` 表与 data-service 接口新增 `project_id`/`region`/`bk_biz_id`/`group_id`/`operators`/`cpu_core`/`is_ignore` 字段及其过滤与返回；去除固资号唯一约束依赖；列表支持 `operators` JSON 命中过滤。

## Impact

- **DB 迁移**：`recycle_host_info` 加 7 字段、删 `idx_uk_asset_id`、加 `idx_project_id_asset_id(project_id, asset_id)` 及查询辅助索引 `idx_bk_biz_id`/`idx_abolish_phase`/`idx_is_ignore`；drop `recycle_module_info`。新增 `dissolve_project` 的 `global_config` 数据。
- **data-service**：`pkg/api/data-service/dissolve/recycle_host.go`、`cmd/data-service/service/dissolve/recycle-host/*`、`pkg/dal/table/dissolve/host/host.go`、`pkg/client/data-service/tcloud-ziyan/dissolve.go`。
- **woa-server**：`cmd/woa-server/logics/dissolve/{logics.go,host,table,config,module}`、`cmd/woa-server/service/dissolve/*`、`cmd/woa-server/types/dissolve/types.go`、scheduler 固资校验、`IsDissolveHost`。
- **裁撤系统客户端**：`pkg/thirdparty/caiche/*`（新增 `ListProjects`，清理 V1 死代码）。
- **配置**：`pkg/cc/ziyan_types.go`、`cmd/woa-server/etc/woa_server.yaml`、`docs/support-file/helm/*`；删除 `pkg/cc` 顶层 `blacklist`、ES 客户端 `blacklist` 入参。
- **删除**：`recycle_module` 全链路、`/dissolve/host/origin|current/list` 及 logics、blacklist 相关代码与对应 API 文档。
- **前端**：明细/总览导出由「列表大分页 + 前端 Excel」改为调用新导出接口（需前端配合）。
