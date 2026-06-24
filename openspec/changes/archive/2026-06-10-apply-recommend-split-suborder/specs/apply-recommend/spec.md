## ADDED Requirements

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
