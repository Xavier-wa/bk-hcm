## Context

当前 agent-server 的 AGUI Runner 使用 `mcp_proxy.go` 中的代理层（`list_tools` / `call_tool`）包装 MCP 工具。该代理层的设计初衷是通过渐进式加载减少 token 消耗，但实际引入了非标准协议、额外 Step 轮次和调用准确率下降等问题。

**现有架构：**
- MCP ToolSet → `wrapMCPToolSetsAsProxy()` → 2 个代理工具（list_tools / call_tool）→ LLM 间接调用
- 框架 `ToolFilter` 只能看到代理工具，无法感知真实 MCP 工具名

**目标架构：**
- MCP ToolSet → 直接注册到框架 → `ToolFilter`（检索索引过滤）→ LLM 直接调用真实工具
- `ToolFilter` 能看到每个真实工具名，支持精确到单工具级别的检索过滤

**关键约束：**
- 框架的 `getFilteredTools()` 对 ToolFilter 结果有 **Invocation 级缓存**：首次 Step 执行 Filter，后续 Step 命中缓存。此行为恰好满足"每条消息检索一次"的需求
- bkaidev 类型 MCP server 需要请求上下文中的 `bk_ticket` 才能连接（`refreshOnRun = true`），工具列表在启动时不可用，索引需支持懒构建
- 框架工具 `transfer_to_agent`、`knowledge_search`、`agentic_knowledge_search` 永远不被 ToolFilter 过滤
- **已验证**：框架 `getFilteredTools()` 内部执行顺序为「先获取 allTools（触发 MCP 连接和 listTools）→ 再逐个调用 ToolFilter」，因此 ToolFilter 执行时 MCP 连接已就绪，`ts.Tools(ctx)` 可安全调用

## Goals / Non-Goals

**Goals:**
- 移除 `mcp_proxy.go` 代理层，MCP 工具直接注册到框架
- 实现基于元数据的工具索引与检索（P0：Keyword + BM25）
- 通过框架标准 `WithToolFilter` 注入动态过滤，Per-Run 级别检索
- 支持 `toolTags` 配置增强检索命中率
- 完善降级策略，确保任何异常场景下系统可用

**Non-Goals:**
- Step 级动态工具追加（方案文档第 4 章，后续扩展方向）
- Embedding 向量语义检索（P2）和 Hybrid 混合检索（P3）
- MCP server 工具列表变更时的自动索引刷新（当前使用 `sync.Once` 构建一次）
- 引入 jieba 等外部中文分词库（当前使用混合 n-gram 分词，无外部依赖）

## Decisions

### 决策 1：移除 Proxy 代理层，直接注册 MCP 工具

**选择**：完全移除 `mcp_proxy.go`，MCP ToolSet 直接通过 `llmagent.WithToolSets(mcpToolSets)` 注册。

**理由**：
- 消除非标准的 list → schema → call 三步调用协议，每次工具调用从 3~4 Step 降为 1 Step
- LLM 直接看到工具完整 schema，提升调用准确率
- 框架 `ToolFilter` 能感知真实工具名，为动态过滤提供前提

**备选方案**：保留 Proxy 但在其上层加 ToolFilter → 放弃，因为 ToolFilter 只能看到代理工具名（`list_tools`/`call_tool`），无法实现工具级过滤。

### 决策 2：检索策略选择 BM25 作为默认

**选择**：P0 阶段实现 KeywordIndex 和 BM25Index，默认使用 BM25。

**理由**：
- BM25 纯 Go 实现，零外部依赖，延迟 < 1ms
- 考虑了词频（TF）、逆文档频率（IDF）和文档长度归一化，比简单关键词匹配更精确
- 工具数量通常 50~200，BM25 在此量级表现良好
- Tags 配置可补充标准化关键词，弥补 BM25 在语义理解上的不足

**备选方案**：
- 仅 Keyword 匹配 → 过于简单，工具描述多样时召回率不足
- Embedding 向量检索 → 需要外部 Embedding API，增加延迟和外部依赖，作为 P2 后续演进
- 小模型意图分类（v1 方案）→ 每次请求 200ms~1s 延迟，需手动维护场景映射，已在 v1 评估中否决

### 决策 3：中文分词策略采用混合 n-gram

**选择**：`tokenize` 函数对英文按空格/下划线拆分并转小写，对中文输出 unigram + bigram（混合 1+2-gram）。

