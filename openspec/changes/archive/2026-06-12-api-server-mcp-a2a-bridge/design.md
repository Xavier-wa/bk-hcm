## Context

### 当前状态
- `agent-server` 的 A2A 协议入口（`POST /api/v1/agent/a2a` + `.well-known/agent-card.json`）已在归档变更 `2026-06-04-agent-server-a2a-protocol` 中落地。中间件链：`mcpCallerOriginMiddleware → bkapiContextMiddleware → readinessMiddleware → A2A handler`，要求上游传 `X-Bkhcm-Caller-Source: api-server` 与 `X-Bkapi-User-Name`。
- `agent-server` 的 `type: "internal"` MCP toolset 类型已经实现（`cmd/agent-server/logics/tool/tool.go`），只注入 `X-Bkapi-User-Name`，不注入任何 token。
- `api-server` 当前形态：`cmd/api-server/service/proxy.go` 是纯反向代理；`filter.go::peekRequest` 会整体读 body 并 `ReplaceAllString` —— **对 SSE 不友好**，因此 MCP 流式入口绝对不能进入 `restFilter`。
- 项目 go.mod 依赖 `trpc.group/trpc-go/trpc-mcp-go`（本次升级到 **v0.0.16**，agent-server 已用作 MCP **client**）与 `trpc.group/trpc-go/trpc-a2a-go v0.2.5`（直接，agent-server 用作 A2A **server**）。
- `trpc-mcp-go` 实测提供完整的 **MCP Server** 能力：`NewServer / RegisterTool / Server.Handler() http.Handler / WithHTTPContextFunc / WithStatelessMode / WithPostSSEEnabled / WithGetSSEEnabled`；可以直接挂到 api-server 的外层 mux。
- `trpc-a2a-go` 的 `client.A2AClient.StreamMessage` 提供 `message/stream` 的 SSE 解析能力，避免手写 SSE 解析器。

### 约束
- 不修改 `cmd/api-server/service/proxy.go` 与 `cmd/api-server/service/filter.go`（保护 proxy 链路）。
- 不修改 `cmd/agent-server/`（agent-server 已就绪）。
- 不修改 `cmd/web-server/` 与 AGUI 任何相关代码（保护 AGUI 链路）。
- 默认 `mcp.enable: false`，存量部署零影响。

### 利益相关方
- **OpenClaw 接入方**：经蓝鲸网关 streamable HTTP MCP 接入，需要明确 url / header / `contextId` 复用规则。
- **HCM agent-server 维护者**：本变更不改其代码，但需要在 `enforceCallerOrigin: true` 时确保 `X-Bkhcm-Caller-Source: api-server` 被透传。
- **HCM 安全 / 网关运维**：需在蓝鲸网关上为 `/prod/api/v2/mcp-servers/{name}/mcp/` 配置 streamable HTTP 资源，并保证 `enable_streaming: true`。

## Goals / Non-Goals

**Goals:**
- 在 api-server 提供一个**与 proxy 完全隔离**的 MCP streamable HTTP 入口（北向），把 OpenClaw 的 MCP `tools/call(send_message)` 翻译为 agent-server 的 A2A `message/stream`，并把 A2A SSE 流翻译回 MCP `notifications/progress` + `CallToolResult`。
- 在 api-server 提供**仅内网可达**的内置 HCM MCP Server（南向），把 cloud/woa/account 的关键能力暴露为 MCP 工具，供 agent-server LLM 内网直连，避免绕回蓝鲸普通网关。
- 通过 `contextId` 维持多轮连续对话（A2A 原生语义）。
- 提供 OpenClaw 接入 SKILL，约束调用参数与连续对话规则，并给出端到端可复现的请求/返回值示例。
- 三条不变性护栏：proxy 行为零变化、AGUI 链路零变化、agent-server 零变化。

**Non-Goals:**
- 不实现 OAuth 2.1 / PKCE 等用户态 access_token 流程（方案 2 已演进为「网关 JWT → bk_username」内部信任模型）。
- 不在本期实现 `prompts/list` / `resources/list` 等次要 MCP 方法（直接返回空列表）。
- 不实现 A2A `tasks/resubscribe`、`input-required` 等高级交互（保留为后续扩展点）。
- 不重写 api-server 现有 proxy 实现（任何 `/api/v1/{cloud,woa,account}/...` 路径行为完全不动）。
- 不修改蓝鲸网关上的资源配置（由网关运维侧配置，本变更只在 SKILL 中给出网关侧期望路径）。

## Decisions

### D1：MCP 协议编解码层直接复用 `trpc-mcp-go` 的 Server，**不**手写 JSON-RPC

**决定**：北向 MCP ingress 与南向内置 HCM MCP 均用 `trpc-mcp-go.NewServer(...)` 构建。北向只注册一个聚合工具 `send_message`，工具 handler 内部把请求翻译为 A2A 调用；南向注册 HCM 各业务工具。

**理由**：
- TAPD 134907600 要求实现 6 类 method（`initialize` / `notifications/initialized` / `tools/list` / `tools/call` / `ping` / `notifications/cancelled`）与 4 类标准错误码 —— `trpc-mcp-go` 全部内置，且实现已经过单元测试。
- `Server.Handler() http.Handler` 可直接挂到外层 mux，与 trpc-a2a-go 在 agent-server 侧的集成模式一致，团队学习成本低。
- `WithHTTPContextFunc` 提供「HTTP 入站 → ctx」注入点，方便把 bk_username / bk_biz_id 等从 header 拷贝到 ctx，供 tool handler 内构造 A2A 请求时使用。
- 自研 JSON-RPC 引入额外维护成本，没有 ROI。

