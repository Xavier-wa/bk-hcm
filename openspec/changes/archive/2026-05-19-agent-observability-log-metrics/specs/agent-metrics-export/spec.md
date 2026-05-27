## ADDED Requirements

### Requirement: OTel Metrics 必须桥接到 Prometheus

trpc-agent-go 框架内置的 OpenTelemetry metrics（包括 Chat、Tool、Agent 三大类共 13 个指标）MUST 通过 Prometheus Exporter 桥接到现有 Prometheus Registry，并通过 `/metrics` 端点暴露。

#### Scenario: OTel Chat 指标可通过 /metrics 获取

- **WHEN** Agent 执行 LLM 推理并产生 Token 消耗
- **THEN** 访问 `/metrics` 端点可获取 `trpc_agent_go_client_request_cnt`、`gen_ai_client_token_usage` 等 Chat 指标

#### Scenario: OTel Tool 指标可通过 /metrics 获取

- **WHEN** Agent 调用工具并记录执行时间
- **THEN** 访问 `/metrics` 端点可获取 `gen_ai_client_operation_duration{tool_name="xxx"}` 等 Tool 指标

#### Scenario: OTel Agent 指标可通过 /metrics 获取

- **WHEN** Agent Run 完成并记录 Token 使用量和耗时
- **THEN** 访问 `/metrics` 端点可获取 `gen_ai_request_cnt`、`gen_ai_client_time_to_first_token` 等 Agent 指标

### Requirement: 指标维度必须完整保留

桥接过程 MUST 保留 trpc-agent-go 原始指标的所有维度（labels），包括 model、tool_name、agent_name、user_id、session_id、error_type、stream、token_type 等，确保指标可按维度聚合分析。

#### Scenario: 指标维度完整性验证

- **WHEN** 访问 `/metrics` 端点获取 `gen_ai_client_token_usage` 指标
- **THEN** 指标包含 `model` 和 `token_type` 维度标签

#### Scenario: 多维度聚合查询支持

- **WHEN** Prometheus 查询 `sum(gen_ai_client_token_usage{model="gpt-4o",token_type="input"})`
- **THEN** 返回该模型输入 Token 总消耗量

### Requirement: 指标初始化必须在服务启动时完成

OTel MeterProvider、Prometheus Exporter 以及 trpc-agent-go 内置指标 MUST 在 agent-server 启动阶段初始化，早于任何 Agent 调用，确保从服务启动开始就能采集指标。

#### Scenario: 服务启动时初始化 OTel Metrics

- **WHEN** agent-server 启动并执行 `InitMetrics()` 流程
- **THEN** `metrics.InitOTelMetrics(metrics.Register())` 创建 Prometheus Exporter 并将其作为 Reader 注册到全局 MeterProvider（通过 `otel.SetMeterProvider`）

#### Scenario: 显式触发 trpc-agent-go 内置指标注册

- **WHEN** OTel MeterProvider 设置完成后
- **THEN** 必须调用 `trpcmetric.InitMeterProvider(otel.GetMeterProvider())`，确保框架内置 13 个指标在第一次 Agent 调用之前已注册到 Prometheus Registry

#### Scenario: 初始化失败时服务启动中断

- **WHEN** OTel Metrics 或 trpc-agent-go MeterProvider 初始化失败（如 Exporter 创建错误）
- **THEN** 服务启动流程中断，返回错误并终止进程

### Requirement: 不影响现有 Prometheus 指标

新增的 OTel 指标桥接 MUST 与现有 Prometheus 指标（如 `hcm_version_info`、进程指标、Go runtime 指标）共存，不能覆盖或冲突。

#### Scenario: 现有指标正常暴露

- **WHEN** OTel Metrics 桥接完成后访问 `/metrics` 端点
- **THEN** 同时包含现有 HCM 指标（`hcm_version_info`）和新增 OTel 指标（`trpc_agent_go_*`）

#### Scenario: 指标名称无冲突

- **WHEN** OTel Metrics 和现有 Prometheus 指标注册到同一 Registry
- **THEN** 无指标名称冲突错误，所有指标正常注册
