## Context

账单同步模块（`cmd/account-server/logics/bill`）为每个账号创建独立的 Controller（`RootAccountController`、`MainAccountController`、`DailySummaryController`、`DailySplitController`）。每个 Controller 在 `Start()` 时会启动若干定时 goroutine，这些 goroutine 在首次触发时都会调用 `id_generator` 的 `SELECT ... FOR UPDATE` 语句申请自增 ID。

当大量账号同时被初始化时（例如系统重启或每小时 15 分的定时同步窗口），所有 goroutine 在相同时刻并发打到同一张表，导致行锁排队，出现大批量慢查询（实测每小时约 4k 条 > 1s 的慢日志）。

`pkg/tools/utils/wait` 包中已存在 `JitterUntil` 函数，用于带 jitter 的周期性循环，但原实现在进入循环前会立即执行一次 `f()`，无法分散各 controller 的启动时刻。

## Goals / Non-Goals

**Goals:**
- 在 `JitterUntil` 中增加启动前随机延迟，将各 controller 的首次执行时刻分散到 `[0, min(1min, period)]` 范围内
- 将 bill 包内手写的 `time.NewTicker` 循环统一迁移为 `wait.JitterUntil`，减少重复代码

**Non-Goals:**
- 不修改各 Controller 的业务逻辑或定时间隔配置
- 不引入外部依赖
- 不解决 `id_generator` 表本身的设计问题（如分段缓存、乐观锁等）

## Decisions

### 决策 1：将启动延迟嵌入 `JitterUntil` 而非新建独立函数

**方案 A**: 新建 `RunWithJitter(ctx, maxJitter, fn)` 一次性启动工具，在 controller `Start()` 中包裹每个 goroutine 的启动

**方案 B（采用）**: 在 `JitterUntil` 入口处直接增加启动延迟逻辑

**理由**: bill 包内的 controller 本身就需要将手写 ticker 循环迁移为 `JitterUntil`，启动延迟可以自然内聚在循环函数内，无需在调用侧额外包装。调用方代码改动最小，语义也更连贯：`JitterUntil` 描述的就是"带抖动的周期循环"，启动延迟是其抖动语义的自然延伸。

---

### 决策 2：延迟上限取 `min(1min, period)`

**理由**: 若 `period` 本身小于 1 分钟（如 dispatcher 中的 2s、5s、20s 场景），以 `period` 作为 delay 上限可避免启动延迟超出一个完整周期，使调用方行为保持合理。

---

### 决策 3：沿用 `jitterFactor` 对启动延迟做二次抖动

**理由**: 当 `jitterFactor > 0` 时，对启动延迟再做一次 `Jitter(delay, jitterFactor)` 可使分布更均匀，且复用已有的 `Jitter` 函数，无需额外参数。

## Risks / Trade-offs

- **[风险] 影响所有 `JitterUntil` 调用方**: 修改 `JitterUntil` 行为是全局性的，现有调用方（如 `dispatcher.go`、`res-sync/service.go`）会受到意料之外的启动延迟。其中 dispatcher 的 period 较短（2~20s），delay 被 cap 后影响有限，但仍需调用方评估是否可接受。
- **[Trade-off] 隐式行为**: 启动延迟是 `JitterUntil` 的隐式行为，不体现在函数签名中，需通过注释说明。
- **[风险] 行为变更**: bill controller 原有的"立即执行一次"逻辑被移除，首次执行会延迟，需确认上层业务可接受。

## Migration Plan

1. 修改 `pkg/tools/utils/wait/wait.go`，在 `JitterUntil` 入口增加启动延迟逻辑
2. 将四个 bill controller 的 `time.NewTicker` 手写循环替换为 `wait.JitterUntil`
3. 移除 controller 中原有的"立即执行一次"调用
4. 编译验证，无 API/数据库变更，直接部署即可

## Open Questions

无
