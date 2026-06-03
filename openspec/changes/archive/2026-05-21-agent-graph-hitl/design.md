## Context

### 当前 Graph 拓扑

现有 `BuildGraph` 构建的 ReAct 图包含 3 个节点：

```
START → llm ──[AddToolsConditionalEdges]──┬─ tool_calls ──→ tool ─┐
                                          │                        │
                                          └─ no tool_calls ─→ fallback → END
                                                               
(tool → llm 循环边)
```

- **llm 节点**：LLM Node，决定调用工具或直接响应
- **tool 节点**：Tools Node，执行 LLM 请求的工具
- **fallback 节点**：Function Node，规范化最终输出

`AddToolsConditionalEdges("llm", "tool", "fallback")` 根据 LLM 输出中的 `tool_calls` 决定路由：
- 有 `tool_calls` → `tool` 节点
- 无 `tool_calls` → `fallback` 节点

### HITL 需求场景

LLM 在某些场景下需要用户主动确认或选择：
1. **操作确认**："即将删除 3 台主机，是否继续？"
2. **方案选择**："检测到两种配置方案，请选择：A. 高性能型 B. 经济型"
3. **参数补充**："请确认业务 ID：12345 是否正确？"

这些场景需要：
- LLM 主动发起确认请求（通过调用特殊工具）
- Graph 暂停执行，等待用户输入
- 用户选择后，Graph 恢复并将结果带回 LLM

### 技术约束

1. **工具声明限制**：`human_confirm` 是**纯声明工具**，无实际执行逻辑，不提供 Tool Callbacks
2. **中断位置**：`graph.Interrupt` 在 **hitl Function Node 内部** 调用，而非 tool 层
3. **用户输入来源**：用户选择来自 **Interrupt 的 resume value**（即 `graph.Command.ResumeMap`），不是工具执行结果
4. **消息传递**：HITL 节点恢复后必须主动将用户选择追加为 message，LLM 才能看到
5. **SSE 事件**：通过 `translator.BeforeTranslateCallback` 在事件翻译前追加自定义事件，让前端识别 HITL 中断状态

## Goals / Non-Goals

**Goals:**

- 在现有 3 节点 ReAct 图基础上引入第 4 个 `hitl` 节点
- 实现 LLM 通过 `human_confirm` 工具主动触发 HITL 流程
- Graph 在 `hitl` 节点自动触发中断，保存 checkpoint
- 用户选择后 Graph 从 `hitl` 节点恢复，将选择结果带回 LLM 上下文
- 条件边由二向分支改为三向分支（llm → hitl / tool / fallback）
- 向后兼容，现有非 HITL 流程不受影响

**Non-Goals:**

- 不修改 AGUI HTTP 接口层（`/api/v1/agent/agui`）
- 不修改 Runner 层的 checkpoint 存储机制
- 不引入新的外部依赖
- 不支持多轮 HITL 嵌套（单轮对话内多次中断）
- 不实现前端 UI 层面的 HITL 特殊展示（保持 SSE 流式输出）

## Decisions

### D1: HITL 工具为纯声明工具，无执行逻辑

**选择**：`human_confirm` 注册为普通 function tool（仅声明给 LLM 可调用），**无实际执行函数，不提供 Tool Callbacks**。hitl 节点负责处理所有中断逻辑。

```go
// human_confirm 工具声明（仅声明，无执行逻辑，无回调）
declaredTool(&tool.Declaration{
    Name:        constant.HumanConfirmToolName,
    Description: "请求用户确认或选择，当需要用户在多个选项中做出选择时使用",
    InputSchema: &tool.Schema{
        Type: "object",
        Properties: map[string]*tool.Schema{
            "question": {Type: "string", Description: "向用户展示的问题"},
            "options":  {Type: "array", Description: "选项列表（可选）"},
        },
        Required: []string{"question"},
    },
})

// 注意：无 Function 实现，无 Tool Callbacks 注册
```

**理由**：
- `human_confirm` 只是**占位/路由标记**，用于触发条件边路由到 hitl 节点
- 实际中断逻辑在 hitl Function Node 内部实现，更清晰的职责分离
- 用户选择来自 Interrupt 的 resume value，不是工具执行结果

**替代方案**（已拒绝）：
- 通过 `BeforeTool` 回调在 tool 层调用 `graph.Interrupt`（拒绝：设计复杂，resume value 需通过 tool result 传递，增加不必要的转换层）
- 将 `human_confirm` 作为独立 Function Node（拒绝：破坏 ReAct 循环，LLM 无法自主决定何时触发 HITL）
- 在 LLM Node 后统一加 `WithInterruptAfter`（拒绝：无法区分哪些情况需要 HITL，所有调用都会中断）

