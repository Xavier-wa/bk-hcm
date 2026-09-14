## ADDED Requirements

### Requirement: 元工具输入 schema 增加可选 tool_intent

`search_tools`、`get_tool_schema`、`execute_tool` 的 `InputSchema` SHALL 增加可选字符串字段 `tool_intent`，描述与其它工具共用（`toolIntentSchemaProperty`），引导模型按当前场景概括**本次**调用目的，而不是复述元工具名。该字段 MUST NOT 进入 `Required`。`execute_tool` 的信封结构 `ExecuteToolParams` SHALL 同步增加 `tool_intent`，与 `tool_name` / `parameters` / `schema_token` 并列；`tool_intent` MUST NOT 放入内层 `parameters`（内层仍按 MCP schema 严格校验未知 key）。

`tool_intent` SHALL NOT 参与 `schema_token` 校验、SHALL NOT 传入 MCP 实际调用。模型未填或填空时，元工具 SHALL 按原逻辑执行，不返回参数错误。

#### Scenario: execute_tool 信封含 tool_intent 仍能执行

- **GIVEN** 工具注册表中存在 `search_code`，Agent 已持有有效 `schema_token`
- **WHEN** Agent 调用 `execute_tool`，信封为 `{tool_name, parameters={"query":"TODO"}, schema_token, tool_intent="正在搜索代码中的 TODO"}`
- **THEN** 实际调用 MCP 工具 `search_code` 且 `success=true`；传入 MCP 的参数不含 `tool_intent`

#### Scenario: execute_tool 未填 tool_intent 行为不变

- **GIVEN** 工具注册表中存在 `search_code`，Agent 已持有有效 `schema_token`
- **WHEN** Agent 调用 `execute_tool` 且信封不含 `tool_intent`
- **THEN** 执行结果与增加该字段之前一致，不返回 `invalid_parameters`

#### Scenario: search_tools 与 get_tool_schema schema 暴露 tool_intent

- **GIVEN** 模型将调用 `search_tools` 或 `get_tool_schema`
- **WHEN** 系统向模型暴露该元工具的 `Declaration`
- **THEN** `InputSchema.Properties` 含可选 `tool_intent`，且 `Required` 不含该字段

#### Scenario: tool_intent 不进入内层 parameters 校验

- **GIVEN** MCP 工具 `search_code` 的 schema 仅定义 `query`
- **WHEN** Agent 调用 `execute_tool`，`parameters={"query":"TODO"}`，信封顶层另带 `tool_intent`
- **THEN** 内层 `validateObject` 不因 `tool_intent` 报「未知 key」，MCP 工具被调用
