# MCP 工具代理（Tool Proxy）技术方案

> **文档版本**：v1.0
> **适用范围**：agent-server Graph 模式（MVP 阶段）
> **状态**：方案设计完成，待实施

---

## 1. 概述

MCP 工具代理（Tool Proxy）是 agent-server 引入的一项 Agent 工具管理能力，旨在解决 **MCP 工具数量规模化增长** 带来的上下文窗口占用、工具选择准确率下降和响应延迟增加等问题。

核心思路：将 N 个实际 MCP 工具抽象为 **3 个元工具**（`search_tools`、`get_tool_schema`、`execute_tool`），Agent 只需理解这 3 个固定接口，即可通过语义搜索动态发现并调用任意数量的 MCP 工具。

**预期收益**：

| 指标 | 现状（50+ 工具） | 目标 |
|------|-----------------|------|
| 工具定义 Token 占用 | 10k–25k tokens | ~500 tokens（3 个元工具） |
| Token 消耗降幅 | — | **90%+** |
| 语义搜索延迟（P95） | — | **< 500ms** |
| 对外 API / 前端协议 | — | **无变更** |

---

## 2. 背景与问题

### 2.1 现状

agent-server 当前通过 `BuildMCPToolSets()` 构建 MCP ToolSet，并将所有工具通过 Graph Agent 的 `WithToolSets` 一次性注册到 LLM 上下文。随着接入的 MCP 工具数量增加（预计达到 50+），出现以下问题：

1. **Token 消耗激增**：每个工具定义占用 200–500 tokens，50 个工具即占用 10k–25k tokens，严重挤占对话上下文
2. **工具选择准确率下降**：工具列表过长时，LLM 难以做出正确选择，出现「选择瘫痪」
3. **响应延迟增加**：冗长的工具列表导致 LLM 推理时间增加
4. **维护困难**：每新增工具都需重新评估上下文管理策略

### 2.2 现有方案的不足

项目曾引入 `dynamicToolLoading` 机制：在每次 Run 时用用户消息做向量/BM25 检索，静默过滤工具列表。实测存在以下问题：

| 维度 | dynamicToolLoading | 问题 |
|------|-------------------|------|
| 检索触发 | 框架 ToolFilter，用**用户原话**静默过滤 | 短句/指代（「继续」「可以」）导致 query 质量差 |
| LLM 可见工具 | Top-N 个真实 MCP schema | 仍占用大量 Token |
| 可观测性 | 检索过程对 LLM 不可见 | 难以调试和优化 |

Tool Proxy 将检索与调用意图分离：由 LLM **显式调用** `search_tools` 并主动构造检索 query，可观测性更好，上下文占用更低。

---

## 3. 方案概述

### 3.1 核心思想

```
传统模式：Agent 上下文 ← [Tool₁, Tool₂, ..., Tool₅₀]  （N 个完整 schema）

Tool Proxy：Agent 上下文 ← [search_tools, get_tool_schema, execute_tool]  （固定 3 个元工具）
                              ↓
                    语义搜索 → 按需发现 → 按需执行
```

Agent 的工作流从「直接选择工具」变为「搜索 →（可选查 schema）→ 执行」三步流程，但上下文窗口始终只承载 3 个元工具的定义。

### 3.2 三个元工具

| 元工具 | 职责 | 典型场景 |
|--------|------|----------|
| `search_tools` | 根据任务描述进行语义搜索，返回最相关工具列表（含完整 schema） | 用户说「帮我在代码中搜索 TODO」 |
| `get_tool_schema` | 按工具名称查询完整 JSON Schema | 已知工具名，需确认参数格式 |
| `execute_tool` | 按工具名称和参数调用实际 MCP 工具 | 搜索到目标工具后执行 |

> **设计要点**：`search_tools` 直接返回完整 schema（D7），Agent 通常无需额外调用 `get_tool_schema`，减少一轮交互。

### 3.3 端到端示例

用户输入：「帮我在代码中搜索包含 TODO 的文件」

