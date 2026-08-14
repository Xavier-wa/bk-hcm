## 1. 配置层（cc + app 转换）

- [x] 1.1 在 `pkg/cc/service.go` 中新增 `AgentDynamicToolLoadingConfig` 结构体（`Enabled bool`, `Strategy string`, `TopN int`, `ScoreThreshold float64`, `ToolTags map[string][]string`），并在 `AgentToolsConfig` 中新增 `DynamicToolLoading *AgentDynamicToolLoadingConfig` 字段（yaml tag: `dynamicToolLoading`）
- [x] 1.2 在 `cmd/agent-server/logics/runtime.go` 中新增 `DynamicToolLoadingConfig` 内部配置结构体（`Enabled bool`, `Strategy string`, `TopN int`, `ScoreThreshold float64`, `ToolTags map[string][]string`），并在 `ToolsConfig` 中新增 `DynamicToolLoading *DynamicToolLoadingConfig` 字段
- [x] 1.3 在 `cmd/agent-server/app/app.go` 的 `toolsToLogicsConfig()` 中新增 `DynamicToolLoading` 的转换逻辑（`cc.AgentDynamicToolLoadingConfig` → `logics.DynamicToolLoadingConfig`），包含 `TopN` 校验：`enabled: true` 时 `topN <= 0` 则使用默认值 10
- [x] 1.4 在 `cmd/agent-server/etc/agent_server.yaml` 的 `tools` 配置节下新增 `dynamicToolLoading` 配置示例（默认 `enabled: false`，含 strategy、topN、scoreThreshold、toolTags 注释示例）
- [x] 1.5 在 `pkg/cc/service.go` 的 `AgentDynamicToolLoadingConfig` 中新增 `QueryContextWindow int` 字段（yaml tag: `queryContextWindow`）
- [x] 1.6 在 `cmd/agent-server/logics/runtime.go` 的 `DynamicToolLoadingConfig` 中新增 `QueryContextWindow int` 字段
- [x] 1.7 在 `cmd/agent-server/app/app.go` 的 `toolsToLogicsConfig()` 中新增 `QueryContextWindow` 转换逻辑：`enabled: true` 时 `queryContextWindow <= 0` 则使用默认值 3
- [x] 1.8 在 `cmd/agent-server/etc/agent_server.yaml` 的 `dynamicToolLoading` 注释示例中新增 `queryContextWindow: 3`

## 2. 工具元数据与索引（tool_index.go）

- [x] 2.1 新建 `cmd/agent-server/logics/tool_index.go`，定义 `ToolMeta`、`ParamMeta`、`ToolMatch` 结构体
- [x] 2.2 实现 `extractToolMeta(t tool.Tool, tags []string) ToolMeta` 函数：从 `tool.Declaration` 提取元数据，拼接 `SearchText`
- [x] 2.3 实现 `tokenize(text string) []string` 分词函数：英文按空格和下划线拆分并转小写；中文输出 unigram（单字）+ bigram（双字组合）混合 n-gram；数字和特殊字符保留为独立 token
- [x] 2.4 定义 `ToolIndex` 接口（`Build([]ToolMeta) error` + `Search(query string, topN int, scoreThreshold float64) []ToolMatch`）。`Search` 内部流程：评分排序 → 若 `scoreThreshold > 0`，过滤得分 < `maxScore * scoreThreshold` 的结果 → 取 topN → 返回
- [x] 2.5 实现 `KeywordIndex`：子串匹配 + Tags 精确匹配加分，按得分降序排列，应用 scoreThreshold 过滤后返回 Top-N
- [x] 2.6 实现 `BM25Index`：倒排索引构建、BM25 评分（TF-IDF + 文档长度归一化，k1=1.2, b=0.75），按得分降序排列，应用 scoreThreshold 过滤后返回 Top-N

## 3. 动态 ToolFilter（tool_filter.go）

