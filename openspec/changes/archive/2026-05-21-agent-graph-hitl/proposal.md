## Why

当前 agent-server 的 ReAct Graph 仅支持 LLM 自动决策 → 调用工具 → 返回结果的标准流程。当 LLM 需要在多个决策分支中由用户主动选择（如确认操作、选择配置方案）时，缺乏人机交互机制。引入 Human-in-the-Loop（HITL）能力后，LLM 可通过特殊工具主动请求用户确认，Graph 自动触发中断并保存 checkpoint，用户选择后恢复执行并将结果带回 LLM 上下文，实现更灵活的智能助手交互模式。

## What Changes

- **新增 `human_confirm` 特殊工具**：注册为 LLM 可调用的工具，不执行业务逻辑，仅作为路由标记触发 HITL 流程
- **新增 `hitl` 节点**：Function Node 实现，调用 `graph.Interrupt` 触发中断，保存 checkpoint，恢复后将用户选择注入 messages
- **Graph 拓扑改造**：
  - 条件边由二向分支（`llm` → `tool` / `fallback`）改为三向分支（`llm` → `hitl` / `tool` / `fallback`）
  - 新增 `hitl` → `llm` 的边，实现 HITL 循环
- **新增常量定义**：`HumanConfirmToolName` 等 HITL 相关常量
- **新增 HITL 工具实现模块**：`cmd/agent-server/logics/agent/hitl/` 目录，包含工具定义、节点实现、回调处理

## Capabilities

### New Capabilities

- `hitl-graph-topology`: HITL Graph 拓扑扩展——在现有 3 节点 ReAct 图基础上引入第 4 个 `hitl` 节点，实现三向条件边路由（hitl/tool/fallback）和 HITL 循环回边
- `hitl-tool-confirm`: HITL 人工确认工具——`human_confirm` 工具声明与拦截机制，通过 Tool Callbacks 在 BeforeTool 阶段调用 `graph.Interrupt`
- `hitl-interrupt-resume`: HITL 中断恢复机制——`hitl` 节点内 `graph.Interrupt` 幂等调用、checkpoint 保存/恢复、ResumeMap 用户选择传递
- `hitl-message-inject`: HITL 消息注入——恢复后将用户选择结果追加为 message，确保 LLM 能看到用户输入并继续推理

### Modified Capabilities

- 无

## Impact

- **代码文件**：
  - `cmd/agent-server/logics/agent/graph_build.go` — 修改 Graph 拓扑和条件边配置
  - `pkg/criteria/constant/aiagent.go` — 新增 HITL 相关常量
  - `cmd/agent-server/logics/agent/hitl/tool.go` — 新增 `human_confirm` 工具定义（新增文件）
  - `cmd/agent-server/logics/agent/hitl/node.go` — 新增 `hitl` 节点实现（新增文件）
  - `cmd/agent-server/logics/agent/hitl/callback.go` — 新增 Tool Callbacks 拦截逻辑（新增文件）

- **依赖变更**：无新增外部依赖，使用现有 `trpc.group/trpc-go/trpc-agent-go/graph` 包的 `Interrupt` 能力

- **接口影响**：
  - AGUI HTTP 接口无变化，HITL 中断/恢复对前端透明
  - Runner 层需适配：检测到 interrupt checkpoint 时，下一轮请求将用户消息封装为 `Command.ResumeMap`

- **配置影响**：无

- **兼容性**：向后兼容，现有非 HITL 流程不受影响
