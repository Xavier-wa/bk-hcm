# third-party-bill-first-pull-window

## ADDED Requirements

### Requirement: 账期内不得自动发起当期日拉取任务

系统 SHALL 在 UTC 本月账期内，不对已接入的自动日拉三方云（AWS / Azure / GCP / Huawei）**新建**当期 `BillDailyPullTask`。该规则对上述厂商一致，不得只对某一厂商放开月中自动发起。闸口 SHALL 只拦截自动发起（补缺天 / `createDailyPullTaskStub`），不得因此跳过已存在日任务的续跑。

#### Scenario: 月中不新建当期日任务
- **WHEN** 当前时间为某一 UTC 自然月的月中（例如 2026-07-15 10:00 UTC），且该账期当前 version 尚无对应日任务（或仅应补新的日期）
- **THEN** 系统 SHALL 不得为 UTC 本月新建 `BillDailyPullTask` stub，也不得因自动调度落新的当期过程原始账单

#### Scenario: 已有当期日任务继续跑完
- **WHEN** UTC 本月已存在日拉取任务（例如上线前旧逻辑已创建）
- **THEN** 系统 SHALL 继续为这些任务补 FlowID、在 flow 失败或取消时重建，并允许后续分账与日汇总推进，不得因时间窗整段停止 `EnsurePullTask`

#### Scenario: 账期最后一天仍不得把未开窗的自动结果标为初版
- **WHEN** 当前时间为 UTC 本月最后一天 23:59，且本月从未自动开窗发起
- **THEN** 系统 SHALL 保持该账期自动初版未就绪，不得把本月月中新发起的拉取标为初版

#### Scenario: 各自动日拉厂商规则一致
- **WHEN** 同一时刻分别检查 AWS、Azure、GCP、Huawei 的当期自动发起
- **THEN** 各厂商 SHALL 均不自动新建 UTC 本月日任务，不得出现「仅 AWS 改期、其他云仍月中发起」

### Requirement: 次月 1 号 16:00 北京时间首次自动发起上月拉取并作为初版

系统 SHALL 仅在北京时间（Asia/Shanghai）次月 1 号 16:00 及之后，对 UTC 上月账期**允许新建**日拉取任务。该次成功拉取结果即为该账期初版。1 号逢周末或法定节假日 SHALL 仍按 1 号 16:00 触发，不顺延。各自动日拉厂商使用同一触发时刻，不得单独改为 22:00。

账期边界继续使用 UTC 自然月（`GetCurrentMonthUTC` / `GetLastMonthUTC`），不得改写历史账单归属。不新增初版标记字段；版本以现网 `CurrentVersion` 为准。

#### Scenario: 1 号 16:00 前不新建上月日任务
- **WHEN** 当前北京时间早于当月 1 号 16:00（例如 2026-08-01 15:59，Asia/Shanghai），目标账期为 UTC 上月，且该 version 尚无日任务
- **THEN** 系统 SHALL 不为该上月新建日任务，上月自动初版保持未就绪

#### Scenario: 1 号 16:00 前上月已有任务仍续跑
- **WHEN** 当前北京时间早于当月 1 号 16:00，UTC 上月已存在日拉取任务
- **THEN** 系统 SHALL 继续续跑这些任务，不得因未到 16:00 而停止已有 flow

#### Scenario: 1 号 16:00 起触发上月全量日拉
- **WHEN** 当前北京时间到达或超过当月 1 号 16:00（允许相对该时刻 ±5 分钟，由约 30 秒一轮的控制器轮询保证），目标账期为 UTC 上月
- **THEN** 系统 SHALL 对该上月展开全部账单日并创建/续跑日拉取任务；任一厂商该次拉取成功后，该厂商该账期初版就绪

#### Scenario: 首次拉取覆盖上月全部日期
- **WHEN** 北京时间 1 号 16:00 后首次允许为 UTC 上月新建日任务
- **THEN** 待创建日期列表 SHALL 包含该月全部自然日（不再按「当天 - BillDelay」逐日追加），该次结果作为初版而不是月底收尾的月中数据

#### Scenario: 节假日不顺延
- **WHEN** 次月 1 号为周末或法定节假日
- **THEN** 系统 SHALL 仍在该日北京时间 16:00 允许新建上月日任务，不得改到下一个工作日

### Requirement: 日分账与日汇总不套用发起时间窗

系统 SHALL 不对日分账、日汇总、主账号月汇总轮询、根账号月汇总轮询、monthtask 套用与「新建日 stub」相同的时间窗。这些步骤 SHALL 只消费已存在的日任务；无日任务时不得为了「对齐时间窗」而整段禁用它们（以免卡住上线前已在途的账期）。

