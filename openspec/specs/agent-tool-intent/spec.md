# Spec: agent-tool-intent

## Purpose

定义 agent-server 中「工具调用中文说明」能力：模型可见的每一个工具（自实现元工具、框架内置工具、本地声明工具）在其入参 schema 中都暴露一个可选字符串字段 `tool_intent`，供模型填写一句概括**本次**调用目的的中文说明。该字段随工具调用既有的 `TOOL_CALL_ARGS` 事件正常下发，前端直接从中解析展示，后端不为此新增或改动任何 AG-UI 事件、不做解析或兜底处理。

## Requirements

### Requirement: 工具声明暴露可选 tool_intent

系统 SHALL 让模型可见的每一个工具在 `InputSchema.Properties` 中暴露可选字符串字段 `tool_intent`。该字段的 `Description` SHALL 引导模型填写**本次**调用在当前对话场景下的一句话中文目的（总结性、场景性，建议「正在…」开头），并明示同一工具在不同上下文必须写出不同目的；SHALL NOT 把描述写成按工具名固定的文案。`tool_intent` MUST NOT 进入 `Required`，MUST NOT 参与工具业务参数校验；缺失、空串或仅空白 SHALL 视为未填写，且 SHALL NOT 导致工具调用失败。所有工具 SHALL 共用同一段字段描述（由 `toolIntentSchemaProperty` 提供）。

框架内置工具（`skill_load` / `skill_list_docs` / `skill_select_docs`）以及本地声明工具（`human_confirm` / `select_account`）SHALL 通过只重写 `Declaration()` 的装饰器注入该字段；`Call` 及其它方法 SHALL 原样透传到内层实现。

#### Scenario: 框架工具 schema 含可选 tool_intent

- **GIVEN** 模型将调用 `skill_load` 或 `human_confirm`
- **WHEN** 系统向模型暴露该工具的 `Declaration`
- **THEN** `InputSchema.Properties` 含 `tool_intent`（type=string），且 `Required` 不含 `tool_intent`

#### Scenario: 未填 tool_intent 不阻断执行

- **GIVEN** 模型调用带装饰器的本地/框架工具，入参不含 `tool_intent` 或值为空
- **WHEN** 工具执行
- **THEN** 内层工具按原参数执行成功或按原错误返回，不因缺少 `tool_intent` 失败

#### Scenario: 装饰器不改写原 Required

- **GIVEN** 某本地工具原 `Required` 为 `["question"]`
- **WHEN** 经 `tool_intent` 装饰器包装后读取 `Declaration`
- **THEN** `Required` 仍只含原字段，不含 `tool_intent`

#### Scenario: 字段描述要求场景性而不是工具名复述

- **GIVEN** 任一模型可见工具（含 `search_tools` / `execute_tool` / `skill_load`）
- **WHEN** 读取其 `InputSchema.Properties["tool_intent"].Description`
- **THEN** 描述要求概括本次调用目的与场景、禁止只复述工具名，并说明同一工具不同上下文应写出不同文案

### Requirement: tool_intent 随 TOOL_CALL_ARGS 下发，不额外插入说明事件

系统 SHALL NOT 在 AG-UI 翻译层（`customTranslator.Translate`）额外插入用于承载 `tool_intent` 的
`TEXT_MESSAGE_CHUNK` 或其它事件。`tool_intent` 作为工具入参 JSON 的顶层字段，随该次调用既有的
`TOOL_CALL_ARGS` 事件流式下发即可；前端 SHALL 自行从 `TOOL_CALL_ARGS`（或 `/history` 快照中
`toolCalls[].function.arguments`）解析顶层 `tool_intent` 字段用于展示，后端不做解析、不做兜底
文案、不维护额外的 `messageId` 绑定关系。

#### Scenario: 模型填写了 tool_intent

- **GIVEN** 模型调用某工具且入参顶层 `tool_intent` 为「正在查询账号列表...」，`toolCallId` 为 `call-1`
- **WHEN** 翻译层产出该次工具调用的 AG-UI 事件
- **THEN** 事件序为 `TOOL_CALL_START` → `TOOL_CALL_ARGS`（`delta` 中含 `"tool_intent":"正在查询账号列表..."`）→ `TOOL_CALL_END`，翻译层不插入任何额外事件

#### Scenario: tool_intent 缺失不影响事件序

- **GIVEN** 模型调用某工具且入参无 `tool_intent` 或值为空/空白
- **WHEN** 翻译层产出该次工具调用的 AG-UI 事件
- **THEN** 事件序与改动前完全一致，翻译层不因缺少 `tool_intent` 补发任何事件或兜底文案

#### Scenario: 并行多个 tool_calls 互不影响

- **GIVEN** 一次 LLM 事件含两个 tool_calls，id 分别为 `c1`、`c2`
- **WHEN** 翻译层产出 AG-UI 事件
- **THEN** 事件序中只有各自的 `TOOL_CALL_START` / `TOOL_CALL_ARGS` / `TOOL_CALL_END`，不含任何为 `tool_intent` 插入的额外事件
