# tool-proxy 能力规格

## ADDED Requirements

### Requirement: search_tools 元工具

`search_tools` 元工具根据用户的任务描述，通过语义搜索找到最相关的 MCP 工具，并直接返回工具的完整 schema。

#### Scenario: 正常搜索工具

Given 工具注册表中已有工具 `search_code`（描述为"在代码库中搜索代码"）
And 该工具的向量化表示已存储到向量数据库
When Agent 调用 `search_tools` 并传入 `query="在代码中搜索包含 TODO 的文件"`
Then 返回包含 `search_code` 工具的列表
And 返回结果包含工具的 `name`、`description`、`relevance_score`、`schema`、`schema_token`
And `search_code` 的 `relevance_score` > 0.5
And 返回的 `schema_token` 可直接用于后续 `execute_tool` 调用

#### Scenario: 返回结果数量限制

Given 工具注册表中有 20 个与查询相关的工具
When Agent 调用 `search_tools` 并传入 `query="搜索代码"` 且 `top_k=5`
Then 返回结果数量为 5

#### Scenario: 无相关工具

Given 工具注册表中没有任何与"天气预报"相关的工具
When Agent 调用 `search_tools` 并传入 `query="今天天气怎么样"`
Then 返回空列表 `tools: []`
And `success` 为 `true`

---
### Requirement: get_tool_schema 元工具

 元工具根据工具名称，返回工具的完整调用 schema（JSON Schema 格式）。

#### Scenario: 查询已有工具的 schema

Given 工具注册表中存在工具 `search_code`
When Agent 调用 `get_tool_schema` 并传入 `tool_name="search_code"`
Then 返回 `success=true`
And 返回结果包含 `search_code` 的完整 schema（包含 `properties`、`required` 等字段）
And 返回结果包含 `schema_token`，可直接用于后续 `execute_tool` 调用

#### Scenario: 查询不存在的工具

Given 工具注册表中不存在工具 `nonexistent_tool`
When Agent 调用 `get_tool_schema` 并传入 `tool_name="nonexistent_tool"`
Then 返回 `success=false`
And 错误信息明确提示"工具不存在"

---



### Requirement: execute_tool 元工具

`execute_tool` 元工具根据工具名称、参数和 schema_token 调用实际的 MCP 工具并返回执行结果。调用前须完成三层校验：schema_token 校验 → 严格参数校验 → 执行。

#### Scenario: 正常执行工具

Given 工具注册表中存在工具 `search_code`
And Agent 已通过 `search_tools` 或 `get_tool_schema` 获取了有效的 `schema_token`
And 该工具的 schema 要求 `query` 为必需参数
When Agent 调用 `execute_tool` 并传入 `tool_name="search_code"`、`parameters={"query": "TODO"}` 和有效 `schema_token`
Then 实际调用 MCP 工具 `search_code`
And 返回 `success=true`
And 返回结果包含工具的执行结果

#### Scenario: schema_token 缺失或无效

Given 工具注册表中存在工具 `search_code`
When Agent 调用 `execute_tool` 时未传入 `schema_token`，或传入了无效/伪造的 token
Then 返回 `success=false`
And 错误类型为 `schema_token_invalid`
And 错误响应中包含 `required_schema` 字段，附有该工具的完整 schema
And 错误信息提示"请先调用 search_tools 或 get_tool_schema 获取最新 schema 和 token"

#### Scenario: 参数验证失败（缺少必填字段）

Given 工具 `search_code` 的 schema 要求 `query` 为必需参数
And Agent 已持有有效 `schema_token`
When Agent 调用 `execute_tool` 并传入 `tool_name="search_code"`、`parameters={}` 和有效 `schema_token`
Then 返回 `success=false`
And 错误类型为 `invalid_parameters`
And 错误信息提示"`query` 是必需的"
And 错误响应中包含 `required_schema` 字段，附有该工具的完整 schema

#### Scenario: 参数包含 schema 未定义的 key（幻觉参数）

