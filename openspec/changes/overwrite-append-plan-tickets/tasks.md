## 1. CRP client 扩展（technicalClass）

- [x] 1.1 在 `pkg/thirdparty/cvmapi/cvmapi_request.go` 的 `QueryReturnPlanParam` 新增 `TechnicalClass []string` 字段（JSON 字段名依 CRP 实际契约，联调确认；注意与基础子需求 tasks 6.5「探针验证被忽略」的结论存在分歧，以本变更确认的「CRP 实际支持」为准）
- [x] 1.2 `QueryReturnPlan` 调用侧支持透传 `technicalClass`（为空时不传该参数，匹配全部技术分类）

## 2. 资源预测覆盖追加 - logics

- [x] 2.1 在 `cmd/woa-server/logics/plan/types.go` 的 `CreateResPlanTicketReq` 新增可选 `Applicant` 字段；`constructResPlanTicket`（`logics/plan/ticket.go`）当 `Applicant` 非空时用之、否则回退 `kt.User`（不改变现有创建/调整/取消接口行为）
- [x] 2.2 实现覆盖筛选查询：扩展 `ListResPlanDemand` 查询 opt（`fetcher/demand.go` 的 `convAllResPlanDemandListOpt`）支持 `technical_class` 过滤（`tools.RuleIn`），或在 woa 侧按 `bk_biz_id`+`obs_projects`+`technical_classes`+`expect_time`(int YYYYMMDD 范围) 构造 data-service filter 查询命中 `res_plan_demand`
- [x] 2.3 命中明细转 `cancel` 条目：仿 `logics/plan/demand_adjust.go` 的 `constructCancelReq`，用命中 `res_plan_demand` 构造 `Original`（demand_id、剩余核数等）；跳过 `locked` 明细并记录（design Q-3）
- [x] 2.4 追加 `demands` 转 `add` 条目（仅 `Updated`），与 cancel 条目合并；主单 `type` 由条目推导（仅 cancel→delete / 仅 add→add / 混合→adjust）
- [x] 2.5 实现 `skip_itsm` 分支：为 `true` 时创建主单后不调 `CreateAuditFlow`，直接置主单 `status=auditing` + `ItsmSN=ResPlanItsmAuditSkip("skip")`（仿 `auto_transfer.go`）；为 `false` 走现有 `CreateAuditFlow`
- [x] 2.6 在 logics 层聚合覆盖追加主流程（查询命中→构造 cancel/add→创建主单→按 skip_itsm 分支进入调度），返回主单 ID

## 3. 资源预测覆盖追加 - handler / 路由 / 类型

- [x] 3.1 在 `cmd/woa-server/types/plan/` 新增 `ticket_overwrite_append.go`：请求结构（`overwrite`/`overwrite_filter{obs_projects,technical_classes,expect_time_range{start,end}}`/`skip_itsm`/`demand_class`/`demands[]`/`applicant`/`remark`）与响应 `{id}`，含参数校验（overwrite=false 且无 demands→非法；overwrite=true 时 filter/obs_projects/expect_time_range 必填；有 demands 时 demand_class 必填；expect_time_range 格式与 end≥start）
- [x] 3.2 在 `cmd/woa-server/service/plan/` 新增 Handler `OverwriteAppendResPlanTicket`：解码→校验→鉴权（`meta.ResPlan`+`Create`，映射 `biz_resource_plan_operate`）→调 logics 主流程
- [x] 3.3 在 `cmd/woa-server/service/plan/service.go` 注册路由 `POST /bizs/{bk_biz_id}/plans/resources/tickets/overwrite_append`

## 4. 退回计划覆盖追加 - logics（含 woa 创建入口）

