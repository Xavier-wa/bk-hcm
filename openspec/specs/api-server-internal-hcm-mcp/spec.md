# api-server-internal-hcm-mcp Specification

## Purpose
TBD - created by archiving change api-server-mcp-a2a-bridge. Update Purpose after archive.
## Requirements
### Requirement: 南向内部 MCP 入口路径与端口复用

api-server SHALL 在外层 `http.ServeMux` 上前缀挂载 `/api/v1/mcp/internal/hcm/mcp/`，作为南向**内部 HCM MCP Server**入口，仅供 agent-server LLM tool calling 使用。

该入口 SHALL 复用 api-server 主端口（默认 `8080`），不另起独立端口（最小化运维变更面）。

该入口与北向 ingress 共享同一 `http.ServeMux`，但 SHALL 使用**独立的** `trpc-mcp-go.Server` 实例（命名 `hcm-internal-mcp`，区别于北向的 `hcm-agent`）；两者注册的工具集完全不同。

#### Scenario: 内部 MCP 路径独立处理

- **WHEN** agent-server 通过 `http://api-server:8080/api/v1/mcp/internal/hcm/mcp/` 发送 MCP 请求
- **THEN** api-server SHALL 路由到南向 MCP server handler（`hcm-internal-mcp`）
- **AND** SHALL 不进入北向 ingress 的 `hcm-agent` server 实例

#### Scenario: 端口与现有 proxy 共存

- **WHEN** api-server 启动并监听 `:8080`
- **THEN** `/api/v1/mcp/internal/hcm/mcp/` 与 `/api/v1/cloud/...` 等代理路径 SHALL 同端口共存
- **AND** 两者路由相互独立（不存在路径冲突）

### Requirement: 仅内网访问的网络隔离

南向内部 MCP 入口 SHALL 通过以下任一方式实现"仅 agent-server 可访问"的网络隔离（在部署文档中明确）：

- 方式 A：Kubernetes `NetworkPolicy` 限制 ingress 仅允许 `agent-server` Pod 的 label selector；
- 方式 B：蓝鲸网关 `Path Strip` 配置（不向网关发布 `/api/v1/mcp/internal/` 段，使其无法从外部访问）；
- 方式 C：api-server 自行在该路径中间件中按 `RemoteAddr` 限制内网 IP 段（兜底）。

至少 SHALL 实现"方式 A + 方式 B"（部署侧网络隔离），方式 C 作为安全冗余可选启用。

#### Scenario: 外部公网请求被拒

- **WHEN** 外部公网客户端尝试访问 `https://<bk-apigw-host>/api/v1/mcp/internal/hcm/mcp/`
- **THEN** 蓝鲸网关 SHALL 返回 404 或同等错误（路径未发布）
- **AND** 请求 SHALL 不到达 api-server

#### Scenario: 集群内非 agent-server pod 被拒（如启用方式 A）

- **WHEN** 集群内非 agent-server pod 尝试访问 `http://api-server:8080/api/v1/mcp/internal/hcm/mcp/`
- **THEN** `NetworkPolicy` SHALL 阻断该请求
- **AND** 请求 SHALL 在网络层失败（不到达 api-server）

### Requirement: 强制 caller-source 鉴权 + bk_username 透传

南向 MCP server handler SHALL 在入口中间件强制校验：

1. HTTP header `X-Bkhcm-Caller-Source` SHALL 等于 `agent-server`（固定字面量）；否则返回 `HTTP 403 Forbidden`。
2. HTTP header `X-Bkapi-User-Name` SHALL 非空；该值作为权威用户身份写入 ctx，下游工具实现根据此值做权限校验。
3. SHALL 不调用 `gwparser.Parse`（南向调用方为内部 agent-server，没有 `X-Bkapi-JWT`）。

南向 MCP SHALL 不执行任何 JWT 解析；身份完全依赖 agent-server 注入的 header。

#### Scenario: 合法内部调用通过

- **WHEN** agent-server 调用南向 MCP 时携带 `X-Bkhcm-Caller-Source: agent-server` + `X-Bkapi-User-Name: alice`
- **THEN** 中间件 SHALL 通过校验
- **AND** ctx 中 SHALL 携带 `bk_username=alice`

#### Scenario: 缺 caller-source 拒绝

- **WHEN** 请求未携带 `X-Bkhcm-Caller-Source` 或值不等于 `agent-server`
- **THEN** api-server SHALL 返回 `HTTP 403 Forbidden`
- **AND** SHALL 不调用任何工具 handler

### Requirement: 工具集按内置 MCP server 从本地 OpenAPI yaml 加载

api-server SHALL 在启动阶段从 `ApiServerSetting.MCP.Internal.Servers[]` 配置中为每个条目分别创建一个 `trpc-mcp-go.Server` 实例，并 SHALL 在对应 `basePath` 挂载独立 MCP endpoint。

每个 `servers[]` 条目 SHALL 包含：

- `name`：MCP server 标识；
- `basePath`：该 MCP server 的 HTTP 路径前缀；
- `openapiSpecPath`：本地 OpenAPI 3.0 yaml 文件路径；
- `includeOperationIDs`：可选工具白名单；
- `internalOnlyTools`：可选本地专用工具列表。

