# apply-recommend Specification

## Purpose
为主机申领提供基于历史已交付设备的离线 Top-N 推荐能力：通过 woa-server 定时任务从已交付设备数据中按用户维度与业务维度聚合高频申领组合，持久化推荐结果并定期清理过期数据，对外提供 Top-N 推荐查询与手动触发统计接口，帮助用户在申领时快速选择常用的资源组合。

## Requirements

### Requirement: 离线推荐统计定时任务
系统 SHALL 在 woa-server 中注册名为 `apply_recommend_offline` 的 cron 任务，任务执行间隔由配置 `applyRecommend.interval`（单位：分钟）驱动，多副本部署时仅 master 节点执行，非 master 节点跳过。

#### Scenario: master 节点执行任务
- **WHEN** cron 触发时当前节点为 master
- **THEN** 任务正常执行完整流程（拉取 → 聚合 → 写库 → 清理）

#### Scenario: 非 master 节点跳过
- **WHEN** cron 触发时当前节点非 master
- **THEN** 任务立即返回，不执行任何 DB 操作

#### Scenario: 配置驱动调度间隔
- **WHEN** `applyRecommend.interval` 设置为 720
- **THEN** 任务两次执行之间的间隔为 720 分钟

### Requirement: 已交付设备数据源查询
系统 SHALL 从 `ziyan_cvm_device_info` 表分页拉取满足以下条件的记录作为统计源数据：`is_delivered = true` 且 `updated_at >= now - lookbackDays`；`lookbackDays` 由配置驱动，默认 90 天；分页拉取使用默认分页（每页 500 行）。

#### Scenario: 正常拉取
- **WHEN** cron 任务执行，`lookbackDays=90`
- **THEN** 仅拉取 `is_delivered=true` 且 `updated_at >= now-90d` 的记录，超出范围的记录不纳入聚合

#### Scenario: 拉取失败
- **WHEN** 分页查询 DB 返回错误
- **THEN** 记录 Error 日志，本轮任务终止，不写入任何推荐数据

### Requirement: 地域信息补全
系统 SHALL 在聚合前补全源记录的地域。聚合三元组中的 region 取自记录的 `cloud_region`；当 `cloud_region` 为空时，系统 SHALL 按 `cloud_zone` 或 `zone_name` 批量查询 zone 表（`ListZoneExt`）映射出 region 后回填；补全后 `cloud_region` 仍为空的记录记录 Error 日志并跳过，不纳入聚合。

#### Scenario: 通过可用区映射地域
- **WHEN** 源记录 `cloud_region` 为空但 `cloud_zone`（或 `zone_name`）有值
- **THEN** 通过 zone 表将其映射为 region 并回填 `cloud_region`，用于后续聚合

#### Scenario: 地域无法补全
- **WHEN** 源记录 `cloud_region` 为空且 `cloud_zone` / `zone_name` 均无法映射出 region
- **THEN** 记录 Error 日志并跳过该记录，不纳入聚合

### Requirement: 用户维度 Top-K 聚合
系统 SHALL 以 `(bk_biz_id, bk_username, require_type, region, device_type)` 五元组为 key 对源数据计数（其中 region 取自补全后的 `cloud_region`），按 `(bk_biz_id, bk_username)` 二级分组后每组按 `count` 倒序取前 `maxRows` 条（`maxRows` 由配置驱动，默认 5）。

#### Scenario: 正常聚合
- **WHEN** 用户 A 在业务 X 下有 8 条交付，分布于 3 个不同三元组
- **THEN** 用户 A 在业务 X 下生成 ≤ 3 条推荐，按 count 倒序，且不超过 maxRows 条

#### Scenario: 超出 maxRows
- **WHEN** 某 (biz, user) 分组下有 7 个不同三元组
- **THEN** 仅保留 count 最高的 maxRows 条，其余丢弃

### Requirement: 业务维度 Top-K 聚合
系统 SHALL 以 `(bk_biz_id, require_type, region, device_type)` 四元组为 key 对同一批源数据计数（其中 region 取自补全后的 `cloud_region`），按 `bk_biz_id` 分组后每组按 `count` 倒序取前 `maxRows` 条，作为用户维度数据不足时的补充候选。

#### Scenario: 正常聚合
- **WHEN** 业务 X 下所有用户合计 100 条交付，分布于 7 个不同三元组
- **THEN** 业务 X 生成 ≤ 7 条推荐，按 count 倒序，且不超过 maxRows 条

### Requirement: 推荐数据持久化
系统 SHALL 将聚合结果持久化到两张 MySQL 推荐表（`ziyan_cvm_apply_user_recommend` / `ziyan_cvm_apply_biz_recommend`），采用"先 Delete 再 BatchCreate"方式按分组写入：用户表以 `(bk_biz_id, bk_username)` 分组、业务表以 `bk_biz_id` 分组，先删除该分组的旧推荐行再批量创建新行。写入通过 data-service 标准 CRUD 接口（`BatchDelete` / `BatchCreate`）完成。

