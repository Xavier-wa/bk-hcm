## Context

agent-server 基于 trpc-agent-go 框架构建，已实现 LLM 调用、工具执行、Memory 操作的日志记录，并暴露 `/metrics` Prometheus 端点。当前问题：(1) 日志缺少 rid，无法串联调用链；(2) trpc-agent-go 的 OTel metrics 未桥接，无法监控 Agent 运营指标。

**当前架构**：
- `cmd/agent-server/logics/logger/` - 已有 LLM/Tool/Memory 日志记录（装饰器模式）
- `pkg/metrics/metric.go` - Prometheus Registry 和 `/metrics` 端点
- `cmd/agent-server/service/service.go` - `bkapiContextMiddleware` 已读取/生成 rid 写入 request header（`constant.RidKey`），但**未注入 context**
- `pkg/rest/request.go` - 已有私有的 `ridFromContext(ctx)`，用于 HTTP 客户端 outbound 请求时取 rid

**约束**：
- rid 当前仅写入 request header，callback / 中间件需要 context value 才能拿到 rid，因此需要补充 `context.WithValue` 注入
- trpc-agent-go 使用 OpenTelemetry SDK（`go.opentelemetry.io/otel`），不能直接注册到 Prometheus；框架内置 metrics 由 `trpcmetric.InitMeterProvider` 触发注册
- 不能影响现有 Prometheus 指标（`hcm_version_info` 等）

## Goals / Non-Goals

**Goals:**
- 在所有 Agent 日志中注入 rid，支持调用链追踪
- 将 trpc-agent-go 的 13 个 OTel 指标桥接到 Prometheus，通过 `/metrics` 暴露
- 保持代码改动最小化，复用现有基础设施

**Non-Goals:**
- 不实现分布式 Tracing（等待 trpc-agent-go PR 合入）
- 不修改日志格式和内容（仅添加 rid 字段）
- 不改变现有 `/metrics` 端点路径和认证机制

## Decisions

### 决策 1：复用 `pkg/rest` 中的 rid 提取函数，避免重复实现

**方案**：将 `pkg/rest/request.go` 中原私有的 `ridFromContext(ctx) string` 提升为导出函数 `RidFromContext`，agent-server 各 callback / 中间件统一通过 `rest.RidFromContext(ctx)` 获取 rid。

**理由**：
- `pkg/rest` 已实现 nil context、类型断言、空值降级等完整容错逻辑，无需重复造轮子
- 与 HTTP 客户端使用的 rid 提取逻辑保持一致，避免不同位置出现行为分歧
- 后续调整 rid 提取策略（如添加新的 fallback 来源）只需修改一处

**替代方案**：
- ❌ 在 `cmd/agent-server/logics/logger/` 新增 `rid.go` - 与 `pkg/rest` 中的实现重复，维护成本高
- ❌ 每个 callback 独立实现 `ctx.Value(...).(string)` - 代码重复，容错逻辑容易不一致
- ❌ 使用 `kit.Kit` 透传 - callback 层面没有 kit 对象，需要重构参数传递

### 决策 2：日志格式统一在消息末尾追加 `rid: %s`

**方案**：所有日志在消息末尾固定追加 `, rid: %s` 字段，rid 为空时也输出（呈现为 `rid: `）；rid 取值来自 `rest.RidFromContext(ctx)`。

**理由**：
- 与项目现有日志规范保持一致（参考 `pkg/rest/handler.go`）
- `rid: ` 前缀便于日志系统解析和过滤（`grep "rid: <uuid>"`）
- 不为空值场景做分支判断，保持日志模板统一、可读，避免每个调用点写两份日志语句

**替代方案**：
- ❌ 空 rid 时省略 `rid:` 字段 - 需要每个日志点写 if/else 两份模板，重复且易遗漏
- ❌ 使用 JSON 结构化日志 - 需要重构整个日志系统，成本过高

### 决策 3：bkapiContextMiddleware 同时注入 context 与响应头

**方案**：
1. 在 `bkapiContextMiddleware` 中读取或生成 rid 后，通过 `context.WithValue(ctx, constant.RidKey, rid)` 注入到请求 context，使后续 callback / 中间件可通过 `rest.RidFromContext` 取到
2. 新增 `ridResponseWriter` 包装 `http.ResponseWriter`，在 `WriteHeader` / `Write` 首次调用前向响应头写入 `X-Bkapi-Request-Id`，便于客户端在异步流式响应中也能拿到 rid

**理由**：
- request header 是给下游服务用的；callback 链路里只有 context 可用，必须显式注入
- 流式响应（SSE/AG-UI）和异常分支都可能在 handler 内部决定 status code，包装 writer 能保证不论谁先写入，rid 头都不会丢
- 写入 header 的逻辑用 `headerWritten` 标志保证只写一次，避免 status code 已下发后又设置 header 的非法操作

**替代方案**：
- ❌ 仅在 handler 里手动 `w.Header().Set(...)` - 容易漏掉部分路径，无法覆盖框架托管的响应
- ❌ 在最外层中间件再包一层只负责设置响应头 - 与 rid 生成逻辑分离，难以保证两者使用同一 rid

### 决策 4：OTel Metrics 使用官方 Prometheus Exporter 桥接，并显式触发 trpc-agent-go 内置指标注册

