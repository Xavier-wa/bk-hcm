## Context

机房裁撤功能用于跟踪和管理需从机房下线的服务器，统计各业务的裁撤进度与可申领配额。当前实现（woa-server `cmd/woa-server/logics/dissolve/`）存在五个核心问题：

1. **查询链路长**：总览(`ListResDissolveTable`)/明细(`FindOriginHost`+`FindCurHost`)在运行时实时聚合「`recycle_host_info` + CC + ES 快照」，并行调用、内存合并，响应慢且依赖重。
2. **组织维度错配**：以人工维护的 `recycle_module_info`（裁撤模块）为维度，与裁撤系统真实维度「裁撤项目(project)」不一致。
3. **唯一约束不合理**：`recycle_host_info` 对 `asset_id` 建唯一索引（`idx_uk_asset_id`，SQL 0020），同一固资号无法在多个项目并存；同步逻辑因此存在内存去重择优(`addHost`)。
4. **配置写死**：裁撤项目(`resourceDissolve.projectIDs`)、黑名单(顶层 `blacklist`)写在 yaml，调整需改配置重启。
5. **越权直连 DB**：woa-server 裁撤 CRUD/Sync 直连 `pkg/dal/dao`（`logics/dissolve/host/host.go:76,95,115,121`），违反「仅 data-service 操作 DB」。

现有基础设施可复用：data-service 已有 `recycle_host` 的 proto/handler 与 `DissolveClient`（4 方法）；裁撤配置已规范走 `global_config`（`config_type=res_dissolve`）的 client；`global_config` 唯一键 `(config_type, config_key)`。

## Goals / Non-Goals

**Goals:**

- 同步阶段一次性把 `project_id`/`region`/`bk_biz_id`/`group_id`/`operators`/`cpu_core`/`is_ignore` 落库到 `recycle_host_info`。
- 总览/明细/导出/配额汇总改为仅查本地表 + 内存聚合，去除运行时 CC/ES 聚合（导出按需 ES 补扩展字段）。
- 以裁撤项目取代裁撤模块，diff 键改 `(project_id, asset_id)`，下线模块全链路。
- 裁撤项目、忽略业务配置动态化（`global_config` + 服务配置），无需改代码。
- woa-server 对 `recycle_host_info` 全部读写经 data-service client。

**Non-Goals:**

- 不重构裁撤申领/配额计算的业务规则（配额系数、业务偏移、已申领核数统计逻辑保持不变，仅切换数据来源）。
- 不改动 MongoDB 侧「任务回收主机」(`dal/task/dao/recycle_host.go`)，与本次裁撤表无关。
- 不统一裁撤系统 V1/V2 接口（V1 为死代码，仅清理，不做接口合并）。
- 不引入实时配置生效机制；`ignore_biz`/`dissolve_project` 调整随下一同步周期生效。

## Decisions

### D1. 字段来源：cpu_core 取裁撤系统，operators 走 CC 优先 ES 兜底

字段补全分两类：

**A. 裁撤系统(CaiChe DeviceV2)直接提供** — `project_id`(`projectId`)、`asset_id`(`serAssetId`)、`inner_ip`(`serverLanIp`)、`module`(`modName`)、`abolish_phase`、`project_name`、**`cpu_core`(`cpuLogicCoreNum`)**、`region`（用 `availabilityZoneName` 匹配 `woa_zone.zone_name` 取 `region_id`）。

**B. CC 优先、ES 兜底** — `bk_biz_id`、`group_id`(`bk_oper_grp_name_id`)、`operators`(`operator`+`bk_bak_operator`)。

> 决策点：`cpu_core` 原方案取自 CC 的 `bk_cpu`，但裁撤系统 `DeviceV2.CPULogicCoreNum` 已提供，**直接取裁撤系统值**可减少 CC 调用、加快同步、且核数随设备同步天然一致；`operators` 保持取 CC（`operator`+`bk_bak_operator`），因负责人信息以 CC 为权威且需与展示口径一致。

**B 类的有状态补全规则**（关键，区别于现状无状态全覆盖）：