Given 工具 `search_code` 的 schema 仅定义 `query` 字段
And Agent 已持有有效 `schema_token`
When Agent 调用 `execute_tool` 并传入 `parameters={"query": "TODO", "hallucinated_field": "x"}`
Then 返回 `success=false`
And 错误类型为 `invalid_parameters`
And 错误信息包含 `"hallucinated_field" 不在工具 schema 定义中`
And 错误响应中包含 `required_schema` 字段，附有该工具的完整 schema
And 实际 MCP 工具未被调用

#### Scenario: MCP 工具调用失败

Given 工具 `search_code` 的 MCP Server 不可用
And Agent 已持有有效 `schema_token` 且参数合法
When Agent 调用 `execute_tool` 并传入有效参数
Then 返回 `success=false`
And 错误类型为 `execution_error`
And 包含原始错误信息

---

### Requirement: 工具注册表管理

工具注册表维护所有 MCP 工具的元数据（名称、描述、schema、类别等），支持工具的注册、更新、删除、查询。

#### Scenario: 初始化时加载所有工具

Given MCP ToolSet 配置中有 3 个 ToolSet，共包含 50 个工具
When `ToolRegistry` 初始化时
Then 所有 50 个工具的元数据被加载到内存注册表
And 每个工具的 `SearchText` 被正确构建（拼接 name + description + tags + parameter names）

#### Scenario: 查询工具信息

Given 工具注册表中存在工具 `search_code`
When 调用 `GetTool("search_code")`
Then 返回完整的 `ToolMetadata` 对象
And 包含 `Name`、`Description`、`Schema`、`Category` 等字段

#### Scenario: 工具不存在时查询

Given 工具注册表中不存在工具 `nonexistent_tool`
When 调用 `GetTool("nonexistent_tool")`
Then 返回错误，错误信息包含工具名称

---

### Requirement: 工具向量化存储和语义检索

使用 Embedding 模型将工具定义向量化，并存储到向量数据库，支持语义搜索。

#### Scenario: 工具向量化

Given 工具 `search_code` 的 `SearchText` 为 `"search_code 在代码库中搜索代码 query string description ..."`
When 调用 `EmbeddingIndex.Build()` 进行向量化
Then 该工具的向量表示被正确计算并存储
And 向量维度与 Embedding 模型输出维度一致

#### Scenario: 语义搜索返回相关工具

Given 向量数据库中已存储 50 个工具的向量
When 调用 `Search(query="在代码中搜索包含 TODO 的文件", topK=5)`
Then 返回评分最高的 5 个工具
And `search_code` 工具的评分排在前 3 位

#### Scenario: 查询向量化失败的处理

Given Embedding 服务不可用
When 调用 `EmbeddingIndex.Build()`
Then 返回错误，错误信息明确提示 Embedding 调用失败
And 不影响已加载的工具元数据（降级为仅关键词搜索）

---

### Requirement: 将 3 个元工具注册到 Agent（Graph 模式 MVP）

将 `search_tools`、`get_tool_schema`、`execute_tool` 作为元工具注册到 Graph Agent，替代直接注册所有 MCP 工具。LLM Agent 模式不在 MVP 范围内。

#### Scenario: Graph 模式注册元工具

Given Agent 运行在 Graph 模式（`agui.mode=graph`）
When `BuildGraph` 编译图时
Then `search_tools`、`get_tool_schema`、`execute_tool` 3 个元工具被注册到 LLM Node 与 Tool Node 的可用工具列表
And 原有 MCP 工具不通过 `WithToolSets(toolset.TS)` 暴露给 LLM
And skill 工具与 `human_confirm` 仍正常注册

#### Scenario: Graph 模式不注入 dynamicToolLoading

Given Graph 模式已启用 Tool Proxy
When 用户发起一次 Run
Then 不注入 `agent.WithToolFilter`（`dynamicToolLoading` 对该模式无效）

#### Scenario: 启动时预加载工具注册表

