# apply-recommend Specification

## Purpose
为主机申领提供基于历史已交付设备的离线 Top-N 推荐能力：通过 woa-server 定时任务从已交付设备数据中按用户维度与业务维度聚合高频申领组合，持久化推荐结果并定期清理过期数据，对外提供 Top-N 推荐查询与手动触发统计接口，帮助用户在申领时快速选择常用的资源组合。

## Requirements

### Requirement: 推荐相关表 image_id 字段扩展
系统 SHALL 在 `ziyan_cvm_device_info`、`ziyan_cvm_apply_user_recommend`、`ziyan_cvm_apply_biz_recommend` 三张表新增 `image_id` 字段（`varchar(64) NOT NULL DEFAULT ''`，兼容历史空值）；两张推荐表的唯一键 SHALL 由 `...|device_type` 调整为 `...|device_type|image_id`，使最小推荐单元从三元组变为含镜像的四元组。

#### Scenario: 表结构包含 image_id
- **WHEN** 执行 SQL 迁移后查看三张表结构
- **THEN** 三张表均存在 `image_id` 字段，两张推荐表的唯一键包含 `image_id`

#### Scenario: 历史空值兼容
- **WHEN** 迁移在已有数据的表上执行
- **THEN** 存量行 `image_id` 默认为空串 `''`，不破坏既有唯一性约束

### Requirement: 设备交付写入透传 image_id
系统 SHALL 在设备生成/交付写入链路中透传镜像：构建设备信息时 `image_id` 取自 `order.Spec.ImageId`，经 `types.DeviceInfo` 写入 `ziyan_cvm_device_info.image_id`，保证新交付设备记录自带镜像信息。

#### Scenario: 新交付设备携带镜像
- **WHEN** 一笔含 `image_id` 的子单完成设备生成并落库
- **THEN** 对应 `ziyan_cvm_device_info` 记录的 `image_id` 等于子单申领的镜像

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

### Requirement: 离线聚合前过滤不支持的项目类型
系统 SHALL 在自研云（vendor: tcloud-ziyan）离线推荐统计将已交付设备记录按 `(require_type, region, device_type, image_id)` 聚合并写入推荐表之前，判定每条记录的项目类型是否属于当前受支持集合。判定口径 MUST 与创建推荐数据时的 `RequireType.Validate()` 同源，受支持集合为后端代码枚举：常规项目(1) / 春节保障(2) / 机房裁撤(3) / 滚服项目(6) / 小额绿通(7) / 春保资源池(8) / 短租项目(9)。不属于该集合的取值（含 0、4、5 及任何历史下线值）MUST 视为不支持。

判定为不支持的记录 MUST 直接跳过：不参与用户维度与业务维度计数，不写入 `ziyan_cvm_apply_user_recommend` / `ziyan_cvm_apply_biz_recommend`。滚服项目(6) 属于受支持集合，MUST NOT 被本过滤剔除。判定 MUST 在进程内完成，SHALL NOT 新增远程查询，SHALL NOT 修改 `ziyan_cvm_device_info` 源表，SHALL NOT 将不支持类型替换为其他类型。

被跳过的记录 MUST 逐条输出 Warn 级别日志，内容 MUST 包含该记录 ID、其 `require_type` 取值，以及 rid。系统 SHALL NOT 为此过滤额外输出汇总统计。

某个 (业务, 用户) 或 (业务) 分组在过滤后无任何候选时，本轮 MUST NOT 为该分组写入任何行（含 `require_type=0` 或其他占位行）；该分组的历史推荐行 MUST 按既有过期清理规则（`updated_at < 本轮任务开始时间`）删除。

本过滤 MUST NOT 改变在线读取接口 `ApplyRecommendTop` / `ApplyRecommendByStatic` / `ApplyRecommendByPlan` 的请求与响应结构；MUST NOT 改动读取侧既有过滤逻辑（含 `ApplyRecommendByStatic` 不返回滚服项目）。`ApplyRecommendByPlan` 候选不来自推荐表，不受本过滤影响。

#### Scenario: 不支持的项目类型不参与聚合且任务成功
- **GIVEN** 历史设备记录中存在一条 `require_type` 不在受支持集合内（如已下线类型或 0）的已交付记录
- **WHEN** 离线推荐任务执行
- **THEN** 该记录不参与聚合，两张推荐表中不出现该项目类型的行，任务整体执行成功，且日志中无 `unsupported require type` 创建失败

#### Scenario: 全部为受支持类型时聚合结果不变
- **GIVEN** 历史设备记录的 `require_type` 全部为受支持集合内的取值
- **WHEN** 离线推荐任务执行
- **THEN** 聚合与写入结果与增加本过滤前完全一致（同一份数据源下，两张推荐表的行数与内容逐行相同）

#### Scenario: 滚服项目不被本次过滤剔除
- **GIVEN** 历史设备记录中存在 `require_type=6`（滚服项目）的已交付记录
- **WHEN** 离线推荐任务执行
- **THEN** 该记录照常参与聚合并写入推荐表，不被本次过滤剔除

#### Scenario: 越界取值跳过并输出 Warn
- **GIVEN** 历史设备记录的 `require_type` 为 0 或 4、5 等不在受支持集合内的取值
- **WHEN** 离线推荐任务执行
- **THEN** 该记录被跳过，且日志中可查到对应 Warn 记录（含记录 ID 与该项目类型取值）

#### Scenario: 整组被过滤不写入占位行
- **GIVEN** 某 (业务, 用户) 分组下全部历史记录的项目类型都不受支持
- **WHEN** 离线推荐任务执行
- **THEN** 该分组本轮不写入任何行，且不产生 `require_type=0` 或其他占位行；其历史推荐行在过期清理后被删除

#### Scenario: 源表记录不被修改
- **GIVEN** 任务执行完成
- **WHEN** 查询 `ziyan_cvm_device_info`
- **THEN** 源表记录数与 `require_type` 取值均未被修改

#### Scenario: 读取推荐表链路结果正确且读侧滚服规则不变
- **GIVEN** 推荐表已按新规则重建
- **WHEN** 依次调用 `ApplyRecommendTop`、`ApplyRecommendByStatic`
- **THEN** 返回项的项目类型均属于受支持集合，接口响应结构与修复前一致，且 `ApplyRecommendByStatic` 仍按既有读侧规则不返回滚服项目