该 yaml 文件由蓝鲸 API 网关「资源配置列表」导出，是 HCM 对内接口的唯一权威描述（接口字段一改，开发者在网关后台改资源配置 → 自动覆盖到本 yaml）。**api-server 不再调用蓝鲸网关 MCP-proxy `tools/list` 接口同步 schema**，**不再使用任何远程同步机制、本地缓存机制、周期同步 goroutine**。

加载规则：

- **tool name** = OpenAPI `operationId`；
- **tool description** = OpenAPI `description`（若为空则用 `summary`）；为弥补 `responses.200.schema.data.additionalProperties: true` 的语义缺失，OpenAPI `responses.200.content.application/json.example` SHALL 被 JSON 化序列化后**追加**到 tool description 末尾（`Example response: <json>`），让 LLM 能从 example 推导返回结构；
- **InputSchema** = OpenAPI `requestBody.content.application/json.schema` 与 `parameters`（path / query）合并后的 JSON Schema：每个 path/query parameter SHALL 作为 InputSchema 的 properties 字段加入，`required: true` 的 parameter SHALL 加入 required 列表；
- **backend.method/path** = `x-bk-apigateway-resource.backend.method` 与 `x-bk-apigateway-resource.backend.path`（path 模板中的 `{env.url_path_prefix}` SHALL 被剥离，得到形如 `/api/v1/cloud/...` 的路径）。

api-server SHALL 支持配置 `ApiServerSetting.MCP.Internal.Servers[].IncludeOperationIDs` 白名单：

- 当白名单非空：仅注册 `operationId` 命中白名单（支持精确匹配与 glob 模式如 `list_*`、`*_cvm`）的接口；
- 当白名单为空：注册 yaml 中**全部** `operationId`（不过滤）。

启动期 yaml 解析失败（文件缺失、yaml 语法错误、关键字段缺失）SHALL 阻塞启动并返回错误。

#### Scenario: yaml 解析成功后工具立即可用

- **WHEN** api-server 启动且 `mcp.internal.enable: true`，并配置 `servers[]` 中某个 server 的 `openapiSpecPath` 指向合法 yaml
- **THEN** 该 server 的南向 MCP `tools/list` SHALL 在启动后立即返回该 yaml 中所有命中该 server 白名单的工具
- **AND** SHALL 不发起任何到蓝鲸网关 MCP-proxy 的网络调用
- **AND** SHALL 不创建任何周期同步 goroutine
- **AND** SHALL 不写任何本地缓存文件

#### Scenario: yaml 文件缺失阻塞启动

- **WHEN** `mcp.internal.enable: true` 但某个 `servers[].openapiSpecPath` 指向的文件不存在
- **THEN** api-server SHALL 启动失败并返回明确错误
- **AND** 错误信息 SHALL 包含路径与原因

#### Scenario: tool description 内嵌 example 返回结构

- **WHEN** OpenAPI 接口 `list_account_with_extension` 的 `responses.200.schema.data` 是 `additionalProperties: true`，但 `example` 字段有完整结构示例
- **THEN** 注册到 MCP 的工具 description 末尾 SHALL 追加 `Example response: {...}`（JSON 序列化后的 example）
- **AND** LLM 调用方 SHALL 能从该示例推断出 `details[].vendor`、`details[].bk_biz_id` 等字段

#### Scenario: IncludeOperationIDs 白名单过滤

- **WHEN** yaml 含 203 个 operationId，某个 server 配置 `includeOperationIDs: ["list_cvms", "list_*_quota"]`
- **THEN** 南向 MCP `tools/list` SHALL 仅返回命中白名单的工具（精确匹配 `list_cvms` 加 glob 命中 `list_biz_quota`、`list_account_quota` 等）
- **AND** SHALL 不暴露白名单外的工具

#### Scenario: 白名单为空时全部暴露

- **WHEN** 某个 server 的 `includeOperationIDs` 为空数组或缺失
- **THEN** 南向 MCP `tools/list` SHALL 包含 yaml 中**全部**有合法 `operationId` 的接口

### Requirement: 内部专用工具（OpenAPI yaml 之外的本地工具）

api-server SHALL 支持配置 `ApiServerSetting.MCP.Internal.Servers[].InternalOnlyTools` 列表，定义**网关资源未发布、由 api-server 本地代码维护**的内部专用工具。

该列表用于「LLM 决策需要但不对外开放」的少量能力（如内部审计聚合查询）；schema 与 handler 均由 api-server 本地代码维护，**不**与 OpenAPI yaml 共享数据源。

`InternalOnlyTools` 的工具集 SHALL 与 OpenAPI yaml 加载的工具集**取并集**作为最终南向工具集；工具名冲突时 `InternalOnlyTools` SHALL 覆盖 OpenAPI 同名工具。

InternalOnlyTools 的数量预期 SHALL 控制在 5 个以内；超过 5 个时应优先把工具补到网关资源配置（让 OpenAPI yaml 自动覆盖）。

#### Scenario: internal-only 工具仅南向可见