```
用户消息
  ↓
LLM 调用 search_tools(query="在代码中搜索包含 TODO 的文件")
  ↓
返回 [{ name: "search_code", schema: {...}, relevance_score: 0.95 }]
  ↓
LLM 调用 execute_tool(tool_name="search_code", parameters={"query": "TODO"})
  ↓
返回搜索结果 → 呈现给用户
```

---

## 4. 架构设计

### 4.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Agent（Graph 模式）                      │
│  LLM Node / Tool Node                                       │
│  可见工具：[search_tools, get_tool_schema, execute_tool]     │
│            + skill 工具 + HITL 工具                           │
└──────────────────────────┬──────────────────────────────────┘
                           │ 元工具调用
┌──────────────────────────▼──────────────────────────────────┐
│                    toolproxy 模块                            │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │ ToolRegistry│  │ VectorStore  │  │ 元工具实现           │ │
│  │ 工具元数据   │  │ 语义检索      │  │ search/schema/exec │ │
│  │ 注册/查询    │  │ EmbeddingIndex│  │                     │ │
│  └──────┬──────┘  └──────┬───────┘  └──────────┬──────────┘ │
│         │                │                      │            │
│         └────────────────┴──────────────────────┘            │
│                          │                                   │
└──────────────────────────┼───────────────────────────────────┘
                           │ execute_tool 调用
┌──────────────────────────▼──────────────────────────────────┐
│              MCP ToolSet（现有，不变）                        │
│  BuildMCPToolSets() → 各 MCP Server 实际工具                  │
│  含 bkaidev 认证注入、回调逻辑                                 │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 模块职责

新增 `cmd/agent-server/logics/toolproxy/` 包，包含 7 个文件：

| 文件 | 职责 |
|------|------|
| `types.go` | 公共类型定义 |
| `registry.go` | 工具注册表管理 |
| `vectorstore.go` | 向量化存储和语义检索 |
| `search.go` / `schema.go` / `execute.go` | 3 个元工具实现 |
| `config.go` | 从 global_config 读取初始化 access_token |

### 4.3 复用现有组件

| 组件 | 用途 |
|------|------|
| `tool/tool_index.go` — `ToolMeta`、`EmbeddingIndex` | 工具元数据提取与向量语义检索 |
| `tool/tool.go` — `BuildMCPToolSets()` | MCP ToolSet 构建（含 bkaidev 认证） |
| `trpc-agent-go` — `tool.Tool` 接口 | 元工具标准注册与执行 |
| Runner / SessionService / AG-UI SSE | 服务层完全复用，无变更 |

### 4.4 数据流与单一数据源原则

```
VectorStore.Search()  →  返回 toolName + relevanceScore
        ↓
registry.GetTool(toolName)  →  获取最新 ToolMetadata（含 schema）
        ↓
组装结果返回给 Agent
```

**设计原则**：`ToolRegistry` 是工具元数据的唯一权威来源；`VectorStore` 仅存储 toolName 与向量，不缓存 metadata 拷贝，避免数据同步问题和并发读写风险。

---

## 5. 核心设计

### 5.1 工具注册表（ToolRegistry）

复用现有 `tool.ToolMeta` 结构，扩展为 `ToolMetadata`：

```go
type ToolMetadata struct {
    tool.ToolMeta                              // Name, Description, Parameters, Tags, SearchText
    Category    string                         // 工具类别（code/file/web 等）
    Schema      map[string]interface{}         // 完整 JSON Schema
    Examples    []map[string]interface{}       // 调用示例
    UsageCount  int                            // 预留：使用统计（阶段 3）
    SuccessRate float64                        // 预留：成功率（阶段 3）
}
```

- 并发安全：`sync.RWMutex` 保护读写
- 工具命名：与现有 `LazyToolIndex` 一致，采用 `{toolsetName}_{rawName}` 前缀规则

### 5.2 向量存储（VectorStore）