**理由**：
- 纯算法实现，零外部依赖，零维护成本
- BM25 的 IDF 天然消噪——高频单字（如"的""有"）IDF 低，不会污染排序；有意义的 bigram（如"云服""服务"）IDF 高，排序靠前
- 与 Tags 互补——Tags 解决工具侧的同义词注入（用户说"主机"→ 命中工具 Tags 中的"主机"），n-gram 解决子词级别的模糊匹配
- 实现简单，tokenize 函数约 20 行

**备选方案**：
- 纯单字切分 → BM25 下噪声大，常见字 IDF 低导致排序质量差
- 纯 2-gram → 丢失单字精确匹配（如"CVM"中的"C"匹配查询中的"C"）
- jieba 等外部分词库 → 引入 CGo 依赖或纯 Go 词典文件，增加构建和维护成本，P0 不引入
- 小词典前向最大匹配 → 需维护词典，与 Tags 功能重叠

### 决策 4：ToolFilter 集成方式

**选择**：使用 Per-Run 级 `agent.WithToolFilter()` 通过 `RunOptionResolver` 注入，结合 Invocation state 自定义缓存。

**理由**：
- 动态工具过滤是 Per-Run（每条用户消息）行为，Per-Run 注入语义更准确
- 框架对 `RunOptions.ToolFilter` 自动豁免框架工具（`transfer_to_agent`、`knowledge_search` 等），Filter 无需手动处理豁免逻辑
- 框架的 Invocation 级缓存保证同一消息内工具集稳定
- 自定义缓存（`custom:retrieved_tools`）避免 Filter 对每个 tool 重复检索（Filter 对每个 tool 调用一次，但检索只需做一次）
- `service.go` 中已有 `RunOptionResolver`（用于 Per-Run 模型选择），将 ToolFilter 合入同一 resolver 即可，耦合极小

**实现方式**：
- `logics.Runtime` 新增 `DynamicToolFilter() tool.FilterFunc` 方法，返回已构建好的 filter 函数（或 nil）
- `service.New()` 中的 `RunOptionResolver` 合并模型选择和工具过滤两个 RunOption
- filter 函数内部通过 `agent.InvocationFromContext(ctx)` 获取当前 Invocation，从用户消息提取查询并检索

**备选方案**：使用 `llmagent.WithToolFilter()` Per-Agent 级注入 → 放弃，因为 Agent 级 Filter 作用于所有工具（含框架工具），需要手动维护豁免列表；且语义上动态过滤是 Per-Run 行为，不应绑定到 Agent 生命周期。

### 决策 5：索引构建时机采用懒构建

**选择**：使用 `sync.Once` 在首次 ToolFilter 执行时懒构建索引。

**理由**：
- bkaidev 类型 MCP server 在启动时工具列表不可用（需请求上下文中的 bk_ticket）
- `sync.Once` 保证线程安全且只构建一次
- 首次请求时索引构建为纯内存操作，耗时极短

**备选方案**：启动时构建 → 只适用于 stdio 类型 MCP server，无法覆盖 bkaidev 场景。

### 决策 6：索引使用框架前缀名

**选择**：`lazyToolIndex` 构建索引时，使用框架 `NamedToolSet` 的前缀命名规则（`{ToolSetName}_{RawToolName}`），确保索引中的工具名与 Per-Run ToolFilter 收到的工具名一致。`toolTags` 配置键使用 MCP 原始工具名（不含前缀），由 `lazyToolIndex` 在构建时自动匹配。

**理由**：
- 框架的 `NamedToolSet` 会为有 `Name()` 的 ToolSet 中的工具自动添加前缀（格式：`name + "_" + rawName`）
- Per-Run `ToolFilter` 收到的 `tool.Declaration().Name` 是前缀后的名称（如 `my-mcp_list_cvms`）
- 但 `ts.Tools(ctx)` 返回的是原始 MCP 工具（`Declaration().Name` 为 `list_cvms`）
- 因此索引构建时需主动拼接前缀：`frameworkName = ts.Name() + "_" + rawName`（当 `ts.Name()` 非空时）
- `mcpToolNames` 集合和检索结果白名单均使用前缀名，确保 Filter 匹配正确

**`toolTags` 键的解析规则**：
- 运维在 YAML 中使用原始工具名（如 `list_cvms`），无需关心 ToolSet 前缀
- `lazyToolIndex` 构建时通过原始名查找 `toolTags[rawName]`，将标签注入到以前缀名为 key 的 ToolMeta 中

### 决策 7：查询文本提取采用多轮上下文窗口 + Session Summary 补充

