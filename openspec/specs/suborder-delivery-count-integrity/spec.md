# suborder-delivery-count-integrity Specification

## Purpose
TBD - created by archiving change fix-scheduler-queue-premature-release. Update Purpose after archive.
## Requirements
### Requirement: 子单已交付台数不超过需求总数

系统 SHALL 保证任一申请子单的已交付台数不超过该子单的需求总数，即 `success_num ≤ total_num`。

在容量充足的正常路径上，子单 SHALL 恰好交付到需求总数：`success_num` 等于 `total_num`、`pending_num` 为 0、状态为 DONE，且 `ziyan_cvm_device_info` 中该子单 `is_delivered = true` 的行数等于需求总数，不多也不少。

生产入口总闸的在途数统计逻辑 MUST NOT 因本能力而改动；本能力通过恢复「同一子单串行处理」这一前提，使总闸原有的 `scheduledCount >= total_num` 判断重新成立。

追溯：业务规则 R-001；验收 AC-005 / AC-006 / AC-P03。

#### Scenario: 分 Campus 子单恰好交付到需求数

- **GIVEN** 一个分 Campus 的 CVM 子单需求数 424 台、各可用区容量充足
- **WHEN** 调度器完成全部生产与匹配轮次
- **THEN** 子单 `success_num = 424`、`pending_num = 0`、状态为 DONE，且 `ziyan_cvm_device_info` 中该子单 `is_delivered = true` 的行数等于 424（AC-005）

#### Scenario: 已调度数达标时不再新增生产

- **GIVEN** 某子单需求数 424 台、已落库设备 400 台、在途生产记录合计 24 台
- **WHEN** 调度器在这批在途记录回执前再次轮询到该子单
- **THEN** 不新增任何 `ziyan_cvm_generate_record` 行，日志出现 `apply order <id> has been scheduled 424 cvm (existing: 400, generatingCount: 24)`（AC-006）

#### Scenario: 上线后无新增超发子单

- **GIVEN** 本能力上线
- **WHEN** 连续 30 天每日执行 `SELECT COUNT(*) FROM ziyan_cvm_apply_suborder WHERE success_num > total_num AND created_at >= <上线日期>`
- **THEN** 每日结果均为 0，即 `success_num > total_num` 的子单新增数量为 0（AC-P03）

### Requirement: 交付数统计口径与展示口径保持不变

系统 MUST NOT 改变「交付数由 `is_delivered = true` 的设备条数统计得出」这一既有口径，也 MUST NOT 改变 `delivered_core` 按这批设备机型累加 CPU 核数的算法。

前端申请单明细的「总数 / 待交付 / 已交付」三列 MUST 仍分别取自子单的 `total_num` / `pending_num` / `success_num`；`total_num` / `success_num` / `pending_num` 的语义与 `list_ticket_apply` 等既有对外接口的字段 MUST NOT 变更。

追溯：业务规则 R-005；验收 AC-009 / AC-010。

#### Scenario: 前端三列字段来源不变

- **GIVEN** 本能力上线后
- **WHEN** 在前端查看任一已完成申请单明细
- **THEN** 「总数 / 待交付 / 已交付」三列仍分别取自 `total_num` / `pending_num` / `success_num`，且子单 `delivered_core` 与上线前同口径（AC-009）

#### Scenario: 上线前后统计口径一致

- **GIVEN** 同一个已完成子单在上线前后各取一次数据快照
- **WHEN** 比对两次快照，SQL 比对 `success_num` 与 `SELECT COUNT(*) ... WHERE is_delivered = 1`，并对交付数统计相关逻辑做代码走查
- **THEN** `success_num` 仍等于该子单在 `ziyan_cvm_device_info` 中 `is_delivered = true` 的设备条数，`delivered_core` 仍等于按这批设备机型累加的 CPU 核数；二者任一不相等即视为统计口径已变更，本条不通过（AC-010）

