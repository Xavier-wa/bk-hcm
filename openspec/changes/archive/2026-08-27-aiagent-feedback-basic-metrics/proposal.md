## Why

HCM Agent 上线后没有 run 级运行时统计：框架轨道表 `aiagent_session_track_events` 只是事件流水，`aiagent_session` 只记会话与消息计数，产品无法回答「用了多少次、完成了多少、取消率与耗时怎样」。本期只落地 Prometheus 指标，作为看板与告警的即时口径。

## What Changes

- 新增 `hcm_aiagent_*` 监控指标（照抄 `pkg/metrics` 既有写法，**无独立 enabled 开关**）：`run_total` / `run_duration_seconds` / `run_inflight` / `run_cancel`。
- 仅在真实 `POST /agui` 且 session 校验通过后挂本轮元数据，事件回调就地打点；`/history` 不挂、不计。
- `POST /cancel` 鉴权解析后立刻计 `run_cancel`，不减 `inflight`，不把 `cancel` 写入 `run_total`。

**本期不做**：`aiagent_run` 表、DAO、data-service 内部接口、事件回调异步落库、`/cancel` CAS、`pkg/cron` 孤儿清扫、主动反馈 UI / 反馈表、被动模型评估、`stats/run_overview`、`stats/run_list`、新 IAM 权限点、`runObservation.enabled` 总开关。

## Capabilities

### New Capabilities

- `aiagent-run-metrics`（**已交付**，已写入 `openspec/specs/aiagent-run-metrics/`）：`hcm_aiagent_*` 指标定义、主动注册、就地打点规则（含 cancel 独立）。无业务开关，与现有 `hcm_clb_*` / `hcm_http_*` 一致。

### Modified Capabilities

无。现有 `agent-metrics-export` 只覆盖 trpc-agent-go OTel 桥接，不改其需求。

## Impact

**受影响服务**：

| 层 | 服务 | 改动 |
| --- | --- | --- |
| Service | agent-server | `RunMeta`、事件就地打点、`/cancel` 计 `run_cancel`、`EnsureAiagentMetric` |

**受影响代码**：

- `pkg/metrics/`：`AiagentSubSys` + `aiagent.go`
- `cmd/agent-server/`：`InitMetrics` 后注册、中间件挂 `RunMeta`、translator / `/cancel` 打点

**不涉及**：data-service / 建表 / DAO、auth-server / IAM、cloud-server / woa-server、对外 stats API、前端、新外部依赖、主动反馈与模型评估落地。

**风险**：

- 打点跑在事件回调 / `/cancel` 热路径，必须不阻塞、不 panic 外泄、不回写事件。
- `/cancel` 之后框架仍可能补发 `RUN_FINISHED` / `RUN_ERROR`，看板完成率会把这批算进 `finished`；取消量看 `run_cancel`。
