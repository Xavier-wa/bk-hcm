## ADDED Requirements

### Requirement: CRP 退回计划 client 方法封装

系统 SHALL 在 `pkg/thirdparty/cvmapi` 中封装 CRP（云梯）退回计划相关的 JSON-RPC 方法，统一遵循 CRP JSON-RPC 契约，并统一透传提单人 `userName`。封装的方法 MUST 覆盖：新增（`submitAppendOrder`）、调整&删除（`submitAdjustOrderForApi`）、单据状态查询（`queryOrderDetail`）、退回原因大类查询（`getReasonClassByObsProject`），并扩展现有的退回计划明细查询（`queryReturnPlanItem`）。撤单（`cancelTodo`）本期不封装。

#### Scenario: 提交新增退回计划单

- **WHEN** 调用 `submitAppendOrder`，传入同一 部门 + 规划产品 + 项目类型 + 资源池 的一组退回计划明细及提单人 `userName`
- **THEN** 请求体符合 CRP JSON-RPC 契约，`userName` 正确传递，成功时返回 CRP 退回计划单号

#### Scenario: 提交删除（覆盖）退回计划单

- **WHEN** 调用 `submitAdjustOrderForApi`，以删除模式传入 `src=[{id}]`、`update=[]` 及提单人 `userName`
- **THEN** 请求体符合 CRP 删除单契约，`userName` 正确传递，成功时返回 CRP 单号

#### Scenario: 查询 CRP 单据状态

- **WHEN** 以 CRP 单号 `orderId` 调用 `queryOrderDetail`
- **THEN** 返回该单据的流转状态与明细信息，供调度器推进子单状态机

#### Scenario: 查询退回原因大类

- **WHEN** 按 OBS 项目类型调用 `getReasonClassByObsProject`
- **THEN** 返回该项目类型下可选的退回原因大类列表

#### Scenario: CRP 返回错误时原样透传

- **WHEN** CRP 返回业务错误（如 -20003 / -30000 等 error 结构）
- **THEN** client 将错误信息原样透传给上层，不吞错、不改写错误语义
