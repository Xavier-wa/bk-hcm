# 设计审查报告

## 概览

- **变更名称**: agent-graph-hitl
- **本次审查时间**: 2025-01-24（第 2 次审查）
- **审查历史**: 第 2 次审查（修正后复审）
- **审查范围**: proposal + design + tasks
- **总体结论**: 通过

## 问题统计

| 类别 | 未解决 | 已解决 | 新引入 | 总计 |
|------|--------|--------|--------|------|
| 严重问题 | 0 | 2 | 0 | 2 |
| 警告问题 | 0 | 3 | 1 | 4 |
| 建议事项 | 0 | 2 | 1 | 3 |

---

## 严重问题（已解决）

| 编号 | 问题描述 | 位置 | 首次发现 | 解决时间 | 修复说明 |
|------|----------|------|----------|----------|----------|
| R1 | 条件边路由逻辑存在边界情况漏洞：当 LLM 同时输出 `human_confirm` 和其他工具调用时，路由只检查是否存在 `human_confirm`，但其他工具调用会被忽略 | design.md:146-154 | 2025-01-23 | 2025-01-24 | 已在 D2 条件边路由中增加多工具调用边界情况处理：`hasHumanConfirm && hasOtherTools → error` |
| R2 | `hitl` 节点消息注入逻辑依赖 tool message 的 Content 格式，但未定义明确的 JSON Schema 约束 | design.md:189-214 | 2025-01-23 | 2025-01-24 | 废弃原设计，改为 hitl 节点内部调用 `graph.Interrupt`，用户选择直接来自 resume value，不再依赖 tool result |

## 警告问题（已解决/未解决）

| 编号 | 问题描述 | 位置 | 首次发现 | 状态 | 说明 |
|------|----------|------|----------|------|------|
| W1 | `graph.Interrupt` 幂等性防御 | design.md:261-263 | 2025-01-23 | 已解决 | 用户确认无需处理，单轮对话限制一次 HITL |
| W2 | 消息格式国际化 | design.md:270-273 | 2025-01-23 | 已解决 | 用户确认无需处理，用户输入原样传递 |
| W3 | 异常路由日志 | design.md:134-144 | 2025-01-23 | 已解决 | 用户确认无需处理 |
| W4 | `isHITLInterruptEvent` 函数实现未定义 | design.md:343 | 2025-01-24 | **新引入** | D6 中使用了 `isHITLInterruptEvent(event)` 但未定义实现逻辑 |

## 建议事项（已采纳/未采纳）

| 编号 | 建议内容 | 首次提出 | 状态 | 说明 |
|------|----------|----------|------|------|
| S1 | 在 `human_confirm` 工具参数中增加 `timeout` 或 `expires_at` 字段 | 2025-01-23 | 未采纳 | 用户确认当前设计已满足基本需求 |
| S2 | 考虑在 HITL 中断时通过 SSE 发送特殊事件类型 | 2025-01-23 | 已采纳 | 已新增 D6 决策，使用 `translator.BeforeTranslateCallback` |
| S3 | 补充 Runner 层适配任务 | 2025-01-24 | **新引入** | proposal.md:41-42 提及 Runner 层需适配，但 tasks.md 中未体现 |

---

## 本次新引入的问题

### 警告问题

| 编号 | 问题描述 | 位置 | 优化建议 |
|------|----------|------|----------|
| W4 | `isHITLInterruptEvent` 函数实现未定义 | design.md:343 | 补充该函数的实现逻辑说明，明确如何判断 HITL 中断事件（如检查 event.Type 或 event.Data 中的特定字段） |

### 建议事项

| 编号 | 建议内容 | 备注 |
|------|----------|------|
| S3 | 补充 Runner 层适配任务 | proposal.md:41-42 提及 Runner 层需适配 ResumeMap，建议在 tasks.md 中增加相应任务 |

---

## 已解决的问题

