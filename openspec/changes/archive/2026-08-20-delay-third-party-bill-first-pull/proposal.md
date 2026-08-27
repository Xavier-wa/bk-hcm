## Why

三方云账单现网在账期内按日持续自动拉取，月底再收尾成初版；这版数据与终版差距大，运营每个次月月初几乎都要手动重拉。月中拉取既不能对外承诺准确，又会被月初重拉覆盖。本期把账期首次自动拉取改到次月 1 号 16:00（北京时间），账期内不再**自动发起**当期日任务，把资源用在云厂商账单相对稳定的时点。

## What Changes

- 闸口只拦截**自动发起**新的日原始拉取任务（新建 `BillDailyPullTask` stub）；已存在的日任务继续补 flow、失败重试，分账 / 日汇总 / 月任务不额外加窗
- UTC 本月：不自动新建日任务；UTC 上月：北京时间当月 1 号 16:00 之前不新建，到达后一次展开上月全部日期，该次成功结果即为该账期初版
- 账期定义保持 UTC 自然月，不改历史账单归属
- 保留现网手动重拉（`ReaccountRootAccountSummary`）。现网仅 accounted / confirmed / synced 可重拉；能点手拉时时间窗已开，**不需要**手拉穿窗或 `CurrentVersion` 旁路
- 不改查询 API、前端、云厂商 SDK；稳态下开窗前无新日任务则查询为空态。上线当月已有在途任务继续跑完，不清理
- Zenlayer 不走自动日拉（导入创建任务），本期不改其调度

## Capabilities

### New Capabilities

- `third-party-bill-first-pull-window`: 三方云初版自动拉取时间窗——只拦自动新建日任务，不拦进行中任务；本月不新发起、上月在北京时间 1 号 16:00 后才新发起；手拉沿用现网状态门，无需穿窗

### Modified Capabilities

（无需修改现有 spec）

## Impact

- **日拉取器**：`cmd/account-server/logics/bill/puller/daily/daily.go` 的 `EnsurePullTask` / `ensureDailyPulling`（补缺天循环）
- **时间工具**：新增「某 UTC 账期此刻是否允许自动**发起**日任务」判断（北京时间 16:00 阈值）
- **厂商拉取器**：AWS / Azure / GCP / Huawei 仍复用 `DailyPuller`，不改 SDK；Zenlayer 不在自动日拉路径
- **手动重拉**：`billsummaryroot/reaccount.go` 行为保持不变
- **不改**：日分账 / 日汇总 / 月任务控制器（只消费已有日任务）、账单查询接口、前端列表、OBS 同步、终版确认流程
