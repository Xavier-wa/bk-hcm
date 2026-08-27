## Context

account-server 账单控制器按 UTC 自然月同时调度「本月 + 上月」：

- 日原始拉取：`MainAccountController.syncDailyRawBill` → `ensureDailyRawPullTask` → 各厂商 `EnsurePullTask` → `daily.DailyPuller.EnsurePullTask` → `getBillDays`（按 `now - BillDelay` 逐日展开）→ `ensureDailyPulling`（先续跑已有记录，再为缺的天 `createDailyPullTaskStub`）
- 日分账 / 日汇总：只遍历已有 pull task，不会自己「发起」新的账单日
- 月界：`pkg/tools/times` 的 `GetCurrentMonthUTC` / `GetLastMonthUTC`（UTC 月切 ≈ 北京时间 08:00）
- 循环间隔：`DailySummarySyncDuration` 默认 30s，可满足「1 号 16:00 ±5 分钟」触发
- AWS / Azure / GCP / Huawei 均 `BillDelay=1`，共用 `DailyPuller`；Zenlayer `EnsurePullTask` 为空，任务由导入创建
- 手动重拉：`ReaccountRootAccountSummary` 仅在根汇总为 accounted / confirmed / synced 时升 `CurrentVersion` 回到 `accounting`；账期内当期几乎一直是 accounting，现网本身不能手拉

现网问题：UTC 本月从月初就开始逐日**发起**新任务。本期只推迟「自动发起」，不打断已发起的流水线。

需求：`docs/reqs/35130916/`（TAPD 1069995598135130916）。探索结论已并入本文（2026-08-20）。

## Goals / Non-Goals

**Goals:**

- UTC 本月：不自动**新建**日拉取 stub；已有日任务继续跑完（补 FlowID、失败重试、分账、日汇总）
- UTC 上月：北京时间当月 1 号 16:00 之前不新建 stub；到达后一次展开上月全部日期，该次成功结果即初版
- 不新增「初版」标记，运营用 `CurrentVersion` 区分版本即可
- 汇总头继续为本月 / 上月创建；查询 / 前端 / SDK 不改
- 闸口可单测（16:00 前后、月界、跨年）

**Non-Goals:**

- 不改终版策略、定向重拉某几日、按云厂商分时、22:00
- 不改账期定义（仍 UTC 月），不重算历史归属
- 不改 Zenlayer 导入创建任务
- 不改 `ReaccountRootAccountSummary` 的状态门与升版本语义
- 不做手拉穿窗（`force` 参数、`CurrentVersion > 1` 旁路、核算中强制发起）
- 不新增抽检工作台、IM/邮件通知通道
- 不改账单查询 API 与前端空态文案
- 不上线时自动删除本月已有过程日任务

## Decisions

### 决策 1：闸口只拦「新建日 stub」，不拦进行中

**选择**：在 `pkg/tools/times` 新增 `ShouldAutoPullBillPeriod(now time.Time, billYear, billMonth int) bool`。应用到 `daily.DailyPuller.EnsurePullTask`（或 `ensureDailyPulling` 的补缺天循环）：未开窗时把待创建的 `dayList` 置空并打 Info（vendor、账期、账号、rid），**仍执行**已有任务的续跑（空 FlowID 建 flow、flow 失败/取消重建）。

日分账、日汇总、主/根月汇总轮询、monthtask **不套时间窗**。无新 stub 时它们自然没有新的 pulled 天可推进；有在途任务时必须能跑完。

**备选**：在 `ensureDailyRawPullTask` 未开窗则整段 `return`，连带 skip split / summary / monthtask。

**理由**：`EnsurePullTask` 本身分两段——续跑已有记录 vs 按 `dayList` 补缺天。整段短路会把 8 月中上线时已经在跑的天卡死，与「继续跑完」相反。分账/汇总并不发起新的账单日，加窗只会误伤在途。

伪逻辑（是否允许**新建** stub）：

1. 用 `now.UTC()` 得到 UTC 本月 `(curY, curM)` 与上月
2. 若 `(billYear, billMonth)` == UTC 本月 → `false`
3. 若 == UTC 上月 → 当且仅当 `now` 在 `Asia/Shanghai` 下 `>= time.Date(curY, curM, 1, 16, 0, 0, 0, loc)`
4. 更早账期（日拉控制器当前不会传入）→ `true`

时区：`time.LoadLocation("Asia/Shanghai")` + `sync.Once` 缓存；加载失败则 `time.FixedZone("CST", 8*3600)`。

### 决策 2：账期继续用 UTC 月，16:00 用北京时间

**选择**：账期切月仍走 `GetCurrentMonthUTC` / `GetLastMonthUTC`；仅把「上月允许新建 stub」从「UTC 月切立刻」（约北京 08:00）推迟到北京 1 号 16:00。

**备选**：把账期改成北京时间自然月。

**理由**：改账期会移动历史账单归属。UTC 月切后到北京 16:00 之间不新建上月 stub，符合「次月 1 号 16:00 才首次自动发起」。

