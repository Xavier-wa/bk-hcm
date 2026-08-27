> 本期范围：仅 `aiagent-run-metrics`。`aiagent_run` 表 / DAO / data-service / 异步落库 / `/cancel` CAS / cron 清扫已移出本 change。

## 1. 监控指标（无 enabled 开关）

- [x] 1.1 在 `pkg/metrics/metric.go` 新增 `AiagentSubSys = "aiagent"`
- [x] 1.2 新建 `pkg/metrics/aiagent.go`：仅 `run_total` / `run_duration_seconds` / `run_inflight` / `run_cancel`；label 用 `bkcc_biz_id`；`EnsureAiagentMetric` + 不返回 error 的 Observe/Inc；**不要** `write_fail_total`
- [x] 1.3 在 `cmd/agent-server/app/app.go` 的 `InitMetrics` 之后立刻调用 `EnsureAiagentMetric()`

## 2. 本轮元数据（仅打点用）

- [x] 2.1 定义 `RunMeta`，仅在 `sessionCodeMiddleware` 的 `/agui` 路径、session 校验通过后写入 request context；`/history` 不挂

## 3. 测试与核对

- [x] 3.1 补指标核对：running / cancel 不进 `run_total`；cancel 不减 inflight；耗时只有 finished/error；无 `write_fail_total`；无 `runObservation.enabled`