### Requirement: 用户维度 Top-K 聚合
系统 SHALL 以 `(bk_biz_id, bk_username, require_type, region, device_type, image_id)` 六元组为 key 对源数据计数（其中 region 取自补全后的 `cloud_region`，image_id 取自源记录的 `image_id`），按 `(bk_biz_id, bk_username)` 二级分组后每组按 `count` 倒序取前 `maxRows` 条（`maxRows` 由配置驱动，默认 5）。

#### Scenario: 正常聚合
- **WHEN** 用户 A 在业务 X 下有 8 条交付，分布于 3 个不同四元组（含镜像）
- **THEN** 用户 A 在业务 X 下生成 ≤ 3 条推荐，按 count 倒序，且不超过 maxRows 条

#### Scenario: 同机型不同镜像独立计数
- **WHEN** 用户 A 在相同 `(require_type, region, device_type)` 下使用了两个不同 `image_id`
- **THEN** 两个镜像各自成为独立候选行分别计数，不合并

#### Scenario: 超出 maxRows
- **WHEN** 某 (biz, user) 分组下有 7 个不同四元组
- **THEN** 仅保留 count 最高的 maxRows 条，其余丢弃

### Requirement: 业务维度 Top-K 聚合
系统 SHALL 以 `(bk_biz_id, require_type, region, device_type, image_id)` 五元组为 key 对同一批源数据计数（其中 region 取自补全后的 `cloud_region`，image_id 取自源记录的 `image_id`），按 `bk_biz_id` 分组后每组按 `count` 倒序取前 `maxRows` 条，作为用户维度数据不足时的补充候选。

#### Scenario: 正常聚合
- **WHEN** 业务 X 下所有用户合计 100 条交付，分布于 7 个不同五元组（含镜像）
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
系统 SHALL 在 woa-server 业务路由下提供 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/top` 接口，`bk_biz_id` 通过路径参数传入，请求体仅含 `bk_username` 与 `limit`；调用前 SHALL 通过业务访问鉴权（`Biz` / `Access`），鉴权失败直接返回。接口按 `(bk_biz_id, bk_username, limit)` 返回 Top-N 推荐结果，每条结果 SHALL 包含 `image_id`；策略为"先人后业务补足"：先取用户表 Top-limit，不足时从业务表 Top-limit 中在内存去重补足，去重键为 `(require_type, region, device_type, image_id)` 四元组，user 行与 biz 行不全局混排，`source` 字段区分来源（枚举 `user` / `biz`，定义于 `pkg/criteria/enumor/woa_ziyan.go`）。

#### Scenario: 用户数据充足
- **WHEN** 调用 API 路径传 `bk_biz_id=200`、请求体传 `bk_username=alice, limit=5`，用户表有 5 条记录
- **THEN** 不查业务表，返回 5 条 source=user 的推荐，每条含 image_id

#### Scenario: 用户数据不足，业务数据补足
- **WHEN** 调用 API 传 `limit=5`，用户表有 3 条，业务表有足够数据
- **THEN** 返回 5 条：前 3 条 source=user，后 2 条 source=biz，且 biz 行的四元组（含 image_id）与 user 行无重叠

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

### Requirement: 离线偏好叠加库存的在线推荐接口
系统 SHALL 在 woa-server 业务路由下提供 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/by_static_recommend` 接口，`bk_biz_id` 通过路径参数传入。调用前 SHALL 通过业务访问鉴权（`Biz` / `Access`），鉴权失败直接返回。接口在离线静态推荐候选之上叠加库存校验与默认值补全，返回若干推荐方案，每个方案 SHALL 仅包含 1 个子单，并携带 `source` 字段标识推荐来源（`user`-用户维度 / `biz`-业务维度）；子单 SHALL 包含 需求类型、地域、可用区、机型、镜像(image_id)、资源分配方式、申请数量、计费模式、系统盘、数据盘。滚服项目（`require_type=6`）的子单 SHALL 额外携带五个滚服字段（`charge_months`、`bk_asset_id`、`inherit_instance_id`、`billing_start_time`、`billing_expire_time`）。本接口为独立新接口，不改动既有 `GetBizApplyRecommendTop`。

请求体必填 `bk_username` 与返回方案数 `limit`；可选 A 类过滤字段（需求类型 / 地域 / 机型 / 镜像 image_id）与 B 类覆盖字段（可用区 / 资源分配方式 / 申请数量）。`require_type` SHALL 接受滚服项目(6)——原先对滚服直接返回 `require type ... is temporarily not supported` 的参数校验拒绝 SHALL 被移除。

#### Scenario: 有静态推荐数据且库存充足、未传 zone
- **WHEN** 存在该业务/用户的静态推荐数据、对应候选库存充足，且请求未传 zone
- **THEN** 返回按 `count` 倒序的若干单子单方案，每个方案 `zone=全部(all)`、`res_assign=有资源区域优先(1)`，并补全 image_id / 计费模式 / 系统盘 / 数据盘等默认值

#### Scenario: 所有候选库存不足
- **WHEN** 所有静态推荐候选在库存校验后均不满足申请数量
- **THEN** 返回空方案列表，不报错

#### Scenario: 参数校验失败
- **WHEN** 请求 `bk_username` 为空、`limit` 超出允许范围，或路径 `bk_biz_id ≤ 0`
- **THEN** 返回 InvalidParameter 错误

#### Scenario: 业务访问鉴权失败
- **WHEN** 调用方对路径中的 `bk_biz_id` 不具备业务访问权限
- **THEN** 返回鉴权错误，不查询任何推荐数据

#### Scenario: 接受滚服需求类型
- **GIVEN** 请求传 `require_type=6`（滚服项目）
- **WHEN** 调用该接口
- **THEN** 参数校验通过，不再返回"require type ... is temporarily not supported"

### Requirement: 静态推荐候选查询与 A 类过滤
系统 SHALL 复用「先人后业务补足」的推荐查询逻辑获取候选：先查用户维度 `ziyan_cvm_apply_user_recommend`，不足时从业务维度 `ziyan_cvm_apply_biz_recommend` 去重补足，去重键为 `(require_type, region, device_type, image_id)`。A 类入参（需求类型 / 地域 / 机型 / 镜像 image_id）中非空者 SHALL 作为过滤条件在查询层收窄候选；现有 `GetBizApplyRecommendTop` 不传 A 类过滤时行为 SHALL 保持不变。

候选查询 SHALL NOT 再排除滚服项目——原先无条件附加的 `require_type != 滚服(6)` DB 过滤规则 SHALL 被移除，滚服候选与其他需求类型的候选一视同仁参与后续流程。

#### Scenario: A 类过滤收窄候选
- **WHEN** 请求传入 `region` 与 `device_type` 过滤条件
- **THEN** 仅返回匹配该地域与机型的静态推荐候选参与后续库存校验与组装

