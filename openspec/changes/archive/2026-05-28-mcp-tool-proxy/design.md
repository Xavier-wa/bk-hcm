## Context

agent-server 当前通过 `tool.BuildMCPToolSets()` 构建 MCP ToolSet，并将所有工具通过 `graph.WithToolSets(toolset.TS)` 注册到 Agent。当工具数量达到 50+ 时，工具定义占用 10k-25k tokens，严重影响上下文窗口利用率和 LLM 选择准确率。

项目已有完整的工具索引体系（`tool/tool_index.go`）：
- `ToolMeta` 结构体：工具元数据（名称、描述、参数、标签、搜索文本）
- `ToolIndex` 接口：`Build(ctx, tools)` / `Search(ctx, query, topN, scoreThreshold)`
- 三种实现：`KeywordIndex`（关键词）、`BM25Index`（BM25 算法）、`EmbeddingIndex`（向量余弦相似度，基于 `embedder.Embedder`）

`trpc-agent-go` 框架的 `tool.Tool` 接口提供了 `Declaration()` 方法获取工具声明（`Name`、`Description`、`InputSchema`），以及 `Execute(ctx, args)` 方法执行工具调用。

**约束**：
- Runner、SessionService、MemoryService、AG-UI SSE 服务层完全复用，不可破坏
- MCP ToolSet 构建逻辑（含 bkaidev 认证注入）保持不变，需扩展 hook 支持 `access_token` 鉴权
- Graph 模式需兼容现有的 `bk_username` / `bk_ticket` 上下文注入机制（`execute_tool` 运行时仍用用户 ticket）
- Tool Proxy 初始化凭据从 `global_config`（`auth/access_token`）按 `initVirtualUser` 读取，不写入本地 yaml
- Go 错误处理必须显式，不能忽略 error 返回值

## Goals / Non-Goals

**Goals:**

- 实现 3 个元工具（`search_tools`、`get_tool_schema`、`execute_tool`），替代 N 个实际 MCP 工具注册到 Agent
- 复用现有 `tool/tool_index.go` 的 `ToolMeta` 和 `EmbeddingIndex` 实现工具向量化和语义检索
- 实现工具注册表管理，支持工具的注册、更新、查询、删除
- 确保端到端流程可跑通：Agent 使用元工具完成"搜索 → 执行"完整流程
- Token 消耗降低 90%+（从 N 个工具定义降低到 3 个元工具定义）
- 启动时完成工具加载与向量化，用户首请求无额外索引构建延迟

**Non-Goals:**

- **不接入 LLM Agent 模式**（`llmagent`）—— MVP 仅 Graph 模式；阶段 2 再改 `agent_llm.go`
- **不在 Graph 模式保留 `dynamicToolLoading`** —— 与 Tool Proxy 目标重叠；且「用户消息 → 静默向量过滤」召回准确性不足，已由 LLM 主动调用 `search_tools` 替代
- 不实现混合检索（语义 + 关键词 + 使用统计）—— 这是阶段 2 优化目标
- 不实现智能错误处理（详细错误信息和建议）—— 这是阶段 2 优化目标
- 不实现工具使用统计和推荐功能 —— 这是阶段 3 高级功能
- 不引入外部向量数据库（Faiss/Chroma/Milvus）—— MVP 使用内存向量检索
- 不改造前端或 AG-UI 协议层

## Decisions

### D1：模块结构 —— 新增 `toolproxy` 包

**选择**：在 `cmd/agent-server/logics/toolproxy/` 下新增 7 个文件，作为独立的 `toolproxy` 包。

**理由**：
- 与现有 `tool/` 包职责分离：`tool/` 负责 MCP ToolSet 构建和工具索引，`toolproxy/` 负责元工具实现
- 包内聚合相关功能（注册表、向量存储、元工具实现），便于独立测试和维护
- 文件粒度与现有代码一致（`tool_index.go`、`tool_callbacks.go` 等）

**文件结构**：

| 文件 | 职责 |
|------|------|
| `types.go` | 公共类型定义（`ToolMetadata`、`ToolRegistry`、`ToolProxy` 等） |
| `registry.go` | 工具注册表管理（注册、更新、删除、查询） |
| `proxy.go` | `ToolProxy` 生命周期：`Build`、`StartRefreshLoop`、`GetProxyToolSet`、索引重建 |
| `search.go` | `search_tools` 元工具实现（语义搜索 + 返回 schema） |
| `schema.go` | `get_tool_schema` 元工具实现（查询工具注册表，返回 schema） |
| `execute.go` | `execute_tool` 元工具实现（参数验证 + 调用 MCP Tool） |
| `config.go` | 从 global_config 读取初始化 access_token |

> MVP 不单独引入 `vectorstore.go` / `VectorStore` 接口，直接向持 `*tool.EmbeddingIndex`（见 D3、D15）。

**替代方案**：在现有 `tool/` 包内新增文件（拒绝：职责混淆，`tool/` 已有明确职责——MCP ToolSet 构建和工具索引）

---

### D2：工具注册表设计 —— 复用 `tool.ToolMeta`，新增 `ToolRegistry` 结构体

**选择**：复用 `tool.ToolMeta` 作为工具元数据结构，新增 `ToolRegistry` 结构体管理所有工具的元数据。

**数据结构**：

