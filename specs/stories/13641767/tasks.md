# Tasks — Story 13641767

> 基于 plan.md / research.md / data-model.md  
> 模式：TDD · **纯后端**（本期不含前端变更）  
> 格式：`- [ ] TNNN [P?] [USn] 描述（含文件路径）`

**图例**：`[P]` = 可与同阶段其他 `[P]` 任务并行（不同文件、无未完成依赖）

---

## Phase 1：Walking Skeleton — 枚举基础（US-1 前置）

**目标**：主单类型 `budget_declare` 可被识别与校验，为 overwrite_append 与 meta 提供基础。

- [x] T001 [US1] 编写 `ValidateRootTicketType` 失败/成功表驱动测试 in `pkg/criteria/enumor/woa_ziyan_resplan_test.go`（覆盖 add/adjust/delete/budget_declare 与非法值）
- [x] T002 [US1] 实现 `RPTicketTypeBudgetDeclare`、中文名映射、`GetRPTicketTypeMembers()` 扩展、`ValidateRootTicketType()` in `pkg/criteria/enumor/woa_ziyan_resplan.go`；确认 `Validate()` 与 `GetPRSubTicketTypeMembers()` **不含** budget_declare
- [x] T003 [US1] 运行 `go test ./pkg/criteria/enumor/... -run ValidateRootTicketType` 确认 T001 通过

**独立测试标准**：单元测试 green；`budget_declare.Name()` 返回「预算申报」。

---

## Phase 2：US-1 — finops 预算同步（后端核心）

**用户故事**：finops 通过 overwrite_append 必填 `type=budget_declare` 创建预算申报主单，子单 HCM 管理员 skip。

### 2.1 overwrite_append 必填 type（FR-002）

- [x] T004 [US1] 在 `cmd/woa-server/types/plan/ticket_overwrite_append_test.go` 增加测试：缺 `type`、非法 `type`、四值合法、`type=budget_declare`
- [x] T005 [US1] `OverwriteAppendResPlanTicketReq` 增加 `Type` 字段（`json:"type" validate:"required"`）及 `Validate()` 调用 `ValidateRootTicketType()` in `cmd/woa-server/types/plan/ticket_overwrite_append.go`
- [x] T006 [US1] 修改 `OverwriteAppendResPlanTicket`：用 `req.Type` 落库主单，移除 L91-92 `deriveOverwriteAppendTicketType` 自动推导 in `cmd/woa-server/logics/plan/overwrite_append.go`（函数可留单测或后续删）
- [x] T007 [US1] 运行 `go test ./cmd/woa-server/types/plan/... -run OverwriteAppendResPlanTicketReq` 确认 T004 通过

**独立测试标准**：AC-002 单元层覆盖；显式 `type=add` 时主单类型为 add（AC-T01 逻辑层前置）。

### 2.2 拆单路由（FR-003）

- [x] T008 [US1] 编写/扩展测试：`ticket.Type=budget_declare` 时路由到 `SplitAdjustTicket`（纯追加/纯删除/混合均可；不要求再分 SplitAdd/Delete）
- [x] T009 [US1] `createSubTicket` 增加 `RPTicketTypeBudgetDeclare` 分支：直接调用 `SplitAdjustTicket` in `cmd/woa-server/logics/plan/dispatcher/sub_ticket.go`
- [x] T010 [US1] 重试拆单 switch 同步：`budget_declare` → `SplitAdjustTicket` in `cmd/woa-server/logics/plan/sub_ticket.go`
- [x] T011 [US1] 运行相关包单元测试确认 T008 通过

**独立测试标准**：AC-003 — 主单 budget_declare 拆单走 adjust 路径；子单 `sub_type` 不为 `budget_declare`；覆盖/追加行为由明细处理（常规组多为 adjust）。

### 2.3 HCM 管理员免审（FR-004）

- [x] T012 [US1] 编写测试：父单 `budget_declare` + 含非本年度期望时间明细 → `AdminAuditStatus=skip` in `cmd/woa-server/logics/plan/splitter/sub_ticket_test.go`（新建）
- [x] T013 [US1] `constructSubTicketCreateReq` 增加 `ticket.Type == RPTicketTypeBudgetDeclare` 强制 skip 分支（忽略 `hasNonCurrentYear`）in `cmd/woa-server/logics/plan/splitter/sub_ticket.go`
- [x] T014 [US1] 运行 `go test ./cmd/woa-server/logics/plan/splitter/...` 确认 T012 通过

**独立测试标准**：AC-004、AC-T02 单元覆盖；CRP 路径无改动（AC-005 靠代码审查 + 集成测试声明不修改 crp_adjust）。

### 2.4 创建入口限制（FR-006）

- [x] T015 [US1] 编写测试：`CreateResPlanTicketReq` 传入 `budget_declare` 返回 unsupported in `cmd/woa-server/logics/plan/types_test.go`（新建或扩展）
- [x] T016 [US1] `CreateResPlanTicketReq.Validate` 对 `RPTicketTypeBudgetDeclare` 显式拒绝 in `cmd/woa-server/logics/plan/types.go`
- [x] T017 [US1] 运行 `go test ./cmd/woa-server/logics/plan/... -run CreateResPlanTicketReq` 确认 T015 通过

