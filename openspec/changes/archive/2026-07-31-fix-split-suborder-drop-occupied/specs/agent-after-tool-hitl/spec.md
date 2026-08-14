## MODIFIED Requirements

### Requirement: after_tool_hitl 对拆单试算结果按出单中断

`after_tool_hitl` 节点处理 `get_biz_apply_recommend_split_suborder`（拆单试算）结果时 SHALL 判断**本次返回的增量子单**数量：当 `len(Suborders) >= 1` 时 SHALL 在调用 `graph.Interrupt` 之前构造「主单 + 多子单」详细方案 payload（中断 key 基 `after_tool_hitl.recommend_suborder_confirm.interrupt`，payload 含 `suborders`）并触发中断；当无增量子单时 SHALL 不触发中断并路由回 `llm`。

由于拆单试算接口在增量拆分场景下**只返回本次新算出的增量子单、不回显入参 `occupied_suborders`**，`after_tool_hitl` SHALL 从触发本次中断的那次工具调用的 arguments 中解析 `occupied_suborders`（SHALL 兼容 tool-proxy schema `{parameters:{body_param:{occupied_suborders}}}` 与直连 schema `{body_param:{occupied_suborders}}` 两种嵌套，与 `extractLimit` 的取值模式一致），并将其与返回的增量子单按「已占用子单在前、本次增量子单在后」的顺序合并，以合并后的**完整清单**作为 payload 的 `suborders`，使确认卡片展示本批次全部已确认配置。当 arguments 中不含 `occupied_suborders`、解析失败或其为空数组时 SHALL 退化为仅使用返回的增量子单，且 SHALL NOT 因解析失败而阻断中断。

合并 SHALL 仅作用于中断展示 payload：SHALL NOT 改变拆单试算接口的请求参数与返回契约，SHALL NOT 以合并后的数量作为中断判定依据。

#### Scenario: 拆单试算出详细单触发中断

- **WHEN** `get_biz_apply_recommend_split_suborder` 返回增量子单数 `>= 1` 且入参未带 `occupied_suborders`
- **THEN** `after_tool_hitl` 以返回的增量子单构造主单+子单 payload 并触发中断，等待用户确认

#### Scenario: 增量拆分时合并已占用子单一并展示

- **WHEN** 入参带 `occupied_suborders`（如已确认的 S2.MEDIUM4 子单）且 `get_biz_apply_recommend_split_suborder` 返回增量子单（如 S3.MEDIUM4）
- **THEN** 中断 payload 的 `suborders` 为「已占用子单 + 增量子单」的合并清单（已占用在前），确认卡片同时展示两条配置，历史已确认子单不丢失

#### Scenario: 占用子单解析失败退化为仅增量

- **WHEN** 工具调用 arguments 中 `occupied_suborders` 缺失、结构非法或为空数组
- **THEN** `after_tool_hitl` 仅以返回的增量子单构造 payload 并正常触发中断，不报错、不阻断流程

#### Scenario: 中断判定只看增量子单

- **WHEN** 入参带 `occupied_suborders` 但本次返回的增量子单为空（余量/库存不足，可分配总量 < 1）
- **THEN** `after_tool_hitl` 不触发中断、不因历史占用子单弹出确认卡片，路由回 `llm`

#### Scenario: 拆单试算空结果回 llm

- **WHEN** `get_biz_apply_recommend_split_suborder` 返回空（余量/库存不足，可分配总量 < 1）
- **THEN** `after_tool_hitl` 不触发中断，路由回 `llm`