#### Scenario: A 类过滤传了但查不到匹配
- **WHEN** 请求传入 A 类过滤条件，但静态推荐表中无任何匹配候选
- **THEN** 直接返回空方案列表（不强构、不降级）

#### Scenario: 既有 Top-N 查询行为不变
- **WHEN** `GetBizApplyRecommendTop` 调用推荐查询且不传 A 类过滤
- **THEN** 查询条件与返回结果与扩展前完全一致

#### Scenario: 滚服候选不再被 DB 过滤排除
- **GIVEN** 静态推荐表中存在该业务的 `require_type=6` 记录
- **WHEN** 调用静态推荐候选查询
- **THEN** 这些滚服候选出现在候选集合中，不再被 `require_type != 6` 规则排除

### Requirement: 库存校验按需求类型分支
系统 SHALL 使用静态表 `device_capacity` 校验候选库存，要求 `capacity ≥ 申请数量`。校验前 SHALL 先判断候选的 `require_type.NotNeedVerifyCapacity()`：为真（如小额绿通）则跳过库存校验直接保留候选；为假则按库存校验结果保留或剔除。库存查询 SHALL 批量进行（按候选涉及的 require_type / region / device_type 批量查询后内存过滤），不含镜像维度。

#### Scenario: 免库存校验的需求类型
- **WHEN** 候选的需求类型满足 `NotNeedVerifyCapacity()`（小额绿通）
- **THEN** 跳过 device_capacity 校验，直接保留该候选

#### Scenario: 候选在 device_capacity 查不到
- **WHEN** 需校验库存的候选在 `device_capacity` 中查不到匹配记录
- **THEN** 视为库存不足，剔除该候选

#### Scenario: 库存不足剔除
- **WHEN** 需校验库存的候选 `capacity < 申请数量`
- **THEN** 剔除该候选，不进入方案组装

### Requirement: 可用区与资源分配方式联动
系统 SHALL 根据是否传入 zone 联动决定可用区与资源分配方式：未传 zone 时返回 `zone=全部(all)` 且 `res_assign` 取 B 类入参值（未传入参时默认 `有资源区域优先(1)`），库存校验仅做候选过滤（region 下任一 zone 满足阈值即保留），不回填具体 zone；传入 zone 时返回该具体 zone 且不返回 `res_assign`，库存按该 zone 校验，不足则剔除方案。

#### Scenario: 未传 zone 且未传 res_assign
- **WHEN** 请求未传 zone 和 res_assign，且 region 下存在至少一个 zone 满足库存阈值
- **THEN** 方案返回 `zone=全部(all)`、`res_assign=1`（有资源区域优先），不回填具体 zone

#### Scenario: 未传 zone 但传入 res_assign
- **WHEN** 请求未传 zone，但传入了 B 类 `res_assign` 字段
- **THEN** 方案返回 `zone=全部(all)`，`res_assign` 取入参值

#### Scenario: 传入具体 zone
- **WHEN** 请求传入具体 zone
- **THEN** 方案返回该 zone、不返回 `res_assign`；该 zone 库存不足时剔除该方案

### Requirement: 默认值补全与申请数量配置
系统 SHALL 为每个方案补全下单必需的默认值：image_id 取自静态推荐表；计费模式对非滚服需求类型固定 `PREPAID`，对滚服项目（`require_type=6`）SHALL 取自继承固资的 `instance_charge_type`、SHALL NOT 写死 `PREPAID`；系统盘默认 `CLOUD_PREMIUM / 100G / 1 块`；数据盘默认 `CLOUD_PREMIUM / 500G / 1 块`。申请数量 SHALL 取入参，未传时取 `cc.ApplyRecommend` 配置的默认值（默认 10），且 SHALL NOT 使用推荐表 `count` 作为申请数量；`count` 仅用于候选排序。新增配置项 SHALL 同步至 `etc/woa_server.yaml` 与 helm values。

#### Scenario: 申请数量取默认配置
- **WHEN** 请求未传申请数量
- **THEN** 各方案的申请数量取 `cc.ApplyRecommend` 默认值（默认 10），库存阈值同样按该值校验

#### Scenario: 申请数量取入参
- **WHEN** 请求传入申请数量
- **THEN** 各方案的申请数量与库存校验阈值均取该入参值

#### Scenario: 默认字段补全
- **WHEN** 非滚服候选通过库存校验进入方案组装
- **THEN** 方案补全 image_id（静态表）、计费模式 PREPAID、系统盘 CLOUD_PREMIUM/100G/1、数据盘 CLOUD_PREMIUM/500G/1

#### Scenario: 滚服计费模式不走默认值
- **GIVEN** 一个滚服候选已完成继承固资补全，固资的 `instance_charge_type` 为 `POSTPAID_BY_HOUR`
- **WHEN** 该候选进入方案组装
- **THEN** 方案的计费模式为 `POSTPAID_BY_HOUR`，不被默认值 `PREPAID` 覆盖

