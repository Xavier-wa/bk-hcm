## 1. 时间窗闸口

- [x] 1.1 在 `pkg/tools/times` 新增 `ShouldAutoPullBillPeriod(now time.Time, billYear, billMonth int) bool`：UTC 本月恒为 false；UTC 上月仅当 `now` 在 Asia/Shanghai 下 ≥ 当月 1 号 16:00 为 true；更早账期为 true。Location 用 `sync.Once` 缓存，LoadLocation 失败则 FixedZone UTC+8
- [x] 1.2 为闸口编写单测，至少覆盖：本月任意时刻 false；上月在 1 号 15:59 CST false、16:00 CST true、16:01 CST true；UTC 月切后、北京 16:00 前上月仍 false；跨年（1 月拉上月 12 月）；节假日不改变 16:00 判定

## 2. 只拦新建日任务，不拦续跑

- [x] 2.1 在 `daily.DailyPuller.EnsurePullTask`（或 `ensureDailyPulling` 补缺天循环）调用闸口：未开窗则不 `createDailyPullTaskStub`（`dayList` 置空），并 `logs.Infof`（vendor、账期、账号、rid）；已有任务的 FlowID / 失败重建逻辑必须仍执行
- [x] 2.2 不要在 `ensureDailyRawPullTask`、`syncDailySplit`、`syncDailySummary`、月汇总轮询、`EnsureMonthTask` 上按时间窗整段 return
- [x] 2.3 确认 AWS / Azure / GCP / Huawei 仍走同一 `DailyPuller`；Zenlayer `EnsurePullTask` 保持空实现；`reaccount.go` 不改

## 3. 汇总头与可观测性

- [x] 3.1 `ensureBillSummary`（主账号 / 根账号）保持为本月+上月建汇总头
- [x] 3.2 自动拉取失败路径保持现网 flow failed + `Warnf`/`Errorf`（vendor、账期、账号、rid），不新增通知通道，初版保持未就绪

## 4. 验证

- [x] 4.1 闸口单测通过：`go test ./pkg/tools/times/ -count=1`
- [x] 4.2 编译 account-server 及相关包，确认无编译错误
- [x] 4.3 自测清单（可用固定时钟或日志核对，不必真连云）：UTC 本月不新建 stub、已有 stub 仍续跑；北京 1 号 16:00 前上月不新建；16:00±5min 上月一次展开全部日期；无日任务时查询为空；Reaccount 状态门与接口不变
