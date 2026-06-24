## ADDED Requirements

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
