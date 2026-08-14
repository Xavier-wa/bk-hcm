# embedding-tool-index

## ADDED Requirements

### Requirement: ToolIndex 接口支持 context.Context
`ToolIndex` 接口的 `Build` 与 `Search` 方法 SHALL 增加 `context.Context` 作为第一个参数。`KeywordIndex` 与 `BM25Index` 的实现 SHALL 接受该参数但可忽略（`_ context.Context`）。`lazyToolIndex` 的 `ensureBuild` 与 `makeDynamicToolFilter` 中 SHALL 将请求 ctx 传入所有 `Build` / `Search` 调用。

#### Scenario: KeywordIndex 与 BM25Index 接受 ctx 但行为不变
- **WHEN** 以任意非空 `context.Context` 调用 `KeywordIndex.Build` 或 `BM25Index.Search`
- **THEN** 系统 SHALL 返回与传入 `context.Background()` 完全相同的结果，ctx 不影响检索逻辑

#### Scenario: lazyToolIndex 传递请求 ctx 到 Build
- **WHEN** `lazyToolIndex.ensureBuild(ctx)` 首次被调用
- **THEN** 系统 SHALL 将该 `ctx` 透传给 `index.Build(ctx, metas)`

#### Scenario: lazyToolIndex 传递请求 ctx 到 Search
- **WHEN** `makeDynamicToolFilter` 内调用 `lazy.index.Search`
- **THEN** 系统 SHALL 将 filter 函数接收到的 `ctx` 透传给 `Search(ctx, query, topN, threshold)`

---

### Requirement: buildEmbeddingClient 公共函数
系统 SHALL 提供包级函数 `buildEmbeddingClient(gatewayCfg BKAPIGatewayConfig, embedCfg EmbeddingConfig) *openaiembed.Embedder`，封装 openaiembed 客户端的构建逻辑（BaseURL、APIKey、BK 鉴权中间件、Model、Dimensions）。`buildSQLiteVecMemoryService` SHALL 改为调用此函数，不得保留原有重复的构建代码。

#### Scenario: 使用 APIKey 鉴权构建 embedder
- **WHEN** `gatewayCfg.APIKey` 非空、`gatewayCfg.AppCode` 为空
- **THEN** 系统 SHALL 构造一个带 `WithBaseURL` 和 `WithAPIKey` 的 `openaiembed.Embedder`，不注入 BK 鉴权中间件

#### Scenario: 使用 BK 应用鉴权构建 embedder
- **WHEN** `gatewayCfg.AppCode` 或 `gatewayCfg.AppSecret` 非空
- **THEN** 系统 SHALL 在 embedder 上注入 `WithRequestOptions` 中间件，该中间件在每次请求时从 `context.Context` 提取 bk_username / bk_ticket 并写入 `X-Bkapi-Authorization` 请求头；若 ctx 中无用户信息则回退到 `gatewayCfg.DefaultUser` / `gatewayCfg.DefaultTicket`

#### Scenario: 配置 Model 与 Dimensions
- **WHEN** `embedCfg.Model` 非空或 `embedCfg.Dimensions > 0`
- **THEN** 系统 SHALL 分别调用 `WithModel` / `WithDimensions` 选项；若对应字段为零值则不注入该选项，由 SDK 使用默认值

---

### Requirement: EmbeddingIndex 实现
系统 SHALL 提供 `EmbeddingIndex` 结构体，实现 `ToolIndex` 接口，使用向量余弦相似度进行工具检索，并通过 `NewEmbeddingIndex(embedder *openaiembed.Embedder) *EmbeddingIndex` 构造。

#### Scenario: Build 成功——并发生成工具向量
- **WHEN** `EmbeddingIndex.Build(ctx, tools)` 被调用，所有工具的 Embedding API 调用均成功
- **THEN** 系统 SHALL 以最多 `embeddingBuildConcurrency`（代码内常量，建议值 8）个并发 goroutine 调用 `embedder.GetEmbedding(ctx, tool.SearchText)`，结果按工具原始顺序存入 `[][]float64`，并返回 `nil` error

#### Scenario: Build 成功——向量维度一致性校验
- **WHEN** `EmbeddingIndex.Build(ctx, tools)` 调用成功，但某工具返回的向量维度与第一个工具不同
- **THEN** 系统 SHALL 返回包含维度不一致信息的 `error`，不存储任何工具向量