Given `global_config` 中 `config_type=auth`、`config_key=access_token` 已配置 `{"bk-hcm": "<valid-token>"}`
And `agent_server.yaml` 中 `tools.toolProxy.initVirtualUser=bk-hcm`
And MCP ToolSet 配置中有 3 个 ToolSet，共包含 50 个工具
When agent-server 启动并调用 `ToolProxy.Build(initCtx)`
Then 使用 `bk-hcm` 对应的 access_token 鉴权拉取全部 50 个工具元数据到 Registry
And 完成向量化并构建 VectorStore
And 用户首次 Run 调用 `search_tools` 时无需等待索引构建

#### Scenario: initVirtualUser 无对应 token

Given `global_config` 中 `access_token` 仅包含 `{"other-user": "<token>"}`
And `initVirtualUser=bk-hcm`
When agent-server 启动
Then Tool Proxy 构建失败并提示 virtual-user `bk-hcm` 无对应 token

#### Scenario: 启动时 access_token 配置缺失

Given `global_config` 中未配置 `auth/access_token`
When agent-server 启动
Then Tool Proxy 构建失败并记录错误日志
And 根据 `toolProxy.required` 配置决定拒绝启动或降级为全量 MCP 注册

#### Scenario: 定时刷新工具注册表

Given Tool Proxy 已成功启动且 `refreshInterval=30m`、`initVirtualUser=bk-hcm`
When 定时器触发刷新
Then 从 global_config `auth/access_token` 读取 `bk-hcm` 对应 token 并重新拉取 MCP 工具列表
And 增量更新 Registry 与 VectorStore
And 刷新期间 `search_tools` 仍可服务（使用上次成功构建的索引，或短暂等待写锁）

---

### Requirement: 端到端流程可跑通

验证完整的"搜索 → 执行"流程可以在 Agent 中正常运行。

#### Scenario: 完整流程 - 搜索工具并执行

Given Agent 已启动并加载了 3 个元工具
When 用户发送消息："帮我在代码中搜索包含 TODO 的文件"
Then Agent 调用 `search_tools(query="在代码中搜索包含 TODO 的文件")`
And 获取到 `search_code` 工具及其 schema
Then Agent 调用 `execute_tool(tool_name="search_code", parameters={"query": "TODO"})`
And 返回搜索结果给用户
#### Scenario: 完整流程 - 先查询 schema 再执行

Given Agent 已启动并加载了 3 个元工具
When 用户发送消息："使用 search_code 工具搜索 TODO"
Then Agent 先调用 `get_tool_schema(tool_name="search_code")` 获取 schema 和 schema_token
And 然后调用 `execute_tool(tool_name="search_code", parameters={"query": "TODO"}, schema_token="...")`
And 返回搜索结果给用户

---

### Requirement: Schema Token 强制机制

通过无状态 HMAC Schema Token，工程层强制 LLM 在调用 `execute_tool` 前必须先获取工具 schema（见设计 D18）。

#### Scenario: schema_token 由 search_tools 返回

Given 工具注册表中存在工具 `search_code`
When Agent 调用 `search_tools` 并获取到 `search_code` 工具
Then 返回结果中每个工具项包含 `schema_token` 字段
And 该 `schema_token` 可直接用于后续 `execute_tool` 的 `schema_token` 参数

#### Scenario: schema_token 由 get_tool_schema 返回

Given 工具注册表中存在工具 `search_code`
When Agent 调用 `get_tool_schema(tool_name="search_code")`
Then 返回结果中包含 `schema_token` 字段
And 该 `schema_token` 可直接用于后续 `execute_tool` 的 `schema_token` 参数

#### Scenario: schema_token 在服务重启后失效

Given Agent 持有重启前生成的旧 `schema_token`
When 服务重启后 Agent 尝试使用旧 `schema_token` 调用 `execute_tool`
Then 返回 `success=false`，错误类型为 `schema_token_invalid`
And 错误响应中包含 `required_schema`，Agent 可据此重新调用 step 1 获取新 token