```
按 asset_id 批量查 CC：
  ├─ CC 命中 → 用 CC 值填 B 类字段；与 DB 旧值比较，有变化则更新
  └─ CC 未命中：
        ├─ DB 旧记录 B 类字段已有值 → 跳过（保留旧值）
        └─ DB 旧记录 B 类字段无值   → 查 ES 快照(originDate, 索引 app_device_pass_dtl_{originDate})补齐
```

因此同步必须把 DB 旧记录(按 `(project_id, asset_id)`)一并加载参与判断，不能纯靠 CaiChe 全量覆盖。

### D2. ignore 机制取代 blacklist

取得 `bk_biz_id` 后，若命中服务配置 `ignore_biz`(`[]int64`) 则主机 `is_ignore=true`，否则 `false`，业务变化时随之更新。查询侧黑名单过滤(`getBlackBizIDName`/`GetESCond` 的 `BlackList`)全部移除，统一用查询条件 `is_ignore=false`。删除顶层 `blacklist` 配置、ES 客户端 `blacklist` 入参与 `EsCli.blacklist` 冗余字段（该字段当前传入后从未被读取）。

**备选**：保留 blacklist 作运行时过滤。否决——与「同步落库、查询读本地」目标矛盾，且现状 blacklist 双轨（ES 字段冗余 + CC 名称转换）已是技术债。

### D3. 同步逻辑重写

```
1. 读 dissolve_project 配置 → 取所有周期 projects[].id 并集（替代 yaml projectIDs）
2. 预加载映射：woa_zone(zone_name→region_id)、ignore_biz 集合
3. CaiChe ListDeviceV2 分页拉取（projectId 并集 + svrTypeName 过滤；
   abolishPhase 四态 incomplete/complete/bsiComplete/retain — 现状缺 retain，需补）
4. A 类字段直接落 + region 映射
5. B 类字段批量 CC 查询，CC 优先 / ES 兜底（D1 有状态规则）
6. 计算 is_ignore（D2）
7. diff(键=project_id+asset_id) → 写库全部走 data-service client
```

删除现状 `addHost` 按 asset_id 去重择优逻辑；`diff`(`host.go:405`)的 map key 由 `asset_id` 改为 `project_id+asset_id` 复合键。

**备选**：保留 asset_id 维度、引入"主项目"概念。否决——违背「同一固资号多项目并存」目标。

### D4. 查询侧重写为本地表 + 内存聚合

- **总览** `POST /dissolve/table/list`：查 `recycle_host_info` 按 `bk_biz_id` 分组聚合（原始 = 命中全部、当前 = `abolish_phase != complete`），进度 = (原始数−当前数)/原始数；已申领核数仍查 `device_info`（逻辑不变）；woa-server 分页拉命中记录、内存聚合，**不新增聚合接口**；附「合计」行。
- **明细** `POST /dissolve/host/detail/list`：分页查本地表，支持 `bk_biz_ids`/`project_ids`/`group_ids`/`operators`/`modules`/`inner_ips`/`asset_ids`/`status` 过滤，四态↔两态转换（见 D5），`operators` 走 JSON 命中过滤（`tools.RuleJsonOverlaps`，任一命中即返回）。
- **查询导出的裁撤主机明细** `POST /dissolve/host/detail/export/list`：复用明细查询，分页 JSON（上限放大到 5000，不导 Excel），新增可选 `snapshot_date`——传入时按该日期 ES 快照按固资号补充性能/属性扩展字段。
- **配额汇总** `POST /dissolve/cpu_core/summary`：`total_core` 改查本地表按业务原始 `sum(cpu_core)`，其余（已申领、系数、偏移、可申请额度）不变。

所有查询恒定附加 `is_ignore=false` 与 `project_id NOT IN listExcludedProjectIDs`。删除 `/dissolve/host/origin/list`、`/dissolve/host/current/list` 及 `FindOriginHost`/`FindCurHost`/`getAllModuleName`/`fillProjectName`/`ReqForGetHost`。

### D5. 裁撤状态四态↔两态转换

DB/裁撤系统四态：`incomplete`/`complete`/`bsiComplete`/`retain`。对外明细两态：

- 请求 `complete` → DB `abolish_phase = complete`；请求 `incomplete` → DB `IN (incomplete, bsiComplete, retain)`。
- 响应：DB 四态归并回 `complete`/`incomplete` 返回。