**方案**：
1. 添加依赖 `go.opentelemetry.io/otel/exporters/prometheus` 和 `go.opentelemetry.io/otel/sdk/metric`
2. 在 `pkg/metrics/otel.go` 创建 `InitOTelMetrics(registry prometheus.Registerer) error` 函数（参数使用 `Registerer` 接口而非具体类型，便于测试时传入 mock）
3. 使用 `promexporter.New(promexporter.WithRegisterer(registry), promexporter.WithoutScopeInfo())` 创建 Exporter，`WithoutScopeInfo` 关闭冗余的 `otel_scope_*` 元信息，减少指标膨胀
4. 创建 `metric.NewMeterProvider(metric.WithReader(exporter))` 并通过 `otel.SetMeterProvider(provider)` 设置为全局 provider
5. 在 `cmd/agent-server/app/app.go` 的 `InitMetrics()` 后依次调用：
   - `metrics.InitOTelMetrics(metrics.Register())` - 桥接 OTel 到 Prometheus Registry
   - `trpcmetric.InitMeterProvider(otel.GetMeterProvider())` - 显式触发 trpc-agent-go 内置指标在全局 provider 上注册

**理由**：
- OpenTelemetry 官方 Prometheus Exporter 自动处理指标类型转换（Counter/Histogram → Prometheus 格式）
- 桥接到同一 Registry 确保 OTel 和现有指标通过同一 `/metrics` 端点暴露
- trpc-agent-go 的内置指标采用懒注册模式，需要在框架使用 meter 前显式调用 `InitMeterProvider`，否则 `/metrics` 中不会出现框架指标

**替代方案**：
- ❌ 手动将 OTel Collector 指标推送到 Prometheus - 需要额外部署 Collector 组件，架构复杂度高
- ❌ 重写 trpc-agent-go 指标为 Prometheus 原生格式 - 需要 fork 框架代码，维护成本高
- ❌ 仅 `otel.SetMeterProvider` 不调用 `trpcmetric.InitMeterProvider` - 测试发现框架内置指标不会出现在 `/metrics`

### 决策 5：OTel 初始化失败中断服务启动

**方案**：`InitOTelMetrics()` 返回 error，初始化失败时 `app.prepare()` 返回错误并终止服务启动。

**理由**：
- 可观测性是核心能力，初始化失败意味着监控缺失，应及早暴露问题
- 避免服务在无指标状态下运行，导致生产问题难以排查

**替代方案**：
- ❌ 初始化失败仅记录 Warn 日志 - 服务带病运行，生产问题难以发现

## Risks / Trade-offs

### 风险 1：Context 中缺少 rid 导致日志中出现空 rid

**风险**：如果某些调用路径（如启动阶段、定时任务）context 中没有 rid，日志末尾会出现 `rid: `（空值）。

**缓解措施**：
- `rest.RidFromContext()` 在 ctx 为 nil 或类型断言失败时返回空字符串而非 panic，确保业务逻辑不受影响
- HTTP 入口处 `bkapiContextMiddleware` 在缺失时通过 `uuid.UUID()` 兜底生成 rid 并注入 context，覆盖绝大多数请求路径
- 监控日志中 `rid: ` 后无内容的比例，识别缺失场景并补充 rid 注入点

### 风险 2：OTel 依赖版本冲突

**风险**：trpc-agent-go 和新增的 OTel Exporter 依赖不同 OTel 版本，可能导致编译或运行时错误。

**实际处理**：
- `go.opentelemetry.io/otel` 系列从 v1.29.0 统一升级到 v1.38.0，与 OTel Prometheus Exporter v0.51.0 兼容
- `github.com/prometheus/client_golang` 从 v1.14.0 升级到 v1.20.1，以满足新版 Exporter 的最低版本要求
- 通过 `go mod tidy` + 本地启动验证指标注册和暴露正常

### 风险 3：指标量激增导致 /metrics 响应变慢

**风险**：新增 13 个 OTel 指标，每个指标可能有多个维度标签，指标基数（cardinality）增加可能影响 `/metrics` 端点性能。

**缓解措施**：
- OTel 指标维度已由 trpc-agent-go 框架控制，不新增自定义维度
- Prometheus 默认对高基数指标有性能优化（如采样）
- 监控 `/metrics` 端点响应时间，必要时启用 Prometheus 的 `metric_relabel_configs` 过滤低价值维度

## Migration Plan

**部署步骤**：
1. 合并代码到主干
2. 本地测试验证：
   - 启动 agent-server，检查 OTel 初始化日志
   - 发起 Agent 请求，验证日志包含 rid
   - 访问 `/metrics`，确认 OTel 指标暴露（如 `trpc_agent_go_client_request_cnt`）
3. 灰度发布到测试环境，监控 `/metrics` 性能和日志完整性
4. 全量发布到生产环境

**回滚策略**：
- 代码回滚：恢复到上一版本，OTel 指标消失但不影响业务功能
- 降级方案：如 OTel 初始化失败，可临时注释 `InitOTelMetrics()` 调用并重新编译

**验证标准**：
- 日志中 90%+ 请求包含 rid（排除启动和定时任务）
- `/metrics` 端点包含所有 13 个 OTel 指标
- `/metrics` 响应时间 < 200ms（p95）

## Open Questions

1. ❓ 是否需要在日志中添加其他 context 字段（如 user_id、session_id）？
   - **当前决策**：仅添加 rid，其他字段等待业务需求明确后再扩展
   
2. ❓ OTel 指标是否需要自定义前缀（如 `hcm_agent_*`）以区分 HCM 自有指标？
   - **当前决策**：保持 trpc-agent-go 原始指标名（`trpc_agent_go_*` / `gen_ai_*`），便于与框架文档对齐
   
3. ❓ 是否需要为 OTel Metrics 添加自定义维度（如 `deployment`、`cluster`）？
   - **当前决策**：不添加，复用 Prometheus Registry 的全局 labels（`process_name`、`host`）
