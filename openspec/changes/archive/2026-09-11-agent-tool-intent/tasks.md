## 1. 共享 schema 声明

- [x] 1.1 在 `pkg/criteria/constant/aiagent.go` 新增 `ToolIntentArgKey = "tool_intent"`
- [x] 1.2 新增 `cmd/agent-server/logics/tool/tool_intent.go`（**非** `toolproxy/intent.go`，见下方偏差 1）：`ToolIntentSchemaProperty()`（string，Description 按 design D1：总结性 + 场景性 + 同工具不同目的须写出不同文案，建议「正在…」开头）、`DeclWithToolIntent(decl)`
- [x] 1.3 单测 `ToolIntentSchemaProperty()`：Description 含「本次」/场景目的要求，且不含按工具名固定的文案模板

### 实现期偏差（已在编码中确认，design 已同步）

1. **共享 helper 位置**：`toolproxy` 已 import `logics/tool`，若 helper 放 `toolproxy/intent.go` 而装饰器放 `logics/tool`，会构成 import cycle。故 helper 与装饰器同放 `cmd/agent-server/logics/tool/`（`toolproxy` 反向引用），仍保持单一来源。函数改为导出（`ToolIntentSchemaProperty` / `DeclWithToolIntent`）。
2. **装饰器不能用泛型内嵌**：Go 禁止内嵌类型参数（`embedded field type cannot be a (pointer to a) type parameter`），design 原方案的 `ToolWithIntent[T]{T}` 无法编译。改为按内层能力选择变体的非泛型装饰器，详见 3.1。

## 2. 元工具声明（tool-proxy）

- [x] 2.1 `ExecuteToolParams` 增加 `ToolIntent string \`json:"tool_intent,omitempty"\``；`execute_tool` 的 `Declaration` 在 `Properties` 加入 `tool_intent`，不进 `Required`
- [x] 2.2 `search_tools` / `get_tool_schema` 的 `InputSchema` 同样加入可选 `tool_intent`；解析函数忽略该字段
- [x] 2.3 单测：三元工具 `Declaration` 含 `tool_intent` 且不在 `Required`；`execute_tool` 信封带 `tool_intent` 时仍能通过参数校验并调用 MCP，传入 MCP 的 args 不含 `tool_intent`
- [x] 2.4 单测：信封不含 `tool_intent` 时 `execute_tool` 行为与改前一致；`ResolveToolCall` 解开信封后 `Name`/`Arguments` 不受 `tool_intent` 影响

## 3. 框架/本地工具装饰器

- [x] 3.1 新增装饰器 `cmd/agent-server/logics/tool/tool_intent_wrapper.go`：`NewToolWithIntent(inner trpctool.Tool) trpctool.Tool`，只重写 `Declaration()`（浅拷贝 `Properties` 后写入 `tool_intent`，不改 `Required`），`Call` 靠内嵌透传。**偏差 2 落地**：Go 禁止内嵌类型参数，改为按内层能力挑选变体——`ToolWithIntent`（纯声明）/ `CallableToolWithIntent`（可调用）/ `statefulToolWithIntent`（可调用 + 显式转发 `StateDelta`、`StateDeltaForInvocation`）。框架用结构化断言发现这些能力，包装若吞掉会让 `skill_load` 状态静默不落；命中本装饰器无法转发的能力（`StreamableTool` / `StreamInner` / `SkipSummarization` / 结构化流式错误）时原样返回 inner 并打 Warn
- [x] 3.2 `buildSkillTools` 对 `skill_load` / `skill_list_docs` / `skill_select_docs` / `human_confirm` 包装 `NewToolWithIntent`
- [x] 3.3 `select_account` 写入 `skillTools` 时同样包装
- [x] 3.4 单测：包装后 schema 含可选 `tool_intent`、原 `Required` 不变；包装后 `Call` 仍走到内层（mock）；纯声明工具不会因包装变可执行；`StateDelta` 系列能力被保留；账号门禁 `makeAccountGateBeforeTool` 对带 `tool_intent` 的 `execute_tool` 信封行为不变

## 4. 文档

- [x] 4.1 更新 `docs/api-docs/web-server/docs/service/agent/agui.md`：说明 `tool_intent` 是工具入参 JSON 的普通字段，随既有 `TOOL_CALL_ARGS` 正常下发，由前端解析展示；不新增、不改动任何 AG-UI 事件类型

## 5. 回归

- [x] 5.1 跑 `cmd/agent-server/...` 全量单测。新增用例全绿；`logics/agent`、`logics/toolproxy`、`logics/tool` 全绿（门禁、HITL 路由、`ResolveToolCall`、装饰器无回归）
