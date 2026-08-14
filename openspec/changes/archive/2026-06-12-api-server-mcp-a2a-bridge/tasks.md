## 1. 配置与脚手架

- [x] 1.1 在 `pkg/cc/service.go::ApiServerSetting` 新增 `MCP MCPServerSetting` 字段（含 `Ingress`、`Internal`、`Bridge` 子段）与 `A2APassthrough A2APassthroughSetting` 字段，默认全部 `Enable: false`
- [x] 1.2 在 `pkg/cc/api_mcp.go` 新建文件，定义 `MCPServerSetting`、`MCPIngressSetting`、`MCPInternalSetting`、`MCPBridgeSetting`、`A2APassthroughSetting` 结构体，并补充 `trySetDefault` / `Validate`（超时、连接池等默认值）。**agent-server 实例地址通过 etcd 服务发现获取，不在 yaml 中手填 URL**（与 web-server proxy 同源）。 **2026-06-09 变更**：随任务 7 改造，`MCPSchemaSyncSetting` 已删除；`MCPInternalSetting` 改为持有 `OpenAPISpecPath / IncludeOperationIDs / InternalOnlyTools` 字段
- [x] 1.3 在 `cmd/api-server/etc/api_server.yaml` 补充 `mcp:` 与 `a2aPassthrough:` 段示例配置（含中文注释、必填 / 可选标注），与 1.2 默认值一致
- [x] 1.4 评估后**不新增** CLI flag（MCP / A2A 透传相关配置项通过 yaml 控制更合适）
- [x] 1.5 启动期校验：当 `mcp.internal.enable: true` 时 `mcp.internal.openapiSpecPath` 必填（且文件存在），`mcp.internal.includeOperationIDs` 允许为空；bridge / a2aPassthrough 由 etcd 服务发现兜底，**无需手填 URL**。 **2026-06-09 变更**：原校验项 `schemaSync.gatewayURL` 已下线（OpenAPI 改本地加载，详见任务 7）

## 2. 复用基础组件抽取

- [x] 2.1 抽取通用上行身份解析中间件 `cmd/api-server/service/mcp/middleware/identity.go`：内部调用 `pkg/runtime/gwparser.Parse`，将 `bk_username` / `tenant_id` / `app_code` / `rid` 写入 `ctx`，**MUST NOT** 调用 `peekRequest`。同时提供 `IdentityHTTPContextFunc`（给 trpc-mcp-go.WithHTTPContextFunc 用）与 `IdentityMiddleware`（给 A2A passthrough 的 `http.Handler` 链用）两种调用风格
- [x] 2.2 抽取通用下行 header 注入工具 `cmd/api-server/service/mcp/middleware/inject_header.go`：在 outbound HTTP 请求中注入 `X-Bkapi-User-Name`、`X-Bkapi-App-Code`、`X-Bk-Tenant-Id`、`X-Bkapi-Request-Id`、`X-Bkhcm-Caller-Source: api-server`；已有 header 会被覆盖以防上游伪造透传
- [x] 2.3 抽取 caller-source 鉴权中间件 `cmd/api-server/service/mcp/middleware/caller_source.go`：校验 `X-Bkhcm-Caller-Source: agent-server`，失败返回 HTTP 403 + `errf.PermissionDenied`。同时暴露纯函数 `RequireCallerSource(h, expected) error` 与 http handler 包装器 `CallerSourceMiddleware`
- [x] 2.4 抽取 metrics 注册工具 `cmd/api-server/service/mcp/metrics/metrics.go`：定义 `hcm_api_server_mcp_tools_call_total{tool,mcp_server_name,status}`、`hcm_api_server_mcp_tools_call_duration_seconds`、`hcm_api_server_mcp_bridge_active_tasks` 等指标；通过 `InitMCPMetrics()`（once 保护）注册到 `pkg/metrics` 全局 registerer，nil holder 时所有 accessor 静默 no-op。**2026-06-09 变更**：随任务 7 改造，`hcm_api_server_mcp_internal_schema_stale` 指标已下线（不再有网关同步链路）
- [x] 2.5 单元测试：`middleware/middleware_test.go`（22 个 case，覆盖 ParseFromHeader 成功 / 失败、KitFromCtx round trip、HTTPContextFunc 成功 / 失败、IdentityMiddleware 通过 / 403、RequireCallerSource 4 个分支、CallerSourceMiddleware 通过 / 拒绝 / 空 expected 直通、InjectInternalHeaders 全字段 / rid 自动生成 / 覆盖伪造 header / nil kt / nil dest no-panic / 空 callerSource、writeForbidden 响应体）+ `metrics/metrics_test.go`（6 个 case，覆盖 once 幂等、Counter 累加、Histogram 采样、Gauge Inc/Set、stale bool 映射、holder=nil 静默 no-op）。全部通过，零 lint 报错