```go
// cmd/agent-server/logics/toolproxy/types.go

// ToolMetadata 扩展了 tool.ToolMeta，增加 Category 和 Schema 字段
type ToolMetadata struct {
    tool.ToolMeta        // 嵌入现有 ToolMeta（Name, Description, Parameters, Tags, SearchText）
    Category    string                 `json:"category"`     // 工具类别（code/file/web 等）
    Schema      map[string]interface{} `json:"schema"`      // 工具的完整 JSON Schema
    Examples    []map[string]interface{} `json:"examples"`   // 工具调用示例
    UsageCount  int                    `json:"usage_count"` // 使用次数（阶段 1 预留，阶段 3 使用）
    SuccessRate float64                `json:"success_rate"` // 成功率（阶段 1 预留，阶段 3 使用）
}

// ToolRegistry 维护所有工具的元数据，支持并发安全的注册、更新、删除、查询
type ToolRegistry struct {
    mu    sync.RWMutex
    tools map[string]*ToolMetadata // key: tool name
}
```

**理由**：
- 复用 `tool.ToolMeta` 避免重复定义，且 `tool.ExtractToolMeta(t tool.Tool, tags []string)` 可直接生成 `ToolMeta`
- `ToolRegistry` 使用 `sync.RWMutex` 保证并发安全（Agent 运行时可能并发查询工具）
- `Schema` 字段直接从 `tool.Declaration().InputSchema` 获取，无需重新构造

**替代方案**：完全重新定义工具元数据结构（拒绝：重复劳动，且无法复用 `tool.ExtractToolMeta`）

---

### D3：向量检索设计 —— MVP 直接使用 `tool.EmbeddingIndex`

**选择（MVP）**：`ToolProxy` 直接持有 `*tool.EmbeddingIndex`，不引入 `VectorStore` 接口或 `vectorstore.go` 封装层。阶段 2 再按需抽取接口以适配 Faiss/Chroma 等外部向量库（见 D15）。

**数据结构**：

```go
type ToolProxy struct {
    registry    *ToolRegistry
    index       *tool.EmbeddingIndex  // 语义检索，复用 tool/tool_index.go
    mcpToolSets *tool.MCPToolSet
    emb         embedder.Embedder
    buildOK     bool
    // ...
}
```

**索引构建与搜索**：

- `Build()`：`loadTools` 写入 `ToolRegistry` 后，从 Registry 导出 `[]tool.ToolMeta`，调用 `index.Build(ctx, metas)`
- `search_tools`：调用 `index.Search(ctx, query, topK, scoreThreshold)` 得到 `[]tool.ToolMatch`，再 `registry.GetTool(name)` 组装含 schema 的返回
- 定时刷新：Registry diff 后全量 `index.Build`（工具 <100 可接受）

**设计要点：EmbeddingIndex 不存储 ToolMetadata 业务字段**

`EmbeddingIndex` 内部持有 `ToolMeta` 副本用于向量化，但 `search_tools` 返回给 Agent 的 schema / description **一律从 `ToolRegistry` 实时读取**，Registry 为唯一权威数据源。

**理由**：
- 复用已有 `EmbeddingIndex`，MVP 零额外抽象
- 避免 `VectorStore` + `toolNames[]` 双份映射的维护成本
- 阶段 2 若需外部向量库，再从 `ToolProxy` 抽 `VectorStore` 接口

**替代方案**：MVP 即引入 `VectorStore` 接口 + `EmbeddingVectorStore` 封装（拒绝：过度抽象，YAGNI）

---

### D4：元工具实现策略 —— 实现 `tool.Tool` 接口

**选择**：3 个元工具均实现 `tool.Tool` 接口，通过 `tool.NewTool()` 包装成标准工具注册到 Agent。

**`tool.Tool` 接口核心方法**：

```go
type Tool interface {
    Declaration() ToolDeclaration           // 返回工具声明（Name, Description, InputSchema）
    Execute(ctx context.Context, args json.RawMessage) (any, error) // 执行工具
}
```

**实现方式**：

```go
// cmd/agent-server/logics/toolproxy/search.go

// SearchToolsTool 实现 search_tools 元工具
type SearchToolsTool struct {
    registry *ToolRegistry
    index    *tool.EmbeddingIndex
}

func (t *SearchToolsTool) Declaration() tool.ToolDeclaration {
    return tool.ToolDeclaration{
        Name: "search_tools",
        // Description 须写清分工（见 D17）：未知工具名 / 探索任务时使用
        Description: "【探索未知工具时使用】根据任务描述语义搜索最相关的 MCP 工具，返回工具列表及完整 schema。当用户未指定具体工具名、或你不确定该用哪个工具时调用。若用户已明确工具名，请改用 get_tool_schema。",
        InputSchema: buildSearchToolsSchema(),
    }
}

func (t *SearchToolsTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
    // 1. 解析参数（query, top_k, category）
    // 2. 向量化 query：emb.GetEmbedding(ctx, query)
    // 3. 调用 vectorStore.Search(queryVector, topK)
    // 4. 可选：按 category 过滤
    // 5. 返回工具列表（含 schema）
}
```

**同理实现**：
- `GetToolSchemaTool`（`get_tool_schema` 元工具）
- `ExecuteToolTool`（`execute_tool` 元工具）

