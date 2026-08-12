# 技术调研 — Story 13641767

> 需求：资源预测主单新增「预算申报」（`budget_declare`），overwrite_append 必填 `type`，子单 HCM 管理员免审。

## 调研范围

| 主题 | 代码位置 | 结论摘要 |
|------|---------|---------|
| 主单类型枚举 | `pkg/criteria/enumor/woa_ziyan_resplan.go` | 现有 `RPTicketType` 含 add/adjust/delete 等；主单 meta 列表来自 `GetRPTicketTypeMembers()` |
| overwrite_append 主单 type | `cmd/woa-server/logics/plan/overwrite_append.go` L91-92 | 现网调用 `deriveOverwriteAppendTicketType` 自动推导，需改为请求 `type` |
| 请求体结构 | `cmd/woa-server/types/plan/ticket_overwrite_append.go` | 尚无 `type` 字段，需新增必填 |
| 子单 HCM 管理员 skip | `cmd/woa-server/logics/plan/splitter/sub_ticket.go` L167-175 | 非 transfer 且非跨年时 skip；跨年明细不 skip |
| 拆单路由 | `cmd/woa-server/logics/plan/dispatcher/sub_ticket.go` L598-611 | 按主单 `ticket.Type` switch；`budget_declare` 会 default 失败 → **统一映射 SplitAdjustTicket** |
| meta ticket_types | `cmd/woa-server/service/meta/meta.go` L266-278 | `ListTicketType` → `GetRPTicketTypeMembers()` + `Name()` |
| 列表筛选 | `cmd/woa-server/types/plan/ticket.go` L120-123 | `TicketTypes` 逐项调用 `RPTicketType.Validate()`，须改用主单类型校验 |
| 人工创建 | `cmd/woa-server/service/plan/ticket.go` L281-282 | 固定 `RPTicketTypeAdd`，无 type 选择器 |
| 列表筛选 | meta `ticket_types` + List Validate | 后端扩展后筛选参数可用；**本期不改 front** |
| 详情 type_name | 后端 `Name()` 映射 | 接口返回「预算申报」；前端模板本期不改 |
| API 文档 | `docs/api-docs/.../overwrite_append_biz_resource_plan_ticket.md` | 尚无 `type` 必填说明 |

---

## R1：`deriveOverwriteAppendTicketType` 自动推导

**Decision**：移除 overwrite_append 主流程中的自动推导，改为使用请求体必填 `type` 落库主单。`deriveOverwriteAppendTicketType` 不再用于主单落库，也不再用于拆单路由（见 R4）；可保留作单测辅助或后续删除死代码。

**Rationale**：
- spec FR-002 明确要求禁止省略、禁止自动推导。
- 现网 L91-92：`ticketType := deriveOverwriteAppendTicketType(...)` 须去掉。
- 拆单对 `budget_declare` 统一走 `SplitAdjustTicket`（产品确认，优先降低复杂度）。

**Alternatives considered**：
- **A. 请求 type + 拆单再按明细推导 Split***：语义最贴纯 add/delete，但路由分支多 → 不采纳（复杂度更高）。
- **B. 请求 type + budget_declare 一律 SplitAdjust（采纳）**：实现最简；纯追加/纯删除常规组子单多为 `adjust`（见 R4）。

---

## R2：枚举 `budget_declare` 校验分层

**Decision**：新增 `RPTicketTypeBudgetDeclare = "budget_declare"` 及中文名；新增 `ValidateRootTicketType()`（add/adjust/delete/budget_declare）；**不**将 `budget_declare` 加入通用 `Validate()`（子单类型校验）及 `GetPRSubTicketTypeMembers()`；`GetRPTicketTypeMembers()` 增加该值供 meta/筛选。

**Rationale**：
- FR-001/FR-006：仅主单、仅 overwrite_append 可创建；子单枚举不新增。
- 列表筛选 `ListResPlanTicketReq.Validate` 须接受 `budget_declare`，改用 `ValidateRootTicketType()`。
- overwrite_append 的 `type` 校验允许四值：`budget_declare|add|adjust|delete`。

**Alternatives considered**：
- **A. 加入通用 Validate()**：子单/CRP 路径可能误接受 → 不采纳。
- **B. 分层校验（采纳）**：主单与 overwrite_append 专用校验，创建入口 default 拒绝。

---

## R3：子单 HCM 管理员免审（AdminAudit skip）

**Decision**：在 `constructSubTicketCreateReq` 增加独立分支：当父主单 `ticket.Type == RPTicketTypeBudgetDeclare` 时，强制 `AdminAuditStatus = skip`，**忽略** `hasNonCurrentYear` 限制；不修改 CRP 相关逻辑。

**Rationale**：
- 现网 L167-175：跨年明细 `hasNonCurrentYear=true` 时不 skip。
- FR-004 / AC-T02 要求含跨年亦 skip。
- CRP 管理员节点在后续 stage 处理，本函数仅设 HCM admin_audit 初始状态；CRP 不跳过（AC-005）由现有 CRP 流程保证，无需改动 `crp_adjust.go`。

