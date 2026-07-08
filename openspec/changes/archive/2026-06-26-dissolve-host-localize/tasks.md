## 1. DB 迁移与表结构

- [x] 1.1 新建 SQL 迁移文件 `scripts/sql/9999_*_resource_dissolve.sql`（SQLVER=9999, HCMVER=v9.9.9）：`recycle_host_info` 增加 `project_id`/`region`/`bk_biz_id`/`group_id`/`operators`(json)/`cpu_core`/`is_ignore` 字段
- [x] 1.2 同 SQL 文件：删除 `idx_uk_asset_id`，新增 `idx_project_id_asset_id(project_id, asset_id)` 及辅助索引 `idx_bk_biz_id`/`idx_abolish_phase`/`idx_is_ignore`
- [x] 1.3 同 SQL 文件：`drop table recycle_module_info`，更新 `hcm_version` 视图
- [x] 1.4 更新表结构体与列描述符 `pkg/dal/table/dissolve/host/host.go`（新增 7 字段、调整 `InsertValidate` 去除 asset_id 唯一性隐含依赖）
- [x] 1.5 删除 `pkg/dal/table/dissolve/module/` 及 `pkg/dal/table/table.go` 中 `recycle_module_info` 表名常量（与 9.1 合并执行，避免中间编译失败）

## 2. 裁撤系统客户端（CaiChe）

- [x] 2.1 在 `pkg/thirdparty/caiche/caiche_response.go` 新增 `ListProjects` 响应结构体（`id`/`projectName`/`projectType`）
- [x] 2.2 在 `pkg/thirdparty/caiche/caicheapi.go` 新增 `ListProjects` 方法（路径 `/openapi_gateway/abolish-backend/device/listProjects`，复用 token）并加入接口定义
- [x] 2.3 清理未被调用的 V1 `ListDevice`/`transferToHost` 死代码（请求/响应结构体一并清理）

## 3. 配置变更

- [x] 3.1 调整 `pkg/cc/ziyan_types.go` 的 `ResourceDissolve`：删 `ProjectIDs`，`ListExcludedProjectNames([]string)` 改名 `ListExcludedProjectIDs([]int)`，新增 `IgnoreBiz([]int64)`，更新 `validate()`
- [x] 3.2 删除 `pkg/cc/service.go` 顶层 `Blacklist` 字段及相关读取
- [x] 3.3 更新 `cmd/woa-server/etc/woa_server.yaml`：调整 `resourceDissolve`、删除 `blacklist`
- [x] 3.4 更新 `docs/support-file/helm/values.yaml` 与 `templates/woaserver/configmap.yaml`：同步 `resourceDissolve`、删除 `blacklist`
- [x] 3.5 ES 客户端 `pkg/thirdparty/es/client.go` 去除 `blacklist` 入参与 `EsCli.blacklist` 冗余字段；更新 `service.go:267` 构造调用

## 4. data-service 扩展（dissolve-host-crud）

- [x] 4.1 扩展 proto `pkg/api/data-service/dissolve/recycle_host.go`：创建/更新请求增 7 字段，列表请求/响应支持新字段
- [x] 4.2 更新 data-service handler `cmd/data-service/service/dissolve/recycle-host/{create,update,query}.go` 处理新字段；列表支持 `operators` JSON 命中过滤（通用 filter 透传自动支持）
- [x] 4.3 扩展 client `pkg/client/data-service/tcloud-ziyan/dissolve.go` 对应请求/响应字段（随 proto 自动生效）

## 5. global_config 裁撤项目配置

- [x] 5.1 在 `pkg/criteria/enumor/global_config.go` 新增 `dissolve_project` config_key 枚举
- [x] 5.2 定义 `dissolve_project` 配置结构体（裁撤周期数组）及校验（日期合法、start<=end、projects 非空、id>0）
- [x] 5.3 扩展 `cmd/woa-server/logics/dissolve/config/config.go`：读写 `dissolve_project` 配置

## 6. woa-server 同步逻辑重写（dissolve-host-sync）

