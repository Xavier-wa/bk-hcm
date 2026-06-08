# Change: 主机申领机型推荐 - 离线统计与查询接口

## Why

主机申领 AI Agent 当前在用户进入对话时才实时扫描申请单据，存在冷启动延迟，且缺乏基于历史偏好的主动推荐能力。本变更建立"离线统计 + 持久化 + 查询接口"的数据管道，作为推荐系统的数据生产基础，让 Agent 拿到三元组（需求类型/地域/机型）即可即时渲染推荐卡片，无需实时计算。本变更是父需求"机型自动推荐"的数据生产侧；兄弟需求负责实时库存校验与综合评分。

## What Changes

- **新增 woa-server cron 任务** `ApplyRecommendOfflineTask`：按配置间隔（默认 720 分钟）从 `ziyan_cvm_device_info`（已交付设备表）拉取近 N 天（`updated_at >= now - lookbackDays`）交付记录，补全地域信息（`cloud_region` 为空时按 `cloud_zone` / `zone_name` 查 zone 表映射）后，在内存中按用户/业务维度聚合 Top-K 三元组，分别"分组 Delete + BatchCreate"写入两张推荐表；同一任务循环结束后清理本轮未触达的过期行
- **新增 2 张 MySQL 表**：
  - `ziyan_cvm_apply_user_recommend`：用户维度（`(bk_biz_id, bk_username)` 联合 key）下的 Top-K 推荐
  - `ziyan_cvm_apply_biz_recommend`：业务维度（`bk_biz_id`）下的 Top-K 推荐，作为用户维度数据不足时的补充
- **新增 data-service 接口**：两张表各一套标准 CRUD，分两层：
  - **DAO 层**（`pkg/dal/dao/cvm-apply/`）：`CreateWithTx` / `UpdateWithTx` / `List` / `DeleteWithTx`，与项目现有 DAO 接口形态一致
  - **data-service HTTP client**（`pkg/client/data-service/tcloud-ziyan/`）：`BatchCreate` / `List` / `BatchUpdate` / `BatchDelete`，与同目录 `ZiyanCvmDeviceInfoClient` 命名保持一致
  - 所有业务策略（Top-K 查询、地域补全、按 `updated_at` 清理、分组 Delete+BatchCreate 编排、按人补业务）全部在 woa-server logics 层实现，data-service 接口只做通用 CRUD，不感知推荐逻辑
- **新增 woa-server HTTP 推荐查询接口** `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/top`：`bk_biz_id` 通过路径参数传入，请求体仅含 `bk_username` 与 `limit`，调用前经业务访问鉴权（`Biz` / `Access`）；先取用户表 Top-`limit`，不足时拉业务表 Top-`limit` 后在内存中按 `(require_type, region, device_type)` 三元组去重补足；user 行与 biz 行不全局混排，由 `source` 字段区分来源
- **新增 woa-server res-sync 手动触发接口** `POST /api/v1/woa/apply_recommend/sync`：复用 cron 任务 `Do()` 入口手动触发一次离线统计，带权限校验（`ZiyanCvmCreate` + `Find`）
- **新增配置段** `applyRecommend`：包含 `interval` / `lookbackDays` / `maxRows`，触发间隔、回溯窗口、Top-K 上限完全配置驱动，同步至 helm values
- **多副本互斥保护**：复用现有 `serviced.State.IsMaster()` 模式，非 master 节点直接跳过 cron 执行
- **不含**：MCP Server 工具封装（由 agent 团队在 agent 仓库交付）、近似/等效替代机型推荐（后期人工配置表落地）、实时库存校验与综合评分（兄弟需求）

## Capabilities

### New Capabilities

- `apply-recommend`：主机申领历史偏好的离线统计、持久化与 Top-N 推荐查询能力。涵盖 cron 数据生产、按人补业务的查询编排，但不涉及 MCP 协议封装、实时库存校验与综合评分；data-service 层只承诺通用 CRUD，所有推荐相关的业务策略由 woa-server logics 实现

### Modified Capabilities

（无）

## Impact

- **新增代码**：
  - `cmd/woa-server/task/apply_recommend.go`（cron 任务，参考 `device_capacity.go` 模式）
  - `cmd/woa-server/service/cvm/recommend.go`（Top-N 查询 HTTP handler）
  - `cmd/woa-server/service/res-sync/apply_recommend.go`（手动触发 HTTP handler，带权限校验）
  - `cmd/woa-server/logics/applyrecommend/`（`aggregator.go` 聚合器 + `logics.go` 主流程编排、地域补全、按分组写入与过期清理）
  - `cmd/data-service/service/cvm-apply/cvm-apply-user-recommend/`、`cvm-apply-biz-recommend/`（2 张表的 service 层）
  - `pkg/dal/dao/cvm-apply/`（2 张表的 DAO 实现，接口 `CreateWithTx` / `UpdateWithTx` / `List` / `DeleteWithTx`）
  - `pkg/dal/table/cvm-apply/`（2 张表的 model）
  - `pkg/api/data-service/cvm-apply/ziyan_cvm_apply_recommend.go`、`pkg/api/woa-server/cvm_apply_recommend.go`（请求/响应类型）
  - `pkg/client/data-service/tcloud-ziyan/`（`ziyan_cvm_apply_user_recommend.go`、`ziyan_cvm_apply_biz_recommend.go` client 方法）
- **配置与脚本**：
  - `scripts/sql/9999_20260527_1500_apply_recommend.sql`（建表 + id_generator 注册）
  - `pkg/dal/table/table.go`（2 个表名常量 + `TableMap`）
  - `pkg/criteria/enumor/cron_task.go`（新增 `CronTaskApplyRecommendOffline` 常量）
  - `pkg/criteria/enumor/woa_ziyan.go`（新增 `ApplyRecommendSource` 枚举，值 `user` / `biz`）
  - `pkg/dal/dao/cloud/zone/zone.go`（地域补全所需的 zone 查询能力）
  - `pkg/cc/ziyan_types.go`（新增 `ApplyRecommend` 配置结构体 + `trySetDefault`）+ `pkg/cc/service.go`（挂载并设默认值）+ `cmd/woa-server/etc/woa_server.yaml`（`applyRecommend` 配置段）
  - `docs/support-file/helm/values.yaml`、`templates/woaserver/configmap.yaml`（同步 helm values）
- **新增对外接口**：2 个 HTTP 接口 —— `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/top`（接口文档 `docs/api-docs/web-server/docs/biz/get_apply_recommend_top.md`，版本 `v9.9.9+`）与 `POST /api/v1/woa/apply_recommend/sync`（手动触发）
- **数据规模**：用户表上限 ~150w 行；业务表上限 ~3k 行（基于 300 业务 × 500 用户 × maxRows=5）
- **依赖**：复用 `pkg/cron` 调度框架、`serviced.State` master 选举、zone 表地域映射；不引入新外部依赖
- **风险点 / 待决策项**（详见 `design.md`）：
  - 90 天交付记录全量拉入内存的 OOM 风险（全量 vs 流式聚合）
  - 过期清理触发大批量 DELETE 的锁/binlog 影响（一次性 DELETE vs 分批限速 vs swap 表）
  - Upsert 编排策略选型（`INSERT ... ON DUPLICATE KEY UPDATE` vs 分组 `Delete + Insert` vs `List + Diff`），且必须保证即使 `count` 未变也刷新 `updated_at`，否则 F-006 过期清理会误删
  - cron 编排原子性：写 user 表成功、写 biz 表失败时如何避免半成品数据（AC-S03）