**Alternatives considered**：
- **A. 修改全局 skip 规则**：影响非 budget_declare 单据 → 不采纳。
- **B. 主单类型分支（采纳）**：范围最小、符合 spec。

---

## R4：拆单路由对 `budget_declare` 的处理

**Decision**（产品确认 2026-07-23）：在 `dispatcher/sub_ticket.go` 的 `createSubTicket` 与 `sub_ticket.go` 重试拆单处，当 `ticket.Type == budget_declare` 时**一律**调用 `SplitAdjustTicket`（与 `adjust` 主单相同路径）。请求显式 `type=add|adjust|delete` 仍走现有 switch。

**Rationale**：
- 现网 switch 不含 `budget_declare`，不扩展会 default 失败。
- `SplitAdjustTicket` 内部仍会按明细拆 cancel/add 并走 prepareDelete/prepareAdd，覆盖/追加行为可执行。
- 统一 adjust 路径可减少分支与测试矩阵；接受纯追加/纯删除时常规组 `sub_type` 经 `CanMerged()` 合并后多为 `adjust`（非 `add`/`delete`）。
- transfer / delay 等特殊组仍按 adjust 拆单现网逻辑产出。

**已知取舍**：
- 纯追加 / 纯删除：子单类型标签多为「调整」，与显式 `type=add/delete` 路径不一致；预算申报场景可接受。
- CRP 走 adjust 分支而非专用 add/delete 分支；与混合明细预算同步一致。

**Alternatives considered**：
- **A. 按明细推导 SplitAdd/Adjust/Delete**：子单类型更贴语义，复杂度更高 → 不采纳。
- **B. 一律 SplitAdjust（采纳）**：实现与认知成本最低。

---

## R5：创建入口限制（FR-006）

**Decision**：后端在 `logics/plan/types.go` 的 `CreateResPlanTicketReq.Validate` 对 `budget_declare` 走 default 返回 unsupported；overwrite_append 单独校验允许四值；人工创建 API 固定 add，无需新增前端类型选择器。

**Rationale**：
- 人工创建无 type 字段（service 层写死 add）。
- meta 列表需含 budget_declare 供筛选（FR-005），与「创建不可选」不冲突。
- 其他 API（adjust/cancel/overwrite 老接口）经 `RPTicketType.Validate()` 拒绝 budget_declare。

**Alternatives considered**：
- **A. 前端 meta 过滤 budget_declare**：创建表单本就不消费 ticket_types → 非必要。
- **B. 仅后端硬拒绝（采纳）**：本期纯后端，创建入口由 API Validate 保障。

---

## R6：前端露出策略 — 本期不做

**Decision**：**本期不修改 `front/`**。FR-005 仅交付后端 meta/列表/详情字段；前端 TS 联合类型、column 渲染等如有需要，划归后续需求。

**Rationale**：
- 产品确认本需求为纯后端。
- 列表筛选若已动态消费 meta，后端扩展后可能无需前端改动即可使用；不作为本期验收阻塞项。

**Alternatives considered**：
- **A. 同步改前端 TS + column（原方案）**：超出本期范围 → 不采纳。
- **B. 纯后端交付（采纳）**。

---

## R7：测试与文档

**Decision**：
- 单元测试：`ticket_overwrite_append_test.go` 增加 type 必填/枚举；`sub_ticket.go` 或独立 test 覆盖 budget_declare + 跨年 skip；enumor 校验测试。
- 集成测试：新增 `test/integration/resource-plan/`（或同级目录）覆盖 overwrite_append type 传参、拆单 admin skip；退回计划 overwrite_append 回归引用现有 `return-plan/overwrite_append_test.go`。
- API 文档：更新 `overwrite_append_biz_resource_plan_ticket.md`，标注破坏性变更。

**Rationale**：spec 测试策略与 AC-001~008、AC-T01/T02 对齐；finops 联调依赖文档。

**Alternatives considered**：仅单元测试 → 无法覆盖拆单+dispatcher 链路 → 不采纳。

---

## 架构合规预检（依赖方向）

| 检查项 | 结论 |
|--------|------|
| cloud/woa-server 不直连 DB | ✅ 沿用 data-service client |
| 枚举放 enumor | ✅ |
| Handler → Service → Logics → Client | ✅ 无新层 |
| 退回计划不改 | ✅ 不触碰 `return-plan/ticket_overwrite_append.go` |

---

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| finops 未同步传 type（TR-001） | 文档标注 breaking change；集成测试覆盖缺参 |
| budget_declare 拆单路由遗漏 | dispatcher/sub_ticket 双点修改 + 集成测试 |
| 列表筛选 Validate 未更新 | ListResPlanTicketReq 改用 ValidateRootTicketType |