关键时序（举例）：

| 北京时间 | UTC | 本月/上月 | 新建上月 stub | 已有任务 |
|---|---|---|---|---|
| 08-01 08:00 | 08-01 00:00 | Aug / Jul | 否 | 继续 |
| 08-01 16:00 | 08-01 08:00 | Aug / Jul | 是，展开 7 月全部日期 | 继续 |
| 08-15 10:00 | 08-15 02:00 | Aug / Jul | 7 月窗已开；8 月否 | 继续 |
| 09-01 08:00 | 09-01 00:00 | Sep / Aug | 8 月尚未到 16:00，否 | 继续 |

### 决策 3：开窗后一次拉满上月

**选择**：时间窗打开后仍走现网 `getBillDays`。此时上月全部日期都早于 `now - BillDelay`（Delay=1），一次返回整月天数，等价于该账期首次全量发起。

**备选**：单独写「忽略 BillDelay、强制整月」的展开函数。

**理由**：不必分叉。不改各厂商 `BillDelay`。上线当月本月缺的后半段日期，等到该月变成「上月」且过了 1 号 16:00 再一次补齐。

### 决策 4：汇总头仍创建；无初版标记

**选择**：`ensureBillSummary` 继续为本月、上月建 summary。不新增 `is_first_version` 一类字段；运营用 `CurrentVersion` 人工确认版本。

**备选**：本月不建 summary；或加初版标记。

**理由**：列表需要账期占位（F-004 是无过程明细，不是账期从列表消失）。初版标记对抽检帮助有限，超出本期。

### 决策 5：手拉不穿窗，也不改 Reaccount

**选择**：`reaccount.go` 不改。不增加 `force`、不把 `CurrentVersion > 1` 当旁路。

稳态下：账期内当期几乎一直是 `accounting`，现网 Reaccount 直接拒绝。能手拉时根汇总已是 accounted / confirmed / synced，说明该账期已经走过次月 1 号 16:00 的自动初版，日拉时间窗已开，升版本后控制器按现网 `EnsurePullTask` 为新 version 建全日任务即可。

**备选**：核算中也允许强制发起；或 version>1 绕过闸口。

**理由**：抽检不通过 / 自动拉取失败后的补拉，发生在已核算或窗已开之后。为「任何时候手拉」做穿窗，会和现网状态门重复，也容易在本月 v2 上重新打开逐日补天。

切月当天 08:00–16:00、且 UTC 上月**已被旧逻辑核算完**时点重拉，新 version 会碰到窗仍关、又无该 version 的旧 stub。窗口极窄，上线避开 1 号上午即可；稳态不出现。本期不为此做旁路。

### 决策 6：失败通知沿用 flow 失败 + 日志

**选择**：拉取失败保持任务 `failed`、`Warnf`/`Errorf`（vendor、账期、账号、rid）；初版保持未就绪。不新增企业微信/邮件。

**理由**：账单链路没有独立 IM 通知；运营从任务流与日志看失败后再手拉。

### 决策 7：Zenlayer 不改

**选择**：不把闸口写进 Zenlayer 导入模块。

**理由**：它不走自动日拉；改导入节奏超出「推迟自动初版」。

## Risks / Trade-offs

- **[Risk] 机器 TZ 不是北京** → 闸口内部用 `Asia/Shanghai`，不依赖 `time.Now()` 的 Location
- **[Risk] UTC 月切到 16:00 之间上月不新建 stub** → 预期；已有任务继续；新「上月」等到当天 16:00 再发起
- **[Risk] 16:00 后整月并行建日任务** → 沿用现网随机 sleep；单账号最多 31 天
- **[Risk] 上线当月已有过程日任务** → 继续跑完，不删除；不再为后面的日期自动建 stub；缺天等到该月成为上月且过 16:00 再补
- **[Risk] 本月 summary 占位、金额 0** → 接受；不改前端文案
- **[Risk] 无日任务时月汇总 flow 仍可能按现网空转** → 接受，不为空转单独加窗（加窗会误伤在途）
- **[Risk] 1 号 08:00–16:00 对已 accounted 的上月点重拉** → 切月当天少发；不做穿窗
- **[Risk] 单云 16:00 拉取失败** → 其他云继续；失败云未就绪 + 日志，窗已开后可手拉

## Migration Plan

1. 发布 account-server（times 闸口 + DailyPuller 补缺天短路），无需 SQL、无需前端
2. 观察下一个北京时间 1 号：08:00–16:00 上月无**新**日任务；16:00±5min 出现上月全日任务；上线前已有任务的账号仍能续跑
3. 回滚：回退 account-server 即恢复「本月+上月立即补天」；已生成的日任务不会自动消失
4. 上线窗口建议避开 1 号 08:00–17:00，以免切月真空 + 首次开窗与发布重叠

## Open Questions

无。Q-001～Q-005 已确认。实现层：账期 UTC、16:00 北京时间、只拦新建 stub、不改 Reaccount、不做手拉穿窗、Zenlayer 不改、无初版标记。