## 3. MCP 协议编解码层（trpc-mcp-go 接入）

- [x] 3.1 新建 `cmd/api-server/service/mcp/server.go`：`BuildBaseServer(name, version, component, extraOpts...) *mcpsdk.Server` 工厂，统一注入 `WithServerPath("")`（禁用 SDK 内置 path 严格校验，让上层 mux 决定挂载路径）+ `WithStatelessMode(true)`（北/南向均无状态）+ `WithServerLogger(hcmLoggerAdapter)`。同时新建 `cmd/api-server/service/mcp/logger.go` 把 trpc-mcp-go.Logger（10 个方法）适配到 `hcm/pkg/logs`，Debug 级降级到 `logs.V(2)`
- [x] 3.2 实现 `cmd/api-server/service/mcp/handler.go::NewMCPHTTPHandler(server *mcpsdk.Server, opts ...HandlerOption) http.Handler`。**不**消费 request body、**不**包装 response 写入（仅最小 `statusCapturingWriter` 抓 status code 并透传 `http.Flusher`），保留 streamable HTTP / SSE 实时下发语义。`HandlerOption` 提供 `WithMiddleware` / `WithLogComponent` / `WithoutPanicRecovery` / `WithoutRequestLog`
- [x] 3.3 中间件分层（外→内）：panic recovery（兜底 500，仅在 WriteHeader 未发生时写入；SSE 中途 panic 仅记日志不破坏流）→ request log（begin/end + 耗时 + status + rid）→ 调用方注入链（identity/caller-source）→ SDK handler。识别 `statusCapturingWriter.wroteHeader` 避免在 SSE 路径写出 superfluous header
- [x] 3.4 单元测试 `handler_test.go`（14 个 case）：`initialize` 返回正确 `serverInfo`、`ping` 返回空 result、`tools/list`/`prompts/list`/`resources/list` 默认返回空数组、未知 method 返回 JSON-RPC `-32601`、**非法 JSON 在 trpc-mcp-go transport 层返回 HTTP 400**（实测 SDK 行为，与原任务文字所述 `-32700` 不同——更新为实际行为）、非 POST 返回 405、`WithMiddleware` 调用顺序断言（洋葱模型）、nil 中间件防御、panic recovery 兜底 500、`WithoutPanicRecovery` 时 panic 透传、`statusCapturingWriter` 二次 WriteHeader 抑制 + Flush 透传、Logger 适配器端到端不 panic

## 4. 北向 MCP ingress + tools/list 聚合

