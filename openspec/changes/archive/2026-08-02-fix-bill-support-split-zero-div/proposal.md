## Why

账单月度任务在执行 Support / 公共费用（CommonExpense）分摊时，按二级账号 `current_month_cost` 占比做除法。当参与分摊账号费用合计为 0 时会触发 `decimal division by 0` panic，导致 `async_flow_task` 失败、月度任务卡住。灰环境 AWS support split 已复现（TAPD #136713430），需尽快修复并覆盖华为、GCP 同类逻辑。

## What Changes

- 在 AWS / 华为 / GCP Support 分摊逻辑中增加零基数保护，避免 `Div(0)` panic
- `summaryTotal = 0` 且 `batchSum ≠ 0` 时，对参与分摊账号做**平均分摊**（金额之和等于 `batchSum`）
- `batchSum = 0` 且 `summaryTotal = 0` 时，视为无需分摊并成功结束
- `summaryTotal ≠ 0` 时保持原有比例分摊行为不变
- 抽取可单测的分摊计算函数，并补充零基数 / 常规比例场景单测

## Capabilities

### New Capabilities

- `bill-support-common-expense-split`: Support / CommonExpense 月度分摊的零基数安全分摊策略（平均分摊 / 双零成功结束 / 常规比例分摊）

### Modified Capabilities

（无现有 spec 级行为变更）

## Impact

- **Task Server**：`cmd/task-server/logics/action/bill/monthtask/aws_support.go`、`huawei_support.go`、`gcp_support.go`；新增共享分摊计算与单测
- **API / 表结构**：无变更
- **历史数据**：不统一回刷；上线后新执行生效