### D2: Graph 拓扑改造——三向条件边

**选择**：将 `AddToolsConditionalEdges` 替换为自定义 `AddConditionalEdges`，实现三向分支：

```
START → llm ──[AddConditionalEdges]──┬─ human_confirm ──→ hitl ─┐
                                     │   (仅 human_confirm)      │
                                     ├─ other_tool_calls ──→ tool ┤
                                     │                           │
                                     └─ no_tool_calls ─→ fallback → END

(hitl → llm 循环边, tool → llm 循环边)
```

**条件边路由逻辑**（含 R1 多工具调用边界情况处理）：

```go
stateGraph.AddConditionalEdges("llm", func(ctx context.Context, state graph.State) (string, error) {
    messages, _ := state[graph.StateKeyMessages].([]model.Message)
    if len(messages) == 0 {
        return "fallback", nil
    }
    
    lastMsg := messages[len(messages)-1]
    
    // 检查是否有 tool_calls
    if len(lastMsg.ToolCalls) == 0 {
        return "fallback", nil
    }
    
    // 检查 tool_calls 内容
    hasHumanConfirm := false
    hasOtherTools := false
    
    for _, tc := range lastMsg.ToolCalls {
        if tc.Function.Name == constant.HumanConfirmToolName {
            hasHumanConfirm = true
        } else {
            hasOtherTools = true
        }
    }
    
    // R1: 多工具调用边界情况处理
    if hasHumanConfirm && hasOtherTools {
        // LLM 同时输出 human_confirm 和其他工具调用，拒绝执行
        return "", fmt.Errorf("invalid tool calls: human_confirm cannot be combined with other tools")
    }
    
    if hasHumanConfirm {
        // 只有 human_confirm，路由到 hitl 节点
        return "hitl", nil
    }
    
    // 其他工具调用（无 human_confirm）
    return "tool", nil
}, map[string]string{
    "hitl":     "hitl",
    "tool":     "tool",
    "fallback": "fallback",
})

// 添加循环边
stateGraph.AddEdge("hitl", "llm")
stateGraph.AddEdge("tool", "llm")
```

**路由规则总结**：
- **只有** `human_confirm` → 路由到 `hitl`
- 有 `human_confirm` **且** 有其他工具 → **报错**（R1 边界情况）
- 有其他工具（无 `human_confirm`）→ 路由到 `tool`
- 无 `tool_calls` → 路由到 `fallback`

**理由**：
- `AddToolsConditionalEdges` 只支持二向分支（有 tool_calls / 无 tool_calls）
- 需要区分 `human_confirm` 和其他工具调用，必须自定义条件边
- 保持 `hitl` → `llm` 和 `tool` → `llm` 的循环，支持 ReAct 模式
- R1 边界情况处理确保系统行为明确，避免工具调用丢失

**替代方案**（已拒绝）：
- 在 `tool` 节点内部通过 `BeforeTool` 回调区分（拒绝：无法避免 tool 节点的标准工具执行逻辑，且路由不清晰）
- 使用子图（Subgraph）封装 HITL 流程（拒绝：增加复杂度，当前场景不需要）

### D3: hitl 节点实现——中断与消息注入

**选择**：`hitl` 节点是一个 Function Node，职责：
1. 从 state 中解析 `human_confirm` 的 tool call 参数（question, options）
2. 调用 `graph.Interrupt(ctx, state, "human_confirm", {question, options})`
3. 首次调用抛出 InterruptError（中断）
4. Resume 时返回 resume value（用户输入字符串）
5. 将用户输入追加为 `role=user` 的 message
6. 返回 state

```go
func makeHITLNode() graph.NodeFunc {
    return func(ctx context.Context, state graph.State) (any, error) {
        messages, _ := state[graph.StateKeyMessages].([]model.Message)
        
        // 1. 从最后一条 assistant message 中找到 human_confirm 的 tool call
        var toolCall *model.ToolCall
        for i := len(messages) - 1; i >= 0; i-- {
            if messages[i].Role == model.RoleAssistant {
                for _, tc := range messages[i].ToolCalls {
                    if tc.Function.Name == constant.HumanConfirmToolName {
                        toolCall = &tc
                        break
                    }
                }
                if toolCall != nil {
                    break
                }
            }
        }
        
        if toolCall == nil {
            return nil, errors.New("hitl node: no human_confirm tool call found")
        }
        
        // 2. 解析参数
        var args HumanConfirmArgs
        if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
            return nil, fmt.Errorf("hitl node: failed to parse human_confirm args: %w", err)
        }
        
        // 3. 调用 graph.Interrupt
        // 首次调用：抛出 InterruptError，中断执行
        // Resume 调用：返回 resume value（用户输入）
        resumeValue, err := graph.Interrupt(ctx, state, constant.HITLInterruptKey, map[string]any{
            "question": args.Question,
            "options":  args.Options,
        })
        if err != nil {
            // 首次调用：err 是 InterruptError，向上传播
            return nil, err
        }
        
        // Resume 后：resumeValue 是用户输入字符串
        userChoice, ok := resumeValue.(string)
        if !ok {
            return nil, fmt.Errorf("hitl node: invalid resume value type")
        }
        
        // 4. 将用户选择追加为 user message
        newMsg := model.Message{
            Role:    model.RoleUser,
            Content: userChoice,
        }
        
        return graph.State{
            graph.StateKeyMessages: append(messages, newMsg),
        }, nil
    }
}
```