**备选**：手写 JSON-RPC 编解码层 —— ❌ 拒绝，重复造轮子。

### D2：北向 / 南向使用两个独立的 trpc-mcp-go.Server 实例；北向接受**任意** `mcp_server_name`

**决定**：
- **北向**：`apiserver-mcp-ingress.Server`，路径模式 `/api/v1/mcp/servers/{mcp_server_name}/mcp/`，**接受任意 `mcp_server_name`**（如 `bk-hcm-devhk-tcloud-ziyan-cvm`、`bk-hcm-devhk-tcloud-cvm-operate`、`bk-hcm-finops`），所有 server name **共用同一个 `trpc-mcp-go.Server` 实例**，只注册一个聚合工具 `send_message`。
- **南向**：`apiserver-mcp-internal.Server`，路径 `/api/v1/mcp/internal/hcm/mcp/`，注册 HCM 业务工具集（`list_cvms` / `query_clb` 等）。

**北向多 `mcp_server_name` 的实现细节**：
- mux 层挂载前缀 `/api/v1/mcp/servers/`，handler 内解析路径变量 `{mcp_server_name}`；
- 把解析出的 `mcp_server_name` 写入 ctx（通过 `trpc-mcp-go.WithHTTPContextFunc`），供日志、metrics 标签、未来差异化策略使用；
- 把 path 改写为 `trpc-mcp-go.Server` 在 `WithServerPath` 注册时的固定路径，再调 `Server.Handler().ServeHTTP(w, r)`；
- 第一阶段**不做** server name 白名单（注册哪些由网关侧管控）；不做按 server name 差异化 `system prompt` / 工具子集 —— 留作扩展点（参见 D8 备注）。

**理由**：
- 蓝鲸 API 网关上 HCM 已经注册了多个 MCP server name，OpenClaw 可以同时接入多个；如果 api-server 只支持固定一个 `hcm-agent`，与现状不兼容、迁移困难。
- 所有 `mcp_server_name` 在方案 2 的聚合模式下行为一致（都走 `send_message` → A2A），**功能上没有差异**；但保留路径变量为「网关侧差异化限流/审计」与「未来多 Agent 路由」打开扩展点，零额外成本。
- 北向与南向 server 的 **暴露面 / 鉴权策略 / 工具集** 完全不同：
  - 北向暴露给外网（经网关），上游有蓝鲸 JWT；
  - 南向只允许 agent-server 调用，不校验网关 JWT，靠 `X-Bkhcm-Caller-Source` + 内网白名单。
- 两个独立 Server 便于在中间件、stateless 模式、tool 注册策略上各自演进，不会因「为南向加一个工具」而影响北向。

**备选**：单 Server 通过 ToolFilter / Path 区分 —— ❌ 拒绝，鉴权策略难以混在同一中间件链中。

### D3：路径冲突 —— MCP 路径必须在外层 mux 上**早于** proxy catch-all 注册

**决定**：在 `cmd/api-server/service/service.go::ListenAndServeRest()` 中，按下列顺序对外层 `root := http.NewServeMux()` 注册：

```go
// 1) MCP 路径（启用时）—— 使用更长的具体前缀
if mcpCfg.Ingress.Enable {
    root.Handle("/api/v1/mcp/servers/hcm-agent/mcp/", ingressHandlerWithMiddlewares)
}
if mcpCfg.Internal.Enable {
    root.Handle("/api/v1/mcp/internal/hcm/mcp/", internalHandlerWithMiddlewares)
}

// 2) 其余流量 —— 走原有 proxy
root.HandleFunc("/", s.proxy.apiSet().ServeHTTP)
```

Go 1.22+ 的 `http.ServeMux` 按**最长匹配前缀**优先（且 1.22+ 引入的 method+path pattern 也兼容），`/api/v1/mcp/...` 严格长于 `/`，**proxy 路径 `/api/v1/cloud/...` / `/api/v1/woa/...` / `/api/v1/account/...` 都不会被 MCP 前缀截胡**。

**理由**：
- 这是「不修改 proxy.go」前提下最简单的隔离方式，零回归风险。
- 也跳开了 `restFilter`（restFilter 是 `restful.WebService` 内的 filter，仅作用在 proxy 的 ws 上）。
- agent-server 的 mux 已经用同样的模式集成 AGUI / A2A / proxy，统一性好。

**备选**：在 proxy.go 内增加路径白名单 —— ❌ 拒绝，触碰存量代码就有回归风险；且 `restFilter::peekRequest` 不能直接关掉。

### D4：上游网关 JWT 解析复用现有 `gwparser`，但**不复用** `peekRequest`

**决定**：在 MCP ingress 入口的中间件链上调用 `gwparser.Parse(r.Context(), r.Header)`，把解析出来的 `bk_username` / `tenant_id` / `app_code` / `rid` 写入 ctx（用 `WithHTTPContextFunc`），交给 tool handler 使用。**不**调用 `peekRequest`（不缓存 body）。

**理由**：
- `gwparser.Parse` 是 api-server 现成的成熟实现（`pkg/runtime/gwparser`），且被 cloud/woa/account proxy 全链路使用，复用即可享受 JWT 校验、租户解析、`disableJWT` 联调开关等能力。
- `peekRequest` 整读 body 后用 `ReplaceAllString` 压缩日志 —— 对 streamable HTTP 的 POST 请求会强行物化 body，进而破坏 SSE 转发；MCP 路径不进 `restFilter`，自然规避。

