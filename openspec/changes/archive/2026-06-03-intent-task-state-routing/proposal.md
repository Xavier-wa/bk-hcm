## Why

当前意图识别节点在每轮对话结束（fallback interrupt resume）后都会重新运行，但多轮任务（如主机申领）的中间对话轮次（用户回答 LLM 的追问）并非新意图，强制重新识别会导致意图被误判为 chat，使进行中的任务工作流断链。需要引入任务状态机制，让进行中的任务持续路由到 llm 而不触发意图识别，只有在任务明确完成后才重置为新任务入口。

## What Changes

- **新增** `finish_task` 声明性工具：LLM 在任务完成时主动调用，触发任务状态清除
- **新增** `StateKeyTaskStatus` graph state key，记录任务状态（`idle` / `in_progress` / `completed`）
- **修改** fallback resume 后的路由逻辑：根据 `StateKeyTaskStatus` 决定走 `intent_recognition` 还是直接走 `llm`
- **修改** intent_recognition 路由：识别到支持的意图后写入 `in_progress` 状态
- **修改** tool 节点：执行 `finish_task` 时清除任务状态（写入 `completed`），清除 `StateKeyIntent`
- **移除** 原来依赖 `previousIntent` hint 做"延续 vs 切换"判断的脆弱逻辑（可保留作为补充，但不再作为主要依据）

## Capabilities

### New Capabilities

- `agent-task-state`: Agent graph 任务状态机，包含 `finish_task` 工具声明、`StateKeyTaskStatus` state key、任务状态感知的 fallback 路由

### Modified Capabilities

- （无现有 spec 需要 delta，agent 相关逻辑均为新增）

## Impact

- `cmd/agent-server/logics/agent/graph_build.go`：fallback 后路由逻辑变更，注册 `finish_task` 工具
- `cmd/agent-server/logics/agent/intent/node.go`：intent 识别成功后写入 `in_progress` 任务状态
- `cmd/agent-server/logics/agent/` 新增 task 子包：`finish_task` 工具声明 + tool 节点 post-callback
- `pkg/criteria/constant/aiagent.go`：新增 `StateKeyTaskStatus` 常量
- `pkg/criteria/enumor/`：新增 `TaskStatus` 枚举类型
- 无 API、数据库、外部依赖变更
