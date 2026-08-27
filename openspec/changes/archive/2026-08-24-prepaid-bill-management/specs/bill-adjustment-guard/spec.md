## ADDED Requirements

### Requirement: 调账写守卫拆分为来源守卫与状态守卫

系统 SHALL 将现网 `checkAdjustmentUnconfirmed` 拆分为两类守卫：**来源守卫**判定 `source`，`source=prepaid` 一律拒绝编辑与删除；**状态守卫**判定 `push_status=pushing` 拒绝、`settle_state=settled` 拒绝，且仅用于编辑与删除路径；批量确认路径只保留「已确认拒绝」的现网行为，MUST NOT 接入状态守卫。守卫拆分 MUST 覆盖 `checkAdjustmentUnconfirmed` 现有的全部 4 处调用点，MUST NOT 遗漏任何调用点。守卫判定 MUST 复用已有 list 查询结果，MUST NOT 新增额外的 DB 查询。

**改造后的守卫矩阵**

| 操作 | `source=prepaid` | `push_status=pushing` | `settle_state=settled` | `state=confirmed` |
|---|---|---|---|---|
| 编辑 | 拒绝 | 拒绝 | 拒绝 | 放行（相对现网为行为变化） |
| 删除 / 批量删除 | 拒绝 | 拒绝 | 拒绝 | 放行（相对现网为行为变化） |
| 批量确认 | 拒绝 | — | — | 拒绝（现网行为保留） |

#### Scenario: 4 处调用点行为逐点符合守卫矩阵

- **GIVEN** 守卫已拆分
- **WHEN** 逐一验证 `UpdateBillAdjustmentItem`、`BatchConfirmBillAdjustmentItem`、`DeleteBillAdjustmentItem`、`BatchDeleteBillAdjustmentItem` 四处调用点
- **THEN** 各调用点行为与守卫矩阵逐条一致
- **AND** 无遗漏的调用点

#### Scenario: 守卫判定不新增 DB 查询

- **GIVEN** 守卫新增 `source` / `push_status` / `settle_state` 三字段判定
- **WHEN** 调用编辑、删除、批量确认接口
- **THEN** 相比改造前不新增额外的 DB 查询，判定复用已有 list 查询结果
- **AND** 响应时间无显著劣化

### Requirement: prepaid 来源调账禁止人工写操作

系统 SHALL 对 `source=prepaid` 的调账拒绝一切人工编辑、删除与批量确认操作，返回参数错误且记录零变更。该拒绝 MUST 无条件生效，即使该条 `push_status=unpushed` 且 `settle_state=unsettled` 也 MUST 拒绝。

#### Scenario: prepaid 调账编辑与删除一律被拒

- **GIVEN** 一条 `source=prepaid` 的调账
- **WHEN** 运营分别调用编辑接口与删除接口
- **THEN** 两者均返回参数错误且记录零变更
- **AND** 即使该条 `push_status=unpushed`、`settle_state=unsettled` 也一律拒绝

#### Scenario: prepaid 调账不参与确认流转

- **GIVEN** 某条 `source=prepaid` 的调账
- **WHEN** 运营调用批量确认接口传入该 ID
- **THEN** 接口返回参数错误
- **AND** 该条记录字段零变更

### Requirement: 放开 manual 已确认调账的编辑与删除

系统 SHALL 允许 `source=manual` 的调账在 `push_status ≠ pushing` **且** `settle_state ≠ settled` 时被编辑与删除，包括 `state=confirmed` 的记录。编辑成功后 MUST 同步回退 `state` → `unconfirmed` 与 `push_status` → `unpushed`，使该条重新走「确认 → 推送」链路。

> ⚠️ **这是对 PRD v2.12 十三节「不涉及『账单调整』页手工录入/确认流程的改造」的显式偏离，不是实现 bug。**
> 偏离依据：用户 2026-08-20 对 Q-005 的决策（选 B）+ 技术方案 §10 第 2/3/4 条。
> 现网 `checkAdjustmentUnconfirmed` 对已确认记录一律拒绝编辑与删除；改造后编辑与删除两条路径放行，仅批量确认仍拒绝。
> 风险等级中高，四道缓解措施为：`settled` 守卫兜住存量、`pushing` 守卫防推送中改动、批量整批原子拒绝、上线前知会运营。详见 design.md 的「显式偏离 #1」。

#### Scenario: 未确认调账可编辑（现网行为不回归）

- **GIVEN** 一条 `source=manual`、`state=unconfirmed` 的调账
- **WHEN** 运营调用编辑接口
- **THEN** 编辑成功

#### Scenario: 已确认已推送调账可编辑且状态回退

- **GIVEN** 一条 `source=manual`、`state=confirmed`、`push_status=pushed`、`settle_state=unsettled` 的调账
- **WHEN** 运营调用编辑接口修改金额
- **THEN** 编辑成功（现网此场景为拒绝）
- **AND** 该条的 `state` 被同步回退为 `unconfirmed`，`push_status` 被同步回退为 `unpushed`

#### Scenario: 已确认调账可删除

- **GIVEN** 一条 `source=manual`、`state=confirmed`、`push_status=pushed`、`settle_state=unsettled` 的调账
- **WHEN** 运营调用删除接口
- **THEN** 删除成功

#### Scenario: 三类守卫命中时编辑与删除均被拒

- **GIVEN** 三条 `source=manual`、`state=confirmed` 的调账，分别为 `push_status=pushing`、`settle_state=settled`、`source` 改为 `prepaid`
- **WHEN** 逐一调用编辑与删除接口
- **THEN** 三种情况全部被拒绝
- **AND** 记录零变更