**选择**：ToolFilter 提取查询文本时，从 `inv.Session.Events` 中回看最近 N 条用户消息（`QueryContextWindow`，默认 3），拼接为查询文本。当 Session 有 Summary 时，将 Summary 文本追加到查询前部作为全局语义补充。单条消息的文本提取兼容纯文本（`Content`）和多模态（`ContentParts` 中 `ContentTypeText` 拼接）两种格式。

**理由**：
- 多轮对话中，用户可能发送"继续"、"可以"、"好的"等短消息，仅凭当前消息无法匹配到任何工具
- 工具需求可能跨越多轮对话——例如"买机器→查机型→确认购买"，第三轮需要的是第一轮的购买工具，而非第二轮的查询工具
- 滑动窗口拼接多轮用户消息后，BM25 的 TF-IDF 机制天然地让信息密度高的消息贡献更多权重，无需人为区分"长/短消息"
- Session Summary 补充窗口外的历史语义，覆盖超过 N 轮的长对话场景
- Summary 依赖 Session 摘要功能已启用（`SessionSummaryConfig.Enabled`），未启用时优雅降级为纯窗口模式

**查询构建流程**：
1. 从 `inv.Session.Events` 倒序扫描，提取最近 `QueryContextWindow - 1` 条历史用户消息（当前消息算 1 条）
2. 若 Session 有 Summary 文本（通过 `SessionService.GetSessionSummaryText`），将其作为查询前缀
3. 按时间正序拼接：`[Summary] + [历史用户消息...] + [当前用户消息]`
4. Session 为 nil 或 `QueryContextWindow <= 1` 时退化为仅用当前消息（兼容无 Session 场景）

**备选方案**：
- 仅用当前消息（原设计）→ 多轮对话中短消息检索失效，已否决
- 上一轮工具缓存 fallback → "子任务→回到主任务"模式下失效（缓存的是子任务工具而非主任务工具），已否决
- 仅短消息时 fallback → 需要定义"短消息"阈值，边界模糊且无法覆盖"第三轮需要第一轮工具"的场景

### 决策 8：新增文件组织

**选择**：新增两个文件，按职责分离：
- `cmd/agent-server/logics/tool_index.go`：ToolMeta 结构、ToolIndex 接口、KeywordIndex、BM25Index 实现、元数据提取函数
- `cmd/agent-server/logics/tool_filter.go`：DynamicToolLoadingConfig、makeDynamicToolFilter、lazyToolIndex

**理由**：索引逻辑（数据结构 + 算法）与过滤逻辑（框架集成 + 配置）关注点不同，分开便于后续独立演进（如增加 EmbeddingIndex 时只需修改 `tool_index.go`）。

### 决策 9：配置归属与转换链

**选择**：配置放在 `AgentToolsConfig`（cc 层）/ `ToolsConfig`（logics 层），通过 `toolsToLogicsConfig()` 转换。

**理由**：
- 动态工具加载本质上是工具的过滤策略，与 MCP ToolSets 和 Skills 同属工具层面的关注点，放 `AgentToolsConfig` 比 `AgentAGUI` 更符合语义
- YAML 配置嵌套在 `tools.dynamicToolLoading` 下，运维在工具配置区域内统一管理
- 转换链 `cc.AgentToolsConfig` → `toolsToLogicsConfig()` → `logics.ToolsConfig`，在 `logics.New()` 中与 `mcpToolSets` 同一作用域内，构建 `lazyToolIndex` 时可直接拿到两者
- `logics.Runtime` 通过 `DynamicToolFilter()` 方法向 service 层暴露已构建好的 filter 函数，service 层在 `RunOptionResolver` 中注入，职责边界清晰
- `toolTags` 作为 `map[string][]string` 配置，运维可按需为工具补充检索标签

**备选方案**：放在 `AgentAGUI` / `AGUIServiceConfig` → 放弃，因为 AGUI 配置的定位是 UI/交互层（AppName、AllowedModels、Stream、Prompt），混入工具过滤策略会造成关注点混乱。

### 决策 10：Skill 工具和框架工具不参与动态过滤

**选择**：ToolFilter 仅过滤 MCP 工具，Skill 工具和框架内置工具直接放行。

**理由**：
- Skill 工具通过 `llmagent.WithSkills(skillRepo)` 独立注册，其加载/选择由框架的 Skill 机制管理，不属于 MCP ToolSet 范畴
- 框架内置工具（`transfer_to_agent`、`knowledge_search` 等）由框架对 `RunOptions.ToolFilter` 自动豁免，无需 Filter 处理

