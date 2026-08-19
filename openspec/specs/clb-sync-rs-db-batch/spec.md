# clb-sync-rs-db-batch Specification

## Purpose
tcloud-ziyan CLB 同步查本地 RS 时，按在用目标组 ID 批量 IN 查询，消除按目标组逐个 ListTarget 的 N+1。
## Requirements
### Requirement: 按在用目标组批量查询本地 RS

tcloud-ziyan CLB 同步在对比云上与本地 RS 时，系统 MUST 先按负载均衡 ID（及可选的监听器云 ID）查询 `target_group_listener_rule_rel`，再对去重后的目标组 ID 使用 `IN` 批量查询 `load_balancer_target`，MUST NOT 对每个目标组发起一次独立的 `ListTarget`。

批量大小 MUST 为 `CloudResourceSyncMaxLimit`（100）。单次列表页大小 MUST 为默认分页上限（500），结果超过一页时 MUST 翻页直到取完。

查询口径 MUST 是挂在 rel 上的在用目标组，MUST NOT 用 `load_balancer_target_group` 全表作为 RS 查询输入。

Vendor MUST 为 tcloud-ziyan。

#### Scenario: 多个在用目标组只产生批次级 ListTarget

- **GIVEN** 某 LB（或本批监听器）在 rel 上有 113 个去重目标组
- **WHEN** 同步这些目标组的本地 RS
- **THEN** `ListTarget` 调用次数为按 100 切批后再按 500 翻页的次数，而不是 113
- **AND** 日志 `list_target_calls` 等于实际 RPC 次数而不是目标组个数

#### Scenario: 没有 rel 时不查 RS

- **GIVEN** 该 LB（或本批监听器）没有任何 `target_group_listener_rule_rel`
- **WHEN** 查询本地 RS
- **THEN** 系统 MUST NOT 调用 `ListTarget`
- **AND** 返回空的 rel 与 RS 映射

### Requirement: 无 RS 的目标组在映射中保留空值

批量查询结果按 `target_group_id` 归类后，在用但没有任何 RS 的目标组 MUST 在结果映射中仍占有该 key，值为空（与逐目标组查询时「该 TG 无 RS」的语义一致），以便后续 diff 能把云上 RS 写入该目标组，而不是跳过。

#### Scenario: 在用目标组没有 RS

- **GIVEN** rel 中存在目标组 T，且 `ListTarget` 未返回 T 的任何 RS
- **WHEN** 完成本地 RS 批量查询
- **THEN** 结果映射包含 key T
- **AND** T 对应的 RS 列表为空
