## Context

HCM 现有「资源预测」(`res_plan`) 已具备完备的单据体系（主单/子单/状态/明细）、拆单（splitter）、调度（dispatcher）、ITSM 与 CRP 对接能力，代码分层为：`woa-server/service`（Handler）→ `woa-server/logics/plan`（业务/拆单/调度）→ `data-service`（CRUD）→ `pkg/dal`（DAO/表）；CRP 对接位于 `pkg/thirdparty/cvmapi`。

退回计划是一条全新链路，此前从未对接。本变更平行复用资源预测的分层与调度模型，但做以下简化：**不落退回计划明细本地表**（明细以 CRP 为准）、**无 ITSM 阶段**、**不单独设状态表**（`status`/`message` 合并进主单）。CRP 退回计划的 Go 侧方法此前仅有 `queryReturnPlanItem`（封装为 `QueryReturnPlan`），其余为 Python 探针（`scripts/crp_return_plan/`），需在 Go 侧新增。

约束：
- 对外接口契约以 `docs/api-docs/web-server/docs/biz/scr/return-plan/` 下已就绪文档为准（版本 v9.9.9）。
- 非 data-service 服务禁止直连 DB。
- 所有 client 调用经 `pkg/client/common/request.go` 封装。
- 枚举放 `pkg/criteria/enumor/`，常量放 `pkg/criteria/constant/`，字符串/数字禁止写死。

## Goals / Non-Goals

**Goals:**

- 新增 CRP 退回计划 client 方法（`submitAppendOrder`/`submitAdjustOrderForApi`/`queryOrderDetail`/`getReasonClassByObsProject`/`cancelTodo`，扩展 `queryReturnPlanItem`）。
- 新增退回计划主单/子单两张表 + DAO + data-service CRUD + client 封装 + SQL 迁移脚本 + 枚举。
- 新增拆单逻辑（部门+规划产品+项目类型+资源池 分组；add/cancel 分单）。
- 新增退回计划调度器（无 ITSM，子单提单→轮询→状态聚合；失败原因聚合进主单 message；失败子单单独重试）。
- 新增 7 个管理接口（列表/详情/子单列表/子单详情/重试/终止/退回原因大类）。

**Non-Goals:**

- 面向 finops 的 `overwrite_append` 覆盖追加接口（上层子需求）。
- 退回计划明细本地持久化。
- ITSM 审批流程。
- 退回计划 update（本期仅新增 + 删除）。
- 业务→组织映射（`GetBizOrgRel`）的接入（覆盖追加接口才需要，属上层）。

## Decisions

### D-1：数据模型采用「主单 + 子单」两表，不设状态表、不落明细表

参照 `res_plan_ticket`/`res_plan_sub_ticket`，但去掉 `res_plan_ticket_status` 与 `res_plan_demand`。

**主单 `return_plan_ticket`**（表名常量加入 `pkg/dal/table/table.go`，struct 于 `pkg/dal/table/return-plan/`）：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| type | string | add/adjust/cancel，由 `details` 构成推导（见下方 type 推导规则） |
| details | json | 退回计划条目列表，每条由 `original`(原始退回计划)与 `updated`(变更后退回计划)构成，条目类型按两部分有无推导（仅 updated→add / 仅 original→cancel / 兼有→adjust）；供详情展示与拆单，非可靠数据源 |
| applicant | string | 提单人 |
| bk_biz_id / bk_biz_name | int64/string | 业务 |
| op_product_id/name、plan_product_id/name、virtual_dept_id/name | | 组织维度 |
| status | string | init/auditing/rejected/partial_rejected/revoked/done/failed/partial_failed/terminated |
| message | string | 失败原因（聚合各失败子单原因） |
| remark | string | 说明 |
| submitted_at | string | 提单时间 |
| creator/reviser/created_at/updated_at | | 审计字段 |

**子单 `return_plan_sub_ticket`**（与 CRP 单一一对应）：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 主键 |
| ticket_id | string | 父单 ID |
| sub_type | string | add（CRP 新增单）/cancel（CRP 删除单）/adjust（CRP 调整单），由分组内明细的 original/updated 有无推导 |
| sub_details | json | 该子单对应的退回计划条目 |
| bk_biz_id、op/plan/virtual 组织维度 | | 拆单维度锚点 |
| obs_project | string | 项目类型（拆单维度） |
| resource_pool_name | string | 资源池（拆单维度） |
| status | string | init/auditing/rejected/revoked/invalid/done/failed/terminated |
| crp_sn / crp_url | string | CRP 单号/链接 |
| message | string | 失败原因 |
| submitted_at、审计字段 | | |