**执行流程**：

```
首次调用（中断）：
  hitl node → 解析参数 → graph.Interrupt → 抛出 InterruptError → checkpoint 保存 → SSE 结束

Resume（恢复）：
  Runner 构造 Command{ResumeMap: {"human_confirm": "用户输入"}} → 
  hitl node → 解析参数 → graph.Interrupt → 返回 "用户输入" → 
  追加 user message → 返回 state → 路由到 llm → LLM 继续推理
```

**理由**：
- 中断逻辑集中在 hitl 节点，职责清晰
- 用户选择直接来自 Interrupt resume value，无需通过 tool result 转换
- 消息注入在 hitl 节点完成，LLM 能立即看到用户选择

**替代方案**（已拒绝）：
- 在 tool 层通过 `BeforeTool` 回调调用 `graph.Interrupt`（拒绝：设计复杂，增加不必要的回调层）
- 合并 `hitl` 和 `tool` 节点（拒绝：破坏 ReAct 模式，LLM 无法区分 HITL 和普通工具调用）

### D4: Interrupt Key 设计

**选择**：使用固定 key `"human_confirm"` 作为 `graph.Interrupt` 的 key。

```go
const (
    HITLInterruptKey = "human_confirm"
)

// hitl 节点内调用 Interrupt
resumeValue, err := graph.Interrupt(ctx, state, HITLInterruptKey, map[string]any{
    "question": args.Question,
    "options":  args.Options,
})
```

**Resume 时**（Runner 层构造 Command）：
```go
command := &graph.Command{
    ResumeMap: map[string]any{
        HITLInterruptKey: userInput, // 用户输入字符串
    },
}

runtimeState := graph.State{
    graph.StateKeyCommand:    command,
    graph.CfgKeyLineageID:    lineageID,
    graph.CfgKeyCheckpointID: checkpointID,
}
```

**理由**：
- 固定 key 简化 Resume 逻辑，Runner 层无需动态解析 key
- 单轮对话内只支持一次 HITL，固定 key 足够

**风险与缓解**：
- **风险**：若 LLM 在一轮对话中多次调用 `human_confirm`，第二次调用时 `graph.Interrupt` 会检测到 `UsedInterrupts` 中已有该 key，返回上次的 resume value 而非重新 interrupt。
- **缓解**：当前设计每轮对话（一次 Runner.Run）仅触发一次 interrupt，然后 SSE 结束。用户回复后是新一轮 Runner.Run（resume），`UsedInterrupts` 已清空。

### D5: 消息注入策略

**选择**：HITL 节点恢复后，将用户选择追加为 `role=user` 的 message。

```go
newMsg := model.Message{
    Role:    model.RoleUser,
    Content: userChoice, // 用户输入原样传递
}

return graph.State{
    graph.StateKeyMessages: append(messages, newMsg),
}, nil
```

**理由**：
- LLM 通过 `messages` 维护对话历史，追加 user message 是最自然的上下文传递方式
- 符合 ReAct 模式：user message → LLM 推理 → tool call / response
- 用户输入原样传递，不添加额外前缀，避免影响 LLM 理解

**替代方案**（已拒绝）：
- 写入自定义 State key（拒绝：LLM Node 只读取 `StateKeyMessages`，自定义 key 对 LLM 不可见）
- 修改 system prompt 注入用户选择（拒绝：需要重新编译 prompt，增加复杂度）

### D6: SSE 自定义事件处理（S2）

**选择**：使用 `translator.BeforeTranslateCallback` 在事件翻译前追加/替换一个 customEvent，让前端能识别 HITL 中断状态。

```go
// 在 graph_build.go 中配置 translator 时添加 BeforeTranslateCallback
translatorConfig := &translator.Config{
    BeforeTranslateCallback: func(ctx context.Context, event *translator.Event) (*translator.Event, error) {
        // 检查是否是 HITL 中断事件
        if isHITLInterruptEvent(event) {
            // 替换为自定义 HITL 事件，前端可识别
            return &translator.Event{
                Type: "hitl_interrupt",
                Data: map[string]any{
                    "question": event.Data["question"],
                    "options":  event.Data["options"],
                },
            }, nil
        }
        return event, nil
    },
}
```