**备选**：自己实现 JWT 解析 —— ❌ 拒绝，重复实现且与 proxy 行为不一致。

### D5：MCP↔A2A 双向桥接采用 client→server 流式管道，**禁止整包缓冲**

**决定**：`send_message` 的 tool handler 流程：

```text
1. tool handler 接收 MCP tools/call 请求；
2. 提取 arguments.text、arguments.contextId、arguments.bk_biz_id、arguments.model_name；
3. 缺 contextId → 自动生成 ctx-<uuid>，在最终 CallToolResult.meta 回传；
4. 构造 A2A SendMessageParams：message.parts[text]、metadata.bkBizId、metadata.source="mcp"；
5. 通过 trpc-a2a-go A2AClient.StreamMessage 发起流式请求，注入：
     X-Bkapi-User-Name / X-Bkapi-App-Code / X-Bk-Tenant-Id / X-Bkapi-Request-Id /
     X-Bkhcm-Caller-Source=api-server
6. 收到的每一个 A2A SSE 事件 → 转换为对应 MCP 输出：
     - task_status_update(submitted/working) → Server.SendNotification("notifications/progress", ...)
     - task_artifact_update(TextPart, thought=false) → 累积到 artifactBuffer[artifactId] + 节流进度
     - task_artifact_update(TextPart, thought=true) → 仅 progress 透出，不入最终 content
     - task_artifact_update(DataPart, function_call/response) → 仅 progress 透出
     - task_artifact_update(FilePart) → 累积到最终 content
     - task_status_update(state=completed, final=true) → 触发 CallToolResult 组装
     - task_status_update(state=failed/canceled/rejected, final=true) → CallToolResult{isError:true,...}
7. 客户端断连（ctx Done）→ 调 A2AClient.CancelTask(taskId) 取消 agent 侧任务；
8. 硬上限 90s + 周期 keepalive notification（trpc-mcp-go 内置 SSE 心跳由 responder_sse 提供）。
```

**理由**：
- 严格遵守 TAPD 134907464 给出的事件→消息映射表，技术上等价于该表。
- 关键 trade-off：「文本流」与「图片/文件」走不同的最终化路径 —— 文本按 `artifactId` 流式累积，`lastChunk=true` 触发结果块；图片/文件直接缓冲到最终 `content`。
- 「无 progressToken 时不发 progress」由 MCP 协议层语义保证（`trpc-mcp-go` 的 SendNotification 在 session 无对应 progressToken 时是 no-op，进一步保险见 D6）。

**备选**：直接转发 A2A SSE 流给 MCP 客户端 —— ❌ 拒绝，MCP `tools/call` 的语义是「最终一次 CallToolResult + 可选 progress 通知」，不能把 A2A 事件 1:1 透传。

### D6：progressToken → taskId 映射在请求作用域内维护，结束即销毁

**决定**：每个 `tools/call` 请求在 tool handler 内维护一个 `progressToken / taskId / contextId / artifactBuffer` 的请求作用域对象，handler 返回时一并释放，**不**做跨请求持久化（避免内存泄漏 + 跨用户串扰）。

**理由**：
- MCP `notifications/cancelled` 的 `params.requestId` 即 `progressToken`，需要在「同一会话」内查到对应的 `taskId`。
- 一次 `tools/call` 的生命周期一定大于其所有 progress 通知 —— 把映射放在 tool handler 的 closure 里即可，无需全局 map。
- 若未来要支持「跨 `tools/call` 取消上一次任务」，再引入 sessionID → taskId 的 server-scope 映射。

### D7：南向内置 HCM MCP 工具集 = 本地 OpenAPI 3.0 yaml（蓝鲸网关「资源配置列表」导出）

> **2026-06-09 决策变更**：原 D7「自动同步自蓝鲸网关 MCP-proxy `tools/list`」与 D12「网关同步 + 内部扩展混合模式」**作废**。改为「本地 OpenAPI yaml 单一数据源」，原因详见本节及 D12 更新。

