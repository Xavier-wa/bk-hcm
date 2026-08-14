## 1. 基础类型和接口定义

- [x] 1.1 创建 `cmd/agent-server/logics/toolproxy/types.go`，定义 `ToolMetadata`、`ToolRegistry`（不含 `VectorStore` 接口，见设计 D15）
- [x] 1.2 创建 `cmd/agent-server/logics/toolproxy/proxy.go`，定义 `ToolProxy` 结构体，包含 `registry`、`*tool.EmbeddingIndex`、`mcpToolSets`、`emb`、`buildOK`、`refreshMu`、`stopRefresh` 字段，以及 `NewToolProxy()`、`Build(ctx)`、`StartRefreshLoop(ctx, interval)`、`GetProxyToolSet()` 方法（对应设计 D6、D11、D15）

## 1b. 鉴权与配置

- [x] 1b.1 扩展 `auth/bkapi.go`：新增 `WithAccessToken`、`AccessTokenFromContext`、`AccessTokenAuthHeaderValue`（对应设计 D6）
- [x] 1b.2 扩展 `tool/tool.go` bkaidev hook：优先 `bk_ticket`，fallback `access_token`（对应设计 D6）
- [x] 1b.3 创建 `cmd/agent-server/logics/toolproxy/config.go`：实现 `LoadInitAccessToken(cli, virtualUser)`，从 global_config `auth/access_token` 按 virtual-user key 读取 token（对应设计 D10）
- [x] 1b.4 新增 `enumor.GlobalConfigTypeAuth` 常量（若尚不存在）；复用 `config_key=access_token`
- [x] 1b.5 新增 `agent_server.yaml` 中 `tools.toolProxy` 配置段（`enabled`、`initVirtualUser`、`refreshInterval`、`required`、`topN`、`embedding`）（对应设计 D10、D11）

## 1c. Prompt 工程

- [x] 1c.1 在 `cmd/agent-server/etc/prompts/system_prompt.md` 增加「元工具使用工作流（强制）」段落（场景 A/B、禁止直接调用 MCP 工具名）（对应设计 D12；已完成）

## 2. 工具注册表实现

- [x] 2.1 创建 `cmd/agent-server/logics/toolproxy/registry.go`，实现 `ToolRegistry` 的 `Register()`、`Update()`、`Delete()`、`GetTool()`、`ListTools()`、`ExportToolMetas()` 方法，使用 `sync.RWMutex` 保证并发安全（对应设计 D2）
- [x] 2.2 实现 `loadTools(ctx, mcpToolSets)` 方法：复用 `tool.SafeGetTools`，工具名前缀规则与 `LazyToolIndex.ensureBuild` 一致（`{toolsetName}_{rawName}`），调用 `tool.ExtractToolMeta()` 写入注册表（对应设计 D6）
- [x] 2.3 在 `ToolProxy.Build` / `StartRefreshLoop` 中实现索引构建与刷新：`index.Build(ctx, registry.ExportToolMetas())`；刷新时 Registry diff 后重建 index（对应设计 D3、D11、D15）

## 4. search_tools 元工具实现

- [x] 4.1 创建 `cmd/agent-server/logics/toolproxy/search.go`，实现 `SearchToolsTool` 结构体（含 `registry`、`*tool.EmbeddingIndex`），实现 `tool.CallableTool` 接口（对应设计 D4、D7、D17）
- [x] 4.2 实现 `Call()` 方法：检查 `buildOK` → 解析参数（query、top_k、category）→ `index.Search()` → `registry.GetTool()` 组装结果；`buildOK=false` 时返回结构化错误（对应规格 1、设计 D3、D6）
- [x] 4.3 实现 `buildSearchToolsSchema()`；`Declaration().Description` 写清「探索未知工具时使用」（对应设计 D4、D17）

## 5. get_tool_schema 元工具实现

- [x] 5.1 创建 `cmd/agent-server/logics/toolproxy/schema.go`，实现 `GetToolSchemaTool` 结构体，实现 `tool.CallableTool` 接口（对应设计 D4、D17）
- [x] 5.2 实现 `Call()` 方法：解析 `tool_name` → `registry.GetTool()` → 返回完整 schema；`Declaration().Description` 写清「已知工具名时使用」（对应规格 2、设计 D17）

## 6. execute_tool 元工具实现