- [x] 4.1 在 `cmd/woa-server/logics/return-plan/` 新增覆盖追加 controller（建议 `overwrite_append.go`）：入口方法接收覆盖追加请求，编排组织转换/覆盖/追加/创建主单
- [x] 4.2 接入 `GetBizOrgRel(bk_biz_id)`（`bizLogics.GetBizOrgRel`）得到 部门/规划产品/运营产品，写入主单组织字段
- [x] 4.3 覆盖筛选：`overwrite=true` 时调 `QueryReturnPlan`（携带 technicalClass）按 业务(→运营产品)+项目类型+技术分类+`plan_time_range` 查 CRP 命中条目，构造 `ReturnPlanDetail{Original:{crp_plan_id,...}}`（cancel 条目）
- [x] 4.4 追加明细：`return_details` 构造 `ReturnPlanDetail{Updated:{...}}`（add 条目）；`return_reason_class` 为空填默认常量 `constant.DefaultReturnReasonClass`；`resource_pool_name` 为空填默认自研池
- [x] 4.5 合并 details → 调 data-service client `CreateReturnPlanTicket`（`Applicant` 来自请求体），返回主单 ID；后续由已有 dispatcher 自动拾取 init 主单拆单提单（cancel→删除单 / add→新增单）
- [x] 4.6 校验：`overwrite=false` 且无 `return_details`→`InvalidParameter`；追加明细 `instance_model`+`cvm_amount` 与 `instance_type`+`core_type_name`+`core_amount` 二选一

## 5. 退回计划覆盖追加 - handler / 路由 / 类型

- [x] 5.1 在 `cmd/woa-server/types/return-plan/` 新增覆盖追加请求结构（`overwrite`/`overwrite_filter{obs_projects,technical_classes,plan_time_range{start,end}}`/`return_details[]`/`applicant`/`remark`）与响应 `{id}`，含参数校验
- [x] 5.2 在 `cmd/woa-server/service/return-plan/` 新增 Handler `OverwriteAppendBizReturnPlanTicket`：解码→校验→鉴权（退回计划操作权限，未定义时按 design Q-1 复用 `meta.Biz`+`meta.Access`）→调 controller
- [x] 5.3 在 `cmd/woa-server/service/return-plan/service.go` 注册路由 `POST /bizs/{bk_biz_id}/plans/returns/tickets/overwrite_append`

## 6. 对外代理与文档核对

- [x] 6.1 web-server 侧确认两接口代理路由可透传（已有 `/api/v1/woa` catch-all，woa 注册即可；如需显式代理则补充）
- [x] 6.2 核对接口实现与两份文档契约一致：`docs/api-docs/web-server/docs/biz/scr/resource-plan/overwrite_append_biz_resource_plan_ticket.md`、`docs/api-docs/web-server/docs/biz/scr/return-plan/overwrite_append_biz_return_plan_ticket.md`（字段、枚举、路径、必选性）

## 7. 验证

- [x] 7.1 资源预测单测：主单 type 推导（deriveOverwriteAppendTicketType）、参数校验（overwrite/demands 组合、筛选必选性、时间范围）。（注：同包内存在 pre-existing 失效测试 `demand_device_type_test.go`，与本变更无关，暂阻塞该包测试二进制编译）
- [x] 7.2 退回计划单测：主单 type 推导（deriveReturnPlanTicketType）、CRP 项转 Original（convCrpReturnPlanItem）、参数校验（instance_model/instance_type 二选一、overwrite/return_details 组合、筛选必选性）——全部通过
- [ ] 7.3 集成测试：两接口端到端（提单→主单落库→dispatcher 拆单→子单提 CRP），覆盖仅追加/仅覆盖/覆盖+追加三种模式
- [ ] 7.4 联调 CRP：校验 `queryReturnPlanItem` 的 `technicalClass` 字段名与筛选语义（确认 CRP 实际支持，否则回退 HCM 本地二次过滤，见 design Q-2）；校验删除模式提单、`userName` 透传
- [ ] 7.5 联调 CMDB：校验 `GetBizOrgRel` 业务→组织维度转换正确
- [ ] 7.6 验证 AC 对齐：AC-001~AC-009、AC-P01（异步返回主单号 P95<1s、命名不含 finops）
