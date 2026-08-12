## Why

HCM 从未对接过 CRP（云梯）的「退回计划」能力，缺少单据体系、CRP client 封装与流转调度。为支撑 finops 预算同步链路（上层子需求），需要先在 HCM 侧建设一套退回计划单据管理的基础能力：本地仅落地单据与流转状态、不持久化退回计划明细（明细以 CRP 为准），并对接 CRP 完成提单与状态跟踪。本变更是退回计划整体能力的地基。

## What Changes

- 新增 CRP 退回计划 client 封装：在 `pkg/thirdparty/cvmapi` 扩展 `submitAppendOrder`（新增）、`submitAdjustOrderForApi`（调整&删除）、`queryOrderDetail`（单据状态）、`getReasonClassByObsProject`（退回原因大类），并扩展现有 `queryReturnPlanItem`。
- 新增退回计划单据数据模型与 data-service CRUD：主单 `return_plan_ticket`（含 `status`/`message`，不单独设状态表）、子单 `return_plan_sub_ticket`（与 CRP 单一一对应，落 `crp_sn`/`crp_url`）。**不新增退回计划明细本地表**。
- 新增退回计划拆单逻辑：按 部门 + 规划产品 + 项目类型 + 资源池 分组；新增（add）与删除（cancel）拆为不同子单，每子单对应一个 CRP 单据。
- 新增退回计划调度流转与状态机：新建 return-plan dispatcher，**无 ITSM 阶段**，主单创建后子单直接进入 CRP 提单 → 轮询 → 状态聚合；失败子单可单独重试。主单进入 `failed`/`partial_failed` 时，将各失败子单的失败原因聚合拼接进主单 `message`。
- 新增一组退回计划管理接口（woa-server 入口，经内部微服务调用 data-service）：列表、详情、子单列表、子单详情、重试、终止、退回原因大类，共 7 个。

不包含（划归上层子需求或明确排除）：面向 finops 的 `overwrite_append` 覆盖追加接口、退回计划明细本地持久化、ITSM 审批流程、退回计划的 update（本期仅新增 + 删除）、CRP 撤单 `cancelTodo`（本期不封装，终止仅改本地状态、不撤回 CRP）。

## Capabilities

### New Capabilities

- `crp-return-plan-client`: CRP 退回计划 JSON-RPC client 封装（新增/调整删除/单据状态查询/退回原因大类/明细查询扩展）。
- `return-plan-ticket-dataservice`: 退回计划主单/子单两张表的数据模型、DAO、data-service CRUD 原子接口与 client 封装（非 data-service 服务禁止直连 DB）。
- `return-plan-ticket-dispatch`: 退回计划拆单逻辑与调度流转（子单/主单状态机、CRP 提单、状态轮询、状态聚合、失败重试）。
- `return-plan-ticket-management`: 退回计划管理接口（列表/详情/子单列表/子单详情/重试/终止/退回原因大类）。

### Modified Capabilities

（无：本变更不改动现有能力的 spec 级行为，仅在 `cvmapi` 中新增方法。）

## Impact

- 新增表：`return_plan_ticket`、`return_plan_sub_ticket`（含 SQL 迁移脚本、id_generator 注册）。
- 新增枚举：`ReturnPlanTicketType`、`ReturnPlanTicketStatus`、`ReturnPlanSubTicketType`、`ReturnPlanSubTicketStatus`。
- 新增/扩展代码目录：
  - `pkg/thirdparty/cvmapi/`（CRP 退回计划方法）
  - `pkg/dal/table/return-plan/`、`pkg/dal/dao/return-plan/`、`pkg/dal/table/table.go`（表名常量）
  - `pkg/api/data-service/return-plan/`、`pkg/client/data-service/global/`
  - `cmd/data-service/service/return-plan/`
  - `cmd/woa-server/logics/return-plan/`（含 `splitter/`、`dispatcher/`）、`cmd/woa-server/service/plan/`（退回计划管理接口 handler/路由）
  - `cmd/cloud-server` 或 woa 层对外入口 + web-server 代理
- 外部依赖：CRP（云梯）退回计划 JSON-RPC 接口。
- 接口契约：`docs/api-docs/web-server/docs/biz/scr/return-plan/` 下已就绪的接口文档（版本 v9.9.9，上线前替换）。
