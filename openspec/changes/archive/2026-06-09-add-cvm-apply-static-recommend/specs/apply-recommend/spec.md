## ADDED Requirements

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
