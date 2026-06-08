# Spec: tool-proxy

## Purpose

定义 agent-server 的 MCP 工具代理（Tool Proxy）能力。通过 3 个元工具（`search_tools`、`get_tool_schema`、`execute_tool`）替代将全部 MCP 工具定义注入 LLM 上下文，由 Agent 经语义搜索动态发现工具并按 schema 执行，降低 Token 消耗并提升工具选择准确率。MVP 仅覆盖 Graph 模式（`agui.mode=graph`）。

## Requirements

### Requirement: search_tools 元工具

`search_tools` 元工具根据用户的任务描述，通过语义搜索找到最相关的 MCP 工具，并直接返回工具的完整 schema。

#### Scenario: 正常搜索工具

- **GIVEN** 工具注册表中已有工具 `search_code`（描述为"在代码库中搜索代码"），且该工具的向量化表示已存储
- **WHEN** Agent 调用 `search_tools` 并传入 `query="在代码中搜索包含 TODO 的文件"`
- **THEN** 返回包含 `search_code` 的列表，含 `name`、`description`、`relevance_score`、`schema`、`schema_token`
- **AND** `search_code` 的 `relevance_score` > 0.5，且 `schema_token` 可直接用于后续 `execute_tool`

#### Scenario: 返回结果数量限制

- **GIVEN** 工具注册表中有 20 个与查询相关的工具
- **WHEN** Agent 调用 `search_tools` 并传入 `query="搜索代码"` 且 `top_k=5`
- **THEN** 返回结果数量为 5

#### Scenario: 无相关工具

- **GIVEN** 工具注册表中没有任何与"天气预报"相关的工具
- **WHEN** Agent 调用 `search_tools` 并传入 `query="今天天气怎么样"`
- **THEN** 返回空列表 `tools: []`，且 `success` 为 `true`

### Requirement: get_tool_schema 元工具

`get_tool_schema` 元工具根据工具名称，返回工具的完整调用 schema（JSON Schema 格式）。

#### Scenario: 查询已有工具的 schema

- **GIVEN** 工具注册表中存在工具 `search_code`
- **WHEN** Agent 调用 `get_tool_schema` 并传入 `tool_name="search_code"`
- **THEN** 返回 `success=true`，含完整 schema（`properties`、`required` 等）及可用于 `execute_tool` 的 `schema_token`

#### Scenario: 查询不存在的工具

- **GIVEN** 工具注册表中不存在工具 `nonexistent_tool`
- **WHEN** Agent 调用 `get_tool_schema` 并传入 `tool_name="nonexistent_tool"`
- **THEN** 返回 `success=false`，错误信息明确提示"工具不存在"

### Requirement: execute_tool 元工具

`execute_tool` 元工具根据工具名称、参数和 `schema_token` 调用实际的 MCP 工具并返回执行结果。调用前须完成三层校验：schema_token 校验 → 严格参数校验 → 执行。

#### Scenario: 正常执行工具

- **GIVEN** 工具注册表中存在工具 `search_code`，Agent 已持有有效 `schema_token`，schema 要求 `query` 为必需参数
- **WHEN** Agent 调用 `execute_tool` 并传入 `tool_name="search_code"`、`parameters={"query": "TODO"}` 和有效 `schema_token`
- **THEN** 实际调用 MCP 工具 `search_code`，返回 `success=true` 及执行结果

#### Scenario: schema_token 缺失或无效

- **GIVEN** 工具注册表中存在工具 `search_code`
- **WHEN** Agent 调用 `execute_tool` 时未传入 `schema_token`，或传入了无效/伪造的 token
- **THEN** 返回 `success=false`，错误类型为 `schema_token_invalid`，响应含 `required_schema` 及提示须先调用 `search_tools` 或 `get_tool_schema`

#### Scenario: 参数验证失败（缺少必填字段）