MVP 基于内存 `EmbeddingIndex` 实现，接口抽象便于后续接入 Faiss / Chroma 等外部向量库：

```go
type VectorStore interface {
    Add(ctx context.Context, toolName string, vector []float64) error
    Search(ctx context.Context, queryVector []float64, topK int) ([]*ToolSearchResult, error)
    Delete(ctx context.Context, toolName string) error
    Build(ctx context.Context, registry *ToolRegistry, emb embedder.Embedder) error
}
```

默认参数（MVP 硬编码）：`topK=5`，`scoreThreshold=0.3`。

### 5.3 元工具接口规范

三个元工具均实现 `tool.Tool` 接口，通过标准工具注册机制集成到 Graph Agent。

#### search_tools

**输入**：

```json
{
  "query": "在代码中搜索包含 TODO 的文件",
  "top_k": 5,
  "category": "code"
}
```

**输出**：

```json
{
  "success": true,
  "tools": [
    {
      "name": "search_code",
      "description": "在代码库中搜索代码",
      "relevance_score": 0.95,
      "category": "code",
      "schema": { "type": "object", "properties": { "query": { "type": "string" } }, "required": ["query"] },
      "examples": [{ "query": "函数 main" }]
    }
  ],
  "total": 1
}
```

#### get_tool_schema

**输入**：`{ "tool_name": "search_code" }`
**输出**：指定工具的完整 JSON Schema。

#### execute_tool

**输入**：

```json
{
  "tool_name": "search_code",
  "parameters": { "query": "TODO" }
}
```

**输出**：

```json
{
  "success": true,
  "result": { /* MCP 工具执行结果 */ }
}
```

### 5.4 错误处理

元工具执行失败时返回结构化错误，不抛出 panic，保证 Graph 不中断：

| 错误类型 | 说明 |
|----------|------|
| `invalid_parameters` | 参数验证失败 |
| `tool_not_found` | 工具不存在 |
| `execution_error` | MCP 工具执行失败 |
| `embedding_error` | 向量化服务不可用 |

```json
{
  "success": false,
  "error": {
    "type": "invalid_parameters",
    "message": "参数 query 是必需的",
    "suggestion": { "query": "请提供搜索关键词，例如 'TODO'" }
  }
}
```

### 5.5 初始化：启动时预加载（global_config access_token）

原方案采用首次 Run 懒加载（`sync.Once` + 用户 `bk_ticket`），首请求需等待 5–30s 向量化。优化后改为 **启动时预加载**：从 `global_config` 读取服务账号 `access_token`，在 agent-server 启动阶段完成工具加载与向量化。

**鉴权分离**：

| 场景 | X-Bkapi-Authorization 内容 | 来源 |
|------|---------------------------|------|
| 启动预加载 / 定时刷新 | `{"access_token": "z"}` | global_config `auth/access_token`，按 `initVirtualUser` 取 key |
| 用户 Run 执行工具 | `{"bk_app_code":"x","bk_app_secret":"y","bk_username":"...","bk_ticket":"z"}` | 请求 Cookie |

索引构建与用户执行鉴权分离：预加载用服务 token 拉取工具列表，实际 `execute_tool` 仍走用户 `bk_ticket`，权限边界不变。

**global_config 配置**（凭据，按 virtual-user 分 key）：

| 字段 | 值 |
|------|-----|
| `config_type` | `auth` |
| `config_key` | `access_token` |
| `config_value` | `{"bk-hcm": "<token>"}` |

**agent_server.yaml 配置**（选用 virtual-user）：

```yaml
tools:
  toolProxy:
    initVirtualUser: bk-hcm   # 对应 config_value 中的 key
    topN: 5
    embedding:
      model: hunyuan-embedding
      dimensions: 1536
```

**启动流程**：