- [x] 4.1 在 `cmd/api-server/service/mcp/ingress/` 新建包，实现 `BuildIngressServer(cfg, bridge) *mcpsdk.Server`。**架构调整**：将 a2aClient 参数替换为 `BridgeHandler` 接口（含 `SendMessage` + `Cancel` 两个方法），让协议层（ingress）与业务层（bridge）解耦——任务 4 用 `NoopBridgeHandler` 占位即可独立编译 / 测试，任务 5 在 bridge 包提供真正基于 `trpc-a2a-go A2AClient` 的实现并通过 `Register` 注入
- [x] 4.2 在 ingress server 中注册唯一工具 `send_message`：`ToolAnnotations.Title="HCM Agent"`、`ReadOnlyHint=false`、`OpenWorldHint=true`；inputSchema 含 `text` / `contextId` / `bk_biz_id` / `model_name` 四字段，`required: ["text"]`；handler 完成参数校验（含 `TextMaxLength` 上限）后委托给 `BridgeHandler.SendMessage`。**SDK 版本与兼容性说明**：升级 trpc-mcp-go v0.0.14 → **v0.0.16**（拿到 SSE close panic、context metadata 等 fix）；该版本 `manager_tools.go:252` 依然故意丢弃 `_meta.progressToken`（CHANGELOG 未提修复，源码确认）。新增 `cancel_token.go`（JSON-RPC 层中间件 `mcpsdk.WithMiddleware`）兜底把 progressToken 写入 ctx——**仅用于支持 cancel 反查 (progressToken → A2A taskId)**；progress 通知发送由 SDK 自动注入的 `mcp.GetNotificationSender(ctx)` 完成，与本中间件无关。未来 SDK 修复后移除中间件即可，handler 侧已 fallback 到 `req.Params.Meta.ProgressToken`。
- [x] 4.3 在 ingress server 注册 `notifications/cancelled` 处理器：从 `notification.Params.AdditionalFields["requestId"]` 提取 progressToken，委托给 `BridgeHandler.Cancel`；缺 requestId 时记 warn 静默忽略
- [x] 4.4 `Register(mux, cfg, bridge) error` 把 ingress handler 挂到 `<basePath>/{mcp_server_name}/mcp/`（用 Go 1.22+ ServeMux 路径变量）。中间件链：`mcp.NewMCPHTTPHandler`（panic recovery + request log） → `middleware.IdentityMiddleware`（JWT 失败 403） → `pathMiddleware`（提取 mcp_server_name 写入 ctx） → SDK handler。`cfg.Enable=false` 时静默跳过挂载
- [x] 4.5 单元测试 `ingress_test.go`（15 个 case）：`TestToolsList_IdenticalAcrossMCPServerNames` 用 `reflect.DeepEqual` 验证 `/foo/mcp/` 与 `/bar-with-dash/mcp/` 两条路径 `tools/list` **完全相同**且仅含 send_message；其余覆盖：disable 不挂载、nil mux 报错、tools/list 字段完整性、tools/call 透传到 bridge（含 progressToken）、缺 text 拦截不调 bridge、text 超长拦截、缺 JWT 返回 403、notifications/cancelled 透传 + 缺 requestId 忽略、pathMiddleware 提取路径变量、NoopBridge 安全、pathPattern 三种 basePath 形态、progressTokenMiddleware 非 tools/call 透传 + nil token 忽略

## 5. MCP → A2A bridge（send_message handler）

- [x] 5.1 在 `cmd/api-server/service/mcp/bridge/a2a_client.go` 中实现 `trpc-a2a-go.A2AClient` 的 HTTP transport 单例（keep-alive，连接池 100，read timeout 300s 可配置）；agent-server 实例 SHALL 通过 `discovery.NewAPIDiscovery(cc.AgentServerName, dis).GetServers()` 获取，每次 `StreamMessage` 前选实例 + 拼路径 `/api/v1/agent/a2a`
- [x] 5.2 实现 `cmd/api-server/service/mcp/bridge/task_map.go::ProgressTaskMap`：并发安全 `map[progressToken]taskId`，含 max size + entry TTL（默认 10000 + 30 分钟），LRU 兜底
- [x] 5.3 实现 `cmd/api-server/service/mcp/bridge/send_message.go::HandleSendMessage(ctx, args, server) (CallToolResult, error)`：
  - 参数校验（`text` 必填）
  - 从 ctx 取出 `progressToken`
  - 调用 `A2AClient.StreamMessage(ctx, A2AMessage{text, contextId, metadata})`
  - 写入 ProgressTaskMap
  - 循环消费 A2A 流式事件
- [x] 5.4 实现事件转换：`TaskStatusUpdateEvent`/`TaskArtifactUpdateEvent` → MCP `notifications/progress`（通过 `server.SendNotification`），终态 → `CallToolResult`
- [x] 5.5 实现 `bridge.CancelByProgressToken(progressToken string)`：从 ProgressTaskMap 查 taskId，调用 `A2AClient.CancelTask(taskId)`；找不到时记 warn 日志不报错
- [x] 5.6 单元测试（mock A2A client）：
  - 文本对话端到端 happy path
  - contextId 首轮生成与二轮复用
  - 缺 text 返回 `-32602`
  - A2A 流中 5 个 artifact event 转换为 5 个 `notifications/progress`
  - `failed` 状态返回 `isError: true`
  - cancel 触发 `CancelTask`
  - ProgressTaskMap 溢出 LRU 淘汰

## 6. 北向 A2A passthrough（反向代理）

