## ADDED Requirements

### Requirement: MCP ToolSet `type: "internal"` 支持

`agent-server` 的 `tool.BuildMCPToolSets()` SHALL 支持新的 MCP ToolSet 配置项 `type: "internal"`。该类型用于 agent-server LLM 调用内部微服务 MCP（如 api-server 内置 HCM MCP）的内网直连场景。

`type` 取值 SHALL 通过 `pkg/criteria/constant` 中的常量约束，本变更新增常量 `MCPTypeInternal = "internal"`。

#### Scenario: 加载 internal 类型的 MCP ToolSet

- **WHEN** `agent_server.yaml` 中配置：
  ```yaml
  tools:
    mcp:
      - name: "hcm-internal-cvm"
        type: "internal"
        transport: "streamable_http"
        serverUrl: "http://api-server-internal/api/v1/mcp/internal/hcm/mcp/"
        timeout: "30s"
  ```
- **THEN** agent-server 启动时 SHALL 成功构建该 ToolSet 并注册到 `runTime.AGUIRunner` 的 ToolSet 列表
- **AND** 启动日志 SHALL 记录 `name`、`type`、`transport`、`serverUrl`

### Requirement: bk_username 注入策略

对于 `type: "internal"` 的 MCP ToolSet，agent-server SHALL 在每次发起 MCP HTTP 请求前从 `request context` 中读取 `bk_username`（通过 `auth.BKUsernameFromContext(ctx)`），并将其作为 HTTP header `X-Bkapi-User-Name`（值取自 `constant.UserKey`）注入到出站请求中。

#### Scenario: 上下文中存在 bk_username 时正常注入

- **GIVEN** A2A 请求经 `bkapiContextMiddleware` 处理后 context 中包含 `bk_username = "jettxiao"`
- **WHEN** LLM 触发对 `type: "internal"` ToolSet 的工具调用
- **THEN** 出站 MCP HTTP 请求的 `X-Bkapi-User-Name` header SHALL 等于 `"jettxiao"`

#### Scenario: 上下文中无 bk_username 时不注入

- **GIVEN** context 中不包含 `bk_username`
- **WHEN** LLM 触发对 `type: "internal"` ToolSet 的工具调用
- **THEN** 出站 MCP HTTP 请求 SHALL NOT 设置 `X-Bkapi-User-Name` header
- **AND** 请求 SHALL 继续发出（不阻断），由下游内置 MCP 决定是否拒绝匿名调用

### Requirement: 与 bkaidev 类型完全隔离

`type: "internal"` 的 MCP ToolSet SHALL NOT 触发任何 `bkaidev` 类型的注入逻辑。具体而言：

- SHALL NOT 注入 `X-Bkapi-Authorization` header
- SHALL NOT 从 context 中读取或注入 `bk_ticket`
- SHALL NOT 读取 `cc.AgentServer().Tools.BKAIDev` 配置（即使该配置存在）
- SHALL NOT 因 `tools.bkAIDev` 配置缺失而构建失败或告警

#### Scenario: 无 bkAIDev 全局配置时 internal toolset 仍可构建

- **GIVEN** `agent_server.yaml` 中 `tools.bkAIDev` 段未配置（或 `appCode` / `appSecret` 为空字符串）
- **WHEN** 同一 yaml 中仅声明 `type: "internal"` 的 MCP ToolSet
- **THEN** `agent-server` SHALL 成功启动
- **AND** SHALL NOT 输出 `"tools.bkAIDev config is nil"` 警告（该警告仅针对 `type: "bkaidev"`）

#### Scenario: bkaidev 类型行为不受 internal 类型存在的影响

- **GIVEN** 同一 yaml 中同时存在 `type: "bkaidev"` 与 `type: "internal"` 两个 ToolSet
- **WHEN** 请求触发 `bkaidev` ToolSet 的工具调用
- **THEN** 该工具调用 SHALL 仍然按现有 `bkaidev` 分支注入 `X-Bkapi-Authorization`（含 appCode + appSecret + bk_ticket）
- **AND** internal ToolSet 的存在 SHALL NOT 修改 bkaidev ToolSet 的 header 注入逻辑

### Requirement: 配置校验允许 internal 类型

`pkg/cc/service.go` 中的 MCP ToolSet 配置校验 SHALL 接受 `type` 值为 `"internal"`，且不要求同时存在 `tools.bkAIDev` 配置块。校验 SHALL 拒绝 `type` 为未知字符串。

#### Scenario: 接受 internal 类型

- **WHEN** 配置中 `tools.mcp[0].type` 为 `"internal"`
- **THEN** `AgentServerSetting.Validate()` SHALL 返回 nil

#### Scenario: 拒绝未知类型

- **WHEN** 配置中 `tools.mcp[0].type` 为既不属于 `"bkaidev"` 也不属于 `"internal"` 的字符串（如 `"unknown"`）
- **THEN** `AgentServerSetting.Validate()` SHALL 返回错误，错误信息中 SHALL 包含该非法 `type` 值
