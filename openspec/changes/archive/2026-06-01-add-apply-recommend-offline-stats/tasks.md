## 1. SQL 与表名注册

- [x] 1.1 创建 `scripts/sql/0077_20260527_1500_apply_recommend.sql`，包含 `ziyan_cvm_apply_user_recommend` 和 `ziyan_cvm_apply_biz_recommend` 建表语句，以及两个资源的 `id_generator` 注册（SQLVER=9999, HCMVER=v9.9.9）
- [x] 1.2 在 `pkg/dal/table/table.go` 新增 `ZiyanCvmApplyUserRecommendTable` / `ZiyanCvmApplyBizRecommendTable` 常量并加入 `TableMap`

## 2. 数据模型层（pkg/dal/table）

- [x] 2.1 创建 `pkg/dal/table/cvm-apply/ziyan_cvm_apply_user_recommend.go`：定义 `ZiyanCvmApplyUserRecommend` 结构体、ColumnDescriptor、`InsertValidate` / `UpdateValidate`
- [x] 2.2 创建 `pkg/dal/table/cvm-apply/ziyan_cvm_apply_biz_recommend.go`：同上，去掉 `bk_username` 字段

## 3. DAO 层（pkg/dal/dao）

- [x] 3.1 创建 `pkg/dal/dao/cvm-apply/ziyan_cvm_apply_user_recommend.go`：实现 `Interface`（`CreateWithTx` / `UpdateWithTx` / `List` / `DeleteWithTx`），参考 `device_capacity.go` 模式
- [x] 3.2 创建 `pkg/dal/dao/cvm-apply/ziyan_cvm_apply_biz_recommend.go`：同上

## 4. 配置层

- [x] 4.1 在 `pkg/cc/ziyan_types.go` 新增 `ApplyRecommend` 配置结构体（`Interval` / `LookbackDays` / `MaxRows`）及 `trySetDefault`（默认 720/90/5），并在 `pkg/cc/service.go` 挂载到 `WoaServerSetting` 且调用默认值设置
- [x] 4.2 在 `cmd/woa-server/etc/woa_server.yaml` 新增 `applyRecommend` 段，填入默认值（interval=720, lookbackDays=90, maxRows=5）
- [x] 4.3 在 `docs/support-file/helm/values.yaml` 与 `templates/woaserver/configmap.yaml` 同步新增 `applyRecommend` 配置项

## 5. API 类型定义（pkg/api）

- [x] 5.1 在 `pkg/api/data-service/cvm-apply/ziyan_cvm_apply_recommend.go` 新增 user / biz 推荐表的请求/响应类型（`BatchCreateZiyanCvmApplyUserRecommendReq` / `ZiyanCvmApplyUserRecommendListReq` / `ZiyanCvmApplyUserRecommendListResult` 等，biz 同上）
- [x] 5.2 在 `pkg/api/woa-server/cvm_apply_recommend.go`（package `woaserver`）新增 `ApplyRecommendTopReq`（`limit` 校验 min=1,max=20）/ `ApplyRecommendTopResp` 及响应元素类型（含 `source` 字段）
- [x] 5.3 在 `pkg/criteria/enumor/woa_ziyan.go` 新增 `ApplyRecommendSource` 枚举（`user` / `biz`）

## 6. data-service 路由与 Service 层

- [x] 6.1 在 `cmd/data-service/service/cvm-apply/cvm-apply-user-recommend/` 新增 user 推荐表 Service（`BatchCreate` / `List` / `BatchUpdate` / `BatchDelete` 路由 handler）
- [x] 6.2 在 `cmd/data-service/service/cvm-apply/cvm-apply-biz-recommend/` 新增 biz 推荐表 Service，同上
- [x] 6.3 在 `cmd/data-service/service/service.go` 中挂载新增路由

## 7. data-service HTTP Client（pkg/client）

- [x] 7.1 在 `pkg/client/data-service/tcloud-ziyan/` 新增 `ZiyanCvmApplyUserRecommendClient`（`BatchCreate` / `List` / `BatchUpdate` / `BatchDelete`），参考 `ZiyanCvmDeviceInfoClient` 模式
- [x] 7.2 新增 `ZiyanCvmApplyBizRecommendClient`，同上
- [x] 7.3 在 `pkg/client/data-service/tcloud-ziyan/` 的 ClientSet 中注册两个新 Client

## 8. woa-server cron 任务框架

- [x] 8.1 在 `pkg/criteria/enumor/cron_task.go` 新增常量 `CronTaskApplyRecommendOffline = "apply_recommend_offline"`
- [x] 8.2 创建 `cmd/woa-server/task/apply_recommend.go`：定义 `ApplyRecommendOfflineTask` 结构体，实现 `Name()` / `Next()`（读 `cc.WoaServer().ApplyRecommend.Interval`）/ `Do()`（master 判断 + 调用 logics），参考 `device_capacity.go`
- [x] 8.3 在 `cmd/woa-server/service/service.go` 的 `initCronTask` 中实例化并注册 `ApplyRecommendOfflineTask`

## 9. woa-server 推荐业务逻辑（logics）

- [x] 9.1 创建 `cmd/woa-server/logics/applyrecommend/aggregator.go`：实现 `AggregateUser`（五元组计数 + 按 (biz, user) 分组 Top-K 切分）和 `AggregateBiz`（四元组计数 + 按 biz 分组 Top-K 切分）
- [x] 9.2 创建 `cmd/woa-server/logics/applyrecommend/logics.go`：实现主流程 `GenerateRecommend`（记录 startTime → 分页拉取 `ziyan_cvm_device_info`（`is_delivered=true` 且 `updated_at >= now-lookbackDays`）→ 地域补全 → 聚合 → 按 (biz, user) 分组 BatchDelete + BatchCreate 写 user 表 → 按 biz 分组写 biz 表 → 分批过期清理）
- [x] 9.3 实现地域补全 `fillOutCloudRegion`：对 `cloud_region` 为空的记录按 `cloud_zone` / `zone_name` 批量查 `Zone.ListZoneExt` 映射 region 回填，仍为空则记 Error 跳过
- [x] 9.4 实现分批过期清理函数：分页 List 获取 `updated_at < startTime` 的行 ID，按批 BatchDelete（默认分页每批 500 行）

## 10. woa-server HTTP 推荐查询接口

- [x] 10.1 创建 `cmd/woa-server/service/task/recommend.go`：实现 `GetBizApplyRecommendTop` Handler，从路径参数取 `bk_biz_id`、请求体解码 `bk_username` 与 `limit`、校验（limit ∈ [1,20]）、业务访问鉴权（`Biz` / `Access`）、按人补业务编排逻辑（user 行三元组去重）、拼接返回
- [x] 10.2 在 `cmd/woa-server/service/task/service.go` 的 `bizService`（路由前缀 `/bizs/{bk_biz_id}/task`）注册业务路由 `POST /apply/recommend/top`（对外 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/top`）
- [x] 10.3 在 `docs/api-docs/web-server/docs/biz/get_apply_recommend_top.md` 新增接口文档，版本标注 `v9.9.9+`

## 11. woa-server res-sync 手动触发接口

- [x] 11.1 创建 `cmd/woa-server/service/res-sync/apply_recommend.go`：实现 `SyncApplyRecommend` Handler，带权限校验（`ZiyanCvmCreate` + `Find`），复用 cron 任务 `Do()` 入口
- [x] 11.2 在 `cmd/woa-server/service/res-sync/service.go` 注册路由（`GetURL()` = `/apply_recommend/sync`，对外 `POST /api/v1/woa/apply_recommend/sync`）
