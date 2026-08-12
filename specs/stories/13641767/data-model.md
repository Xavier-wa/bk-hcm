# 数据模型 — Story 13641767

> 本期无 DDL 迁移；仅在应用层扩展枚举取值与校验规则。

## 1. 实体概览

| 实体 | 表 | 变更类型 |
|------|-----|---------|
| 资源预测主单 | `res_plan_ticket` | 枚举扩展 |
| 资源预测子单 | `res_plan_sub_ticket` | 行为扩展（admin_audit_status 初始值规则） |

## 2. 主单 `res_plan_ticket`

| 字段 | DB 列 | 类型 | 变更 |
|------|-------|------|------|
| `type` / `ticket_type` | `type` | `enumor.RPTicketType` (varchar) | 允许新值 `budget_declare` |
| `type_name` / `ticket_type_name` | （推导字段，非独立列） | string | 允许「预算申报」 |

### 2.1 主单类型枚举（扩展后）

| 值 | 展示名 | 适用场景 |
|----|--------|---------|
| `add` | 新增 | 人工创建 / overwrite_append 显式指定 |
| `adjust` | 调整 | 人工调整 / overwrite_append 显式指定 |
| `delete` | 取消 | 取消单 / overwrite_append 显式指定 |
| `budget_declare` | 预算申报 | **仅** overwrite_append，finops 预算同步 |

**校验**：
- 主单/meta/列表筛选：`ValidateRootTicketType()`（含 budget_declare）
- 子单 sub_type：**不**包含 budget_declare
- 通用 `RPTicketType.Validate()`：保持现有子单类型集合，不含 budget_declare

### 2.2 创建约束

| 入口 | 允许的 type |
|------|------------|
| overwrite_append | `budget_declare` \| `add` \| `adjust` \| `delete`（必填） |
| 人工创建 / adjust / cancel 等 | add/adjust/delete（**拒绝** budget_declare） |

## 3. 子单 `res_plan_sub_ticket`

| 字段 | DB 列 | 变更 |
|------|-------|------|
| `sub_type` | `sub_type` | **无新枚举值**；仍由明细推导 add/adjust/delete/transfer 等 |
| `admin_audit_status` | `admin_audit_status` | 当父主单 `type=budget_declare` 时，创建子单初始值可为 `skip`（含跨年明细） |

### 3.1 子单类型（不变）

沿用 `GetPRSubTicketTypeMembers()`：add, adjust, delay, delete, transfer, transfer_exempt。

### 3.2 管理员审批状态

| 值 | 含义 |
|----|------|
| `auditing` | 待 HCM 管理员审批 |
| `skip` | 跳过 HCM 管理员审批 |
| `done` / `rejected` | 审批完成/驳回 |

**budget_declare 规则**：父主单为 budget_declare 时，子单创建时 **一律** `admin_audit_status=skip`，不受期望交付时间是否跨年影响。

**CRP 阶段**：`stage=crp_audit` 及 CRP 管理员节点逻辑不变，不因 budget_declare 自动跳过。

## 4. API 请求模型（增量）

### OverwriteAppendResPlanTicketReq

```go
type OverwriteAppendResPlanTicketReq struct {
    // ... 现有字段 ...
    Type enumor.RPTicketType `json:"type" validate:"required"`
}
```

**校验规则**：
- `required`：缺失 → 参数错误
- `ValidateRootTicketType()`：仅允许四值

## 5. 关系与行为解耦

```
主单 type=budget_declare
  ├── 落库：type=budget_declare, type_name=预算申报
  ├── 拆单路由：按 demands 中 cancel/add → derive → SplitAdjust|Add|Delete
  └── 子单：sub_type ∈ {add,adjust,delete,...}（非 budget_declare）
           admin_audit_status = skip（HCM）
```

## 6. 兼容性

| 项 | 说明 |
|----|------|
| DB schema | `type` 列为字符串，无需 migration |
| 历史数据 | 无 budget_declare 存量；不影响 |
| 退回计划 | 独立类型体系（ReturnPlanTicketType），本期不改 |

## 7. 不涉及

- 新表 / 新索引
- finops 侧数据模型
- CRP / ITSM 外部 schema