**实现方式**：由于使用 Per-Run 级 `agent.WithToolFilter()`（决策 3），框架自动将工具分为"用户工具"和"框架工具"，仅对用户工具执行 Filter。Filter 内部通过 `lazyToolIndex` 构建索引时记录的 MCP 工具名集合（`mcpToolNames`）区分 MCP 工具和 Skill 工具：不在集合中的用户工具（即 Skill 工具）直接放行。

### 决策 11：降级策略区分"系统错误"和"无匹配结果"

**选择**：
- **系统错误**（索引构建失败、无法获取 Invocation、无法获取用户消息）→ 全量放行所有 MCP 工具
- **检索无结果**（BM25/Keyword 正常执行但无匹配）→ 不放行 MCP 工具，LLM 仅看到 Skill 和框架工具

**理由**：
- 检索无结果通常意味着用户消息与工具无关（如"早上好""谢谢"等纯聊天），此时不暴露 MCP 工具可节省 token，且 LLM 会正确地以纯文本回复
- 如果是 Tags 配置不足导致的假阴性（如用户说"查主机"但未配"主机" tag），这是运维配置问题，不应靠全量放行来掩盖
- 系统错误是非预期故障，全量放行保障可用性

**缓存行为**：检索无结果时写入空 `map[string]bool{}`，同一 Invocation 内后续 tool 查缓存返回 `false`，MCP 工具被正确过滤；Skill 工具不在 `mcpToolNames` 中，直接放行。

### 决策 12：`sync.Once` 懒构建失败不重试

**选择**：P0 阶段接受 `sync.Once` 的一次性语义，构建失败时降级为全量放行。

**理由**：
- `refreshOnRun = true` 时，框架保证第一次 Run 时 MCP 连接可用，ToolFilter 在 Run 内部执行，此时 MCP 已连接成功，构建失败概率极低
- 即使构建失败，降级全量放行不影响系统可用性，只是失去过滤优化
- 后续可扩展为可重试的 once 模式（如 `sync.Mutex` + error check），作为后续优化方向

### 决策 13：检索结果使用相对分数阈值过滤

**选择**：新增 `scoreThreshold`（float64，0.0~1.0，默认 0）配置。检索完成后，取结果中的最高分 `maxScore`，过滤掉得分 < `maxScore * scoreThreshold` 的结果，再取 topN。

**理由**：
- topN 只控制数量上限，不控制质量——topN=10 时可能包含 BM25 得分极低的噪声工具
- BM25 分数是绝对值、依赖语料，不适合配置固定阈值；相对阈值（占最高分的比例）与语料无关，运维更易理解
- 默认 0 表示不启用阈值（只靠 topN），保持向后兼容

**执行顺序**：`ToolIndex.Search(query, topN)` 内部先按 BM25/Keyword 评分排序 → 应用 `scoreThreshold` 过滤 → 取 topN → 返回。

### 决策 14：topN 强制大于 0

**选择**：当 `enabled: true` 时，`topN` 必须 > 0。配置校验阶段对 `topN <= 0` 报错或使用默认值 10。

**理由**：
- `topN: 0`（全量放行）与 `enabled: false`（不注入 Filter）效果相同，但多了索引构建和检索开销，语义冗余
- 强制 > 0 消除歧义，简化代码路径

### 决策 15：索引部分构建失败时容忍

**选择**：`lazyToolIndex.ensureBuild` 遍历多个 MCP ToolSet 时，若某个 ToolSet 的 `ts.Tools(ctx)` 失败（如连接超时），跳过该 ToolSet 并继续构建其余部分。只要至少有一个工具成功提取，索引即视为构建成功。全部 ToolSet 均失败时，视为构建失败，降级全量放行。

**理由**：
- 多个 MCP ToolSet（如 stdio + bkaidev）中一个失败不应阻塞另一个的过滤能力
- 部分索引好过无索引——成功部分的工具可被正确过滤，失败部分的工具因不在 `mcpToolNames` 中会被 Filter 直接放行（等同于 Skill 工具的处理路径），不会被误过滤
- 失败 ToolSet 的工具「意外放行」是安全的降级行为

### 决策 16：多轮查询上下文窗口配置

**选择**：新增 `QueryContextWindow int`（默认 3）配置项，控制查询文本回看的用户消息条数。

