## ADDED Requirements

### Requirement: 报表汇总头
系统 SHALL 对统计周期内所有命中的预测返还单，按 `QueryOrderInfo.AllCoreAmount` 求和得到 CPU 总核数，按 `CvmData` 中 Unit="GB" 的 Value 求和得到内存总量，并展示统计开始/结束日期。

#### Scenario: 多单据汇总
- **GIVEN** 统计周期内命中多条 `QueryOrderInfo`
- **WHEN** 系统生成汇总头
- **THEN** CPU 总核数 = 所有单据 `AllCoreAmount` 之和
- **AND** 内存总量 = 所有单据 `CvmData`（Unit="GB"）Value 之和
- **AND** 展示统计开始日期（上周一 00:00:00）与结束日期（上周日 23:59:59）

### Requirement: 明细表字段映射
系统 SHALL 为每条预测返还单生成一行明细，全部 10 列（规划产品、运营产品、项目类型、日期、机型族、城市、CPU总核数、内存总量(GB)、磁盘总量(GB)、CRP单据链接）均来自 CRP `QueryOrderInfo` / `CvmData`。

#### Scenario: 字段全部取自 CRP
- **GIVEN** 一条 `QueryOrderInfo`
- **WHEN** 系统生成明细行
- **THEN** 规划产品取自 `PlanProductName`、运营产品取自 `ToPlanProductName`、项目类型取自 `OrderTypeName`、日期取自 `UseTime`（或 `CreateTime`）、机型族取自 `CvmData[].Name`、城市取自 `CvmData[]` 关联字段、CPU总核数取自 `AllCoreAmount`、内存总量取自 `CvmData[].Value`（GB）、磁盘总量取自 `AllDiskAmount`、CRP单据链接由 `OrderID` 拼接

#### Scenario: 磁盘总量取 CRP 真实值
- **GIVEN** 一条 `QueryOrderInfo` 含 `AllDiskAmount`
- **WHEN** 系统生成明细行的磁盘总量列
- **THEN** 该列值 = `QueryOrderInfo.AllDiskAmount`（GB）

### Requirement: 无单据简化样式
系统 SHALL 在命中 0 条时生成简化邮件，正文为"本统计周期内，无资源预测转移免审单"，不渲染空表。

#### Scenario: 命中 0 条
- **GIVEN** 统计周期内无任何命中记录（或 CRP 全部无返回）
- **WHEN** 系统组装邮件
- **THEN** 正文为"本统计周期内，无资源预测转移免审单"
- **AND** 不渲染空明细表