**决定**：南向 MCP `tools/list` 返回的工具集由 api-server **启动时一次性**从本地 OpenAPI 3.0 yaml 文件解析得到（默认 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm_internal_mcp.yaml`，由蓝鲸 API 网关「资源配置列表」页面**手动导出**到本仓库）。每个 OpenAPI `path` 节点的 `operationId` 即为 MCP `tool name`，`description` 与 `summary` 拼出 tool description（追加 `responses.200.content.application/json.example` 的 JSON 序列化以弥补 `additionalProperties: true` 时的 schema 缺失），`requestBody.schema` 与 `parameters` 合并为 MCP `InputSchema`，`x-bk-apigateway-resource.backend.method/path` 作为 `tools/call` 的本地后端路由目标（path 模板剥离 `{env.url_path_prefix}` 前缀后命中 api-server 自身的 `/api/v1/{cloud,woa,account}` proxy 链路）。

> 注意：项目根目录的 `remote-agent-http-sse-adapter.py` 是 ACP↔AG-UI 翻译器（见文件 docstring），**它本身不暴露 MCP 工具集**；不在本变更讨论范围。

**理由**：
- **唯一数据源**：OpenAPI yaml 是 HCM 对内接口的权威描述（开发者在蓝鲸网关后台修改资源配置 → 重新导出 → 覆盖 yaml → 自动同步到 api-server MCP tool 列表），避免「网关后台 / OpenAPI / mcp_tools.yaml 三处描述」的双源/三源维护漂移；
- **零外部依赖**：调用蓝鲸网关 MCP-proxy `tools/list` 同步会引入「网关临时不可达 → api-server 启动失败 / 缓存陈旧」等运维复杂度；本地 yaml 走仓库版本控制，自然有审计与回滚能力；
- **example 是意外彩蛋**：实测新导出的 yaml 中有 37 个接口的 `data` 字段是 `additionalProperties: true`，但 `example` 字段包含完整真实结构。把 example 内嵌到 description 让 LLM 仍能拿到字段结构（实测 LLM 对 example 与 schema 的理解效果接近）；
- **CI lint 倒逼接口 owner 补全网关资源配置**：把「响应字段不完整」「path 参数缺声明」等问题反向倒逼到接口 owner 的日常 MR 中，长期提升对外 API 文档质量（一举两得）；
- **TAPD 134907568 范围微调**：原任务表述「与现有蓝鲸网关 MCP 暴露的工具集对齐」改为「与蓝鲸网关资源配置列表对齐」，业务本质一致（两者都是网关上的接口清单），但实现解耦了「调用时机」与「网关依赖」。

**备选**：
- 远程同步蓝鲸网关 MCP-proxy（原 D7） —— ❌ 拒绝：依赖网关可用性、引入缓存陈旧问题、运维复杂度高；
- 独立 `mcp_tools.yaml` 注册表（方案 C） —— ❌ 拒绝：与 `pkg/api` Go 结构体 / 网关资源配置形成双源维护，接口字段一改容易遗忘；
- 基于 Go 结构体 tag 反射生成 schema（方案 B） —— ⚠️ 保留为长期演进：现有 Request 结构体很多用 `map[string]interface{}` 或动态字段，要重构一批结构体才能让 LLM 看懂，工程量大、阶段错配。

### D12：南向工具集 schema 加载与 `tools/call` 本地路由（OpenAPI 驱动）

> **2026-06-09 决策变更**：原 D12「网关同步 + 内部扩展工具配置混合模式」**作废**。改为 OpenAPI yaml 单次加载 + 本地 backend 路由模式。配置项变化：删除 `schemaSync.*` / `toolFilter`，新增 `openapiSpecPath` / `includeOperationIDs`；`internalOnlyTools` 从 `[]string` 升级为结构化条目。

**决定**：南向 `tools/list` 返回内容由两部分合并而成：

```yaml
mcp:
  internal:
    enable: true

    # ── 主体 schema：本地 OpenAPI 3.0 yaml（蓝鲸网关「资源配置列表」导出）──
    openapiSpecPath: "docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm_internal_mcp.yaml"

    # ── 工具暴露白名单（精确匹配 + glob）──
    # 空数组 = 不过滤，暴露 yaml 中全部 operationId
    includeOperationIDs:
      - "list_*"            # 所有 list_* 接口（查询类）
      - "get_*"             # 所有 get_* 接口（详情类）
      - "describe_*"        # 所有 describe_* 接口

    # ── 扩展：仅内部可见、yaml 之外的本地工具（数量预期 ≤ 5）──
    # 用于「LLM 决策需要但不进网关资源配置」的能力，如内部审计聚合查询。
    # 每条都必须显式配置 backend 路由 + InputSchema。
    internalOnlyTools:
      - name: "hcm_internal_query_audit_log"
        description: "查询 HCM 内部审计日志（仅 LLM 内部使用，外部不可见）"
        inputSchema:
          type: object
          properties:
            biz_id: { type: integer, description: "业务 ID" }
            time_range: { type: string, description: "时间范围，如 1h/24h/7d" }
          required: [biz_id]
        backend:
          method: POST
          path: "/api/v1/cloud/audit/list"
```

**加载策略**：
1. **启动**：`OpenAPILoader.Load(openapiSpecPath, includeOperationIDs)` 一次性解析 → 得到 `[]mcpsdk.Tool` + `map[name]ToolBackend`；
2. **合并**：`InternalOnlyTools` 中的工具按 name 与 OpenAPI 加载结果取**并集**，同名时 `InternalOnlyTools` 覆盖（让运维有手段临时覆盖 OpenAPI 中有问题的接口）；
3. **失败行为**：yaml 文件不存在 / 解析失败 / 关键字段缺失 SHALL 阻塞 api-server 启动并返回明确错误（不允许"半启动"）；
4. **运行时**：**不**做周期同步、**不**写本地缓存、**不**启动后台 goroutine；接口字段变更通过「网关后台改配置 → 重新导出 yaml → MR 提交 → 重启 api-server」流程同步（与代码部署节奏对齐）。

**调用路由策略（`tools/call` 后端映射）**：
- **OpenAPI 来源的工具**：使用 yaml 中 `x-bk-apigateway-resource.backend.method/path`（剥离 `{env.url_path_prefix}` 前缀后）作为内部 HTTP 调用目标；
- **InternalOnlyTools 的工具**：使用配置中显式的 `backend.method/path`；
- **路径模板替换**：tool args 中的 `bk_biz_id` 等字段 SHALL 自动替换 path 中的 `{bk_biz_id}` 占位，剩余字段作为 body 传给后端；
- **header 注入**：每次后端调用注入 `X-Bkapi-User-Name`（来自 ctx）、`X-Bkhcm-Caller-Source: api-server`、`X-Bkapi-Request-Id`；**不**注入 `X-Bkapi-JWT` 或 `bk_ticket`（南向是内部信任链路）。

**调用时序**：

```text
agent LLM ─ tools/call(list_cvms, {bk_biz_id: 100, page: {...}}) ─→ api-server 内置 MCP
                                       ├─ 查 tool backend → POST /api/v1/cloud/bizs/100/cvms/list（path 模板替换后）
                                       ├─ 注入 X-Bkapi-User-Name (from ctx)
                                       └─→ api-server 自身 mux（self-call）→ proxy → cloud-server
                                              └─→ 基于 bk_username 资源过滤 + 返回
