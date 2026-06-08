## MODIFIED Requirements

### Requirement: JitterUntil 在首次执行前增加随机启动延迟

`JitterUntil` SHALL 在进入周期循环、首次调用 `f()` 之前，等待一段随机延迟：
- 基准延迟为 `min(1min, period)`
- 若 `jitterFactor > 0`，对基准延迟应用 `Jitter(delay, jitterFactor)` 进行二次抖动
- 等待期间若 context 被取消，则立即退出，不执行 `f()`

#### Scenario: 正常启动，period 大于 1 分钟

- **WHEN** 以 `period > 1min`、`jitterFactor = 0.5` 调用 `JitterUntil`
- **THEN** 首次执行 `f()` 前等待 `[1min, 1.5min)` 范围内的随机时长

#### Scenario: period 小于 1 分钟时 delay 被 cap

- **WHEN** 以 `period = 20s`、`jitterFactor = 0.5` 调用 `JitterUntil`
- **THEN** 启动延迟上限为 `20s`，实际等待时长在 `[20s, 30s)` 范围内

#### Scenario: context 取消时提前退出，不执行 f()

- **WHEN** 调用 `JitterUntil` 后，启动延迟等待期间 context 被取消
- **THEN** `f()` 不被调用，函数正常返回

#### Scenario: jitterFactor 为 0 时延迟不做抖动

- **WHEN** 以 `jitterFactor = 0` 调用 `JitterUntil`，`period = 2min`
- **THEN** 启动延迟固定为 `1min`，无随机抖动

### Requirement: bill controller 使用 JitterUntil 替代手写 ticker 循环

bill 包内的 `RootAccountController`、`MainAccountController`、`MainSummaryDailyController`、`MainDailySplitController` SHALL 使用 `wait.JitterUntil` 实现周期同步循环，不再使用 `time.NewTicker` 手写循环。

#### Scenario: 移除立即执行一次的模式

- **WHEN** controller `Start()` 被调用
- **THEN** 对应的同步 goroutine 不再在启动时立即执行一次，而是等待 JitterUntil 内的随机延迟后再首次执行