**注册到 Agent**：

```go
// cmd/agent-server/logics/agent/graph_build.go

// 构建元工具
searchTool := toolproxy.NewSearchToolsTool(registry, vectorStore)
schemaTool := toolproxy.NewGetToolSchemaTool(registry)
executeTool := toolproxy.NewExecuteToolTool(registry, mcpToolSets)

// 将元工具包装成 tool.ToolSet
proxyToolSet := []tool.Tool{searchTool, schemaTool, executeTool}
stateGraph.AddLLMNode("llm", mdl, prompt, proxyToolSet, llmOpts...)
```

**理由**：
- 实现 `tool.Tool` 接口可无缝集成到现有 Agent 框架（Graph Agent 和 LLM Agent 均支持）
- 元工具的执行逻辑完全自定义，可调用 `vectorStore` 和 `registry`
- 标准工具声明（`ToolDeclaration`）包含 `InputSchema`，Agent/LLM 可正确理解元工具的使用方式

**替代方案**：在 Graph 中新增 Function Node 调用 `search_tools` 逻辑（拒绝：破坏现有 Agent 工具注册机制，且无法复用框架的工具调用链路）

---

### D5：execute_tool 的工具调用策略 —— 通过 `tool.Tool.Execute()` 调用实际工具

**选择**：`execute_tool` 元工具内部通过 `tool.Tool.Execute(ctx, args)` 调用实际的 MCP 工具。调用前须完成三层校验：schema_token 校验 → 参数严格校验（含未知 key 检测）→ 执行。

**实现流程**：

```go
// cmd/agent-server/logics/toolproxy/execute.go

func (t *ExecuteToolTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
    // 1. 解析参数（tool_name, parameters, schema_token）
    var params struct {
        ToolName    string                 `json:"tool_name"`
        Parameters  map[string]interface{} `json:"parameters"`
        SchemaToken string                 `json:"schema_token"`
    }
    json.Unmarshal(jsonArgs, &params)

    // 2. 验证工具是否存在
    meta, err := t.proxy.RegistrySnapshot().GetTool(params.ToolName)
    if err != nil {
        return buildErrorResult("tool_not_found", "tool "+params.ToolName+" not found", nil), nil
    }

    // 3. Schema Token 校验（见 D18）
    if !t.proxy.verifySchemaToken(meta.Name, meta.Schema, params.SchemaToken) {
        return buildErrorResultWithSchema("schema_token_invalid",
            "schema_token 无效，请先调用 search_tools 或 get_tool_schema 获取最新 schema 和 token",
            meta.Schema), nil
    }

    // 4. 参数严格校验（含未知 key 检测）
    if err := validateParameters(meta.Schema, params.Parameters); err != nil {
        return buildErrorResultWithSchema("invalid_parameters", err.Error(), meta.Schema), nil
    }

    // 5. 调用实际工具
    argsBytes, _ := json.Marshal(params.Parameters)
    result, err := t.proxy.CallTool(ctx, meta.Name, argsBytes)
    if err != nil {
        if isPermissionDenied(err) {
            return buildErrorResult("permission_denied", constant.PermissionDeniedMsg, nil), nil
        }
        return buildErrorResult("execution_error", err.Error(), nil), nil
    }

    return buildSuccessResult(result), nil
}
```

**参数严格校验策略**（`validateObject`）：
- 当 schema 定义了 `properties` 且未显式设置 `additionalProperties: true` 时，拒绝所有 schema 未定义的参数 key
- 未知 key 被视为 LLM 幻觉，立即返回错误并附带完整 schema，不继续执行
- 已有的必填字段检查、类型校验逻辑保持不变

**错误响应**：schema_token 无效或参数校验失败时，错误响应中附带工具完整 schema（`required_schema` 字段），LLM 可在同一 response 中直接获取 schema，无需额外 round trip。

**理由**：
- 直接调用 `tool.Tool.Execute()` 复用 MCP ToolSet 的调用链路（含 bkaidev 认证注入）
- schema_token 校验（D18）确保 LLM 在执行前已获取并阅读工具 schema
- 严格未知 key 检测防止因 LLM 幻觉参数导致工具误执行
- 错误内嵌 schema 减少 LLM 纠错所需的交互轮次

**替代方案**：通过 MCP Client 直接调用 MCP Server 的 `call_tool` 接口（拒绝：绕过 ToolSet 封装，丢失认证注入和回调逻辑）

---

### D6：初始化流程 —— 启动时预加载（global_config access_token）

**选择**：在 `logics.New()` 中从 `global_config` 读取初始化用 `access_token`，构造带鉴权的 `ctx`，**启动时同步完成** `loadTools` + 向量化；Graph 构建时注册元工具。元工具执行时直接使用已就绪的 Registry 与 VectorStore，不再懒加载。

**与 `dynamicToolLoading` 的差异**（弃用原因）：

| | dynamicToolLoading | Tool Proxy |
|--|-------------------|------------|
| 检索触发 | 框架 ToolFilter，用**用户原话**静默过滤 | LLM **显式**调用 `search_tools`，自构造 query |
| LLM 可见工具 | Top-N 个真实 MCP schema | 固定 3 个元工具 schema |
| 准确性瓶颈 | 短句/指代（「继续」「可以」）导致 query 质量差 | LLM 将任务改写为检索意图后再搜索 |