**明细条目 `type` 推导规则**：每条 `details` 由 `original`(原始退回计划，来自 CRP `queryReturnPlanItem`，含 `crp_plan_id`)与 `updated`(变更后退回计划)两部分构成，仿资源预测 `ResPlanDemand` 的 `Original`/`Updated`。单条条目类型按两部分有无推导——仅 `updated` → `add`(新增)；仅 `original` → `cancel`(取消/删除)；兼有 → `adjust`(调整)。

**主单 `type` 推导规则**：主单 `type` 不由前端直接指定，而是由 `details` 中各条目类型汇总推导——全为同一类型时取该类型（全 add→`add`/全 cancel→`cancel`/全 adjust→`adjust`）；多种类型混合（含 adjust，或 add 与 cancel 并存）→ `adjust`。子单 `sub_type` 与其分组内明细类型一致（纯 `add`/`cancel`/`adjust`，拆单时按类型+项目类型+资源池分组，天然分属不同子单）。推导在创建主单 / 拆单阶段完成并回填主单。

**详情接口数据源**：主单详情、子单详情接口返回本地单据快照 `details`/`sub_details`，**不实时 proxy CRP**。接口无外部依赖，满足 P95 < 1s；本地终止而 CRP 仍在执行时存在状态背离，属产品可接受（文档已声明）。

**替代方案**：完全照搬 res_plan 四表结构。否决理由：退回计划不落明细、不走 ITSM、无独立状态流转阶段，四表会引入无用复杂度。

### D-2：枚举定义

在 `pkg/criteria/enumor/` 新增（建议文件 `return_plan.go`）：

- `ReturnPlanTicketType`：`add`/`adjust`/`cancel`（子单类型复用本枚举，恒为纯 `add`/`cancel`，不单独定义子单类型枚举）
- `ReturnPlanTicketStatus`：`init`/`auditing`/`rejected`/`partial_rejected`/`revoked`/`done`/`failed`/`partial_failed`/`terminated`
- `ReturnPlanSubTicketStatus`：`init`/`auditing`/`rejected`/`revoked`/`invalid`/`done`/`failed`/`terminated`

其中 `rejected`/`partial_rejected`/`revoked`/`invalid` 源自 CRP 审批流（CRP 单据存在审批环节，可能被驳回、撤销或失效）。每个枚举提供 `Validate()`、`Name()`（map 查表）与 `GetXxxMembers()`；主单/子单状态另提供 `IsUnfinished()`，主单提供 `IsNonFinalState()`（`rejected`/`partial_rejected`/`failed`/`partial_failed` 可终止/重试）。资源池 `resource_pool_name`（自研池/公有池，DB 列名 `res_pool_name`）与项目类型 `ObsProject` 复用现有枚举/常量；默认退回原因大类「成本优化&利用率提升」定义为常量。

### D-3：CRP client 封装映射

在 `pkg/thirdparty/cvmapi` 扩展方法（method 名常量入 `constvar.go`，请求/响应 struct 分别入 `cvmapi_request.go`/`cvmapi_response.go`），映射自 `scripts/crp_return_plan/` 探针：

| Go 方法 | CRP method | URL | 用途 |
|---------|-----------|-----|------|
| `SubmitAppendReturnOrder` | `submitAppendOrder` | CVM_URL | 新增退回计划单 |
| `SubmitAdjustReturnOrderForApi` | `submitAdjustOrderForApi` | CVM_URL | 删除模式 `src=[{id}]`、`update=[]` |
| `QueryReturnOrderDetail` | `queryOrderDetail` | CVM_URL | 单据状态轮询 |
| `GetReasonClassByObsProject` | `getReasonClassByObsProject` | ORDER_URL | 退回原因大类 |
| `QueryReturnPlan`（已有） | `queryReturnPlanItem` | CVM_URL | 明细查询（按需扩展参数） |

`cancelTodo`（撤单）本期不封装：终止仅改 HCM 本地状态、不撤回 CRP，故无需该方法；后续覆盖追加子需求如需撤单再补。

