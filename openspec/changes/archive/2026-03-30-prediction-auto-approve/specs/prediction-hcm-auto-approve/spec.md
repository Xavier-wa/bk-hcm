## ADDED Requirements

### Requirement: HCM 管理员审批节点自动过单
在 HCM 审批流程的 `checkAdminAuditStatus` 环节，当预测单满足自动过单条件时，系统 SHALL 自动设置审批状态为 `skip`（`RPAdminAuditStatusSkip`），跳过人工审批环节。

#### Scenario: 满足条件时自动跳过审批
- **GIVEN** 预测单进入 `checkAdminAuditStatus` 流程
- **AND** 预测单满足自动过单条件（标准型、≤1500核、≤45TB）
- **AND** 当前 `AdminAuditStatus` 为待审批状态
- **WHEN** 系统执行 `checkAdminAuditStatus`
- **THEN** 系统设置 `AdminAuditStatus = RPAdminAuditStatusSkip`
- **AND** 记录自动审批日志，包含预测单号、条件检查结果
- **AND** 流程继续执行后续步骤

#### Scenario: 不满足条件时进入人工审批
- **GIVEN** 预测单进入 `checkAdminAuditStatus` 流程
- **AND** 预测单不满足自动过单条件
- **WHEN** 系统执行 `checkAdminAuditStatus`
- **THEN** 系统不修改 `AdminAuditStatus`
- **AND** 预测单保持待人工审批状态

#### Scenario: 已审批状态不重复处理
- **GIVEN** 预测单进入 `checkAdminAuditStatus` 流程
- **AND** `AdminAuditStatus` 已为通过或拒绝状态
- **WHEN** 系统执行 `checkAdminAuditStatus`
- **THEN** 系统不执行自动过单逻辑
- **AND** 保持现有状态不变

#### Scenario: 自动过单状态更新失败时记录错误
- **GIVEN** 预测单满足自动过单条件
- **AND** 更新 `AdminAuditStatus` 失败（如数据库错误）
- **WHEN** 系统尝试设置 `skip` 状态
- **THEN** 记录错误日志
- **AND** 流程中断或重试

### Requirement: HCM 自动过单日志记录
系统 SHALL 在自动过单时记录审计日志，包含：
- 预测单号
- 审批节点（admin_audit）
- 条件检查结果（CPU核心数、CBS容量）
- 操作时间
- 操作结果（skip）

注：skip 状态不需要填审批人（AdminAuditOperator）

#### Scenario: 记录完整审计日志
- **WHEN** 系统成功执行 HCM 自动过单
- **THEN** 日志包含预测单号
- **AND** 日志包含 "admin_audit" 节点标识
- **AND** 日志包含整单 CPU 核心数
- **AND** 日志包含整单 CBS 容量
- **AND** 日志包含操作时间戳
- **AND** 日志包含操作结果 "skip"
