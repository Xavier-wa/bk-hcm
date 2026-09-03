## Why

在预测单（资源计划）审批中，管理员往往同时在两套系统对同一工单重复审批：

- **HCM**：部门审批（管理员审批）节点，审批人 = `SubTicketInfo.AdminAuditOperator`，状态 = `AdminAuditStatus = passed`。
- **CRP**：公司审批链路的「部门管理员」节点（`status=1`），待审批人 = `PlanOrderData.BaseInfo.CurrentProcessor`。

现状下，同一管理员既在 HCM 部门审批通过，又出现在 CRP 部门管理员节点的待审批人列表中，需要**在两个平台对同一工单做两次审批**，产生冗余的二次人工等待，业务侧需反复催单。

已有的 `2026-03-30-prediction-auto-approve` 仅在阈值内（标准型机型、CPU≤1500核、CBS≤45TB、仅追加类型）自动过单 CRP 部门管理员节点。本需求在**不改动阈值逻辑**的前提下，新增一条独立触发路径：**当 CRP 部门管理员节点待审批人中包含已在 HCM 部门审批通过的人（RTX 英文名精确匹配）时，系统自动过单该节点**，避免同一人重复审批。两条触发路径为"或"关系，且新路径覆盖全部子单类型（追加/删除/变更/转移）。

需求文档：`docs/reqs/预测单审批优化.md`（TAPD 需求 1069995598128407634，状态 approved，DoR 8/8 通过）。

## What Changes

- **跨系统同一人识别**：新增函数 `checkSamePersonAutoApprove(hcmOperator string, crpPendingApprovers []string) string`，判断 HCM 部门审批通过人的 RTX 英文名是否出现在 CRP 部门管理员节点待审批人列表中，命中则返回匹配 RTX，否则返回空串。

- **扩展 CRP 部门管理员自动过单触发条件**：改造 `tryAutoApproveCrpDeptAdmin`，在现有阈值检查（`checkPredictionAutoApprove`）基础上增加同一人检查，二者为"**或**"关系：
  - 阈值命中（仅追加类型）→ 沿用现有逻辑，`Operator = strings.Split(constant.AdminHandler, ";")[0]`（"dommyzhang"）。
  - 同一人命中 → `Operator = 匹配 RTX`，整节点通过，审计标记"系统自动通过（重复审批跳过）"。
  - 任一命中即自动过单；**同一人路径不受"仅追加类型"限制**，删除/变更/转移子单也适用。

- **HCM 部门审批人读取**：复用 `SubTicketInfo.AdminAuditOperator`（已存在字段），在 HCM 部门审批状态 = `passed` 时取该值作为 HCM 审批人 RTX。确认子单加载逻辑已填充该字段。

- **CRP 待审批人解析**：在 `checkCrpTicket` 检测到 `status=1`（部门管理员节点）时，从 `PlanOrderData.BaseInfo.CurrentProcessor` 解析待审批人列表（按 `;`, `,`, 空格拆分并裁剪，大小写不敏感比对），传入自动过单逻辑。

- **触发机制复用**：沿用现有 Dispatcher 的 CRP 状态轮询链路（在 `checkCrpTicket` 发现 `status=1` 时触发），**不新增触发机制、不引入 CRP 回调**。

- **审计留痕**：同一人路径自动过单时，审批流记录"系统自动通过（重复审批跳过）"，含操作人（匹配 RTX）、时间、原因；接口调用失败保持节点原状态、记日志、等待重试/人工（与现有一致）。

## Capabilities

### New Capabilities

- `prediction-crp-same-person-auto-approve`：CRP 部门管理员节点「跨系统同一人」自动过单能力——当 CRP 部门管理员节点待审批人列表中包含已在 HCM 部门审批通过的人（RTX 英文名精确匹配）时，系统自动调用 `ConfirmOrderForIEG` 完成审批，`Operator` 使用匹配 RTX，独立于现有阈值自动过单，二者为"或"关系，覆盖全部子单类型。

### Modified Capabilities

<!-- 本需求不修改既有 capability 的契约，仅在同一审批域内的 tryAutoApproveCrpDeptAdmin 内新增触发分支，故不标记 MODIFIED；相关上下文见 prediction-crp-auto-approve（archive）。 -->

## Impact

- **cmd/woa-server/types/plan/sub_ticket.go（确认/复用）**：`SubTicketInfo` 已含 `AdminAuditOperator *string`、`AdminAuditStatus`；确认子单加载逻辑在 HCM 部门审批 = passed 时填充 `AdminAuditOperator`。
- **cmd/woa-server/logics/plan/dispatcher/sub_ticket.go（修改）**：`checkCrpTicket` 在 `status=1` 时解析 `PlanOrderData.BaseInfo.CurrentProcessor` 为待审批人列表，传入 `tryAutoApproveCrpDeptAdmin`。
- **cmd/woa-server/logics/plan/dispatcher/auto_approve.go（修改/新增）**：新增 `checkSamePersonAutoApprove`；改造 `tryAutoApproveCrpDeptAdmin` 支持"阈值 或 同一人"双触发，同一人路径使用匹配 RTX 作为 Operator 并打"重复审批跳过"审计。
- **cmd/woa-server/logics/plan/dispatcher/auto_approve_test.go（新增用例）**：补充同一人正/反例、阈值不满足仍过、全类型覆盖、审计标记断言。
- **pkg/thirdparty/cvmapi/（复用）**：`QueryPlanOrder`（`CurrentProcessor`）、`ConfirmOrderForIEG` 现有接口不变。
- **pkg/criteria/constant/（复用）**：`constant.AdminHandler` 现有常量不变。
