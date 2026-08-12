## 1. 枚举与常量

- [x] 1.1 在 `pkg/criteria/enumor/` 新增退回计划枚举（建议 `return_plan.go`）：`ReturnPlanTicketType`(add/adjust/cancel，子单类型复用此枚举)、`ReturnPlanTicketStatus`(init/auditing/rejected/partial_rejected/revoked/done/failed/partial_failed/terminated)、`ReturnPlanSubTicketStatus`(init/auditing/rejected/revoked/invalid/done/failed/terminated)，各附 `Validate()`/`Name()`(map)/`GetXxxMembers()`，状态附 `IsUnfinished()`、主单附 `IsNonFinalState()`（rejected/partial_rejected/failed/partial_failed 源自 CRP 审批流）
- [x] 1.2 在 `pkg/criteria/constant/` 新增默认退回原因大类常量「成本优化&利用率提升」及其它退回计划相关常量（资源池默认值等，复用已有则不重复定义）

## 2. 数据模型与表

- [x] 2.1 在 `pkg/dal/table/table.go` 新增表名常量 `ReturnPlanTicketTable`、`ReturnPlanSubTicketTable`
- [x] 2.2 在 `pkg/dal/table/return-plan/` 新增主单 `ReturnPlanTicketTable` struct（字段见 design D-1，含 `details` JSON、`status`/`message`、组织维度、审计字段）
- [x] 2.3 在 `pkg/dal/table/return-plan/` 新增子单 `ReturnPlanSubTicketTable` struct（含 `ticket_id`、`sub_type`、`sub_details` JSON、拆单维度、`crp_sn`/`crp_url`、`status`/`message`、审计字段）及 `details`/`sub_details` 的 JSON 元素类型定义
- [x] 2.4 在 `scripts/sql/` 新增 `9999_*` 建表脚本：两张表 DDL + `id_generator` 注册 + `hcm_version` 视图更新（格式参照 `0049_..._res_plan_sub_ticket.sql`，版本号上线前替换）

## 3. DAO 层

- [x] 3.1 在 `pkg/dal/dao/return-plan/` 新增主单 DAO（`CreateWithTx`/`UpdateWithTx`/`List`/`DeleteWithTx`）与类型定义（list result）
- [x] 3.2 在 `pkg/dal/dao/return-plan/` 新增子单 DAO（`CreateWithTx` 本身即批量语义/`UpdateWithTx` 返回 affected rows/`List`/`DeleteWithTx`）
- [x] 3.3 在 `pkg/dal/dao/dao.go` 的 dao Set 接口与工厂中注册 `ReturnPlanTicket()`/`ReturnPlanSubTicket()`

## 4. data-service proto 与 service

- [x] 4.1 在 `pkg/api/data-service/return-plan/` 新增主单/子单的请求/响应结构（主单 Create 单条/Update/List/Delete；子单 BatchCreate/BatchUpdate/List/Delete/StatusCAS，CAS 扩展支持写 `crp_sn`/`crp_url`）
- [x] 4.2 在 `cmd/data-service/service/return-plan/` 新增 service 聚合入口与主单 CRUD handler + 路由注册
- [x] 4.3 在 `cmd/data-service/service/return-plan/` 新增子单 CRUD handler（批量创建、更新、列表、状态更新 CAS）+ 路由注册
- [x] 4.4 将 return-plan service 注册进 data-service 的服务初始化流程（`cmd/data-service/service/service.go`）

## 5. data-service client 封装

- [x] 5.1 在 `pkg/client/data-service/global/` 新增 `ReturnPlanClient`，封装主单/子单 CRUD（经 `pkg/client/common/request.go`）
- [x] 5.2 将 `ReturnPlanClient` 挂载到 data-service client set（`pkg/client/data-service/global/client.go`）

## 6. CRP client 封装

- [x] 6.1 在 `pkg/thirdparty/cvmapi/constvar.go` 新增退回计划 method 名常量与工厂函数（`submitAppendOrder`/`submitAdjustOrderForApi`/`queryOrderDetail`/`getReasonClassByObsProject`）
- [x] 6.2 在 `cvmapi_request.go` 新增各方法请求 struct（含 `userName`；删除模式 `src=[{id}]`、`update=[]`）
- [x] 6.3 在 `cvmapi_response.go` 新增各方法响应 struct（`submitAppendOrder` result 为按资源池的单号列表、`queryOrderDetail` 含 status/statusMsg/details/adjust、`getReasonClassByObsProject` 含原因大类列表）
- [x] 6.4 在 `cvmapi.go` 的 `CVMClientInterface` 与实现中新增方法 `SubmitAppendReturnOrder`/`SubmitAdjustReturnOrderForApi`/`QueryReturnOrderDetail`/`GetReasonClassByObsProject`，thin 模式 CRP error 原样透传（`cancelTodo` 本期不封装）
- [x] 6.5 评估结论：本期无需扩展 `queryReturnPlanItem` 参数（`technicalClass` 经探针验证被 CRP 忽略、iwiki 契约未列出，需要时在 HCM 侧本地二次过滤；现有 `QueryReturnPlanParam` 字段已满足拆单/查询）

## 7. 拆单逻辑

