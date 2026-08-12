## Context

HCM 资源预测（`res_plan_ticket`）已具备完备的单据体系（主单/子单/状态/明细）、拆单（`splitter`）、调度（`dispatcher`）、ITSM 与 CRP 对接能力，分层为：`woa-server/service/plan`（Handler）→ `woa-server/logics/plan`（业务/拆单/调度）→ `data-service`（CRUD）→ `pkg/dal`（DAO/表），CRP 对接位于 `pkg/thirdparty/cvmapi`。退回计划（`return_plan_ticket`/`return_plan_sub_ticket`）由基础子需求 `return-plan-ticket-base` 建设，已就绪表/DAO/data-service CRUD/CRP client/拆单/调度/7 个管理接口，但 **woa-server 侧无主单创建入口**（现仅 data-service 层 `CreateReturnPlanTicket`）。

本变更新增两个面向系统对接（finops）的业务视角「覆盖追加」接口，接口契约以两份已就绪文档为准：
- `docs/api-docs/web-server/docs/biz/scr/resource-plan/overwrite_append_biz_resource_plan_ticket.md`
- `docs/api-docs/web-server/docs/biz/scr/return-plan/overwrite_append_biz_return_plan_ticket.md`

约束（沿用项目规范）：非 data-service 服务禁止直连 DB；所有 client 调用经 `pkg/client/common/request.go` 封装；枚举放 `pkg/criteria/enumor/`、常量放 `pkg/criteria/constant/`，字符串/数字禁止写死；接口路径/命名不含 finops。

## Goals / Non-Goals

**Goals:**

- 新增资源预测覆盖追加 Handler + 路由 + 请求/响应类型，复用 `res_plan_ticket` 全套链路；实现覆盖筛选（→ `cancel` 条目）、追加（→ `add` 条目）合并 `type=adjust` 主单、`skip_itsm`、请求体 `applicant`。
- 新增退回计划覆盖追加 Handler + 路由 + 请求/响应类型；新建 woa-server 退回计划主单创建 controller；实现 `GetBizOrgRel` 转换、CRP 覆盖筛选删除、追加、默认退回原因大类、请求体 `applicant`。
- 扩展 CRP `queryReturnPlanItem`（`QueryReturnPlan`）请求参数新增 `technicalClass` 列表。
- web-server 侧补充两个接口代理路由。

**Non-Goals:**

- 退回计划单据体系与 CRP client 基础封装（`return-plan-ticket-base`）。
- finops 侧「预算 → 结构化明细」换算。
- 退回计划 update（调整）能力（本期覆盖=删除、追加=新增）。
- 现有资源预测创建/调整/取消接口的行为改动（`applicant` 改造仅作用于新接口）。

## Decisions

### D-1：资源预测覆盖追加分层与入口

在 `cmd/woa-server/service/plan/` 新增 Handler `OverwriteAppendResPlanTicket`，路由 `POST .../plans/resources/tickets/overwrite_append` 注册于 `service/plan/service.go`。请求/响应类型置于 `cmd/woa-server/types/plan/`（新增 `ticket_overwrite_append.go`）。Handler 流程遵循标准签名 `func(cts *rest.Contexts) (interface{}, error)`：解码 → 校验 → 鉴权（`meta.ResPlan` + `Create`/`Update`）→ 调 logics controller，返回 `{"id": ticketID}`。

**替代方案**：直接改造现有 `CreateBizResPlanTicket` 增加覆盖参数。否决：覆盖追加语义（cancel+add 混合、skip_itsm、applicant）与标准创建差异大，独立入口更清晰、不污染现有链路。

### D-2：资源预测覆盖 = 命中明细转 cancel 条目（不物理删本地行）

`overwrite=true` 时，按 `bk_biz_id` + `obs_projects` + `technical_classes`（空则全部）+ `expect_time_range`（按 `expect_time`，DB 为 int `YYYYMMDD`）查询本地 `res_plan_demand`，将命中明细构造为 `cancel` 条目（`Original` 填 demand_id 与剩余核数等，仿 `logics/plan/demand_adjust.go` 的 `constructCancelReq`）。追加 `demands` 构造为 `add` 条目（仅 `Updated`）。二者合并进同一主单，主单 `type` 由条目推导：仅 cancel→`delete`、仅 add→`add`、混合→`adjust`。随后复用现有 `Controller.CreateResPlanTicket` + 拆单 + 调度链路。

