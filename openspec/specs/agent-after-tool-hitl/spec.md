# agent-after-tool-hitl Specification

## Purpose

为主机申领 Agent 的 Graph 提供「工具调用后中断」能力：在保留既有 `hitl`（`human_confirm`，工具调用前中断）的同时，新增 `after_tool_hitl` 节点与 `tool` 节点条件路由，使 Agent 能在调用离线偏好推荐、预测余量推荐、拆单试算等推荐工具后，按结果判定是否中断让用户确认方案，并以 Graph state 累积合并推荐候选、恢复后回到 `llm` 继续任务。

## Requirements

### Requirement: tool 节点后置条件路由识别推荐工具

`BuildGraph` SHALL 将 `tool` 节点的固定回边替换为条件路由，由路由函数检查最近一条 assistant 消息的 `tool_calls`：当包含 `get_biz_apply_recommend_by_static`（离线偏好推荐）、`get_biz_apply_recommend_by_plan`（预测余量推荐）或 `get_biz_apply_recommend_split_suborder`（拆单试算）之一时 SHALL 路由到 `after_tool_hitl` 节点；否则 SHALL 路由到 `llm` 节点（保持原有行为）。`hitl`（`human_confirm`，工具调用前中断）节点与其路由 SHALL 保持不变。

#### Scenario: 调用推荐工具后进入 after_tool_hitl

- **WHEN** `tool` 节点执行的本轮 `tool_calls` 含三个推荐工具之一
- **THEN** 路由到 `after_tool_hitl` 节点

#### Scenario: 非推荐工具保持回 llm

- **WHEN** `tool` 节点执行的本轮 `tool_calls` 不含推荐工具（如 skill 工具或其他 MCP 工具）
- **THEN** 路由到 `llm` 节点

#### Scenario: 工具调用前中断不受影响

- **WHEN** LLM 仅调用 `human_confirm`
- **THEN** 仍路由到 `hitl` 节点并触发工具调用前中断，行为与改造前一致

### Requirement: after_tool_hitl 对离线偏好推荐结果按库存够数判定中断

`after_tool_hitl` 节点处理 `get_biz_apply_recommend_by_static`（离线偏好推荐）结果时 SHALL 比较返回方案数与入参 `limit`：当 `len(Items) >= limit` 时 SHALL 在调用 `graph.Interrupt` 之前构造方案 payload（中断 key 基 `after_tool_hitl.recommend_select.interrupt`，payload 含 `recommendations`）并触发中断；当 `len(Items) < limit`（含为 0）时 SHALL 不触发中断并路由回 `llm`。`limit` 缺省值 SHALL 为 `DefaultRecommendLimit=5`（入参缺失时由 `extractLimit` 兜底）。

#### Scenario: 离线偏好推荐方案够数触发中断

- **WHEN** `get_biz_apply_recommend_by_static` 返回方案数 `>= limit`
- **THEN** `after_tool_hitl` 构造方案 payload 后触发中断，等待用户选择

#### Scenario: 离线偏好推荐方案不足回 llm 补充

- **WHEN** `get_biz_apply_recommend_by_static` 返回方案数 `< limit`
- **THEN** `after_tool_hitl` 不触发中断，路由回 `llm`，由 LLM 续调 `get_biz_apply_recommend_by_plan`

### Requirement: after_tool_hitl 对预测余量推荐结果合并候选并按有方案中断

`after_tool_hitl` 节点处理 `get_biz_apply_recommend_by_plan`（预测余量推荐）结果时 SHALL 将其追加到已累积的离线偏好推荐候选之后（离线偏好在前、预测余量补足，过滤无 `Suborder` 的无效项；当前实现为顺序拼接 + 过滤，不做组合键去重）；当合并后 `len > 0` 时 SHALL 在调用 `graph.Interrupt` 之前构造合并方案 payload（中断 key 基 `after_tool_hitl.recommend_select.interrupt`）并触发中断；当合并后无方案时 SHALL 不触发中断并路由回 `llm`。

#### Scenario: 预测余量推荐后合并有方案触发中断