统一 `userName` 由上层传入。错误按现有 cvmapi 模式原样透传（不改写 CRP error）。方法名/method 常量禁止写死。

### D-4：拆单逻辑

新建 `cmd/woa-server/logics/return-plan/splitter/`。输入主单 `details`（每条含 original/updated），先按明细 original/updated 有无推导条目类型（add/cancel/adjust），再按 `类型 + 项目类型 + 资源池` 分组（部门/规划产品来自主单头，单主单内恒定），不同类型天然分属不同子单（满足 CRP「新增/删除/调整不同单」约束），每子单落 `return_plan_sub_ticket`，同时回填主单 `type`。相比 res_plan splitter 无「可转移预测/延期」等复杂分支，实现更简单。

### D-5：调度流转与状态机

新建 `cmd/woa-server/logics/return-plan/dispatcher/`，复用 res_plan dispatcher 的「主单队列 + 子单队列、仅 master 执行、watcher + handler goroutine」模型。**无 HCM 侧 ITSM 阶段**（不接入蓝鲸 ITSM），但 CRP 单据自身存在审批环节，故 `auditing` 轮询可能终结为成功/驳回/撤销/失效等多种终态：

```
[*] --> init: 创建主单/子单
init --> auditing: 调度器调 CRP 提单(写 crp_sn)
auditing --> done: queryOrderDetail 轮询=成功
auditing --> rejected: CRP 审批驳回
auditing --> revoked: CRP 审批撤销
auditing --> invalid: CRP 单据失效
auditing --> failed: CRP 报错/轮询失败
auditing --> failed: 超过 AuditFlowTimeoutDay(28天) 仍未达终态 → 超时
failed --> auditing: retry
rejected --> auditing: retry
failed --> terminated: terminate
rejected --> terminated: terminate
done --> [*]
```

- 子单提单：`add` → `SubmitAppendReturnOrder`(取明细 updated)；`cancel` → `SubmitAdjustReturnOrderForApi` 删除模式(`src=[{original.crp_plan_id}]`、`update=[]`)；`adjust` → `SubmitAdjustReturnOrderForApi` 调整模式(`src=[{original.crp_plan_id}]` 与 `update=[{变更后}]` 一一对应)；成功写 `crp_sn`/`crp_url`，状态 → auditing。
- 状态轮询：`QueryReturnOrderDetail(orderId=crp_sn)` 推进子单 → done / rejected / revoked / invalid / failed。
- 主单聚合：全部 done → done；含失败/驳回子单且非全部失败/驳回 → partial_failed / partial_rejected；全部 failed → failed；全部 rejected → rejected。

**轮询周期与超时（沿用 res_plan，无 ITSM 语义映射为 CRP 流转）**：复用 res_plan dispatcher 的 watcher + handler 周期模型（watcher 20s/jitter 0.5 入队，handler 子单 2s、主单 5s，各 10 worker，仅 master 执行；当前实现为硬编码带 `// TODO: get interval from config`）。时间边界复用现有常量，不新造：

| 常量 | 值 | 作用（退回计划语义） |
|------|----|----------------------|
| `PendingTicketTraceDay` | 42 天 | watcher 只捞最近 42 天提单、`auditing` 态子单入队追踪 |
| `AuditFlowTimeoutDay` | 28 天 | 子单自 `submitted_at` 起超过 28 天仍在 `auditing`（CRP 提单/轮询未达终态）→ 置 `failed`、`message` 记录超时，参与主单聚合 |

超时语义由 res_plan 的「ITSM 审批流超时」平移为「CRP 提单/`queryOrderDetail` 轮询长时间未达终态」。

### D-6：失败原因聚合进主单 message

主单聚合状态时，若结果为 `failed`/`partial_failed`/`rejected`/`partial_rejected` 等含未成功子单的状态，收集所有未成功（`failed`/`rejected`/`revoked`/`invalid`）子单的 `message`，按固定格式拼接（建议 `[子单ID/资源池] 原因` 逐条换行拼接）后写入主单 `message`。聚合发生在调度器主单状态聚合环节，随主单状态一并更新。字段长度受表定义约束，超长时截断并保留可读性（参考 res_plan message 扩容脚本 `0059`）。

### D-7：管理接口分层与路由