**鉴权方式对比**（初始化 vs 运行时执行）：

| 场景 | 鉴权头 | 说明 |
|------|--------|------|
| 启动预加载 / 定时刷新 | `{"access_token": "z"}` | 来自 `global_config` auth/access_token，按 initVirtualUser 选取 |
| 用户 Run 执行 `execute_tool` | `{"bk_app_code":"x","bk_app_secret":"y","bk_username":"...","bk_ticket":"z"}` | 来自请求 Cookie，用户级鉴权 |

两种鉴权均通过 `X-Bkapi-Authorization` 头注入，区别仅在于 JSON 内容。初始化用 `access_token` 可在无用户请求时拉取 MCP 工具列表；实际工具调用仍走用户 `bk_ticket`，权限边界不变。

**global_config 存储**（见 D10）：

```
# access_token 凭据（按 virtual-user 分 key 存储）
config_type: auth
config_key:  access_token
config_value: {"bk-hcm": "<token>"}

# 选用哪个 virtual-user 的 token（agent_server.yaml）
tools.toolProxy.initVirtualUser: bk-hcm
tools.toolProxy.topN: 5
tools.toolProxy.embedding.model: hunyuan-embedding
tools.toolProxy.embedding.dimensions: 1536
```

**初始化流程**：

```go
// cmd/agent-server/logics/runtime.go :: New()

mcpToolSets, err := tool.BuildMCPToolSets()

// 1. 从 global_config 读取 access_token map，按 initVirtualUser 选取 token
virtualUser := cc.AgentServer().Tools.ToolProxy.InitVirtualUser // e.g. "bk-hcm"
initToken, err := toolproxy.LoadInitAccessToken(dataServiceClient, virtualUser)

// 2. 构造初始化 ctx（注入 access_token）
initCtx := auth.WithAccessToken(context.Background(), initToken)

// 3. 启动时同步构建 Registry + VectorStore
proxy, err := toolproxy.NewToolProxy(mcpToolSets, embedder, topN)
if err := proxy.Build(initCtx); err != nil {
    // 启动失败或降级（见 R5）
}

// 4. 启动定时刷新（可选，见 D11）
proxy.StartRefreshLoop(initCtx, refreshInterval)

// Graph 模式（MVP）：
compiledGraph, err := agent.BuildGraph(..., proxy, ...)
// Graph 模式且 proxy 启用时：不设置 dynamicToolFilter（见 D9）
```

**`ToolProxy` 结构体**：

```go
type ToolProxy struct {
    registry    *ToolRegistry
    index       *tool.EmbeddingIndex
    mcpToolSets *tool.MCPToolSet
    emb         embedder.Embedder

    buildOK bool // 启动构建是否成功

    refreshMu   sync.RWMutex // 定时刷新时保护 registry / index
    stopRefresh context.CancelFunc
}

// NewToolProxy 构造 ToolProxy，不访问 MCP
func NewToolProxy(mcpToolSets *tool.MCPToolSet, emb embedder.Embedder, topN int) *ToolProxy { ... }

// Build 启动时同步加载工具并向量化（调用一次）
func (p *ToolProxy) Build(ctx context.Context) error {
    // 1. loadTools(ctx, mcpToolSets)：复用 tool.safeGetTools
    // 2. index.Build(ctx, registry.ExportToolMetas())
    // 失败时 buildOK=false，返回 error
}

// StartRefreshLoop 启动后台定时刷新（refreshInterval=0 时不启动）
func (p *ToolProxy) StartRefreshLoop(ctx context.Context, interval time.Duration) { ... }
```

**`loadTools` 命名约定**：必须与 `tool/tool_filter.go` 中 `LazyToolIndex.ensureBuild` 一致——`frameworkName = {toolsetName}_{rawName}`（toolset 名为空则用 rawName），`execute_tool.findActualTool` 按 frameworkName 查找。

**MCP 鉴权 hook 扩展**（`tool/tool.go`）：

bkaidev ToolSet 的 `WithHTTPBeforeRequest` hook 需支持两种鉴权模式，优先级：**请求 ctx 中的 `bk_ticket` > ctx 中的 `access_token`**：

```go
// auth/bkapi.go 新增
func WithAccessToken(ctx context.Context, token string) context.Context { ... }
func AccessTokenFromContext(ctx context.Context) string { ... }
func AccessTokenAuthHeaderValue(token string) string {
    return fmt.Sprintf(`{"access_token":"%s"}`, token)
}

// tool.go hook 逻辑
ticket := auth.BKTicketFromContext(ctx)
if ticket != "" {
    req.Header.Set(constant.BKGWAuthKey, auth.BKApiAuthHeaderValue(...))
} else if token := auth.AccessTokenFromContext(ctx); token != "" {
    req.Header.Set(constant.BKGWAuthKey, auth.AccessTokenAuthHeaderValue(token))
}
```

**Graph 集成**（`graph_build.go`，见 D13）：