```

**理由**：
- **维护成本极低**：HCM 接口字段一改，开发者改完 Go 代码 + 网关资源配置之后，重新导出一次 yaml 提交即可，**不需要触动 api-server 任何代码**；
- **零运行时外部依赖**：启动后不再访问蓝鲸网关，网关宕机不影响南向 MCP 调用；
- **完全兼容方案 2 初衷**：方案 2 反对的是「**调用时**绕回网关」（inner-jwt 校验、用户态丢失）；本地 yaml 加载是启动期一次性内存操作，调用仍内网直连 cloud/woa/account-server；
- **`internalOnlyTools` 的设计意图重新定位**：从「与网关同步并列的来源」改为「OpenAPI yaml 之外的逃生口」，仅用于「运维审计 / 内部状态查询 / 调试工具」等不适合进网关资源配置的少量场景；
- **白名单优于黑名单**：原 `ToolFilter` 黑名单（排除某些工具）改为 `IncludeOperationIDs` 白名单（仅暴露某些工具），更符合「最小暴露面」安全原则；空白名单时退化为全暴露行为。

**备选**：
- 远程同步蓝鲸网关 MCP-proxy（原方案） —— ❌ 拒绝，详见 D7 更新；
- 静态在 Go 代码中注册全部工具（手写 mcp.NewTool） —— ❌ 拒绝：每加一个 HCM 接口都要改 api-server 代码，维护成本高；
- 完全跳过加载，直接代理 `tools/call` 给网关 MCP-proxy —— ❌ 拒绝：违背方案 2 初衷（调用时绕回网关，用户态丢失）。

### D11：北向同时暴露 **A2A 透传 + AgentCard**，让外部 A2A 客户端可以直连

**决定**：除了 D1~D2 的 MCP ingress，**api-server 同时暴露 A2A 协议透传与 AgentCard 端点**：

| 路径 | 方法 | 行为 |
|---|---|---|
| `POST /api/v1/agent/a2a` | POST（含 SSE 响应） | 反向代理到 agent-server `/api/v1/agent/a2a`，注入身份与 caller header |
| `GET /api/v1/agent/.well-known/agent-card.json` | GET | 反向代理 agent-server 的 A2A v0.2.2 AgentCard |
| `GET /api/v1/agent/.well-known/agent.json` | GET | 反向代理 agent-server 的 A2A 0.1.x 兼容 AgentCard |

实现要点：
- 使用 `net/http/httputil.NewSingleHostReverseProxy`（默认对 `text/event-stream` 是流式友好的，会随 Flush 推 chunk），**不**复用 `cmd/api-server/service/proxy.go` 的 `io.Copy` 反代（其 body 处理对 SSE 不友好）。
- `Director` 中执行：① 解析蓝鲸网关 JWT 拿到 `bk_username` / `app_code` / `tenant_id` / `rid`；② 注入下游 header：`X-Bkapi-User-Name` / `X-Bkapi-App-Code` / `X-Bk-Tenant-Id` / `X-Bkapi-Request-Id` / **`X-Bkhcm-Caller-Source: api-server`**（与 agent-server `mcpCallerOriginMiddleware` 期望对齐）；③ 通过 `serviced.Discover` 解析到 agent-server 实例地址。
- 该路径段独立于 MCP ingress，对应新增 capability `api-server-a2a-passthrough`。
- 与 AGUI 路径互不影响：当前 api-server 没有 proxy 任何 `/api/v1/agent/*` 路径（proxy 路由仅识别 cloud/woa/account）；新增 `/api/v1/agent/a2a` 与 `/api/v1/agent/.well-known/*` 后，**`/api/v1/agent/agui|cancel|history` 仍然不被 api-server 处理**（web-server 是直接调 agent-server，不经过 api-server），AGUI 链路完全不动。

**理由**：
- A2A 的核心价值之一就是 `.well-known/agent-card.json` 的 Agent 间互发现 —— 不暴露则外部 A2A 原生客户端（非 MCP 路线）无法接入 HCM 智能助手。
- agent-server 已经实现了完整的 A2A 入口与 AgentCard，对外暴露只需要一层「鉴权 + caller header 注入」的反代，工作量低、风险可控。
- 与 MCP ingress 形成互补：
  - **MCP ingress 路径**（`/api/v1/mcp/servers/hcm-agent/mcp/`）—— 给 OpenClaw / Cursor 等 MCP 原生客户端用；
  - **A2A 透传路径**（`/api/v1/agent/a2a` + `/.well-known/*`）—— 给 trpc-a2a-go / a2a-python 等 A2A 原生客户端用。
- 蓝鲸网关侧：网关 A2A 资源指向 api-server 的 `/api/v1/agent/a2a` 与 `/.well-known/*`（路径与 agent-server 一致，但请求落点是 api-server，由 api-server 完成鉴权解析后再反代到 agent-server）。

**备选**：
- 让外部 A2A 客户端**经蓝鲸网关直连 agent-server** —— ❌ 拒绝，会绕过 api-server 的统一入口，破坏「单一对外鉴权点」原则；且 agent-server 的 A2A `mcpCallerOriginMiddleware` 期望 `X-Bkhcm-Caller-Source: api-server`，直连会让该校验形同虚设。
- **只暴露 MCP ingress 不暴露 A2A** —— ❌ 拒绝，违背 A2A 协议生态化的目标，且 AgentCard 是 A2A 客户端发现服务的入口。

### D8：聚合工具名固定为 `send_message`；OpenClaw 看到的 `tools/list` 永远是它一个

**决定**：北向所有 `mcp_server_name` 上的 `tools/list` **统一返回** 一个工具 `send_message`，title 为 `HCM Agent`，输入 schema 四字段：

```json
{
  "type": "object",
  "properties": {
    "text":      {"type": "string",  "description": "用户问题或操作意图"},
    "contextId": {"type": "string",  "description": "会话上下文 ID，连续对话固定复用"},
    "bk_biz_id": {"type": "integer", "description": "业务 ID，可选"},
    "model_name":{"type": "string",  "description": "模型名称，可选"}
  },
  "required": ["text"]
}
```

**关键澄清（聚合模式）**：
- OpenClaw 通过 `tools/list` 看到的工具 **由 api-server 自己生成**（与 agent-server 无关），永远只是 `send_message` 一个；
- agent-server 内部的具体 HCM 工具（`list_cvms` / `query_clb` 等）通过**南向** MCP（`/api/v1/mcp/internal/hcm/mcp/`）暴露给 agent-server LLM —— **这部分对 OpenClaw 完全不可见**；
- 这与「网关 MCP-proxy 直接把 HCM REST API 转 MCP 给 OpenClaw」（旧模式）完全不同 —— 旧模式下 OpenClaw 端 LLM 要自己学习 HCM 工具语义；本方案下 OpenClaw 只需要会聊天，HCM Agent 自己规划工具调用。

**理由**：
- iWiki 4021231666 主文 3.5 节定义即此名称，与 OpenClaw 接入约束对齐。
- 与 `chat` 这种通用名相比，`send_message` 与 A2A 协议的 `message/stream` 在语义上更贴近，方便后续扩展 `tools/call(get_task)` 等。
- 所有 `mcp_server_name` 统一返回同一工具集，让客户端配置「调用哪个 server name」只影响审计/限流维度，不影响功能 —— 简化客户端心智模型。

**扩展点**（不在本期范围）：
- 后续如需按 `mcp_server_name` 暴露不同工具集（例如 `bk-hcm-finops` 只允许 FinOps 类 send_message 变体），可在 `tools/list` handler 内按 ctx 中的 `mcp_server_name` 做工具过滤；
- 后续如需按 `mcp_server_name` 注入差异化 system prompt / agent skill 偏置，可在 `send_message` 调 A2A 时把 `mcp_server_name` 透传到 metadata，让 agent-server 据此挂载对应 skill。

### D9：服务发现按需扩展

**决定**：仅当 `mcp.ingress.enable: true` 时，在 `cmd/api-server/app/app.go` 的 `discOpt.Services` 中追加 `cc.AgentServerName`；存量启用配置下不变更服务发现订阅。

**理由**：
- 启用前对 etcd 没有任何新增 watch，对存量影响为 0。
- A2AClient 通过 `discovery.APIDiscovery(cc.AgentServerName).GetServers()` 拿到 agent-server 实例地址即可，无需新增 client 包。

### D10：日志安全约束

**决定**：MCP 与桥接代码层禁打印：
- 蓝鲸网关 JWT 原文；
- `bk_ticket`；
- `X-Bkapi-Authorization` header 完整值；
- A2A `function_call.args` 中的工具入参原文（仅打印工具名 + 入参长度）。

允许打印：
- 解析后的 `bk_username` / `app_code` / `tenant_id`（明文，业务标识，不敏感）；
- `rid` / `taskId` / `contextId` / `artifactId`；
- A2A `state` 状态机变化。

**理由**：与 `.cursor/rules/logging-standard.mdc` 一致；同时符合方案 2 文档「日志中禁止打印 `bk_ticket` / JWT 等敏感凭证明文」的安全要求。

### D13：bridge / a2aPassthrough 的 agent-server 实例统一走 etcd 服务发现（不在 yaml 中手填 URL）

**决定**：
- `MCPBridgeSetting` **移除** `AgentServerA2AURL` 字段；`A2APassthroughSetting` **移除** `AgentServerURL` 字段；
- api-server 启动时通过 `discovery.NewAPIDiscovery(cc.AgentServerName, dis)` 创建一个共享的 agent-server 服务发现器（**与 `cmd/web-server/service/proxy.go` 同源**）；
- **bridge**：在 `send_message` tool handler 内每次调用前 `APIDiscovery.GetServers()` 选实例，拼接固定路径段 `constant.A2ABasePathDefault + constant.A2AJSONRPCSubPath`（即 `/api/v1/agent/a2a`）作为 `trpc-a2a-go.A2AClient.StreamMessage` 的目标 URL；
- **a2aPassthrough**：在 `httputil.ReverseProxy.Director` 内每次转发前 `APIDiscovery.GetServers()` 选实例，重写 `req.URL.Scheme` 与 `req.URL.Host`。

**理由**：
- 与 web-server proxy 对 agent-server 的访问模式完全一致，**全局架构统一**：api-server / web-server 都用 etcd 服务发现，不引入新的"硬编码 URL"模式；
- agent-server 扩容 / 缩容 / 实例迁移完全自动生效，运维**无需修改 yaml 重启**；
- 物理机 / docker / K8s 部署形态都被 etcd 服务发现统一抽象，部署文档简化；
- `trpc-a2a-go.A2AClient` 不在结构体里持有 baseURL，可以每次调用构造 URL（已验证）；`httputil.ReverseProxy.Director` 是 per-request 钩子，每次重新选 host 没有额外开销。

**例外**：南向 `mcp.internal.openapiSpecPath` 是本地文件路径（不是 URL），与 etcd 服务发现无关；默认值指向仓库内 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm_internal_mcp.yaml`，运维可在 yaml 中覆盖。

**备选**：方案 A：手填 URL（已废弃） —— 与 web-server 模式不一致、运维复杂、单实例假设不利于扩容；方案 B：etcd + override 兜底 —— 评估后认为 etcd 已经覆盖所有部署形态，override 字段反而增加配置心智，**不引入**。

## Risks / Trade-offs

| 风险 | 影响 | 缓解 |
|---|---|---|
| `trpc-mcp-go v0.0.16` 是较新版本，server 侧能力可能有 BUG | 中 | 单元测试覆盖所有 method；联调阶段一旦遇到协议层 BUG 立刻上报上游；已知 bug：`manager_tools.go:252` 丢弃 `_meta.progressToken`，本项目通过 `cancel_token.go` JSON-RPC 中间件兜底，仅服务于 cancel 反查路径，progress 通知走 SDK 原生 `GetNotificationSender(ctx)` 无影响 |
| 蓝鲸网关对 `text/event-stream` 缓冲导致 progress 延迟 | 中 | 在 SKILL 文档中明确要求网关资源 `enable_streaming: true`；每个 progress 后强制 Flush（responder_sse 内置） |
| 同一 contextId 多端并发可能让 agent 收到交错请求 | 低 | A2A 协议本身无并发约束，由 agent-server 侧 session 串行化决定；SKILL 中告知客户端「同一 contextId 不要并发请求」 |
| 内置 MCP 仅靠 `bk_username` 信任，header 可伪造 | 中 | 端点仅内网可达（启动时检查监听地址或独立端口）；`X-Bkhcm-Caller-Source` 白名单；agent-server → api-server 之间走内网；不接受外网访问 |
| MCP `notifications/cancelled` 的 progressToken 与 taskId 解耦后映射可能丢 | 低 | 映射放在 tool handler closure 中，与请求生命周期严格绑定；测试用例覆盖「断连即取消」「显式 cancelled 即取消」 |
| `proxy.go::peekRequest` 未来有人改成全局 filter 影响 MCP 路径 | 中 | MCP 路径不走 `proxy.apiSet()`，无论 filter 怎么改都不会影响；同时在 PR review 时核查 |
| 同名工具在北向 `send_message` 与南向工具集出现冲突 | 低 | 两个 Server 实例是独立的工具命名空间，物理隔离，不存在跨 Server 名冲突 |
| trpc-a2a-go A2AClient 对 agent-server 重启时连接异常的处理 | 低 | tool handler 出错时返回 `CallToolResult{isError:true, message:...}`，由 OpenClaw 决定重试 |

## Migration Plan

### 阶段化上线（可逐步开关）

| 阶段 | 行为 | 配置 | 影响面 |
|---|---|---|---|
| **0. 当前** | api-server 仅 proxy | `mcp` 段未配置或不存在 | 行为不变 |
| **1. 灰度南向** | 仅启用内置 HCM MCP；agent-server 改 toolset 指向内置 MCP | `mcp.internal.enable: true` | 仅 agent-server LLM 内部调用切换；外部 OpenClaw 路径不变 |
| **2. 联调北向** | 启用北向 ingress 但暂不接 OpenClaw；本地 curl 测试 | `mcp.ingress.enable: true` | 新增 `/api/v1/mcp/servers/...` 路径；其他不变 |
| **3. 接入 OpenClaw** | 蓝鲸网关配置生效，OpenClaw 切换到新路径 | 同上 + 网关侧上线 | 旧的 `remote-agent-http-sse-adapter.py` 逐步停用 |

### 回滚

- 任一阶段失败：将对应开关置为 `false`，重启 api-server 即可。
- 服务发现是按需订阅（D9），关闭后立即停止对 agent-server 的 etcd watch。
- proxy 路径与 AGUI 链路在所有阶段下行为零变化，回滚不影响其它功能。

## Open Questions

### 1. 南向内置 HCM MCP 工具集来源 → **已决策：方案 B+（网关自动同步 + 内部扩展工具配置 + 可选过滤）**

**先澄清两个 `tools/list` 的区别**（关键概念）：

| | 调用方 | 路径 | 返回内容 |
|---|---|---|---|
| **北向 `tools/list`** | OpenClaw | `/api/v1/mcp/servers/{name}/mcp/` | **api-server 自己生成的 `[send_message]`**，与 owner 无关 |
| **南向 `tools/list`** | agent-server LLM（toolset） | `/api/v1/mcp/internal/hcm/mcp/` | **本地 OpenAPI 3.0 yaml 加载 + 内部扩展工具**（详见 D7 / D12 更新） |

**决定**（详见 D7 / D12 更新版）：
- 主体：启动时一次性从本地 `MCP.Internal.OpenAPISpecPath`（默认 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm_internal_mcp.yaml`，蓝鲸网关「资源配置列表」导出）解析工具集；
- 暴露白名单：cc 中 `includeOperationIDs`（精确匹配 + glob）控制最终暴露的工具集，空白名单时全暴露；
- 扩展：cc 中 `internalOnlyTools` 配置「OpenAPI yaml 之外的本地工具」（结构化条目，含 name/description/inputSchema/backend），数量预期 ≤ 5；
- 启动策略：yaml 单次同步加载，不做周期同步、不写本地缓存、不依赖外部网络；
- 调用路由：所有工具都走 api-server 自身 proxy 链路（self-call），按 yaml 中 `x-bk-apigateway-resource.backend.method/path` 或 `internalOnlyTools[].backend` 拼接目标 URL（剥离 `{env.url_path_prefix}`，替换 path 模板占位）。

**Owner 在本变更中的实际任务**：
- 维护 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm_internal_mcp.yaml`：接口字段变更后从蓝鲸网关「资源配置列表」重新导出覆盖即可（属于 MR 日常流程）；
- 提供 `includeOperationIDs` 初始白名单（推荐第一阶段只暴露查询类 `list_* / get_* / describe_*`，操作类逐步开放）；
- 提供 `internalOnlyTools` 的初始列表（可空，后续增量配置）；
- CI 校验：在 MR 流程加 lint 倒推接口 owner 补全网关资源配置中的 requestBody / responses 字段。

**日常维护负担**：接近 0 —— HCM 接口字段变更时，开发者改完 Go 代码 + 网关资源配置之后，重新导出一次 yaml 提交即可，**不需要触动 api-server 任何代码**。

### 2. 南向 MCP 端口 → **已决策：复用 8080**

**决定**：南向内置 HCM MCP 与 proxy / 北向 MCP ingress / A2A 透传**复用同一个 8080 端口**。

**安全护栏**（替代物理端口隔离的弱化措施）：
- mux 上 `/api/v1/mcp/internal/hcm/mcp/` handler 强制校验 `X-Bkhcm-Caller-Source: agent-server`（来自调用方注入），缺失或不匹配立即返回 403；
- 同时在 cc 配置中提供 `mcp.internal.allowedSourceIPs`（CIDR 列表，默认仅集群内网段），中间件做 `RemoteAddr` 网段校验；
- 不在文档/网关上暴露该路径；蓝鲸 API 网关**不**为该路径注册资源。
- 后续若有审计要求强物理隔离，再独立监听端口 8081（仅绑定内网 IP），切换成本低（mux 拆分即可）。

### 3. agent-server `enforceCallerOrigin` 是否切 true → **已决策：由配置决定**

**决定**：
- agent-server 侧 `a2a.enforceCallerOrigin` 由各环境 `agent_server.yaml` 配置项决定，本变更不强制要求线上切 `true`/`false`。
- api-server 侧**始终**在向 agent-server 发起的 A2A 调用（无论是 MCP ingress 内部转发，还是 A2A 透传反代）中**注入** `X-Bkhcm-Caller-Source: api-server`，确保任一环境下都能通过 agent-server 的 caller 校验。
- 推荐策略（写入 SKILL 与 etc 注释）：dev/test 环境 `enforceCallerOrigin: false`（便于联调放行），生产环境 `enforceCallerOrigin: true`（拦截非法来源）。

### 4. `mcp_server_name` 多名称 → **已决策：支持任意名称，统一处理**

**已澄清的实际场景**：HCM 在蓝鲸 API 网关已注册多个 MCP server name（如 `bk-hcm-devhk-tcloud-ziyan-cvm` / `bk-hcm-devhk-tcloud-cvm-operate` / `bk-hcm-finops` 等），OpenClaw 可以同时配置多个 MCP url。api-server 必须对所有 server name 统一处理（解析 JWT → MCP↔A2A 转换 → 调 agent-server）。

**决定**（已写入 D2 / D8）：
- api-server 北向 ingress 路径模式：`/api/v1/mcp/servers/{mcp_server_name}/mcp/`，**接受任意 `mcp_server_name`**；
- 所有 `mcp_server_name` 共用同一个 `trpc-mcp-go.Server` 实例，`tools/list` 统一返回 `[send_message]`；
- `mcp_server_name` 注入 ctx → 日志 / metrics 标签，便于网关侧审计与按 name 限流；
- 第一阶段**不做** server name 白名单（接受网关侧注册的所有 name），不做按 name 差异化的工具集 / system prompt（保留为扩展点）。

**功能影响分析**：
- 方案 2 的聚合模式下，所有 `mcp_server_name` 行为一致（都走 `send_message` → A2A → agent-server），**功能上无差异**；
- OpenClaw 客户端配置多个 url 的价值主要在于网关侧的差异化限流/审计，以及未来按业务拆 Agent 的扩展空间；
- 与 D7 的南向工具集差异化是**两个独立的层面**：北向 server name 是「客户端入口标识」，南向工具集是「LLM 能调用的实际能力」。

### 5. 北向 ingress 是否暴露 AgentCard → **已决策：暴露，新增 A2A 透传 capability**

**回答用户问题「不暴露的话外部 Agent 如何跟 HCM 的 agent 进行 A2A 通信」**：完全正确，本设计同意你的判断。**design 已新增 D11**，让 api-server 同时暴露：

- `POST /api/v1/agent/a2a`（A2A JSON-RPC 透传到 agent-server）
- `GET /api/v1/agent/.well-known/agent-card.json`（A2A v0.2.2 AgentCard）
- `GET /api/v1/agent/.well-known/agent.json`（A2A 0.1.x 兼容 AgentCard）

对应**新增 capability** `api-server-a2a-passthrough`（已同步到 proposal.md）；OpenClaw 等 MCP 客户端走 MCP ingress、`trpc-a2a-go` / `a2a-python` 等 A2A 原生客户端走 A2A 透传。两条路径上的鉴权由 api-server 统一完成，agent-server 侧 `mcpCallerOriginMiddleware` 校验由 api-server 注入的 `X-Bkhcm-Caller-Source: api-server`。