#### Scenario: Build 失败——任意工具 Embedding 调用出错
- **WHEN** `EmbeddingIndex.Build(ctx, tools)` 期间任意一个 `GetEmbedding` 调用返回 `error`
- **THEN** 系统 SHALL 终止其余并发调用（通过 ctx 取消），`Build` 返回该 `error`；`lazyToolIndex` 将保持 `buildOK == false`，动态过滤降级为放行全部工具

#### Scenario: Search 成功——余弦相似度打分
- **WHEN** `EmbeddingIndex.Search(ctx, query, topN, scoreThreshold)` 被调用，Build 已成功，query Embedding 调用成功
- **THEN** 系统 SHALL 对 query 调用一次 `GetEmbedding`，与每个工具向量计算余弦相似度，调用 `applyThresholdAndTopN` 排序截断后返回 `[]ToolMatch`；`ToolMatch.Score` 为余弦相似度值

#### Scenario: Search 失败——Embedding API 调用出错
- **WHEN** `EmbeddingIndex.Search` 中对 query 的 `GetEmbedding` 调用返回 `error`
- **THEN** 系统 SHALL 返回 `nil`（空结果），`makeDynamicToolFilter` 已有 passAll 降级逻辑将放行全部 MCP 工具

#### Scenario: Search 在 Build 未完成时调用
- **WHEN** `EmbeddingIndex.Search` 被调用但内部向量为空（Build 未被调用或 Build 失败）
- **THEN** 系统 SHALL 返回 `nil`

---

### Requirement: DynamicToolLoadingConfig 支持 Embedding 配置
`DynamicToolLoadingConfig` 结构体 SHALL 增加 `Embedding EmbeddingConfig` 字段，用于在 `strategy: "embedding"` 时提供 Embedding 模型名称与向量维度配置。

#### Scenario: strategy 为 embedding 时读取 Embedding 字段
- **WHEN** `DynamicToolLoadingConfig.Strategy == "embedding"` 且 `Embedding.Model` 非空
- **THEN** 系统 SHALL 使用 `Embedding.Model` 与 `Embedding.Dimensions` 构造 `EmbeddingIndex`

#### Scenario: strategy 为 embedding 时 Embedding 字段使用零值
- **WHEN** `DynamicToolLoadingConfig.Strategy == "embedding"` 且 `Embedding.Model` 为空
- **THEN** 系统 SHALL 构造 `EmbeddingIndex` 时不设置 Model 与 Dimensions，由 SDK 使用默认值（`text-embedding-3-small`，1536 维）

---

### Requirement: buildToolIndex 支持 embedding 策略
`buildToolIndex` 函数 SHALL 增加 `gatewayCfg BKAPIGatewayConfig` 参数。当 `cfg.Strategy` 大小写不敏感匹配 `"embedding"` 时，SHALL 调用 `buildEmbeddingClient(gatewayCfg, cfg.Embedding)` 并返回 `NewEmbeddingIndex(embedder)`。其余策略（`"keyword"`、默认 `"bm25"`）的行为 SHALL 保持不变。`runtime.go` 中调用 `buildToolIndex` 处 SHALL 传入 aidev 网关默认配置（`providers["aidev"]`）。

#### Scenario: strategy 为 embedding 时返回 EmbeddingIndex
- **WHEN** `buildToolIndex` 以 `Strategy: "embedding"`（或 `"Embedding"`）配置调用
- **THEN** 系统 SHALL 返回一个 `*EmbeddingIndex` 实例

#### Scenario: strategy 为 keyword 或 bm25 时不受影响
- **WHEN** `buildToolIndex` 以 `Strategy: "keyword"` 或 `Strategy: "bm25"` 调用
- **THEN** 系统 SHALL 分别返回 `*KeywordIndex` 和 `*BM25Index`，与改造前行为一致

---

### Requirement: agent_server.yaml 提供 embedding 策略配置示例
`cmd/agent-server/etc/agent_server.yaml` 中 `dynamicToolLoading` 配置段 SHALL 增加注释，说明 `strategy: embedding` 时所需的 `embedding.model` / `embedding.dimensions` 字段，并注明需要 aidev 网关可访问。

#### Scenario: 配置文件包含 embedding 示例注释
- **WHEN** 运维人员查阅 `agent_server.yaml`
- **THEN** 文件中 `dynamicToolLoading` 部分 SHALL 存在关于 `strategy: embedding` 及 `embedding:` 子字段的注释说明
