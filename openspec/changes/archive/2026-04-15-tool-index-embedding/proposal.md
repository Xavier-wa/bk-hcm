## Why

现有 `ToolIndex` 只有基于关键词子串匹配（`KeywordIndex`）和 BM25 全文检索（`BM25Index`）两种策略，对于语义相近但用词不同的查询（如"查看主机列表" vs "list_cvm_instances"）召回效果差。通过新增基于向量余弦相似度的 `EmbeddingIndex`，复用已有的蓝鲸 aidev 网关 Embedding 能力，可以显著提升语义检索精度，使 Agent 在复杂多轮对话中更准确地匹配 MCP 工具。

## What Changes

- **新增** `EmbeddingIndex`：基于向量余弦相似度的 `ToolIndex` 实现，Build 阶段并发调用 Embedding API 生成工具向量，Search 阶段对 query 生成向量后与各工具向量逐一计算余弦相似度。
- **BREAKING** `ToolIndex` 接口增加 `context.Context` 参数：`Build(ctx, tools)` / `Search(ctx, query, topN, threshold)`；同步更新 `KeywordIndex`、`BM25Index`（忽略 ctx）及所有调用方。
- **新增** `buildEmbeddingClient(gatewayCfg, embedCfg)` 公共函数：从 `buildSQLiteVecMemoryService` 提取 openaiembed 客户端构建逻辑，memory 与 tool index 共用，消除重复代码。
- **扩展** `DynamicToolLoadingConfig`：增加 `Embedding EmbeddingConfig` 字段，供 `strategy: "embedding"` 时使用。
- **扩展** `buildToolIndex`：增加 `gatewayCfg BKAPIGatewayConfig` 参数，`strategy == "embedding"` 时返回 `NewEmbeddingIndex`；调用方（`runtime.go`）传入 aidev 默认网关配置。
- **新增** `agent_server.yaml` 中 `strategy: embedding` 配置示例注释。

## Capabilities

### New Capabilities

- `embedding-tool-index`：新增基于 Embedding 向量相似度的 ToolIndex 策略，包括接口 ctx 化改造、公共 embedder 构建函数抽取、EmbeddingIndex 实现（并发 Build + 余弦相似度 Search）、配置扩展与集成。

### Modified Capabilities

（无已有 spec 需变更）

## Impact

- **代码文件**：
  - `cmd/agent-server/logics/tool_index.go`：接口改造 + `EmbeddingIndex` 实现
  - `cmd/agent-server/logics/tool_filter.go`：`buildToolIndex` 签名变更 + `lazyToolIndex` ctx 透传
  - `cmd/agent-server/logics/runtime.go`：`DynamicToolLoadingConfig` 结构体扩展 + `buildEmbeddingClient` 提取 + 调用处变更
  - `cmd/agent-server/etc/agent_server.yaml`：配置示例补充
- **接口兼容性**：`ToolIndex` 接口为包内类型，无对外导出影响，但所有内部实现均需同步更新。
- **依赖**：复用已有 `trpc.group/trpc-go/trpc-agent-go/knowledge/embedder/openai`，无新增外部依赖。
- **运行时**：仅在 `strategy: "embedding"` 时触发 HTTP Embedding 调用；其他策略路径无变化。
