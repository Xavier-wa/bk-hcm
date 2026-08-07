## ADDED Requirements

### Requirement: Support 公共费用分摊在零基数时不得 panic

系统在执行 AWS / 华为 / GCP Support（CommonExpense）月度任务 split 时，MUST 在参与分摊账号 `CurrentMonthCost` 合计为 0 的情况下避免除零 panic，并按约定策略完成或结束任务。

#### Scenario: 零基数且存在公共费用时平均分摊

- **WHEN** 参与分摊账号的 `CurrentMonthCost` 合计 `summaryTotal = 0`，且公共费用 `batchSum ≠ 0`
- **THEN** 系统 MUST 将 `batchSum` 平均分配给各参与分摊账号，各账号分摊额之和 MUST 等于 `batchSum`，且 MUST NOT panic

#### Scenario: 双零时成功结束

- **WHEN** `batchSum = 0` 且 `summaryTotal = 0`
- **THEN** 系统 MUST 视为无需分摊并成功结束，MUST NOT 因除零失败

#### Scenario: 非零基数保持比例分摊

- **WHEN** `summaryTotal ≠ 0`
- **THEN** 系统 MUST 继续按 `batchSum * CurrentMonthCost / summaryTotal` 比例分摊，行为与修复前一致

#### Scenario: 单个账号消费为 0 但不触发平均分摊

- **WHEN** 部分参与分摊账号 `CurrentMonthCost = 0`，但其余账号合计使 `summaryTotal ≠ 0`
- **THEN** 系统 MUST 走比例分摊；消费为 0 的账号分摊额为 0，MUST NOT 进入平均分摊分支
