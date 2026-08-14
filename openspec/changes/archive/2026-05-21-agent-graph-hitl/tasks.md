# HITL Graph 任务清单

## 1. 常量定义

- [x] 1.1 在 `pkg/criteria/constant/aiagent.go` 中新增 `HumanConfirmToolName` 常量（值为 `"human_confirm"`），用于标识 HITL 工具名称
- [x] 1.2 在 `pkg/criteria/constant/aiagent.go` 中新增 `HITLInterruptKey` 常量（值为 `"human_confirm"`），用于 `graph.Interrupt` 的 key

## 2. HITL 模块核心实现

### 2.1 工具声明

- [x] 2.1.1 新建 `cmd/agent-server/logics/agent/hitl/tool.go` 文件
- [x] 2.1.2 定义 `HumanConfirmArgs` 结构体，包含 `Question string` 和 `Options []string` 字段，添加 JSON tag
- [x] 2.1.3 实现 `HumanConfirmTool()` 函数，返回 `*tool.Declaration`，包含工具名称、描述和参数 schema（`question` 为必填 string，`options` 为可选 array）

### 2.2 HITL 节点实现（含中断逻辑）

- [x] 2.2.1 新建 `cmd/agent-server/logics/agent/hitl/node.go` 文件
- [x] 2.2.2 定义 `HITLNode` 结构体（Function Node）
- [x] 2.2.3 实现 `NewHITLNode()` 构造函数
- [x] 2.2.4 实现 `Execute(ctx context.Context, state graph.State) (any, error)` 方法：
  - 从 `state[graph.StateKeyMessages]` 读取消息列表
  - 逆序遍历消息，找到最后一条 `Role == model.RoleAssistant` 且包含 `human_confirm` tool call 的消息
  - 解析 tool call 的 `Function.Arguments` 为 `HumanConfirmArgs` 结构体
  - 调用 `graph.Interrupt(ctx, state, constant.HITLInterruptKey, interruptValue)`，其中 `interruptValue` 为包含 `question` 和 `options` 的 map
  - 处理 `InterruptError`：首次调用时向上传播错误（中断）
  - Resume 时返回 resume value（用户输入字符串）
  - 构造新的 user message：`model.Message{Role: model.RoleUser, Content: userChoice}`
  - 返回 `graph.State{graph.StateKeyMessages: append(messages, newMsg)}`

### 2.3 模块入口

- [x] 2.3.1 新建 `cmd/agent-server/logics/agent/hitl/hitl.go` 文件
- [x] 2.3.2 实现 `GetTool()` 函数，返回 HITL 工具声明（调用 `HumanConfirmTool()`）
- [x] 2.3.3 实现 `GetNode()` 函数，返回 `graph.NodeFunc`（HITL 节点执行函数）

## 3. Graph 拓扑改造

- [x] 3.1 修改 `cmd/agent-server/logics/agent/graph_build.go`：
  - 在 `BuildGraph` 函数中，使用 `stateGraph.AddNode("hitl", makeHITLNode())` 注册 hitl 节点
  - 移除原有的 `AddToolsConditionalEdges("llm", "tool", "fallback")` 调用
  - 新增自定义条件边 `AddConditionalEdges("llm", routingFunc, map[string]string{"hitl": "hitl", "tool": "tool", "fallback": "fallback"})`
  - 实现 `routingFunc` 函数：
    - 从 state 读取 `StateKeyMessages`，获取最后一条消息
    - 若无 `tool_calls`，返回 `"fallback"`
    - 遍历 `tool_calls`，检查是否包含 `human_confirm` 和其他工具：
      - 若同时存在 `human_confirm` 和其他工具，返回错误（R1 边界情况）
      - 若只有 `human_confirm`，返回 `"hitl"`
      - 若只有其他工具（无 `human_confirm`），返回 `"tool"`
  - 添加 `hitl` → `llm` 的循环边：`stateGraph.AddEdge("hitl", "llm")`
  - 保留原有的 `tool` → `llm` 循环边

## 4. 工具集成

- [x] 4.1 在 `cmd/agent-server/logics/agent/graph_build.go` 中，将 `human_confirm` 工具添加到工具集（`declaredTools`）

## 5. SSE 自定义事件处理（S2）

- [x] 5.1 在 `cmd/agent-server/logics/agent/graph_build.go` 中配置 translator 时添加 `BeforeTranslateCallback`：
  - 实现回调函数，检查事件类型和内容
  - 若检测到 HITL 中断事件（包含 `human_confirm` 的 interrupt value），替换为自定义事件类型 `hitl_interrupt`
  - 自定义事件数据包含 `question` 和 `options` 字段

## 6. 单元测试

### 6.1 HITL 工具测试

- [x] 6.1.1 新建 `cmd/agent-server/logics/agent/hitl/tool_test.go` 文件
- [x] 6.1.2 测试 `HumanConfirmTool()` 返回的工具声明：验证名称、描述、参数 schema 正确
- [x] 6.1.3 测试 `HumanConfirmArgs` 的 JSON 序列化/反序列化

