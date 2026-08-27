## ADDED Requirements

### Requirement: 预付费账单跨账期列表查询

系统 SHALL 提供 `POST /api/v1/account/bills/prepaid_items/list` 接口，支持跨账期查询，不要求指定固定账单月份。请求 SHALL 采用与既有账单查询接口一致的 `filter` 表达式（`core.ListWithoutFieldReq`），可筛选字段 MUST 为预付费主表列，字段名合法性与单页上限（`core.DefaultMaxPageLimit`，500）MUST 由 data-service DAO 统一校验。接口 MUST NOT 暴露 `fields` 参数，避免调用方裁剪掉派生所依赖的列。核算状态与累计核算金额为查询期实时派生、主表无物理列，MUST NOT 作为可筛选字段。按使用时间筛选的区间重叠语义（`usage_end_at >= 起点` 且 `usage_start_at <= 终点`）SHALL 由调用方在 filter 中表达，接口文档 MUST 给出该写法。账号类筛选 MUST 只接收 ID（名称由前端转换）。响应 SHALL 返回主表全字段加派生的 `accounting_state`（`pending` / `accounting` / `accounted`）、`accounted_cost` 与 `accounted_rmb_cost`。详情页所需字段 SHALL 由本接口提供，MUST NOT 另开详情接口。

#### Scenario: 按订单月份筛选

- **GIVEN** filter 传入 `order_year eq 2026` 与 `order_month eq 7`
- **WHEN** 查询
- **THEN** 只返回 `order_year=2026` 且 `order_month=7` 的记录

#### Scenario: 按使用时间区间重叠筛选

- **GIVEN** filter 传入 `usage_end_at gte 区间起点` 与 `usage_start_at lte 区间终点`
- **WHEN** 查询
- **THEN** 返回全部使用区间与查询区间有交集的记录
- **AND** 包含开始于查询区间之前但仍在区间内使用的跨账期订单

#### Scenario: 派生字段不可参与筛选

- **GIVEN** filter 传入 `accounting_state` 或 `accounted_cost` 作为筛选字段
- **WHEN** 查询
- **THEN** 返回参数错误（字段不在主表列中）

#### Scenario: 多条件按 AND 组合

- **GIVEN** filter 的 `op=and` 且同时传入使用时间区间、运营产品、GPU 型号、云厂商四项规则
- **WHEN** 查询
- **THEN** 各条件按 AND 组合生效
- **AND** 返回结果同时满足全部条件

#### Scenario: 单页超上限被拒

- **GIVEN** 请求单页 501 条
- **WHEN** 调用列表接口
- **THEN** 返回 `errf.InvalidParameter`

#### Scenario: 列表接口复用为详情页数据源

- **GIVEN** 前端详情页需要的字段
- **WHEN** 调用本列表接口按 ID 筛选单条
- **THEN** 返回的主表全字段加派生字段足以支撑详情页展示
- **AND** 无需另开详情接口

#### Scenario: 列表查询性能

- **GIVEN** 预付费主表 10 万条、调账表现网量级
- **WHEN** 用 wrk 100 并发压测列表接口（单页 500）
- **THEN** P99 < 1s

### Requirement: 实例级行过滤三分支

系统的预付费查询 SHALL 复用现网 `ListAuthorizedInstances(MainAccount, Find)` 取 `IDs` 与 `IsAny`，并按三分支处理：`IsAny=true` 时 MUST NOT 叠加任何账号过滤条件；`IsAny=false` 且 `IDs` 非空时 MUST 叠加 `RuleIn("main_account_id", IDs)`；`IsAny=false` 且 `IDs` 为空时 MUST 直接返回空列表（count=0），MUST NOT 返回全量数据，MUST NOT 返回 500。叠加时调用方的 `filter` 表达式 MUST 整体作为一条子规则挂在鉴权规则之下按 AND 组合，MUST NOT 与鉴权规则平铺合并 —— 平铺会在调用方 `op=or` 时把鉴权条件并入 or 从而失效。