### Requirement: 预测余量叠加库存的在线推荐接口
系统 SHALL 在 woa-server 业务路由下提供 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/by_plan` 接口，`bk_biz_id` 通过路径参数传入。调用前 SHALL 通过业务访问鉴权（`Biz` / `Access`），鉴权失败直接返回。接口在**实时预测余量候选**之上叠加库存校验与默认值补全，返回若干推荐方案，每个方案 SHALL 仅包含 1 个子单；子单 SHALL 包含 需求类型、地域、可用区、机型、镜像(image_id)、资源分配方式、申请数量、计费模式、系统盘、数据盘。本接口候选来源为预测余量（不读静态推荐表），与接口1 相互独立、互不改动既有 `GetBizApplyRecommendTop` / `GetBizApplyRecommendByStatic`。

请求体必填返回方案数 `limit`；可选 A 类过滤字段（需求类型 / 地域 / 机型 / 镜像 image_id）与 B 类覆盖字段（可用区 / 资源分配方式 / 申请数量）。需求类型未传时 SHALL 归一为常规项目（`RequireTypeRegular=1`）参与预测匹配。注：本接口不含 `bk_username` 字段（与接口1 `by_static_recommend` 不同）。

#### Scenario: 预测内有余量且库存充足、未传 zone
- **WHEN** 业务当月预测内池存在可用机型余量、对应候选库存充足，且请求未传 zone
- **THEN** 返回按预测余量倒序的若干单子单方案，每个方案计费模式为包年包月（PREPAID）、`zone=全部(all)`、`res_assign=有资源区域优先(1)`，并补全 image_id / 系统盘 / 数据盘等默认值

#### Scenario: 预测内无候选回退预测外
- **WHEN** 所有 region 的预测内池均无满足门槛的候选，但预测外池存在可用机型余量
- **THEN** 整体回退预测外池，返回方案的计费模式为按量计费（POSTPAID_BY_HOUR）

#### Scenario: 预测或库存全不足
- **WHEN** 预测内与预测外均无满足余量门槛的候选，或全部候选库存不足
- **THEN** 返回空方案列表，不报错

#### Scenario: 参数校验失败
- **WHEN** 请求 `limit` 超出允许范围（< 1 或 > 20），或路径 `bk_biz_id ≤ 0`
- **THEN** 返回 InvalidParameter 错误

#### Scenario: 业务访问鉴权失败
- **WHEN** 调用方对路径中的 `bk_biz_id` 不具备业务访问权限
- **THEN** 返回鉴权错误，不查询任何预测或库存数据

### Requirement: 预测余量候选匹配与余量门槛
系统 SHALL 复用既有预测能力获取候选，不重写预测逻辑：调用 `GetProdResRemainPoolMatch` 取业务当月预测余量池，region 集合取入参 region（非空时仅该 region）或池内出现的全部 region；对每个 region 调用 `GetPlanTypeAvlDeviceTypesV2` 取预测内可用机型与余量核数。预测内全部 region 均无候选时 SHALL 全局回退预测外池。候选 SHALL 满足余量硬门槛 `预测余量核数 ≥ 申请数量 × 机型核数`（机型核数取自 `GetAllDeviceTypeMap`），不满足者剔除；余量仅用于门槛与排序，SHALL NOT 在响应中返回。A 类 `device_type` 入参非空时 SHALL 仅保留该机型候选。

#### Scenario: 余量门槛剔除
- **WHEN** 某预测候选的预测余量核数 `< 申请数量 × 机型核数`
- **THEN** 剔除该候选，不进入库存校验与方案组装

#### Scenario: device_type 过滤无匹配
- **WHEN** 请求传入 A 类 `device_type` 过滤条件，但预测余量候选中无该机型
- **THEN** 直接返回空方案列表（不强构、不降级）

#### Scenario: 地域过滤
- **WHEN** 请求传入 `region`
- **THEN** 仅匹配该 region 的预测余量候选；未传 region 时遍历预测池内出现的全部 region

### Requirement: 预测推荐的库存校验与可用区联动
系统 SHALL 复用与接口1 一致的库存校验逻辑：使用静态表 `device_capacity` 校验候选库存，要求 `capacity ≥ 申请数量`，库存查询不含镜像维度。校验前 SHALL 先判断候选的 `require_type.NotNeedVerifyCapacity()`：为真则跳过库存校验直接保留。可用区与资源分配方式 SHALL 联动：未传 zone 时返回 `zone=全部(all)` 且 `res_assign` 取 B 类入参值（未传时默认 `有资源区域优先(1)`），库存仅做候选过滤（region 下任一 zone 满足阈值即保留），不回填具体 zone；传入 zone 时返回该具体 zone 且不返回 `res_assign`，库存按该 zone 校验，不足则剔除方案。

#### Scenario: 未传 zone 的库存候选过滤
- **WHEN** 请求未传 zone，且 region 下存在至少一个 zone 满足库存阈值
- **THEN** 方案返回 `zone=全部(all)`、`res_assign=1`，不回填具体 zone

#### Scenario: 传入具体 zone 的库存校验
- **WHEN** 请求传入具体 zone
- **THEN** 方案返回该 zone、不返回 `res_assign`；该 zone 库存不足时剔除该方案

#### Scenario: 候选在 device_capacity 查不到
- **WHEN** 需校验库存的候选在 `device_capacity` 中查不到匹配记录
- **THEN** 视为库存不足，剔除该候选

### Requirement: 预测推荐的默认值补全与镜像回查
系统 SHALL 为每个方案补全下单必需的默认值：计费模式随命中预测池来源（预测内 PREPAID / 预测外 POSTPAID_BY_HOUR）；系统盘默认 `CLOUD_PREMIUM / 100G / 1 块`；数据盘默认 `CLOUD_PREMIUM / 500G / 1 块`。镜像 image_id SHALL 按优先级补全：入参 image_id → 按 `(require_type, region, device_type)` 回查历史子单 `ziyan_cvm_apply_suborder` 最近一次非空 image_id → 默认 `cvmapi.DftImageID`。申请数量 SHALL 取入参，未传时取 `cc.ApplyRecommend` 配置的默认值（默认 10），且 SHALL NOT 使用预测余量核数作为申请数量。方案 SHALL 按预测余量倒序去重（去重键 `region|device_type`）后取前 `limit` 个。

#### Scenario: image_id 入参优先
- **WHEN** 请求传入 image_id
- **THEN** 各方案的 image_id 取该入参值，不回查历史子单

#### Scenario: image_id 历史回查
- **WHEN** 请求未传 image_id，且存在该 `(require_type, region, device_type)` 的历史子单含非空 image_id
- **THEN** 各方案的 image_id 取最近一条历史子单的 image_id

#### Scenario: image_id 默认兜底
- **WHEN** 请求未传 image_id 且无可回查的历史子单 image_id
- **THEN** 各方案的 image_id 取默认 `cvmapi.DftImageID`

#### Scenario: 申请数量取默认配置
- **WHEN** 请求未传申请数量
- **THEN** 各方案的申请数量取 `cc.ApplyRecommend` 默认值（默认 10），余量门槛与库存阈值同样按该值校验

### Requirement: 主机申请单据拆分试算接口
系统 SHALL 在 woa-server 业务路由下提供 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/split_suborder` 接口，`bk_biz_id` 通过路径参数传入。调用前 SHALL 通过业务访问鉴权（`Biz` / `Access`），鉴权失败直接返回。接口将一个确定方案按计费模式拆分，组装成 1 个主机申请单据（主单）含多个子单，**纯试算、不落库**。请求体必填字段 SHALL 包含：需求类型、地域、可用区、机型、镜像(image_id)、资源分配方式、申请数量(replicas)、系统盘；可选字段 SHALL 包含：数据盘(data_disk)、`occupied_suborders`（已占用子单数组，用于增量拆分）、由推荐结果透传的五个滚服字段。响应 SHALL 为扁平结构 `{ suborders: [...] }`，每个子单 SHALL 包含 机型、镜像(image_id)、申请数量、地域、可用区、计费模式、需求类型、系统盘、数据盘、资源分配方式；滚服子单 SHALL 额外包含五个滚服字段。本接口与既有 `GetBizApplyRecommendTop` / `GetBizApplyRecommendByStatic` / `GetBizApplyRecommendByPlan` 相互独立、互不改动。

