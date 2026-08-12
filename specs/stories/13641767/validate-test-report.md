# validate-test Report — Story 13641767

## Verdict
LGTM

## Checked artifacts
- `specs/stories/13641767/spec.md`
- `specs/stories/13641767/plan.md`
- `specs/stories/13641767/tasks.md`
- `specs/stories/13641767/context.md`
- `git diff b9745347621395a1346deebb6bbcaf413db14aac`（Code scope）
- 单元测试：
  - `pkg/criteria/enumor/woa_ziyan_resplan_test.go`
  - `cmd/woa-server/types/plan/ticket_overwrite_append_test.go`
  - `cmd/woa-server/types/plan/ticket_test.go`
  - `cmd/woa-server/logics/plan/splitter/sub_ticket_test.go`
  - `cmd/woa-server/logics/plan/types_test.go`
- 集成 stub：`test/integration/resource-plan/overwrite_append_budget_declare_test.go`

## Reference baselines
- `specs/stories/13641767/spec.md`（AC-001~008、AC-T01/T02、AC-P01、AC-S01）
- `specs/stories/13641767/tasks.md`（Phase 3 备注：integration skip stub；AC 由单元覆盖可接受）
- `.cursor/skills/qa-engineer/references/test-design.md`（金字塔 / BDD / 边界）
- `.cursor/skills/tapd-story-pipeline/references/report-template.md`

## Execution note
本机复跑单元测试（`GOMODCACHE=$HOME/go/pkg/mod`）均通过：
`enumor` / `types/plan` / `logics/plan/splitter` / `logics/plan`（CreateResPlanTicketReq）。

---

## Findings

### A1
- **类别**：Testability / Completeness
- **严重性**：MEDIUM
- **位置**：`test/integration/resource-plan/overwrite_append_budget_declare_test.go`
- **总结**：集成层仅为 `t.Skip` stub，AC-001~004 无真实端到端断言。
- **根因**：`code-self`（实现阶段已声明无 suite；tasks.md Phase 3 备注接受由单元覆盖）
- **修改建议**：后续补齐 resource-plan suite/fixture 后启用 T021–T024；**本期不因 stub 单独升 CRITICAL/HIGH**（与 tasks 声明一致）。

### A2
- **类别**：Testability
- **严重性**：MEDIUM
- **位置**：`cmd/woa-server/logics/plan/splitter/sub_ticket_test.go:L58-L79`（`rootTicketSplitKind`）
- **总结**：AC-003 拆单路由用例通过测试内镜像 switch 断言，未直接调用 `dispatcher.createSubTicket` / `retrySplitResPlanTickets`。
- **根因**：`code-self`
- **修改建议**：抽取可测路由函数，或对 createSubTicket 做 table-driven 薄封装单测，避免镜像与生产 switch 漂移。

### A3
- **类别**：Completeness
- **严重性**：MEDIUM
- **位置**：`cmd/woa-server/logics/plan/overwrite_append.go:L91-L103`（无对应 `*_test` 断言）
- **总结**：AC-001 / AC-T01「主单 type=req.Type」无单元直接断言 `persist` 入参；依赖 T006 实现与请求 Validate。
- **根因**：`code-self`（tasks 验收对照写明 AC-001=T006+单元/文档）
- **修改建议**：为 overwrite_append 增加可注入/可断言的轻量单测，或集成 suite 启用后覆盖。

### A4
- **类别**：Completeness
- **严重性**：LOW
- **位置**：`cmd/woa-server/logics/plan/splitter/sub_ticket_test.go:L34-L56`
- **总结**：admin skip 仅覆盖「budget_declare + 跨年 → skip」；缺「非 budget_declare + 跨年仍不 skip」回归对照，以及「budget_declare + 本年度 → skip」。
- **根因**：`code-self`
- **修改建议**：补充 1~2 个对照用例，锁定 FR-004 分支条件。

### A5
- **类别**：Completeness
- **严重性**：LOW
- **位置**：AC-005 / AC-008 / AC-P01 / AC-S01
- **总结**：CRP 不跳过、退回计划无回归、性能与鉴权依赖声明/代码审查，无自动化用例。
- **根因**：`code-self`（tasks 已标注沿用现网 / 路径未改）
- **修改建议**：发布前 finops 联调清单勾选；AC-008 可在有 suite 时跑 return-plan overwrite_append 回归。

