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
系统 SHALL 在 woa-server 业务路由下提供 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/by_static_recommend` 接口，`bk_biz_id` 通过路径参数传入。调用前 SHALL 通过业务访问鉴权（`Biz` / `Access`），鉴权失败直接返回。接口在离线静态推荐候选之上叠加库存校验与默认值补全，返回若干推荐方案，每个方案 SHALL 仅包含 1 个子单，并携带 `source` 字段标识推荐来源（`user`-用户维度 / `biz`-业务维度）；子单 SHALL 包含 需求类型、地域、可用区、机型、镜像(image_id)、资源分配方式、申请数量、计费模式、系统盘、数据盘。本接口为独立新接口，不改动既有 `GetBizApplyRecommendTop`。

请求体必填 `bk_username` 与返回方案数 `limit`；可选 A 类过滤字段（需求类型 / 地域 / 机型 / 镜像 image_id）与 B 类覆盖字段（可用区 / 资源分配方式 / 申请数量）。

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

### Requirement: 静态推荐候选查询与 A 类过滤
系统 SHALL 复用「先人后业务补足」的推荐查询逻辑获取候选：先查用户维度 `ziyan_cvm_apply_user_recommend`，不足时从业务维度 `ziyan_cvm_apply_biz_recommend` 去重补足，去重键为 `(require_type, region, device_type, image_id)`。A 类入参（需求类型 / 地域 / 机型 / 镜像 image_id）中非空者 SHALL 作为过滤条件在查询层收窄候选；现有 `GetBizApplyRecommendTop` 不传 A 类过滤时行为 SHALL 保持不变。

#### Scenario: A 类过滤收窄候选
- **WHEN** 请求传入 `region` 与 `device_type` 过滤条件
- **THEN** 仅返回匹配该地域与机型的静态推荐候选参与后续库存校验与组装

#### Scenario: A 类过滤传了但查不到匹配
- **WHEN** 请求传入 A 类过滤条件，但静态推荐表中无任何匹配候选
- **THEN** 直接返回空方案列表（不强构、不降级）

#### Scenario: 既有 Top-N 查询行为不变
- **WHEN** `GetBizApplyRecommendTop` 调用推荐查询且不传 A 类过滤
- **THEN** 查询条件与返回结果与扩展前完全一致

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
系统 SHALL 为每个方案补全下单必需的默认值：image_id 取自静态推荐表；计费模式固定 `PREPAID`；系统盘默认 `CLOUD_PREMIUM / 100G / 1 块`；数据盘默认 `CLOUD_PREMIUM / 500G / 1 块`。申请数量 SHALL 取入参，未传时取 `cc.ApplyRecommend` 配置的默认值（默认 10），且 SHALL NOT 使用推荐表 `count` 作为申请数量；`count` 仅用于候选排序。新增配置项 SHALL 同步至 `etc/woa_server.yaml` 与 helm values。

#### Scenario: 申请数量取默认配置
- **WHEN** 请求未传申请数量
- **THEN** 各方案的申请数量取 `cc.ApplyRecommend` 默认值（默认 10），库存阈值同样按该值校验

#### Scenario: 申请数量取入参
- **WHEN** 请求传入申请数量
- **THEN** 各方案的申请数量与库存校验阈值均取该入参值

#### Scenario: 默认字段补全
- **WHEN** 候选通过库存校验进入方案组装
- **THEN** 方案补全 image_id（静态表）、计费模式 PREPAID、系统盘 CLOUD_PREMIUM/100G/1、数据盘 CLOUD_PREMIUM/500G/1

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
系统 SHALL 在 woa-server 业务路由下提供 `POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/split_suborder` 接口，`bk_biz_id` 通过路径参数传入。调用前 SHALL 通过业务访问鉴权（`Biz` / `Access`），鉴权失败直接返回。接口将一个确定方案按计费模式拆分，组装成 1 个主机申请单据（主单）含多个子单，**纯试算、不落库**。请求体必填字段 SHALL 包含：需求类型、地域、可用区、机型、镜像(image_id)、资源分配方式、申请数量(replicas)、系统盘；可选字段 SHALL 包含：数据盘(data_disk)、`occupied_suborders`（已占用子单数组，用于增量拆分）。响应 SHALL 为扁平结构 `{ suborders: [...] }`，每个子单 SHALL 包含 机型、镜像(image_id)、申请数量、地域、可用区、计费模式、需求类型、系统盘、数据盘、资源分配方式。本接口与既有 `GetBizApplyRecommendTop` / `GetBizApplyRecommendByStatic` / `GetBizApplyRecommendByPlan` 相互独立、互不改动。

#### Scenario: 鉴权失败
- **WHEN** 调用方无该业务的 `Biz`/`Access` 权限
- **THEN** 接口直接返回鉴权错误，不执行任何拆分计算

#### Scenario: 必填参数缺失
- **WHEN** 请求缺少需求类型 / 地域 / 可用区 / 机型 / 镜像 / 资源分配方式 / 申请数量 / 系统盘中任一字段
- **THEN** 返回 `InvalidParameter` 错误

#### Scenario: 纯试算不落库
- **WHEN** 接口成功返回主单与子单
- **THEN** 系统 SHALL NOT 写入 `ziyan_cvm_apply_order` / `ziyan_cvm_apply_suborder` 等任何表

### Requirement: 按计费模式的子单拆分
系统 SHALL 以**计费模式**为唯一拆分轴，`zone` 原样透传单值（`all` 或具体值），SHALL NOT 按 zone 拆分。预测内余量满足的台数 SHALL 组装为包年包月（PREPAID）子单；预测内装不下、溢出到预测外余量的台数 SHALL 组装为按量计费（POSTPAID_BY_HOUR）子单。一个入参组合 SHALL 最多拆 2 个子单；不需校验预测的需求类型 SHALL 只出 1 个 PREPAID 子单。各子单的「申请数量(replicas)」SHALL 为拆分后实际分配台数，合计 SHALL ≤ 入参 replicas；数量 ≤ 0 的子单 SHALL 被丢弃。

#### Scenario: 预测内装不下溢出预测外
- **WHEN** 需求类型为常规(1)，申请数量在预测内余量装不下，预测外余量可补足部分
- **THEN** 返回 1 个主单 + 2 个子单（预测内 PREPAID + 预测外 POSTPAID_BY_HOUR），各子单数量合计 ≤ 入参 replicas

#### Scenario: 预测内可全部满足
- **WHEN** 申请数量在预测内余量内即可全部满足
- **THEN** 仅返回 1 个 PREPAID 子单，无 POSTPAID 子单

#### Scenario: 丢弃零数量子单
- **WHEN** 预测外分配台数计算结果为 0
- **THEN** 不返回该 POSTPAID 子单

### Requirement: 按需求类型差异化的预测与库存校验
系统 SHALL 复用 `RequireType.NeedVerifyResPlan()` / `NotNeedVerifyCapacity()` 判定校验差异。常规(1)/春保(2)/短租(9) SHALL 校验预测内 + 预测外并查库存，最多 2 子单；机房裁撤(3) SHALL 校验预测但忽略预测内外、余量合并为单池**只算一次**（避免 `GetPlanTypeAvlDeviceTypesV2` 预测内/外对裁撤返回同份余量导致重复计数），输出 1 个 PREPAID 子单并查库存；滚服(6)/春保资源池(8) SHALL NOT 校验预测，输出 1 个 PREPAID 子单并查库存；小额绿通(7) SHALL NOT 校验预测且 SHALL 跳过库存校验，输出 1 个 PREPAID 子单。计费模式 SHALL 由系统按预测池来源推导（预测内 PREPAID / 预测外 POSTPAID_BY_HOUR / 不校验预测 = PREPAID），其余字段（地域、可用区、镜像、磁盘、资源分配方式）SHALL 原样透传入参。

#### Scenario: 小额绿通跳过库存
- **WHEN** 需求类型为小额绿通(7)
- **THEN** 跳过库存校验，返回 1 个 PREPAID 子单，数量为入参 replicas

#### Scenario: 机房裁撤单池只算一次
- **WHEN** 需求类型为机房裁撤(3)
- **THEN** 预测内外余量合并为单池仅计一次，返回 1 个 PREPAID 子单，且不重复计数预测余量

#### Scenario: 滚服不校验预测
- **WHEN** 需求类型为滚服(6)
- **THEN** 不校验预测，按库存上限返回 1 个 PREPAID 子单

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