```go
// 方案 B：元工具单独 ToolSet，通过 WithToolSets 注册；skill/HITL 仍走 skillTools map
proxyToolSet := proxy.GetProxyToolSet() // 含 search_tools / get_tool_schema / execute_tool

// LLM Node
opts := []graph.Option{
    graph.WithToolSets([]tool.ToolSet{proxyToolSet}),
    // 不注册 graph.WithToolSets(toolset.TS)
    // 不启用 graph.WithRefreshToolSetsOnRun（见 D14）
}
stateGraph.AddLLMNode("llm", mdl, prompt, skillTools, llmOpts...)

// Tool Node：skillTools + WithToolSets([]tool.ToolSet{proxyToolSet})
// execute_tool 内部持有 mcpToolSets，执行时用请求 ctx（含用户 bk_ticket）
```

**理由**：
- 启动预加载消除首次 Run 的 5–30s 向量化延迟，用户首请求体验与后续请求一致
- `access_token` 存于 `global_config`，运维可独立轮换，无需改 yaml 或重启（配合定时刷新）
- 工具列表加载与用户执行鉴权分离：索引构建用服务 token，实际调用仍用用户 `bk_ticket`

**替代方案**：
- 首次 Run 懒加载（`sync.Once` + 请求 `bk_ticket`）（拒绝：首请求延迟高，向量化阻塞用户交互）
- `access_token` 写入 `agent_server.yaml`（拒绝：敏感凭据不宜落配置文件，且轮换需重启）

---

### D10：初始化 access_token 存储 —— global_config + virtual-user 选择

**选择**：Tool Proxy 初始化用的 `access_token` 按 virtual-user 维度存储在 HCM `global_config` 表；通过 `agent_server.yaml` 中的 `initVirtualUser` 指定首次加载 tool 时使用哪个 virtual-user 的 token。

**global_config 约定**（凭据存储，支持多 virtual-user）：

| 字段 | 值 |
|------|-----|
| `config_type` | `auth` |
| `config_key` | `access_token` |
| `config_value` | `{"bk-hcm": "<token>", "<other-user>": "<token>"}` |

`config_value` 为 JSON 对象，**key 为 virtual-user 名称**，value 为对应 access_token。同一配置可维护多个 virtual-user 的 token，便于环境隔离或账号轮换。

**agent_server.yaml 约定**（选用哪个 virtual-user）：

```yaml
tools:
  toolProxy:
    enabled: true
    initVirtualUser: bk-hcm   # 首次加载 tool / 定时刷新时使用的 virtual-user，对应 global_config access_token 中的 key
    topN: 5                   # search_tools 默认返回数量和最大返回数量
    embedding:
      model: hunyuan-embedding
      dimensions: 1536
    refreshInterval: 30m
    required: true
```

**读取流程**：

```go
// cmd/agent-server/logics/toolproxy/config.go

func LoadInitAccessToken(cli *dsset.Client, virtualUser string) (string, error) {
    // 1. 调用 data-service Global.GlobalConfig.List
    //    filter: config_type=auth, config_key=access_token
    // 2. 解析 config_value 为 map[string]string
    // 3. 按 virtualUser（来自 cc.AgentServer().Tools.ToolProxy.InitVirtualUser）取 token
    // 4. virtualUser 为空或 key 不存在时返回明确错误
}
```

**理由**：
- 与项目 auth 类 global_config 约定一致，access_token 集中管理
- 多 virtual-user 共用一个 config 条目，避免为每个账号单独建 key
- `initVirtualUser` 放 yaml：非敏感、可按环境差异化配置（dev/staging/prod 使用不同 virtual-user）

**运维要求**：
- 部署前须在 `global_config` 预置 `auth/access_token`，且包含 `initVirtualUser` 对应 key 的有效 token
- 切换 virtual-user 只需改 yaml 中的 `initVirtualUser` 并重启（或配合定时刷新重新读取）
- token 过期时更新 DB 中对应 virtual-user 的 value；日志禁止打印完整 token

**替代方案**：
- virtual-user 选择也写入 global_config（拒绝：运维需同时改 DB + yaml 才能切换，且非敏感配置放 yaml 更直观）
- 环境变量注入 token（拒绝：与 global_config 统一管理方式不一致）

---

### D11：工具注册表定时刷新

**选择**：启动构建完成后，可选启动后台定时任务，使用同一 `access_token` 重新拉取 MCP 工具列表并增量更新 Registry 与 VectorStore。

**配置**（`agent_server.yaml`）：

```yaml
tools:
  toolProxy:
    enabled: true
    initVirtualUser: bk-hcm  # 定时刷新时使用的 virtual-user token（见 D10）
    topN: 5
    embedding:
      model: hunyuan-embedding
      dimensions: 1536
    refreshInterval: 30m     # 0 表示不刷新
    required: true
```

**刷新流程**：

```
定时器触发
  ↓
重新读取 global_config auth/access_token，按 initVirtualUser 取 token（支持 token 轮换）
  ↓
loadTools(initCtx) — 拉取最新 MCP 工具列表
  ↓
diff 与当前 Registry（新增 / 删除 / 变更）
  ↓
registry 增量更新 + index 全量/增量重建
  ↓
refreshMu 写锁保护，刷新期间 search_tools 短暂等待
```

**增量策略（MVP）**：
- 工具新增：注册到 Registry + 向量化加入 VectorStore
- 工具删除：从 Registry + VectorStore 移除
- 工具描述/schema 变更：更新 Registry + 重新向量化

