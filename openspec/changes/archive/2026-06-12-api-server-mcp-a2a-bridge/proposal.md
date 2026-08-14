## Why

iWiki 方案 2（[4021231666](https://iwiki.xxxx.com/p/4021231666)）的整体目标是让 OpenClaw 等外部 Agent 经蓝鲸网关以标准 MCP streamable HTTP 直连 HCM 智能助手并支持连续多轮对话，同时让 agent-server 的 LLM 在调用 HCM 工具时走「内网直连 api-server 内置 HCM MCP」而非绕回蓝鲸普通网关。

agent-server 侧的 A2A 协议入口与 `type: "internal"` MCP toolset 已经随归档变更 `2026-06-04-agent-server-a2a-protocol`（对应 TAPD 134021038）落地；**api-server 侧目前仍是纯 proxy，没有任何 MCP 协议处理能力**，因此对应的 4 个 TAPD 子需求（134907464 / 134907568 / 134907600 / 134907653）都阻塞，OpenClaw 接入也缺乏权威 SKILL 文档。本变更补齐 api-server 改造，让方案 2 端到端可用。

## What Changes

### 北向（OpenClaw → HCM）：api-server MCP ingress
- **新增** MCP streamable HTTP 入口：`POST /api/v1/mcp/servers/{mcp_server_name}/mcp/` 与 `GET /api/v1/mcp/servers/{mcp_server_name}/mcp/`，与现有 `/api/v1/{cloud,woa,account}` proxy 路径互不相交。
- **支持多 `mcp_server_name`**：HCM 在蓝鲸网关已注册多个 MCP server name（如 `bk-hcm-devhk-tcloud-ziyan-cvm` / `bk-hcm-devhk-tcloud-cvm-operate` / `bk-hcm-finops` 等），OpenClaw 可同时接入任意一个或多个。所有 `mcp_server_name` 在 api-server 上**共用同一份处理逻辑**（解析 JWT → 注入 caller header → MCP↔A2A 转换 → 调 agent-server），`tools/list` 统一返回 `[send_message]`；`mcp_server_name` 记入日志 / metrics 标签，便于网关侧审计与按 name 限流。
- **新增** MCP JSON-RPC 2.0 编解码层：覆盖 `initialize` / `notifications/initialized` / `tools/list` / `tools/call(send_message)` / `ping` / `notifications/cancelled`，错误按 `-32600 / -32601 / -32602 / -32603` 返回。
- **新增** MCP↔A2A 双向桥接：`tools/call(send_message)` → A2A `message/stream`；A2A SSE 4 类事件流 → MCP `notifications/progress` + 最终 `CallToolResult`，包含：
  - `taskId / contextId / artifactId` 三级映射；MCP 侧 `progressToken` ↔ A2A `taskId` 双向表；
  - 文本 artifact 按 `artifactId` 流式累积，`lastChunk=true` 才纳入最终 `content`；
  - `metadata.thought=true` 的 TextPart **只通过 progress 透出，不写入最终结果**；
  - `function_call` / `function_response` DataPart 通过 progress 通知透出，不写入最终结果；
  - `FilePart` 累积到最终 `content`（`ImageContent` / `AudioContent` / `EmbeddedResource`）；
  - 无 `progressToken` 时关闭所有 progress 通知，全量缓冲 → 一次性返回 `CallToolResult`；
  - 客户端断连 → 自动 `tasks/cancel(taskId)`；硬上限 90s + keepalive。
- **新增** 上游鉴权解析：由蓝鲸网关签发的 `X-Bkapi-JWT` 经现有 `gwparser` 链路解析为 `bk_username` / `tenant_id` / `app_code`，**复用** `cmd/api-server/service/filter.go` 的 `gwparser.Parse` 能力（仅复用解析，不复用 `peekRequest` 缓存）。
- **新增** 内部 header 注入：转发 agent-server 时设置 `X-Bkapi-User-Name` / `X-Bkapi-App-Code` / `X-Bk-Tenant-Id` / `X-Bkapi-Request-Id` / `X-Bkhcm-Caller-Source=api-server`（与 agent-server `mcpCallerOriginMiddleware` 期望对齐）。

### 北向（A2A 原生客户端 → HCM）：api-server A2A 透传
- **新增** A2A 协议透传路径，供 `trpc-a2a-go` / `a2a-python` 等 A2A 原生客户端经蓝鲸网关接入 HCM 智能助手：
  - `POST /api/v1/agent/a2a` — A2A JSON-RPC 透传到 agent-server 同名端点；
  - `GET /api/v1/agent/.well-known/agent-card.json` — A2A v0.2.2 AgentCard 透传；
  - `GET /api/v1/agent/.well-known/agent.json` — A2A 0.1.x 兼容 AgentCard 透传。
- **实现**：使用 `net/http/httputil.ReverseProxy`（对 SSE 流式响应友好），在 Director 中：解析蓝鲸网关 JWT 拿到 `bk_username`/`tenant_id`/`app_code`/`rid`、注入下游 header（`X-Bkapi-User-Name`/`X-Bkapi-App-Code`/`X-Bk-Tenant-Id`/`X-Bkapi-Request-Id`/**`X-Bkhcm-Caller-Source: api-server`**）、通过 `serviced.Discover` 解析 agent-server 实例。
- **不影响 AGUI**：api-server 当前未代理 `/api/v1/agent/*` 路径（proxy 仅识别 cloud/woa/account），本次只新增 `a2a` 与 `.well-known/*` 两段，**`/api/v1/agent/{agui,cancel,history}` 在 api-server 上仍然 404**（web-server 直接调 agent-server 不经过 api-server）。

### 南向（agent-server LLM → HCM 工具）：api-server 内置 HCM MCP Server
- **新增** 仅内网可达的 MCP Server：`POST /api/v1/mcp/internal/hcm/mcp/`，使用 `trpc-mcp-go` 暴露 HCM 内部工具集（云资源查询、变更辅助等）。
- **工具集数据源**：启动时从本地 OpenAPI 3.0 yaml 文件（默认 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm_internal_mcp.yaml`，从蓝鲸 API 网关「资源配置列表」导出）一次性加载工具集，**不再**调用蓝鲸网关 MCP-proxy `tools/list` 接口同步、**不再**做周期同步或本地缓存。OpenAPI yaml 是接口字段、入参、出参、字段描述的**唯一数据源**——开发者在网关后台改资源配置后重新导出即可同步到 api-server 的 MCP tool 列表。
- **行为约定**：**不**校验网关 JWT；信任 `X-Bkapi-User-Name`（来自调用方注入的 `bk_username`），并将其透传到下游 cloud/woa/account-server 用于资源过滤。
- **白名单约束**：拒绝外网/非 agent-server 来源调用（基于 `X-Bkhcm-Caller-Source` 或网段配置）；日志禁打印任何 token / JWT 明文。
- **工具暴露策略**（第一阶段）：默认暴露 OpenAPI yaml 中**全部**有合法 `operationId` 的接口；运维可通过 `MCP.Internal.IncludeOperationIDs` 白名单（精确匹配 + glob 模式）裁剪暴露面。`InternalOnlyTools` 保留为「OpenAPI yaml 之外的本地工具逃生口」，数量预期控制在 5 个以内。

### 文档与接入约束
- **新增** `.cursor/skills/openclaw-mcp-access/SKILL.md`：OpenClaw 接入约束 + 蓝鲸网关 URL + 必填 header + body 中 `contextId` 复用规则 + 流式 `Accept: text/event-stream` + 真实的请求/响应示例（含 SSE event 序列）+ 多轮连续对话示例 + 失败/取消处理。

### 不变性护栏（必须遵守）
- ✅ **proxy 行为零修改**：`cmd/api-server/service/proxy.go`、`cmd/api-server/service/filter.go` 不改一行；所有 `/api/v1/{cloud,woa,account}/...` 请求仍走原有 `restful.WebService → restFilter → proxy.Do`。
- ✅ **AGUI 链路零修改**：web-server → agent-server 的 `/api/v1/agent/{agui,cancel,history}` 完全不动；`bk_ticket` 流程不动；agent-server 进程不动。
- ✅ **agent-server A2A 入口零修改**：复用归档变更已落地的 `/api/v1/agent/a2a`、AgentCard、`mcpCallerOriginMiddleware`、`bkapiContextMiddleware`、`readinessMiddleware`。
- ✅ **现有 cc 配置兼容**：所有新增 cc 字段默认关闭（`api-server.mcp.enable: false`、`api-server.mcp.internal.enable: false`），存量配置无需修改即可启动。

## Capabilities

### New Capabilities

- `api-server-mcp-ingress`：api-server 北向 MCP streamable HTTP 入口路由与 SSE 支持；与现有 proxy / restFilter 完全隔离；上游鉴权（蓝鲸网关 JWT 解析）。
- `api-server-mcp-protocol-codec`：MCP JSON-RPC 2.0 编解码层；6 类 method 的本地处理与路由；标准错误码；progressToken / requestId 生命周期管理。
- `api-server-mcp-to-a2a-bridge`：MCP↔A2A 协议桥接；A2A 4 类 SSE 事件 / 9 类 Part 到 MCP `notifications/progress` + `CallToolResult` 的双向映射；contextId / taskId / artifactId 三级映射；artifact 流式聚合；思维链旁路；客户端断连 / 取消传播。
- `api-server-a2a-passthrough`：api-server A2A 协议透传 + AgentCard 反代；统一注入鉴权与 caller header；不影响 AGUI 链路。
- `api-server-internal-hcm-mcp`：api-server 南向内置 HCM MCP Server；仅内网可达（mux 上强制校验 `X-Bkhcm-Caller-Source` + `allowedSourceIPs` 网段白名单）；不校验网关 JWT，基于 `bk_username` 信任；**工具集从本地 OpenAPI 3.0 yaml 单次加载**（蓝鲸网关「资源配置列表」导出），支持 `internalOnlyTools` 注入网关 yaml 之外的本地专用工具，支持 `IncludeOperationIDs` 白名单（精确 + glob）裁剪暴露面；调用时**不走网关**，由 api-server 按 yaml 中的 backend 元信息直接转发到 cloud/woa/account-server。
- `openclaw-mcp-access-skill`：OpenClaw 接入 SKILL（`.cursor/skills/openclaw-mcp-access/SKILL.md`），约束调用参数（网关 URL、header、body `contextId` 复用、`Accept: text/event-stream`），并给出请求与返回值示例（含 SSE 事件序列、多轮连续对话）。

### Modified Capabilities

> 当前 `openspec/specs/` 下与 api-server 直接相关的能力（`agent-a2a-protocol` / `agent-mcp-internal-toolset`）已在 agent-server 侧落地，本次**只在 api-server 侧新增能力**，**不修改**现有 spec 的 Requirements。

## Impact

### 代码改动（仅 api-server 一个进程 + 1 个 cc 配置 + 1 份 SKILL）
- `cmd/api-server/service/mcp/ingress/`（**新增包**）：北向 MCP streamable HTTP handler、JSON-RPC 编解码（复用 `trpc-mcp-go.Server`）、上游 JWT 解析与内部 header 注入、A2A 后端客户端（复用 `trpc-a2a-go.A2AClient`）。
- `cmd/api-server/service/mcp/bridge/`（**新增包**）：MCP↔A2A 双向事件桥接（含 progressToken/taskId 映射、artifact 聚合、思维链旁路、取消传播）。
- `cmd/api-server/service/mcp/internal/`（**新增包**）：南向内置 HCM MCP Server 与工具集装配；包含 OpenAPI yaml 加载子模块（启动时一次性解析本地 OpenAPI 3.0 yaml，`operationId → tool name`，`requestBody/parameters → InputSchema`，`responses.example` 内嵌到 description 末尾，白名单/合并 `internalOnlyTools`）；调用路由表与 `tools/call` 转发；调用下游通过 api-server 自身 proxy 链路（cloud/woa/account-server）；中间件强制 `X-Bkhcm-Caller-Source: agent-server` + 网段白名单。**不**依赖蓝鲸网关 MCP-proxy，**不**做周期同步、**不**写本地缓存。
- `cmd/api-server/service/a2apass/`（**新增包**）：A2A 透传与 AgentCard 反代；基于 `httputil.ReverseProxy`；Director 中完成 JWT 解析 + header 注入 + agent-server 服务发现。
- `cmd/api-server/service/service.go`：在 `ListenAndServeRest()` 中，外层 `http.ServeMux` 按下列顺序注册：① `/api/v1/mcp/servers/`（北向 MCP，前缀挂载，handler 内解析 `{mcp_server_name}`）；② `/api/v1/mcp/internal/hcm/mcp/`（南向 MCP）；③ `/api/v1/agent/a2a` 与 `/api/v1/agent/.well-known/`（A2A 透传）；④ `root.HandleFunc("/", s.proxy.apiSet().ServeHTTP)`（catch-all 原 proxy）。所有新增路径都不进入 `restFilter`。
- `cmd/api-server/app/app.go`：新增 agent-server 的服务发现订阅（`discOpt.Services` 追加 `cc.AgentServerName`），用于 MCP ingress 转发；不影响现有 cloud/woa/account 发现。
- `pkg/cc/service.go`：`ApiServerSetting` 新增 `MCP MCPServerSetting`，含 `Enable` / `Ingress` / `Internal` 子段；默认关闭。
- `pkg/criteria/constant/`：复用已有 `MCPCallerSourceHeader` / `UserKey` / `RidKey`；如需新增「内部 MCP path 前缀」、「聚合工具名 `send_message`」常量，统一放入 `aiagent.go` 或新建 `mcp.go`。
- `cmd/api-server/etc/api_server.yaml`：追加 `mcp:` 配置段示例（默认 `enable: false`，含 `ingress` / `internal` 注释样例）。
- `.cursor/skills/openclaw-mcp-access/SKILL.md`：新增对外接入 SKILL（含真实请求/返回值示例）。

### 依赖
- 已存在 `trpc.group/trpc-go/trpc-mcp-go`（agent-server 引用）与 `trpc.group/trpc-go/trpc-a2a-go v0.2.5`（agent-server 直接依赖、本次提升到 api-server 也直接依赖）。
- 本次将 `trpc-mcp-go` 从 `v0.0.14` 升级到 `v0.0.16`（SSE close panic / context metadata 等 fix）；仅在 api-server 的 import 中新增对这两个包的直接使用。

### 服务发现与配置
- api-server 需要新增 `cc.AgentServerName` 到服务发现，访问 agent-server `/api/v1/agent/a2a`。
- 内置 HCM MCP 调用下游 cloud/woa/account-server 时继续使用既有 `discovery.APIDiscovery`，无新增。

### API 变化
- **新增**对外路径（经蓝鲸网关）：
  - `POST /api/v1/mcp/servers/{mcp_server_name}/mcp/`（北向 MCP ingress，`{mcp_server_name}` 为通配占位，接受蓝鲸网关已注册的任意 HCM MCP server name）
  - `GET  /api/v1/mcp/servers/{mcp_server_name}/mcp/`
  - `POST /api/v1/agent/a2a`（A2A 协议透传，含 SSE 响应）
  - `GET  /api/v1/agent/.well-known/agent-card.json`（A2A v0.2.2 AgentCard）
  - `GET  /api/v1/agent/.well-known/agent.json`（A2A 0.1.x 兼容 AgentCard）
- **新增**内网路径（不对外暴露）：
  - `POST /api/v1/mcp/internal/hcm/mcp/`（南向内置 HCM MCP，复用 8080，靠 caller header + 网段白名单隔离）
- **不变**：
  - `/api/v1/{cloud,woa,account}/...`（proxy 路径，零修改）；
  - `/api/v1/agent/{agui,cancel,history,sessions/*,memory,skill,prompt,readiness}`（这些路径在 api-server 上**仍然不存在**；web-server 直接调 agent-server，AGUI 链路不变）；
  - `/healthz`、`/alivez`。

### 运行时行为
- 默认 `mcp.enable: false`：所有 MCP 路径未注册，行为与当前 api-server 完全一致；
- 启用后，MCP 路径与 proxy 路径在外层 mux 上**完全独立**，互不影响（详见 design.md 中的 mux 注册顺序与冲突分析）。

### 兼容性
- 对存量 OpenClaw 接入脚本 `remote-agent-http-sse-adapter.py`：**无影响**（其继续走 agent-server 的旧入口），但新接入方案落地后建议切换；本次不删除该脚本。
- 对 web-server / cloud-server / woa-server / account-server / data-service / hc-service：**零代码改动**。

### 测试
- 单元：JSON-RPC 编解码、A2A 事件→MCP 消息映射、progressToken↔taskId 映射、artifact 聚合、取消传播。
- 集成：本地启动 api-server + agent-server，curl 模拟 OpenClaw 经 `/api/v1/mcp/servers/.../mcp/` 发起 `tools/call(send_message)`，校验 SSE progress + 最终 CallToolResult；同 `contextId` 多轮连续对话；客户端断连触发 `tasks/cancel`。
- 回归：现有 `/api/v1/{cloud,woa,account}` 路径全量回归（接口列表抽样）；AGUI 链路端到端冒烟（`POST /api/v1/agent/agui` + `/cancel` + `/history`）。

### 关联 TAPD
- 134021038 — aiagent A2A 协议实现（agent-server，**已完成**，归档变更 `2026-06-04-agent-server-a2a-protocol`）。
- 134907464 — 整理 A2A 事件流与 MCP 的映射关系（**本变更覆盖**，落在 `api-server-mcp-to-a2a-bridge` capability）。
- 134907568 — APIServer 外部 MCP ingress route 处理协议转换（**本变更覆盖**，`api-server-mcp-ingress`）。
- 134907600 — APIServer 实现 MCP 协议编解码层（**本变更覆盖**，`api-server-mcp-protocol-codec`）。
- 134907653 — APIServer 实现 MCP 与 A2A 请求与响应的事件转换（**本变更覆盖**，`api-server-mcp-to-a2a-bridge`）。
- 134021169 — aiagent 新增企微 channel - 新增 api-server 访问入口（父需求，方案 2 整体目标）。