| 编号 | 原问题描述 | 首次发现 | 解决时间 | 修复方式 |
|------|------------|----------|----------|----------|
| R1 | 多工具调用边界情况处理缺失 | 2025-01-23 | 2025-01-24 | 在 D2 条件边路由逻辑中增加 `hasHumanConfirm && hasOtherTools → error` |
| R2 | 消息注入逻辑依赖 tool result schema | 2025-01-23 | 2025-01-24 | 废弃 BeforeTool 回调方案，改为 hitl 节点直接处理中断，用户选择来自 resume value |
| R3 | Tool Callbacks 和 interrupt 逻辑位置 | 2025-01-23 | 2025-01-24 | 已废弃 callback.go，graph.Interrupt 移至 hitl Function Node |
| W1 | Interrupt 幂等性防御 | 2025-01-23 | 2025-01-24 | 用户确认无需处理 |
| W2 | 消息格式国际化 | 2025-01-23 | 2025-01-24 | 用户确认无需处理 |
| W3 | 异常路由日志 | 2025-01-23 | 2025-01-24 | 用户确认无需处理 |
| S2 | SSE 自定义事件处理 | 2025-01-23 | 2025-01-24 | 已新增 D6 决策，使用 `translator.BeforeTranslateCallback` |

---

## 详细审查分析

### 1. 核心设计变更审查

#### 1.1 D1: 纯声明工具方案（✅ 通过）

**审查结论**: 设计合理

**分析**:
- `human_confirm` 作为纯声明工具（仅 `tool.Declaration`，无 Function 实现，无 Tool Callbacks）是可行的
- LLM 会将该工具视为可调用的 function tool，在需要用户确认时生成 tool_call
- 由于无实际执行函数，tool_call 不会被执行，而是被条件边路由到 hitl 节点处理

**潜在风险与缓解**:
- **风险**: LLM 可能期望工具执行后有 result 返回
- **缓解**: hitl 节点在 resume 后注入 user message，LLM 会将该 message 视为"用户回复"，符合对话流预期

#### 1.2 D2: 三向条件边路由（✅ 通过）

**审查结论**: 设计合理，边界情况处理完整

**分析**:
- 路由逻辑覆盖所有边界情况：
  - 只有 `human_confirm` → hitl ✅
  - `human_confirm` + 其他工具 → 报错 ✅ (R1 已修复)
  - 其他工具 → tool ✅
  - 无 tool_calls → fallback ✅

**代码审查**:
```go
// R1 边界情况处理正确
if hasHumanConfirm && hasOtherTools {
    return "", fmt.Errorf("invalid tool calls: human_confirm cannot be combined with other tools")
}
```

**建议**: 考虑在错误信息中包含实际调用的工具列表，便于调试

#### 1.3 D3: hitl 节点中断与消息注入（✅ 通过）

**审查结论**: 设计合理，实现方案可行

**分析**:
- `graph.Interrupt` 在 hitl Function Node 内部调用符合 SDK 设计模式
- 参考 `examples/graph/nested_interrupt/main.go` 中的 `askNode` 实现，模式一致
- 消息顺序设计正确：
  ```
  assistant (tool_call) → hitl (interrupt) → resume → user (choice) → llm
  ```

**幂等性考虑**:
- 首次调用：`graph.Interrupt` 抛出 `InterruptError`，中断执行
- Resume 调用：`graph.Interrupt` 返回 resume value，继续执行
- 设计文档中已说明单轮对话仅支持一次 HITL，符合当前需求

#### 1.4 D4: Interrupt Key 设计（✅ 通过）

**审查结论**: 设计合理

**分析**:
- 固定 key `"human_confirm"` 简化 Resume 逻辑
- 单轮对话限制一次 HITL，固定 key 足够
- 风险与缓解措施已在文档中说明

#### 1.5 D5: 消息注入策略（✅ 通过）

**审查结论**: 设计合理

**分析**:
- 用户选择原样追加为 user message 是最自然的上下文传递方式
- 符合 ReAct 模式，LLM 能立即看到用户选择并继续推理
- 不添加额外前缀避免影响 LLM 理解

#### 1.6 D6: SSE 自定义事件处理（⚠️ 需补充）

**审查结论**: 方案可行，但缺少关键实现细节

**问题**:
- `isHITLInterruptEvent(event)` 函数实现未定义（W4）
- 需要明确如何判断一个事件是 HITL 中断事件

**建议实现**:
```go
func isHITLInterruptEvent(event *translator.Event) bool {
    // 方案 1: 检查 event 类型
    if event.Type != "interrupt" {
        return false
    }
    // 方案 2: 检查 event.Data 中的 key
    if _, ok := event.Data["human_confirm"]; ok {
        return true
    }
    return false
}
```

### 2. 任务清单一致性审查

#### 2.1 覆盖完整性（✅ 通过）

**审查结论**: 任务清单覆盖完整