### 6.2 HITL 节点测试

- [x] 6.2.1 新建 `cmd/agent-server/logics/agent/hitl/node_test.go` 文件
- [x] 6.2.2 测试 `HITLNode.Execute` 首次调用（中断场景）：
  - 构造包含 `human_confirm` tool call 的 state
  - 验证调用 `graph.Interrupt` 并返回 `InterruptError`
- [x] 6.2.3 测试 `HITLNode.Execute` Resume 场景：
  - 构造包含 `ResumeMap` 的 state（key 为 `human_confirm`，value 为用户输入）
  - 验证返回的 state 中 messages 追加了 user message
- [x] 6.2.4 测试异常场景：
  - state 中无 `human_confirm` tool call，验证返回错误
  - messages 为空或最后一条非 assistant message，验证返回错误

### 6.3 Graph 拓扑测试

- [x] 6.3.1 新建/修改 `cmd/agent-server/logics/agent/graph_build_test.go` 文件
- [x] 6.3.2 测试条件边路由逻辑：
  - LLM 输出只有 `human_confirm` tool_call → 路由到 `hitl`
  - LLM 输出 `human_confirm` + 其他 tool_call → 返回错误（R1 边界情况）
  - LLM 输出其他 tool_call（无 `human_confirm`）→ 路由到 `tool`
  - LLM 无 tool_call → 路由到 `fallback`
- [x] 6.3.3 测试 `hitl` → `llm` 边存在且正确

### 6.4 SSE 自定义事件测试

- [x] 6.4.1 测试 `BeforeTranslateCallback` 正确识别 HITL 中断事件
- [x] 6.4.2 测试自定义事件 `hitl_interrupt` 格式正确，包含 `question` 和 `options`

## 7. 集成测试

- [x] 7.1 编写端到端 HITL 流程测试（可放在 `cmd/agent-server/logics/agent/hitl/integration_test.go` 或现有集成测试文件中）：
  - 模拟完整流程：LLM 调用 `human_confirm` → Graph 中断 → Resume 用户选择 → hitl 节点执行 → LLM 继续
  - 验证 checkpoint 保存和恢复机制
  - 验证消息顺序：assistant (tool_call) → interrupt → user (choice) → assistant (response)

## 8. 回归测试

- [x] 8.1 验证现有非 HITL 工具调用流程不受影响（`tool` 节点正常执行）
- [x] 8.2 验证现有直接响应流程不受影响（`fallback` 节点正常执行）
- [x] 8.3 验证 ReAct 循环（`tool` → `llm` → `tool`）正常工作
- [x] 8.4 验证 SSE 自定义事件不影响现有事件流

## 废弃任务（原设计中存在，现不再需要）

以下任务在原设计中存在，但根据用户修正意见已废弃：

- ~~BeforeTool 回调实现（原 2.2.x）~~：改为 hitl 节点直接处理中断，不再需要 `BeforeTool` 回调
- ~~callback.go 文件（原 2.2.1、2.2.2、2.2.3、2.2.4）~~：不再需要
- ~~GetCallbacks() 函数（原 2.4.3）~~：不再需要
- ~~工具回调注册（原 4.2）~~：不再需要

## 依赖关系

```
1. 常量定义 (1.x)
    ↓
2. HITL 模块核心实现 (2.x)
    ├── 2.1 工具声明 (依赖 1.x)
    └── 2.2 HITL 节点 (依赖 1.x)
    ↓
3. Graph 拓扑改造 (3.x) (依赖 2.x)
    ↓
4. 工具集成 (4.x) (依赖 2.x, 3.x)
    ↓
5. SSE 自定义事件 (5.x) (依赖 3.x)
    ↓
6. 单元测试 (6.x) (依赖 2.x, 3.x, 4.x, 5.x)
    ↓
7. 集成测试 (7.x) (依赖 6.x)
    ↓
8. 回归测试 (8.x) (依赖 7.x)
```

## 验收标准

- [ ] `human_confirm` 工具正确注册，LLM 可以调用
- [ ] 调用 `human_confirm` 时 Graph 正确触发中断，保存 checkpoint
- [ ] Resume 时用户选择正确传递，hitl 节点将选择追加为 user message
- [ ] 条件边路由正确：只有 human_confirm → hitl，其他 tool → tool，无 tool → fallback
- [ ] R1 边界情况处理：human_confirm + 其他工具 → 报错
- [ ] hitl → llm 循环边正常工作，LLM 能看到用户选择并继续推理
- [ ] SSE 自定义事件 `hitl_interrupt` 正确发送，前端可识别
- [ ] 现有非 HITL 流程不受影响
- [ ] 所有单元测试通过
- [ ] 集成测试验证完整 HITL 流程