`require_type` SHALL 接受滚服项目(6)——原先对滚服直接返回 `require type ... is temporarily not supported` 的参数校验拒绝 SHALL 被移除，并 SHALL 新增三条滚服专属校验，任一不通过均返回参数校验失败并明确指出是哪个字段，MUST NOT 静默降级为非滚服处理：

1. `inherit_instance_id` 不得为空；
2. `charge_type` 必须是合法计费模式（`PREPAID` / `POSTPAID_BY_HOUR`）——滚服子单的计费模式取自入参而非推导，留空会让子单落到空计费模式；
3. `charge_type == PREPAID` 时 `charge_months` 必须 ≥ 1——包年包月子单没有购买时长无法提单。按量计费无套餐时长，此时 `charge_months` 不校验。

#### Scenario: 滚服缺 charge_type

- **GIVEN** `require_type=6`、`inherit_instance_id` 已填但 `charge_type` 为空的拆单请求
- **WHEN** 调用该接口
- **THEN** 返回参数校验失败并指出 `charge_type` 取值非法

#### Scenario: 滚服包年包月缺购买时长

- **GIVEN** `require_type=6`、`charge_type=PREPAID`、`charge_months=0` 的拆单请求
- **WHEN** 调用该接口
- **THEN** 返回参数校验失败并指出 `charge_months` 必须大于 0

#### Scenario: 滚服按量计费不校验购买时长

- **GIVEN** `require_type=6`、`charge_type=POSTPAID_BY_HOUR`、`charge_months=0` 的拆单请求
- **WHEN** 调用该接口
- **THEN** 参数校验通过

#### Scenario: 鉴权失败
- **WHEN** 调用方无该业务的 `Biz`/`Access` 权限
- **THEN** 接口直接返回鉴权错误，不执行任何拆分计算

#### Scenario: 必填参数缺失
- **WHEN** 请求缺少需求类型 / 地域 / 可用区 / 机型 / 镜像 / 资源分配方式 / 申请数量 / 系统盘中任一字段
- **THEN** 返回 `InvalidParameter` 错误

#### Scenario: 纯试算不落库
- **WHEN** 接口成功返回主单与子单
- **THEN** 系统 SHALL NOT 写入 `ziyan_cvm_apply_order` / `ziyan_cvm_apply_suborder` 等任何表

#### Scenario: 滚服缺 inherit_instance_id
- **GIVEN** `require_type=6` 但 `inherit_instance_id` 为空的拆单请求
- **WHEN** 调用该接口
- **THEN** 返回参数校验失败并明确指出缺 `inherit_instance_id`，不静默按非滚服处理

### Requirement: 按计费模式的子单拆分
系统 SHALL 以**计费模式**为唯一拆分轴，`zone` 原样透传单值（`all` 或具体值），SHALL NOT 按 zone 拆分。预测内余量满足的台数 SHALL 组装为包年包月（PREPAID）子单；预测内装不下、溢出到预测外余量的台数 SHALL 组装为按量计费（POSTPAID_BY_HOUR）子单。一个入参组合 SHALL 最多拆 2 个子单；不需校验预测的需求类型 SHALL 只出 1 个子单，其计费模式对非滚服需求类型为 PREPAID、对滚服项目（`require_type=6`）SHALL 取自入参透传的固资计费模式。各子单的「申请数量(replicas)」SHALL 为拆分后实际分配台数，合计 SHALL ≤ 入参 replicas；数量 ≤ 0 的子单 SHALL 被丢弃。

滚服的短路 SHALL 发生在子单组装环节而非分配算法内：分配结果（预测内 + 预测外）SHALL 先合并为一个总台数，再以入参 `charge_type` 组装出唯一那个子单；总台数 ≤ 0 时 SHALL 返回空子单列表。滚服不校验预测，分配结果恒落在预测内一侧，合并只是为了让"滚服只出 1 个子单"这条约束不依赖分配算法的内部形态。

#### Scenario: 预测内装不下溢出预测外
- **WHEN** 需求类型为常规(1)，申请数量在预测内余量装不下，预测外余量可补足部分
- **THEN** 返回 1 个主单 + 2 个子单（预测内 PREPAID + 预测外 POSTPAID_BY_HOUR），各子单数量合计 ≤ 入参 replicas

#### Scenario: 预测内可全部满足
- **WHEN** 申请数量在预测内余量内即可全部满足
- **THEN** 仅返回 1 个 PREPAID 子单，无 POSTPAID 子单

#### Scenario: 丢弃零数量子单
- **WHEN** 预测外分配台数计算结果为 0
- **THEN** 不返回该 POSTPAID 子单

#### Scenario: 滚服子单计费模式取自透传值
- **GIVEN** 一个 `require_type=6`、透传 `charge_type=POSTPAID_BY_HOUR`、`charge_months=8` 的拆单请求
- **WHEN** 调用拆单接口
- **THEN** 返回的 1 个子单其 `charge_type` 为 `POSTPAID_BY_HOUR`、`charge_months` 为 8，不被推导成 PREPAID

### Requirement: 按需求类型差异化的预测与库存校验
系统 SHALL 复用 `RequireType.NeedVerifyResPlan()` / `NotNeedVerifyCapacity()` 判定校验差异。常规(1)/春保(2)/短租(9) SHALL 校验预测内 + 预测外并查库存，最多 2 子单；机房裁撤(3) SHALL 校验预测但忽略预测内外、余量合并为单池**只算一次**（避免 `GetPlanTypeAvlDeviceTypesV2` 预测内/外对裁撤返回同份余量导致重复计数），输出 1 个 PREPAID 子单并查库存；滚服(6)/春保资源池(8) SHALL NOT 校验预测，输出 1 个子单并查库存；小额绿通(7) SHALL NOT 校验预测且 SHALL 跳过库存校验，输出 1 个 PREPAID 子单。计费模式 SHALL 由系统按预测池来源推导（预测内 PREPAID / 预测外 POSTPAID_BY_HOUR / 不校验预测 = PREPAID），**滚服(6) 例外**：其 `charge_type` 与 `charge_months` SHALL 短路取自入参透传的继承固资信息，不进入预测内外推导；其余字段（地域、可用区、镜像、磁盘、资源分配方式）SHALL 原样透传入参。滚服(6) 的库存约束 SHALL 保持不变（`NotNeedVerifyCapacity()` 对滚服仍返回 false，库存按常规项目口径查），拆单算法本身（`NeedVerifyResPlan()` / `computePlanAvailable` / `allocateSplit` / 库存扣减模型）SHALL NOT 改动。

