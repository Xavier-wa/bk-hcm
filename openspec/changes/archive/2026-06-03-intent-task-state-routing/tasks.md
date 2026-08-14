## 1. 基础类型与常量

- [x] 1.1 在 `pkg/criteria/enumor/` 新增 `TaskStatus` 枚举类型，定义 `TaskStatusIdle`、`TaskStatusInProgress`、`TaskStatusCompleted` 枚举值，并实现 `Validate()` 方法
- [x] 1.2 在 `pkg/criteria/constant/aiagent.go` 新增 `StateKeyTaskStatus = "task_status"` 常量
- [x] 1.3 在 `pkg/criteria/constant/aiagent.go` 新增 `FinishTaskToolName = "finish_task"` 常量

## 2. finish_task 工具声明

- [x] 2.1 在 `cmd/agent-server/logics/agent/` 下新建 `task/` 子包，创建 `finish_task.go`，定义 `FinishTaskTool()` 函数返回 `*tool.Declaration`（工具名、描述、参数 schema），描述中明确调用时机：任务达到终态（成功/失败/用户放弃）时调用
- [x] 2.2 在 `task/` 包中实现 `GetToolWrapper()` 返回 `tool.Tool` 接口（与 hitl 包中 `declaredToolWrapper` 模式一致），`finish_task` 工具调用时返回固定 tool result `"task finished"`
- [x] 2.3 在 `graph_build.go` 中注册 `finish_task` 工具到 `skillTools` map

## 3. finish_task 执行后清除任务状态

- [x] 3.1 在 `task/` 包中实现 `MakeFinishTaskCallback()` 函数，返回 `graph.PostNodeCallbackFunc`，在 tool 节点执行后检测 messages 中是否有 `finish_task` tool result，若有则返回 state delta：`{StateKeyTaskStatus: TaskStatusCompleted, StateKeyIntent: ""}`
- [x] 3.2 在 `graph_build.go` 的 `genToolNodeOptions()` 中，将 `MakeFinishTaskCallback()` 追加到 `graph.WithPostNodeCallback` 选项（与现有 `MakeSkillLoadAfterToolCallback` 并列）

## 4. intent_recognition 写入 in_progress 状态

- [x] 4.1 修改 `cmd/agent-server/logics/agent/intent/node.go` 中的 `intentState()` 函数：当 intent 为支持的业务意图（`host_apply` 等）时，state delta 中同时写入 `StateKeyTaskStatus: TaskStatusInProgress`；当为不支持意图（`chat`、`resource_query`）时写入 `StateKeyTaskStatus: TaskStatusIdle`

## 5. fallback 后路由逻辑变更

- [x] 5.1 修改 `graph_build.go` 中 `makeIntentRoutingFunc()` 的逻辑：将其重命名为 `makePostIntentRoutingFunc()`，逻辑不变（仍按 intent 路由到 llm 或 fallback）
- [x] 5.2 新增 `makePostFallbackRoutingFunc()` 函数：读取 `StateKeyTaskStatus`，值为 `in_progress` 时返回 `"llm"`，否则返回 `"intent_recognition"`
- [x] 5.3 修改 `graph_build.go` 中 `stateGraph.AddEdge("fallback", "intent_recognition")` 为 `stateGraph.AddConditionalEdges("fallback", makePostFallbackRoutingFunc(), map[string]string{"llm": "llm", "intent_recognition": "intent_recognition"})`
- [x] 5.4 更新 `BuildGraph` 函数头注释，反映新的 fallback 路由拓扑

## 6. 单元测试

- [x] 6.1 为 `TaskStatus` 枚举的 `Validate()` 方法编写单元测试
- [x] 6.2 为 `MakeFinishTaskCallback()` 编写单元测试：验证有 `finish_task` tool result 时正确清除状态，无时不修改状态
- [x] 6.3 为 `makePostFallbackRoutingFunc()` 编写单元测试：验证 `in_progress` 路由到 `llm`，`idle`/`completed`/缺失 路由到 `intent_recognition`
- [x] 6.4 为 `intentState()` 修改后的逻辑编写单元测试：验证 `host_apply` 写入 `in_progress`，`chat` 写入 `idle`
