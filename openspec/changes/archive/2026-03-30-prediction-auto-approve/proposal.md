## Why

当前预测单审批流程中，BG资源管理员（事业群级别的资源审批管理员）需要手动审批每一笔CVM标准型预测单，存在以下问题：

1. **审批效率低下**：大量小额预测单（≤1500核心、≤45TB CBS）仍需人工审批，占用BG资源管理员大量时间。这些小额预测单审批逻辑相对固定，人工介入价值较低。

2. **审批流程冗长**：预测单需要经过多个审批节点，每个节点都可能产生等待：
   - ITSM系统：资源管理员审批
   - HCM系统：管理员审批（admin_audit）
   - CRP系统：部门管理员审批（status=1）→ 规划经理 → 资源经理 → 架平 → 资源总监

3. **业务痛点**：BG资源管理员日常审批工作繁重，小额预测单审批周期过长，影响业务资源获取效率。

通过实现自动过单功能，可以在满足特定条件时自动完成BG资源管理员审批节点，提升审批效率。

## What Changes

- **自动过单阈值常量定义**：在 `pkg/criteria/constant/res_plan.go` 中新增自动过单阈值常量，定义 CPU核心数阈值（1500核）、CBS容量阈值（45TB=46080GB）。机型判断使用已有枚举 `enumor.DeviceFamilyStandard`（"标准型"）。

- **自动过单条件检查器**：实现统一的条件检查逻辑 `checkPredictionAutoApprove`，判断预测单是否满足自动过单条件。前置条件：只有"追加"类型的需求单才允许自动过单，包含"删除"或"变更"类型的单据不走自动过单。三个条件必须全部满足：机型全部为标准型（空机型也不允许）、整单CPU≤1500核、整单CBS≤45TB。

- **ITSM资源管理员节点自动过单**：在ITSM审批流程的资源管理员节点（`ResPlanItsmStepNameAdminApproval = "资源管理员"`），当条件满足时调用 `itsmClient.ApproveNode()` 自动完成审批，Operator 使用 `enumor.ItsmOperatorHcm`（值为 "admin"）。

- **HCM管理员审批节点自动过单**：在 `checkAdminAuditStatus` 流程中，当条件满足时自动设置审批状态为 `skip`，跳过人工审批环节。skip 状态不需要填审批人。

- **CRP部门管理员节点自动过单**：在CRP审批流程中，当状态为部门管理员审批节点（status=1）且条件满足时，调用云梯审批接口 `ConfirmOrderForIEG` 自动完成审批。`approveResult=0`（同意），`status=1`（部门管理员节点），`operator` 使用 `strings.Split(constant.AdminHandler, ";")[0]`（值为 "dommyzhang"）。

## Capabilities

### New Capabilities

- `prediction-auto-approve-checker`: 预测单自动过单条件检查能力——实现统一的条件判断逻辑，检查预测单是否满足自动过单条件。前置条件：只有"追加"类型需求允许自动过单。三个条件：标准型CVM（空机型不允许）、≤1500核心、≤45TB CBS。返回检查结果（是否可自动过单、原因说明、整单CPU核心数、整单CBS容量）。

- `prediction-itsm-auto-approve`: ITSM资源管理员审批自动过单能力——在ITSM审批流程的资源管理员节点（`ResPlanItsmStepNameAdminApproval`），当条件满足时调用 `itsmClient.ApproveNode()` 自动完成审批，Operator 使用系统账号 `enumor.ItsmOperatorHcm`（"admin"）。

- `prediction-hcm-auto-approve`: HCM管理员审批自动过单能力——在 `checkAdminAuditStatus` 流程中，当条件满足时自动设置审批状态为 `skip`，跳过人工审批环节，skip 状态不需要填审批人，记录自动审批日志。

- `prediction-crp-auto-approve`: CRP部门管理员审批自动过单能力——在CRP审批流程中，当状态为部门管理员审批节点（status=1）且条件满足时，调用云梯审批接口 `ConfirmOrderForIEG` 自动完成审批，`operator` 使用 `constant.AdminHandler` 中的第一个账号（"dommyzhang"）。

### Modified Capabilities

<!-- 无 -->

## Impact

- **pkg/criteria/constant/res_plan.go（修改）**：新增自动过单阈值常量定义，包括 `AutoApproveCPUCoreThreshold`（1500）、`AutoApproveCBSSizeThreshold`（46080），与资源规划相关常量放在一起

- **pkg/criteria/enumor/device.go（已有）**：使用已有的 `DeviceFamilyStandard`（"标准型"）枚举

- **woa-server / logics/plan/dispatcher（新增）**：`auto_approve.go` 新增 `checkPredictionAutoApprove` 函数，实现统一的条件检查逻辑，返回 `autoApproveCheckResult` 结构体

- **woa-server / ITSM回调处理（修改）**：在资源管理员审批节点检测到待审批时，调用条件检查器，条件满足时调用 `itsmClient.ApproveNode()` 自动完成审批

- **woa-server / dispatcher（主改动）**：`cmd/woa-server/logics/plan/dispatcher/sub_ticket.go` 的 `checkAdminAuditStatus` 函数增加自动过单条件检查，满足条件时设置 `AdminAuditStatus = RPAdminAuditStatusSkip`，skip 状态不需要填审批人

- **woa-server / CRP交互（新增/修改）**：监听CRP审批状态，当 `status=1`（部门管理员审批）时触发条件检查，条件满足时调用云梯审批接口 `ConfirmOrderForIEG`

- **日志与审计（新增）**：记录自动过单审计日志，包含预测单号、审批节点、条件检查结果、阈值配置、操作时间、操作结果

- **系统账号配置**：ITSM使用 `enumor.ItsmOperatorHcm`（"admin"），CRP使用 `constant.AdminHandler` 第一个账号（"dommyzhang"），HCM内部处理无需外部账号
