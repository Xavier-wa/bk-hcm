## Why

账单 Controller 启动时所有 goroutine 在同一时刻触发，导致 `id_generator` 表的 `SELECT ... FOR UPDATE` 语句出现高并发行锁竞争，每小时 15 分左右产生约 4k 条超过 1 秒的慢查询。

## What Changes

- 在 `pkg/tools/utils/wait/JitterUntil` 中增加启动前的随机延迟：最长 1 分钟，若 `period < 1min` 则以 `period` 为上限，并在 `jitterFactor > 0` 时对延迟做抖动，使各 goroutine 的首次执行时间均匀分散。
- 将 `rootaccount_controller.go`、`mainaccount_controller.go`、`summarydaily_controller.go`、`dailysplit_controller.go` 的手写 `time.NewTicker` 循环替换为 `wait.JitterUntil`，消除原有的"立即执行一次"模式，统一由 `JitterUntil` 内部的启动延迟控制首次执行时机。

## Capabilities

### New Capabilities

无新增能力。

### Modified Capabilities

- `wait/JitterUntil`：在首次执行 `f()` 之前新增随机启动延迟（`[0, min(1min, period)] × (1 + jitterFactor)` 范围内），context 取消时可提前退出。

## Impact

- **修改文件**: `pkg/tools/utils/wait/wait.go`（JitterUntil 增加启动延迟）、`rootaccount_controller.go`、`mainaccount_controller.go`、`summarydaily_controller.go`、`dailysplit_controller.go`（改用 JitterUntil 循环）
- **无 API 变更**，无数据库 schema 变更，纯内部重构