**技术分类筛选**：现有 `ListResPlanDemand` 的 list filter 不支持 `technical_class`（`convAllResPlanDemandListOpt` 无该字段规则）。方案：扩展该查询 opt 增加 `technical_class` 过滤规则（`res_plan_demand.technical_class` 为 DB 字段，用 `tools.RuleIn`），或在 woa 侧构造 data-service 通用 filter 查询。时间范围用 `tools.RuleGreaterThanEqual`/`RuleLessThanEqual` 对 int `YYYYMMDD`。

**边界**：命中明细若处于 `locked`（提单流转中）状态，跳过并在响应/日志提示（避免覆盖流转中的明细）。仅覆盖不新增 → 无 add 条目、主单 `type=delete`；`overwrite=false` 且无 `demands` → `InvalidParameter`。

**替代方案**：物理 `DeleteResPlanDemand`（data-service 已有）。已由用户否决：需走 CRP 删除审批保证与云侧一致，物理删除会丢失审批与 CRP 同步语义。

### D-3：资源预测 skip_itsm 实现

复用现有跳过 ITSM 机制：常量 `ResPlanItsmAuditSkip = "skip"`（`pkg/criteria/constant/ziyan.go`）。`skip_itsm=true` 时，创建主单后不调用 `CreateAuditFlow`，而是直接置主单 `status=auditing` + `ItsmSN=skip`（仿 `auto_transfer.go` L278-287），dispatcher 识别 `skip` 直接进入拆单（`dispatcher/ticket.go` 现有分支）。`skip_itsm=false` 走现有 `CreateAuditFlow`。

### D-4：applicant 透传（仅新接口）

新接口请求体 `applicant` 必填。资源预测 logics 层 `CreateResPlanTicketReq` 增加可选 `Applicant` 字段；`constructResPlanTicket` 当 `Applicant` 非空时用之、否则回退 `kt.User`（保持现有接口行为不变）。退回计划 data-service `ReturnPlanTicketCreateReq` 已支持 `Applicant`，woa controller 直接透传。二者的 `applicant` 均贯穿后续子单 `Creator` 与 CRP `userName`（现有链路已如此传递）。

### D-5：退回计划覆盖追加分层与 woa-server 创建入口

在 `cmd/woa-server/service/return-plan/` 新增 Handler `OverwriteAppendBizReturnPlanTicket`，路由 `POST .../plans/returns/tickets/overwrite_append` 注册于 `service/return-plan/service.go`。请求/响应类型置于 `cmd/woa-server/types/return-plan/`。

在 `cmd/woa-server/logics/return-plan/` 新增主单创建 controller 方法（如 `CreateReturnPlanTicket`，建议 `overwrite_append.go`）：
1. `GetBizOrgRel(bk_biz_id)`（`bizLogics.GetBizOrgRel`）→ 部门/规划产品/运营产品。
2. `overwrite=true`：调 `QueryReturnPlan`（扩展 technicalClass）按 业务(→运营产品) + 项目类型 + 技术分类 + `plan_time_range` 查 CRP 命中条目 → 构造 `ReturnPlanDetail{Original:{crp_plan_id,...}}`（cancel 条目）。
3. `return_details` → 构造 `ReturnPlanDetail{Updated:{...}}`（add 条目），`return_reason_class` 为空填默认常量。
4. 合并 details → 调 data-service client `CreateReturnPlanTicket`（`Applicant` 来自请求）→ 返回主单 ID。
5. 后续由已有 return-plan dispatcher watcher 自动拾取 init 主单 → 拆单（`splitter.Split`，cancel/add 天然分不同子单）→ 子单提 CRP（cancel→`SubmitAdjustReturnOrderForApi` 删除模式；add→`SubmitAppendReturnOrder`）。

