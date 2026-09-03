## Context

### 现有流程（已落地于 2026-03-30-prediction-auto-approve）

```
HCM 流程:
  ticket → checkAdminAuditStatus
    └─ AdminAuditStatus 检查 → 待审批/已审批/跳过
    审批人记录于 SubTicketInfo.AdminAuditOperator（HCM 部门审批 = 管理员审批）

CRP 流程:
  ticket → 部门管理员(status=1) → 规划经理 → 资源经理 → 架平 → 资源总监
    └─ checkCrpTicket 轮询 QueryPlanOrder
         └─ status=1 → tryAutoApproveCrpDeptAdmin
              └─ 阈值满足(checkPredictionAutoApprove) → ConfirmOrderForIEG(operator=dommyzhang)
              └─ 或同一人命中 → ConfirmOrderForIEG(operator=匹配 RTX)
              └─ 非幂等 10010 视为成功，等下轮 QueryPlanOrder
```

### 现有问题

- 同一管理员既在 HCM 部门审批通过，又出现在 CRP 部门管理员节点待审批人列表，需重复审批。
- 现有 `tryAutoApproveCrpDeptAdmin` 仅覆盖"追加类型 + 阈值"路径，删除/变更/转移子单及超阈值但同一人已审的场景仍走人工，产生冗余等待与催单。

### 涉及文件

| 文件 | 角色 |
|------|------|
| `cmd/woa-server/types/plan/sub_ticket.go` | `SubTicketInfo`（`AdminAuditOperator` / `AdminAuditStatus`） |
| `cmd/woa-server/logics/plan/fetcher/sub_ticket.go` | 子单加载，填充 `AdminAuditOperator` |
| `cmd/woa-server/logics/plan/dispatcher/sub_ticket.go` | `checkCrpTicket` / `tryAutoApproveCrpDeptAdmin` / 非幂等错误处理 |
| `cmd/woa-server/logics/plan/dispatcher/auto_approve.go` | `resolveCrpDeptAdminAutoApprove` / `checkSamePersonAutoApprove` / `parseCrpPendingApprovers` |
| `cmd/woa-server/logics/plan/dispatcher/auto_approve_test.go` | 同人/阈值/解析/10010 单测 |
| `pkg/criteria/constant/res_plan.go` | 审计前缀与 CRP 10010 错误常量 |
| `pkg/thirdparty/cvmapi/` | `QueryPlanOrder`（`CurrentProcessor`）/`ConfirmOrderForIEG` |

## Goals / Non-Goals

**Goals:**

- 在 CRP 部门管理员节点实现"跨系统同一人"自动过单，避免同一管理员在 HCM/CRP 重复审批。
- 与现有阈值自动过单并存（"或"关系），不改变阈值路径行为。
- 覆盖全部子单类型（追加/删除/变更/转移）。
- 复用现有轮询链路与接口，审计留痕与异常兜底与现状一致。
- CRP 非幂等场景下拦截 10010，子单不失败，等待下轮 QueryPlanOrder。

**Non-Goals:**

- 不涉及 ITSM 流程。
- 不改动 CRP 其它节点（规划经理/资源经理/架平/资源总监）的自动化。
- 不改动现有阈值自动过单的常量与判断逻辑。
- 不做 CRP → HCM 的反向去重。
- 不引入新触发机制或 CRP 回调。
- 不使用子单 `message` 存内部 pending 标记（避免污染前端展示）。

## Decisions

### D1: HCM 部门审批通过人来源

**选择**：复用 `SubTicketInfo.AdminAuditOperator`（`string` 字段），当且仅当 `AdminAuditStatus == RPAdminAuditStatusDone` 时取值作为 HCM 审批人 RTX。

**理由**：
- 与需求澄清一致：HCM 部门审批 = 管理员审批（admin audit）。
- 字段已存在于数据表与结构体，与 DB `admin_audit_operator` 列类型一致，无需指针。

**改动点**：`GetSubTicketInfo` 在加载子单时填充 `AdminAuditOperator`。

### D2: CRP 待审批人列表解析

**选择**：在 `checkCrpTicket` 检测到 `status=1` 时，取 `PlanOrderData.BaseInfo.CurrentProcessor`（string），按分隔符 `;` 拆分并裁剪空白，得到待审批人 RTX 列表（与 `fetcher/crp.go` 一致）。

**理由**：
- `CurrentProcessor` 为 CRP 部门管理员节点当前待审批人字段，类型稳定（已确认存在于 `cvmapi_response.go`）。
- CRP 实际返回格式为 `;` 分隔，不额外支持 `|`/空格分隔。

### D3: 同一人识别函数

**选择**：新增 `checkSamePersonAutoApprove(hcmOperator string, crpPendingApprovers []string) string`：

```go
// checkSamePersonAutoApprove 跨系统同一人识别。
// 命中则返回匹配 RTX（用于作为 CRP 确认 Operator），否则返回空串。
// 比对待口径：RTX 英文名精确匹配（裁剪空白、大小写不敏感）。
func checkSamePersonAutoApprove(hcmOperator string, crpPendingApprovers []string) string
```

**理由**：
- 比对待口径 = RTX 英文名精确匹配（需求澄清 D2=A）。
- 与现有阈值检查解耦，独立触发。