**理由**：
- MCP Server 侧工具列表可能动态增减，定时刷新保证索引与线上一致
- 避免每次用户请求触发 MCP list_tools，降低运行时开销
- 全量重建向量索引在工具 <100 时可接受（<5s）；工具量大时阶段 2 优化为增量向量化

**替代方案**：仅在启动时加载一次（拒绝：MCP 工具变更需重启 agent-server 才能感知）

---

### D9：Graph 模式停用 `dynamicToolLoading`

**选择**：当 Graph 模式启用 Tool Proxy 时，`runtime.New()` 不再构建 `dynamicToolFilter`，`service.makeRunOptionResolver` 不注入 `agent.WithToolFilter`。

**理由**：Proxy 已将 MCP 收敛为 3 个元工具，ToolFilter 无过滤对象；避免两套检索逻辑并存造成困惑。

**LLM Agent 模式**：MVP 不改；仍可按配置使用 `dynamicToolLoading`，直至阶段 2 接入 Tool Proxy。

---

### D7：search_tools 返回格式 —— 直接返回 schema，减少交互次数

**选择**：`search_tools` 返回结果中直接包含工具的完整 schema，Agent 无需额外调用其他工具获取 schema。

**返回格式**：

```json
{
  "success": true,
  "tools": [
    {
      "name": "search_code",
      "description": "在代码库中搜索代码",
      "relevance_score": 0.95,
      "category": "code",
      "schema": {
        "type": "object",
        "properties": {
          "query": {
            "type": "string",
            "description": "搜索关键词"
          }
        },
        "required": ["query"]
      },
      "examples": [
        {
          "query": "函数 main"
        }
      ]
    }
  ],
  "total": 1
}
```

**理由**：
- 减少交互次数，Agent 无需额外调用其他工具获取 schema
- Schema 在向量化时已存储到 `ToolMetadata`，无需额外查询
- 降低延迟和成本

**替代方案**：`search_tools` 只返回工具名称和描述，Agent 需要时再调用其他工具获取 schema（拒绝：增加交互次数和延迟）

---

### D8：错误处理策略 —— 返回结构化错误，不抛出 panic

**选择**：元工具执行失败时返回结构化错误结果（`success=false`, `error` 对象），不抛出 panic 或返回 error。

**错误类型**：

| 错误类型 | 说明 | 示例 |
|----------|------|------|
| `schema_token_invalid` | schema_token 缺失或无效 | LLM 未先调用 `search_tools`/`get_tool_schema` 即尝试 `execute_tool` |
| `invalid_parameters` | 参数验证失败（含未知 key） | `query` 是必需参数但未提供；或传入了 schema 中未定义的字段 |
| `tool_not_found` | 工具不存在 | 工具 `nonexistent_tool` 未在注册表中 |
| `execution_error` | 工具执行失败 | MCP Server 返回错误 |
| `permission_denied` | 用户权限不足 | `execute_tool` 时 bk_ticket 无权调用目标工具（见 D16） |
| `embedding_error` | 向量化失败 | Embedding 服务不可用 |

**返回格式（含 schema 的错误）**：

当 `schema_token_invalid` 或 `invalid_parameters` 时，错误响应中附带工具完整 schema，LLM 可直接阅读并修正参数，无需额外调用 `get_tool_schema`：

```json
{
  "success": false,
  "error": {
    "type": "invalid_parameters",
    "message": "参数 \"hallucinated_field\" 不在工具 schema 定义中，请重新阅读 schema。完整 schema 已附在 required_schema 中，请按 schema 格式重新构造 parameters 并携带有效 schema_token 后重试。",
    "required_schema": {
      "type": "object",
      "properties": {
        "query": { "type": "string", "description": "搜索关键词" }
      },
      "required": ["query"]
    }
  }
}
```

**普通错误格式**（`tool_not_found`、`execution_error`、`permission_denied`）：

```json
{
  "success": false,
  "error": {
    "type": "execution_error",
    "message": "MCP Server 连接超时"
  }
}
```

**理由**：
- 结构化错误便于 Agent/LLM 理解错误原因并决定下一步行动
- 不抛出 panic 保证整个 graph 不会崩溃（符合 `trpc-agent-go` 框架的错误处理约定）
- 错误中内嵌 `required_schema` 减少 LLM 纠错所需的交互轮次（兜底保底，即使 LLM 跳过 schema 获取步骤，也能在错误响应中立即得到 schema）

**替代方案**：执行失败时返回 `error`（拒绝：框架会将 error 视为工具执行失败，可能导致 Agent 中断）

---

## Risks / Trade-offs

### R1：向量化成本高

**风险**：每次初始化时对所有工具进行向量化，若工具数量多（>1000），初始化时间可能过长（10-30 秒）。

**缓解**：
- MVP 阶段工具数量少（<100），向量化时间 < 5 秒
- 后续迭代引入向量缓存（将向量持久化到文件或 Redis），避免重复向量化
- 使用并发向量化（`tool.EmbeddingIndex` 已实现 `embeddingBuildConcurrency=8` 的并发控制）

### R2：语义搜索准确率不足

**风险**：Embedding 模型的语义理解能力有限，可能导致 Top-5 准确率 < 60%。