- **WHEN** `InternalOnlyTools` 包含 `internal_audit_query`
- **THEN** 南向 MCP `tools/list` SHALL 包含 `internal_audit_query`
- **AND** 北向 MCP（OpenClaw 视角）的 `tools/list` SHALL 仍仅返回 `[send_message]`（不暴露 `internal_audit_query`）

#### Scenario: 同名时 InternalOnlyTools 覆盖 OpenAPI

- **WHEN** OpenAPI yaml 含 `list_secret_key`，且 `InternalOnlyTools` 也配置了同名工具
- **THEN** 南向 MCP `tools/list` 中的 `list_secret_key` SHALL 取自 `InternalOnlyTools` 配置（覆盖 OpenAPI 加载的版本）

### Requirement: tools/call 本地后端路由

南向 MCP server SHALL 按工具的 backend 元信息路由 `tools/call`，**所有工具调用 SHALL 走 api-server 自身处理链路**，**SHALL 不**反向调用蓝鲸网关或外部 MCP-proxy。

后端调用规则：

- **OpenAPI yaml 来源的工具**：使用 yaml 中 `x-bk-apigateway-resource.backend.method/path`（剥离 `{env.url_path_prefix}` 前缀后）作为内部 HTTP 调用目标；调用通过 api-server 现有 proxy 链路（`cloud-server`、`woa-server`、`account-server`）完成；
- **InternalOnlyTools 的工具**：使用配置中显式的 `backend.method/path`，路径同样命中 api-server 现有 proxy 路由；
- 调用时 SHALL 在请求 header 中携带 `X-Bkapi-User-Name`（从 ctx 中 `kit.Kit.User` 取得）；
- 调用时 SHALL 不携带 `X-Bkapi-JWT`、`bk_ticket` 等外部凭证（南向链路是内部信任）。

`tools/call` 调用 SHALL 在 5s 内启动响应；上游下游服务超时默认 60s（可配置）。

#### Scenario: OpenAPI 工具走本地 proxy 链路

- **WHEN** agent-server 调用 `tools/call(list_cvms, arguments={bk_biz_id: 100, page: {...}})`
- **THEN** api-server SHALL 通过内部 HTTP 调用 `/api/v1/cloud/bizs/100/cvms/list`（具体 path 来自 yaml backend）
- **AND** SHALL 在 header 中携带 `X-Bkapi-User-Name: <ctx.user>`
- **AND** SHALL 不发起任何到蓝鲸网关或外部 MCP-proxy 的连接

#### Scenario: InternalOnlyTools 工具走本地 handler

- **WHEN** agent-server 调用 `tools/call(internal_audit_query, ...)`
- **THEN** 请求 SHALL 由 api-server 本地 handler 处理（路径来自 `InternalOnlyTools[].backend`）
- **AND** SHALL 不调用蓝鲸网关

#### Scenario: 后端调用超时返回错误

- **WHEN** 工具调用对应的内部服务 60s 未响应
- **THEN** 南向 MCP SHALL 返回 `CallToolResult{isError: true, content:[{text: "后端调用超时"}]}` 或 JSON-RPC `-32603`
- **AND** SHALL 输出 `Errorf` 日志含 `err: %v, rid: %s`

### Requirement: 启用开关与默认值

api-server SHALL 在 `ApiServerSetting.MCP.Internal` 段提供 `Enable bool` 子项，**默认 `false`**。

当 `mcp.internal.enable: false` 时，api-server SHALL 不在外层 mux 上注册 `/api/v1/mcp/internal/` 路径段，南向 MCP 相关初始化逻辑（OpenAPI 加载、tool 注册）SHALL 不执行。

#### Scenario: 默认关闭零额外资源

- **WHEN** 存量 `api_server.yaml` 不含 `mcp.internal` 段
- **THEN** api-server SHALL 不监听 `/api/v1/mcp/internal/`
- **AND** SHALL 不读取任何 OpenAPI yaml
- **AND** memory / goroutine count SHALL 不因本变更显著增加

### Requirement: 与 agent-server 现有 MCPTypeInternal 协同

agent-server `agent_server.yaml` 的 `mcpServers[].type: "internal"` MCP 工具集配置 SHALL 支持指向本 api-server 南向 MCP 入口（`http://api-server:8080/api/v1/mcp/internal/hcm/mcp/`）。

api-server 本变更 MUST NOT 要求修改 agent-server `internal` MCP 客户端实现（即 agent-server 现有 MCP client 调用接口不变）。

agent-server 与 api-server 之间的协议层完全遵循标准 MCP streamable HTTP 规范（JSON-RPC 2.0）。

#### Scenario: agent-server 配置切换无代码改动

- **WHEN** 运维更新 `agent_server.yaml` 将 `mcpServers[].url` 从老的 MCP 端点切换到 `http://api-server:8080/api/v1/mcp/internal/hcm/mcp/`
- **THEN** agent-server SHALL 无需任何代码修改即可加载新配置
- **AND** agent-server LLM SHALL 能从 api-server 南向 MCP 正确获取 `tools/list` 与执行 `tools/call`

