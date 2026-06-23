## Context

agent-server 使用 trpc-agent-go 的 graph 框架驱动 ReAct 流程。当前确认能力有两套：

- `confirmToolSet`（`tool_confirm.go`）：对整个 toolset 的泛化软拦截，仅返回文本提示，LLM 可绕过。
- `hitl` 节点 + `human_confirm` 工具：真正的 `graph.Interrupt` 中断，但依赖 LLM 主动调用 `human_confirm`，靠 system prompt 软约束。

主机申领提单（`create_biz_apply`）属高风险写操作，需要"提单前必经确认"的工程化保障，且要在用户确认后做库存/预测前置校验、支持用户修改参数后再提单。

关键框架事实（已核对 trpc-agent-go v1.8.1）：

- 自定义事件由中断 key 驱动：节点 `graph.Interrupt(ctx, state, key, value)` 后，`key/value` 进入 `PregelStepMetadata`，translator 据 key 前缀 emit `CustomEvent`。
- tool 节点 `extractToolCallsFromState` 从消息尾部向前找最近一条带 tool_calls 的 assistant 消息，**遇到 user 消息即停止**。
- resume 时 checkpoint 的 `StateKeyMessages` 被保留，前端历史不覆盖；新用户输入仅作为 `ResumeChannel` 值（`graph.Interrupt` 返回值）。
- `StateKeyUserInput` 在 resume 时可能残留，LLM 节点 `executeUserInputStage` 会用它替换/追加 user 消息。
- 消息 reducer `MessageReducer` 对 `MessageOp` 接口做类型断言，**接受外部自定义 `MessageOp` 实现**。

## Goals / Non-Goals

**Goals:**

- 提供路由层强制的工具调用前确认门禁，LLM 无法跳过受门禁工具的确认。
- 通用且可扩展：新增受门禁工具仅需实现接口并注册，不改图结构。
- 不同工具可定制确认卡片内容与确认后副作用/校验。
- 支持用户在确认卡片上修改参数后再提单。
- 落地 `create_biz_apply` 申领场景（校验逻辑占位）。

**Non-Goals:**

- 不实现真实库存/预测校验接口（本期占位函数）。
- 不改造现有 `hitl` 节点与 `human_confirm` 工具的行为。
- 前端卡片渲染与交互在另一前端任务中实现，本设计仅约定事件与协议契约。

## Decisions

### D1：新增独立 `tool_gate` 节点，而非增强 `confirmToolSet`

`confirmToolSet` 在 `tool.Call` 时拦截，属软提示，无法产生真正中断，且无法在确认后改写参数/路由。选择在图中新增 `tool_gate` 节点，置于 `llm` 与 `tool` 之间，由 `llm` 的条件路由按"工具名是否受门禁"分流。门禁是路由层工程逻辑，LLM 不可绕过。

- 拓扑：`llm → {tool_gate | hitl | tool | fallback}`；`tool_gate → {tool(proceed) | llm(reject)}`。
- 备选：复用 `hitl`（被否，hitl 依赖 LLM 主动调用，且语义为通用选择而非强制门禁）。

### D2：`Gate` 接口 + 注册表作为扩展点

通用节点不含业务；每个受门禁工具实现 `Gate{ToolName, BuildConfirm, OnConfirm}`，注册到 `Registry`（`map[toolName]Gate`）。选择接口而非纯函数 hook，因 handler 通常需持有依赖（如校验所需 client/toolset）。

### D3：确认事件复用中断 key 机制

`tool_gate` 用中断 key `tool_confirm:<tool>:<callID>`，translator 检测 `tool_confirm:` 前缀 emit `tool.confirm` 事件（与现有 `hitl.interrupt` 并列）。无需新增事件通道。key 带 `callID` 保证多次申领的中断 key 唯一、可正确 resume。

### D4：对称结构化确认协议 + 原始入参透传

- 下发（事件 value）：`{tool, title, summary, fields, args}`，`fields` 为只读人类可读展示，`args` 为原始 tool 入参（剥 `body_param` 后）作为可编辑事实源。
- 回传（resume JSON）：`{action: "confirm"|"cancel", args?}`，`args` 存在即用户改过。

