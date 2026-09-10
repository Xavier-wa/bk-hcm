## Why

IEG（互娱）主机在执行销毁回收时，其释放的资源作为"返还预测"回流到 IEG 中转池，并由 CRP 生成"转移免审单"（中转池内部转移、CRP 自动免审通过）。目前业务方缺少一份按周汇总的"销毁返还预测"视图，无法直观掌握每周有多少主机销毁返还预测回流到 IEG 中转池、分别来自哪些规划/运营产品、分布在哪些城市与机型族、规模（CPU/内存/磁盘）有多大，只能依赖人工线下拉取统计。

> **架构说明**：海垒（HCM）在"主机销毁返还"流程中仅记录返还任务到 `cr_ReturnTask`，**不生成也不处理"预测返还单"**——"预测返还单"是 CRP（云梯需求平台）侧单据。报表统计链路分两步：(1) 从 HCM `cr_ReturnTask` 取返还候选 → (2) 逐条按销毁单号调 CRP `queryOrderList` 查对应的预测返还单明细。

通过**每周一定时邮件**的形式，自动汇总上一统计周期内 IEG 技术运营部的"销毁返还预测转移免审单"，以固定模板推送给相关干系人，替代人工线下统计。

## What Changes

- **每周一定时触发**：新增定时任务，每周一 08:00:00 触发（由定时任务判断当天为周一且到达 08:00），仅由主节点（master）执行，避免多节点重复发送（与现有 `confirm_notice` 机制一致）。统计周期固定为上周一 00:00:00 ~ 上周日 23:59:59。

- **按口径筛选候选并查 CRP**：取数主体为 MongoDB 集合 `cr_ReturnTask`（非 `res_plan_sub_ticket`，后者仅用于月底清理过期预测，与本次无关）。海垒侧筛选条件：`status = SUCCESS`、`update_at`（= status 置为 SUCCESS 的写入时间）落在统计周期内。**不再使用 `return_forecast` 作为筛选条件**，也不再经 `GetBizOrgRel` 做部门过滤。销毁单号取 `cr_ReturnTask.task_id`（形如 `TH...`，即 CRP 的 `DestroyReturnPlanOrderId`），逐条调 CRP `queryOrderList`（HCM 已封装 `ResPlanFetcher.GetOrderList`），返回的每条 `QueryOrderInfo`（`Status=0`）即一条预测返还单，明细字段全部来自 CRP。

- **生成汇总头与明细表**：汇总头 CPU 总核数 = 所有命中单据 `QueryOrderInfo.AllCoreAmount` 求和，内存总量 = 所有单据 `CvmData` 中 Unit="GB" 的 Value 求和；明细表每一条 CRP 预测返还单生成一行，全部 10 个字段（规划产品、运营产品、项目类型、日期、机型族、城市、CPU总核数、内存总量(GB)、磁盘总量(GB)、CRP单据链接）均来自 CRP `QueryOrderInfo` / `CvmData`。

- **组装并发送 HTML 邮件**：邮件标题 `IEG-销毁返还预测统计周报-{{发送日（周一当天日期）}}`（如 `2026-01-12`）。有单据时正文为标题 + 汇总头 + "转移免审单详情"明细表 + 备注（含疑问联系人 / ICR）；命中 0 条时发送简化邮件，正文为"本统计周期内，无资源预测转移免审单"，不渲染空表。收件人通过**配置文件/配置项**定义（非硬编码），默认空列表，由部署环境配置。复用现有 HCM 邮件网关（cmsi）发送。

## Capabilities

### New Capabilities

- `destroy-return-forecast-weekly-scheduler`: 销毁返还预测周报定时调度能力——每周一 08:00:00 由 master 节点触发，计算统计周期（上周一 00:00:00 ~ 上周日 23:59:59），仅执行一次，避免多节点重复发送。

- `destroy-return-forecast-data-fetch`: 销毁返还预测数据取数能力——从 `cr_ReturnTask` 按 `status=SUCCESS` + `update_at` 在周期内筛选候选集，逐条以 `task_id` 调 CRP `queryOrderList`（`GetOrderList`）取预测返还单明细，所有明细字段统一来自 CRP 返回。

- `destroy-return-forecast-report-build`: 销毁返还预测报表组装能力——汇总命中单据的 CPU/内存总量，构建含全部 10 列的明细表（有单据样式），以及命中 0 条时的简化样式（不渲染空表）。

- `destroy-return-forecast-mail-send`: 销毁返还预测邮件发送能力——以配置化收件人列表经 cmsi 网关发送 HTML 邮件，标题日期为发送日（周一当天）；CRP 任一条查询失败时整体失败，不发送不完整报表。

### Modified Capabilities

<!-- 无 -->

## Impact

- **定时任务框架（新增/复用）**：在现有 HCM 定时任务框架中注册周报任务，复用 `confirm_notice` 的 master 选举机制，仅由主节点执行。

- **woa-server / logics/plan（新增）**：新增销毁返还预测周报逻辑模块，包含调度入口、取数（`cr_ReturnTask` + `ResPlanFetcher.GetOrderList`）、报表组装、邮件发送四个子流程。

- **数据访问（复用）**：`cr_ReturnTask` 查询经 `pkg/dal/` 层；CRP 调用复用 `ResPlanFetcher.GetOrderList`（`pkg/thirdparty/cvmapi`）。

- **配置（新增）**：新增收件人等配置项（配置文件/配置项，非硬编码），支持运维动态调整。

- **邮件网关（复用）**：复用现有 cmsi 邮件发送客户端，邮件为 HTML 富文本，样式对齐现有 HCM 通知邮件。

- **外部依赖**：CRP `queryOrderList` 接口需可用（HCM 生产代码已有封装且在线上运行）；MongoDB 直读 `cr_ReturnTask`。
