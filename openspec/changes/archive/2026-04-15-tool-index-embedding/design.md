## Context

`ToolIndex` 是 agent-server 动态工具过滤的核心抽象，当前有 `KeywordIndex`（子串匹配）和 `BM25Index`（全文检索）两种实现，统一通过 `buildToolIndex` 工厂函数按 `strategy` 配置项创建，由 `lazyToolIndex` 封装懒加载逻辑。

Embedding 能力现已由 `buildSQLiteVecMemoryService` 以 `openaiembed.New(...)` 的形式接入，鉴权逻辑（`X-Bkapi-Authorization` 中间件）、网关配置（`BKAPIGatewayConfig`）均已存在，只是尚未被 `ToolIndex` 体系复用。

当前接口签名 `Build(tools) / Search(query, topN, threshold)` 不携带 `context.Context`，无法传递 HTTP 请求所需的 BK 票据和超时信息。

## Goals / Non-Goals

**Goals:**
- 新增 `EmbeddingIndex`，实现向量余弦相似度工具检索，策略值 `"embedding"`
- `ToolIndex` 接口增加 `ctx` 参数，保证鉴权上下文与请求生命周期一致
- 抽取 `buildEmbeddingClient` 公共函数，消除 memory 与 tool index 两处重复的 embedder 构建代码
- Build 阶段并发调用 Embedding API，并发量由代码内常量控制

**Non-Goals:**
- 向量持久化缓存（首版全进程内存存储，无需 sqlite-vec）
- 与 BM25 混合打分（"hybrid search"）
- 自动调参 topN / scoreThreshold
- 将 Build 并发量暴露为 YAML 配置项（首版常量即可）

## Decisions

### 决策 1：ToolIndex 接口增加 ctx 而非在 EmbeddingIndex 内部缓存 ctx

**选项 A（采用）**：接口签名变为 `Build(ctx, tools) / Search(ctx, query, topN, threshold)`，调用方按请求传入 ctx。

**选项 B（放弃）**：`EmbeddingIndex` 内部保存「最近一次 ctx」，接口不变。

**理由**：选项 B 在并发场景下存在 data race，且 ctx 生命周期与请求不对齐（Build ctx 与 Search ctx 来自不同请求）。接口 ctx 化虽为 Breaking Change，但 `ToolIndex` 是包内类型，影响范围完全可控（`KeywordIndex`、`BM25Index` 仅需在实现中将 ctx 丢弃即可，调用方统一传入 `context.Background()` 或请求 ctx）。

---

### 决策 2：Build 阶段并发调用，并发量由代码内常量控制

**选项 A（采用）**：在 `tool_index.go` 定义常量 `embeddingBuildConcurrency = 8`，用 semaphore（带缓冲 channel）控制并发，`errgroup` 收集结果与错误。

**选项 B（放弃）**：顺序循环调用。

**选项 C（暂不采用）**：将并发量作为 YAML 配置项。

**理由**：顺序调用在工具数量多时（100+ 工具）冷启动延迟可能达到数十秒。并发 8 可将延迟降低到约 1/8。首版用常量足够灵活，可在后续版本按需配置化。`errgroup` + semaphore 是 Go 标准的并发模式，安全且易于测试。

---

### 决策 3：buildEmbeddingClient 抽取为 runtime.go 包级函数

**理由**：`buildSQLiteVecMemoryService` 中已有完整的 `openaiembed` 构建逻辑（BaseURL、APIKey、BK 鉴权中间件、Model、Dimensions），新增 `buildEmbeddingClient(gatewayCfg BKAPIGatewayConfig, embedCfg EmbeddingConfig) *openaiembed.Embedder` 替换两处重复代码，接口清晰且便于单元测试（可 mock `gatewayCfg.BaseURL` 为测试地址）。

---

### 决策 4：向量存储方案——进程内 [][]float64

