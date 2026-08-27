# bill-adjustment-push-status

## Purpose

调账 OBS 推送状态两端打点：pushing / pushed / failed 与残留兜底纠正。

## Requirements

### Requirement: 推送前批量置推送中

系统 SHALL 在 SyncController 创建调账同步 Flow 时，批量将该 `(vendor, 年, 月)` 下 `state=confirmed` 的调账置为 `push_status=pushing`。`state=unconfirmed` 的调账 MUST NOT 被置位，且 MUST NOT 进入 OBS。批量置位 MUST 按 `constant.BatchOperationMaxLimit` 分片，MUST NOT 使用单条循环更新。

> 这是与 OBS MySQL 的集成点：调账数据经现网 `obs_sync_adjustment` Flow 直连写入 OBS 库，本 capability 只在两端打点，不改变其「整月批量 clean + 全量插入」的批处理本质。

#### Scenario: 已确认调账在 Flow 创建前被置为推送中

- **GIVEN** 账期 2026-08 下有 10 条 `state=confirmed` 的调账
- **WHEN** SyncController 创建该账期的调账同步 Flow
- **THEN** 这 10 条的 `push_status` 在 Flow 创建前被批量置为 `pushing`

#### Scenario: 未确认调账不被置位也不进 OBS

- **GIVEN** 同账期下另有 3 条 `state=unconfirmed` 的人工调账
- **WHEN** 同一 Flow 创建与执行
- **THEN** 这 3 条的 `push_status` 保持 `unpushed`
- **AND** OBS 侧不存在这 3 条的数据

### Requirement: 推送后整月统一回写与兜底纠正

系统 SHALL 在 `SyncAdjustmentAction.Run` 的**所有分页全部插入成功后**统一回写：本次实际处理的 ID 集合置 `pushed`，该账期内其余 `pushing` 残留置回 `unpushed`。系统 MUST NOT 逐页回写 —— `clean` 是插入前一次性完成的，中途失败时 OBS 数据本就不完整，此时把已插入页标成 `pushed` 会产生「HCM 说已推送、OBS 实际缺数据」的错账。

#### Scenario: 整月全部插入成功后置已推送

- **GIVEN** 账期 2026-08 下 10 条调账被置 `pushing`
- **WHEN** Flow 全部分页插入 OBS 成功
- **THEN** 本次实际插入的 10 条置 `pushed`
- **AND** 该账期内不存在 `push_status=pushing` 的记录

#### Scenario: 中途分页失败时不得部分置已推送

- **GIVEN** Flow 分 3 页插入，第 2 页插入失败
- **WHEN** Flow 结束
- **THEN** 前 1 页已插入的条目不得被置为 `pushed`
- **AND** 相关调账按失败分支置 `failed`

#### Scenario: pushing 残留被兜底纠正

- **GIVEN** 置 `pushing` 后其中 2 条在 Flow 捞取前被删除、Flow 实际只插入 8 条
- **WHEN** Flow 成功返回
- **THEN** 8 条置 `pushed`
- **AND** 该账期内其余 `pushing` 残留被兜底置回 `unpushed`

### Requirement: 推送失败处理

系统 SHALL 在 Flow 失败或取消时将相关调账置 `push_status=failed` 并写入 `push_fail_reason`，随后按现网既有行为重建 Flow（调账随之回到 `pushing`）。本期 MUST NOT 引入重试计数与重试上限，MUST NOT 新建独立告警通道，复用现网 `BillSyncRecord` 状态与既有运维告警。

#### Scenario: Flow 失败时置失败并写原因

- **GIVEN** 某账期调账已被置 `pushing`
- **WHEN** Flow 失败或取消
- **THEN** 相关调账置 `push_status=failed`
- **AND** `push_fail_reason` 非空

#### Scenario: Flow 失败后重建 Flow

- **GIVEN** Flow 失败且相关调账已置 `failed`
- **WHEN** 系统重建 Flow
- **THEN** 相关调账重新回到 `pushing`

### Requirement: 推送状态不变式

系统 SHALL 保证一次整月推送结束后，该账期内不存在 `push_status=pushing` 的残留 —— 要么 `pushed`，要么被兜底纠正回 `unpushed`，要么 `failed`。`push_status` 枚举 SHALL 仅有四态，MUST NOT 引入「超时」态（超时在现网表现为 Flow 失败，已被 `failed` 覆盖）。

#### Scenario: 整月推送结束后无 pushing 残留

- **GIVEN** 某账期的整月推送已结束（成功、部分残留或失败任一情形）
- **WHEN** 查询该账期全部调账的 `push_status`
- **THEN** 取值仅可能为 `pushed`、`unpushed` 或 `failed`
- **AND** 不存在 `pushing`

#### Scenario: push_status 枚举仅四态

- **GIVEN** `push_status` 枚举
- **WHEN** 检查取值
- **THEN** 仅 `unpushed` / `pushing` / `pushed` / `failed` 四态
- **AND** 无「超时」态

### Requirement: failed 态放行调用方覆盖的联动保证

系统 SHALL 正确写入 `failed` 态，使写入域的「`failed` 放行覆盖重推」判定成立。这是有意设计：推送失败后 调用方必须能重新推送修正数据，否则该账期会被永久锁死。

#### Scenario: failed 调账不阻塞调用方重推

- **GIVEN** 某预付费单下调账推送失败停留 `failed`
- **WHEN** 调用方重推该单
- **THEN** 写入放行
- **AND** 本 capability 保证 `failed` 态被正确写入以支撑该放行判定

### Requirement: 存量数据不被推送打点误伤

系统 SHALL 保证存量调账（已刷 `push_status=pushed`）在 SyncController 正常创建当期 Flow 时不被重新置为 `pushing`。打点只在 SyncController 主动创建该账期 Flow 时发生。运营手动重触发历史月同步为已知例外。

#### Scenario: 历史账期存量记录不被重新置为推送中

- **GIVEN** 存量调账已刷 `push_status=pushed`
- **WHEN** SyncController 正常创建当期 Flow
- **THEN** 历史账期的存量记录不被重新置为 `pushing`

### Requirement: gpu_type 未命中 OBS 清单时的下游行为

系统对 `res_sub_class`（取自 `gpu_type`）不在厂商卡型清单内的 prepaid 调账 SHALL 在 OBS 同步时按现网 `splitAdjustmentResSubClass` 的 default 分支行为处理：GPU 卡型字段为空串且同步过程不报错。这是 D-13 的残留下游行为，不是软校验放行。

#### Scenario: 清单外 gpu_type 在 OBS 侧静默为空且不报错

- **GIVEN** 一条 `res_class=gpu_card` 且 `res_sub_class` 不在厂商卡型清单内的 prepaid 调账
- **WHEN** 整月 OBS 同步执行
- **THEN** OBS 侧该行的 GPU 卡型字段为空串
- **AND** 同步过程不报错

### Requirement: data-service 批量更新推送状态接口

系统 SHALL 在 data-service 提供调账 `push_status` 与 `push_fail_reason` 的批量更新接口，供 account-server 的 SyncController 与 task-server 的推送 Flow 调用。批量操作 MUST 按 `constant.BatchOperationMaxLimit` 分片。除 data-service 外的服务 MUST NOT 直接操作 DB。

#### Scenario: 批量置位分片执行且无性能劣化

- **GIVEN** 单账期调账条目为现网量级
- **WHEN** 执行推送前置位与推送后回写
- **THEN** 均按 `BatchOperationMaxLimit` 分片批量执行，不出现单条循环更新
- **AND** 整月同步总耗时相比改造前无显著劣化
