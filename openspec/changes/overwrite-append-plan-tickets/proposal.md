## Why

finops「上报预算」后需要把结构化的预算明细批量转成资源预测与退回计划，并在重复上报时按条件覆盖已有数据以保证最终一致。HCM 侧的资源预测（`res_plan_ticket`）与退回计划（`return_plan_ticket`，基础子需求 `return-plan-ticket-base` 已建设）单据体系均已就绪，但缺少面向系统对接的「覆盖追加」业务视角接口。本变更补齐这最后一环，让预算明细自动进入资源预测与 CRP 退回计划链路。

## What Changes

- 新增**资源预测覆盖追加接口**（业务视角）：`POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/overwrite_append`。支持覆盖开关 + 覆盖筛选（项目类型 + 技术分类 + 期望交付时间范围）、`skip_itsm` 跳过 ITSM 审批、请求体 `applicant` 提单人；覆盖命中的本地 `res_plan_demand` 转为 `cancel` 条目，追加明细作为 `add` 条目，合并为 `type=adjust` 的同一主单提单。
- 新增**退回计划覆盖追加接口**（业务视角）：`POST /api/v1/woa/bizs/{bk_biz_id}/plans/returns/tickets/overwrite_append`。经 `GetBizOrgRel` 完成业务→部门/规划产品/运营产品转换；覆盖开关开启时按 业务 + 项目类型 + 技术分类 + 计划退回时间范围经 CRP `queryReturnPlanItem` 查得待删除条目并构造 `cancel` 条目，追加明细构造 `add` 条目，合并为 `type=adjust` 主单提单；`returnReasonClass` 为空时填默认「成本优化&利用率提升」。
- 新增 woa-server 侧退回计划主单创建 controller（基础子需求仅在 data-service 层提供创建能力，woa-server 无创建入口）。
- 扩展 CRP `queryReturnPlanItem`（Go 方法 `QueryReturnPlan`）请求参数，新增 `technicalClass` 列表字段以支持退回计划覆盖按技术分类筛选。
- 资源预测提单人改造：仅新接口支持从请求体 `applicant` 覆盖 `kt.User`，现有创建/调整/取消接口保持不变。
- 覆盖能力（开关控制）与「仅覆盖不新增」（传覆盖开关且不传追加明细）模式；`overwrite=false` 且无追加明细视为参数非法。
- web-server 侧补充两个接口的代理路由。

不包含（明确排除）：退回计划单据体系与 CRP client 基础封装（见 `return-plan-ticket-base`）、finops 侧「预算 → 结构化明细」换算、退回计划的 update（调整）能力、接口路径/命名中出现 finops 字样。

## Capabilities

### New Capabilities

- `resource-plan-overwrite-append`: 资源预测覆盖追加接口（覆盖筛选转 `cancel` 条目 + 追加 `add` 条目合并 `adjust` 主单、`skip_itsm` 跳过审批、请求体 `applicant` 提单人、仅覆盖不新增）。
- `return-plan-overwrite-append`: 退回计划覆盖追加接口（`GetBizOrgRel` 业务转换、CRP 覆盖筛选删除、追加、默认退回原因大类、仅覆盖不新增、woa-server 主单创建入口、CRP `queryReturnPlanItem` 扩展 `technicalClass`）。

### Modified Capabilities

（无：`crp-return-plan-client`、`return-plan-ticket-*` 等能力仍在 `return-plan-ticket-base` 变更内、尚未归档为 `openspec/specs/` 正式 spec，对 `queryReturnPlanItem` 的 `technicalClass` 扩展作为 `return-plan-overwrite-append` 的实现细节体现于其 spec/design。）

## Impact

- 新增/扩展代码目录：
  - `cmd/woa-server/service/plan/`（资源预测覆盖追加 Handler + 路由）、`cmd/woa-server/service/return-plan/`（退回计划覆盖追加 Handler + 路由）
  - `cmd/woa-server/types/plan/`、`cmd/woa-server/types/return-plan/`（请求/响应结构体）
  - `cmd/woa-server/logics/plan/`（资源预测覆盖筛选→cancel 条目、skip_itsm 分支、applicant 透传）
  - `cmd/woa-server/logics/return-plan/`（新建主单创建 controller、GetBizOrgRel 接入、覆盖筛选、追加）
  - `pkg/thirdparty/cvmapi/`（`QueryReturnPlan` 请求参数扩展 `technicalClass`）
  - `cmd/web-server`（两个接口代理路由）
- 接口契约：`docs/api-docs/web-server/docs/biz/scr/resource-plan/overwrite_append_biz_resource_plan_ticket.md`、`docs/api-docs/web-server/docs/biz/scr/return-plan/overwrite_append_biz_return_plan_ticket.md`（版本 v9.9.9，已就绪）。
- 权限：资源预测接口 `biz_resource_plan_operate`（业务-资源预测操作）；退回计划接口业务-退回计划操作（沿用 base 现状，如 IAM 未定义则复用业务访问权限，详见 design）。
- 外部依赖：CRP（云梯）退回计划 JSON-RPC 接口、CMDB（`GetBizOrgRel` 业务组织关系）。
- 无存量数据/表结构变更：本变更为纯增量（新接口 + 复用现有表/枚举/调度），不新增表。
