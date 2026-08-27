# prepaid-settle-task

## Purpose

account-server 定账 cron：纯时间闸门、双表独立判定、手动触发与配置双写。

## Requirements

### Requirement: account-server cron 基础设施

系统 SHALL 在 account-server 新增 `initCronTask`（`cron.Init` + `cron.Register`），并新建 `cmd/account-server/task/bill_settle.go` 实现 `croncore.Task` 的四个方法。account-server 此前无任何 cron 任务（现有后台任务为 BillManager / SyncController / ExchangeRateController 三个 goroutine），本要求为从零新增基础设施。

#### Scenario: 启动时 cron 任务已注册

- **GIVEN** account-server 启动
- **WHEN** 检查启动日志与已注册任务
- **THEN** `initCronTask` 已执行
- **AND** 定账任务已通过 `cron.Register` 注册

### Requirement: 多副本 master 去重

系统的定账任务 SHALL 在 `Do(kt)` 首行执行 `sd.IsMaster()` 检查，非 master 实例 MUST 直接返回。`pkg/cron/core/scheduler.go` 的调度器不做 leader 选举，多副本去重 MUST 由任务自身负责。

#### Scenario: 多副本下仅 master 执行扫描

- **GIVEN** account-server 以多副本部署
- **WHEN** 定账任务被调度
- **THEN** 仅 master 实例执行扫描
- **AND** 非 master 实例在 `Do(kt)` 首行即返回，日志可证

### Requirement: 纯时间闸门的锁定时点

系统的定账 SHALL 由纯时间闸门决定，锁定时点为**基准月份次月 9 日 00:00:00（Asia/Shanghai）**，即 8 日整日仍可被调用方覆盖重推，9 日零点起转 `settled`。定账 MUST NOT 受调用方推送次数或任何「终稿」声明的影响，系统 MUST NOT 接受由外部系统直接触发锁定的入参。锁定日 SHALL 可配置，默认 8（语义为「8 日已过」）。

#### Scenario: 8 日整日仍在可覆盖窗口内

- **GIVEN** 订单月 2026-07 的预付费单
- **WHEN** 系统时间为 2026-08-08 23:59:59
- **THEN** 主表仍为 `settle_state=unsettled`
- **AND** 当系统时间为 2026-08-09 00:00:01 且定账任务已执行时，主表为 `settled`

#### Scenario: 外部终稿标志位不影响定账

- **GIVEN** 调用方在请求体中携带任何形如「终稿」或 `is_final` 的标志位
- **WHEN** 推送
- **THEN** 该标志不影响 `settle_state`
- **AND** 主表仍按时间闸门判定

### Requirement: 双表独立判定

系统 SHALL 对预付费主表与调账表分别独立判定定账：主表按订单月份 `order_year` / `order_month`；调账条目按各自账期 `bill_year` / `bill_month`。两者 MUST NOT 联动。`settle_state` 一旦置为 `settled` MUST 单向不可逆，扫描条件 MUST 仅捞 `unsettled` 记录，MUST NOT 重复更新已定账记录。

调账表的扫描条件 SHALL 额外限定 `state=confirmed`：`settled` 会被守卫拦住确认与编辑且单向不可逆，若把人工录入且仍待确认的调账定账，运营将永久无法确认该条。预付费派生调账落库即 `confirmed`，本条件不影响预付费链路。

> **待产品确认的口径假设**：「账已定不可再改」的前提是这笔账已经确认过。因此定账跳过 `state=unconfirmed` 的人工调账。若产品要求「无论是否确认，账期一过一律定账」，则需同时给出待确认调账被锁死后的恢复手段。
>
> **衍生后果（同一决策的一部分，也待产品确认）**：本条件与回溯窗口（默认 3 个月）叠加会留下一个缺口 —— 一笔人工调账若在其账期**滑出回溯窗口之后**才被确认，就再也不会被任何一轮扫描捞到，`settle_state` 长期停在 `unsettled`，因而长期可编辑可删除，与「越过锁定时点即冻结」的意图相悖。系统 MUST NOT 为此改为全表扫描（代价大于收益，见 RK-5）；该缺口 MUST 在运维文档中说明，并给出以临时调大 `lookbackMonth` 使其重回扫描范围的处置步骤。

#### Scenario: 待确认的人工调账不被定账

- **GIVEN** 账期 2026-07 有一条 `source=manual`、`state=unconfirmed`、`settle_state=unsettled` 的调账
- **WHEN** 该账期锁定时点已过且定账任务执行一轮
- **THEN** 该条仍为 `settle_state=unsettled`
- **AND** 运营仍可正常确认、编辑与删除该条
- **AND** 同账期 `state=confirmed` 的调账已被置为 `settled`

#### Scenario: 主表定账不带动分摊调账定账

- **GIVEN** 订单月 2026-07、分摊至 2026-08 ~ 2026-10 的预付费单
- **WHEN** 系统时间（Asia/Shanghai）到达 2026-08-09 00:00:00 后定账任务执行一轮
- **THEN** 主表 `settle_state=settled`
- **AND** 账期 2026-08 / 09 / 10 的三条调增条目仍全部为 `unsettled`
- **AND** 账期 2026-08 的调增条目要到 2026-09-09 00:00:00 后才转 `settled`

#### Scenario: 已定账记录单向不可逆且不被重复更新

