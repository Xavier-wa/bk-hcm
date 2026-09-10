## Context

### 现有流程

IEG 主机销毁回收返还链路：

```
主机销毁回收（CVM）
  └─ HCM 写 cr_ReturnTask（status=SUCCESS 表示返还成功）
       ├─ task_id：销毁单号（形如 TH...），即 CRP 的 DestroyReturnPlanOrderId
       ├─ bk_biz_id：业务 ID
       └─ update_at：status 置为 SUCCESS 的写入时间（统计周期判定字段）
            └─ 逐条 task_id → CRP queryOrderList（ResPlanFetcher.GetOrderList）
                 └─ 返回 QueryOrderInfo 列表（直接采用接口返回的全部单据）
```

### 现有问题

- 业务方缺少按周汇总的"销毁返还预测"视图，依赖人工线下统计。
- HCM 不产预测返还单，全部来自 CRP；`cr_ReturnTask` 已是海垒侧本部门业务数据，无需再经 `GetBizOrgRel` 做部门过滤。
- `return_forecast` 字段在生产链路中已失效（仅一步式入口永远 false、CVM 调用硬编码 ReturnForecast=2），不可作为筛选条件。

### 涉及文件 / 服务

| 对象 | 角色 |
|------|------|
| `cmd/woa-server/logics/plan/fetcher/crp.go` | `ResPlanFetcher.GetOrderList` 封装，调 CRP `queryOrderList` |
| `cmd/woa-server/logics/plan/destory_cvm.go` | 销毁返还生产链路，`cr_ReturnTask` 取 `task_id` 调 `GetOrderList` 的参考实现 |
| `pkg/thirdparty/cvmapi/` | CRP 接口客户端，含 `QueryOrderInfo` / `CvmData` 结构 |
| `cr_ReturnTask`（Mongo） | 取数主体，经 `pkg/dal/` 层查询 |
| cmsi 邮件网关 | 复用现有通知邮件发送客户端 |

## Goals / Non-Goals

**Goals:**

- 每周一 08:00 自动统计上一自然周 IEG 技术运营部的销毁返还预测，生成并发送 HTML 周报邮件。
- 明细表全部字段从 CRP 取值（含日期、磁盘），不再依赖 `return_forecast_time` / `queryCvmTypeList`。
- 收件人配置化，支持运维动态调整。
- 有单据 / 无单据两种邮件样式，CRP 单条查询失败时整体失败，不发送不完整报表。

**Non-Goals:**

- 不涉及 `res_plan_sub_ticket` 月底清理过期预测逻辑（与本次无关）。
- 不涉及非 IEG 技术运营部的返还预测报表。
- 不涉及 `queryCvmTypeList` 接口调用（磁盘等数据统一从 CRP 取）。
- 不涉及邮件交互（仅单向通知，无点击跳转/回复）。
- 不涉及历史数据补发（仅从需求上线后开始按周发送）。

## Decisions

### D1: 取数口径

**选择**：取数主体为 `cr_ReturnTask`，筛选条件为 `status = SUCCESS` + `update_at` 在统计周期内。

**理由**：
- `cr_ReturnTask` 是海垒在销毁返还流程中唯一写入的返还记录，且 `update_at`（status 置为 SUCCESS 的同一事务写入）准确反映"实际成功流入中转池的时间"。
- `cr_ReturnTask` 已是海垒侧本部门业务数据，无需再经 `GetBizOrgRel` 做部门过滤。
- `return_forecast` 不可靠，仅作为生产链路触发开关，不作为筛选条件；"是否是预测返还"以 CRP `queryOrderList` 是否返回有效记录为准。

**数据来源**：`cr_ReturnTask.task_id`（销毁单号）= CRP `DestroyReturnPlanOrderId`。

### D2: 统计周期

**选择**：统计周期 = 上周一 00:00:00 ~ 上周日 23:59:59，以 `cr_ReturnTask.update_at` 落入周期判定。

**理由**：以"实际成功时间"归属统计周期，避免"创建于上周、本周才成功"的单据被漏计或误计。

### D3: 明细字段来源

**选择**：明细表全部 10 列（规划产品、运营产品、项目类型、日期、机型族、城市、CPU总核数、内存总量(GB)、磁盘总量(GB)、CRP单据链接）统一从 CRP `QueryOrderInfo` / `CvmData` 取值。

| 列 | 来源 |
|----|------|
| 规划产品 | `QueryOrderInfo.PlanProductName` |
| 运营产品 | `QueryOrderInfo.ToPlanProductName` |
| 项目类型 | `QueryOrderInfo.OrderTypeName` |
| 日期 | `QueryOrderInfo.UseTime`（或 `CreateTime`） |
| 机型族 | `CvmData[].Name` |
| 城市 | `CvmData[]` 关联字段 |
| CPU总核数 | `QueryOrderInfo.AllCoreAmount` |
| 内存总量(GB) | `CvmData[].Value`（Unit="GB"） |
| 磁盘总量(GB) | `QueryOrderInfo.AllDiskAmount` |
| CRP单据链接 | `QueryOrderInfo.OrderID` 拼接  |

**理由**：`QueryOrderInfo` 结构已包含全部所需字段，无需再单独调 `queryCvmTypeList`。

### D4: 邮件标题与收件人

**选择**：标题 `IEG-销毁返还预测统计周报-{{发送日（周一当天日期）}}`（如 `2026-01-12`）；收件人从配置读取，默认空列表，由部署环境配置。

**理由**：标题日期取发送日而非周期结束日；收件人配置化以支持运维动态调整，无需改代码重新部署。

### D5: 执行节点与容错

**选择**：定时任务仅由 master 节点执行；CRP `queryOrderList` 任一条查询失败时记录错误日志并整体失败，不发送不完整报表，待手动重试。

**理由**：与现有 `confirm_notice` 机制一致，避免多节点重复发送；不完整数据会误导业务，失败告警后由人工重试更稳妥。

## Risks / Trade-offs

**[风险] CRP 接口偶发失败**
→ 任一条失败则任务失败且不发送邮件，记录错误日志，由人工手动重试。

**[风险] 单封邮件明细过长**
→ 明细行数建议有上限保护（如超过 500 行分页或截断并提示）。

**[风险] 多节点重复发送**
→ 仅 master 执行，复用现有 master 选举机制。

**[权衡] 磁盘总量依赖 CRP 返回**
→ 需求原始描述中"磁盘总量字段等 CRP 提供相关接口"，本次以 `QueryOrderInfo.AllDiskAmount` 为准，无需等待独立接口。

## Resolved Questions

- **取数对象**：`cr_ReturnTask`（非 `res_plan_sub_ticket`）。
- **部门过滤**：不需要；`cr_ReturnTask` 已是海垒侧本部门业务数据。
- **销毁单号**：`cr_ReturnTask.task_id`（非 `suborder_id`）。
- **筛选开关**：不使用 `return_forecast`；是否预测返还以 CRP 返回为准。
- **字段来源**：明细表全部字段来自 CRP，日期取 `UseTime`/`CreateTime`，磁盘取 `AllDiskAmount`。
- **标题日期**：发送日（周一当天）。
- **收件人**：配置文件/配置项管理，非硬编码。
- **时间字段**：`update_at`（status=SUCCESS 写入时间）。
