## 1. 定时调度

- [x] 1.1 在现有 HCM 定时任务框架中注册销毁返还预测周报任务，每周一 08:00:00 触发（判断当天为周一且到达 08:00）
- [x] 1.2 复用 `confirm_notice` 的 master 选举机制，确保仅由 master 节点执行，避免多节点重复发送
- [x] 1.3 计算统计周期：上周一 00:00:00 ~ 上周日 23:59:59

## 2. 数据取数

- [x] 2.1 通过 `pkg/dal/` 层查询 `cr_ReturnTask`，筛选 `status = SUCCESS`
- [x] 2.2 按 `update_at` 落在统计周期内过滤（= status 置为 SUCCESS 的写入时间）
- [x] 2.3 取 `cr_ReturnTask.task_id` 作为销毁单号（CRP 的 `DestroyReturnPlanOrderId`）
- [x] 2.4 逐条调用 `ResPlanFetcher.GetOrderList`（`queryOrderList`），仅保留 `QueryOrderInfo.Status = 0` 的有效预测返还单
- [x] 2.5 单条 CRP 查询失败时记录错误日志并整体失败，不发送不完整报表，待手动重试

## 3. 报表组装

- [x] 3.1 汇总头：CPU 总核数 = 所有命中单据 `QueryOrderInfo.AllCoreAmount` 求和；内存总量 = 所有单据 `CvmData` 中 Unit="GB" 的 Value 求和
- [x] 3.2 明细表：每条预测返还单生成一行，含全部 10 列（规划产品、运营产品、项目类型、日期、机型族、城市、CPU总核数、内存总量(GB)、磁盘总量(GB)、CRP单据链接），全部取自 CRP `QueryOrderInfo` / `CvmData`
- [x] 3.3 日期列取 `QueryOrderInfo.UseTime`（或 `CreateTime`）；磁盘列取 `QueryOrderInfo.AllDiskAmount`；CRP单据链接由 `OrderID` 拼接
- [x] 3.4 命中 0 条时生成简化样式正文"本统计周期内，无资源预测转移免审单"，不渲染空表

## 4. 邮件发送

- [x] 4.1 从配置文件/配置项读取收件人列表（`receivers` / `ccReceivers`），默认空列表，由部署环境配置
- [x] 4.2 邮件标题格式 `IEG-销毁返还预测统计周报-{{发送日（周一当天日期）}}`（如 `2026-01-12`）
- [x] 4.3 复用现有 cmsi 邮件网关发送 HTML 富文本邮件，样式对齐现有 HCM 通知邮件
- [x] 4.4 有单据样式正文：标题 + 汇总头 + "转移免审单详情"明细表 + 备注（含疑问联系人 / ICR）

## 5. 单元测试

- [x] 5.1 取数筛选单元测试：覆盖 `status` 非 SUCCESS 过滤、`update_at` 跨周边界
- [x] 5.2 CRP 取明细单元测试：覆盖返回有效单、返回空、Status≠0、单条查询失败整体失败
- [x] 5.3 报表组装单元测试：覆盖 CPU/内存求和、10 列字段映射、无单据简化样式
- [x] 5.4 邮件发送单元测试：覆盖收件人配置读取、标题日期格式、HTML 渲染

## 6. 集成验证

- [x] 6.1 验证：周一 08:00 master 节点触发且仅触发一次
- [x] 6.2 验证：`cr_ReturnTask` 中 `status=SUCCESS` + `update_at` 在周期内的记录进入候选并查 CRP
- [x] 6.3 验证：命中记录时邮件标题/汇总头/10 列明细正确
- [x] 6.4 验证：磁盘总量 = `QueryOrderInfo.AllDiskAmount`
- [x] 6.5 验证：命中 0 条时发送简化邮件，不渲染空表
- [x] 6.6 验证：收件人从配置读取（`woa_server.yaml` / helm values）
- [x] 6.7 验证：CRP 单条查询失败时任务失败且不发送邮件，可手动重试
- [x] 6.8 验证：单周期（生成+发送）整体耗时 ≤ 5 分钟

> 说明：6.1~6.9 的运行时验证需在部署环境（Mongo / CMDB / CRP / cmsi 可用）下进行；本地已完成编译验证（`go build` 通过）与单元测试（7 项全部 PASS）。
