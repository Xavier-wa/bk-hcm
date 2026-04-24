# dynamic-tool-loading

## ADDED Requirements

### Requirement: MCP 工具直接注册

系统 SHALL 将 MCP ToolSet 中的工具直接注册到框架（通过 `llmagent.WithToolSets(mcpToolSets)`），LLM SHALL 直接看到每个 MCP 工具的完整 `tool.Declaration`（含 name、description、inputSchema）并直接调用。系统 SHALL 移除 `mcp_proxy.go` 中的 `wrapMCPToolSetsAsProxy()` 函数及 `mcpProxyToolSet`、`mcpListTool`、`mcpCallTool` 等相关类型，不再暴露 `list_tools` / `call_tool` 代理工具。

#### Scenario: MCP 工具直接暴露给 LLM

- **WHEN** agent-server 启动并注册了 MCP ToolSet
- **THEN** LLM 看到的工具列表中包含每个 MCP 工具的真实名称和完整 schema，不包含 `list_tools` 或 `call_tool` 代理工具

#### Scenario: LLM 直接调用 MCP 工具

- **WHEN** LLM 决定调用某个 MCP 工具（如 `list_cvms`）
- **THEN** 框架直接将调用路由到对应的 MCP 工具执行，无需经过 list → schema → call 的间接步骤

### Requirement: 工具元数据提取

系统 SHALL 从每个 MCP 工具的 `tool.Declaration` 中自动提取结构化元数据（`ToolMeta`），包括：工具名称（`Name`）、功能描述（`Description`）、参数列表摘要（`Parameters`，含名称、类型、是否必填）、可选标签（`Tags`，来自配置）。系统 SHALL 将上述字段拼接为 `SearchText` 用于全文检索。

#### Scenario: 从 tool.Declaration 提取元数据

- **WHEN** 系统构建工具索引时遍历 MCP ToolSet 中的每个工具
- **THEN** 系统为每个工具生成一个 `ToolMeta`，其 `Name` 和 `Description` 来自 `tool.Declaration`，`Parameters` 从 `InputSchema.Properties` 提取，`Tags` 从配置的 `toolTags` 中按工具名匹配

#### Scenario: SearchText 拼接

- **WHEN** 系统生成 `ToolMeta` 时
- **THEN** `SearchText` SHALL 包含 `Name`、`Description`、所有 `Tags` 和所有参数名称的拼接文本

### Requirement: 工具索引接口

系统 SHALL 提供统一的 `ToolIndex` 接口，定义 `Build(tools []ToolMeta) error` 和 `Search(query string, topN int) []ToolMatch` 两个方法。`Search` 返回按相关性得分降序排列的 Top-N 匹配结果。

#### Scenario: 索引构建与检索

- **WHEN** 调用 `Build` 传入一组 `ToolMeta`，然后调用 `Search` 传入查询文本和 topN
- **THEN** 返回得分最高的 topN 个匹配工具，按得分降序排列；无匹配时返回空切片

### Requirement: 混合 n-gram 分词

系统 SHALL 实现 `tokenize(text string) []string` 分词函数，对输入文本执行以下处理：
- 英文部分：按空格和下划线拆分，转小写
- 中文部分：输出 unigram（单字）和 bigram（双字组合）
- 数字和特殊字符：保留为独立 token

该分词函数 SHALL 被 KeywordIndex、BM25Index 的索引构建和检索查询两侧共同使用，确保分词一致性。

#### Scenario: 英文分词

- **WHEN** 输入为 "list_cvms"
- **THEN** 输出 ["list", "cvms"]

#### Scenario: 中文混合 n-gram 分词

- **WHEN** 输入为 "云服务器"
- **THEN** 输出包含 unigram ["云", "服", "务", "器"] 和 bigram ["云服", "服务", "务器"]

#### Scenario: 中英文混合分词

- **WHEN** 输入为 "查一下 CVM 主机"
- **THEN** 英文 "cvm" 单独作为 token，中文 "查一下" 和 "主机" 各自产出 unigram + bigram

### Requirement: KeywordIndex 关键词检索策略

系统 SHALL 实现 `KeywordIndex`，基于关键词子串匹配检索工具。对查询文本分词后，在每个工具的 `SearchText` 中查找匹配（大小写不敏感），命中关键词越多得分越高。Tags 精确匹配（大小写不敏感）SHALL 获得额外加分。

#### Scenario: 关键词匹配命中