### D4: 扩展自动过单触发（或关系 + Operator 分流）

**选择**：新增 `resolveCrpDeptAdminAutoApprove(...)`，由 `tryAutoApproveCrpDeptAdmin` 调用，逻辑为：

```go
// 1) 阈值路径（仅追加类型）
thr := checkPredictionAutoApprove(kt, demands)
// 2) 同一人路径（全类型）
same := checkSamePersonAutoApprove(hcmOperator, crpPending)
// 3) 或关系（阈值优先）
if thr.CanAutoApprove {
    operator = strings.Split(constant.AdminHandler, ";")[0]   // dommyzhang，现状不变
    approve(operator, reason="自动过单: ...")
} else if same != "" {
    operator = same                                        // 匹配 RTX
    approve(operator, reason=constant.SamePersonAutoApproveMemoPrefix + " (operator: ...)")
}
// 二者皆否：保持人工
```

**关键约束**：
- 同一人路径**不经** `checkPredictionAutoApprove` 的"仅追加类型"前置——删除/变更/转移子单，只要 HCM 审批人 ∈ CRP 待审批人即自动过。
- `approve` 复用现有 `ConfirmOrderForIEG` 调用（`ApproveResult=0`, `Status=PlanOrderStatusDeptAdmin`），仅 `Operator` 与审计原因随路径变化。
- 整节点通过：匹配时调一次接口，其余待审批人无需再审（CRP 部门管理员节点或签）。

**理由**：
- 满足需求澄清 D1（或）、D3（全类型）、D4（整节点通过）、D6（审计）。

### D5: 触发机制复用

**选择**：沿用 `checkCrpTicket` 在 `status=1` 时的现有轮询触发，每轮先 `QueryPlanOrder` 再决策是否调用 `ConfirmOrderForIEG`，**不新增定时任务/回调**。

**理由**：需求澄清 Q-001 结论（轮询复用，挂 checkCrpTicket status=1 扩展）。

### D6: 审计与异常

**选择**：
- 同一人路径 CRP 审批意见前缀为 `constant.SamePersonAutoApproveMemoPrefix`（「系统自动通过（重复审批跳过）」），操作人 = 匹配 RTX。
- `ConfirmOrderForIEG` 调用失败：`logs.Errorf` + 保持节点原状态 + 等待重试/人工（与现有阈值路径一致）。

### D7: CRP 非幂等防重

**背景**：已与 CRP（williamhhu）确认 `ConfirmOrderForIEG` 对已审批节点**非幂等**；重复提交典型错误为 **`10010, "单据状态已经发生改变"`**。不建议持续轮询 confirm，应先 QueryPlanOrder 再 confirm（HCM 已遵循）。

**选择**：
1. 在 `tryAutoApproveCrpDeptAdmin` 中识别 CRP 返回的 10010 + 上述文案（`resp.Error` 或 `resp.Result` 均兼容）。
2. 命中时 **`logs.Warnf` 并视为自动过单成功**（`return true, nil`），子单不置失败，等待下轮 `QueryPlanOrder` 推进。
3. **不使用**子单 `message` 存内部 pending 标记（review 反馈：message 不适合、会污染前端）。

**理由**：
- 与 pandafyang 建议一致：拦截非幂等报错后直接忽略，不失败，等下次轮询。
- 无 DB migration，无 UI 副作用。

## Risks / Trade-offs

**[风险] HCM 审批人字段未加载** → D1 已要求 `GetSubTicketInfo` 填充；若 `AdminAuditStatus != done` 或 operator 为空，仅该子单跳过同人自动过单（降级为人工/阈值），不影响其它。

**[风险] CurrentProcessor 格式变化** → D2 仅按 `;` 拆分；若 CRP 变更分隔符需同步调整（与 `fetcher/crp.go` 一并维护）。

**[风险] RTX 大小写/别名不一致** → 比对待口径为裁剪 + 大小写不敏感；若 HCM 与 CRP 同一人 RTX 表述不一致（如带域），需产品/数据侧保证同源。

**[风险] CRP 10010 文案变更** → 使用 `strings.Contains` 匹配核心文案；若 CRP 调整需同步常量。

**[权衡] 同人路径使用真实 RTX 作为 Operator** → 审计更真实（显示本人通过），区别于阈值路径使用 dommyzhang；两条路径审计原因不同，便于追溯。

## Resolved Questions

- **HCM 部门审批 = 管理员审批**：`AdminAuditStatus`/`AdminAuditOperator` 承载，已确认。
- **CRP 待审批人字段**：`PlanOrderData.BaseInfo.CurrentProcessor`（cvmapi_response.go），已确认。
- **触发时机**：轮询复用 `checkCrpTicket` status=1，已确认（Q-001）。
- **比对口径**：RTX 英文名精确匹配（裁剪+大小写不敏感），已确认。
- **覆盖类型**：全类型（追加/删除/变更/转移），已确认。
- **CRP 幂等性**：已与 CRP 确认非幂等；重复提交错误码 **10010**，文案 **「单据状态已经发生改变」**；HCM 按 D7 拦截忽略。
- **分支命名建议**：`res-plan-admin-approval-optimize`（与需求名一致）。
