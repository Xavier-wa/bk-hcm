## Context

父需求「agent回复交互体验优化」的后端单（TAPD 1069995598137828675）只承接「调用工具增加说明信息」。前端在工具调用行上方展示一句中文说明，说明文案由模型在调用工具时产出；前端直接从工具入参 JSON（`TOOL_CALL_ARGS` / `/history` 快照中的 `toolCalls[].function.arguments`）里解析展示，后端不需要另外下发或转换任何 AG-UI 事件。

**当前实现约束：**

1. 模型可见工具分两类：
   - **自实现元工具**（`toolproxy`）：`execute_tool` / `search_tools` / `get_tool_schema`，`Declaration()` 由本仓库维护。
   - **框架/本地工具**：`skill_load` / `skill_list_docs` / `skill_select_docs`（`trpc-agent-go/tool/skill`，Declaration 不可改）以及 `human_confirm` / `select_account`（本仓库声明，经 `buildSkillTools` 注册）。
2. MCP 真实工具不直接暴露给模型，一律走 `execute_tool` 信封 `{tool_name, parameters, schema_token}`；`validateObject` 对内层 `parameters` 默认拒绝未知 key。
3. 账号门禁 `makeAccountGateBeforeTool` 用 `ResolveToolCall` 解开信封，只读 `tool_name` / 内层 `parameters`，不应被信封新字段打断。

## Goals / Non-Goals

**Goals:**

- 模型调用任意可见工具时，都能在入参里填写一句可选中文 `tool_intent`。
- `tool_intent` 随该次调用既有的 `TOOL_CALL_ARGS` 事件正常流式下发，不新增、不改动 AG-UI 事件类型或事件序。
- `tool_intent` 不参与工具业务校验，不影响执行、门禁、schema_token。

**Non-Goals:**

- 改 system prompt 引导模型叙述意图。
- 按工具名维护中文映射表或按工具拼接兜底模板。
- 前端展开收起与背景色等展示细节（由前端兄弟单负责）。
- 新增 HTTP 接口或 AG-UI 事件类型。
- 后端解析/校验 `tool_intent` 的内容，或为其维护兜底文案（解析、展示、兜底均由前端负责）。
- 把 `tool_intent` 写入内层 MCP `parameters`（会被严格校验拒绝）。

## Decisions

### D1. 字段名用 `tool_intent`，文案灵活性完全靠 schema 描述

需求澄清与 TAPD 已统一为 `tool_intent`。字段 optional，不进 `Required`。所有工具共用同一段 `toolIntentSchemaProperty().Description`（元工具直接挂、装饰器也挂这段），**不按工具名写不同描述、不按工具拼兜底模板**。

描述必须让模型自己产出「这次为什么调」，而不是复述工具名。要点：

1. **总结性**：一句中文，写目的，不堆参数字段名、不写技术工具名。
2. **场景性 / 本次性**：根据当前对话上下文概括**这一次**调用要做什么；同一工具在不同轮次、不同目的下必须写出不同文案。
3. **建议句式**：以「正在…」开头，用户能看懂。

推荐描述原文（实现时写入 `toolIntentSchemaProperty`，可微调措辞但不得改成按工具名固定文案）：

```text
面向用户的一句话中文说明，概括【本次】调用在当前对话场景下要做什么。
要求：写目的与场景，不要复述工具名或参数字段名；同一工具在不同上下文必须写出不同目的
（例如同是 search_tools，一次写「正在查找能列出云账号的工具」，另一次写「正在查找能查询磁盘的工具」；
同是 execute_tool，一次写「正在查询该业务下的云账号」，另一次写「正在提交主机申领单」）。
一句、总结性、用户能看懂，建议以「正在」开头。
```

**否决**：沿用 iWiki `call_intent`——与已澄清需求用语不一致。

**否决**：按工具写死说明或用 `query`/`tool_name` 拼模板——同工具不同目的时模板会失真，且和「文案来自模型」冲突。

### D2. 不新增 AG-UI 事件或后端解析逻辑，前端直接从 TOOL_CALL_ARGS 解析

`tool_intent` 是工具入参 JSON 的顶层字段，随该次调用既有的 `TOOL_CALL_START` / `TOOL_CALL_ARGS` / `TOOL_CALL_END` 序列正常下发；`/history` 快照中同样在 `toolCalls[].function.arguments` 里。前端直接解析这个字段即可展示，不需要后端：

- 在 `customTranslator.Translate` 里按 `toolCallId` 缓存拼接 `TOOL_CALL_ARGS` 分片；
- 额外插入一条承载文案的消息事件（如 `TEXT_MESSAGE_CHUNK`）并维护 `messageId` 绑定关系；
- 解析 `tool_intent` 或在缺失时补一段兜底文案。