- **WHEN** 用户消息为"查一下 CVM"，工具 `list_cvms` 的 `SearchText` 包含 "CVM"，Tags 包含 "CVM"
- **THEN** `list_cvms` 被命中，得分包含子串匹配分和 Tags 精确匹配额外加分

#### Scenario: 无关键词命中

- **WHEN** 用户消息中的所有关键词均未在任何工具的 `SearchText` 中匹配到
- **THEN** 返回空结果

### Requirement: BM25Index 检索策略

系统 SHALL 实现 `BM25Index`，基于 BM25 算法（考虑词频 TF、逆文档频率 IDF 和文档长度归一化）检索工具。SHALL 为纯 Go 实现，不引入外部依赖。

#### Scenario: BM25 检索排序

- **WHEN** 用户消息为"帮我查一下北京的 CVM 有哪些"，索引中有 `list_cvms`（Tags 含 "CVM"、"云服务器"）和 `query_bill`（Tags 含 "账单"）
- **THEN** `list_cvms` 的 BM25 得分高于 `query_bill`，排在结果前列

#### Scenario: TopN 截断

- **WHEN** BM25 检索返回的匹配工具数量超过 topN
- **THEN** 只返回得分最高的 topN 个结果

### Requirement: 动态 ToolFilter 集成

系统 SHALL 通过 Per-Run 级 `agent.WithToolFilter()` 注入一个动态过滤函数，该函数通过 `RunOptionResolver` 在每次 Run 时提供。该过滤函数在每个 Invocation 的首次 Step 中执行检索（从用户消息中提取查询文本，调用 `ToolIndex.Search` 获取 Top-N），并将结果写入 Invocation state 缓存。同一 Invocation 内后续 Step 由框架的 Invocation 级缓存接管，工具集保持稳定。框架内置工具（`transfer_to_agent`、`knowledge_search` 等）由框架自动豁免，不经过 ToolFilter。

`logics.Runtime` SHALL 提供 `DynamicToolFilter() tool.FilterFunc` 方法，返回已构建好的 filter 函数（未启用时返回 nil）。service 层的 `RunOptionResolver` SHALL 在 filter 非 nil 时将其作为 `agent.WithToolFilter()` RunOption 注入。

#### Scenario: 首次 Step 执行检索并缓存

- **WHEN** 用户发送一条消息触发新的 Run/Invocation，ToolFilter 首次执行
- **THEN** Filter 从用户消息提取查询文本，调用索引检索，将匹配的工具名写入 Invocation state 缓存（`custom:retrieved_tools`），仅放行匹配到的 MCP 工具

#### Scenario: 后续 Step 命中缓存

- **WHEN** 同一 Invocation 内 ToolFilter 再次被调用（Step 2~N）
- **THEN** Filter 从 Invocation state 缓存读取上次检索结果，直接返回，不重复执行检索

#### Scenario: 新 Run 重新检索

- **WHEN** 用户发送下一条消息，触发新的 Run（新的 Invocation）
- **THEN** RunOptionResolver 注入新的 ToolFilter RunOption，Filter 重新执行检索（新的 Invocation state 无缓存），基于新消息内容选择工具集

### Requirement: Skill 工具和框架工具不参与动态过滤

ToolFilter SHALL 仅过滤 MCP 工具。框架内置工具（`transfer_to_agent`、`knowledge_search` 等）由框架对 Per-Run ToolFilter 自动豁免。Skill 工具（通过 `skillRepo` / `llmagent.WithSkills()` 注册）SHALL 在 Filter 内部放行。Filter SHALL 维护一个"MCP 工具名集合"（在索引构建时记录），不在此集合中的用户工具（即 Skill 工具）直接返回 `true`。

#### Scenario: Skill 工具直接放行

- **WHEN** ToolFilter 收到一个由 Skill 注册的工具
- **THEN** 该工具名不在 MCP 工具名集合中，Filter 返回 `true`，工具不被过滤

#### Scenario: 框架内置工具自动豁免

- **WHEN** 框架执行 Per-Run ToolFilter 时遇到 `transfer_to_agent` 或 `knowledge_search` 等内置工具
- **THEN** 框架自动跳过 ToolFilter，内置工具始终保留在工具列表中

### Requirement: 懒构建索引