#### Scenario: 有在途日任务时分账汇总可推进
- **WHEN** 某账期时间窗未开，但已有日任务处于 pulling / pulled / split 等状态
- **THEN** 系统 SHALL 按现网逻辑继续创建或续跑对应的 split / daily summary，不得因未开窗而跳过该账期

#### Scenario: 开窗后分账与日汇总可继续
- **WHEN** UTC 上月已过北京时间 1 号 16:00，且对应日拉取任务已处于 pulled / split 等后续状态
- **THEN** 系统 SHALL 按现网逻辑继续日分账与日汇总，不得额外延迟到第二天

### Requirement: 出账前查询无新的当期过程明细

在北京时间次月 1 号 16:00 触发上月首次**自动发起**之前，系统 SHALL 不因本期改造而恢复月中自动补天。查询入口可打开，空态与现网「无账单」对齐即可。已完成初版出账的历史账期仍按现网方式可查。

允许继续为 UTC 本月 / 上月创建账单汇总头记录（summary）。上线当月若已有过程日任务，那些明细可以继续存在并跑完，SHALL 不在本期做自动清理。

#### Scenario: 稳态下出账前查询为空
- **WHEN** 业务用户在某一 UTC 自然月月中打开该月三方云账单查询，且该账期从未自动或手动发起过日任务
- **THEN** 系统 SHALL 不返回该账期初版明细，也不返回新的月中过程明细（空列表或现网等价空态）

#### Scenario: 历史已出账期仍可查
- **WHEN** 用户在后续月份查询一个已经完成初版出账的历史账期
- **THEN** 系统 SHALL 仍按现网方式返回该账期已出账数据

### Requirement: 手动重拉沿用现网状态门且不另做穿窗

系统 SHALL 保留现网一级账号账单重拉（`ReaccountRootAccountSummary`：仅当根汇总为 accounted / confirmed / synced 时提升 `CurrentVersion` 并将状态改回 `accounting`）。本期 SHALL 不增加强制发起、也不把版本号当作时间窗旁路。初版出账后不得再自动强制全量重拉；是否重拉由运营抽检决定。

能发起现网手拉时，该账期已走过次月 1 号 16:00 的自动初版，日拉发起窗已开；升版本后 SHALL 按现网通道为新 version 拉取。本期不新增抽检页面或抽检规则引擎。

#### Scenario: 抽检通过后不再自动全量重拉
- **WHEN** 某账期初版已就绪且抽检通过，到达该次月后续任意时刻
- **THEN** 系统 SHALL 不得再自动强制对该账期全量重拉一遍

#### Scenario: 抽检不通过可手动重拉
- **WHEN** 初版已就绪（根汇总已 accounted / confirmed / synced）但抽检不通过，账单运营对一级账号发起现网手动重拉并成功
- **THEN** 系统 SHALL 提升版本并回到核算中，按现网拉取通道更新该账期账单为本次手动拉取结果

#### Scenario: 核算中不能手拉（现网状态门）
- **WHEN** 根汇总仍为 accounting（账期内当期的典型状态）
- **THEN** 系统 SHALL 拒绝 Reaccount，与改造前一致，不得为此新增穿窗或强制发起

#### Scenario: 自动拉取失败后可在窗已开后手动补拉
- **WHEN** 北京时间 1 号 16:00 后某厂商上月自动拉取失败或未完成
- **THEN** 该厂商该账期初版保持未就绪，系统 SHALL 记录失败日志（含 vendor、账期、账号、rid），且不得回退使用月中新发起数据当初版；若之后满足现网 Reaccount 状态门，运营可手动补拉

#### Scenario: 手动重拉失败保持原数据
- **WHEN** 运营发起的手动重拉失败
- **THEN** 系统 SHALL 保持重拉前已有数据（若有），返回或记录失败原因，允许再次发起

### Requirement: Zenlayer 不走自动日拉路径本期不改

Zenlayer 的 `EnsurePullTask` 为空实现，日任务由账单导入模块创建。系统 SHALL 不把 Zenlayer 纳入本次自动日拉时间窗改造，也不得因此为其补上月中自动拉取。

#### Scenario: Zenlayer 调度保持导入创建任务
- **WHEN** 主账号控制器对 Zenlayer 调用 `EnsurePullTask`
- **THEN** 行为 SHALL 仍为 no-op，日任务继续由导入模块创建，不因本需求改为次月 1 号 16:00 自动拉