- [x] 6.1 新建 `cmd/api-server/service/a2apass/proxy.go::BuildPassthroughHandler(cfg, dis) http.Handler`：基于 `net/http/httputil.ReverseProxy`，`FlushInterval: -1`，禁用响应缓冲；`Director` 内每次调用 `discovery.NewAPIDiscovery(cc.AgentServerName, dis).GetServers()` 选 agent-server 实例并重写 `req.URL.Scheme` / `req.URL.Host`
- [x] 6.2 在外层中间件中调用 identity 中间件（2.1）并注入 header（2.2）
- [x] 6.3 在 `cmd/api-server/service/a2apass/a2apass.go::Register(mux *http.ServeMux)` 中将 handler 挂载到 `/api/v1/agent/a2a` 与 `/api/v1/agent/.well-known/`
- [x] 6.4 单元测试：
  - 反向代理保留 SSE 帧顺序与实时性（注入测试 server 模拟 agent-server）
  - request body / response body 字节级一致
  - 注入了 `X-Bkhcm-Caller-Source: api-server`
  - 缺 JWT 返回 403

## 7. 南向内部 HCM MCP server（OpenAPI 驱动）

> **2026-06-09 变更**：放弃「调用蓝鲸网关 MCP-proxy `tools/list` 周期同步 + 本地缓存 + 反向透传调用」模式（详见 design.md D7 / D12 更新），改为「启动时直接解析本地 OpenAPI 3.0 yaml → 注册 MCP tool list」单一数据源模式。`SchemaSyncer` 被 `OpenAPILoader` 替换；网关 URL / 缓存路径 / 周期同步等配置项一并下线。任务 7.1~7.5 保持勾选状态，代码与测试同步改写。

- [x] 7.1 **删除** `cmd/api-server/service/mcp/internal/schema_sync.go`，**新增** `cmd/api-server/service/mcp/internal/openapi_loader.go::OpenAPILoader`：
  - 启动时一次性读取 `MCP.Internal.OpenAPISpecPath` 指向的 OpenAPI 3.0 yaml（默认 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm_internal_mcp.yaml`）
  - 解析每个 path：`operationId → tool name`，`description / summary → tool description`，`requestBody + parameters → InputSchema (JSON Schema)`，`x-bk-apigateway-resource.backend.method/path → tool backend`
  - `responses.200.content.application/json.example` JSON 化序列化后追加到 tool description 末尾（弥补 `additionalProperties: true` 时 schema 缺失）
  - 支持 `MCP.Internal.IncludeOperationIDs` 白名单（精确匹配 + glob）
  - 解析失败（文件不存在 / yaml 语法错误 / 关键字段缺失）阻塞启动并返回错误
  - **不**发起任何网络调用、**不**写本地缓存、**不**启动周期 goroutine
- [x] 7.2 改写 `cmd/api-server/service/mcp/internal/dispatcher.go`：所有工具统一走「本地 backend 路由」，**删除** `forwardGateway` 全部代码：
  - dispatcher 持有 `name → ToolBackend{method, path}` 映射表，注册时由 `internal.Register` 一次性灌入
  - `tools/call` 时：构造内部 HTTP 请求，header 注入 `X-Bkapi-User-Name` / `X-Bkhcm-Caller-Source: api-server` / `X-Bkapi-Request-Id`，body = tool args（去除 path 参数）；目标 URL 由 ToolBackend 拼接（path 模板中的 `{bk_biz_id}` 等占位由 args 中同名字段替换）
  - 第一阶段 backend caller 抽象成接口，默认实现走 `http://localhost:<api-server-port>` self-call（不走外网），便于后续任务 8 集成时切换；测试用 mock backend 验证
- [x] 7.3 改写 `cmd/api-server/service/mcp/internal/internal.go::Register`：
  - 移除 `SchemaSyncer.Start(ctx)` 调用与 SchemaSyncer 相关代码
  - 启动时调用 `OpenAPILoader.Load(cfg.OpenAPISpecPath, cfg.IncludeOperationIDs)` 拿到 `[]mcpsdk.Tool` + `map[name]ToolBackend`
  - 合并 `InternalOnlyTools`（同名时覆盖 OpenAPI）后一次性注册到 `trpc-mcp-go.Server`
  - 其它（独立 server 实例、caller-source 中间件、bk_username 提取）保持不变
