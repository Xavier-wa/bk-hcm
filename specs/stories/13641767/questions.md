# Clarification Questions — Story 13641767

## Q1 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：资源预测主单新增类型的英文枚举值与中文展示名？
**影响**：影响枚举扩展与前后端类型展示；阻塞。
**建议候选**：
- A. `budget_declare` / 「预算申报」（req.md 澄清记录已确认）
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：主单类型枚举值为 `budget_declare`，展示名「预算申报」；仅作用于资源预测主单（`res_plan_ticket.type`），子单类型枚举不新增该值。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（规则 R-001、澄清记录第 1 轮）

## Q2 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：overwrite_append 的 `type` 是否必填？允许哪些取值？是否仍自动推导主单类型？
**影响**：影响接口契约与 finops 联调；阻塞。
**建议候选**：
- A. 必填；允许 `budget_declare` | `add` | `adjust` | `delete`；禁止省略与按明细自动推导（req.md Q-001 已确认）
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：`type` 为 overwrite_append 请求体必填字段；允许显式传 `budget_declare` / `add` / `adjust` / `delete`；系统不得默认赋值，也不得再调用 `deriveOverwriteAppendTicketType` 推导主单类型。现网实现（`cmd/woa-server/logics/plan/overwrite_append.go` L91-92）为自动推导，本期改为使用请求值。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（F-002、规则 R-003、澄清记录第 2 轮）；docs/api-docs/web-server/docs/biz/scr/resource-plan/overwrite_append_biz_resource_plan_ticket.md

## Q3 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：`budget_declare` 是否仅允许通过 overwrite_append 创建？
**影响**：影响创建入口校验范围；阻塞。
**建议候选**：
- A. 是，仅 overwrite_append 可传；其他创建入口（含前端人工提单）拒绝
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：`budget_declare` 仅允许通过资源预测 overwrite_append 传入；前端人工新建/调整预测单不可选择或提交该类型；其他复用创建逻辑的 API 同样拒绝。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（F-006、规则 R-004）

## Q4 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：主单为 `budget_declare` 时，子单类型与实际覆盖/追加/删除行为如何决定？
**影响**：影响拆单逻辑与详情展示；阻塞。
**建议候选**：
- A. 主单类型固定为请求值；子单类型仍按明细推导；行为由明细决定
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：主单 `type`/`type_name` 落库为请求指定的 `budget_declare`/「预算申报」；子单 `sub_type` 仍按既有拆单规则由明细推导（add/adjust/delete 等）；覆盖、追加、删除行为与现网 overwrite_append 一致，由 cancel/add 明细组合决定。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（F-003、规则 R-005）；cmd/woa-server/logics/plan/overwrite_append.go

## Q5 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：主单为 `budget_declare` 时，HCM 管理员审批跳过范围是否含跨年明细？CRP 管理员是否跳过？
**影响**：影响子单审批流转；阻塞。
**建议候选**：
- A. HCM 管理员一律 skip（含跨年）；CRP 管理员节点不跳过
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：当主单 `type=budget_declare` 时，子单 HCM 管理员审批状态一律置为 `skip`（含跨年明细，不受 `hasNonCurrentYear` 限制）；CRP 侧管理员审批节点保持现有逻辑，不因本类型自动跳过。现网 `sub_ticket.go` L167-175 对跨年与非 delete 类型有 skip 规则，本期需增加主单 `budget_declare` 的独立分支并覆盖跨年限制。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（F-004、规则 R-006、澄清记录第 1 轮）；cmd/woa-server/logics/plan/splitter/sub_ticket.go

## Q6 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：前端需在哪些位置露出「预算申报」？人工提单是否可选？
**影响**：影响前端 meta/列表/详情与创建表单；阻塞。
**建议候选**：
- A. meta ticket_types、列表筛选、详情「需求类型」露出；人工提单不可选
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：meta `ticket_types`（`ListTicketType` → `GetRPTicketTypeMembers`）需包含「预算申报」；主单列表按类型筛选可选；详情「需求类型」展示「预算申报」（`basic/index.vue` 的 `type_name`）；子单列表仍展示子单自身类型。前端人工新建预测单类型选择器不包含 `budget_declare`。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（F-005、F-006）；cmd/woa-server/service/meta/meta.go；front/src/store/ticket/resource-plan.ts；front/src/components/resource-plan/applications/detail/basic/index.vue

## Q7 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：退回计划 overwrite_append 是否纳入本期改造？
**影响**：影响范围边界；非阻塞。
**建议候选**：
- A. 本期不改，行为与上线前一致
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：退回计划（`return_plan`）overwrite_append 及类型枚举本期不改造；无预算申报类型、无 HCM 管理员免审变更。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（边界范围、AC-008）；cmd/woa-server/logics/return-plan/overwrite_append.go

## Q8 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：overwrite_append 新增必填 `type` 对已上线调用方是否兼容？
**影响**：影响联调与发布策略；阻塞。
**建议候选**：
- A. 破坏性变更，调用方必须同步传参
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：overwrite_append 新增必填 `type` 为破坏性变更；未传 `type` 返回参数错误；finops 预算同步场景须传 `type=budget_declare`；其他调用方可显式传 `add`/`adjust`/`delete` 以保持与旧推导结果一致的行为语义。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（兼容性、AC-002）；cmd/woa-server/types/plan/ticket_overwrite_append.go（当前无 type 字段）

## Q9 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：权限与 ITSM 跳过开关是否变更？
**影响**：影响安全与审批链路；非阻塞。
**建议候选**：
- A. 沿用 `biz_resource_plan_operate`；`skip_itsm` 语义不变
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：调用 overwrite_append 仍须 `biz_resource_plan_operate` 权限；`skip_itsm` 仅控制 ITSM 审批是否跳过，与 HCM 管理员免审逻辑独立，语义不变。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（权限规则、F-004 边界条件、AC-S01）

## Q10 [resolved_by_doc] — 来源：subagent(speckit.specify)
**问题**：性能与非功能目标是否有增量要求？
**影响**：影响 NFR 章节；非阻塞。
**建议候选**：
- A. 沿用父需求：overwrite_append 异步返回主单 ID，P95 < 1s；无新增并发/容量要求
**提出方**：subagent(speckit.specify) / attempt=1 / round=1 / ts=2026-07-23T20:10:00+08:00
**答复**：不因新增 `type` 校验引入同步长耗时；P95 返回主单号目标沿用父需求（< 1s）；无新增并发、容量、可用性要求。
**答复方**：subagent(自答) / ts=2026-07-23T20:10:00+08:00
**文档来源**：specs/stories/13641767/req.md（非功能需求、AC-P01）