### A6
- **类别**：Testability
- **严重性**：LOW
- **位置**：`cmd/woa-server/logics/plan/overwrite_append_test.go`（`deriveOverwriteAppendTicketType`）
- **总结**：主单 type 已改为必填 `req.Type`，旧推导函数单测仍保留（plan 允许留死代码）。
- **根因**：`code-self`
- **修改建议**：删除死代码与单测，或标注 Deprecated，避免误导「主单 type 仍自动推导」。

---

## 覆盖评估表

### 1. 测试金字塔

| 层级 | 现状 | 评估 |
|------|------|------|
| 单元 | 枚举 / Validate / List 筛选 / Create 拒绝 / admin skip / 路由镜像 | ✅ 主力覆盖，与本期纯后端范围匹配 |
| 集成 | skip stub（无 TestMain/suite） | ⚠️ 可接受（tasks 声明）；非 CRITICAL |
| E2E/手工 | 计划 finops 联调（AC-P01/S01） | ⚠️ 发布前执行，不阻塞本阶段 LGTM |

### 2. AC / BDD 完整性

| AC | 风险 | 覆盖方式 | 状态 |
|----|------|----------|------|
| AC-001 主单预算申报 | P0 | T006 实现 + Name/Validate；集成 stub | ⚠️ 单元间接；按 tasks 可接受 |
| AC-002 缺 type 参数错误 | P0 | `ticket_overwrite_append_test` type required/invalid | ✅ |
| AC-003 SplitAdjust / sub_type≠budget_declare | P0 | constructSubTicket SubType=adjust + 镜像路由 | ⚠️ 路由非生产函数 |
| AC-004 admin skip（含跨年） | P0 | `TestConstructSubTicketCreateReq_BudgetDeclareAdminSkip` | ✅ |
| AC-005 CRP 不自动 skip | P1 | 未改 CRP 路径（审查） | ⚠️ 声明覆盖 |
| AC-006 非 overwrite_append 拒绝 | P0 | `TestCreateResPlanTicketReq_RejectBudgetDeclare` | ✅ |
| AC-007 meta/列表筛选 | P0 | GetRPTicketTypeMembers + List Validate | ✅ |
| AC-008 退回计划无回归 | P1 | 路径未改（tasks） | ⚠️ 声明覆盖 |
| AC-T01 显式 type=add | P1 | Validate type add ok | ⚠️ 落库间接 |
| AC-T02 跨年仍 skip | P0 | 同 AC-004 单测 | ✅ |
| AC-P01 异步 P95 | P2 | 沿用现网 | ⚠️ 联调 |
| AC-S01 鉴权 | P0 | 沿用 `biz_resource_plan_operate` | ⚠️ 联调/现网 |

### 3. 边界 / 错误路径

| 场景 | 覆盖 |
|------|------|
| 缺 type / 非法 type（delay 等） | ✅ |
| 四值合法（add/adjust/delete/budget_declare） | ✅ |
| 子单 Validate 不含 budget_declare | ✅ |
| 列表拒绝仅子单类型（delay） | ✅ |
| Create 入口 unsupported | ✅ |
| overwrite=false 且无 demands 等既有负向 | ✅（回归保留） |
| 非 budget_declare 跨年不 skip（对照） | ❌（LOW） |
| 真实 API/DB 端到端 | ❌ stub（MEDIUM，已声明） |

### 4. 核心路径结论

| 核心路径 | 结论 |
|----------|------|
| 必填 type → 参数校验 | 单元充分 |
| budget_declare → HCM admin skip（含跨年） | 单元充分 |
| budget_declare → SplitAdjust 路由 | 生产代码已改；测试为镜像，残余漂移风险 MEDIUM |
| 创建入口独占 overwrite_append | 单元充分 |
| meta/列表露出 | 单元充分 |

---

## 准入说明

- **无 CRITICAL / HIGH** → Verdict = **LGTM**
- MEDIUM 项均为「声明可接受的集成缺口 / 测试可维护性」，不强制本轮 needs_fix
- 建议在合并后或下一迭代：启用 integration suite、消除路由镜像、补 admin-skip 对照用例
