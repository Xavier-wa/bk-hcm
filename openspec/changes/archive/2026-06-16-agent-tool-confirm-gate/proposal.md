## Why

主机申领提单（`create_biz_apply`）是高风险写操作，提单前必须有用户最终确认。当前仅靠 system prompt 约束 LLM 调用 `human_confirm`，属软约束，LLM 可能跳过，无法保证"提单前必经确认"。需要一个工程化、路由层强制的确认门禁，且能针对申领等特定场景下发结构化确认信息、在用户确认后执行前置校验。

## What Changes

- 新增**通用"工具调用前确认门禁"节点 `tool_gate`**，插在 `llm` 与 `tool` 之间。只要 LLM 产生受门禁工具的 tool_call，路由层强制中断并要求用户确认，LLM 无法绕过。
- 新增 **`Gate` 接口 + 注册表**扩展机制：不同工具可定制确认卡片内容（`BuildConfirm`）与确认后副作用/校验（`OnConfirm`），通用节点不含业务逻辑。
- 新增 **`create_biz_apply` 门禁实现**（首个落地场景）：下发申领单明细确认卡片；用户确认后执行库存/预测前置校验（**本期用占位函数，后续接真实接口**）。
- 新增 **`tool.confirm` 自定义事件**：translator 检测 `tool_confirm:` 中断 key 前缀，向前端 emit 结构化确认内容，前端渲染为确认卡片。
- 支持**确认时修改参数**：前后端采用对称结构化协议（下发 `args` / 回传 `args`，原始 tool 入参透传），用户改后的参数通过自定义 `MessageOp` 回写 tool_call 后执行。
- 受门禁工具从泛化 `confirmToolSet` 中排除，避免双重确认。
- **门禁配置（代码注册表 + yaml 开关）**：受门禁工具集以代码注册表为事实源（注册了 `Gate` 实现即受保护）；新增可选 yaml 配置 `tools.confirmGate`（`enabled` + `tools` 列表）叠加在注册表之上，用于不改代码地灰度启用或紧急关闭，路由判定 = 已注册 AND 配置启用。

## Capabilities

### New Capabilities
- `agent-tool-confirm-gate`: Agent 图中通用的工具调用前确认门禁能力，含强制中断、结构化确认事件、可扩展的 per-tool 确认/校验 hook，以及主机申领场景的首个实现。

### Modified Capabilities
<!-- 无既有 spec 的需求级变更 -->

## Impact

- **新增**：`cmd/agent-server/logics/agent/toolgate/`（通用节点、注册表、`Gate` 接口、自定义 `MessageOp`、申领 handler）。
- **修改**：`cmd/agent-server/logics/agent/graph_build.go`（注册节点、扩展 `llm` 路由、新增门禁后路由与条件边）、`cmd/agent-server/service/agui-event/translator.go`（新增 `tool.confirm` 事件）、`pkg/criteria/constant/aiagent.go`（新增常量）、`cmd/agent-server/logics/tool/tool_confirm.go`（排除受门禁工具）、`pkg/cc/service.go`（新增 `tools.confirmGate` 配置结构）。
- **前端（另起任务）**：`use-event.ts`/`use-stream.ts` 处理 `tool.confirm` 事件，新增申领确认卡片组件，确认/取消按结构化 JSON 回传。
- **依赖**：不新增第三方依赖；库存/预测校验接口本期占位。
- **范围**：仅影响 agent-server 服务层与前端 chatbot，不涉及其他服务层/资源层。