- [x] 6.1 创建 `cmd/agent-server/logics/toolproxy/execute.go`，实现 `ExecuteToolTool` 结构体，实现 `tool.CallableTool` 接口（对应设计 D4、D5、D17）
- [x] 6.2 实现 `Call()` 方法：解析参数 → 验证工具存在性 → 参数验证（基于 schema）→ 查找实际工具 → 调用 `actualTool.Call()` → 返回执行结果（对应规格 3、设计 D5）
- [x] 6.3 实现 `findActualTool()` 方法，遍历 MCP ToolSets 查找工具，添加性能优化 TODO 注释（对应设计 D5）
- [x] 6.4 实现 `validateParameters()` 函数，基于 JSON Schema 验证参数（对应设计 D5、规格 3）
- [x] 6.5 实现结构化错误返回 `buildErrorResult()`，含 `permission_denied` / `execution_error` 及权限提示文案（对应设计 D8、D16、D17）；`Declaration().Description` 写清须先获取 schema 再执行

## 7. ToolProxy 初始化和 Graph 集成（MVP）

- [x] 7.1 实现 `NewToolProxy(mcpToolSets, emb)` 与 `Build(ctx)`（启动时同步加载 registry + EmbeddingIndex）（对应设计 D6、D15）
- [x] 7.2 实现 `StartRefreshLoop(ctx, interval)` 定时刷新逻辑（对应设计 D11）
- [x] 7.3 修改 `runtime.go`：`New(clientSet)` 读取 `initVirtualUser` → `LoadInitAccessToken` → `Build(initCtx)` → 启动 refresh loop（对应设计 D6、D9、D10）
- [x] 7.4 修改 `graph_build.go`：方案 B — `BuildGraph` 接受 `*toolproxy.ToolProxy`；LLM/Tool Node 通过 `WithToolSets([]tool.ToolSet{proxy.GetProxyToolSet()})` 注册元工具；skill/HITL 保留在 `skillTools` map；**不**注册 `toolset.TS`（对应设计 D6、D13、规格 6）
- [x] 7.5 修改 `graph_build.go`：Tool Proxy 启用时**关闭** `WithRefreshToolSetsOnRun`，并添加注释说明原因（对应设计 D14）
- [x] 7.6 Graph 模式 Tool Proxy 启用时 `runtime` 不构建 `dynamicToolFilter`，Run 时不注入 `WithToolFilter`（对应设计 D9）
- [x] 7.7 启动构建失败降级：`toolProxy.required=false` 时 warn 日志，Graph 回退为直接注册 `toolset.TS`（对应设计 R5）

## 7b. LLM Agent 模式（阶段 2，本变更不做）

- [ ] 7b.1 修改 `agent_llm.go`，使用 `proxy.GetProxyToolSet()` 注册元工具
- [ ] 7b.2 LLM Agent 模式停用 `dynamicToolLoading`

## 8. 端到端测试和验证

- [x] 8.1 编写单元测试：`ToolRegistry` 的注册、查询、删除功能（对应规格 4）
- [x] 8.2 编写单元测试：`ToolProxy` 索引构建与 `EmbeddingIndex.Search` 集成（对应规格 5、设计 D15）
- [x] 8.3 编写单元测试：3 个元工具的参数解析、执行逻辑、错误处理（含权限失败提示）（对应规格 1、2、3、设计 D16）
- [ ] 8.4 Graph 模式集成测试：bkaidev MCP + 真实 `bk_ticket`，验证「search_tools → execute_tool」全流程（对应规格 7）
- [ ] 8.5 验证 Token 消耗降低 90%+（基线：Graph 模式全量 MCP 工具定义；非 dynamicToolLoading Top-N）（对应设计 Goals）

## 9. Schema Token 强制机制（后续优化，已实现）

- [x] 9.1 在 `proxy.go` 新增 `schemaTokenSecret`（crypto/rand 生成）、`generateSchemaToken()`、`verifySchemaToken()` 方法（对应设计 D18）
- [x] 9.2 `search_tools` 返回值中为每个工具附带 `schema_token`（对应规格 Schema Token、设计 D18）
- [x] 9.3 `get_tool_schema` 返回值中附带 `schema_token`（对应规格 Schema Token、设计 D18）
- [x] 9.4 `execute_tool` Declaration 新增 `schema_token` 必填字段；Call() 中新增 HMAC 校验；schema_token 无效时返回带完整 schema 的结构化错误（对应规格 Schema Token、设计 D18）
- [x] 9.5 `validateObject` 改为默认严格拒绝未知参数 key（仅在 `additionalProperties: true` 时放行）；参数校验失败时错误响应中内嵌完整 `required_schema`（对应规格 execute_tool 参数验证、设计 D5、D8）
- [x] 9.6 优化 `system_prompt.md`：移除场景 A/B 条件分支，改为无条件三步工作流；明确 `schema_token` 必填约束；强化禁止行为描述（对应设计 D12）
- [x] 9.7 新增单元测试：schema_token 缺失/无效场景、幻觉参数 key 场景、错误响应含 `required_schema` 验证（对应规格 Schema Token）
