## ADDED Requirements

### Requirement: Task status state key
系统 SHALL 在 graph.State 中维护一个 `task_status` key，值为枚举类型 `TaskStatus`，取值为 `idle`、`in_progress`、`completed`。该字段通过 checkpoint saver 持久化，服务重启后可恢复。当 checkpoint 中不存在该字段时，系统 SHALL 将其视为 `idle`。

#### Scenario: Fresh conversation has idle task status
- **WHEN** 新对话第一条消息到达，checkpoint 中无 `task_status`
- **THEN** 系统将 task_status 视为 `idle`，路由到 intent_recognition

#### Scenario: Missing task status treated as idle after service restart
- **WHEN** 服务重启后，从旧 checkpoint 中恢复 state，state 中无 `task_status`
- **THEN** 系统将其视为 `idle`，行为退化为旧逻辑（走意图识别），不崩溃

### Requirement: Task becomes in_progress when supported intent is recognized
系统 SHALL 在 intent_recognition 节点成功识别到支持的意图（如 `host_apply`）并路由到 llm 时，将 `task_status` 写入为 `in_progress`，同时保留 `StateKeyIntent`。

#### Scenario: Host apply intent recognized
- **WHEN** 用户输入"帮我申领一台主机"，intent_recognition 识别为 `host_apply`
- **THEN** graph.State 中 `task_status = in_progress`，`intent = host_apply`，路由到 llm 节点

#### Scenario: Unsupported intent does not set in_progress
- **WHEN** 用户输入被识别为不支持的意图（如 `chat`、`resource_query`）
- **THEN** `task_status` 保持 `idle`（或不写入），路由到 fallback 节点

### Requirement: Fallback resume routes based on task status
系统 SHALL 在 fallback 节点 interrupt resume（收到用户下一条消息）后，根据当前 `task_status` 决定路由目标：
- `task_status == in_progress`：直接路由到 `llm` 节点，跳过意图识别
- `task_status != in_progress`（`idle` 或 `completed`）：路由到 `intent_recognition` 节点

#### Scenario: Task in progress - skip intent recognition
- **WHEN** fallback 已 interrupt，用户发送回复"4核8G"，此时 `task_status = in_progress`
- **THEN** fallback resume 后直接路由到 llm，不经过 intent_recognition，主机申领上下文保持完整

#### Scenario: No active task - run intent recognition
- **WHEN** fallback 已 interrupt，`task_status = idle` 或 `completed`，用户发送新消息
- **THEN** fallback resume 后路由到 intent_recognition，重新识别用户意图

### Requirement: finish_task tool declaration
系统 SHALL 提供名为 `finish_task` 的声明性工具（无服务端执行逻辑，纯 LLM 调用声明），工具描述 SHALL 明确说明调用时机：任务达到终态时（成功完成、失败终止、或用户明确放弃）。

#### Scenario: finish_task is registered in skill tools
- **WHEN** graph agent 初始化
- **THEN** `finish_task` 工具出现在 LLM 可用工具列表中

#### Scenario: LLM calls finish_task on task completion
- **WHEN** 主机申领成功，LLM 准备输出"申领成功"回复
- **THEN** LLM 在输出最终回复前调用 `finish_task` 工具

### Requirement: finish_task execution clears task state
系统 SHALL 在 tool 节点检测到 `finish_task` 工具调用时，通过 post-node-callback 将 `task_status` 设置为 `completed`，并清除 `StateKeyIntent`（置为空字符串或删除该 key）。

#### Scenario: Task state cleared after finish_task
- **WHEN** tool 节点执行 `finish_task` 工具调用
- **THEN** `task_status = completed`，`StateKeyIntent = ""`

#### Scenario: Next user message triggers intent recognition after task complete
- **WHEN** `finish_task` 执行后，LLM 输出完成回复，进入 fallback interrupt，用户发送新消息
- **THEN** fallback resume 检测到 `task_status = completed`，路由到 intent_recognition，开始新任务识别

### Requirement: finish_task tool routing in tool node
系统 SHALL 在 tool 节点路由逻辑中将 `finish_task` 视为普通工具（不走 hitl 节点），执行结果返回 tool result 消息后路由回 llm 节点。`finish_task` 的实际执行结果 SHALL 为固定成功消息（`"task finished"`），由工具包装层提供。

#### Scenario: finish_task does not route to hitl
- **WHEN** LLM tool_calls 中包含 `finish_task`
- **THEN** makeRoutingFunc 将其识别为普通工具，路由到 tool 节点而非 hitl 节点