- [x] 7.1 在 `cmd/woa-server/logics/return-plan/splitter/` 实现拆单：明细由 original/updated 构成，按 `ReturnPlanDetail.Type()`(仅 updated→add/仅 original→cancel/兼有→adjust)推导条目类型后，按 (类型+项目类型+资源池) 分组（部门/规划产品来自主单头恒定），add/cancel/adjust 天然分属不同子单，分组结果排序保证稳定
- [x] 7.2 拆单结果落 `return_plan_sub_ticket`（经 data-service client `BatchCreateReturnPlanSubTicket`，以提单人身份创建），每分组=一个子单=一个 CRP 单据
- [x] 7.3 主单 `type` 推导并回填：`deriveTicketType` 按各明细类型汇总推导（全 add→add / 全 cancel→cancel / 全 adjust→adjust / 混合→adjust）并经 client 回填主单；子单 `sub_type` 与分组类型一致（add/cancel/adjust）（附表驱动单测）

## 8. 调度流转与状态机

- [x] 8.1 在 `cmd/woa-server/logics/return-plan/dispatcher/` 搭建调度器骨架（主单队列 + 子单队列、仅 master 执行、watcher + handler goroutine），复用 res_plan dispatcher 模型
- [x] 8.2 主单处理：拉取 pending 主单(init/auditing) → init 触发拆单并置 auditing → 子单经 client 落库入队
- [x] 8.3 子单提单：add → `SubmitAppendReturnOrder`，cancel/adjust → `SubmitAdjustReturnOrderForApi`；成功 CAS init→auditing 并写 `crp_sn`/`crp_url`，失败 CAS init→failed 写原因
- [x] 8.4 子单状态轮询：`QueryReturnOrderDetail(orderId=crp_sn)` 推进子单 → done/rejected/failed（拆单已按资源池分组，一个子单恒对应一个 CRP 单号，提单时校验、多于一个按异常处理）；状态码映射带 TODO 待真实 CRP 校准
- [x] 8.5 主单状态聚合：全部 done→done、部分 failed→partial_failed、全部 failed→failed、全部/部分驳回→rejected/partial_rejected
- [x] 8.6 失败原因聚合：主单进入非 done 终态时，收集未成功子单 `message` 按 `[子单ID/资源池] 原因` 逐行拼接写入主单 `message`（varchar(2048) 超长截断）
- [x] 8.7 子单 auditing 超时：watcher 仅追踪最近 `PendingTicketTraceDay`(42天) 提单子单入队；子单自 `submitted_at` 起超过 `AuditFlowTimeoutDay`(28天) 仍在 auditing → 置 failed 并写超时 message，参与主单聚合（复用现有常量，超时语义映射为 CRP 提单/轮询未达终态）
- [x] 8.8 将退回计划调度器接入 woa-server 启动流程（`cmd/woa-server/service/service.go`）

## 9. 管理接口（woa-server）

- [x] 9.1 在 `cmd/woa-server/logics/return-plan/` 新增管理业务逻辑 controller（列表/详情/子单列表/子单详情/重试/终止/退回原因大类）
- [x] 9.2 重试逻辑：仅对失败态子单重新发起 CRP 提单流转，成功子单不受影响
- [x] 9.3 终止逻辑：将失败态单据置终止态、不可再重试，仅改本地状态、不撤回 CRP
- [x] 9.4 退回原因大类：proxy CRP `getReasonClassByObsProject`
- [x] 9.5 在 `cmd/woa-server/service/plan/` 新增退回计划 handler（建议 `return_ticket.go`）：解码/校验/鉴权 → 调 controller
- [x] 9.6 注册业务视角路由 `/bizs/{bk_biz_id}/plans/returns/...`（list/{id}/sub_tickets list/{id}/retry/terminate/reason_classes list），权限按 design D-7 表配置

## 10. 对外代理与文档

- [x] 10.1 在 web-server 补充退回计划管理接口的代理路由，转发至 woa-server（已有 `/api/v1/woa` catch-all 透传，woa-server 注册路由即可）
- [x] 10.2 核对接口实现与 `docs/api-docs/web-server/docs/biz/scr/return-plan/` 下已就绪文档契约一致（字段、枚举、路径）

## 11. 验证

- [x] 11.1 补充 DAO/拆单/状态聚合/失败原因聚合的单元测试，以及后台调度器集成测试（DAO：集成 CRUD/CAS 用例；拆单：`splitter/splitter_test.go`；状态聚合/失败原因聚合：`dispatcher/aggregate_test.go`；调度器集成：`test/integration/return-plan/dispatcher_test.go` 拆单/提单/轮询/聚合/非 master 跳过；管理接口集成：`management_test.go` 重试/终止/详情/原因大类）
- [ ] 11.2 联调 CRP 提单/删除/轮询/原因大类，校验 `userName` 透传与错误原样返回
- [ ] 11.3 端到端验证管理接口（列表/详情/子单/重试/终止/原因大类）与状态机流转
- [x] 11.4 收尾清理：全功能完成后，清理代码中所有关于「后续阶段」/「管理接口见后续阶段」等阶段性占位注释（`return_plan.go` 注释已清理）