- [x] 6.1 同步入口改造 `cmd/woa-server/logics/dissolve/host/host.go`：从 `dissolve_project` 配置取项目并集替代 `projectIDs`；`abolishPhase` 过滤补 `retain`
- [x] 6.2 预加载 `woa_zone`(zone_name→region_id) 与 `ignore_biz` 集合
- [x] 6.3 A 类字段直接落库 + `region` 由 `availabilityZoneName` 映射 + `cpu_core` 取 `cpuLogicCoreNum`
- [x] 6.4 B 类字段（`bk_biz_id`/`group_id`/`operators`）按「CC 优先、ES 兜底」有状态补全（含 DB 旧值参与判断）
- [x] 6.5 计算 `is_ignore`（命中 `ignore_biz` 置 true，业务变更重算）
- [x] 6.6 删除 `addHost` 去重择优；`diff` 比对键改 `(project_id, asset_id)` 复合键
- [x] 6.7 同步写库（增/改/删）全部改走 data-service client；`getAllHostFromDB` 改走 client

## 7. woa-server 查询逻辑重写（dissolve-host-query）

- [x] 7.1 总览 `ListResDissolveTable`：改查本地表按 `bk_biz_id` 内存聚合，恒附 `is_ignore=false` + `project_id NOT IN listExcludedProjectIDs`，附合计行；`delivered_cpu_core` 仍查 `device_info`
- [x] 7.2 新增明细接口 `POST /dissolve/host/detail/list`：分页查本地表全字段，支持 `operators` JSON 过滤
- [x] 7.3 实现裁撤状态四态↔两态转换（请求与响应两处，含 `retain`）
- [x] 7.4 新增查询导出的裁撤主机明细接口 `POST /dissolve/host/detail/export/list`：复用明细查询，分页 JSON（上限 5000），`snapshot_date` 走 ES 补扩展字段
- [x] 7.5 配额汇总 `summary`：`total_core` 改查本地表 `sum(cpu_core)`，其余逻辑不变

## 8. woa-server 接口与跨模块改造

- [x] 8.1 新增路由与 handler `GET /dissolve/projects`（透传 CaiChe `ListProjects`）
- [x] 8.2 扩展 `GET /dissolve/config`、`PUT /dissolve/config/upsert`：增加 `dissolve_projects` 读写
- [x] 8.3 `IsDissolveHost`（`host/host.go`）改走 data-service client
- [x] 8.4 scheduler `validateDissolveRecycleHost`（`scheduler.go`）确认/适配走 client 的字段

## 9. 删除清单

- [x] 9.1 删除 `recycle_module` 全链路：`pkg/dal/dao/dissolve/module/`、`pkg/dal/dao/types/dissolve/module/`、`dao.go` 注册、`cmd/woa-server/logics/dissolve/module/`、`service/dissolve/module.go` 及路由、`types/dissolve` 模块相关类型
- [x] 9.2 删除旧查询：`/dissolve/host/origin/list`、`/dissolve/host/current/list` 路由与 handler，及 `FindOriginHost`/`FindCurHost`/`getAllModuleName`/`fillProjectName`/`ReqForGetHost`/`getAssetIDByModule`
- [x] 9.3 删除 blacklist 相关：`table/cc.go` 的 `getBlackBizIDName`、`types/dissolve/types.go` 的 `GetESCond` 黑名单条件、`es` 的 `BlackList` 使用
- [x] 9.4 删除被废弃接口的 API 文档（origin/current list、recycled_module CRUD）

## 10. 接口文档

- [x] 10.1 新增 API 文档（v9.9.9+）：`GET /dissolve/projects`、`POST /dissolve/host/detail/list`、`POST /dissolve/host/detail/export/list`
- [x] 10.2 更新 API 文档：`GET /dissolve/config`、`PUT /dissolve/config/upsert`（dissolve_projects）、`POST /dissolve/table/list`、`POST /dissolve/cpu_core/summary`（字段与来源变更）

## 11. 验证

- [x] 11.1 编译/类型检查（沙箱无法下载 go1.24 工具链执行 `go build`，改用 gopls 全模块静态检查，改动包均无错误）
- [x] 11.2 同步补字段四组合（CC 命中/未命中 × ES × 旧值有/无）单测（`host/sync_test.go` 的 `TestFillBizFields`/`TestDiff`）
- [x] 11.3 裁撤状态四态↔两态转换单测（覆盖 retain，`types/dissolve/types_test.go`）
- [ ] 11.4 全量同步回填验证 + 总览/明细/导出/汇总数据一致性核对（需在具备 DB/CC/ES 的真实环境执行）