**缓解**：
- MVP 阶段接受准确率 > 60% 即可（需求文档验收标准）
- 返回足够多的候选（topK=5），Agent 可根据 description 进一步筛选
- 后续迭代引入混合检索（语义 + 关键词 + 使用统计），提升准确率到 > 80%

### R3：execute_tool 查找实际工具的性能

**风险**：`findActualTool()` 遍历所有 ToolSet 查找工具，若工具数量多（>500），查找时间可能过长（10-50ms）。

**缓解**：
- MVP 阶段工具数量少（<100），查找时间 < 5ms，可接受
- 后续迭代引入工具名 → ToolSet 索引（`map[string]tool.Tool`），将查找时间降到 O(1)

### R4：元工具增加交互次数

**风险**：传统模式 Agent 直接看到所有工具，无需额外搜索。`search_tools` 增加了 1 次 LLM 调用（Agent 先调用 `search_tools`，再调用 `execute_tool`）。

**缓解**：
- 这是"工具爆炸"问题的必要权衡：用 1 次额外 LLM 调用换取上下文窗口节省 10k-25k tokens
- `search_tools` 直接返回 schema（D7），避免额外调用 `get_tool_schema`，实际只增加 1 次调用
- 后续迭代可引入"常用工具缓存"机制，Agent 无需每次都调用 `search_tools`

### R5：启动构建失败处理

**风险**：启动时 `global_config` 中无 `auth/access_token`、所选 `initVirtualUser` 无对应 token、token 过期、Embedding 不可用或 MCP 拉取工具失败，导致 `Build()` 失败。

**缓解**：
- `access_token` 缺失或无效：启动时 `logs.Errorf` 并拒绝启动 Tool Proxy（或整体 agent-server 启动失败，取决于配置 `toolProxy.required`）
- `buildOK=false` 时，元工具返回 `success=false` + `embedding_error` / `tool_not_available`
- Graph 模式可选降级：`toolProxy.required=false` 时 warn 并回退为直接注册 `toolset.TS`（与旧行为一致）
- 定时刷新失败：记录 warn 日志，保留上次成功构建的索引继续服务，下次定时器重试

### R6：access_token 安全与过期

**风险**：`access_token` 存于 DB，可能过期或被泄露；初始化 token 权限可能与用户 `bk_ticket` 可见工具范围不一致。

**缓解（MVP 接受度，见 D16）**：
- `access_token` 仅用于 list_tools / 索引构建，不用于 `execute_tool` 实际调用
- **MVP 接受「搜得到但执行可能失败」**：索引为服务账号超集；用户 `execute_tool` 若因权限被拒，返回 `execution_error`，`message` 明确提示「当前用户无权限执行该工具，请联系管理员或确认工具可见范围」
- 运维文档明确 token 轮换流程；定时刷新前可选重新读取 global_config
- 初始化 token 应使用具备全部 MCP ToolSet 读权限的服务账号
- 日志中禁止打印完整 token（仅打印前 4 位 + `***`）

---

### D12：Prompt 工程 —— system_prompt 元工具工作流说明

**选择**：在 `cmd/agent-server/etc/prompts/system_prompt.md` 中明确「元工具标准三步工作流（强制）」，与代码侧 schema_token 机制同步。

**内容要点**（已实现）：
- 移除原先"场景A/B"条件分支结构（该结构给 LLM 提供了"已知工具名可跳过 get_tool_schema"的豁免出口）
- 改为**无条件三步工作流**：步骤 1 获取 schema 和 schema_token → 步骤 2 阅读 schema → 步骤 3 携带 schema_token 执行
- 明确 `schema_token` 是 `execute_tool` 的**必填参数**，由步骤 1 返回，无法猜测或伪造
- 禁止行为措辞从"可能因参数缺失而失败"改为"缺少有效 token 时系统会直接拒绝并返回 schema"
- 补充：传入 schema 未定义的参数 key 会被识别为幻觉参数并拒绝执行

**Prompt 与工程层的协同机制**：

| 层次 | 机制 | 作用 |
|------|------|------|
| System Prompt | 明确三步工作流 + schema_token 说明 | 主动引导 LLM 正确行为 |
| `execute_tool` Declaration | `schema_token` 列为 `required` 字段 | LLM 从 tool description 看到必填约束 |
| `execute_tool` Call() | server 端 HMAC 验证 schema_token | 强制兜底：即使 LLM 跳过步骤 1，也会被拒绝 |
| 错误响应 | 错误中内嵌完整 `required_schema` | 被拒绝后 LLM 可立即在本轮获取 schema 纠错 |

**理由**：仅靠元工具 `Description` 不足以约束 LLM 行为；仅靠 system prompt 属于软约束，LLM 可能忽略；三层协同（prompt + declaration + server 验证）才能形成真正的硬性约束。

---

### D13：Graph 集成 —— 方案 B，独立 proxy ToolSet

**选择**：3 个元工具封装为独立 `tool.ToolSet`，通过 `graph.WithToolSets([]tool.ToolSet{proxyToolSet})` 注册到 LLM Node 与 Tool Node；**不**并入 `skillTools` map。

**理由**：
- skill/HITL 与 MCP 代理职责分离，map 仅保留 skill + `human_confirm`
- LLM / Tool Node 对称注册，避免「能看见但执行不了」
- 与框架现有 `WithToolSets` 机制一致，改动面小

