# Change: Agent 主机推荐 after_tool_hitl 双中断

## Why

主机申领 Agent 已具备离线偏好/预测余量/单据拆分三个在线推荐接口（已在 `cmd/woa-server/service/task/recommend.go` 落地并经 APIGW 暴露为 MCP 工具），但 Graph 当前只有「工具调用前」一个中断节点（`human_confirm` → `hitl`），无法支撑「先调推荐工具拿方案、再按结果决定是否中断让用户确认」的推荐交互闭环。

## What Changes

- **Graph 双中断**：保留 `hitl`（`human_confirm`，即工具调用前中断）不变；新增 `after_tool_hitl`（工具调用后中断）节点，并把 `tool` 节点的固定回边改为条件路由。
- **推荐工具触发与判定**：`get_biz_apply_recommend_by_static`（离线偏好推荐）、`get_biz_apply_recommend_by_plan`（预测余量推荐）、`get_biz_apply_recommend_split_suborder`（拆单试算）调用后进入 `after_tool_hitl`，由节点按结果判定中断或回 `llm`。
- **候选累积合并**：用 Graph state（`recommend:candidates`，以 JSON 字符串存储）累积推荐候选（离线偏好推荐覆盖写；预测余量推荐在已有候选后追加其有效项，离线偏好在前、预测余量补足，过滤无 `Suborder` 的无效项，不做组合键去重），中断 payload 在调用 `graph.Interrupt` 之前构造。
- **范围排除**：推荐流程的 Skill 沉淀、同机型族查询工具与「用户选择扩展 → 换机型 → 再拆单」闭环本期**不实现**。

## Impact

- Affected specs: `agent-after-tool-hitl`（新增）
- Affected code:
  - `cmd/agent-server/logics/agent/graph_build.go`（新增 `after_tool_hitl` 节点、`tool` 节点条件路由）
  - `cmd/agent-server/logics/agent/aftertool/`（新增通用 `after_tool_hitl` 节点包：`node.go` 负责路由/分发/中断与 resume，`parse.go` 负责工具调用定位与多层结果解包）
  - `pkg/criteria/enumor/aiagent.go`（新增节点枚举 `CvmApplyNodeAfterToolHITL` 并纳入 `Validate`）
  - `pkg/criteria/constant/aiagent.go`（新增两个中断 key、resume 自定义事件名 `AfterToolHITLResumeForwardedEventName`、推荐工具名常量、`DefaultRecommendLimit=5`、候选 state key、`ProxyExecuteToolFullName`）
- 不改动：三个推荐接口的业务实现（已完成）、`human_confirm`/`hitl` 既有逻辑。
- 前端外依赖（本期后端不实现）：新增的两个 `after_tool_hitl.*` 中断 key 的渲染，以及通过 `forwardedProps` 回传结构化选择、消费 `after_tool_hitl.resume_forwarded` 自定义事件。