#### Scenario: 小额绿通跳过库存
- **WHEN** 需求类型为小额绿通(7)
- **THEN** 跳过库存校验，返回 1 个 PREPAID 子单，数量为入参 replicas

#### Scenario: 机房裁撤单池只算一次
- **WHEN** 需求类型为机房裁撤(3)
- **THEN** 预测内外余量合并为单池仅计一次，返回 1 个 PREPAID 子单，且不重复计数预测余量

#### Scenario: 滚服不校验预测
- **WHEN** 需求类型为滚服(6)
- **THEN** 不校验预测，按库存上限返回 1 个子单，其计费模式与购买时长取自入参透传的继承固资信息

#### Scenario: 滚服预测余量为 0 时不被推导为按量计费
- **GIVEN** 一个 `require_type=6`、透传 `charge_type=PREPAID` 的拆单请求，且该业务预测余量为 0
- **WHEN** 调用拆单接口
- **THEN** 仍返回 1 个子单，`charge_type` 不因预测余量为 0 而被推导成 `POSTPAID_BY_HOUR`

#### Scenario: 滚服仍受库存封顶
- **GIVEN** 滚服拆单请求对应机型在该 region 的静态库存小于入参 `replicas`
- **WHEN** 调用拆单接口
- **THEN** 子单数量被库存封顶，行为与常规项目一致，滚服不跳过库存校验

### Requirement: 预测余量门槛与库存数值封顶
系统 SHALL 复用 `GetProdResRemainPoolMatch` + `GetPlanTypeAvlDeviceTypesV2` 取本机型预测内/预测外 RemainCore，机型核数取自 `GetAllDeviceTypeMap()[device_type].CpuCore`；单池可满足台数 SHALL 按 `余量核数 / 机型核数` 向下取整（`nPrepaid` / `nPostpaid`），余量仅用于门槛与拆分、SHALL NOT 在响应中返回。库存校验 SHALL 查 `device_capacity` 取**数值上限**（非布尔判断）对拆分总量按「预测内 → 预测外」顺序封顶；`zone=all` 时 SHALL 逐 zone 查 `device_capacity`，取 region 下各 zone 有效容量之**和**作为上限（zone=all 不钉死可用区、可跨 zone 分摊下单，各 zone 先扣本 zone 占用并截断 0 后求和，不回填具体 zone）；`zone=具体值` 时 SHALL 按该 zone 的 capacity 封顶。数量分配 SHALL 为 `takePrepaid = min(replicas, nPrepaid)`、`takePostpaid = min(replicas - takePrepaid, nPostpaid)`，再用库存上限按「预测内 → 预测外」顺序对总量封顶。可分配总量 < 1 时 SHALL 返回空。

#### Scenario: 库存上限低于预测可满足量
- **WHEN** 预测内可满足 11 台、预测外可满足 8 台，但库存上限为 10 台（zone=all 取各 zone 之和）
- **THEN** 按「预测内 → 预测外」封顶后返回总量 10 台，优先填充 PREPAID 子单

#### Scenario: zone=all 取各 zone 之和容量
- **WHEN** `zone=all`，region 下三个 zone 的 capacity 分别为 3 / 8 / 1
- **THEN** 拆分总量库存上限取 12（各 zone 之和），不回填具体 zone，子单 zone 仍为 all

#### Scenario: 余量与库存均不足返回空
- **WHEN** 预测可满足台数与库存上限计算后可分配总量 < 1
- **THEN** 返回空子单列表

#### Scenario: 请求机型无效返回空
- **WHEN** 请求的 `device_type` 未知或已禁用（cpu_core 取不到，≤ 0）
- **THEN** 记 Warn 并返回空子单列表

### Requirement: 增量拆分的占用扣减
系统 SHALL 支持可选入参 `occupied_suborders`（复用子单结构，扣减仅用到 require_type / region / zone / device_type / charge_type / replicas）。传入后 SHALL 先扣减已占用资源再基于剩余资源计算本次**增量子单**，且 SHALL 只返回新算出的增量子单、SHALL NOT 回显传入的占用子单；不传则等价于原拆分行为。每个传入子单的 `require_type` SHALL 等于本次请求的 `require_type`，否则返回 `InvalidParameter`；`region` 允许不同。传入占用子单的 `device_type` 未知/已禁用（cpu_core 取不到）时 SHALL 直接返回错误。扣减策略为**仅扣减、不校验**传入子单自身能否满足，扣减后余量/库存 SHALL 截断为 0。预测余量扣减 SHALL 落在**机型族级**维度：`occupiedCore[(region, TechnicalClass, CoreType, 预测内/外)] += replicas × cpu_core(子单 device_type)`（族信息取自 `GetAllDeviceTypeMap`，预测内/外由子单 charge_type 决定，机房裁撤合并到同一族级 key），候选有效余量 = `RemainCore − occupiedCore[group]`（截断 0）；库存扣减 SHALL 按 `(require_type, region, device_type, zone)` 聚合台数，其中 `zone=all` 的占用视为**浮动占用**（统一扣减）、具体 zone 的占用只扣对应 zone：请求指定具体 zone Z 时有效库存 = `capacity(Z) − 占用(Z) − 浮动占用(all)`（截断 0），请求 `zone=all` 时各 zone 先扣本 zone 具体占用并截断 0 后求和、再扣浮动占用(all)（截断 0）。小额绿通(7) 场景下 `occupied_suborders` SHALL 不生效（既不校验预测也跳过库存）。

#### Scenario: 同族占用削减全族余量
- **WHEN** 传入一个同 `TechnicalClass`+`CoreType`（同族）的占用子单
- **THEN** 候选机型的有效预测余量按该族占用核数等额削减后再参与拆分

#### Scenario: 占用子单 require_type 不一致
- **WHEN** 某个 `occupied_suborders` 元素的 require_type 与本次请求 require_type 不同
- **THEN** 返回 `InvalidParameter` 错误

#### Scenario: 占用子单机型无效
- **WHEN** 某个 `occupied_suborders` 元素的 device_type 未知或已禁用（cpu_core 取不到）
- **THEN** 直接返回错误，不继续拆分计算