**替代方案 A**：元工具放进 `skillTools` map（拒绝：混淆 skill 与 MCP 代理边界）

---

### D14：关闭 `WithRefreshToolSetsOnRun`

**选择**：Tool Proxy 启用时，LLM Node / Tool Node **不再**设置 `graph.WithRefreshToolSetsOnRun(true)`。

**理由**：
- 元工具为进程内静态注册，无 per-run 刷新需求
- MCP 实际调用在 `execute_tool` 内部完成，使用请求 ctx 的 `bk_ticket`
- 避免每次 Run 无意义 refresh proxy ToolSet

**代码注释要求**（实施时写入 `graph_build.go`）：
```go
// Tool Proxy 元工具为静态注册，无需 WithRefreshToolSetsOnRun。
// MCP 实际工具在 execute_tool 内部按需加载，使用请求 ctx 中的 bk_ticket 鉴权。
```

---

### D15：MVP 不抽 VectorStore 接口

**选择**：MVP 阶段 `ToolProxy` 直接持有 `*tool.EmbeddingIndex` + `*ToolRegistry`；不创建 `VectorStore` 接口、`vectorstore.go`、`EmbeddingVectorStore`。

**阶段 2**：若引入 Faiss/Chroma，再抽取 `VectorStore` 接口并替换 `index` 字段。

**理由**：YAGNI；现有 `EmbeddingIndex` 已满足 MVP 语义检索需求。

---

### D16：权限不一致 —— MVP 接受搜得到、执行可能失败

**选择**：不实现 per-user Registry 过滤。`search_tools` 基于服务账号 `access_token` 构建的全量索引；`execute_tool` 用用户 `bk_ticket`，权限不足时返回结构化错误并提示权限问题。

**理由**：per-user 索引成本高，MVP 优先打通主流程；超集索引 + 执行时鉴权是常见模式。

---

### D17：元工具 Description 分工

**选择**：在 3 个元工具的 `Declaration().Description` 中写清使用场景，与 system_prompt 互补（prompt 为强制规则，description 为工具选择时的快速指引）。

| 元工具 | Description 要点 |
|--------|------------------|
| `search_tools` | **探索未知工具时使用**；用户未指定工具名、或不确定用哪个工具时调用；返回含 schema 和 schema_token 的候选列表 |
| `get_tool_schema` | **已知工具名时使用**；已明确工具名时调用；返回完整 schema 和 schema_token |
| `execute_tool` | **执行实际 MCP 工具**；必须先通过 `search_tools` 或 `get_tool_schema` 获取 schema 和 schema_token 后再调用；`schema_token` 是必填字段；禁止猜测参数或传入 schema 未定义的字段 |

**理由**：减少 LLM 冗余调用（如已知工具名仍 search）或跳过 schema 直接 execute 导致失败。`schema_token` 写入 Description 的 required 字段，LLM 从声明层即可感知强制约束。

---

### D18：Schema Token 机制 —— 工程层强制 LLM 先获取 schema

**背景**：System Prompt 和 tool Description 均属软约束，LLM 仍可能在已知工具名时直接调用 `execute_tool` 而跳过 schema 获取步骤，导致参数猜测错误或引入幻觉参数。

**选择**：引入无状态 HMAC Schema Token 机制，从工程层强制 LLM 必须先调用 `search_tools` 或 `get_tool_schema`。

**设计**：

```
token = HMAC-SHA256(toolName + json(schema), serverSecret)[:16 hex chars]
```

- `serverSecret`：进程启动时由 `crypto/rand` 生成，存于 `ToolProxy.schemaTokenSecret`，随进程生命周期
- `search_tools` 和 `get_tool_schema` 均在返回值中附带每个工具的 `schema_token`
- `execute_tool` 将 `schema_token` 列为 `required` 必填字段，server 端验证 HMAC
- token 无状态、不依赖 session 存储，服务重启后已有 token 失效（LLM 重新调用 step 1 即可）

**验证逻辑**：

```go
// proxy.go
func (p *ToolProxy) verifySchemaToken(toolName string, schema map[string]interface{}, token string) bool {
    expected := p.generateSchemaToken(toolName, schema)
    return hmac.Equal([]byte(expected), []byte(token))
}
```

**拒绝响应**：token 无效时返回带完整 `required_schema` 的结构化错误，LLM 可在本轮直接获取 schema 并重试：

```json
{
  "success": false,
  "error": {
    "type": "schema_token_invalid",
    "message": "schema_token 无效，请先调用 search_tools 或 get_tool_schema 获取最新 schema 和 token。完整 schema 已附在 required_schema 中...",
    "required_schema": { ... }
  }
}
```

**三层约束协同**：

```
Prompt（软引导） + Declaration required（声明约束） + HMAC 验证（工程强制）
```

**理由**：
- Token 是唯一不可伪造的约束——LLM 必须实际调用 step 1 才能拿到有效 token
- 无状态设计不引入 session 存储，实现简单、无锁竞争
- 拒绝响应内嵌 schema 作为保底，即使 LLM 被迫跳过 step 1，也能在一轮内得到 schema 并修正

**替代方案**：
- Session 级别追踪（拒绝：需要 session 状态存储，增加服务复杂度）
- 仅靠 prompt 约束（拒绝：软约束，LLM 会优化掉；已通过实测验证不可靠）
