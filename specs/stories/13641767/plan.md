# 开发计划 — Story 13641767

> 需求 ID：1069995598136417767  
> 技术类型：backend（纯后端；本期不含前端变更）  
> 日期：2026-07-23  
> 方法论：TDD / Vertical Slice

## 1. 技术上下文

| 项 | 说明 |
|----|------|
| 技术栈 | Go（woa-server 微服务） |
| 受影响模块 | enumor、overwrite_append 请求/逻辑、拆单 splitter、dispatcher、meta、列表校验、API 文档 |
| 架构约束 | woa-server 经 data-service 访问 DB；无新依赖；退回计划不改 |
| 破坏性变更 | overwrite_append 新增必填 `type`，调用方须同步 |

### 1.1 项目结构（变更文件）

```
pkg/criteria/enumor/woa_ziyan_resplan.go          # 枚举 + ValidateRootTicketType
cmd/woa-server/types/plan/ticket_overwrite_append.go
cmd/woa-server/types/plan/ticket_overwrite_append_test.go
cmd/woa-server/types/plan/ticket.go               # List 筛选校验
cmd/woa-server/logics/plan/overwrite_append.go      # 使用 req.Type
cmd/woa-server/logics/plan/splitter/sub_ticket.go   # budget_declare admin skip
cmd/woa-server/logics/plan/dispatcher/sub_ticket.go # budget_declare 拆单路由
cmd/woa-server/logics/plan/sub_ticket.go            # 重试拆单路由
cmd/woa-server/logics/plan/types.go                 # Create 拒绝 budget_declare
docs/api-docs/web-server/docs/biz/scr/resource-plan/overwrite_append_biz_resource_plan_ticket.md
test/integration/resource-plan/                     # 新增（建议）
# 明确不改：front/** 
```

## 2. 需求映射

| Spec | 实现要点 |
|------|---------|
| FR-001 | `RPTicketTypeBudgetDeclare` + `Name()`「预算申报」+ `GetRPTicketTypeMembers()` |
| FR-002 | `OverwriteAppendResPlanTicketReq.Type` 必填；Validate 四值；移除自动推导落库 |
| FR-003 | 主单用请求 type；`budget_declare` 拆单一律 `SplitAdjustTicket` |
| FR-004 | `constructSubTicketCreateReq` 对 budget_declare 父单强制 admin skip（含跨年） |
| FR-005 | meta/list/detail 后端露出；List 校验改用 `ValidateRootTicketType`；不改 front |
| FR-006 | CreateResPlanTicketReq.Validate 拒绝；overwrite_append 独占 |
| FR-007 | 不修改 return-plan 代码路径 |
| AC-001~008, AC-T01/T02, AC-P01, AC-S01 | 见 §6 测试计划 |

## 3. 接口契约（权威）

### 3.1 overwrite_append（变更）

**路径**：`POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/overwrite_append`

**权限**：`biz_resource_plan_operate`（沿用）

**请求体增量**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | **是** | `budget_declare` \| `add` \| `adjust` \| `delete` |

**行为**：
- 未传或非法 `type` → `InvalidParameter`，不创建主单
- `type=budget_declare` → 主单 `type`/`type_name`=预算申报；拆单统一走 `SplitAdjustTicket`（内部仍按 cancel/add 明细处理覆盖/追加）
- 显式 `add`/`adjust`/`delete` → 主单类型与展示名对应该值，拆单走现有对应 Split*
- 其余字段与父需求一致；`skip_itsm` 语义不变

**响应**（不变）：

```json
{ "id": "<ticket_id>" }
```

**破坏性说明**：已上线调用方必须增加 `type`；finops 预算同步传 `budget_declare`；其他方可按明细语义显式传 add/adjust/delete。

### 3.2 meta ticket_types（间接变更）

**路径**：`POST /api/v1/woa/metas/ticket_types/list`

**响应增量**：`details` 数组增加 `{ "ticket_type": "budget_declare", "ticket_type_name": "预算申报" }`

### 3.3 响应字段（后端）

| JSON 字段 | 说明 |
|-----------|------|
| `type` / `ticket_type` | 可为 `budget_declare` |
| `type_name` / `ticket_type_name` | 「预算申报」 |

列表筛选：`ticket_types: ['budget_declare']`（POST body，沿用现网）。前端映射本期不改。

## 4. 实现方案

### 4.1 后端 — 枚举（FR-001）

