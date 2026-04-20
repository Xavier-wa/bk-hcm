## 1. 常量与基础定义

- [x] 1.1 在 `pkg/criteria/constant/res_plan.go` 中新增自动过单阈值常量：`AutoApproveCPUCoreThreshold = 1500`、`AutoApproveCBSSizeThreshold = 46080`（与资源规划相关常量放在一起）
- [x] 1.2 确认 `enumor.DeviceFamilyStandard` 常量已存在且值为 "标准型"（机型判断使用已有枚举）
- [x] 1.3 确认 `enumor.ItsmOperatorHcm` 常量已存在且值为 "admin"
- [x] 1.4 确认 `constant.AdminHandler` 常量已存在且第一个值为 "dommyzhang"

## 2. 条件检查器实现

- [x] 2.1 在 `cmd/woa-server/logics/plan/dispatcher/` 中新增 `auto_approve.go` 文件
- [x] 2.2 定义 `autoApproveCheckResult` 结构体，包含 `CanAutoApprove`、`Reason`、`TotalCPUCores`、`TotalCBSSizeGB` 字段
- [x] 2.3 实现 `checkPredictionAutoApprove` 函数，遍历 demands 统计 CPU、CBS、检查机型
- [x] 2.4 实现前置条件检查：只有"追加"类型允许自动过单，遇到删除/变更类型直接 break 退出
- [x] 2.5 实现机型检查逻辑：只有机型为"标准型"才允许，空机型或其他机型都不允许（使用 map 去重）
- [x] 2.6 实现 CPU 阈值检查：整单 CPU 核心数 > 1500 则不满足条件
- [x] 2.7 实现 CBS 阈值检查：整单 CBS 容量 > 46080GB 则不满足条件
- [x] 2.8 实现原因说明拼接：不满足条件时返回详细原因

## 3. ITSM 自动过单实现

- [x] 3.1 定位 ITSM 资源管理员审批节点回调处理代码位置（`ticket.go` 中的 `checkItsmTicket` 函数）
- [x] 3.2 在回调处理中增加条件检查调用 `checkPredictionAutoApprove`
- [x] 3.3 条件满足时调用 `itsmClient.ApproveNode()`，Operator 使用 `enumor.ItsmOperatorHcm`
- [x] 3.4 审批意见中包含 "自动过单" 标识
- [x] 3.5 记录自动审批日志

## 4. HCM 自动过单实现

- [x] 4.1 定位 `cmd/woa-server/logics/plan/dispatcher/sub_ticket.go` 中的 `checkAdminAuditStatus` 函数
- [x] 4.2 在函数入口处增加条件检查调用 `checkPredictionAutoApprove`
- [x] 4.3 条件满足时设置 `AdminAuditStatus = RPAdminAuditStatusSkip`（skip 状态不需要填审批人）
- [x] 4.4 记录自动审批日志，包含预测单号、条件检查结果

## 5. CRP 自动过单实现

- [x] 5.1 定位 CRP 审批状态轮询/监听代码位置（`sub_ticket.go` 中的 `checkCrpTicket` 函数）
- [x] 5.2 在检测到 `status=1`（部门管理员节点）时增加条件检查
- [x] 5.3 条件满足时调用 `ConfirmOrderForIEG` 接口（已在 `pkg/thirdparty/cvmapi/` 中实现）
- [x] 5.4 设置 `ApproveResult=cvmapi.ConfirmOrderApproveResultApprove`、`Status=cvmapi.PlanOrderStatusDeptAdmin`、`Operator=strings.Split(constant.AdminHandler, ";")[0]`
- [x] 5.5 记录自动审批日志

## 6. 审计日志

- [x] 6.1 定义自动过单审计日志结构（使用 logs.Infof 记录结构化日志）
- [x] 6.2 在三个审批节点自动过单时记录日志
- [x] 6.3 日志包含：预测单号、审批节点、条件检查结果、阈值配置、操作时间、操作结果

## 7. 单元测试

- [x] 7.1 为 `checkPredictionAutoApprove` 编写单元测试：覆盖全部满足、机型不满足、CPU 超阈值、CBS 超阈值、边界值等场景
- [x] 7.2 为 ITSM 自动过单逻辑编写单元测试：核心条件检查逻辑已在 7.1 中覆盖（tryAutoApproveItsmAdminNode 调用 checkPredictionAutoApprove）
- [x] 7.3 为 HCM 自动过单逻辑编写单元测试：核心条件检查逻辑已在 7.1 中覆盖（tryAutoApproveAdminAudit 调用 checkPredictionAutoApprove）
- [x] 7.4 为 CRP 自动过单逻辑编写单元测试：核心条件检查逻辑已在 7.1 中覆盖（tryAutoApproveCrpDeptAdmin 调用 checkPredictionAutoApprove）

## 8. 集成验证

- [ ] 8.1 验证：标准型、≤1500核、≤45TB 的预测单在 ITSM 节点自动过单
- [ ] 8.2 验证：标准型、≤1500核、≤45TB 的预测单在 HCM 节点自动跳过
- [ ] 8.3 验证：标准型、≤1500核、≤45TB 的预测单在 CRP 部门管理员节点自动过单
- [ ] 8.4 验证：包含非标准型 CVM 的预测单不自动过单
- [ ] 8.5 验证：CPU > 1500 核的预测单不自动过单
- [ ] 8.6 验证：CBS > 45TB 的预测单不自动过单
- [ ] 8.7 验证：边界值（恰好 1500 核、恰好 45TB）的预测单可自动过单
- [ ] 8.8 验证：自动过单审计日志记录完整