- **GIVEN** 工具 `search_code` 的 schema 要求 `query` 为必需参数，Agent 已持有有效 `schema_token`
- **WHEN** Agent 调用 `execute_tool` 并传入 `parameters={}` 和有效 `schema_token`
- **THEN** 返回 `success=false`，错误类型为 `invalid_parameters`，提示 "`query` 是必需的"，响应含 `required_schema`

#### Scenario: 参数包含 schema 未定义的 key（幻觉参数）

- **GIVEN** 工具 `search_code` 的 schema 仅定义 `query` 字段，Agent 已持有有效 `schema_token`
- **WHEN** Agent 调用 `execute_tool` 并传入 `parameters={"query": "TODO", "hallucinated_field": "x"}`
- **THEN** 返回 `success=false`，错误类型为 `invalid_parameters`，提示 `"hallucinated_field" 不在工具 schema 定义中`，响应含 `required_schema`，且实际 MCP 工具未被调用

#### Scenario: MCP 工具调用失败

- **GIVEN** 工具 `search_code` 的 MCP Server 不可用，Agent 已持有有效 `schema_token` 且参数合法
- **WHEN** Agent 调用 `execute_tool` 并传入有效参数
- **THEN** 返回 `success=false`，错误类型为 `execution_error`，包含原始错误信息

### Requirement: 工具注册表管理

工具注册表维护所有 MCP 工具的元数据（名称、描述、schema、类别等），支持工具的注册、更新、删除、查询。

#### Scenario: 初始化时加载所有工具

- **GIVEN** MCP ToolSet 配置中有 3 个 ToolSet，共包含 50 个工具
- **WHEN** `ToolRegistry` 初始化时
- **THEN** 所有 50 个工具的元数据被加载到内存注册表，且每个工具的 `SearchText` 正确构建（name + description + tags + parameter names）

#### Scenario: 查询工具信息

- **GIVEN** 工具注册表中存在工具 `search_code`
- **WHEN** 调用 `GetTool("search_code")`
- **THEN** 返回完整 `ToolMetadata`，含 `Name`、`Description`、`Schema`、`Category` 等字段

#### Scenario: 工具不存在时查询

- **GIVEN** 工具注册表中不存在工具 `nonexistent_tool`
- **WHEN** 调用 `GetTool("nonexistent_tool")`
- **THEN** 返回错误，错误信息包含工具名称

### Requirement: 工具向量化存储和语义检索

使用 Embedding 模型将工具定义向量化，并存储到向量数据库，支持语义搜索。

#### Scenario: 工具向量化

- **GIVEN** 工具 `search_code` 的 `SearchText` 已构建
- **WHEN** 调用 `EmbeddingIndex.Build()` 进行向量化
- **THEN** 该工具的向量表示被正确计算并存储，向量维度与 Embedding 模型输出维度一致

#### Scenario: 语义搜索返回相关工具

- **GIVEN** 向量数据库中已存储 50 个工具的向量
- **WHEN** 调用 `Search(query="在代码中搜索包含 TODO 的文件", topK=5)`
- **THEN** 返回评分最高的 5 个工具，且 `search_code` 评分排在前 3 位

#### Scenario: 查询向量化失败的处理

- **GIVEN** Embedding 服务不可用
- **WHEN** 调用 `EmbeddingIndex.Build()`
- **THEN** 返回错误并明确提示 Embedding 调用失败，且不影响已加载的工具元数据

### Requirement: 将 3 个元工具注册到 Agent（Graph 模式 MVP）

将 `search_tools`、`get_tool_schema`、`execute_tool` 作为元工具注册到 Graph Agent，替代直接注册所有 MCP 工具。LLM Agent 模式不在 MVP 范围内。

#### Scenario: Graph 模式注册元工具