> 这是蓝鲸 IAM（权限中心）集成点：授权实例由权限中心下发，查询侧不新增 action，权限维度统一为二级账号。

#### Scenario: 仅返回已授权二级账号的记录

- **GIVEN** 用户 A 被授权二级账号 M1、M2 的查看权限
- **WHEN** 调用预付费列表接口
- **THEN** 只返回 `main_account_id ∈ {M1, M2}` 的记录

#### Scenario: 无任何授权时返回空列表

- **GIVEN** 用户 B 无任何二级账号授权（`IDs` 为空且 `IsAny=false`）
- **WHEN** 调用列表接口
- **THEN** 返回空列表（count=0）而非全量数据
- **AND** 不返回 500

#### Scenario: 调用方 or 表达式无法绕过行级过滤

- **GIVEN** 用户仅被授权二级账号 M1，请求 filter 为 `op=or` 且含 `main_account_id eq M2`
- **WHEN** 调用列表接口
- **THEN** 仍只返回 `main_account_id ∈ {M1}` 的记录
- **AND** 不返回 M2 的数据

#### Scenario: IsAny 为真时不叠加过滤

- **GIVEN** 用户具有 `IsAny=true` 的权限
- **WHEN** 调用列表接口
- **THEN** 不叠加 `main_account_id` 过滤条件
- **AND** 返回全量分页数据

### Requirement: 主表级核算状态与累计核算金额派生

系统 SHALL 按条目数口径派生主表级核算状态：记 `P` 为该单下 `push_status=pushed` 且 `type=increase` 的条目数，`N` 为该单下 `type=increase` 的条目总数。`P = 0` 为 `pending`（待核算），`0 < P < N` 为 `accounting`（核算中），`P = N` 为 `accounted`（已完成）。累计核算金额 SHALL 为 `push_status=pushed` 且 `type=increase` 的分摊金额之和，同时返回 `accounted_rmb_cost`（已推送调增条目 `rmb_cost` 之和）。订单月份的调减条目 MUST NOT 计入分子或分母。系统 MUST NOT 引入时间维度判定，`P = N` 即为已完成。

#### Scenario: 部分调增已推送时为核算中

- **GIVEN** 某预付费单有 3 条调增（各 1000.00）加 1 条调减（3000.00），其中 1 条调增 `push_status=pushed`
- **WHEN** 查询列表接口
- **THEN** 累计核算金额 = 1000.00
- **AND** 核算状态 = `accounting`（P=1, N=3）

#### Scenario: 全部调增已推送时为已完成且不含调减

- **GIVEN** 3 条调增全部 `pushed`、调减条目 `push_status=unpushed`
- **WHEN** 查询
- **THEN** 累计核算金额 = 3000.00，不含调减条目
- **AND** 核算状态 = `accounted`（P=N=3），不因「当前月 ≤ 最晚分摊月」而停留在核算中

#### Scenario: 仅调减已推送时为待核算

- **GIVEN** 3 条调增均未 `pushed` 但调减条目已 `pushed`
- **WHEN** 查询
- **THEN** 累计核算金额 = 0
- **AND** 核算状态 = `pending`

### Requirement: 派生值不落库且不做遮蔽

系统的核算状态与累计核算金额 SHALL 为查询期实时聚合的派生读模型，MUST NOT 落库，主表 MUST NOT 存在「核算状态」或「累计核算金额」的物理列。后端 SHALL 始终返回真实值，`settle_state=unsettled` 期间也如实反映，MUST NOT 做任何按定账状态的遮蔽或置零 —— 展示遮蔽是前端决策。

#### Scenario: 主表无派生字段物理列

- **GIVEN** 派生计算已实现
- **WHEN** 检查主表结构
- **THEN** 不存在「核算状态」或「累计核算金额」的物理列

#### Scenario: 未定账期间返回真实值不遮蔽