**分析**:
- 常量定义任务：完整 ✅
- HITL 模块实现任务：完整 ✅
- Graph 拓扑改造任务：完整 ✅
- SSE 自定义事件处理任务：完整 ✅
- 测试任务：完整 ✅

#### 2.2 任务一致性（✅ 通过）

**审查结论**: 任务描述与设计文档一致

**分析**:
- 任务 2.2 与 D3 设计一致
- 任务 3.1 与 D2 设计一致
- 任务 5.1 与 D6 设计一致
- 废弃任务标记清晰

#### 2.3 遗漏任务识别（⚠️ 建议补充）

**发现遗漏**:
- Runner 层适配任务（S3）：proposal.md:41-42 提及，但 tasks.md 未体现

**建议补充任务**:
```markdown
## 9. Runner 层适配

- [ ] 9.1 修改 Runner 层代码，检测到 interrupt checkpoint 时：
  - 解析 checkpoint 中的 interrupt state
  - 将用户输入封装为 `graph.Command{ResumeMap: map[string]any{constant.HITLInterruptKey: userInput}}`
  - 构造 runtime state 包含 `CfgKeyLineageID` 和 `CfgKeyCheckpointID`
```

### 3. 与现有系统一致性审查

#### 3.1 Graph 拓扑兼容性（✅ 通过）

**分析**:
- 使用 `AddConditionalEdges` 替换 `AddToolsConditionalEdges` 是合理的扩展
- 现有 `graph_build.go` 中的 `fallback` 逻辑保持不变
- `tool` → `llm` 循环边保持不变

#### 3.2 代码一致性（✅ 通过）

**分析**:
- `graph.Interrupt` 使用方式与 `examples/graph/nested_interrupt/main.go` 一致
- `model.RoleAssistant`、`model.RoleUser` 等常量使用正确
- State key 使用符合框架约定

#### 3.3 接口兼容性（✅ 通过）

**分析**:
- AGUI HTTP 接口无变化，符合 Non-Goals
- SSE 自定义事件通过 `BeforeTranslateCallback` 实现，不修改接口层

### 4. 潜在风险与缓解

| 风险 | 影响 | 缓解措施 | 状态 |
|------|------|----------|------|
| LLM 可能不理解纯声明工具的语义 | 中 | 工具 description 清晰说明用途；hitl 节点正确处理中断 | 已缓解 |
| 多工具调用边界情况 | 低 | R1 已修复，明确报错 | 已解决 |
| Interrupt Key 冲突 | 低 | 单轮对话限制一次 HITL | 已缓解 |
| SSE 事件识别失败 | 中 | 需补充 `isHITLInterruptEvent` 实现（W4） | 待处理 |
| Runner 层未适配 | 高 | 需补充 Runner 层适配任务（S3） | 待处理 |

---

## 结论与建议

### 总体结论: 通过

修正后的设计方案整体合理，技术选型正确，与现有系统架构兼容。第一次审查发现的 2 个严重问题已全部解决，3 个警告问题和 2 个建议事项按用户意见处理完毕。

### 本次新发现的问题

1. **W4 - SSE 事件识别函数未定义**: 非阻塞性问题，实现时补充即可
2. **S3 - Runner 层适配任务遗漏**: 建议补充到 tasks.md

### 后续行动建议

1. **实现阶段重点关注**:
   - 补充 `isHITLInterruptEvent` 函数实现（W4）
   - 补充 Runner 层适配任务（S3）
   - 单元测试 6.3（Graph 拓扑测试）需覆盖 R1 边界情况

2. **测试建议**:
   - 验证纯声明工具是否能被 LLM 正确调用
   - 验证消息顺序是否符合预期
   - 验证 SSE 自定义事件前端可识别

3. **文档建议**:
   - 可在 design.md 中补充 `isHITLInterruptEvent` 的实现示例
   - 补充 Runner 层适配说明

---

## 审查历史记录

### 第 2 次审查（2025-01-24）
- 审查人: gate-reviewer agent
- 结论: 通过
- 发现问题: 0严重/1警告/1建议
- 备注: 修正后复审，第一次审查的 R1/R2 已解决，新增 W4/S3 为次要问题

### 第 1 次审查（2025-01-23）
- 审查人: gate-reviewer agent
- 结论: 有条件通过
- 发现问题: 2严重/3警告/2建议
- 备注: 首次审查，重点关注 Graph 拓扑改造、Interrupt 幂等性、消息注入策略

