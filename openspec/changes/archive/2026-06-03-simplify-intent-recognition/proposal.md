## Why

当前 agent graph 为支持多轮主机申领，引入了 `intent_task_status` 状态机、`finish_intent_task` 工具，以及 fallback resume 后按任务状态在 `llm` 与 `intent_recognition` 之间切换的路由。该方案依赖 LLM 主动调用 `finish_intent_task` 才能结束任务并重新识别意图，增加了提示词复杂度与误判风险。实际只需要：**首次（或未绑定业务意图时）做意图识别**；识别为 `host_apply` 后整个 session 固定走主机申领 ReAct；**仅不支持 ReAct 的意图**在 fallback 后下一轮再重新识别。

## What Changes

- **移除** `finish_intent_task` 工具及其 tool 节点 post-callback、相关常量与测试
- **移除** `StateKeyIntentTaskStatus` / `IntentTaskStatus` 枚举在路由与 state 写入中的使用
- **修改** fallback resume 后路由：依据 `StateKeyIntent` 判断
  - `host_apply` → 直接进入 `llm`（本 session 内不再经过意图识别）
  - 其他 / 空 → 进入 `intent_recognition`（不支持 ReAct 的意图每轮可重新识别）
- **保留** 入口与「未绑定 host_apply」时：`intent_recognition` → `host_apply` 走 `llm`，其余走 `fallback` 返回预设不支持文案
- **移除** `system_prompt.md` 中 `finish_intent_task` 使用规范整段
- **简化** `intent_recognition` 节点：仅写入 `StateKeyIntent`（识别为 `host_apply` 时持久化，供后续 fallback 路由使用）

## Capabilities

### New Capabilities

- `agent-intent-routing`: 简化后的 agent 意图识别与图路由（session 级 host_apply 绑定、不支持意图 fallback 后重新识别）

### Modified Capabilities

- （无现有 main spec 需 delta）

## Impact

- `cmd/agent-server/logics/agent/graph_build.go`：fallback 条件边改为按 `StateKeyIntent` 路由；移除 `finish_intent_task` 注册
- `cmd/agent-server/logics/agent/graph_build_test.go`：更新 fallback 路由用例
- `cmd/agent-server/logics/agent/intent/`：删除 `finish_intent_task.go` 及测试；简化 `node.go`
- `cmd/agent-server/etc/prompts/system_prompt.md`：删除 finish_intent_task 段落
- `pkg/criteria/constant/`、`pkg/criteria/enumor/aiagent.go`：清理 `IntentTaskStatus`、`FinishIntentTaskToolName` 等未使用定义
- 无 API、数据库、外部依赖变更