`execute_tool` 的 `tool_intent` 在信封顶层，与 `tool_name`/`parameters`/`schema_token` 并列，**不在** `parameters` 里，避免被内层 MCP schema 校验拒绝；框架工具同样在入参顶层。这样前后端协议不变、不新增事件类型，翻译层无需任何改动。

**否决**：翻译层在 `TOOL_CALL_END` 前插入一条 `TEXT_MESSAGE_CHUNK` 说明事件——需要翻译层缓存/拼接流式 ARGS、生成并去重 `messageId`、维护兜底文案，且让 `/history` 归约多出一条独立 assistant 消息；前端已确认可以直接解析 `TOOL_CALL_ARGS`，没有必要为展示引入这一整套状态管理。

### D3. 自实现工具改 Declaration；框架工具只包 `Declaration()`

共享 `ToolIntentSchemaProperty()` / `DeclWithToolIntent(decl)` 放 `cmd/agent-server/logics/tool/tool_intent.go`，装饰器与元工具共用，避免两处文案/字段名分叉。

> **实现期修正（位置）**：原计划放 `toolproxy/intent.go`。但 `toolproxy` 已 import `logics/tool`，
> 而装饰器要放在 `logics/tool`，两者会构成 import cycle。改为统一放下层的 `logics/tool`，
> 由 `toolproxy` 反向引用；函数随之导出。

自实现三元工具在 `InputSchema.Properties` 直接加字段；`ExecuteToolParams` 增加 `ToolIntent string \`json:"tool_intent,omitempty"\``。`Call()` 忽略该字段。`ResolveToolCall` 仍只取 `ToolName`/`Parameters`，信封多一个字段不影响门禁。

框架工具用装饰器，只重写 `Declaration()`，其它方法靠内嵌接口透传：

```go
type CallableToolWithIntent struct{ trpctool.CallableTool }

func (w CallableToolWithIntent) Declaration() *trpctool.Declaration {
    // 浅拷贝 Schema 与 Properties，避免改到内层可能复用的共享 map
    return DeclWithToolIntent(w.CallableTool.Declaration())
}
```

> **实现期修正（不能用泛型内嵌，且必须保留能力）**：
>
> 1. Go 禁止内嵌类型参数（`embedded field type cannot be a (pointer to a) type parameter`），
>    原设计的 `ToolWithIntent[T]{T}` 无法编译。
> 2. 更关键的是，框架不是只认 `tool.Tool` / `tool.CallableTool`，而是对工具做**结构化类型断言**
>    来发现可选能力（`internal/flow/processor/functioncall.go`：`StateDeltaForInvocation`、
>    `StateDelta`、`tool.StreamableTool`、`StreamInner`、`SkipSummarization` 等）。
>    单一装饰类型会吞掉这些方法——`skill_load` / `skill_select_docs` 的 `StateDeltaForInvocation`
>    断言一旦落空，skill 状态会**静默**不落盘。
>
> 因此 `NewToolWithIntent(inner) trpctool.Tool` 按内层实际能力挑选变体：`ToolWithIntent`（纯声明，
> 如 `human_confirm`——绝不能因包装变成可执行）/ `CallableToolWithIntent` /
> `statefulToolWithIntent`（显式转发 `StateDelta` 与 `StateDeltaForInvocation`）。
> 命中无法转发的能力时原样返回 `inner` 并打 Warn：宁可少一个说明字段，也不改工具行为。

模型多填的 `tool_intent`，内层 `json.Unmarshal` 会忽略未知字段，**不必**在 `Call()` 前剪字段。`human_confirm` / `select_account` 同样包装，保证「所有模型可见工具」一致。

包装点：`buildSkillTools` 对 skill 三件套与 `human_confirm`；`select_account` 写入 `skillTools` 时再包一层。不要包进账号门禁的 BeforeTool 链——门禁读的是调用 args，不是 Declaration。

## Risks / Trade-offs

- **[模型不填或写成工具名复述]** → 质量靠 D1 的 schema 描述约束，不靠模板；缺失时如何展示由前端决定。
- **[装饰器改到框架 Declaration 的共享 map]** → 浅拷贝 `Properties` 再写入。
- **[信封 `tool_intent` 被误塞进内层 `parameters`]** → schema 把字段放在信封顶层；`validateObject` 继续拒绝内层未知 key；单测锁住「有 tool_intent 的 execute_tool 仍能执行」。

## Migration Plan

合声明侧（字段 + 装饰器）即完成：模型开始能填 `tool_intent`，前端直接从 `TOOL_CALL_ARGS` / `/history` 快照解析展示，无需其它后端改动。回滚只需去掉 `DeclWithToolIntent` 的调用点，字段本身是可选的，不影响任何现有行为。

无需数据迁移。

## Open Questions

- `human_confirm` 是否要对用户隐藏说明行？由前端按工具名过滤即可，后端不特判。
