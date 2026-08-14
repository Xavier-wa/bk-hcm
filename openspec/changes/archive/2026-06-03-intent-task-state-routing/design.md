## Context

当前 agent graph 拓扑：`START → intent_recognition → llm → (tool/hitl/fallback) → intent_recognition`。意图识别在每轮 fallback resume 后都执行，但多轮任务（如主机申领）中 LLM 会先调工具、再向用户追问更多信息，中间轮次的用户回答并非新意图。当前依赖 `previousIntent` hint 让 LLM 判断"延续 vs 切换"，属于软约束，可靠性不足。

当前存在以下问题：
1. 任务进行中（LLM 追问信息），用户简短回答可能被误判为 `chat`，导致 host_apply 工作流断链
2. 没有明确的任务完成信号，intent 何时可以被重置不确定
3. graph state 里的 `StateKeyIntent` 在每轮意图识别后被覆盖，无法区分"正在执行任务"和"新对话轮次"

## Goals / Non-Goals

**Goals:**
- 任务进行中不重新识别意图，直接路由到 llm 继续执行
- 任务完成时有明确信号，LLM 通过 `finish_task` 工具显式声明完成
- 任务状态持久化到 graph state（通过 checkpoint saver），服务重启后可恢复
- 与现有 hitl interrupt/resume 机制正交，不影响 HITL 流程

**Non-Goals:**
- 支持多并发任务（每个 thread 同一时刻只有一个活跃任务）
- 任务超时自动清除（暂不实现）
- 跨会话任务恢复（仅在同一 lineage 内有效）

## Decisions

### 决策1：用 `StateKeyTaskStatus` 驱动 fallback 后路由，而非重新分析消息历史

**选择**：在 graph state 引入 `StateKeyTaskStatus`（枚举：`idle` / `in_progress` / `completed`），fallback resume 后检查该字段决定路由目标。

**备选方案**：分析最近消息历史（如检测是否有 tool_calls），推断任务是否在进行中。

**原因**：消息历史推断是启发式的，"无 tool_calls"不等于任务完成（LLM 可能在追问信息）。显式状态字段确定性强，无歧义，且随 checkpoint 持久化，服务重启后语义完整。

### 决策2：LLM 通过 `finish_task` 工具显式声明任务完成

**选择**：新增 `finish_task` 声明性工具（与 `human_confirm` 同类型：纯声明，无服务端执行逻辑），LLM 在认为任务完成时调用。tool 节点 post-callback 检测到该工具调用后，将 `StateKeyTaskStatus` 设置为 `completed`，清除 `StateKeyIntent`。

**备选方案**：由特定业务工具（如创建主机 API）调用成功时自动清除状态。

**原因**：
- 业务工具感知任务状态属于业务逻辑污染，`finish_task` 解耦业务工具与流程控制
- LLM 能结合上下文判断任务是否真正完成（如主机创建成功、失败、用户主动放弃均可触发）
- 与 `human_confirm` 设计一致，保持工具体系风格统一

### 决策3：intent_recognition 识别到支持意图时写入 `in_progress` 状态

intent_recognition 节点在路由到 llm 时，同时将 `StateKeyTaskStatus` 置为 `in_progress`。路由到 fallback（不支持意图）时不写入或写入 `idle`，保持"等待新任务"状态。

### 决策4：fallback 后路由逻辑内联在 makeIntentRoutingFunc 中

fallback resume 后不新增独立节点，在现有 fallback node 返回时通过 state 携带路由信息，由 conditional edge 函数读取 `StateKeyTaskStatus` 决定 → `llm`（in_progress）or → `intent_recognition`（idle/completed）。

**Graph 拓扑变更（对比）：**

```
# 旧拓扑
START → intent_recognition → (host_apply → llm | other → fallback)
fallback (interrupt/resume) → intent_recognition

# 新拓扑
START → intent_recognition → (host_apply → llm | other → fallback)
fallback (interrupt/resume) → router
  router: StateKeyTaskStatus == in_progress → llm
  router: StateKeyTaskStatus != in_progress → intent_recognition

llm → tool（含 finish_task） → llm
finish_task 执行时：清除 StateKeyIntent，设置 TaskStatus = completed
```

实现上 router 逻辑集成到 fallback node 的 conditional edge，不单独新增节点。

## Risks / Trade-offs

- **[风险] LLM 忘记调 `finish_task`**：任务永远处于 `in_progress`，用户无法切换到新任务 → **缓解**：System prompt 中明确要求 LLM 在任务完成时必须调用 `finish_task`；后续可增加超时兜底机制
- **[风险] LLM 提前调 `finish_task`**（如主机创建 API 失败后误认为完成）→ **缓解**：在 `finish_task` 工具描述中明确调用时机（任务终态：成功/失败/用户放弃）
- **[Trade-off] 增加 LLM 需遵守的工具调用约定**：LLM 需学习 `finish_task` 语义，对 prompt 质量有依赖 → 可接受，与 `human_confirm` 同等级别的约定

## Migration Plan

1. 变更完全向后兼容：新增常量、枚举、工具声明，不修改任何现有 API 或数据库结构
2. 无需数据迁移，checkpoint state 字段新增是幂等的（缺失时按 `idle` 处理）
3. 部署顺序：直接替换 agent-server 进程即可，checkpoint 中无该字段的旧对话在下次 fallback resume 时会按 `idle` 路由到 intent_recognition，行为退化为旧逻辑，不会崩溃