#### Scenario: 只返回增量子单
- **WHEN** 传入 occupied_suborders 并成功计算出增量子单
- **THEN** 响应仅包含本次新算出的增量子单，不回显传入的占用子单

#### Scenario: 绿通场景占用不生效
- **WHEN** 需求类型为小额绿通(7) 且传入 occupied_suborders
- **THEN** 既不扣减预测也不扣减库存，按 replicas 返回 1 个 PREPAID 子单

### Requirement: 滚服字段的推荐契约与非滚服兼容性

`ApplyRecommendSuborder` SHALL 新增五个滚服字段：`charge_months`、`bk_asset_id`、`inherit_instance_id`、`billing_start_time`、`billing_expire_time`，全部带 `omitempty`，且**仅滚服项目（`require_type=6`）填充**。

`ApplyRecommendSplitSubOrderReq` SHALL 同步接收这五个字段**再加 `charge_type`**（共六个滚服相关入参字段），由推荐结果透传进拆单，避免拆单时重新查固资导致前后不一致；`charge_type` 是必须的第六个字段——滚服子单的计费模式不再由预测内外推导，没有它拆单无处取值。六个字段与 `ApplyRecommendSuborder` 上的同名字段类型 MUST 逐一相同。

字段类型 MUST 保证 `omitempty` 在非滚服场景真正生效：`charge_months` 用数值类型（零值命中 `omitempty`，与提单落库的 `ResourceSpec.ChargeMonths` 口径一致），两个时间字段用**指针**类型（Go 的 `encoding/json` 对 struct 类型的 `omitempty` 不生效，直接用 `time.Time` 会让非滚服响应多出 `"0001-01-01T00:00:00Z"` 两个键）。

`ApplyRecommendSuborder.ChargeType` 的语义 SHALL 扩展：滚服场景取自继承固资的 `instance_charge_type`，SHALL NOT 再写死 `PREPAID`。

`charge_months` 与两个时间字段 SHALL 只在取到有效值时写入：`charge_months` 仅在**大于 0** 时写入（`int` → `uint` 的负数转换会溢出成天文数字），时间字段仅在非零值时写入。按量计费固资没有套餐起止时间、已到期固资的剩余月数为非正数，这类零值 MUST NOT 下发。

#### Scenario: 非滚服响应不含五个滚服字段

- **GIVEN** 一个 `require_type=1`（常规项目）的 `ApplyRecommendSuborder` 实例，五个滚服字段均为零值/nil
- **WHEN** 序列化为 JSON
- **THEN** 输出中不含 `charge_months`、`bk_asset_id`、`inherit_instance_id`、`billing_start_time`、`billing_expire_time` 五个键，其余键与改动前逐字段相同

#### Scenario: 拆单入参与子单字段类型自洽

- **GIVEN** `ApplyRecommendSplitSubOrderReq` 已补六个滚服相关字段
- **WHEN** 检查其定义
- **THEN** `charge_type` 与五个滚服字段均与 `ApplyRecommendSuborder` 上的同名字段类型逐一相同

#### Scenario: 非正月数与零时间不下发

- **GIVEN** 一个按量计费的继承固资，无 `billing_expire_time`、剩余月数为负数
- **WHEN** 用它填充滚服子单
- **THEN** 子单的 `charge_months` 保持为 0（序列化时被 `omitempty` 略去）、`billing_expire_time` 保持为 nil，`charge_type` 为 `POSTPAID_BY_HOUR`

#### Scenario: 六种非滚服需求类型的 by_static 响应无回归

- **GIVEN** `require_type` 为常规项目(1) / 春保(2) / 裁撤(3) / 短租(9) / 绿通(7) / 春保资源池(8)
- **WHEN** 调用 `by_static`
- **THEN** 返回的 suborder 不含五个滚服字段，其余字段与改动前逐字段相同

### Requirement: 滚服候选的继承固资补全

`by_static` SHALL 在 `filterCandidatesByCapacity` 之后、`assembleStaticPlans` 之前对滚服候选执行服务端内部补全：按候选的 `device_type` 经 `ListCvmInstanceInfoByDeviceTypes` 反查机型族（`device_group`）→ 按 `region + 机型族` 调继承固资候选查询 logic 取**首条** → 用该固资填充 `charge_type`（取自固资的 `instance_charge_type`）、`charge_months`、`bk_asset_id`、`inherit_instance_id`、`billing_start_time`、`billing_expire_time`。

机型族反查 SHALL 对全部滚服候选的 `device_type` 去重后**一次批量取回**，SHALL NOT 逐候选查询。固资查询 SHALL 在单次请求内按 `(region, 机型族)` 缓存结果，同一组合 MUST 只查一次 CMDB——同一机型族下的多个机型会反查出同一个族。

补全 SHALL 在保留候选数攒够入参 `limit` 时提前终止：后续候选无论去留都排在方案列表 `limit` 名之后，继续查固资与额度不会改变出参，只会白花外部调用。

反查不到机型族、或该族无可继承固资的滚服候选 MUST 被丢弃，MUST NOT 返回一个缺 `inherit_instance_id` 的滚服方案——提单时 `checkRollingServer` 会按 `inherit_instance_id` 反查 CMDB 比较机型族，"先定机型再配固资"的顺序必须由这一步兜住，否则推荐出的方案在提单环节必然失败。

丢弃 MUST 是**静默**的：响应结构不变，SHALL NOT 新增 `filtered_reasons` / `message` 等承载过滤原因的字段，被丢弃的原因只记服务端日志（`logs.Warnf`，含 rid）。只有滚服候选走补全流程，非滚服候选 SHALL 原样保留且 SHALL NOT 额外发起任何 CMDB 查询；候选中一个滚服都没有时 SHALL 直接返回，连机型族反查都不发起。继承固资查询 logic 与机型信息查询调用失败时 SHALL 透出 error，SHALL NOT 静默当作"无候选"。

#### Scenario: 补全后的滚服方案携带固资信息

- **GIVEN** 业务在静态推荐表中有 `require_type=6` 的历史记录，且对应 region + 机型族下存在可继承固资、额度充足
- **WHEN** 调用 `by_static` 传 `require_type=6`
- **THEN** 返回的 items 中存在滚服方案，其 suborder 的 `bk_asset_id`、`inherit_instance_id`、`charge_months` 均非空，`charge_type` 等于该固资的 `instance_charge_type`

#### Scenario: 无可用固资的候选被丢弃

- **GIVEN** 某滚服候选的 region + 机型族下不存在可继承固资
- **WHEN** 调用 `by_static`
- **THEN** 该候选不出现在返回的 items 中，且不返回缺 `inherit_instance_id` 的滚服方案