- **GIVEN** Agent 运行在 Graph 模式（`agui.mode=graph`）
- **WHEN** `BuildGraph` 编译图时
- **THEN** 3 个元工具被注册到 LLM Node 与 Tool Node，原有 MCP 工具不通过 `WithToolSets(toolset.TS)` 暴露，skill 工具与 `human_confirm` 仍正常注册

#### Scenario: Graph 模式不注入 dynamicToolLoading

- **GIVEN** Graph 模式已启用 Tool Proxy
- **WHEN** 用户发起一次 Run
- **THEN** 不注入 `agent.WithToolFilter`

#### Scenario: 启动时预加载工具注册表

- **GIVEN** `global_config` 已配置 `auth/access_token`（含 `bk-hcm`），`initVirtualUser=bk-hcm`，MCP 共 50 个工具
- **WHEN** agent-server 启动并调用 `ToolProxy.Build(initCtx)`
- **THEN** 使用 `bk-hcm` token 拉取全部工具元数据、完成向量化，用户首次 Run 调用 `search_tools` 时无需等待索引构建

#### Scenario: initVirtualUser 无对应 token

- **GIVEN** `access_token` 仅包含 `other-user`，`initVirtualUser=bk-hcm`
- **WHEN** agent-server 启动
- **THEN** Tool Proxy 构建失败并提示 virtual-user `bk-hcm` 无对应 token

#### Scenario: 启动时 access_token 配置缺失

- **GIVEN** `global_config` 中未配置 `auth/access_token`
- **WHEN** agent-server 启动
- **THEN** Tool Proxy 构建失败并记录错误日志，根据 `toolProxy.required` 决定拒绝启动或降级为全量 MCP 注册

#### Scenario: 定时刷新工具注册表

- **GIVEN** Tool Proxy 已成功启动，`refreshInterval=30m`，`initVirtualUser=bk-hcm`
- **WHEN** 定时器触发刷新
- **THEN** 重新拉取 MCP 工具列表并增量更新 Registry 与 VectorStore，刷新期间 `search_tools` 仍可服务

### Requirement: 端到端流程可跑通

验证完整的「搜索 → 执行」流程可以在 Agent 中正常运行。

#### Scenario: 完整流程 - 搜索工具并执行

- **GIVEN** Agent 已启动并加载了 3 个元工具
- **WHEN** 用户发送消息："帮我在代码中搜索包含 TODO 的文件"
- **THEN** Agent 调用 `search_tools` 获取 `search_code` 及 schema，再调用 `execute_tool` 并返回搜索结果

#### Scenario: 完整流程 - 先查询 schema 再执行

- **GIVEN** Agent 已启动并加载了 3 个元工具
- **WHEN** 用户发送消息："使用 search_code 工具搜索 TODO"
- **THEN** Agent 先调用 `get_tool_schema` 获取 schema 和 `schema_token`，再调用 `execute_tool` 并返回搜索结果

### Requirement: Schema Token 强制机制

通过无状态 HMAC Schema Token，工程层强制 LLM 在调用 `execute_tool` 前必须先获取工具 schema。

#### Scenario: schema_token 由 search_tools 返回

- **GIVEN** 工具注册表中存在工具 `search_code`
- **WHEN** Agent 调用 `search_tools` 并获取到 `search_code`
- **THEN** 返回结果中每个工具项包含可直接用于 `execute_tool` 的 `schema_token`

#### Scenario: schema_token 由 get_tool_schema 返回

- **GIVEN** 工具注册表中存在工具 `search_code`
- **WHEN** Agent 调用 `get_tool_schema(tool_name="search_code")`
- **THEN** 返回结果中包含可直接用于 `execute_tool` 的 `schema_token`

#### Scenario: schema_token 在服务重启后失效

- **GIVEN** Agent 持有重启前生成的旧 `schema_token`
- **WHEN** 服务重启后 Agent 使用旧 token 调用 `execute_tool`
- **THEN** 返回 `success=false`，错误类型为 `schema_token_invalid`，响应含 `required_schema` 供 Agent 重新获取
