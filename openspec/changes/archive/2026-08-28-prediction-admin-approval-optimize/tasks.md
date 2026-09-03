## 1. 数据来源确认

- [x] 1.1 确认 `cmd/woa-server/types/plan/sub_ticket.go` 中 `SubTicketInfo` 已含 `AdminAuditOperator string` 与 `AdminAuditStatus` 字段
- [x] 1.2 确认子单加载逻辑（`GetSubTicketInfo`）在 `AdminAuditStatus == done` 时已填充 `AdminAuditOperator`
- [x] 1.3 确认 `pkg/thirdparty/cvmapi/cvmapi_response.go` 中 `PlanOrderData.BaseInfo.CurrentProcessor` 字段存在且类型正确

## 2. CRP 待审批人解析

- [x] 2.1 在 `cmd/woa-server/logics/plan/dispatcher/sub_ticket.go` 的 `checkCrpTicket` 中，定位 `status=1`（部门管理员节点）分支
- [x] 2.2 解析 `PlanOrderData.BaseInfo.CurrentProcessor` 为待审批人列表（按 `;` 拆分并裁剪空白，与 `fetcher/crp.go` 一致）
- [x] 2.3 取得 HCM 审批人 RTX：`SubTicketInfo.AdminAuditOperator`（仅当 `AdminAuditStatus == done`）
- [x] 2.4 将 `adminAuditOperator` 与 `crpPendingApprovers` 传入 `tryAutoApproveCrpDeptAdmin`（扩展函数签名）

## 3. 同一人识别函数

- [x] 3.1 在 `cmd/woa-server/logics/plan/dispatcher/auto_approve.go` 新增 `checkSamePersonAutoApprove(hcmOperator string, crpPendingApprovers []string) string`
- [x] 3.2 实现裁剪空白 + 大小写不敏感精确匹配
- [x] 3.3 命中返回匹配 RTX，未命中返回空串

## 4. 扩展 CRP 部门管理员自动过单（或关系）

- [x] 4.1 改造 `tryAutoApproveCrpDeptAdmin`：保留现有阈值检查 `checkPredictionAutoApprove` 分支
- [x] 4.2 新增同一人分支：调用 `checkSamePersonAutoApprove`，命中则 `Operator = 匹配 RTX`
- [x] 4.3 实现"或"关系：阈值命中（Operator=dommyzhang）**或** 同一人命中（Operator=匹配 RTX）→ 自动过单；二者皆否保持人工
- [x] 4.4 同一人路径**不经**"仅追加类型"前置，覆盖删除/变更/转移子单
- [x] 4.5 复用现有 `ConfirmOrderForIEG` 调用（`ApproveResult=0`, `Status=PlanOrderStatusDeptAdmin`）
- [x] 4.6 同一人路径审计原因使用 `constant.SamePersonAutoApproveMemoPrefix`（「系统自动通过（重复审批跳过）」），操作人=匹配 RTX
- [x] 4.7 接口失败：记录错误日志、保持节点原状态、等待重试/人工（与现有一致）

## 5. CRP 重复提交防重（非幂等兜底）

- [x] 5.1 已与 CRP 确认：`ConfirmOrderForIEG` 对已审批节点**非幂等**；重复提交典型错误为 `10010, "单据状态已经发生改变"`
- [x] 5.2 拦截上述非幂等错误/结果，视为自动过单成功（`logs.Warnf` + 等待下轮 `QueryPlanOrder`），子单不置失败
- [x] 5.3 不使用子单 `message` 存内部标记（避免前端展示内部字符串）

## 6. 单元测试

- [x] 6.1 `checkSamePersonAutoApprove` 单测：命中（含大小写不同）、未命中、多值会签命中其一
- [x] 6.2 `tryAutoApproveCrpDeptAdmin` 单测：同一人命中自动过单且 Operator=匹配 RTX
- [x] 6.3 单测：阈值不满足但同人命中仍自动过单（验证"或"关系）
- [x] 6.4 单测：同人未命中且阈值不满足 → 不自动过单，保持人工
- [x] 6.5 单测：HCM 未通过（`AdminAuditStatus != done`）→ 同人路径不触发
- [x] 6.6 单测：删除/变更/转移子单同人路径仍适用
- [x] 6.7 单测：CRP 接口调用失败 → 记日志、保状态
- [x] 6.8 单测：`parseCrpPendingApprovers` 按 `;` 拆分；`isCrpConfirmOrderStatusChanged` 识别 10010
- [x] 6.9 单测：CRP 返回 10010 时 `tryAutoApproveCrpDeptAdmin` 视为成功

## 7. 集成验证

- [x] 7.1 验证：HCM 部门审批通过人 ∈ CRP 部门管理员待审批人 → 自动过单，Operator=匹配 RTX，CRP 审计「重复审批跳过」（dev 联调已验证）
- [ ] 7.2 验证：超阈值但同人命中 → 仍自动过单
- [ ] 7.3 验证：同人未命中且超阈值 → 不自动过单，保持人工
- [ ] 7.4 验证：HCM 部门审批拒绝/审批中/skip → 同人路径不触发
- [ ] 7.5 验证：删除/变更/转移子单同人路径生效
- [ ] 7.6 验证：现有阈值路径行为不变（追加+标准型+≤1500核+≤45TB 仍用 dommyzhang 过单）
- [ ] 7.7 验证：重复 `ConfirmOrderForIEG` 返回 10010 时子单不失败，下轮正常推进