- **GIVEN** 某预付费单主表 `settle_state=unsettled`（未定账期间）且其中 1 条调增已 `pushed`
- **WHEN** 查询列表接口与详情数据
- **THEN** 累计核算金额返回真实值 1000.00、核算状态返回 `accounting`
- **AND** 不返回 0 也不返回空

### Requirement: 分摊与调账整合明细查询

系统 SHALL 提供 `POST /api/v1/account/bills/prepaid_items/{id}/split_items/list` 接口，以月度分摊行为主行，通过账期（`bill_year` / `bill_month`）匹配关联调账记录。每行 SHALL 含 `adjustment_id`、账期、`accounted`（行级核算布尔）、`type`、`cost` / `rmb_cost`、`currency`、`res_class` / `res_sub_class`、`push_status` / `settle_state`、备注。`{id}` 不存在时 MUST 返回 `errf.RecordNotFound`。

#### Scenario: 返回 N+1 行含订单月的调减行

- **GIVEN** 某预付费单订单月 = 2026-07、分摊覆盖 2026-08 / 09 / 10（共 3 条调增加 1 条调减）
- **WHEN** 调用整合明细接口
- **THEN** 返回 4 行：账期 2026-08 / 09 / 10 的 3 行 `type=increase`，以及账期 2026-07（订单月）的 1 行 `type=decrease`
- **AND** 每行含调账编号、账期、行级核算状态、调账类型、调账金额、备注

#### Scenario: 账期匹配无错位无重复关联

- **GIVEN** 各分摊行与调账通过 `bill_year` / `bill_month` 关联
- **WHEN** 调用接口
- **THEN** 每个分摊月的行都关联到该账期的调账记录
- **AND** 无错位、无重复关联

#### Scenario: 明细行字段覆盖核算与调账关键列

- **GIVEN** 接口已实现
- **WHEN** 检查返回字段
- **THEN** 含 `adjustment_id` / 账期 / `accounted` / `type` / `cost` / `rmb_cost` / `currency` / `res_class` / `res_sub_class` / `push_status` / `settle_state` / `memo`

#### Scenario: 不存在的预付费 ID 返回记录未找到

- **GIVEN** `{id}` 为不存在的预付费账单 ID
- **WHEN** 调用接口
- **THEN** 返回 `errf.RecordNotFound`

### Requirement: 行级核算状态单条判定

系统的整合明细行级核算状态 SHALL 按单条判定：该条调账 `push_status=pushed` 则 `accounted=true`，否则 `accounted=false`。行级判定 MUST NOT 使用主表级的 P/N 聚合口径。

#### Scenario: 行级状态逐条独立判定

- **GIVEN** 某单下 4 条调账中账期 2026-08 的调增 `push_status=pushed`、其余为 `unpushed`
- **WHEN** 调用整合明细接口
- **THEN** 2026-08 行的 `accounted=true`
- **AND** 其余 3 行 `accounted=false`，为单条判定而非 P/N 聚合

### Requirement: 整合明细越权防护与覆盖重推后一致性

系统的整合明细接口 SHALL 与列表接口采用同一二级账号实例级鉴权。用户对 `{id}` 所属二级账号无权限时 MUST 返回空或权限错误，MUST NOT 泄露该二级账号的数据。覆盖重推后，接口返回的调账编号 MUST 全部为新建条目，MUST NOT 出现已被物理删除的旧调账编号。

#### Scenario: 越权请求他人预付费明细不泄露数据

- **GIVEN** 用户仅有二级账号 M1 的权限
- **WHEN** 通过 `{id}` 直接请求属于 M2 的预付费单分摊明细
- **THEN** 返回空或权限错误
- **AND** 不泄露 M2 的数据

#### Scenario: 覆盖重推后不返回已删除的旧调账编号

- **GIVEN** 某单被调用方覆盖重推（旧调账物理删除、新建 N+1 条）
- **WHEN** 调用整合明细接口
- **THEN** 返回的调账编号全部为新建的条目
- **AND** 不出现已删除的旧调账编号