- [x] 7.4 改写 `cmd/api-server/etc/api_server.yaml` 配置示例：
  - **删除** `schemaSync` 段及其字段（`gatewayURL` / `interval` / `timeout` / `cachePath` / `staleAlertAfter`）与 `toolFilter` 字段
  - **新增** `openapiSpecPath` 字段示例（默认 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm_internal_mcp.yaml`）
  - **新增** `includeOperationIDs` 字段示例（白名单，glob 支持）
  - `internalOnlyTools` 升级为结构化条目（含 name/description/inputSchema/backend），用注释说明数量应控制在 5 个以内
  - 保留南向必须 NetworkPolicy / Path Strip 隔离公网访问的注释
- [x] 7.5 改写单元测试 `internal_test.go`：
  - **新增**：`TestOpenAPILoader_Load` 用真实 yaml fixture 验证 `operationId → tool name`、`requestBody → InputSchema`、`example → description 末尾`、`backend.path 剥离 url_path_prefix`
  - **新增**：`TestOpenAPILoader_IncludeWhitelist` 验证白名单 glob 命中与精确匹配
  - **新增**：`TestOpenAPILoader_FileMissingFailsLoudly` 验证文件缺失阻塞启动
  - **新增**：`TestDispatcher_LocalBackendCall` mock backend caller，验证 `tools/call` 注入 user header 与 path 模板替换
  - **新增**：`TestRegister_InternalOnlyOverridesOpenAPI` 验证 InternalOnlyTools 同名覆盖
  - **删除**：网关 mock server、`TestSchemaSyncer_*`、`TestDispatcher_RoutesInternalAndGatewayTools`、`TestDispatcher_GatewayTimeoutReturnsErrorResult`
  - **保留**：`TestRegister_MissingCallerSourceReturnsForbidden`

## 8. 路由集成到 api-server 主流程

- [x] 8.1 修改 `cmd/api-server/service/service.go::ListenAndServeRest()` 之前的 mux 装配阶段（**不修改** `proxy.go` / `filter.go`）：
  - 创建外层 `http.ServeMux`
  - 按顺序注册：① `/api/v1/mcp/servers/` → ingress handler；② `/api/v1/mcp/internal/hcm/mcp/` → internal handler；③ `/api/v1/agent/a2a` 与 `/api/v1/agent/.well-known/` → a2apass handler；④ `/` → 原 `proxy.apiSet().ServeHTTP`（catch-all）
- [x] 8.2 在 `cmd/api-server/app/app.go::Run()` 中根据 `mcp.ingress.enable` / `mcp.internal.enable` / `a2aPassthrough.enable` 条件初始化各组件，关闭时**不**注册对应路径，确保零额外资源开销
- [x] 8.3 在 main goroutine 启动时打印关键路径与开关状态日志（rid="bootstrap"）
- [x] 8.4 冒烟测试：
  - 默认全关：所有 `/api/v1/cloud/...` proxy 路径 200 OK，`/api/v1/mcp/...` 返回 404
  - `mcp.ingress.enable: true`：`POST /api/v1/mcp/servers/foo/mcp/ initialize` 返回合法响应；同时 `/api/v1/cloud/...` 仍 200 OK
  - `a2aPassthrough.enable: true`：`GET /api/v1/agent/.well-known/agent-card.json` 200 OK

## 9. agent-server 协同配置（仅文档与配置示例，不改 agent-server 代码）

- [x] 9.1 在 `cmd/agent-server/etc/agent_server.yaml` 注释中补充示例：`mcpServers[].type: "internal"` 指向 `http://api-server:8080/api/v1/mcp/internal/hcm/mcp/`（**仅注释示例，不实际启用以免影响存量部署**）
- [x] 9.2 验证 agent-server `mcpCallerOriginMiddleware` 在 `enforceCallerOrigin: true` 时能识别 api-server 注入的 `X-Bkhcm-Caller-Source: api-server` 通过；如不通过需在 design.md 中记录差异（**期望无需改动 agent-server**）
- [x] 9.3 端到端联调脚本 `scripts/mcp_a2a_smoke.sh`：模拟蓝鲸网关签发 JWT 调用 `/api/v1/mcp/servers/foo/mcp/` 流程；记录每步 request/response 到日志便于排查

## 10. OpenClaw SKILL 文档

