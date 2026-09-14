## Why

智能助手调用工具时，前端只能看到技术工具名（如 `execute_tool` / `skill_load`），用户无法理解「这次工具在做什么」。父需求「agent回复交互体验优化」要求在工具调用行上方展示一句中文说明。本变更让模型在调用工具时顺带产出 `tool_intent`，作为工具入参随既有的 `TOOL_CALL_ARGS` 事件正常下发；前端直接从入参 JSON 中解析并渲染，后端不需要为此新增或改动任何 AG-UI 事件。

## What Changes

- **工具声明增加可选字段 `tool_intent`**：模型调用工具时按字段描述填写一句中文说明。描述必须引导模型写**本次**调用在当前场景下的目的（总结性、场景性），同一工具不同上下文应写出不同文案；不改 system prompt，不维护按工具名的映射表或模板。
- **自实现元工具直接加字段**：`execute_tool` / `search_tools` / `get_tool_schema` 的 `InputSchema` 与 `execute_tool` 信封 `ExecuteToolParams` 增加 `tool_intent`（optional，不进 `Required`）。
- **框架/本地工具用装饰器注入字段**：`skill_load` / `skill_list_docs` / `skill_select_docs` 以及 `human_confirm` / `select_account` 等本地声明工具，通过一层只重写 `Declaration()` 的装饰器追加 `tool_intent`；`Call` / `StateDelta` 等原样透传，不改工具业务校验。
- **前端直接解析 `TOOL_CALL_ARGS`**：`tool_intent` 随工具入参 JSON 正常流式下发，前端从中解析展示；后端不插入额外的说明事件，也不维护兜底文案。
- **不做**：前端展开/收起与背景色、工具名静态映射表、新 HTTP 接口、`/history` 消息顺序改动。

## Capabilities

### New Capabilities

- `agent-tool-intent`: 工具调用中文说明——`tool_intent` 字段声明、框架工具装饰器；说明文案随 `TOOL_CALL_ARGS` 下发，由前端解析渲染。

### Modified Capabilities

- `tool-proxy`: `execute_tool` / `search_tools` / `get_tool_schema` 的输入 schema（及 `execute_tool` 信封）增加可选 `tool_intent`；该字段不参与 MCP 参数校验与 schema_token 校验。

## Impact

**受影响服务层**：仅 agent-server（Access/Service 层）。无 data-service / DAO / DB schema 变更，无新 HTTP 接口。

**代码**：

| 文件 | 改动 |
|---|---|
| `cmd/agent-server/logics/toolproxy/` | 新增共享 `tool_intent` schema；`execute_tool` 信封与三元工具 Declaration 加字段 |
| `cmd/agent-server/logics/agent/` 或 `logics/tool/` | 新增 `ToolWithIntent` 装饰器；`buildSkillTools` 与 `select_account` 注册处包装 |
| `pkg/criteria/constant/aiagent.go` | 新增字段名常量 `ToolIntentArgKey` |
| `docs/api-docs/web-server/docs/service/agent/agui.md` | 说明 `tool_intent` 随 `TOOL_CALL_ARGS` 下发，由前端解析 |
| 相关单测 | 声明 schema、装饰器不破坏 Required |

**协议**：不新增、不改动 AG-UI event type；`tool_intent` 就是工具入参 JSON 里的一个普通字段，随既有 `TOOL_CALL_ARGS` 流式下发。

**成本与性能**：无额外 SSE 事件、无额外 LLM 调用。`tool_intent` 文案质量依赖模型遵循度，仅作展示。