**理由**：
- 窗口大小影响检索的召回率与噪声比。N=1 等同于仅用当前消息（多轮场景失效），N=3 覆盖大多数"主任务→子任务→回到主任务"的对话模式，N=10+ 可能引入过多噪声导致 BM25 区分度下降
- 做成可配置项，运维可根据实际对话模式调优
- 默认 3 是基于典型任务型对话（1~3 轮完成一个意图）的经验值

### 决策 17：Session Summary 作为全局语义补充

**选择**：查询文本构建时，若 `inv.Session` 有 Summary 文本，将其作为查询前缀拼接到历史消息窗口前面。

**理由**：
- 滑动窗口只覆盖最近 N 轮，长对话中早期建立的意图可能滑出窗口（如第 1 轮"帮我买一台机器"在第 6 轮时已不在 N=3 的窗口内）
- Session Summary 是 LLM 生成的对话摘要，语义密度高，能捕获整个对话的主题和关键意图
- Summary 并非始终可用（依赖 `SessionSummaryConfig.Enabled` 且需要至少一次摘要触发），因此为可选补充，不可用时优雅降级为纯窗口模式
- Summary 放在查询最前面，BM25 中窗口内的最近消息仍贡献更多 TF 权重（因为与当前意图更紧密），Summary 提供的是低频但关键的全局上下文

**获取方式**：通过 `inv.Session.Summaries` 直接读取（无需调用 `SessionService`，因为 Runner 在构建 Invocation 时已将 Session 加载到内存中），取 `SummaryFilterKeyAllContents`（空字符串 key）对应的全局摘要。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|----------|
| **检索召回率不足**：相关工具未被检索命中，LLM 无法完成任务 | topN 设为合理值（默认 10）；toolTags 补充关键词；多轮查询窗口 + Summary 补充上下文；检索无结果时不放行 MCP 工具（区分于系统错误降级） |
| **工具名前缀不匹配**：索引用原始名但 Filter 收到前缀名 | `lazyToolIndex` 构建索引时主动拼接 ToolSet 前缀，确保与框架一致 |
| **移除 Proxy 后 token 可能增加**：全量工具 schema 直接暴露给 LLM（降级全量模式下） | ToolFilter + topN 控制上限；直接调用省去 list/call 的 Step token 开销，整体仍优于 Proxy |
| **中文分词质量影响 BM25 精度** | 采用混合 n-gram（unigram + bigram）分词，BM25 IDF 天然消噪；toolTags 补充标准化中文关键词进一步弥补 |
| **索引与工具列表不同步**：MCP server 动态增减工具后索引过时 | `sync.Once` 懒构建保证首次请求最新；后续可扩展定期刷新机制 |
| **Invocation state key 硬编码** | `custom:retrieved_tools` 定义为包级常量并加注释，框架升级时 review |
| **多轮窗口引入噪声**：拼接多条历史消息后查询文本过长，BM25 区分度下降 | 默认 N=3 控制噪声；BM25 的 IDF 天然降低高频无意义词的权重；topN + scoreThreshold 过滤低分工具 |
| **Session Summary 不可用**：摘要功能未启用或首轮对话尚无摘要 | Summary 为可选补充，不可用时优雅降级为纯窗口模式，不影响基本功能 |
| **Session Events 为空**：新会话首条消息无历史可回看 | 退化为仅用当前消息，与原始行为一致 |

## Migration Plan

1. **部署前**：在 `agent_server.yaml` 中新增 `dynamicToolLoading` 配置节，默认 `enabled: false`
2. **灰度上线**：部署代码后，先保持 `enabled: false`（此时 Proxy 已移除，MCP 工具直接注册，全量暴露给 LLM），验证基线行为正常
3. **开启动态加载**：将 `enabled` 改为 `true`，配置 `strategy: bm25`、`topN: 10`，观察检索效果和 token 消耗变化
4. **Tags 调优**：根据实际使用情况，在 `toolTags` 中为高频工具补充中文关键词
5. **回滚策略**：将 `enabled` 改回 `false` 即可回到全量工具模式，无需回滚代码

## Open Questions

- 混合 n-gram 是否需要加入 trigram (3-gram)？当前计划使用 unigram + bigram，待上线后根据实际检索效果决定是否扩展
- `scoreThreshold` 默认值是否需要预设一个非零值（如 0.2）？当前默认 0（不启用），待上线后根据实际检索效果调优
- `queryContextWindow` 默认值 3 是否合适？需上线后根据实际对话模式（平均多少轮完成一个意图）观察调整
