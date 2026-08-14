## 1. 基础设施准备

- [x] 1.1 将 `pkg/rest/request.go` 中私有的 `ridFromContext` 提升为导出函数 `RidFromContext`，并同步更新 `Request.Do()` 内部调用
- [x] 1.2 在 `go.mod` 添加 OTel 依赖（`go.opentelemetry.io/otel/exporters/prometheus`、`go.opentelemetry.io/otel/sdk/metric`），并将 `go.opentelemetry.io/otel` 系列升级到 v1.38.0、`github.com/prometheus/client_golang` 升级到 v1.20.1

## 2. rid 上下文与响应头注入

- [x] 2.1 修改 `cmd/agent-server/service/service.go` - `bkapiContextMiddleware` 中通过 `context.WithValue(ctx, constant.RidKey, rid)` 将 rid 注入请求 context
- [x] 2.2 新增 `ridResponseWriter`，在 `WriteHeader` / `Write` 首次调用前向响应头写入 `X-Bkapi-Request-Id`，并在中间件中包装 `http.ResponseWriter`

## 3. 日志 rid 注入 - Logger 模块

- [x] 3.1 修改 `cmd/agent-server/logics/logger/model.go` - `MakeModelLoggerCallback()` 所有日志通过 `rest.RidFromContext` 取 rid 并附加到日志末尾
- [x] 3.2 修改 `cmd/agent-server/logics/logger/model.go` - `LLMRequestLogger()` 中间件以及 `logLLMToolsSummary` / `logLLMTokenConfig` 透传 rid
- [x] 3.3 修改 `cmd/agent-server/logics/logger/tool.go` - `BeforeTool` / `AfterTool` callback 日志附加 rid
- [x] 3.4 修改 `cmd/agent-server/logics/logger/tool.go` - `mcpHTTPRespLoggingHandler.Handle()` 错误及响应日志附加 rid
- [x] 3.5 修改 `cmd/agent-server/logics/logger/memory.go` - `loggingMemoryService` 全部方法（Add / Update / Delete / Clear / Read / Search / EnqueueAutoMemoryJob）日志附加 rid

## 4. 日志 rid 注入 - Callbacks / Filters / Skill / Embedding

- [x] 4.1 修改 `cmd/agent-server/logics/model/model_callbacks.go` - `MakeHistoricalToolResultFilter()` 日志附加 rid
- [x] 4.2 修改 `cmd/agent-server/logics/tool/tool_callbacks.go` - `MakeParamFixCallbacks()` 及 `fixStringParams` / `fixNestedParams` 透传 rid
- [x] 4.3 修改 `cmd/agent-server/logics/tool/tool_confirm.go` - `confirmTool.Call` 日志附加 rid
- [x] 4.4 修改 `cmd/agent-server/logics/tool/tool_filter.go` - `MakeDynamicToolFilter()` 日志附加 rid
- [x] 4.5 修改 `cmd/agent-server/logics/tool/tool_index.go` - `LazyToolIndex.ensureBuild()` 与 `EmbeddingIndex.Search()` 日志附加 rid
- [x] 4.6 修改 `cmd/agent-server/logics/skill/graph.go` - `MakeSkillLoadAfterToolCallback()` 及辅助函数 `recordSkillLoadedToState` / `recordSkillSelectedDocsToState` 透传 rid
- [x] 4.7 修改 `cmd/agent-server/logics/skill/prompt_inject.go` - `MakeSkillInjectWithModelCallback()` 与辅助函数日志附加 rid
- [x] 4.8 修改 `cmd/agent-server/logics/embedding/embedding.go` - Embedding HTTP 中间件错误日志附加 rid

## 5. OTel Metrics 桥接实现

- [x] 5.1 创建 `pkg/metrics/otel.go`，实现 `InitOTelMetrics(registry prometheus.Registerer) error`
- [x] 5.2 在 `InitOTelMetrics()` 中通过 `promexporter.New(WithRegisterer, WithoutScopeInfo)` 创建 Exporter 并注册到 Registry
- [x] 5.3 在 `InitOTelMetrics()` 中通过 `metric.NewMeterProvider(metric.WithReader(exporter))` 构建 MeterProvider 并调用 `otel.SetMeterProvider` 设置为全局 provider
- [x] 5.4 修改 `cmd/agent-server/app/app.go` - 在 `InitMetrics()` 后依次调用 `metrics.InitOTelMetrics(metrics.Register())` 和 `trpcmetric.InitMeterProvider(otel.GetMeterProvider())`，初始化失败时返回错误中断启动

## 6. 测试验证

- [x] 6.1 本地启动 agent-server，确认 `otel metrics initialized and bridged to prometheus` 与 `trpc-agent-go metrics initialized` 日志存在
- [x] 6.2 发起 Agent 请求，验证所有日志包含 `rid: <uuid>` 字段
- [x] 6.3 验证响应头包含 `X-Bkapi-Request-Id`，且与日志 rid 一致
- [x] 6.4 访问 `/metrics` 端点，确认包含 OTel 指标（如 `trpc_agent_go_client_request_cnt`、`gen_ai_client_token_usage`）
- [x] 6.5 验证 OTel 指标维度完整（model、tool_name 等 labels 存在）
- [x] 6.6 验证现有 Prometheus 指标（如 `hcm_version_info`）未受影响
