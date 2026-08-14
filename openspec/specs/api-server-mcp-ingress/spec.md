# api-server-mcp-ingress Specification

## Purpose
TBD - created by archiving change api-server-mcp-a2a-bridge. Update Purpose after archive.
## Requirements
### Requirement: MCP streamable HTTP 入口路径与方法

api-server SHALL 在外层 `http.ServeMux` 上前缀挂载 `/api/v1/mcp/servers/` 路径段，作为北向 MCP streamable HTTP 入口，接受任意 `{mcp_server_name}` 路径变量（如 `bk-hcm-devhk-tcloud-ziyan-cvm`、`bk-hcm-devhk-tcloud-cvm-operate` 等）。

该入口 SHALL 同时支持 `POST` 与 `GET` 方法（streamable HTTP 协议要求），并 MUST 不进入 `cmd/api-server/service/proxy.go` 的 `restful.WebService` 路由（即 MUST 跳过 `cmd/api-server/service/filter.go::peekRequest`，避免对 SSE body 的预读破坏流式语义）。

所有 `{mcp_server_name}` 值 SHALL 路由到同一个 `trpc-mcp-go.Server` 实例处理，但 `mcp_server_name` MUST 写入 ctx，供日志 / metrics 标签使用。

#### Scenario: POST 请求路径匹配

- **WHEN** OpenClaw 经蓝鲸网关发送 `POST /api/v1/mcp/servers/bk-hcm-devhk-tcloud-ziyan-cvm/mcp/` 请求
- **THEN** api-server SHALL 将该请求路由到北向 MCP ingress handler
- **AND** SHALL 不经过 `restful.WebService` 与 `restFilter`
- **AND** ctx 中 SHALL 携带 `mcp_server_name=bk-hcm-devhk-tcloud-ziyan-cvm`

#### Scenario: 任意 mcp_server_name 统一处理

- **WHEN** OpenClaw 同时配置两个 MCP url（`.../bk-hcm-devhk-tcloud-ziyan-cvm/mcp/` 与 `.../bk-hcm-finops/mcp/`）并分别发起请求
- **THEN** 两个请求 SHALL 都被路由到同一个 `trpc-mcp-go.Server` 实例
- **AND** 两个请求的 `tools/list` 返回结果 SHALL 完全相同（均为 `[send_message]`）
- **AND** 日志中 SHALL 各自标记 `mcp_server_name` 标签便于审计

#### Scenario: GET 请求支持 streamable HTTP

- **WHEN** MCP client 发起 `GET /api/v1/mcp/servers/{name}/mcp/`（建立 SSE 长连接以接收 server-pushed notifications）
- **THEN** api-server SHALL 返回 `Content-Type: text/event-stream` 并保持连接
- **AND** SHALL 不被 `proxy.go::filter.go::peekRequest` 缓冲破坏

### Requirement: 与现有 proxy 链路完全隔离

api-server SHALL 在外层 mux 上按如下顺序注册路径：① `/api/v1/mcp/servers/`（北向 MCP）；② `/api/v1/mcp/internal/hcm/mcp/`（南向 MCP）；③ `/api/v1/agent/a2a` 与 `/api/v1/agent/.well-known/`（A2A 透传）；④ `/`（catch-all 走原 `proxy.apiSet()`）。

`/api/v1/{cloud,woa,account}/...` 路径段 MUST NOT 被 MCP ingress 截胡，行为与本变更前完全一致。

`cmd/api-server/service/proxy.go` 与 `cmd/api-server/service/filter.go` MUST NOT 在本变更中被修改。

#### Scenario: cloud-server proxy 路径不被截胡

- **WHEN** 外部客户端发送 `POST /api/v1/cloud/cvms/list`
- **THEN** api-server SHALL 将请求路由到 `proxy.apiSet().ServeHTTP`（与本变更前一致）
- **AND** SHALL 经过原有 `restFilter` 链路（含 `gwparser.Parse` + `peekRequest` 日志缓存）

#### Scenario: proxy.go 与 filter.go 零修改

- **WHEN** 本变更 PR 提交时
- **THEN** `git diff` SHALL 不包含 `cmd/api-server/service/proxy.go` 或 `cmd/api-server/service/filter.go` 的任何修改

### Requirement: 上游身份解析复用 gwparser