请求与响应两处都需转换，`retain` 不得遗漏。

### D6. data-service 扩展 + 读写收敛 client

- `recycle_host_info` 表结构体/列描述符增 7 字段；`InsertValidate` 去除对 asset_id 唯一性的隐含依赖。
- proto（`pkg/api/data-service/dissolve/recycle_host.go`）创建/更新增 7 字段，列表过滤经通用 filter 透传支持新字段（含 `operators` JSON 命中，woa-server 侧用 `tools.RuleJsonOverlaps` 构造任一命中条件）与返回。
- woa-server 同步、CRUD、`IsDissolveHost`、scheduler `validateDissolveRecycleHost` 全部改走 `DataService().TCloudZiyan.Dissolve` client。

### D7. 配置变更

- **新增 `dissolve_project`**（`global_config`, `config_type=res_dissolve`）：「裁撤周期」数组（`start`/`end`/`default`/`projects[]{id,memo}`），经现有裁撤配置接口读写；校验日期合法且 `start<=end`、`projects` 非空且 `id>0`。
- **`ResourceDissolve` 调整**：`originDate` 保留（定位 ES 快照日期）；`listExcludedProjectNames`(`[]string`) 改名 `listExcludedProjectIDs`(`[]int`)；新增 `ignore_biz`(`[]int64`)；删除 `projectIDs`；`svrTypeNames`/`syncDissolveHost` 保留。
- **删除**顶层 `blacklist`。需同步改 `etc/woa_server.yaml` 与 `docs/support-file/helm`。

### D8. 裁撤系统项目列表接口

CaiChe 客户端新增 `ListProjects`（`/openapi_gateway/abolish-backend/device/listProjects`，复用 token 机制，无参，响应 `[{id, projectName, projectType}]`）；woa-server 新增 `GET /dissolve/projects` 透传，供前端配置裁撤项目选择。清理未被调用的 V1 `ListDevice`/`transferToHost` 死代码。

## Risks / Trade-offs

- **B 类字段有状态补全易写错** → 同步重写时务必先加载 DB 旧值参与「CC 未命中」分支判断；补充单测覆盖 CC 命中/未命中×旧值有/无的四种组合。
- **去唯一索引前存量重复** → 决策假设当前为单项目无重复（用户确认）；迁移直接删索引 + 加 `idx_project_id_asset_id`，**上线前需一次性全量同步回填新字段与 is_ignore**。
- **查询内存聚合大数据量** → 总览/汇总拉全量聚合，需控制分页次数、仅取聚合所需字段(`bk_biz_id`/`cpu_core`/`abolish_phase`)、关注内存占用。
- **同步性能(CC 限频)** → B 类字段按 asset_id 批量查 CC，需批量化并控制并发，关注 CC 调用限频。
- **配置生效时延** → `ignore_biz`/`dissolve_project` 调整随下一同步周期生效（非实时），需在配置接口/文档说明。
- **状态转换一致性** → 四态↔两态在请求与响应两处转换，`retain` 易漏，需单测覆盖。
- **前端依赖** → 导出由「大分页 + 前端 Excel」改为后端分页 JSON 接口，需前端配合改造（本变更聚焦后端，前端改造另行跟进）。

## Migration Plan

1. 上线 DB 迁移（加字段、删 `idx_uk_asset_id`、加 `idx_project_id_asset_id`、drop `recycle_module_info`）。
2. 写入 `dissolve_project` 配置，调整服务配置(`ignore_biz`/`listExcludedProjectIDs`/删 `projectIDs`/删 `blacklist`)。
3. 部署新 woa-server/data-service，触发一次全量同步回填新字段与 `is_ignore`。
4. 验证总览/明细/导出/汇总数据一致后，前端切换到新导出接口。

**回滚**：迁移文件需保证可逆性评估；新增字段为加列（向后兼容），删表/删索引为不可逆，回滚前需确认存量数据已备份。

## Open Questions

- `woa_zone` 数据是否覆盖全部裁撤机器所在 zone？`availabilityZoneName` 匹配不中时 `region` 留空的展示/过滤行为需确认。
- 线上 `recycle_module_info` 是否仍有人工维护数据或外部流程依赖，删表前需运营确认。
