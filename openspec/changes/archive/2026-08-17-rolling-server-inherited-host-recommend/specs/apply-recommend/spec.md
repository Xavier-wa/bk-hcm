## ADDED Requirements

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

## MODIFIED Requirements

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