- **GIVEN** 某条调账 `settle_state=settled`
- **WHEN** 定账任务再次扫描
- **THEN** 该字段保持 `settled`
- **AND** 扫描条件仅捞 `unsettled`，不会重复更新已定账记录

### Requirement: 迟到即终稿

系统对锁定时点之后才首次写入的记录 SHALL 先建为 `unsettled`，再由下一轮扫描置为 `settled`。这是时间闸门的固有行为，不视为缺陷。

#### Scenario: 锁定时点后首次写入先未定账再定账

- **GIVEN** 一条订单月 2026-07 的主单在 2026-09-01 才首次写入（锁定时点已过）
- **WHEN** 写入完成
- **THEN** 该记录先为 `unsettled`
- **AND** 下一轮定账任务执行后被置为 `settled`

### Requirement: 扫描窗口与批量置位

系统的定账任务 SHALL 分页扫描两张表中「锁定时点已过」且仍为 `unsettled` 的记录并批量置 `settled`。调账表扫描 MUST 限定账期回溯窗口（默认 3 个月）。本期 MUST NOT 依赖不存在的 `idx_bill_year_month`；预付费主表扫描 MUST 能走 `idx_order_year_month`。回溯窗口是有意的性能取舍：超出窗口且仍为 `unsettled` 的条目不会被扫描到，该行为 MUST 在运维说明中写明。

超窗口漏扫有两个已知触发场景，运维文档 MUST 覆盖两者：任务连续停机超过 `lookbackMonth` 个月；以及人工调账在账期滑出窗口之后才被确认（见「双表独立判定」的衍生后果）。两者的处置手段相同：`lookbackMonth` 为配置项，临时调大后重启生效，再手动触发一轮即可补齐，跑完调回默认值。

#### Scenario: 超出回溯窗口的条目不被本轮扫描覆盖

- **GIVEN** 调账表中存在超出回溯窗口（如 6 个月前）且仍为 `unsettled` 的条目
- **WHEN** 定账任务执行一轮
- **THEN** 该条目不被本轮扫描覆盖

#### Scenario: 调大回溯窗口可补齐迟确认的历史调账

- **GIVEN** 一条账期为 6 个月前的人工调账，在账期滑出回溯窗口之后才被确认，当前为 `state=confirmed` 且 `settle_state=unsettled`
- **WHEN** 运维把 `lookbackMonth` 临时调大到覆盖该账期并重启 account-server，再手动触发一轮
- **THEN** 该条被扫描到并置为 `settled`
- **AND** 配置调回默认值后不影响已置位的结果

#### Scenario: 单次扫描耗时与索引命中

- **GIVEN** 调账表回溯 3 个月账期
- **WHEN** 定账任务执行一轮
- **THEN** 单次耗时 < 5min
- **AND** 不产生慢查询告警；预付费主表扫描可走 `idx_order_year_month`

### Requirement: 手动触发接口

系统 SHALL 由定账任务的 `GetURL()` 返回手动触发路径，并在 account-server service 中注册为 POST 接口（cron 包要求每个任务都有外部触发 API）。该接口 SHALL 仅允许管理员调用。

手动触发 MUST NOT 复用带 master 门禁的 `Do(kt)`：多副本部署时请求会被负载均衡打到任意副本，复用 `Do` 会让非 master 副本返回 200 且什么都不做。定账任务 SHALL 额外暴露一个不含 master 判定的执行入口供手动触发调用，master 判定 MUST 只保留在 cron 调度路径上。

该接口为**同步长耗时操作**：一次请求内完成全部回溯账期的分页扫描与批量置位，调用方 MUST 放宽超时。该特性 MUST 在接口文档中说明。

#### Scenario: 管理员手动触发一轮扫描

- **GIVEN** 定账任务已注册
- **WHEN** 管理员 POST 调用 `GetURL()` 返回的路径
- **THEN** 立即触发一轮扫描并返回成功
- **AND** 非管理员调用返回权限错误

#### Scenario: 非 master 副本手动触发仍真正执行

- **GIVEN** account-server 以多副本部署，请求被负载均衡打到非 master 副本
- **WHEN** 管理员 POST 调用手动触发路径
- **THEN** 该副本真正执行一轮完整扫描并置位
- **AND** MUST NOT 出现「返回成功但零变更」的静默空转

### Requirement: 定账任务配置项双写

系统 SHALL 将锁定日（默认 8）、扫描间隔（默认 1 小时）、回溯窗口（默认 3 个月）三项配置同时落地到 `cmd/account-server/etc/account_server.yaml` 与 `docs/support-file/helm/values.yaml`，两处键名 MUST 一致。锁定精度等于扫描周期，最坏 1 小时误差，属被接受的固有行为。

#### Scenario: 两处配置文件键名一致

- **GIVEN** 锁定日配置为 8、扫描间隔配置为 1 小时、回溯窗口配置为 3 个月
- **WHEN** 检查 `cmd/account-server/etc/account_server.yaml` 与 `docs/support-file/helm/values.yaml`
- **THEN** 两处配置项均已落地
- **AND** 两处键名一致

#### Scenario: 锁定精度等于扫描周期

- **GIVEN** 定账任务扫描间隔配置为 1 小时
- **WHEN** 某记录的锁定时点刚过
- **THEN** 该记录在不超过 1 小时内被置为 `settled`
