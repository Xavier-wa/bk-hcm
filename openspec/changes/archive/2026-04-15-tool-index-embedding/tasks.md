## 1. ToolIndex 接口 ctx 化

- [x] 1.1 修改 `tool_index.go`：将 `ToolIndex` 接口方法签名改为 `Build(ctx context.Context, tools []ToolMeta) error` 和 `Search(ctx context.Context, query string, topN int, scoreThreshold float64) []ToolMatch`；在文件顶部 import `"context"`
- [x] 1.2 修改 `tool_index.go`：更新 `KeywordIndex.Build` 与 `KeywordIndex.Search` 签名，接收 `_ context.Context` 并忽略
- [x] 1.3 修改 `tool_index.go`：更新 `BM25Index.Build` 与 `BM25Index.Search` 签名，接收 `_ context.Context` 并忽略
- [x] 1.4 修改 `tool_filter.go`：更新 `lazyToolIndex.ensureBuild` 内 `l.index.Build(ctx, metas)` 调用，传入 `ctx`
- [x] 1.5 修改 `tool_filter.go`：更新 `makeDynamicToolFilter` 内 `lazy.index.Search(ctx, query, ...)` 调用，传入 filter 函数的 `ctx`
- [x] 1.6 修改 `tool_filter_test.go`：更新 `buildTestLazy` 及所有测试用例中对 `Build` / `Search` 的调用，补充 `context.Background()` 参数，确保全量编译通过

## 2. 抽取 buildEmbeddingClient 公共函数

- [x] 2.1 修改 `runtime.go`：新增包级函数 `buildEmbeddingClient(gatewayCfg BKAPIGatewayConfig, embedCfg EmbeddingConfig) *openaiembed.Embedder`，将 `buildSQLiteVecMemoryService` 中从 `var embedOpts []openaiembed.Option` 到 `emb := openaiembed.New(embedOpts...)` 的完整逻辑提取到此函数
- [x] 2.2 修改 `runtime.go`：重构 `buildSQLiteVecMemoryService`，删除提取出的重复代码，改为调用 `buildEmbeddingClient(gatewayCfg, cfg.Embedding)`，确认 memory 路径功能不变

## 3. 新增 EmbeddingIndex 实现

- [x] 3.1 修改 `tool_index.go`：在文件顶部 import 中增加 `"context"`（步骤 1.1 已完成）、`"fmt"`、`"golang.org/x/sync/errgroup"`；新增常量 `embeddingBuildConcurrency = 8`
- [x] 3.2 修改 `tool_index.go`：新增 `EmbeddingIndex` 结构体，字段包含 `emb`（`embedder.Embedder` 接口，兼容 mock 测试）、`tools []ToolMeta`、`vectors [][]float64`；新增构造函数 `NewEmbeddingIndex(emb embedder.Embedder) *EmbeddingIndex`
- [x] 3.3 修改 `tool_index.go`：实现 `EmbeddingIndex.Build(ctx, tools)`——使用带缓冲 channel 作为 semaphore、`errgroup` 并发调用 `embedder.GetEmbedding(ctx, tool.SearchText)`，结果按原始下标写入 `vectors [][]float64`；Build 完成后校验所有向量维度一致，不一致则返回 error
- [x] 3.4 修改 `tool_index.go`：新增包级函数 `cosineSimilarity(a, b []float64) float64`，实现标准余弦相似度（内积除以模长之积）；两向量长度不等时 panic（属于编程错误，由 Build 校验保证不发生）
- [x] 3.5 修改 `tool_index.go`：实现 `EmbeddingIndex.Search(ctx, query, topN, scoreThreshold)`——对 query 调用 `embedder.GetEmbedding`，失败时返回 `nil`；成功后逐工具调用 `cosineSimilarity`，调用 `applyThresholdAndTopN` 返回结果；若 `vectors` 为空直接返回 `nil`

## 4. 配置扩展与 buildToolIndex 集成

- [x] 4.1 修改 `runtime.go`：在 `DynamicToolLoadingConfig` 结构体中新增字段 `Embedding EmbeddingConfig`，并在注释中说明仅 `strategy: "embedding"` 时生效
- [x] 4.2 修改 `tool_filter.go`：将 `buildToolIndex` 函数签名改为 `buildToolIndex(cfg *DynamicToolLoadingConfig, gatewayCfg BKAPIGatewayConfig) ToolIndex`，在 `strategy == "embedding"` 分支中调用 `buildEmbeddingClient(gatewayCfg, cfg.Embedding)` 并返回 `NewEmbeddingIndex(emb)`
- [x] 4.3 修改 `runtime.go`：在 `New` 函数中 `buildToolIndex(d)` 调用处，改为传入 aidev 网关配置：`buildToolIndex(d, resolveProviderConfig("aidev", providers))`

## 5. 配置文件与文档更新

- [x] 5.1 修改 `cmd/agent-server/etc/agent_server.yaml`：在 `dynamicToolLoading` 配置段增加注释，说明 `strategy: embedding` 可用值及所需的 `embedding.model` / `embedding.dimensions` 子字段，注明 aidev 网关必须可访问

## 6. 单元测试

- [x] 6.1 修改 `tool_filter_test.go`：确认步骤 1.6 中所有现有测试全量编译通过，且 `TestDynamicToolFilter_*`、`TestLazyToolIndex_*`、`TestExtractQuery_*` 等用例运行结果不变
- [x] 6.2 新增 `tool_index_test.go`（如不存在则创建）：针对 `cosineSimilarity` 编写纯内存单元测试，覆盖正交向量（期望 0）、同向向量（期望 1）、反向向量（期望 -1）三种场景
- [x] 6.3 新增 `tool_index_test.go`：针对 `EmbeddingIndex` 使用 mock embedder（实现 `embedder.Embedder` 接口，返回固定向量）编写 Build + Search 集成测试，覆盖：正常召回、维度不一致 Build 失败、Search 时 embedder 出错返回 nil 三种场景