**理由**：
- 前端需要明确识别 HITL 中断状态，以展示确认对话框
- `BeforeTranslateCallback` 是 SDK 提供的标准扩展点，在事件翻译前拦截和修改
- 不修改 AGUI HTTP 接口层，符合 Non-Goals

**替代方案**（已拒绝）：
- 修改前端直接解析 `InterruptValue`（拒绝：前端逻辑复杂，需要特殊处理）
- 在 Runner 层发送自定义 SSE 事件（拒绝：需要修改 Runner 层，不符合 Non-Goals）

## Risks / Trade-offs

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| **R1: 多工具调用边界情况** | LLM 同时调用 `human_confirm` 和其他工具时，可能导致工具调用丢失 | 已在 D2 条件边路由逻辑中添加明确报错，拒绝执行此类请求 |
| **R2: Interrupt Key 冲突** | 若 LLM 多次调用 `human_confirm`，第二次可能无法正确触发中断 | 单轮对话限制一次 HITL；如需多次 HITL，使用动态 key（如 `human_confirm_<timestamp>`） |
| **R3: 消息顺序混乱** | HITL 恢复后消息顺序可能不符合预期，导致 LLM 理解错误 | 严格遵循 `hitl` → `llm` 的边，确保 user message 正确追加 |
| **R4: 工具调用超时** | `human_confirm` 调用后用户长时间不响应，checkpoint 占用资源 | 依赖 Runner 层的 session 超时机制（默认 30 分钟），超时后清理 checkpoint |
| **R5: 向后兼容性** | 修改 `graph_build.go` 的条件边可能影响现有流程 | 保持 `fallback` 逻辑不变，仅新增 `hitl` 分支；充分测试现有 tool 调用流程 |

## Migration Plan

### 部署步骤

1. **代码变更**
   - 新增 `pkg/criteria/constant/aiagent.go` 常量
   - 新增 `cmd/agent-server/logics/agent/hitl/` 目录及文件
   - 修改 `cmd/agent-server/logics/agent/graph_build.go`

2. **配置变更**
   - 无需配置变更

3. **验证步骤**
   - 单元测试：`hitl` 节点逻辑、条件边路由
   - 集成测试：完整 HITL 流程（调用 → 中断 → 恢复 → 继续）
   - 回归测试：现有 tool 调用流程不受影响

### 回滚策略

- 代码回滚：还原 `graph_build.go`，移除 `hitl` 目录和常量
- 数据影响：无持久化数据变更，checkpoint 超时自动清理
- 兼容性：回滚后正在进行的 HITL 会话会失败，用户需重新开始对话

## Open Questions

1. **Q: 是否需要支持多选/复杂表单？**
   - 当前设计仅支持简单的单选/确认
   - 如需复杂表单，考虑扩展 `human_confirm` 的 schema 或新增 `human_form` 工具

2. **Q: HITL 超时后是否需要默认行为？**
   - 当前设计依赖 Runner 层超时
   - 如需默认选择（如超时自动取消），可在 hitl 节点实现超时检测逻辑

3. **Q: 是否需要支持多轮 HITL？**
   - 当前设计单轮对话仅支持一次 HITL
   - 如需多轮 HITL，考虑使用动态 interrupt key

## 文件变更清单

### 修改文件

| 文件 | 变更内容 |
|------|----------|
| `cmd/agent-server/logics/agent/graph_build.go` | 替换 `AddToolsConditionalEdges` 为自定义条件边，新增 `hitl` 节点注册和边配置，配置 `translator.BeforeTranslateCallback` 处理 SSE 自定义事件 |
| `pkg/criteria/constant/aiagent.go` | 新增 `HumanConfirmToolName`、`HITLInterruptKey` 常量 |

### 新增文件

| 文件 | 内容 |
|------|------|
| `cmd/agent-server/logics/agent/hitl/tool.go` | `human_confirm` 工具声明、参数结构体定义 |
| `cmd/agent-server/logics/agent/hitl/node.go` | `hitl` 节点实现（Function Node）、中断逻辑、消息注入 |
| `cmd/agent-server/logics/agent/hitl/hitl.go` | 模块入口，导出 HITL 工具集和节点构造器 |

### 废弃文件（原设计中存在，现不再需要）

| 文件 | 废弃原因 |
|------|----------|
| `cmd/agent-server/logics/agent/hitl/callback.go` | 原设计中用于 `BeforeTool` 回调，现改为 hitl 节点直接处理中断，不再需要 |
