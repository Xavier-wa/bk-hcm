## ADDED Requirements

### Requirement: ITSM 资源管理员节点自动过单
在 ITSM 审批流程的资源管理员节点（`ResPlanItsmStepNameAdminApproval = "资源管理员"`），当预测单满足自动过单条件时，系统 SHALL 自动调用 `itsmClient.ApproveNode()` 完成审批。

自动审批参数：
- `Operator`：使用 `enumor.ItsmOperatorHcm`（值为 "admin"）
- 审批意见：包含"自动过单"标识

#### Scenario: 满足条件时自动过单
- **GIVEN** 预测单处于 ITSM 资源管理员审批节点
- **AND** 预测单满足自动过单条件（标准型、≤1500核、≤45TB）
- **WHEN** 系统检测到该节点待审批状态
- **THEN** 系统调用 `itsmClient.ApproveNode()` 自动完成审批
- **AND** `Operator` 为 "admin"
- **AND** 审批意见包含"自动过单"或"auto approve"标识
- **AND** 记录自动审批日志

#### Scenario: 不满足条件时不自动过单
- **GIVEN** 预测单处于 ITSM 资源管理员审批节点
- **AND** 预测单不满足自动过单条件（如 CPU > 1500 核）
- **WHEN** 系统检测到该节点待审批状态
- **THEN** 系统不调用 `itsmClient.ApproveNode()`
- **AND** 预测单保持待人工审批状态

#### Scenario: 非资源管理员节点不触发自动过单
- **GIVEN** 预测单处于 ITSM 其他审批节点（非资源管理员节点）
- **AND** 预测单满足自动过单条件
- **WHEN** 系统检测到该节点
- **THEN** 系统不执行自动过单逻辑
- **AND** 按正常流程处理

#### Scenario: 自动过单失败时记录错误
- **GIVEN** 预测单满足自动过单条件
- **AND** 调用 `itsmClient.ApproveNode()` 返回错误
- **WHEN** 系统尝试自动过单
- **THEN** 记录错误日志，包含预测单号、错误信息
- **AND** 预测单保持当前状态，等待重试或人工介入

### Requirement: ITSM 自动过单操作员账号
系统 SHALL 使用 `enumor.ItsmOperatorHcm`（值为 "admin"）作为 ITSM 自动过单的操作员账号，该账号需具有 ITSM 资源管理员节点的审批权限。

#### Scenario: 使用系统账号执行审批
- **WHEN** 系统执行 ITSM 自动过单
- **THEN** `ApproveNode` 请求中的 `Operator` 字段值为 "admin"
