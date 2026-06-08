## Context

当前 agent graph（`graph_build.go`）拓扑：

```
START → intent_recognition → (host_apply → llm | other → fallback)
llm → hitl / tool / fallback
fallback resume → 按 intent_task_status 路由 (in_progress → llm | idle/completed → intent_recognition)
```

为在多轮主机申领中跳过重复意图识别，引入了 `IntentTaskStatus` 与 `finish_intent_task`（LLM 在任务结束时调用以写入 `completed` 并清空 intent）。该机制与「最简意图路由」目标冲突，且依赖模型自觉结束任务，可靠性不足。

`intent_recognition` 节点已能将用户消息分类为 `host_apply` / `resource_query` / `chat`，并写入 `StateKeyIntent`。仅需用 **intent 本身** 作为 session 级路由依据，无需独立任务状态机。

## Goals / Non-Goals

**Goals:**

- 移除 `finish_intent_task` 及 `IntentTaskStatus` 相关代码与提示词
- fallback resume 后：`StateKeyIntent == host_apply` → `llm`；否则 → `intent_recognition`
- 首次对话入口仍为 `intent_recognition`
- 不支持 ReAct 的意图（`chat`、`resource_query` 等）经 `fallback` 返回固定文案后，用户下一条消息重新走意图识别
- 本 session 一旦识别为 `host_apply`，后续所有用户轮次直达 `llm`，不再调用意图识别 LLM

**Non-Goals:**

- 同 session 内从主机申领切换为其他意图（如资源查询）的显式「结束任务」流程（新 session 自然重置）
- 新增 `resource_query` 等 ReAct 子图（仍走 fallback + 重新识别）
- 修改 `intent_recognition_prompt.md` 的分类规则（除非实现时发现必须精简上下文延续描述）

## Decisions

### 决策 1：用 `StateKeyIntent` 替代 `StateKeyIntentTaskStatus` 做 fallback 路由

**选择**：`makePostFallbackRoutingFunc` 读取 `state[constant.StateKeyIntent]`，若为 `host_apply` 则返回 `"llm"`，否则返回 `"intent_recognition"`。

**理由**：用户明确要求「识别为主机申领后，本 session 不再走意图识别」。`host_apply` 写入 checkpoint 后即可表达该绑定，无需 `in_progress` / `completed` 三态。

**替代方案（已否决）**：

- 每轮 resume 都走 `intent_recognition`：违背多轮申领与 session 绑定需求
- 保留 `IntentTaskStatus` 仅去掉 `finish_intent_task`：状态与 intent 重复，仍需要模型或额外逻辑置 `completed`

### 决策 2：intent_recognition 节点只写 `StateKeyIntent`

`intentState()` 仅返回 `{ StateKeyIntent: string(intent) }`。识别为 `host_apply` 时 intent 持久化在 graph state / checkpoint 中，供后续 fallback 边使用。

不支持的业务意图（`chat`、`resource_query`）同样写入 intent，供 `unsupportedIntentFallbackMessage` 使用；fallback 后 intent 非 `host_apply`，下轮重新识别。

### 决策 3：删除 `finish_intent_task` 全套

删除 `intent/finish_intent_task.go`、`GetFinishIntentTaskToolWrapper` 注册、`MakeFinishIntentTaskCallback`、`system_prompt.md` 中相关段落，以及 `FinishIntentTaskToolName` 等常量（若无其他引用）。

### 决策 4：不支持意图仍走 fallback，不进入 llm

与现网一致：`makeIntentRoutingFunc` 仅 `host_apply` → `llm`，其余 → `fallback`。用户收到预设不支持文案后，下一条消息因 intent ≠ `host_apply` 而进入 `intent_recognition`。

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| Session 内用户改聊其他话题，仍走主机申领 ReAct | 产品接受「一 session 一申领流」；新开会话可重置；后续若需要可加显式重置（非本次范围） |
| 旧 checkpoint 含 `intent_task_status` 无 `intent` | 缺失或非 `host_apply` 的 intent 时 fallback 路由到 `intent_recognition`，行为安全 |
| 意图识别误判为 `host_apply` 后无法自动纠正 | 依赖识别 prompt 质量；与现网一致，不在本次引入 `finish_intent_task` |

## Migration Plan

1. 合并代码后重启 `agent-server`
2. 无 DB / API 变更；graph checkpoint 多出的旧字段可忽略
3. 回滚：恢复 `finish_intent_task` 与 `IntentTaskStatus` 路由分支

## Open Questions

- 无。session 边界以 graph checkpoint / 会话 ID 为准，与现网一致。