复用 `cmd/woa-server/service/plan/` 入口，新增退回计划 handler（建议 `return_ticket.go`），业务视角前缀 `/bizs/{bk_biz_id}/plans/returns/...`：

| 接口 | Method | Path | 权限 |
|------|--------|------|------|
| 列表 | POST | `.../tickets/list` | 业务访问 |
| 详情 | GET | `.../tickets/{id}` | 业务访问 |
| 子单列表 | POST | `.../sub_tickets/list` | 业务访问 |
| 子单详情 | GET | `.../sub_tickets/{id}` | 业务访问 |
| 重试 | POST | `.../tickets/{ticket_id}/retry` | 业务-退回计划操作 |
| 终止 | POST | `.../tickets/{ticket_id}/terminate` | 业务-退回计划操作 |
| 退回原因大类 | POST | `.../reason_classes/list` | 业务访问 |

Handler 统一签名 `func(cts *rest.Contexts) (interface{}, error)`：解码 → 校验 → 鉴权 → 调 `logics/return-plan` controller。业务逻辑经 data-service client 读写单据，reason_classes 经 cvmapi proxy。web-server 侧补充对应代理路由。

### D-8：SQL 迁移脚本

在 `scripts/sql/` 新增 `9999_*` 建表脚本（上线前重命名版本号），含两张表 DDL、`id_generator` 注册（`return_plan_ticket`/`return_plan_sub_ticket`）、`hcm_version` 视图更新。格式参照 `0049_..._res_plan_sub_ticket.sql`。

## Risks / Trade-offs

- [明细以 CRP 为准，本地 `details` JSON 仅供展示/拆单] → 详情接口的明细查询语义需明确以 CRP proxy 为准；主单 `details` 不作为可靠数据源，避免与 CRP 不一致时误用。
- [子单与 CRP 一一对应，但 `submitAppendOrder` 早期设计可能返回多个订单号（自研池/公有池各一）] → 已将资源池纳入拆单分组维度（含新增类型），保证一个子单恒对应一个 CRP 单据；`submitAppendOrder` 校验返回单号数量，多于一个视为异常并返回错误（子单置失败），避免单子单绑定多个 order。
- [终止仅改本地状态、不撤回 CRP] → 与需求一致（AC/文档已声明），但存在「本地终止、CRP 仍在执行」的状态背离，需在文档与 UI 明确提示。
- [失败原因拼接超长] → message 字段长度限制，采用截断策略，必要时参考 res_plan 扩容 message 列。
- [新增 CRP 方法契约依赖 Python 探针推断] → 提单/删除/轮询字段以探针 + CRP 文档双重校验，联调阶段以实际返回为准。

## Migration Plan

1. 新增枚举、表 struct、DAO 并注册到 dao set。
2. 执行/合入 SQL 迁移脚本（含 id_generator）；灰度环境先建表。
3. data-service 注册退回计划 CRUD service + client。
4. cvmapi 新增退回计划方法（可先与探针对齐联调）。
5. 拆单 + 调度器接入（调度器仅 master 执行，随 woa-server 启动）。
6. 管理接口 + web-server 代理路由上线。
7. 回滚：本变更为纯增量（新表 + 新接口 + 新枚举），无存量数据/接口变更；回滚只需下线新接口与调度器、保留空表即可，不影响资源预测链路。

## Open Questions

- Q-E：`planTime` 早于 now+35 天 → 计划透传 CRP 由其校验并原样返回错误（本变更 client 只需保证透传）。
- Q-I：部分子单失败 → 已定：独立重试 + 主单聚合 `partial_failed` + 失败原因聚合进 message。
- Q-N：退回计划按 `technicalClass` 覆盖筛选映射 → 属上层覆盖追加接口，本变更不涉及；待与 CRP 确认字段。
- ~~Q-O：`cancelTodo` 独立 host 的配置来源~~ → 已关闭：`cancelTodo` 本期不封装（终止仅改本地状态、不撤回 CRP），host 配置来源待覆盖追加子需求需要撤单时再定。
- ~~主单 `details` 明细与详情接口返回的明细是否需要与 CRP 实时对齐~~ → 已关闭：详情接口返回本地单据快照 `details`/`sub_details`，本期不实时 proxy CRP（见 D-1「详情接口数据源」）。