**理由**：工具元数据（`[]ToolMeta`）本已常驻内存，平行存储 `[][]float64` 与之按下标对齐，实现简单、无额外依赖，典型工具集（几百个工具，维度 1536）内存占用约 1-2 MB，完全可接受。持久化缓存留作后续优化。

---

### 决策 5：相似度计算——余弦相似度，复用 applyThresholdAndTopN

**理由**：余弦相似度结果域为 `[-1, 1]`（语义场景通常 `[0, 1]`），与现有 `scoreThreshold`（相对最高分的比例）语义一致，无需调整阈值策略。直接复用 `applyThresholdAndTopN` 函数，减少重复逻辑。

---

### 决策 6：DynamicToolLoadingConfig 复用 EmbeddingConfig

`DynamicToolLoadingConfig` 新增 `Embedding EmbeddingConfig` 字段，与 `MemoryStorageConfig.Embedding` 结构体相同，避免引入新类型。`buildToolIndex` 签名增加 `gatewayCfg BKAPIGatewayConfig` 参数，由 `runtime.go` 中调用处传入 aidev 默认网关配置（`providers["aidev"]`）。

## Risks / Trade-offs

- **Build 冷启动延迟**：首次 `ensureBuild` 调用（并发 8，100 工具）预计约 2-5 秒（取决于网关延迟），期间请求 `makeDynamicToolFilter` 会等待 `sync.Once` 完成。→ 已有 `sync.Once` 保证只触发一次；日志中会打印工具数与耗时，便于观测。如延迟不可接受，后续可改为异步预热。

- **Embedding API 限流/不可用**：Build 失败时现有逻辑已降级为 `passAll: true`；Search 失败时需同样降级放行，不因网络抖动误杀工具。→ `EmbeddingIndex.Search` 遇到错误时返回 `nil`，`lazyToolIndex` 路径已有 `passAll` 降级处理。
  - **后续优化**：`sync.Once` 对瞬态错误（网络抖动）语义不合适——Build 一旦失败即永久 `buildOK = false`，所有后续请求均 passAll，无法自愈。`tool.FilterFunc` 接口签名为 `func(ctx, tool) bool`，无 error 返回路径，Build 失败无法直接中断请求。后续可将 `sync.Once` 替换为带退避重试的可重置 build 机制（独立变更实现）。

- **向量维度不匹配**：`Dimensions` 配置与网关实际返回不一致时余弦相似度无意义。→ `Build` 阶段校验所有返回向量的 `len` 均等于第一个向量的维度，不一致则报错，回退 `buildOK = false`。

- **接口 Breaking Change**：`ToolIndex` 接口 ctx 化影响所有实现与调用点。→ 范围完全限制在 `logics` 包内，`KeywordIndex` / `BM25Index` 只需在函数签名上加 `_ context.Context`，现有测试需同步更新（`buildTestLazy` 及相关断言）。

## Migration Plan

1. 先修改 `ToolIndex` 接口及 `KeywordIndex`、`BM25Index`、`lazyToolIndex`，确保现有测试全量编译通过。
2. 抽取 `buildEmbeddingClient`，更新 `buildSQLiteVecMemoryService` 改用此函数，确保 memory 路径功能不变。
3. 新增 `EmbeddingIndex` 及余弦相似度函数，补充单元测试（mock embedder）。
4. 扩展 `DynamicToolLoadingConfig` 与 `buildToolIndex`，更新 `runtime.go` 调用处。
5. 更新 `agent_server.yaml` 配置注释示例。

所有变更在同一 PR 中合并；无 DB Schema 变更；无需数据迁移；回滚只需将 `strategy` 改回 `bm25` 或 `keyword`。

## Open Questions

- 蓝鲸 aidev 网关是否对 `/embeddings` 路径与 `/chat/completions` 使用同一套限流配额？（影响并发常量 8 是否合适）→ 建议联调时关注网关侧的限流返回（HTTP 429），若频繁出现可降低常量值。
