## ADDED Requirements

### Requirement: CRP 部门管理员节点跨系统同一人自动过单
在 CRP 审批流程中，当预测单处于部门管理员审批节点（`status=1`），且 CRP 部门管理员节点待审批人列表中包含已在 HCM 部门审批（管理员审批）通过的人（RTX 英文名精确匹配）时，系统 SHALL 调用 `ConfirmOrderForIEG` 自动完成该节点审批，`Operator` 使用匹配到的 RTX 英文名。

该触发与现有阈值自动过单（`checkPredictionAutoApprove`）为"**或**"关系，且**不受"仅追加类型"限制**，覆盖全部子单类型（追加/删除/变更/转移）。

审批参数（使用枚举类型）：
- `ApproveResult`：`cvmapi.ConfirmOrderApproveResultApprove`（0，同意）
- `Status`：`cvmapi.PlanOrderStatusDeptAdmin`（1，部门管理员节点）
- `Operator`：`匹配到的 RTX 英文名`（非 `constant.AdminHandler` 第一个账号）

#### Scenario: 同一人命中时自动过单（覆盖全类型）
- **GIVEN** 预测单在 CRP 部门管理员审批节点（status=1）
- **AND** HCM 部门审批（管理员审批）已由 RTX 为 `forestchen` 的人通过
- **AND** CRP 部门管理员节点待审批人列表包含 `forestchen`
- **WHEN** 系统（`checkCrpTicket` 轮询 status=1）触发自动过单检查
- **THEN** 系统调用 `ConfirmOrderForIEG` 接口
- **AND** `Operator` 为 `forestchen`（匹配 RTX，而非 dommyzhang）
- **AND** 节点状态变为已通过，`forestchen` 不再收到该节点待办
- **AND** 审计标记"系统自动通过（重复审批跳过）"

#### Scenario: 不满足阈值但同一人命中仍自动过单
- **GIVEN** 预测单不满足阈值条件（如机型非标准型、或 CPU>1500 核）
- **AND** HCM 部门审批通过人 RTX 出现在 CRP 部门管理员待审批人列表
- **WHEN** 系统触发自动过单检查
- **THEN** 仍执行自动过单（证明"或"关系，阈值缺失不阻断同一人路径）

#### Scenario: 同一人未命中且阈值不满足时不自动过单
- **GIVEN** 预测单处于 CRP 部门管理员审批节点（status=1）
- **AND** HCM 部门审批通过人 RTX 不在 CRP 待审批人列表
- **AND** 预测单不满足阈值条件
- **WHEN** 系统触发自动过单检查
- **THEN** 不调用 `ConfirmOrderForIEG`
- **AND** 预测单保持待人工审批状态

#### Scenario: HCM 部门审批未通过时不触发同人自动过单
- **GIVEN** 预测单处于 CRP 部门管理员审批节点（status=1）
- **AND** HCM 部门审批状态 = 已拒绝（或审批中 / 未审批），`AdminAuditOperator` 不可用
- **WHEN** 系统触发自动过单检查
- **THEN** 不触发同人自动过单（仅阈值路径可能触发）

#### Scenario: 非部门管理员节点不触发同人自动过单
- **GIVEN** 预测单在 CRP 系统中处于其它审批节点（status != 1）
- **WHEN** 系统轮询检测到该状态
- **THEN** 不执行同人自动过单逻辑

#### Scenario: 删除/变更/转移子单同人路径仍适用
- **GIVEN** 子单类型为删除、变更或转移（非追加）
- **AND** HCM 部门审批通过人 RTX 出现在 CRP 部门管理员待审批人列表
- **WHEN** 系统触发自动过单检查
- **THEN** 同人路径仍自动过单（区别于阈值路径的"仅追加"限制）