- [x] 3.1 新建 `cmd/agent-server/logics/tool_filter.go`，定义 `retrievedToolsCacheKey` 常量
- [x] 3.2 实现 `lazyToolIndex` 结构体：持有 `[]tool.ToolSet`、`ToolIndex`、`scoreThreshold float64`、`toolTags` 配置、`queryContextWindow int`、`mcpToolNames map[string]bool`（MCP 工具**前缀名**集合），使用 `sync.Once` 懒构建。`ensureBuild(ctx)` 遍历 MCP ToolSet 时：若某个 ToolSet 的 `ts.Tools(ctx)` 失败则跳过该 ToolSet 继续处理其余部分；对每个成功获取的工具计算框架前缀名 `frameworkName`：若 `ts.Name()` 非空则为 `ts.Name() + "_" + rawName`，否则为 `rawName`。用 `frameworkName` 作为 ToolMeta.Name、写入 `mcpToolNames`。通过原始名 `rawName` 查找 `toolTags[rawName]` 注入 Tags。全部 ToolSet 均失败且无工具时视为构建失败
- [x] 3.3 实现 `makeDynamicToolFilter` 函数：返回 `tool.FilterFunc`，逻辑包括——（a）无法获取 Invocation 时**全量放行**（系统错误降级）；（b）检查自定义缓存命中直接返回 `cache[toolName]`；（c）工具名不在 `mcpToolNames`（前缀名集合）中直接放行（Skill 工具）；（d）首次执行时提取用户消息文本（兼容纯文本和多模态，见下方 3.3.1），调用 `lazyToolIndex.ensureBuild` + `Search(query, topN, scoreThreshold)` + 构建白名单 `map[string]bool` + 写入缓存；（e）**构建失败**或**无法获取用户消息**时**全量放行**（系统错误降级）；（f）**检索无结果**时写入空 `map[string]bool{}`，MCP 工具返回 `false`（纯聊天场景，不需要工具）。注意：框架内置工具由 Per-Run ToolFilter 自动豁免，无需在 filter 中处理
- [x] 3.3.1 重构查询文本提取逻辑为多轮上下文窗口 + Summary 补充：（a）新增 `extractQueryWithContext(inv *agent.Invocation, contextWindow int) string` 函数；（b）从 `inv.Session.GetEvents()` 倒序扫描，提取最近 `contextWindow - 1` 条历史用户消息（通过 `IsUserMessage()` 判断），使用 `extractMessageText()` 提取每条消息的文本（兼容 `Content` 和 `ContentParts`）；（c）当 `inv.Session.Summaries` 中存在全局摘要（key 为空字符串 `SummaryFilterKeyAllContents`）时，将 `Summary.Summary` 文本作为查询前缀；（d）按时间正序拼接：`[Summary] + [历史消息...] + [当前消息]`，以空格分隔；（e）`inv.Session` 为 nil 或 `contextWindow <= 1` 时退化为仅用当前消息；（f）所有文本均为空时视为系统错误降级全量放行
- [x] 3.3.2 将原 `extractQueryFromInvocation(inv)` 重构为调用 `extractQueryWithContext(inv, contextWindow)`，保持 `makeDynamicToolFilter` 中的调用签名兼容（`contextWindow` 通过闭包从 `lazyToolIndex` 获取）
- [x] 3.4 实现 `buildToolIndex(cfg *DynamicToolLoadingConfig) ToolIndex` 工厂函数：根据 `cfg.Strategy`（keyword/bm25）创建对应的 `ToolIndex` 实例

## 4. Runtime 集成（移除 Proxy + 暴露 Filter）