---

## 设计修正记录

### 修正时间：2025-01-23

根据用户明确的修正意见，对 design.md 和 tasks.md 进行了以下修正：

#### 重大设计修正（R2/R3）

**核心变更**：废弃原设计中 `BeforeTool` 回调方案，改为 hitl Function Node 直接处理中断

| 原设计 | 修正后 |
|--------|--------|
| `human_confirm` 通过 `BeforeTool` 回调拦截执行 | `human_confirm` 是纯声明工具，无执行逻辑，无 Tool Callbacks |
| `graph.Interrupt` 在 tool 层调用 | `graph.Interrupt` 在 hitl Function Node 内部调用 |
| 用户选择通过 tool result（`CustomResult`）传递 | 用户选择直接来自 Interrupt 的 resume value |
| hitl 节点仅负责消息注入 | hitl 节点负责中断 + 消息注入 |

**废弃文件**：
- ~~`cmd/agent-server/logics/agent/hitl/callback.go`~~ — 不再需要 BeforeTool 回调

**新增设计**：
- D1 决策：明确 `human_confirm` 是纯声明工具
- D3 决策：hitl 节点内部调用 `graph.Interrupt`，resume 后将用户选择注入 messages

#### R1 边界情况处理

**修正内容**：在 D2 条件边路由逻辑中增加多工具调用边界情况处理

```go
// 新增边界情况处理
if hasHumanConfirm && hasOtherTools {
    return "", fmt.Errorf("invalid tool calls: human_confirm cannot be combined with other tools")
}
```

**路由规则更新**：
- 只有 `human_confirm` → hitl
- `human_confirm` + 其他工具 → **报错**（新增）
- 其他工具（无 `human_confirm`）→ tool
- 无 tool_calls → fallback

#### S2 SSE 自定义事件（已采纳）

**修正内容**：新增 D6 决策，使用 `translator.BeforeTranslateCallback` 处理 SSE 自定义事件

```go
translatorConfig := &translator.Config{
    BeforeTranslateCallback: func(ctx context.Context, event *translator.Event) (*translator.Event, error) {
        if isHITLInterruptEvent(event) {
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

#### 无需处理的问题

以下问题根据用户意见**无需处理**：

| 问题编号 | 问题描述 | 不处理原因 |
|----------|----------|------------|
| W1 | `graph.Interrupt` 幂等性防御 | 单轮对话限制一次 HITL，依赖 Runner 层机制 |
| W2 | 消息格式国际化 | 用户输入原样传递，不添加前缀 |
| W3 | 异常路由日志 | 非阻塞性问题，当前设计已足够 |
| S1 | timeout/expires_at 字段 | 当前设计已满足基本需求 |

#### 文件变更清单更新

**修改文件**：
- `cmd/agent-server/logics/agent/graph_build.go` — 增加 SSE 自定义事件处理配置

**新增文件**：
- `cmd/agent-server/logics/agent/hitl/tool.go`
- `cmd/agent-server/logics/agent/hitl/node.go`
- `cmd/agent-server/logics/agent/hitl/hitl.go`

**废弃文件**：
- ~~`cmd/agent-server/logics/agent/hitl/callback.go`~~

#### 任务清单更新

**废弃任务**：
- ~~2.2 BeforeTool 回调实现~~
- ~~2.4.3 GetCallbacks() 函数~~
- ~~4.2 工具回调注册~~

**新增任务**：
- 2.2 HITL 节点实现（含中断逻辑）— 增加 `graph.Interrupt` 调用
- 5. SSE 自定义事件处理（S2）
- 6.4 SSE 自定义事件测试

**更新任务**：
- 3.1 条件边路由 — 增加 R1 多工具调用报错逻辑
- 6.2 HITL 节点测试 — 更新为测试中断 + Resume 场景

#### 修正后的 Graph 拓扑

```
START → llm ──[AddConditionalEdges]──┬─ human_confirm ──→ hitl ─┐
                                     │   (仅 human_confirm)      │
                                     ├─ other_tool_calls ──→ tool ┤
                                     │                           │
                                     └─ no tool_calls ─→ fallback → END

(hitl → llm 循环边, tool → llm 循环边)
```

---

**修正完成确认**：以上修正已按用户意见全部完成，design.md 和 tasks.md 已更新，review-report.md 已添加修正记录。