```
agent-server 启动
  ↓
读取 initVirtualUser（yaml）+ global_config auth/access_token
  ↓
按 virtual-user key 取出 token → 构造 initCtx
  ↓
ToolProxy.Build(initCtx)
  ├── loadTools(initCtx) — 拉取 MCP 工具到 Registry
  └── vectorStore.Build() — 批量向量化
  ↓
StartRefreshLoop（可选，默认 30m）
  ↓
Graph 注册 3 个元工具 → 就绪，首请求无额外延迟
```

**定时刷新**（可选）：后台任务按 `refreshInterval` 重新拉取 MCP 工具列表，增量更新 Registry 与 VectorStore，感知 MCP Server 侧工具变更，无需重启。

**失败处理**：
- `access_token` 缺失/过期：启动失败或降级（取决于 `toolProxy.required` 配置）
- 定时刷新失败：保留上次成功索引继续服务，下次重试

---

## 6. 与现有机制的关系

### 6.1 Graph 模式变更

| 变更项 | 说明 |
|--------|------|
| LLM Node 工具列表 | 3 个元工具 + skill 工具 + HITL 工具，不再暴露全量 MCP schema |
| Tool Node 工具列表 | 同上 |
| `dynamicToolLoading` | Graph 模式下停用（与 Tool Proxy 目标重叠） |
| AG-UI SSE 协议 | 无变更，对前端完全透明 |

### 6.2 配置项

**agent_server.yaml**：

```yaml
tools:
  toolProxy:
    enabled: true
    initVirtualUser: bk-hcm  # 首次加载 / 定时刷新使用的 virtual-user，对应 global_config access_token 中的 key
    topN: 5                  # search_tools 默认返回数量和最大返回数量
    embedding:
      model: hunyuan-embedding
      dimensions: 1536
    refreshInterval: 30m     # 工具索引定时刷新间隔，0 表示不刷新
    required: true           # true=启动构建失败则拒绝启动；false=降级为全量 MCP 注册
```

**global_config 表**（运维预置）：

| config_type | config_key | config_value |
|-------------|------------|--------------|
| `auth` | `access_token` | `{"bk-hcm":"<token>"}` |

### 6.3 不变的部分

- MCP ToolSet 构建逻辑（扩展 bkaidev hook 支持 `access_token` fallback）
- `execute_tool` 通过 `tool.Tool.Execute()` 调用实际工具，复用完整 MCP 调用链路
- Runner、SessionService、MemoryService 等服务层
- 对外 API 和前端交互协议

---

## 7. 实施计划

### 7.1 阶段划分

| 阶段 | 范围 | 状态 |
|------|------|------|
| **阶段 1（MVP）** | Graph 模式 + 3 元工具 + 内存向量检索 | 当前 |
| **阶段 2** | LLM Agent 模式接入；混合检索；智能错误处理 | 规划中 |
| **阶段 3** | 工具使用统计与推荐；外部向量数据库 | 规划中 |

### 7.2 MVP 实施任务概览

1. **基础模块**：`toolproxy` 包类型定义、注册表、向量存储
2. **元工具实现**：`search_tools`、`get_tool_schema`、`execute_tool`
3. **Graph 集成**：`graph_build.go`、`runtime.go`、`service.go` 改造
4. **测试验证**：单元测试 + Graph 模式端到端集成测试

### 7.3 验收标准

- [ ] Graph 模式下 Agent 可通过「search_tools → execute_tool」完成 MCP 工具调用
- [ ] Token 消耗较全量工具注册降低 90%+
- [ ] 语义搜索 Top-5 准确率 > 60%（MVP 可接受阈值）
- [ ] 搜索延迟 P95 < 500ms
- [ ] 对外 API / AG-UI 协议无变更
- [ ] bkaidev MCP + 真实 `bk_ticket` 端到端流程可跑通

---