#### Scenario: 用户表写入
- **WHEN** cron 聚合完成，生成新 Top-K
- **THEN** 对每个 (bk_biz_id, bk_username) 分组，先按 bk_biz_id + bk_username 删除旧推荐行，再批量创建新行

#### Scenario: 业务表写入
- **WHEN** cron 聚合完成，生成新 Top-K
- **THEN** 对每个 bk_biz_id 先删除该 biz 所有旧推荐行，再批量创建新行

#### Scenario: 写入失败
- **WHEN** 某分组的 BatchDelete 或 BatchCreate 返回错误
- **THEN** 记录 Error 日志，本轮任务终止，不继续写入其余分组

### Requirement: 过期推荐数据清理
系统 SHALL 在每轮写入完成后，分批删除两张推荐表中 `updated_at < 本轮任务 startTime` 的行，防止长期无申领活动的用户/业务的过期推荐留存。分批删除每批不超过 500 行。

#### Scenario: 业务长期无交付
- **WHEN** 业务 X 在过去 90 天内无任何交付记录，且 DB 中存有其旧推荐行
- **THEN** 本轮 cron 不写入 X 的新数据；清理步骤将 X 的旧推荐行（`updated_at < startTime`）全部删除

#### Scenario: 清理失败不阻断主流程
- **WHEN** 过期清理的 Delete 返回错误
- **THEN** 记录 Error 日志，本轮任务以错误结束；旧数据留存至下次 cron 清理，不影响推荐查询的正确性

### Requirement: Top-N 推荐查询接口
系统 SHALL 在 woa-server 业务路由下提供 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/top` 接口，`bk_biz_id` 通过路径参数传入，请求体仅含 `bk_username` 与 `limit`；调用前 SHALL 通过业务访问鉴权（`Biz` / `Access`），鉴权失败直接返回。接口按 `(bk_biz_id, bk_username, limit)` 返回 Top-N 推荐三元组，策略为"先人后业务补足"：先取用户表 Top-limit，不足时从业务表 Top-limit 中在内存去重补足，user 行与 biz 行不全局混排，`source` 字段区分来源（枚举 `user` / `biz`，定义于 `pkg/criteria/enumor/woa_ziyan.go`）。

#### Scenario: 用户数据充足
- **WHEN** 调用 API 路径传 `bk_biz_id=200`、请求体传 `bk_username=alice, limit=5`，用户表有 5 条记录
- **THEN** 不查业务表，返回 5 条 source=user 的推荐

#### Scenario: 用户数据不足，业务数据补足
- **WHEN** 调用 API 传 `limit=5`，用户表有 3 条，业务表有足够数据
- **THEN** 返回 5 条：前 3 条 source=user，后 2 条 source=biz，且 biz 行的三元组与 user 行无重叠

#### Scenario: 新用户无任何历史记录
- **WHEN** 用户表无该用户的任何记录
- **THEN** 直接返回业务表 Top-limit 条，source=biz，无需去重

#### Scenario: 总数不足 limit
- **WHEN** 用户表 1 条 + 业务表去重后 1 条，limit=5
- **THEN** 返回 2 条，不强凑，不报错

#### Scenario: 参数校验失败
- **WHEN** 请求 limit=21（超出 max=20）、bk_username 为空，或路径 bk_biz_id ≤ 0
- **THEN** 返回 InvalidParameter 错误

#### Scenario: 业务访问鉴权失败
- **WHEN** 调用方对路径中的 bk_biz_id 不具备业务访问权限
- **THEN** 返回鉴权错误，不查询任何推荐数据

### Requirement: 配置驱动的任务参数
系统 SHALL 在 `woa_server.yaml` 中新增 `applyRecommend` 配置段，所有任务参数（interval / lookbackDays / maxRows）均可在不重新编译的情况下通过配置调整，并同步更新 helm values。

#### Scenario: 使用默认配置
- **WHEN** 未显式配置 `applyRecommend`，使用默认值
- **THEN** `interval=720, lookbackDays=90, maxRows=5` 生效

#### Scenario: 自定义配置
- **WHEN** 配置 `interval=360, lookbackDays=30, maxRows=10`
- **THEN** cron 每 360 分钟触发一次，只统计近 30 天数据，每分组保留 Top-10

### Requirement: 离线统计手动触发接口
系统 SHALL 在 woa-server res-sync 提供 `POST /api/v1/woa/apply_recommend/sync` 接口，用于手动触发一次离线统计任务；调用方需具备自研云 CVM 创建资源的查询权限（`ZiyanCvmCreate` + `Find`）。该接口复用 cron 任务的 `Do()` 入口，仍受 master 节点判断约束。

#### Scenario: 有权限手动触发
- **WHEN** 具备权限的调用方请求该接口且当前节点为 master
- **THEN** 同步执行一次完整离线统计流程并返回结果

#### Scenario: 无权限触发
- **WHEN** 调用方不具备所需权限
- **THEN** 返回鉴权错误，不执行统计