#### Scenario: 机型反查不到机型族的候选被丢弃

- **GIVEN** 某滚服候选的 `device_type` 在 `device_type` 表中查不到所属机型族
- **WHEN** 调用 `by_static`
- **THEN** 该候选被静默丢弃、日志中可查到丢弃原因，且不为该候选发起固资查询

#### Scenario: 同一机型族只查一次固资

- **GIVEN** 同一 region 下有 3 个滚服候选，机型不同但同属一个机型族
- **WHEN** 调用 `by_static`
- **THEN** 该 rid 下针对该机型族的 CMDB 固资查询只发生 1 次，3 个候选补全到同一台固资

#### Scenario: 攒够 limit 后不再查固资

- **GIVEN** `limit=2`，候选列表中有 5 个都能补全成功的滚服候选
- **WHEN** 调用 `by_static`
- **THEN** 返回 2 个方案，且第 3 个及之后的候选不触发固资查询与额度预检

#### Scenario: 计费模式取自固资而非写死 PREPAID

- **GIVEN** 继承固资的 `instance_charge_type` 为 `POSTPAID_BY_HOUR`
- **WHEN** 调用 `by_static`
- **THEN** 该滚服方案的 `charge_type` 为 `POSTPAID_BY_HOUR`，不写死为 `PREPAID`

#### Scenario: 无历史滚服记录时不触发固资查询

- **GIVEN** 业务在静态推荐表中没有 `require_type=6` 的历史记录
- **WHEN** 调用 `by_static` 传 `require_type=6`
- **THEN** 返回空 items（HTTP 200，不报错），且不触发任何 CMDB 固资查询

#### Scenario: 非滚服候选不触发固资查询

- **GIVEN** 请求的 `require_type` 为常规项目(1)
- **WHEN** 调用 `by_static`
- **THEN** 不发起任何继承固资相关的 CMDB 查询，耗时与改动前一致

### Requirement: 滚服候选的额度预检

`by_static` SHALL 对补全固资后的滚服候选做额度预检，超额候选不进推荐结果。预检 MUST 复用 `scheduler` 已有的四步实现（`IsNeedQuotaManage()` → `IsResPoolBiz()` 定 `appliedType`，资源池业务用 `ResourcePoolAppliedType`、否则 `NormalAppliedType` → `GetCpuCoreSum()` → `CanApplyHost()`），SHALL NOT 在推荐链路里重写四步逻辑。构造的核数口径 MUST 与落库 `applied_core = CPUAmount × Replicas` 一致。

预检 SHALL 按**单个候选独立**校验（每个候选各自按 `applyNum` 台算核数），SHALL NOT 做跨候选累加——推荐返回多个方案，用户最终只会选其中一个。

额度不足与额度服务故障 MUST 被区分处理：额度不足的候选**静默丢弃**，`CanApplyHost()` 返回的 `reason` 仅记入服务端日志、不出现在接口响应中；额度服务调用失败 MUST 透出 error，MUST NOT 静默放行该候选。

区分方式 SHALL 是**类型化错误**而非新增出口：额度校验入口 `CheckApplyQuota` 在额度不足时 SHALL 返回一个专用错误类型（承载 `reason`），额度服务调用失败时返回普通 error；推荐链路用 `errors.As` 识别前者后静默丢弃、把 `reason` 记 Warn 日志，识别不到则向上透出。这样四步逻辑与对外入口都只有一份，提单链路与审批建单前置校验的调用点**一行都不用改**（它们只判 `err != nil`）。额度不足的两条分支（滚服与小额绿通）SHALL 统一返回该类型，其在校验函数内部的日志级别 SHALL 从 Error 降为 Warn——对推荐链路它是正常过滤条件，是否算错误由调用方决定。

#### Scenario: 额度不足的候选静默丢弃

- **GIVEN** 业务滚服额度剩余核数小于该候选 `applyNum × 机型CPU核数`
- **WHEN** 调用 `by_static`
- **THEN** 该候选不出现在返回的 items 中，响应体中不含 `filtered_reasons` / `message` 等承载过滤原因的字段，响应结构与额度充足时逐字段一致
- **AND** woa-server 日志中能查到该候选被丢弃的 `reason`

#### Scenario: 额度服务故障透出错误

- **GIVEN** 滚服额度服务（`CanApplyHost`）调用返回 error
- **WHEN** 调用 `by_static` 传 `require_type=6`
- **THEN** 整个请求返回 error，不静默放行也不静默丢弃该候选

#### Scenario: 提单与审批链路的额度校验行为不变

- **GIVEN** 额度校验入口改为在额度不足时返回类型化错误
- **WHEN** 提单预检与审批建单前置校验调用该入口且业务额度不足
- **THEN** 两处仍拿到非 nil error 并按原逻辑中断，调用代码与对外表现与改动前一致

#### Scenario: 单候选独立校验不累加

- **GIVEN** 两个滚服候选各需 X 核、业务剩余额度为 1.5X 核
- **WHEN** 调用 `by_static`
- **THEN** 两个候选都保留（按单候选独立校验），而非只留一个

#### Scenario: 全部滚服候选被丢弃

- **GIVEN** 所有滚服候选均因无固资或额度不足被丢弃
- **WHEN** 调用 `by_static`
- **THEN** 返回空 items（HTTP 200），出参结构不变、不携带被过滤原因

### Requirement: by_plan 对滚服返回空方案

`by_plan` 接口 SHALL NOT 为滚服项目产出推荐方案——滚服不校验预测（`NeedVerifyResPlan()` 为 false），提单时扣减的是资源池业务而非当前业务的预测，用预测余量做门槛与排序会筛掉本可提单成功的方案。`require_type=6` SHALL NOT 被判定为参数错误，接口 SHALL 返回 HTTP 200 与空的 `items` 列表，出参结构不变；服务端 SHALL 记录一条跳过日志（含 rid）。滚服的推荐能力由 `by_static` 承载。

#### Scenario: by_plan 对滚服返回空方案而非报错

- **GIVEN** `require_type=6`
- **WHEN** 调用 `get_biz_apply_recommend_by_plan`
- **THEN** 返回 HTTP 200 与空 `items`，不返回参数校验错误，且不发起预测池与库存查询

#### Scenario: by_plan 常规项目无回归

- **GIVEN** `require_type=1`（常规项目）
- **WHEN** 调用 `get_biz_apply_recommend_by_plan`
- **THEN** 正常返回基于预测余量的推荐方案，不受本次改动影响
