## ADDED Requirements

### Requirement: CRP 部门管理员节点自动过单
在 CRP 审批流程中，当预测单状态为部门管理员审批节点（`status=1`）且满足自动过单条件时，系统 SHALL 调用云梯审批接口 `ConfirmOrderForIEG` 自动完成审批。

审批参数（使用枚举类型）：
- `ApproveResult`：`cvmapi.ConfirmOrderApproveResultApprove`（0，同意）
- `Status`：`cvmapi.PlanOrderStatusDeptAdmin`（1，部门管理员节点）
- `Operator`：使用 `strings.Split(constant.AdminHandler, ";")[0]`（值为 "dommyzhang"）

#### Scenario: 满足条件时自动过单
- **GIVEN** 预测单在 CRP 系统中处于部门管理员审批节点（status=1）
- **AND** 预测单满足自动过单条件（追加类型、标准型、≤1500核、≤45TB）
- **WHEN** 系统检测到该状态
- **THEN** 系统调用 `ConfirmOrderForIEG` 接口
- **AND** `ApproveResult` 为 `ConfirmOrderApproveResultApprove`（0，同意）
- **AND** `Status` 为 `PlanOrderStatusDeptAdmin`（1）
- **AND** `Operator` 为 "dommyzhang"
- **AND** 记录自动审批日志

#### Scenario: 不满足条件时不自动过单
- **GIVEN** 预测单在 CRP 系统中处于部门管理员审批节点（status=1）
- **AND** 预测单不满足自动过单条件
- **WHEN** 系统检测到该状态
- **THEN** 系统不调用 `ConfirmOrderForIEG` 接口
- **AND** 预测单保持待人工审批状态

#### Scenario: 非部门管理员节点不触发自动过单
- **GIVEN** 预测单在 CRP 系统中处于其他审批节点（status != 1）
- **AND** 预测单满足自动过单条件
- **WHEN** 系统检测到该状态
- **THEN** 系统不执行自动过单逻辑
- **AND** 按正常流程等待对应节点审批

#### Scenario: CRP 接口调用失败时记录错误
- **GIVEN** 预测单满足自动过单条件
- **AND** 调用 `ConfirmOrderForIEG` 返回错误
- **WHEN** 系统尝试自动过单
- **THEN** 记录错误日志，包含预测单号、CRP 单号、错误信息
- **AND** 预测单保持当前状态，等待重试或人工介入

### Requirement: CRP 自动过单操作员账号
系统 SHALL 使用 `constant.AdminHandler` 中的第一个账号（"dommyzhang"）作为 CRP 自动过单的操作员，该账号需具有 CRP IEG 部门管理员的审批权限。

获取方式：`strings.Split(constant.AdminHandler, ";")[0]`

#### Scenario: 使用系统账号执行审批
- **WHEN** 系统执行 CRP 自动过单
- **THEN** `ConfirmOrderForIEG` 请求中的 `Operator` 字段值为 "dommyzhang"

#### Scenario: AdminHandler 常量格式
- **GIVEN** `constant.AdminHandler = "dommyzhang;forestchen"`
- **WHEN** 系统解析操作员账号
- **THEN** 使用分号分隔后取第一个值 "dommyzhang"

### Requirement: CRP 审批状态监听
系统 SHALL 监听 CRP 审批状态变化，当检测到 `status=1`（部门管理员审批节点）时触发自动过单条件检查。

#### Scenario: 检测到部门管理员节点时触发检查
- **GIVEN** 系统正在轮询/监听 CRP 审批状态
- **WHEN** 检测到预测单 `status` 变为 1
- **THEN** 系统调用 `checkPredictionAutoApprove` 检查条件
- **AND** 根据检查结果决定是否自动过单

#### Scenario: 状态变化后及时响应
- **GIVEN** 预测单从 status=0 变为 status=1
- **WHEN** 系统下一次轮询检测到该变化
- **THEN** 立即执行自动过单条件检查
- **AND** 条件满足时立即调用审批接口