### Requirement: 跨系统同一人识别比对待口径
系统 SHALL 以提供方 `checkSamePersonAutoApprove(hcmOperator string, crpPendingApprovers []string) string` 的方式识别同一人：对 HCM 审批人 RTX 与 CRP 待审批人列表逐项比对，比对待口径为**裁剪空白后大小写不敏感**的 RTX 英文名精确匹配；命中返回匹配 RTX，未命中返回空串。

#### Scenario: 命中返回匹配 RTX
- **GIVEN** `hcmOperator = "ForestChen"`（大小写不同）
- **AND** `crpPendingApprovers = ["forestchen"]`
- **WHEN** 调用 `checkSamePersonAutoApprove`
- **THEN** 返回 `"forestchen"`（匹配 RTX）

#### Scenario: 未命中返回空串
- **GIVEN** `hcmOperator = "alice"`
- **AND** `crpPendingApprovers = ["forestchen", "bob"]`
- **WHEN** 调用 `checkSamePersonAutoApprove`
- **THEN** 返回 `""`

#### Scenario: 待审批人为多值（会签）时命中其一
- **GIVEN** `hcmOperator = "carol"`
- **AND** `crpPendingApprovers = ["alice", "carol", "bob"]`
- **WHEN** 调用 `checkSamePersonAutoApprove`
- **THEN** 返回 `"carol"`

### Requirement: HCM 部门审批通过人来源
系统 SHALL 以 `SubTicketInfo.AdminAuditOperator` 作为 HCM 部门审批（管理员审批）通过人的 RTX 来源，且仅当其 `AdminAuditStatus == RPAdminAuditStatusPassed` 时取值。

#### Scenario: HCM 已通过时取得审批人
- **GIVEN** 子单 `AdminAuditStatus == passed` 且 `AdminAuditOperator = "forestchen"`
- **WHEN** 系统构建同人自动过单入参
- **THEN** HCM 审批人 RTX = `forestchen`

#### Scenario: HCM 未通过时审批人不可用
- **GIVEN** 子单 `AdminAuditStatus != passed`
- **WHEN** 系统构建同人自动过单入参
- **THEN** HCM 审批人 RTX 为空，同人路径不触发

### Requirement: CRP 部门管理员待审批人解析
系统 SHALL 在 `checkCrpTicket` 检测到 `status=1` 时，从 `PlanOrderData.BaseInfo.CurrentProcessor` 解析 CRP 部门管理员节点待审批人列表（按 `;`、` `、`|` 拆分并裁剪空白），作为同人识别与自动过单的待审批人入参。

#### Scenario: 解析单值待审批人
- **GIVEN** `CurrentProcessor = "forestchen"`
- **WHEN** 系统解析待审批人列表
- **THEN** 得到 `["forestchen"]`

#### Scenario: 解析多值待审批人
- **GIVEN** `CurrentProcessor = "forestchen;bob|carol"`
- **WHEN** 系统解析待审批人列表
- **THEN** 得到 `["forestchen", "bob", "carol"]`

### Requirement: 同人自动过单审计与异常
同人路径自动过单时，系统 SHALL 记录审计日志，标记"系统自动通过（重复审批跳过）"，含操作人（匹配 RTX）、时间、原因；当 `ConfirmOrderForIEG` 调用失败时，系统 SHALL 记录错误日志并保持节点原状态，等待重试或人工介入。

#### Scenario: 审计标记重复审批跳过
- **GIVEN** 同人路径触发自动过单，`Operator = "forestchen"`
- **WHEN** 系统执行自动过单
- **THEN** 审计日志包含标记"系统自动通过（重复审批跳过）"、操作人 `forestchen`、时间、原因

#### Scenario: CRP 接口调用失败时记录错误并保状态
- **GIVEN** 同人路径满足条件
- **AND** 调用 `ConfirmOrderForIEG` 返回错误
- **WHEN** 系统尝试自动过单
- **THEN** 记录错误日志，包含预测单号、CRP 单号、错误信息
- **AND** 节点保持当前状态，等待重试或人工介入