- [x] 4.1 删除 `cmd/agent-server/logics/mcp_proxy.go` 整个文件
- [x] 4.2 修改 `cmd/agent-server/logics/runtime.go` 的 `New()` 函数：移除 `proxyToolSets := wrapMCPToolSetsAsProxy(mcpToolSets)` 调用，后续直接使用 `mcpToolSets`
- [x] 4.3 修改 `New()` 函数：当 `toolsCfg.DynamicToolLoading` 启用时，调用 `buildToolIndex` 构建索引，创建 `lazyToolIndex`，将 `makeDynamicToolFilter(...)` 的结果存储到 `Runtime` 上
- [x] 4.4 在 `Runtime` 结构体新增 `dynamicToolFilter tool.FilterFunc` 字段，新增 `DynamicToolFilter() tool.FilterFunc` 方法（返回已构建的 filter 函数或 nil）
- [x] 4.5 修改 `newAgentWithModel()` 签名：移除原方案中的 `dynamicToolCfg` 参数（不再在 Agent 级注入 ToolFilter）
- [x] 4.6 修改 `Runtime` 结构体的 `aguiMCPToolSets` 字段：存储原始 `mcpToolSets`（不再是 proxyToolSets），确保 `Close()` 正确释放资源
- [x] 4.7 清理 `New()` 和 `buildSkillRepo()` 中对 `proxyToolSets` 的引用，统一替换为 `mcpToolSets`

## 5. Service 层集成（Per-Run ToolFilter 注入）

- [x] 5.1 修改 `cmd/agent-server/service/service.go` 中的 `makeModelRunOptionResolver`：重构为 `makeRunOptionResolver`，合并模型选择和动态工具过滤两个 RunOption。新增 `toolFilter tool.FilterFunc` 参数，当非 nil 时注入 `agent.WithToolFilter(toolFilter)`
- [x] 5.2 修改 `service.New()` 中调用处：将 `rt.DynamicToolFilter()` 传入 `makeRunOptionResolver`

## 6. 单元测试

- [x] 6.1 新建 `cmd/agent-server/logics/tool_index_test.go`：测试 `extractToolMeta`（含 Tags 和无 Tags 场景）、`tokenize`（中英文混合 n-gram 分词、英文拆分、特殊字符处理）、`buildSearchText`
- [x] 6.2 测试 `KeywordIndex`：子串匹配命中/未命中、Tags 加分、TopN 截断、scoreThreshold 过滤、空查询
- [x] 6.3 测试 `BM25Index`：构建索引 → 检索排序正确性、TopN 截断、scoreThreshold 过滤（验证低分被过滤）、空索引检索
- [x] 6.4 新建 `cmd/agent-server/logics/tool_filter_test.go`：测试 `makeDynamicToolFilter` 的系统错误降级场景（无 Invocation → 全量放行、无用户消息 → 全量放行、构建失败 → 全量放行）、检索无结果场景（MCP 工具返回 false、Skill 工具仍放行）、正常检索命中场景、Skill 工具放行
- [x] 6.5 测试 `lazyToolIndex` 前缀名处理：带 Name() 的 ToolSet 生成前缀名索引；空 Name() 使用原始名；`toolTags` 通过原始名匹配注入到前缀名 ToolMeta
- [x] 6.7 测试 `lazyToolIndex` 部分构建失败：一个 ToolSet 成功、一个失败 → 索引包含成功部分；全部失败 → 构建失败降级
- [x] 6.8 测试 topN 校验：`enabled: true` 时 `topN <= 0` 使用默认值 10
- [x] 6.6 测试查询文本提取（重写为多轮上下文场景）：（a）单轮消息（contextWindow=1 或 Session 为 nil）退化为仅用当前消息；（b）多轮滑动窗口：Session 有 5 条用户消息、contextWindow=3 → 查询包含最近 3 条；（c）Session Summary 补充：有 Summary 时查询前缀包含摘要文本，无 Summary 时正常退化；（d）窗口 + Summary 综合场景：验证拼接顺序为 `[Summary] + [历史消息] + [当前消息]`；（e）纯文本和多模态消息混合提取；（f）空消息降级全量放行
- [x] 6.9 测试多轮对话端到端场景：模拟"买机器→查机型→确认购买"三轮对话，验证第三轮 query 包含前两轮关键词，BM25 能同时命中购买工具和查询工具
- [x] 6.10 测试 queryContextWindow 配置校验：`enabled: true` 时 `queryContextWindow <= 0` 使用默认值 3
