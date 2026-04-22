## Context

### 现有流程

预测单审批流程需要经过多个审批节点：

```
ITSM 流程:
  ticket → 资源管理员审批（ResPlanItsmStepNameAdminApproval）
    └─ 人工审批 → 通过/拒绝

HCM 流程:
  ticket → checkAdminAuditStatus
    └─ AdminAuditStatus 检查 → 待审批/已审批/跳过

CRP 流程:
  ticket → 部门管理员(status=1) → 规划经理 → 资源经理 → 架平 → 资源总监
    └─ confirmOrderForIEG(approveResult, status, operator)
```

### 现有问题

- BG资源管理员需要手动审批每一笔预测单，包括大量小额预测单（≤1500核心、≤45TB CBS）
- 审批流程冗长，每个节点都可能产生等待，影响业务资源获取效率
- 小额标准型预测单审批逻辑相对固定，人工介入价值较低

### 涉及文件

| 文件 | 角色 |
|------|------|
| `pkg/criteria/constant/res_plan.go` | 常量定义，自动过单阈值与资源规划常量放在一起 |
| `pkg/criteria/enumor/device.go` | 枚举定义，`DeviceFamilyStandard` 标准型机型 |
| `cmd/woa-server/logics/plan/dispatcher/auto_approve.go` | 自动过单条件检查逻辑 |
| `cmd/woa-server/logics/plan/dispatcher/sub_ticket.go` | `checkAdminAuditStatus` 函数 |
| `pkg/thirdparty/itsm/` | ITSM 客户端，`ApproveNode` 方法 |
| `pkg/thirdparty/cvmapi/` | CRP 接口，`ConfirmOrderForIEG` 方法 |

## Goals / Non-Goals

**Goals:**

- 实现预测单自动过单功能，满足条件时自动完成 BG 资源管理员审批节点
- 支持三个审批系统的自动过单：ITSM、HCM、CRP
- 记录完整的自动过单审计日志

**Non-Goals:**

- 不修改现有审批流程结构，仅在特定条件下自动执行审批操作
- 不涉及自动过单条件的动态配置（当前使用固定阈值）
- 不涉及其他审批节点（如规划经理、资源总监等）的自动化

## Decisions

### D1: 自动过单条件定义

**选择**：前置条件 + 三个条件全部满足时允许自动过单：

**前置条件**：
- 只有"追加"类型的需求单才允许自动过单
- 包含"删除"或"变更"类型需求的单据不走自动过单

**三个条件**：
1. 机型全部为标准型（DeviceFamily == "标准型"），空机型或其他机型都不允许
2. 整单 CPU ≤ 1500 核
3. 整单 CBS ≤ 45TB（46080GB）

**理由**：
- 标准型 CVM 资源充足，审批风险低
- 1500 核和 45TB 为小额预测单的业务定义边界
- 三个条件同时满足可确保自动过单的安全性
- 追加类型风险较低，删除和变更类型需要人工审核

**改动点**：

1. `pkg/criteria/constant/res_plan.go` 新增常量（与资源规划相关常量放在一起）：

```go
const (
    AutoApproveCPUCoreThreshold int64 = 1500

    // 45TB = 45 * 1024 = 46080GB
    AutoApproveCBSSizeThreshold int64 = 46080
)
```

2. 机型判断使用已有枚举 `enumor.DeviceFamilyStandard`（定义在 `pkg/criteria/enumor/device.go`）

### D2: 统一条件检查器实现

**选择**：在 `cmd/woa-server/logics/plan/dispatcher/auto_approve.go` 中实现 `checkPredictionAutoApprove` 函数，返回 `autoApproveCheckResult` 结构体。

**理由**：
- 三个审批节点共用同一套条件判断逻辑，避免重复代码
- 返回结构体便于记录详细的条件检查结果

**数据结构**：

```go
// autoApproveCheckResult 自动过单条件检查结果
type autoApproveCheckResult struct {
    CanAutoApprove bool    // 是否可自动过单
    Reason         string  // 原因说明，不满足条件时包含详细原因
    TotalCPUCores  int64   // 整单 CPU 核心数
    TotalCBSSizeGB int64   // 整单 CBS 容量（GB）
}

func checkPredictionAutoApprove(kt *kit.Kit, demands rpt.ResPlanDemands) *autoApproveCheckResult
```

**检查逻辑**：
1. 遍历 demands，判断需求类型（追加/删除/变更）
2. 遇到删除或变更类型，直接 break 退出，标记不可自动过单
3. 统计 CPU 核心数和 CBS 容量
4. 检查机型是否为标准型（使用 map 去重，避免重复记录相同的非标准机型）
5. 遍历结束后检查 CPU 和 CBS 是否超出阈值

### D3: ITSM 自动过单实现

**选择**：在 ITSM 资源管理员审批节点回调处理中，检测到待审批状态时调用条件检查器，满足条件则调用 `itsmClient.ApproveNode()`。

**关键参数**：
- `Operator`: `enumor.ItsmOperatorHcm`（值为 "admin"）
- 审批意见: 包含 "自动过单" 标识

**改动点**：ITSM 回调处理函数中增加自动过单逻辑

### D4: HCM 自动过单实现

**选择**：在 `checkAdminAuditStatus` 流程中，检查条件后直接设置 `AdminAuditStatus = RPAdminAuditStatusSkip`。

**理由**：
- HCM 内部流程无需外部账号
- `skip` 状态表示跳过人工审批，流程继续执行
- skip 状态不需要填审批人（AdminAuditOperator）

**改动点**：`dispatcher/sub_ticket.go` 的 `checkAdminAuditStatus` 函数

### D5: CRP 自动过单实现

**选择**：监听 CRP 审批状态，当 `status=1`（部门管理员审批）且条件满足时，调用 `confirmOrderForIEG`。

**关键参数**：
- `approveResult`: 0（同意）
- `status`: 1（部门管理员节点）
- `operator`: `strings.Split(constant.AdminHandler, ";")[0]`（"dommyzhang"）

**改动点**：CRP 状态轮询/回调处理中增加自动过单逻辑

### D6: 审计日志记录

**选择**：每次自动过单操作记录详细的审计日志。

**日志内容**：
- 预测单号
- 审批节点（ITSM/HCM/CRP）
- 条件检查结果（CPU核心数、CBS容量、机型）
- 阈值配置
- 操作时间
- 操作结果

## Risks / Trade-offs

**[风险] 自动过单条件可能需要调整**
→ 当前使用固定常量，未来如需动态配置可扩展为配置项

**[风险] 系统账号权限变更**
→ ITSM 使用 "admin" 账号、CRP 使用 "dommyzhang" 账号，需确保账号权限持续有效

**[风险] 审批接口调用失败**
→ 失败时记录错误日志，保持预测单当前状态，等待重试或人工介入

**[权衡] 边界值处理**
→ 采用 ≤ 判断（包含边界值），1500 核和 45TB 恰好满足条件的单据可自动过单

## Resolved Questions

- **ITSM 操作员账号**：使用 `enumor.ItsmOperatorHcm`（"admin"），已在系统中定义
- **CRP 操作员账号**：使用 `constant.AdminHandler` 第一个账号（"dommyzhang"），已在 `ziyan.go` 中定义
- **CBS 容量单位**：统一使用 GB，45TB = 46080GB
- **机型匹配方式**：使用字符串精确匹配 "标准型"
