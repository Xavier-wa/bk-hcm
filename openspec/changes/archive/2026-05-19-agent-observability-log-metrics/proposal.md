## Why

Agent-server 当前虽已实现 LLM 调用、工具执行等基础日志记录和 /metrics 端点，但存在两个关键可观测性缺陷：(1) 日志缺少 rid 标识，无法串联调用链路排查问题；(2) trpc-agent-go 内置的 OTel metrics（Token 消耗、LLM 延迟等）未桥接到 Prometheus，无法监控 Agent 业务指标。

## What Changes

- **日志 rid 注入**：在所有 Agent callback（Model/Tool/Memory）、Skill 图节点、Embedding 中间件、动态工具索引/过滤等日志中注入 rid，实现调用链追踪
- **rid 上下文与响应头**：`bkapiContextMiddleware` 显式将 rid 写入 `context`，并通过包装 `http.ResponseWriter` 在响应头中回写 `X-Bkapi-Request-Id`，便于客户端关联调用链
- **OTel Metrics 桥接**：使用 OpenTelemetry Prometheus Exporter 将 trpc-agent-go 的 13 个内置指标（Chat/Tool/Agent 三大类）桥接到现有 Prometheus Registry；同时通过 `trpcmetric.InitMeterProvider` 显式触发框架内置指标注册，并通过 /metrics 端点暴露

## Capabilities

### New Capabilities

- `agent-log-tracing`: Agent 日志 rid 追踪能力 - 在 LLM、Tool、Memory 等模块日志中统一注入 rid，支持跨模块调用链串联
- `agent-metrics-export`: Agent 指标导出能力 - 桥接 OTel metrics 到 Prometheus，暴露 Token 消耗、LLM 延迟、工具执行时间等运营指标

### Modified Capabilities

<!-- 无需求变更的现有能力 -->

## Impact

**受影响代码**：
- `cmd/agent-server/logics/logger/` - 日志模块（`model.go`/`tool.go`/`memory.go`），全部改为通过 `rest.RidFromContext` 提取 rid 并附加到日志末尾
- `cmd/agent-server/logics/model/model_callbacks.go` - `MakeHistoricalToolResultFilter` 日志注入 rid
- `cmd/agent-server/logics/tool/` - `tool_callbacks.go`/`tool_confirm.go`/`tool_filter.go`/`tool_index.go` 日志注入 rid
- `cmd/agent-server/logics/skill/` - `graph.go`/`prompt_inject.go` 中 Skill 图回调与 Prompt 注入回调日志加 rid
- `cmd/agent-server/logics/embedding/embedding.go` - Embedding HTTP 中间件错误日志加 rid
- `cmd/agent-server/service/service.go` - `bkapiContextMiddleware` 将 rid 注入 context；新增 `ridResponseWriter` 将 rid 回写到响应头
- `pkg/rest/request.go` - 将原私有 `ridFromContext` 提升为导出函数 `RidFromContext`，供 agent-server 复用
- `pkg/metrics/otel.go` - 新增 `InitOTelMetrics(registry prometheus.Registerer)` 桥接逻辑
- `cmd/agent-server/app/app.go` - 在 `InitMetrics()` 后调用 `InitOTelMetrics()`，并调用 `trpcmetric.InitMeterProvider(otel.GetMeterProvider())` 触发框架内置指标注册

**依赖变更**：
- 新增 `go.opentelemetry.io/otel/exporters/prometheus`、`go.opentelemetry.io/otel/sdk/metric`
- `go.opentelemetry.io/otel`/`otel/metric`/`otel/trace` 升级到 v1.38.0
- `github.com/prometheus/client_golang` 升级到 v1.20.1（满足 OTel Prometheus Exporter 的版本要求）

**API/配置变更**：无（纯内部可观测性增强；响应头新增 `X-Bkapi-Request-Id` 仅为附加字段，不破坏现有契约）