- **WHEN** 离线偏好与预测余量合并后方案数 `> 0`
- **THEN** `after_tool_hitl` 以合并方案构造 payload 并触发中断

#### Scenario: 预测余量推荐后仍无方案回 llm 决策

- **WHEN** 离线偏好与预测余量合并去重后无任何方案
- **THEN** `after_tool_hitl` 不触发中断，路由回 `llm` 由其决策（问用户 / 走无预测分支）

### Requirement: after_tool_hitl 对拆单试算结果按出单中断

`after_tool_hitl` 节点处理 `get_biz_apply_recommend_split_suborder`（拆单试算）结果时 SHALL 判断拆分子单数量：当 `len(Suborders) >= 1` 时 SHALL 在调用 `graph.Interrupt` 之前构造「主单 + 多子单」详细方案 payload（中断 key 基 `after_tool_hitl.recommend_suborder_confirm.interrupt`，payload 含 `suborders`）并触发中断；当无子单时 SHALL 不触发中断并路由回 `llm`。

#### Scenario: 拆单试算出详细单触发中断

- **WHEN** `get_biz_apply_recommend_split_suborder` 返回子单数 `>= 1`
- **THEN** `after_tool_hitl` 构造主单+子单 payload 并触发中断，等待用户确认

#### Scenario: 拆单试算空结果回 llm

- **WHEN** `get_biz_apply_recommend_split_suborder` 返回空（余量/库存不足，可分配总量 < 1）
- **THEN** `after_tool_hitl` 不触发中断，路由回 `llm`

### Requirement: 推荐候选以 Graph state 累积且离线偏好推荐为轮边界

`after_tool_hitl` SHALL 以 Graph state 累积推荐候选：处理 `get_biz_apply_recommend_by_static` 时 SHALL 覆盖写候选集合（作为新一轮推荐的边界，丢弃上一轮残留）；处理 `get_biz_apply_recommend_by_plan` 时 SHALL 在已有候选基础上合并去重后写回。中断路径下候选 delta 因 InterruptError 不被应用 SHALL 不影响后续流程，故合并 payload SHALL 在中断前构造完成。

#### Scenario: 离线偏好推荐覆盖写清除上一轮候选

- **WHEN** 新一轮推荐先调用 `get_biz_apply_recommend_by_static`
- **THEN** state 中的推荐候选被其结果覆盖，不含上一轮残留

#### Scenario: 预测余量推荐在离线偏好候选上合并

- **WHEN** 同一轮离线偏好推荐不足后调用 `get_biz_apply_recommend_by_plan`
- **THEN** state 候选为离线偏好候选与预测余量有效项的顺序合并结果（离线偏好在前）

### Requirement: after_tool_hitl 恢复后回到 llm

`after_tool_hitl` 触发中断并经用户输入 resume 后 SHALL 将用户选择并入消息（复用 `message.BuildFallbackResumeDelta`）并路由回 `llm` 继续当前任务；`after_tool_hitl` → `llm` SHALL 为循环边。恢复时 SHALL 优先采用前端经 `forwardedProps` 传入、存于 `RunOptions.RuntimeState[StateKeyForwardedResumeValue]` 的结构化回复：命中（非空字符串）时 SHALL 以其作为用户选择并发自定义事件 `after_tool_hitl.resume_forwarded`（payload `{value}`）、忽略自由文本 resume 值；未命中时 SHALL 回退使用中断 resume 的自由文本值，且该值 SHALL 为非空字符串（否则报错终止）。

#### Scenario: 前端结构化回复优先恢复

- **WHEN** 前端经 `forwardedProps` 传入了非空结构化回复
- **THEN** `after_tool_hitl` 以该结构化值作为用户选择、发 `after_tool_hitl.resume_forwarded` 事件，并将其并入消息回到 `llm`

#### Scenario: 无结构化回复时用自由文本恢复

- **WHEN** 无 `forwardedProps` 结构化回复，仅有中断 resume 的自由文本值
- **THEN** `after_tool_hitl` 校验其为非空字符串后并入消息，流程回到 `llm` 继续
