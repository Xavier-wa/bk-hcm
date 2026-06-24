## ADDED Requirements

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

## MODIFIED Requirements

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
