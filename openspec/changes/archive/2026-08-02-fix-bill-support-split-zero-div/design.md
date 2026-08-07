## Context

Support / CommonExpense 月度任务在 `split` 阶段将根账号公共费用按二级账号当月消费占比分摊。核心计算为：

```text
cost = batchSum * CurrentMonthCost / summaryTotal
```

`summaryTotal` 为参与分摊账号（排除根账号自身与配置排除账号）的 `CurrentMonthCost` 之和。当该和为 0 时，`shopspring/decimal.Div` 会 panic。

涉及实现：
- AWS：`AwsSupportMonthTask.splitCommonExpense`
- 华为：`HuaweiSupportMonthTask.splitCommonExpense`
- GCP：`GcpSupportMonthTask.Split` 内同类比例计算

约束：不改 Pull 口径、不改排除账号规则、不改 `summaryTotal ≠ 0` 时的比例分摊语义；历史账期不统一回刷。

## Goals / Non-Goals

**Goals:**
- 消除三云 Support 分摊中的除零 panic
- 零基数且存在公共费用时按账号数平均分摊，且分摊额之和等于 `batchSum`
- 双零时安全成功结束
- 提供可单测的纯计算函数，降低三处复制逻辑风险

**Non-Goals:**
- 不改造日账 / summary_main 状态机
- 不统一回刷历史账期
- 不调整公共费用 Pull 规则或排除账号配置
- 不引入新的外部依赖或 API

## Decisions

### 1. 抽取共享纯函数做金额分配

在 `monthtask` 包新增 `allocateCommonExpense(batchSum, costs []decimal.Decimal) []decimal.Decimal`：

| 条件 | 行为 |
|------|------|
| `len(costs)==0` | 返回 nil |
| `summaryTotal≠0` | 按比例：`batchSum * cost_i / summaryTotal` |
| `summaryTotal==0` 且 `batchSum==0` | 返回全 0 切片（调用方可据此短路） |
| `summaryTotal==0` 且 `batchSum≠0` | 平均分摊；前 N-1 份用 `Div`，最后一份用 `batchSum - 已分配` 吃掉精度差额 |

**理由**：三云分摊公式一致，纯函数便于单测；余数落在最后一份保证总和严格等于 `batchSum`。

**备选**：各自文件内联 if 分支——否决，易三云行为漂移。

### 2. 调用侧短路双零

当 `summaryTotal.IsZero() && batchSum.IsZero()` 时，AWS/华为/GCP 在生成账单明细前直接返回成功（nil items），并打 Info 日志（含 rid）。

**理由**：与需求「无需分摊、正常结束」一致，避免写入无意义的 0 金额明细。

### 3. 零基数平均分摊打 Warn 日志

进入平均分摊分支时打 Warn（含 rid、root_account_id、账期、账号数、batchSum），便于运维发现「有公共费用但消费基数全 0」的异常数据场景。

### 4. GCP 空列表保护

若 `summaryMainList` 为空，直接返回成功（与 AWS/华为已有行为对齐），避免无意义后续计算。

## Risks / Trade-offs

- [Risk] 零基数平均分摊可能把本应「数据异常」的公共费用摊到无消费账号 → Mitigation：打 Warn；业务已确认此策略优于失败/panic
- [Risk] 平均分摊精度差额 → Mitigation：最后一份吃余数，保证总和等于 `batchSum`
- [Risk] 三云调用点漏改 → Mitigation：共享函数 + 单测覆盖三分支；tasks 逐文件勾选

## Migration Plan

1. 发布 task-server 含修复版本
2. 对新账期 / 重跑的 Support 月度任务自动生效
3. 历史失败账期由运维按需重跑（不自动回刷）
4. 回滚：回退 task-server 版本即可；无 DB schema 变更

## Open Questions

无（已在需求澄清中确认：平均分摊、双零成功、三云一并、历史不回刷）。