### D-6：CRP queryReturnPlanItem 扩展 technicalClass

在 `pkg/thirdparty/cvmapi/cvmapi_request.go` 的 `QueryReturnPlanParam` 新增 `TechnicalClass []string`（JSON 依 CRP 实际字段名，联调确认）。CRP 文档未列该参数但实际支持（已与用户确认）。资源预测侧不经 CRP 查询、直接用本地 `res_plan_demand.technical_class` 过滤，故此扩展仅服务退回计划覆盖筛选。

### D-7：默认退回原因大类与枚举/常量复用

默认退回原因大类「成本优化&利用率提升」复用 `return-plan-ticket-base` 已定义常量 `constant.DefaultReturnReasonClass`（`pkg/criteria/constant/return_plan.go`）。资源池 `resource_pool_name`、项目类型 `ObsProject`、退回计划条目类型推导（`ReturnPlanDetail` original/updated）均复用 base 已有定义，不新增枚举。

### D-8：权限与命名

- 资源预测接口鉴权 `meta.ResPlan` + `Create`（映射 `biz_resource_plan_operate`，`cmd/auth-server/service/auth/gen_id.go` 已支持）。
- 退回计划接口鉴权沿用 base 现状（现管理接口用 `meta.Biz` + `meta.Access`；文档标注「业务-退回计划操作」，若 IAM 未定义该 Action 则本期复用业务访问权限，作为 Open Question 待确认）。
- 两接口路径、字段命名均不含 finops。

## Risks / Trade-offs

- [资源预测 list 查询不支持 technical_class 过滤] → 扩展 `ListResPlanDemand` 查询 opt 或用 data-service 通用 filter；需保证与现有 list 行为兼容。
- [覆盖命中 locked 明细] → 跳过 locked 明细并提示，避免覆盖流转中的预测。
- [skip_itsm 允许外部跳过审批] → 属需求明确要求（AC-003），复用现有 skip 机制；需在文档/权限层面确保仅授信系统调用。
- [CRP `queryReturnPlanItem` 的 `technicalClass` 字段名/语义依赖联调] → 以 CRP 实际接口为准，联调阶段校验；HCM 侧透传，不做本地二次过滤（除非联调发现 CRP 不支持则回退客户端过滤）。
- [退回计划 `submitAppendOrder` 可能按资源池返回多个订单号] → base 已将资源池纳入拆单维度，一子单恒对应一 CRP 单；本接口沿用。
- [applicant 覆盖 kt.User] → 仅新接口生效，现有接口不受影响；CRP 以真实提单人身份提单。

## Migration Plan

1. cvmapi 扩展 `QueryReturnPlanParam.TechnicalClass`（可先与 CRP 联调对齐字段）。
2. 资源预测：logics 增加 `Applicant`、覆盖筛选查询与 cancel 条目构造、skip_itsm 分支；新增 Handler + 路由 + 类型。
3. 退回计划：新增 woa controller（GetBizOrgRel + 覆盖筛选 + 追加）+ Handler + 路由 + 类型。
4. web-server 补充两接口代理路由。
5. 联调 CRP（technicalClass 查询、删除模式提单）与 CMDB（GetBizOrgRel）。
6. 回滚：纯增量（新接口 + 复用现有表/枚举/调度），下线两个新接口即可，不影响资源预测与退回计划存量链路。

## Open Questions

- Q-1：退回计划接口权限点「业务-退回计划操作」在 IAM 中是否已定义对应 Action？未定义时本期复用业务访问权限（`meta.Biz`+`meta.Access`），还是新增 IAM Action？
- Q-2：CRP `queryReturnPlanItem` 的 `technicalClass` 精确字段名与取值口径（是否与 HCM `technical_class` 完全一致），以联调为准。
- Q-3：资源预测覆盖命中 `locked` 明细的处理策略（跳过并提示 vs 直接报错终止），倾向跳过并提示，待确认。
- Q-4（Q-E 沿用）：退回计划 `plan_time` 早于 now+35 天由 CRP 校验并原样返回错误，HCM 侧仅透传。