### Requirement: 批量操作整批原子拒绝

系统的批量删除 SHALL 在 ID 列表中存在任一被守卫拒绝的记录时**整批拒绝**，列表中其余可删记录 MUST NOT 被删除。系统 MUST NOT 允许部分成功。

#### Scenario: 混合 ID 列表整批拒绝零变更

- **GIVEN** 一个批量删除请求的 ID 列表中混入 1 条 `settle_state=settled` 的记录
- **WHEN** 调用批量删除
- **THEN** 整批拒绝
- **AND** 列表中其余可删记录也不被删除，不允许部分成功

### Requirement: 批量确认行为保持不变

系统的批量确认 SHALL 保持现网行为：`state=confirmed` 的记录 MUST 仍被拒绝（不能重复确认）。守卫拆分 MUST NOT 误放开此路径 —— 这是守卫拆分最易出错的一点。

批量确认路径 MUST NOT 判定 `push_status` 与 `settle_state`（守卫矩阵中该行为 `—`）：存量调账已刷 `settled`，一旦把 `settled` 守卫接进确认路径，存量待确认调账将永久不可确认，且 `settled` 单向不可逆没有恢复手段。批量确认只判定 `source`（新增，拒绝 `prepaid`）与 `state`（现网行为保留）。

#### Scenario: 已定账调账仍可被批量确认

- **GIVEN** 一条 `source=manual`、`state=unconfirmed`、`settle_state=settled` 的调账
- **WHEN** 运营调用批量确认接口传入该 ID
- **THEN** 确认成功，行为与现网一致

#### Scenario: 已确认调账批量确认仍被拒

- **GIVEN** 一条 `source=manual`、`state=confirmed` 的调账
- **WHEN** 运营调用批量确认接口传入该 ID
- **THEN** 仍返回拒绝，不能重复确认
- **AND** 守卫拆分未误放开此路径

### Requirement: 存量历史数据保护

系统 SHALL 通过 `settled` 守卫保证本次放开不波及现网历史数据。存量调账已刷 `settle_state=settled`，因此历史账期的已确认调账 MUST 仍不可编辑不可删除。这是本次放开风险可控的关键前提，MUST 专项验证。

#### Scenario: 历史已确认调账被 settled 守卫拒绝

- **GIVEN** 存量调账已刷 `settle_state=settled`
- **WHEN** 运营尝试编辑或删除任意一条历史已确认调账
- **THEN** 被 `settled` 守卫拒绝
- **AND** 本次放开不波及现网历史数据

### Requirement: 编辑回退的连带影响不产生重复行

系统 SHALL 保证被编辑后回退为 `unconfirmed` + `unpushed` 的调账在重新确认并触发整月同步后，OBS 侧该账期数据经 clean 加全量插入后不出现重复行，金额为编辑后的新值。

#### Scenario: 回退后重新推送 OBS 无重复行

- **GIVEN** 一条被编辑后回退为 `unconfirmed` + `unpushed` 的调账
- **WHEN** 运营重新确认该条并触发该账期整月同步
- **THEN** OBS 侧该账期数据经 clean 加全量插入后不出现重复行
- **AND** 金额为编辑后的新值

### Requirement: 调账列表响应体新增三字段

系统的账单调整 list 接口响应体 SHALL 为每条记录新增 `push_status`、`settle_state`、`source` 三个字段，供前端新增两列与隐藏 prepaid 行的操作入口。老字段结构与语义 MUST 保持不变，为向后兼容的新增字段。

#### Scenario: list 响应含三个新字段且向后兼容

- **GIVEN** 调账 list 接口已改造
- **WHEN** 调用该接口
- **THEN** 响应体每条记录均含 `push_status` / `settle_state` / `source` 三个字段且取值正确
- **AND** 老字段结构与语义不变

### Requirement: 编辑删除的 IAM 鉴权不受守卫改造影响

系统的调账编辑与删除 SHALL 继续使用现网 `AccountBill` + `Update` / `Delete` 鉴权，守卫改造 MUST NOT 影响权限判定路径。

> 这是蓝鲸 IAM（权限中心）集成点：鉴权 action 复用现网定义，本 capability 不新增 action。

#### Scenario: 无权限时仍返回权限错误

- **GIVEN** 用户无 `AccountBill + Update` 权限
- **WHEN** 调用编辑接口
- **THEN** 返回权限错误
- **AND** 用户无 `AccountBill + Delete` 权限时删除接口同理返回权限错误

### Requirement: 人工路径 res_sub_class 硬校验保持不变

系统的人工录入路径 SHALL 继续对 `res_class` / `res_sub_class` 执行现网 `validateResSubClass` 的硬校验（`gpu_card` / `gpu_api` 下必填且须命中该厂商卡型清单，`cpu` / `gpu_other` 下必须为空，违反返回 `errf.InvalidParameter`）。本期 MUST NOT 因预付费路径改为服务端填充 `gpu_card` + `gpu_type` 而放松人工路径。预付费写入不经过 `validateResSubClass`。

#### Scenario: 人工路径提交清单外卡型仍被拒

- **GIVEN** 一个 `res_class=gpu_card` 且 `res_sub_class` 不在该厂商卡型清单内的取值
- **WHEN** 由人工创建接口提交
- **THEN** 返回 `errf.InvalidParameter` 被拒
- **AND** 预付费路径以同一字符串作为 `gpu_type` 写入时落库成功（不走人工硬校验）