**独立测试标准**：AC-006 — 非 overwrite_append 创建 API 拒绝 `budget_declare`。

### 2.5 列表/meta（FR-005 后端）

- [x] T018 [P] [US1] 编写测试：列表请求 `ticket_types: ["budget_declare"]` Validate 通过 in `cmd/woa-server/types/plan/ticket_test.go`（新建或扩展）
- [x] T019 [P] [US1] `ListResPlanTicketReq.Validate` 中 `TicketTypes` 改用 `ValidateRootTicketType()` in `cmd/woa-server/types/plan/ticket.go`
- [x] T020 [US1] 运行列表 Validate 单元测试；断言 `GetRPTicketTypeMembers()` 含 budget_declare（meta 间接验证）

**独立测试标准**：AC-007 后端 — meta 含类型；列表筛选参数合法。

---

## Phase 3：集成测试

**依赖**：Phase 2 完成。

- [x] T021 [US1] 新建 `test/integration/resource-plan/overwrite_append_budget_declare_test.go`（或同级目录）：AC-001 创建 budget_declare 主单并断言 type/type_name
- [x] T022 [US1] 同文件：AC-002 缺 type 返回参数错误
- [x] T023 [US1] 同文件：AC-003 拆子单走 adjust 路径、sub_type 非 budget_declare
- [x] T024 [US1] 同文件或子单断言：AC-004 admin_audit_status=skip（含跨年 fixture）
- [x] T025 [US1] 回归：执行 `test/integration/return-plan/overwrite_append_test.go` 确认 AC-008 无回归
- [x] T026 [US1] 运行 `go test ./test/integration/resource-plan/...`（或项目约定命令）确认 T021–T025 通过

**独立测试标准**：集成测试 green；AC-P01 无额外同步阻塞（仅校验+异步返回 id）。

> 备注：`test/integration/resource-plan/` 当前为 skip stub（无 suite/fixture）；AC-001~004 由单元测试覆盖。退回计划 overwrite_append 为独立 API，本期未改其代码路径（AC-008）。

---

## Phase 4：API 文档（FR-002 交付）

**依赖**：T005–T006 接口行为定稿。

- [x] T027 [US1] 更新 `docs/api-docs/web-server/docs/biz/scr/resource-plan/overwrite_append_biz_resource_plan_ticket.md`：增加必填 `type` 参数表、示例 JSON、破坏性变更说明（v9.9.9）
- [x] T028 [US1] 文档自检：type 取值与 spec/plan 一致（budget_declare|add|adjust|delete）；权限与路径不变

**独立测试标准**：文档与 plan.md §3.1 契约一致。

---

## Phase 5：收尾与验收对照

- [x] T029 [US1] 对照 spec.md 验收场景 AC-001~AC-008、AC-T01/T02、AC-S01 逐项勾选（测试报告或 tasks 备注）
- [x] T030 [US1] 运行 `go test` 受影响包全量回归
- [x] T031 [US1] 更新 `specs/stories/13641767/process.log` 记录 implement 阶段就绪（可选，implement 阶段执行）

**验收对照**：
| AC | 覆盖 |
|----|------|
| AC-001 | T006 + 单元/文档（集成 stub skip） |
| AC-002 | T004–T007 |
| AC-003 | T008–T011 |
| AC-004 / AC-T02 | T012–T014 |
| AC-005 | 未改 CRP 路径 |
| AC-006 | T015–T017 |
| AC-007 | T018–T020 |
| AC-008 | 退回计划路径未改 |
| AC-T01 | T004–T007 |
| AC-P01 / AC-S01 | 沿用现网异步+鉴权 |

---

## 依赖关系与执行顺序

```
Phase 1 (T001→T003)
    ↓
Phase 2 后端 (T004→T020)
    ↓
Phase 3 集成 (T021→T026)
    ↓
Phase 4 文档 (T027→T028, 可与 Phase 3 部分并行)
    ↓
Phase 5 收尾 (T029→T031)
```

**关键路径**：T001→T005→T006→T009→T013→T021→T027

**并行机会**：T018/T019 与 T015/T016；T027 与 T021–T026

**本期明确不包含**：`front/` 下任何 TS/Vue 变更（含 ticket_type 联合类型、column 渲染）。

---

## 需求覆盖矩阵

| 验收项 | 任务 |
|--------|------|
| AC-001 | T006, T021 |
| AC-002 | T004–T007, T022 |
| AC-003 | T008–T011, T023 |
| AC-004 | T012–T014, T024 |
| AC-005 | T013（不改 CRP）+ 审查 |
| AC-006 | T015–T017 |
| AC-007 | T018–T020 |
| AC-008 | T025 |
| AC-T01 | T004–T007, T021 |
| AC-T02 | T012–T014, T024 |
| AC-P01 | T026（响应时延观察） |
| AC-S01 | 沿用现有鉴权 + T021 无权限用例（可选补充） |
