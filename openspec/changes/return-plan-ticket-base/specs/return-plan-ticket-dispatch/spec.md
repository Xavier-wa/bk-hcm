## ADDED Requirements

### Requirement: 退回计划拆单逻辑

系统 SHALL 将主单明细按 部门 + 规划产品 + 项目类型 + 资源池 分组拆分为子单，且新增（add）条目与删除（cancel）条目 MUST 拆分为不同子单，每个子单 MUST 对应且仅对应一个 CRP 退回计划单据。

#### Scenario: 按维度分组拆单

- **WHEN** 一张主单包含跨多个 部门/规划产品/项目类型/资源池 的退回计划明细
- **THEN** 按该四维度分组拆分，每组生成独立子单

#### Scenario: 新增与删除分单

- **WHEN** 主单同时包含 add 条目与 cancel 条目
- **THEN** add 条目与 cancel 条目拆分到不同子单，满足 CRP「新增与删除不能同单」约束

### Requirement: 退回计划调度流转与状态机

系统 SHALL 新建退回计划调度器，采用「主单入队 → 处理 → 子单入队 → 处理」模型，仅 master 节点执行。退回计划 MUST NOT 走 HCM 侧 ITSM 阶段（不接入蓝鲸 ITSM），主单创建后子单直接进入 CRP 提单；CRP 单据自身的审批环节可能使子单终结为 `rejected`（审批驳回）、`revoked`（撤销）、`invalid`（失效）等状态。子单调度 MUST 调用 CRP 提单（add 用 `submitAppendOrder`，cancel 用 `submitAdjustOrderForApi`）并写入 `crp_sn`/`crp_url`，随后 MUST 通过 `queryOrderDetail` 轮询推进子单状态机（`done`/`rejected`/`revoked`/`invalid`/`failed`）。主单状态 MUST 聚合子单状态：全部成功 → `done`；全部失败 → `failed`；全部驳回 → `rejected`；含未成功子单但非全部同类 → `partial_failed`/`partial_rejected`。

#### Scenario: 子单提单并轮询

- **WHEN** 子单处于待提单状态且调度器运行
- **THEN** 调度器调用 CRP 提单写入 `crp_sn`/`crp_url`，并通过 `queryOrderDetail` 轮询推进子单状态到 `done`/`rejected`/`revoked`/`invalid`/`failed`

#### Scenario: 无 HCM ITSM 阶段

- **WHEN** 退回计划主单被创建
- **THEN** 子单直接进入 CRP 提单流程，不经过 HCM 侧（蓝鲸 ITSM）审批阶段；CRP 单据自身的审批结果通过 `queryOrderDetail` 轮询获知

#### Scenario: 主单状态聚合

- **WHEN** 所有子单流转结束
- **THEN** 全部成功聚合为 `done`，全部失败聚合为 `failed`，全部驳回聚合为 `rejected`，含未成功子单但非全部同类聚合为 `partial_failed`/`partial_rejected`

### Requirement: 失败原因聚合到主单

系统 SHALL 在主单进入 `failed` 或 `partial_failed` 状态时，将各失败子单的失败原因聚合拼接后写入主单 `message` 字段，以便在主单层面直接查看失败详情。

#### Scenario: 部分子单失败时聚合失败原因

- **WHEN** 主单下部分子单失败、主单聚合为 `partial_failed`
- **THEN** 主单 `message` 包含各失败子单失败原因的拼接内容

#### Scenario: 全部子单失败时聚合失败原因

- **WHEN** 主单下全部子单失败、主单聚合为 `failed`
- **THEN** 主单 `message` 包含各失败子单失败原因的拼接内容

### Requirement: 失败子单可单独重试

系统 SHALL 支持对处于失败态的子单单独重试 CRP 提单流转，重试 MUST NOT 影响已成功的子单。

#### Scenario: 重试仅作用于失败子单

- **WHEN** 主单下某子单 CRP 提单失败并触发重试
- **THEN** 仅该失败子单重新发起 CRP 提单流转，已成功子单状态保持不变
