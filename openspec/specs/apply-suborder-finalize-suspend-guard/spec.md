# apply-suborder-finalize-suspend-guard

## Purpose

主机申请 matcher 在存在 Suspend 生产记录时，不得把已交付满额的子单终态覆盖为 TERMINATE/SUSPEND；跳过覆盖时通过 warning 日志提示人工核对云梯是否存在多余机器。

## Requirements

### Requirement: 已交付满额时 suspend 状态不得覆盖终态

系统 SHALL 在 matcher 判定子单终态时，仅当子单尚未交付满额（已交付设备数小于 `total_num`）才允许 suspend 分支将状态覆盖为 TERMINATE/SUSPEND；已交付满额的子单 MUST 按 `calcApplyOrderStatus` 算出的正常终态（DONE）落库。

生产步骤状态与 Suspend 生产记录的后续处理 MUST 不受该守卫影响：Suspend 记录仍 SHALL 被标记为 Failed 并保留审计 message，生产步骤状态仍 SHALL 按既有逻辑维护。

#### Scenario: 满额交付且存在挂起生产记录时落为 DONE

- **WHEN** 某子单已交付设备数等于 `total_num`、计算出的终态为 DONE，但存在 `status=Suspend` 的生产记录
- **THEN** 子单状态落为 DONE，而非 TERMINATE/SUSPEND；该 Suspend 生产记录仍被改写为 Failed 并保留审计 message

#### Scenario: 部分交付且存在挂起生产记录时维持 TERMINATE

- **WHEN** 某子单已交付设备数小于 `total_num`，且存在 `status=Suspend` 的生产记录使 `suspendCnt + matchedCnt >= total_num`
- **THEN** 子单状态按原逻辑落为 TERMINATE/SUSPEND，守卫不改变该路径行为

### Requirement: 挂起生产记录的人工核对信号

系统 SHALL 在因已交付满额而跳过 suspend 覆盖时输出 warning 级别日志，内容 MUST 包含子单号、挂起生产记录的 generateID、已交付台数与需求台数，以提示人工核对云梯是否存在未纳管的多余机器。

该信号 MUST 只落在日志，不得写入子单 `remark` 等业务字段，避免机器标记与用户备注互相污染。

#### Scenario: 跳过覆盖时输出告警日志

- **WHEN** 某子单因已交付满额而跳过 suspend 覆盖，落为 DONE
- **THEN** 输出 warning 日志，包含挂起生产记录的 generateID 与已交付/需求台数

#### Scenario: 未跳过覆盖时不输出该告警

- **WHEN** 某子单因部分交付而按原逻辑落为 TERMINATE/SUSPEND
- **THEN** 不输出该告警日志