api-server MCP ingress SHALL 在入口中间件中调用 `pkg/runtime/gwparser.Parse(r.Context(), r.Header)`（与 proxy 链路同源），将解析出来的 `bk_username`、`tenant_id`、`app_code`、`rid` 写入请求 ctx，供下游 tool handler 与 A2A 调用使用。

中间件 MUST NOT 调用 `peekRequest` 或任何会物化 body 的逻辑（保留 streamable HTTP 的 body 流式读取能力）。

当 `gwparser.Parse` 返回错误（如 JWT 校验失败）时，api-server SHALL 返回 `HTTP 403 Forbidden` 并附带 `errf` 错误响应体。

#### Scenario: 合法 JWT 通过身份解析

- **WHEN** 经蓝鲸网关的请求携带合法 `X-Bkapi-JWT` header
- **THEN** ingress 中间件 SHALL 通过 `gwparser.Parse` 解析得到 `bk_username` 等字段并写入 ctx
- **AND** 后续 tool handler SHALL 能从 ctx 中读取 `bk_username`

#### Scenario: 非法 JWT 拒绝

- **WHEN** 请求未携带 `X-Bkapi-JWT` 或携带过期 / 伪造 JWT
- **THEN** ingress 中间件 SHALL 返回 `HTTP 403 Forbidden`
- **AND** SHALL 不调用任何下游 tool handler

#### Scenario: 不调用 peekRequest

- **WHEN** ingress 中间件处理 POST 请求
- **THEN** 中间件 SHALL 直接传递 `r.Body` 给 `trpc-mcp-go.Server`
- **AND** SHALL 不调用 `cmd/api-server/service/filter.go` 中的 `peekRequest` 函数

### Requirement: 注入下游 A2A 调用的内部 header

当 ingress 内 tool handler 转发请求到 agent-server A2A 时，handler SHALL 注入下列 HTTP header：

- `X-Bkapi-User-Name`（从 ctx 取 `bk_username`）
- `X-Bkapi-App-Code`（从 ctx 取 `app_code`）
- `X-Bk-Tenant-Id`（从 ctx 取 `tenant_id`）
- `X-Bkapi-Request-Id`（从 ctx 取 `rid`，无则生成 UUID）
- `X-Bkhcm-Caller-Source: api-server`（**始终**注入，固定字面量）

注入完成后 handler SHALL 通过 `trpc-a2a-go.A2AClient.StreamMessage` 调用 agent-server `/api/v1/agent/a2a`。

#### Scenario: 所有内部 header 完整注入

- **WHEN** OpenClaw 经网关发送合法请求触发 `tools/call(send_message)`
- **THEN** api-server 转发到 agent-server 的 HTTP 请求 SHALL 同时携带上述 5 个 header
- **AND** `X-Bkhcm-Caller-Source` 的值 SHALL 严格等于 `api-server`

#### Scenario: agent-server 来源校验通过

- **WHEN** agent-server 配置 `a2a.enforceCallerOrigin: true`
- **AND** api-server 向其转发 A2A 请求
- **THEN** agent-server 的 `mcpCallerOriginMiddleware` SHALL 通过来源校验（不返回 403）

### Requirement: 启用开关与默认值

api-server SHALL 在 `pkg/cc/service.go::ApiServerSetting` 中新增 `MCP MCPServerSetting` 字段，含 `Ingress.Enable` 子项，**默认值为 `false`**。

当 `mcp.ingress.enable: false` 时，api-server SHALL 不在外层 mux 上注册 `/api/v1/mcp/servers/` 路径，所有 MCP ingress 相关代码路径无效。

#### Scenario: 默认关闭时零影响

- **WHEN** 存量部署的 `api_server.yaml` 不包含 `mcp` 段
- **THEN** api-server 启动后 SHALL 不监听 `/api/v1/mcp/servers/` 路径
- **AND** 该路径请求 SHALL 落到 catch-all proxy（返回 404 或路径无效错误）
- **AND** 所有 `/api/v1/{cloud,woa,account}/...` 请求 SHALL 行为完全不变

#### Scenario: 启用开关生效

- **WHEN** `api_server.yaml` 配置 `mcp.ingress.enable: true`
- **AND** api-server 重启后
- **THEN** `/api/v1/mcp/servers/{name}/mcp/` SHALL 返回有效 MCP 响应（非 404）

