## Why

当前 agent-server 的 AGUI Runner 通过自定义的 `list_tools` / `call_tool` 代理层（`mcp_proxy.go`）包装 MCP 工具，存在两个核心问题：（1）非标准的间接调用协议导致每次工具调用需要 2~3 个额外 Step，增加延迟和 token 消耗，且 LLM 无法直接看到完整 schema，降低调用准确率；（2）代理层屏蔽了真实工具名，导致框架的 `ToolFilter` 机制无法感知具体 MCP 工具，阻碍了动态工具过滤的实现。同时，全量工具描述消耗上下文窗口、降低推理质量的问题也亟待解决。

## What Changes

- **移除 MCP Proxy 代理层**：删除 `mcp_proxy.go` 中的 `list_tools` / `call_tool` 间接调用机制，MCP 工具直接以完整的 `tool.Declaration`（含 name、description、inputSchema）注册到框架，LLM 直接调用真实工具。**BREAKING**：移除 `wrapMCPToolSetsAsProxy()` 函数及相关类型。
- **新增工具元数据提取**：从 MCP ToolSet 的 `tool.Declaration` 中自动提取结构化元数据（ToolMeta），包括名称、描述、参数摘要和可选 Tags。
- **新增工具索引与检索**：实现 `ToolIndex` 接口及 KeywordIndex、BM25Index 两种检索策略（P0 阶段），支持按用户消息内容检索匹配的 Top-N 工具。
- **新增动态 ToolFilter**：通过框架标准的 `llmagent.WithToolFilter()` 注入检索过滤逻辑，每条用户消息触发一次检索，同一 Invocation 内工具集保持稳定。
- **新增配置项**：在 `tools` 配置节下新增 `dynamicToolLoading` 子配置，支持启用开关、策略选择、TopN 设置和工具 Tags 配置。
- **完善降级策略**：配置关闭、索引构建失败、检索无结果等场景均降级为全量工具放行，确保系统稳定性。

## Capabilities

### New Capabilities

- `dynamic-tool-loading`: 基于元数据索引的动态工具加载能力，涵盖工具元数据提取、索引构建（Keyword/BM25）、检索过滤（ToolFilter 集成）、懒构建机制和降级策略。

### Modified Capabilities

（无现有 capability 的需求级别变更）

## Impact

- **代码变更**：
  - `cmd/agent-server/logics/mcp_proxy.go`：整文件删除
  - `cmd/agent-server/logics/runtime.go`：移除 proxy 包装调用，新增动态工具加载配置传递和 ToolFilter 注入
  - `cmd/agent-server/logics/` 目录：新增 `tool_index.go`（元数据 + 索引实现）、`tool_filter.go`（ToolFilter + 懒构建）
  - `pkg/cc/service.go`：在 `AgentToolsConfig` 中新增 `DynamicToolLoading` 相关配置结构体
  - `cmd/agent-server/app/app.go`：`toolsToLogicsConfig()` 新增动态工具加载配置转换
  - `cmd/agent-server/etc/agent_server.yaml`：在 `tools` 配置节下新增 `dynamicToolLoading` 配置示例
- **API/协议变更**：LLM 不再通过 `list_tools` / `call_tool` 间接调用，而是直接看到并调用真实 MCP 工具（对外部客户端透明，仅影响 LLM 交互行为）
- **依赖**：无新外部依赖（BM25 为纯 Go 实现），仅依赖框架已有的 `tool.FilterFunc`、`agent.InvocationFromContext`、`agent.GetStateValue`/`SetState` API
- **兼容性**：`dynamicToolLoading.enabled = false` 时行为与移除 proxy 后的基线一致（全量工具直接暴露），不影响现有功能