## 8. 风险与应对

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| **向量化初始化耗时** | 工具 >1000 时初始化 10–30s | MVP 工具 <100，耗时 <5s；后续引入向量缓存 |
| **语义搜索准确率不足** | Top-5 准确率可能 <60% | 返回足够候选供 LLM 二次筛选；阶段 2 引入混合检索 |
| **execute_tool 查找性能** | O(N×M) 遍历 ToolSet | MVP 工具 <100，查找 <5ms；后续构建 O(1) 索引 |
| **元工具增加交互轮次** | 多 1 次 LLM 调用 | 用 1 次调用换取 10k–25k tokens 节省；search 直接返回 schema |
| **启动构建失败** | `auth/access_token` 缺失或 `initVirtualUser` 无对应 token | `required=false` 时降级；运维文档明确 token 与 virtual-user 配置 |
| **access_token 安全** | token 泄露或权限范围不一致 | 仅用于索引构建；日志脱敏；execute 仍用用户 bk_ticket |
| **定时刷新一致性** | 刷新期间索引短暂不一致 | 写锁保护 + 失败保留上次成功索引 |

---

## 9. 后续演进

```
阶段 1（MVP）
  ├── Graph 模式 3 元工具
  ├── 内存 EmbeddingIndex
  └── 停用 dynamicToolLoading（Graph）

阶段 2
  ├── LLM Agent 模式接入
  ├── 混合检索（语义 + 关键词 + 使用统计）
  ├── 智能错误处理与参数建议增强
  └── 向量持久化缓存（减少重启后重建耗时）

阶段 3
  ├── 工具使用统计与智能推荐
  ├── 常用工具缓存（跳过 search 步骤）
  └── 外部向量数据库（Faiss / Chroma / Milvus）
```

配置扩展：`agent_server.yaml` 的 `tools.toolProxy` 段已支持 `refreshInterval`、`topN` 与 `embedding`；后续可扩展 `scoreThreshold`、检索策略等参数。

---

## 10. 总结

MCP 工具代理（Tool Proxy）通过 **「3 个元工具 + 语义搜索 + 按需执行」** 的模式，在不影响对外 API 和前端体验的前提下，解决了 Agent 工具规模化带来的上下文膨胀和选择准确率问题。

**关键优势**：

1. **极简上下文**：N 个工具 → 3 个元工具，Token 消耗降低 90%+
2. **智能发现**：LLM 主动构造检索 query，比静默过滤更准确、更可观测
3. **首请求无延迟**：启动时预加载 + 向量化，消除懒加载带来的 5–30s 首请求等待
4. **平滑集成**：复用现有 MCP ToolSet、EmbeddingIndex、Graph Agent 框架，改动范围可控
5. **渐进演进**：MVP 聚焦 Graph 模式，后续逐步扩展 LLM Agent 模式与高级检索能力

---

## 附录

### A. 相关文档

| 文档 | 说明 |
|------|------|
| `proposal.md` | 变更提案（Why / What / Impact） |
| `design.md` | 详细技术设计（Decisions D1–D11） |
| `specs/spec.md` | 能力规格与验收场景 |
| `tasks.md` | 实施任务清单 |

### B. 术语表

| 术语 | 说明 |
|------|------|
| MCP | Model Context Protocol，模型上下文协议 |
| ToolSet | MCP 工具集合，由 agent-server 配置构建 |
| 元工具 | 代理层暴露给 Agent 的 3 个固定工具接口 |
| ToolRegistry | 工具元数据注册表，运行时工具信息的唯一数据源 |
| VectorStore | 工具向量存储与语义检索抽象 |
| Graph 模式 | agent-server 的 Graph Agent 运行模式（`agui.mode=graph`） |
| dynamicToolLoading | 现有动态工具加载机制，Graph 模式下将被 Tool Proxy 替代 |
| access_token | virtual-user 级鉴权凭据，存于 global_config `auth/access_token`，用于启动预加载和定时刷新 |
| global_config | HCM 全局配置表，`auth/access_token` 按 virtual-user key 存储 token |
| initVirtualUser | yaml 配置项，指定首次加载 tool 使用的 virtual-user，对应 access_token map 的 key |
| virtual-user | 虚拟用户账号名，如 `bk-hcm`，作为 access_token JSON 的 key |
| refreshInterval | 工具索引定时刷新间隔，默认 30m |