1. 在 `woa_ziyan_resplan.go` 增加：
   - `RPTicketTypeBudgetDeclare RPTicketType = "budget_declare"`
   - `rdTicketTypeNameMap` 增加「预算申报」
   - `GetRPTicketTypeMembers()` 追加该常量
   - 新增 `ValidateRootTicketType()`：add/adjust/delete/budget_declare
2. **不**修改 `Validate()`（子单类型集合不变）
3. **不**修改 `GetPRSubTicketTypeMembers()`

### 4.2 后端 — overwrite_append 必填 type（FR-002）

1. `OverwriteAppendResPlanTicketReq` 增加 `Type enumor.RPTicketType \`json:"type" validate:"required"\``
2. `Validate()` 中调用 `r.Type.ValidateRootTicketType()`（或专用 `ValidateOverwriteAppendType()`）
3. `OverwriteAppendResPlanTicket`：
   - 删除 L91-92 推导落库逻辑
   - `createReq.TicketType = req.Type`
4. `deriveOverwriteAppendTicketType` 不再参与主单落库与拆单路由（可留单测或后续删死代码）

### 4.3 后端 — 拆单与免审（FR-003/FR-004）

1. **dispatcher/sub_ticket.go** `createSubTicket`：
   - `budget_declare` → **一律** `SplitAdjustTicket`（与 adjust 相同入参）
2. **sub_ticket.go** 重试拆单：同上
3. **splitter/sub_ticket.go** `constructSubTicketCreateReq`：
   ```go
   if ticket.Type == enumor.RPTicketTypeBudgetDeclare {
       subTicket.AdminAuditStatus = enumor.RPAdminAuditStatusSkip
   } else {
       // 现有 hasNonCurrentYear 逻辑
   }
   ```
4. CRP 路径不改动（AC-005）

### 4.4 后端 — 创建入口限制（FR-006）

1. `CreateResPlanTicketReq.Validate` switch 对 `budget_declare` 显式返回 unsupported（或 default）
2. 确认 adjust/cancel/overwrite（非 append）等路径无法传入 budget_declare

### 4.5 后端 — 列表与 meta（FR-005）

1. `ListResPlanTicketReq.Validate`：`TicketTypes` 改用 `ValidateRootTicketType()`
2. meta `ListTicketType` 随 `GetRPTicketTypeMembers` 自动生效

### 4.6 前端 — 本期不做

本期明确不包含 `front/` 变更。列表/详情若已消费 meta/`type_name`，可能无需前端改动即可部分可见；TS 联合类型与列渲染适配如有需要，另开需求。

### 4.7 API 文档

更新 `overwrite_append_biz_resource_plan_ticket.md`：
- 输入参数表增加 `type`（必填）
- 调用示例增加 `type` 字段
- 增加「破坏性变更」说明段落

## 5. 数据模型

见 `data-model.md`（枚举扩展，无表结构迁移）。

## 6. 测试计划（TDD）

| 层级 | 覆盖 |
|------|------|
| 单元 | `OverwriteAppendResPlanTicketReq.Validate`：缺 type、非法 type、四值合法 |
| 单元 | `ValidateRootTicketType` / enum Name |
| 单元 | `constructSubTicketCreateReq`：budget_declare + 跨年 → skip |
| 单元 | `deriveOverwriteAppendTicketType` 复用（已有逻辑，回归） |
| 集成 | overwrite_append + type=budget_declare → 主单类型正确 |
| 集成 | overwrite_append 缺 type → 参数错误 |
| 集成 | budget_declare 混合明细拆子单 sub_type 非 budget_declare |
| 集成 | 退回计划 overwrite_append 无回归（AC-008） |
| 手工/E2E | finops 预算同步 + skip_itsm 全链路（发布前联调） |

## 7. 实施顺序（Walking Skeleton）

1. 枚举 + ValidateRootTicketType + 单元测试（骨架）
2. overwrite_append type 必填 + 落库 + 单元测试（US-1 核心）
3. 拆单路由 + admin skip + 单元/集成测试（US-1 审批）
4. List 校验 + meta 验证
5. API 文档
6. 集成测试补齐 AC

## 8. 架构合规结论

- 依赖方向：Handler → Service → Logics → Client(data-service) ✅
- 无跨层 DB 访问 ✅
- 无新第三方依赖 ✅
- 安全：沿用 `biz_resource_plan_operate` ✅

## 9. 发布与联调

1. 与 finops 同步发布窗口，调用方增加 `type=budget_declare`
2. 通知其他 overwrite_append 调用方显式传 add/adjust/delete
3. 验证 meta/列表/详情接口字段（前端 UI 适配不在本期）