系统 SHALL 使用 `sync.Once` 在首次 ToolFilter 执行时懒构建工具索引。构建过程遍历所有 MCP ToolSet，对每个 ToolSet 调用 `ts.Tools(ctx)` 提取元数据。若某个 ToolSet 获取工具失败，SHALL 跳过该 ToolSet 并继续处理其余部分。只要至少有一个工具成功提取，索引即视为构建成功。全部 ToolSet 均失败且无工具可提取时，视为构建失败，降级为全量放行。

#### Scenario: 首次请求时懒构建

- **WHEN** 第一次 ToolFilter 执行时索引尚未构建
- **THEN** 系统遍历所有 MCP ToolSet 提取工具元数据，构建索引，记录 "tool index built: N tools indexed" 日志

#### Scenario: 部分 ToolSet 失败时继续构建

- **WHEN** 存在两个 MCP ToolSet，ToolSet A 成功返回 10 个工具，ToolSet B 的 `ts.Tools(ctx)` 失败
- **THEN** 系统跳过 ToolSet B，使用 ToolSet A 的 10 个工具构建索引。ToolSet B 的工具不在 `mcpToolNames` 中，Filter 遇到时直接放行

#### Scenario: 全部 ToolSet 失败降级

- **WHEN** 所有 MCP ToolSet 的 `ts.Tools(ctx)` 均失败，无工具可提取
- **THEN** 系统记录错误日志，`sync.Once` 标记为已完成，后续所有 ToolFilter 调用降级为全量放行

### Requirement: 降级策略

系统 SHALL 区分"系统错误降级"和"检索无结果"两种场景：

**系统错误降级**（全量放行 MCP 工具，Filter 返回 `true`）：
1. `dynamicToolLoading.enabled` 为 `false` 时，不注入 ToolFilter
2. 索引构建失败时，Filter 对所有工具返回 `true`
3. 无法从 context 获取 Invocation 时，Filter 对所有工具返回 `true`
4. 无法从 Invocation 获取用户消息时，Filter 对所有工具返回 `true`

**检索无结果**（不放行 MCP 工具）：
5. 检索正常执行但无匹配结果时，Filter 对 MCP 工具返回 `false`，Skill 工具仍放行

#### Scenario: 配置关闭不注入 Filter

- **WHEN** `dynamicToolLoading.enabled` 为 `false`
- **THEN** 系统不注入 ToolFilter，MCP 工具仍直接注册到框架（全量暴露给 LLM）

#### Scenario: 检索无结果时不放行 MCP 工具

- **WHEN** ToolFilter 执行检索，索引正常返回空匹配（如用户消息为"早上好"）
- **THEN** Filter 将空 `map[string]bool{}` 写入缓存，MCP 工具返回 `false`，Skill 工具仍返回 `true`，LLM 仅看到框架工具和 Skill 工具

#### Scenario: 系统错误时降级全量放行

- **WHEN** 无法从 context 获取 Invocation，或 Invocation 的用户消息内容为空
- **THEN** Filter 对所有工具返回 `true`，LLM 看到全量工具

### Requirement: 动态工具加载配置

系统 SHALL 在 `tools` 配置节（`AgentToolsConfig`）下支持 `dynamicToolLoading` 子配置，包含以下字段：

- `enabled`（bool）：是否启用动态工具过滤，默认 `false`
- `strategy`（string）：检索策略，可选值 `keyword` / `bm25`，默认 `bm25`
- `topN`（int）：返回 Top-N 个最相关工具，必须 > 0（`enabled: true` 时），默认 `10`
- `scoreThreshold`（float64）：相对分数阈值，范围 0.0~1.0，检索结果中得分 < `maxScore * scoreThreshold` 的工具被过滤。默认 `0`（不启用阈值，仅靠 topN 截断）
- `toolTags`（`map[string][]string`）：工具标签映射，key 为 MCP 原始 tool name（不含 ToolSet 前缀），value 为标签列表

配置 SHALL 通过 `toolsToLogicsConfig()` 从 cc 层转换到 logics 层的 `ToolsConfig` 中。

#### Scenario: 配置解析与生效

- **WHEN** `agent_server.yaml` 中配置 `tools.dynamicToolLoading.enabled: true, strategy: bm25, topN: 10`
- **THEN** 系统使用 BM25 策略构建索引，ToolFilter 检索后返回最多 10 个匹配工具

#### Scenario: 默认配置

- **WHEN** `dynamicToolLoading` 配置节不存在或为空
- **THEN** `enabled` 默认为 `false`，不注入 ToolFilter

