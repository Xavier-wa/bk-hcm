## ADDED Requirements

### Requirement: A2A JSON-RPC 端点反向代理

api-server SHALL 在外层 `http.ServeMux` 上前缀挂载 `/api/v1/agent/a2a` 路径段，将所有 HTTP 方法（POST/GET）的请求通过 `net/http/httputil.ReverseProxy` 转发到 `agent-server` 的 `/api/v1/agent/a2a`。

反向代理 SHALL 启用 `FlushInterval: -1` 保证 SSE 实时刷写（不缓冲）；MUST NOT 在转发前预读 / 修改 request body；MUST NOT 设置 response buffering。

后端 agent-server 实例 SHALL 通过 etcd 服务发现 `discovery.NewAPIDiscovery(cc.AgentServerName, dis)` 获取（与 `cmd/web-server/service/proxy.go` 同源），由反向代理 `Director` 在每次转发时调用 `APIDiscovery.GetServers()` 选取实例，不在 yaml 中手填 URL。

api-server **MUST NOT** 在 yaml 中要求手填 `a2aPassthrough.agentServerURL` 字段（已废弃）。

#### Scenario: A2A 流式调用透传不丢帧

- **WHEN** 原生 A2A 客户端发送 `POST /api/v1/agent/a2a` with `Accept: text/event-stream`
- **THEN** api-server SHALL 通过反向代理转发到 agent-server
- **AND** SSE 事件 SHALL 实时推送给客户端（每个事件延迟 < 100ms）
- **AND** SHALL 不被 buffer 阻塞

#### Scenario: agent-server 实例由 etcd 服务发现选取

- **WHEN** 反向代理 Director 被调用
- **THEN** SHALL 调用 `APIDiscovery.GetServers()` 获取 agent-server 实例 URL 列表
- **AND** SHALL 按内置轮询策略选取一个实例作为转发目标

#### Scenario: 不修改 request / response body

- **WHEN** A2A 请求 / 响应通过反向代理
- **THEN** request body 与 response body 字节流 SHALL 与 agent-server 原始行为完全一致（不重写、不压缩、不解压、不修改 JSON 内容）

### Requirement: AgentCard 端点透传

api-server SHALL 在外层 `http.ServeMux` 上注册路径 `/api/v1/agent/.well-known/`（前缀匹配），将该前缀下所有路径（如 `/api/v1/agent/.well-known/agent-card.json`）通过反向代理转发到 agent-server 同路径。

agent-server 实例 SHALL 通过 etcd 服务发现获取（与 A2A JSON-RPC 端点反代同源），共享同一 `httputil.ReverseProxy` 实例与 Director。

#### Scenario: AgentCard 路径透传

- **WHEN** 外部 A2A 客户端发送 `GET /api/v1/agent/.well-known/agent-card.json`
- **THEN** api-server SHALL 通过反向代理转发到 `agent-server` 同路径
- **AND** 响应 body SHALL 与 agent-server 原始 AgentCard JSON 完全一致

#### Scenario: AgentCard URL 字段反映 api-server 入口

- **WHEN** agent-server 已通过 `agent_server.yaml` 中 `a2a.card.url: https://<bk-apigw-host>/api/v1/agent` 配置外部访问 URL
- **THEN** 透过 api-server 透传出来的 AgentCard 中 `url` 字段 SHALL 指向蓝鲸网关对应路径（不暴露 agent-server 内网地址）

### Requirement: 上游身份解析复用并注入内部 header

A2A 透传 handler SHALL 在入口中间件调用 `pkg/runtime/gwparser.Parse`（与 MCP ingress 同源），并在反向代理请求转发前注入以下 header：

- `X-Bkapi-User-Name`（从 ctx 取 `bk_username`）
- `X-Bkapi-App-Code`
- `X-Bk-Tenant-Id`
- `X-Bkapi-Request-Id`
- `X-Bkhcm-Caller-Source: api-server`（固定字面量）

中间件 MUST NOT 调用 `peekRequest`，避免 body 被预读破坏流式语义。

#### Scenario: 注入 caller source 通过 agent-server 校验

- **WHEN** 透传请求转发到 agent-server
- **AND** agent-server 配置 `a2a.enforceCallerOrigin: true`
- **THEN** 反向代理转发的 HTTP 请求 SHALL 携带 `X-Bkhcm-Caller-Source: api-server`
- **AND** agent-server `mcpCallerOriginMiddleware` SHALL 通过校验

#### Scenario: 非法 JWT 拒绝

- **WHEN** 透传请求未携带合法 `X-Bkapi-JWT`
- **THEN** api-server SHALL 返回 `HTTP 403 Forbidden`
- **AND** SHALL 不向 agent-server 转发请求

### Requirement: 不影响 web-server → agent-server AGUI 通信

A2A 透传 SHALL 不修改 agent-server 端任何路由（agent-server 仍接受 `web-server` 经内部网络直连的 `/api/v1/agent/agui/*` 与 `/api/v1/agent/a2a` 请求）。

web-server → agent-server 的 AGUI 通信链路 MUST 在本变更后行为完全不变。

#### Scenario: AGUI 通信路径不受影响

- **WHEN** web-server 通过现有 etcd 服务发现直连 agent-server 的 AGUI 接口
- **THEN** 该链路 SHALL 不经过 api-server
- **AND** 链路行为 SHALL 与本变更前完全一致（QPS / 延迟 / 错误率无显著变化）

#### Scenario: agent-server 路由零改动

- **WHEN** 本变更 PR 提交时
- **THEN** `cmd/agent-server/service/service.go` 与 `cmd/agent-server/service/a2a/*.go` SHALL 不被本变更修改

### Requirement: 启用开关与默认值

api-server SHALL 在 `ApiServerSetting` 中新增 `A2APassthrough A2APassthroughSetting` 字段，含 `Enable bool` 子项，**默认值为 `false`**。

当 `a2aPassthrough.enable: false` 时，api-server SHALL 不在外层 mux 上注册 `/api/v1/agent/a2a` 与 `/api/v1/agent/.well-known/` 路径。

`a2aPassthrough` 启用后 SHALL 不再要求 yaml 中手填 agent-server URL（由 etcd 服务发现兜底）。

#### Scenario: 默认关闭对存量部署零影响

- **WHEN** 存量 `api_server.yaml` 不含 `a2aPassthrough` 段
- **THEN** api-server SHALL 不监听 `/api/v1/agent/*` 路径
- **AND** 该路径请求 SHALL 落到 catch-all proxy（行为与本变更前一致）

#### Scenario: 启用开关生效

- **WHEN** `a2aPassthrough.enable: true` 且 agent-server 已在 etcd 中注册可用实例
- **THEN** `GET /api/v1/agent/.well-known/agent-card.json` SHALL 返回合法 AgentCard JSON
