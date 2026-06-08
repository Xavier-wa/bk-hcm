## Why

当前 agent-server 将配置的 MCP 工具集（ToolSet）中的所有工具定义一次性加载到 Agent 上下文窗口。随着接入的 MCP 工具数量增加（预计达到 50+），会导致：

1. **Token 消耗激增**：每个工具定义占用 200-500 tokens，50 个工具占用 10k-25k tokens，挤占对话上下文
2. **工具选择准确率下降**：工具列表过长时，LLM 难以做出正确选择，出现"选择瘫痪"
3. **响应延迟增加**：工具列表过长导致 LLM 推理时间增加
4. **维护困难**：新增工具需要重新调整上下文管理策略

引入 **MCP 工具代理（Tool Proxy）** 机制，将 N 个实际工具抽象为 3 个"元工具"（search_tools、get_tool_schema、execute_tool），Agent 只需理解这 3 个元工具即可调用任意数量的 MCP 工具，实现极简上下文。

**替代 `dynamicToolLoading`**：现有动态工具加载在每次 Run 时用用户消息做向量/BM25 检索并静默过滤工具，实测召回准确性不足。Tool Proxy 改为由 LLM 在 `search_tools` 步骤中**主动构造检索 query**，再执行 `execute_tool`，检索与调用意图分离，可观测性更好。

## What Changes

本变更（阶段 1：MVP）实现核心功能的最小可行版本，**仅覆盖 Graph 模式**（`agui.mode=graph`）；LLM Agent 模式接入延后到阶段 2。

- **新增 `toolproxy` 模块**：位于 `cmd/agent-server/logics/toolproxy/` 包下，包含 7 个文件：
  - `types.go`：公共类型定义（`ToolMetadata`、`ToolRegistry`、`VectorStore` 接口等）
  - `registry.go`：工具注册表管理（注册、更新、删除、查询）
  - `vectorstore.go`：工具定义的向量化存储和语义检索（基于现有 `tool.EmbeddingIndex`）
  - `search.go`：实现 `search_tools` 元工具（语义搜索 + 返回 schema 和 schema_token）
  - `schema.go`：实现 `get_tool_schema` 元工具（查询工具注册表，返回 schema 和 schema_token）
  - `execute.go`：实现 `execute_tool` 元工具（schema_token 校验 + 严格参数验证 + 调用 MCP Client）
  - `config.go`：从 global_config 读取初始化 access_token

- **启动时预加载**：从 `global_config` 读取 `access_token`，启动阶段完成工具加载与向量化，支持定时刷新
- **扩展 bkaidev 鉴权 hook**：支持 `access_token` 与 `bk_ticket` 两种鉴权模式

- **Schema Token 强制机制**（见设计 D18）：
  - `search_tools` 和 `get_tool_schema` 返回值中附带每个工具的 `schema_token`（HMAC-SHA256，无状态）
  - `execute_tool` 将 `schema_token` 列为必填参数，server 端验证；无效时返回带完整 schema 的结构化错误
  - `execute_tool` 严格拒绝 schema 未定义的参数 key，防止 LLM 幻觉参数导致误执行
  - 所有参数校验失败均在错误响应中内嵌完整 `required_schema`，减少 LLM 纠错所需的交互轮次

- **修改 `agent/graph_build.go`**：Graph 模式下用 3 个元工具替代 `WithToolSets(mcpToolSets)` 对 LLM 的全量暴露；Tool Node 同样只注册元工具 + skill/HITL 工具
- **Graph 模式停用 `dynamicToolLoading`**：启用 Tool Proxy 时不再注入 `agent.WithToolFilter`（与 proxy 目标重叠且检索策略已弃用）

- **复用现有组件**：
  - `tool/tool_index.go` 中的 `ToolMeta` 类型和 `EmbeddingIndex` 实现（向量化存储和语义检索）
  - `tool/tool.go` 中的 `BuildMCPToolSets()` 逻辑（MCP ToolSet 构建）

## Capabilities

### New Capabilities

- `tool-proxy`: MCP 工具代理机制——3 个元工具（search_tools、get_tool_schema、execute_tool）替代 N 个实际工具，Agent 通过语义搜索动态发现相关工具并调用

### Modified Capabilities

（无需修改现有 spec，本次变更不涉及已有 spec 的行为变更）

## Impact

- **代码**：
  - 新增 `cmd/agent-server/logics/toolproxy/` 目录，包含 7 个 Go 文件（~900 行代码，含 config.go）
  - 修改 `cmd/agent-server/logics/auth/bkapi.go`：新增 access_token 鉴权支持
  - 修改 `cmd/agent-server/logics/tool/tool.go`：bkaidev hook 支持 access_token fallback
  - 修改 `cmd/agent-server/logics/agent/graph_build.go`：Graph 模式注册元工具
  - 修改 `cmd/agent-server/logics/runtime.go` / `service/service.go`：启动预加载 + Graph 模式跳过 DynamicToolFilter
  - **不修改** `agent_llm.go`（LLM Agent 模式，阶段 2）

- **依赖**：
  - 新增 `trpc.group/trpc-go/trpc-agent-go/tool` 包引用（`tool.Tool`、`tool.ToolSet` 等类型）
  - 复用已有依赖：`trpc.group/trpc-go/trpc-agent-go/agent/graphagent`、`embedder.Embedder`

- **配置**：
  - 新增 `agent_server.yaml` 中 `tools.toolProxy` 配置段（`enabled`、`initVirtualUser`、`refreshInterval`、`required`、`topN`、`embedding`）
  - 初始化 `access_token` 存储于 `global_config` 表（`config_type=auth`, `config_key=access_token`, `config_value={"bk-hcm":"<token>"}`）
  - `initVirtualUser` 指定首次加载 tool 时使用的 virtual-user，对应 `config_value` 中的 key

- **API**：
  - 无对外 API 变更，Agent 对外的 AG-UI SSE 协议层完全复用
  - 元工具通过 MCP 协议暴露给 Agent，对前端透明

- **基础设施**：
  - 无新增基础设施依赖（向量存储使用内存实现，基于现有 `EmbeddingIndex`）
  - 后续迭代可引入 Faiss/Chroma 等外部向量数据库

- **性能**：
  - Token 消耗降低 90%+（从 N 个工具定义降低到 3 个元工具定义）
  - 搜索延迟 < 500ms（P95，基于内存向量检索）