#### Scenario: toolTags 增强检索

- **WHEN** 配置 `toolTags: { list_cvms: ["CVM", "云服务器", "主机"] }`
- **THEN** `list_cvms` 的 `ToolMeta.Tags` 包含这三个标签，`SearchText` 中包含这些标签文本，BM25/Keyword 检索时可通过这些标签命中该工具

#### Scenario: scoreThreshold 过滤低分结果

- **WHEN** 配置 `scoreThreshold: 0.3`，检索结果中最高分为 8.5
- **THEN** 仅保留得分 >= 2.55（8.5 * 0.3）的工具，低于此分数的结果被过滤，再取 topN

#### Scenario: scoreThreshold 为 0 不启用阈值

- **WHEN** 配置 `scoreThreshold: 0`（或未配置）
- **THEN** 检索结果不做分数过滤，直接取 topN

#### Scenario: topN 校验

- **WHEN** `enabled: true` 且 `topN` 配置为 0 或负数
- **THEN** 系统使用默认值 10

### Requirement: 索引使用框架前缀名

`lazyToolIndex` 构建索引时 SHALL 使用框架 `NamedToolSet` 的前缀命名规则。对于每个 MCP 工具，索引中的工具名 SHALL 为 `{ToolSet.Name()}_{RawToolName}`（当 `ToolSet.Name()` 非空时），确保与 Per-Run ToolFilter 收到的 `tool.Declaration().Name` 一致。

`mcpToolNames` 集合 SHALL 存储前缀名，用于在 Filter 中区分 MCP 工具和 Skill 工具。

`toolTags` 配置的键 SHALL 使用 MCP 原始工具名（不含前缀），`lazyToolIndex` 在构建时 SHALL 自动通过原始名查找 `toolTags[rawName]`，将标签注入到以前缀名为 key 的 `ToolMeta` 中。

#### Scenario: 带前缀的 ToolSet 索引构建

- **WHEN** MCP ToolSet 的 `Name()` 返回 `"bkaidev"`，其中包含原始工具 `list_cvms`
- **THEN** 索引中该工具的 name 为 `"bkaidev_list_cvms"`，`mcpToolNames` 集合包含 `"bkaidev_list_cvms"`

#### Scenario: 无前缀的 ToolSet 索引构建

- **WHEN** MCP ToolSet 的 `Name()` 返回 `""`，其中包含原始工具 `list_cvms`
- **THEN** 索引中该工具的 name 为 `"list_cvms"`

#### Scenario: toolTags 通过原始名匹配

- **WHEN** 配置 `toolTags: { list_cvms: ["CVM", "云服务器"] }`，ToolSet.Name() 为 `"bkaidev"`
- **THEN** 构建索引时，原始名 `list_cvms` 匹配到 Tags `["CVM", "云服务器"]`，注入到前缀名 `"bkaidev_list_cvms"` 的 ToolMeta.Tags 中

#### Scenario: ToolFilter 使用前缀名匹配

- **WHEN** Per-Run ToolFilter 收到工具名 `"bkaidev_list_cvms"`
- **THEN** Filter 在 `mcpToolNames` 中找到该前缀名，确认为 MCP 工具，进入检索结果白名单判断

### Requirement: 多模态消息文本提取

ToolFilter 提取查询文本时 SHALL 兼容纯文本和多模态两种消息格式：
- 优先使用 `inv.Message.Content`（纯文本消息）
- 若 `Content` 为空，SHALL 遍历 `inv.Message.ContentParts`，提取所有 `ContentTypeText` 类型部分的文本并拼接

#### Scenario: 纯文本消息提取

- **WHEN** 用户消息 `inv.Message.Content` 为 "查一下 CVM 主机"
- **THEN** Filter 使用 "查一下 CVM 主机" 作为检索查询

#### Scenario: 多模态消息提取

- **WHEN** 用户消息 `inv.Message.Content` 为空，`inv.Message.ContentParts` 包含 `{Type: ContentTypeText, Text: "这张图里的服务器"}` 和 `{Type: ContentTypeImage, ...}`
- **THEN** Filter 提取文本部分 "这张图里的服务器" 作为检索查询，忽略图片部分

#### Scenario: 多模态多段文本拼接

- **WHEN** `ContentParts` 中包含多个 `ContentTypeText` 部分：`"帮我查"` 和 `"这些主机"`
- **THEN** Filter 拼接为 "帮我查 这些主机" 作为检索查询