选择**原始入参透传**而非 typed 双向映射：`create_biz_apply` 的 `suborders[].spec` 按 `resource_type` 分四种、字段繁多且含废弃/二选一字段，双向映射易丢字段。透传下后端只覆盖不翻译，零映射、不丢参数。校验所需字段在 `OnConfirm` 内**按需只读解析**（read-only view，不影响执行用的完整 JSON）。

### D5：编辑回写用自定义 `MessageOp`

用户改参数后，写自定义 `ReplaceToolCallArgs{ToolID, NewArgs}`（实现 `graph.MessageOp`）覆盖 assistant 消息中对应 tool_call 的 `Arguments`，再路由到 `tool`，tool 节点执行改后参数。依赖 `MessageReducer` 接受外部 `MessageOp` 实现这一事实。

### D6：proceed/reject 的消息与路由处理

- proceed：**不插入任何消息**（保留 assistant tool_calls 作尾部，tool 节点能取到），仅写决策 state，路由到 `tool`。
- reject（取消/校验失败）：补一条 `RoleTool{ToolID, Content: 原因}` 闭合 tool_call、**清空 `StateKeyUserInput`**、路由回 `llm`，由 LLM 自然语言解释为何未提单。清空 user_input 避免确认 token 被 `executeUserInputStage` 当作新用户输入（与 fallback 节点同坑）。

### D7b：门禁配置——代码注册表 + yaml 开关

受门禁工具集以**代码注册表**为事实源：注册了 `Gate` 实现的工具即受保护（卡片内容与确认后校验是工具专属代码，无法纯配置表达）。在此之上叠加可选 yaml 配置 `tools.confirmGate`，用于不改代码地灰度启用/紧急关闭：

```yaml
tools:
  confirmGate:
    enabled: true
    tools:                 # 留空=启用全部已注册门禁；非空=仅启用列表内
      - create_biz_apply
```

路由激活判定 = `IsGated(tool)`（已注册）AND `enabled` AND（`tools` 为空 或 含该工具名）。"配置启用但未注册"按"未启用"处理，不报错。

- 备选 1（纯代码注册表，无配置）：被否，无法不改代码灰度/回滚。
- 备选 2（纯 yaml 驱动）：被否，无 `Gate` 实现则无法生成卡片与校验，退化为通用确认。

### D7：受门禁工具单独成批 + 排除泛化确认

- 路由强制受门禁工具单独成批（不与其他工具或 `human_confirm` 混批），否则 tool 节点会一起执行未确认工具——违反返回 error（沿用现有 R1 风格）。
- `create_biz_apply` 从 `confirmToolSet` 排除，避免门禁卡片 + 泛化文本双重确认。

## Risks / Trade-offs

- [原始入参透传要求前端理解后端 schema] → 由只读 `fields` 提供友好展示，编辑表单绑定 `args`；契约清晰文档化。
- [用户改参后绕过 LLM 直接执行，参数可能非法] → `OnConfirm` 的校验（占位/后续接口）对最终参数把关；工具自身入参校验仍是最后防线。
- [多次申领或并发中断 key 冲突] → key 带 `callID` 保证唯一。
- [reject 路径 tool_call 未闭合导致后续 LLM 请求报错] → 必须补 `RoleTool` result；单测覆盖。
- [校验占位上线后误以为已生效] → 占位函数返回通过并记录日志/注释 TODO，spec 与 tasks 显式标注待接入真实接口。
- [集成点] 仅与前端 chatbot 通过 `tool.confirm` 事件与 resume JSON 协议对接，不涉及 CMDB/IAM/ITSM 直接集成；库存/预测校验后续接 woa/task-server 接口。

## Migration Plan

- 纯增量：新增 `toolgate` 包 + 图节点/路由扩展 + translator 分支 + 常量；不改既有节点行为。
- 前端需同步支持 `tool.confirm` 事件后端禁能才完整；后端先行不影响既有 `hitl`/`fallback` 流程。
- 回滚：从 `graph_build.go` 移除 `tool_gate` 节点与路由分支即可恢复原行为。

## Open Questions

- `create_biz_apply` 在 MCP 暴露给 LLM 的真实工具名需最终确认（按需求暂定 `create_biz_apply`）。
- 库存/预测校验的真实接口与判定口径（可申领容量、预测内/外余量）待后续需求明确。
- `tools.confirmGate` 配置缺省值：本期默认 `enabled: true` 且 `tools` 留空（启用全部已注册门禁）；是否需要按环境差异化由部署配置决定。