- [x] 10.1 新建 `.cursor/skills/openclaw-mcp-access/SKILL.md`，含 YAML frontmatter（`name: openclaw-mcp-access`、`description`、`triggers`、`license: MIT`）
- [x] 10.2 撰写"访问入口"章节：base URL 模板、JWT 鉴权、Content-Type / Accept、protocolVersion、显式提示"仅 send_message 可用"
- [x] 10.3 撰写"调用流程示例"章节（4 步）：`initialize` / `notifications/initialized` / `tools/list` / `tools/call(send_message)`，每步含完整 HTTP 请求 + 响应示例
- [x] 10.4 撰写"参数语义"章节：`text` / `contextId` / `bk_biz_id` / `model_name` 四字段表格，明确必填与上限，禁止扩展字段，contextId 由 HCM 维护
- [x] 10.5 撰写"错误处理"章节：HTTP 403/429/5xx + JSON-RPC -32601/-32602/-32603 共 6 项错误码与 OpenClaw 应有响应
- [x] 10.6 撰写"取消与超时"章节：`notifications/cancelled` 用法、progress 解析、推荐超时 300s、SSE 心跳 30s
- [x] 10.7 撰写"安全与审计"章节：审计字段、prompt injection 警示、多租户 JWT 警示
- [x] 10.8 自查：**不修改** `.cursor/skills/code-review-vibe/SKILL.md`；frontmatter `description ≤ 200 字`

## 11. 单元测试 / 集成测试 / 回归

- [x] 11.1 新增单元测试覆盖率目标：`cmd/api-server/service/mcp/...` 包行覆盖 ≥ 70%
- [x] 11.2 集成测试：起 fake agent-server + fake 蓝鲸网关 MCP-proxy，跑完整北向 happy path 与南向 happy path（基于 `cmd/api-server/test/` 目录的现有测试框架，如无则新建 `cmd/api-server/test/mcp/`）
- [x] 11.3 回归测试：原 proxy 链路（`/api/v1/cloud/cvms/list` 等）端到端冒烟，确保 0 个失败 / 0 个延迟劣化
- [x] 11.4 AGUI 回归：web-server → agent-server AGUI 端到端冒烟，确保链路与时延无变化
- [x] 11.5 性能验证：北向 `tools/call(send_message)` 在 100 并发下 P99 延迟 < 3s（不含 LLM 推理）；南向 `tools/list` P99 < 100ms

## 12. 部署与发布

- [x] 12.1 部署文档：在 `docs/overview/` 新增章节 `mcp_a2a_bridge.md` 或更新 `architecture.md`，说明北向 / 南向 / A2A 透传三条链路与启用步骤
- [x] 12.2 蓝鲸网关侧配置：在网关声明 `bk-hcm-devhk-tcloud-ziyan-cvm`（与其它 mcp_server_name）后端指向 api-server `/api/v1/mcp/servers/{name}/mcp/`；并**显式 Path Strip** `/api/v1/mcp/internal/` 防止外部访问
- [x] 12.3 K8s 部署：在 api-server `Deployment` 加 `NetworkPolicy`（如方案 A），允许 `agent-server` Pod 访问 `/api/v1/mcp/internal/`
- [x] 12.4 发布灰度：先在 hk 联调环境开启 `mcp.ingress.enable: true` + `a2aPassthrough.enable: true`，南向 MCP 暂时**不启用**；OpenClaw 联调通过后再放开南向
- [x] 12.5 监控告警：配置 Grafana 看板（`bk_hcm_api_server_mcp_tools_call_total/_duration_seconds` 等指标）+ 关键指标告警。**2026-06-09 变更**：`bk_hcm_api_server_internal_mcp_schema_stale` 指标随网关同步逻辑下线一并移除

## 13. TAPD / iWiki 闭环

- [x] 13.1 在 TAPD 单 134907464、134907568、134907600、134907653 评论中关联本 OpenSpec 变更 ID 与 PR 链接
- [x] 13.2 iWiki "方案3" 文档（https://iwiki.xxxx.com/p/4021231666）追加"实现进度"章节，链接本变更
- [x] 13.3 在 PR 描述中显式声明三条不变量已校验：① api-server proxy 链路无修改；② web-server → agent-server AGUI 链路无影响；③ agent-server A2A 现有实现不需要改动
- [x] 13.4 完成 archive：`openspec archive api-server-mcp-a2a-bridge`，将 6 个 capability 合入主线 `openspec/specs/`
