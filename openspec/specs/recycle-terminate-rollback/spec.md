# Spec: recycle-terminate-rollback

## Purpose

资源回收单在退回阶段调用公司侧接口失败时，失败的机器会滞留在业务的回收中转池——既不在业务可用范围内，
也没有被公司侧真正回收。本能力让「终止」操作在子单状态为 `RETURN_FAILED` 时，把这些机器退回业务空闲机
模块，替代此前依赖研发手工改库的恢复方式。

回滚失败会阻断终止，单据保持 `RETURN_FAILED` 供用户重试；回滚幂等，重复触发不会重复流转。

## Requirements

### Requirement: 终止操作按子单状态分流回滚

资源回收终止接口 SHALL 仅在子单状态为 `RETURN_FAILED`（退回失败）时触发机器回滚。其余任何状态的终止行为
MUST 与改造前完全一致，不触发任何 CMDB（蓝鲸配置平台）资源流转。

`RETURN_FAILED` 是唯一能证明「中转已成功、机器停留在业务回收中转池、公司侧回收失败」的状态。
`RETURN_PLAN_FAILED`（退回计划失败）语义上机器已被公司侧回收成功，MUST NOT 触发回滚。

#### Scenario: 退回失败子单触发回滚

- **GIVEN** 一个子单状态为 `RETURN_FAILED`
- **WHEN** 调用终止接口
- **THEN** 系统进入回滚流程，识别该子单下需要退回的机器

#### Scenario: 中转失败子单不触发回滚

- **GIVEN** 一个子单状态为 `TRANSIT_FAILED`
- **WHEN** 调用终止接口
- **THEN** 子单置为 `TERMINATE`
- **AND** 不发起任何 CMDB 资源流转调用

#### Scenario: 退回计划失败子单不触发回滚

- **GIVEN** 一个子单状态为 `RETURN_PLAN_FAILED`，其机器已被公司侧回收成功
- **WHEN** 调用终止接口
- **THEN** 子单置为 `TERMINATE`
- **AND** 不发起任何 CMDB 资源流转调用

#### Scenario: 其余可终止状态行为不变

- **GIVEN** 一个子单状态为 `DETECT_FAILED`、`REJECTED`、已提单或未提单之一
- **WHEN** 调用终止接口
- **THEN** 行为与改造前完全一致，子单置为 `TERMINATE` 且无任何机器流转

#### Scenario: 进行中状态仍拒绝终止

- **GIVEN** 一个子单状态为 `TRANSITING` 或 `RETURNING`
- **WHEN** 调用终止接口
- **THEN** 系统拒绝并返回不允许终止的原因
- **AND** 不执行任何回滚

#### Scenario: 终态子单拒绝重复终止

- **GIVEN** 一个子单状态为 `DONE` 或 `TERMINATE`
- **WHEN** 调用终止接口
- **THEN** 系统拒绝并返回原因
- **AND** 不产生任何 CMDB 流转

### Requirement: 按机器粒度识别回滚对象

系统 SHALL 在 `RETURN_FAILED` 子单下逐台判定机器状态，仅把状态为 `RETURN_FAILED` 的机器纳入回滚范围。
已被公司侧成功回收的机器（状态 `DONE`）MUST NOT 被操作。

#### Scenario: 同一子单内部分成功部分失败

- **GIVEN** 一个 `RETURN_FAILED` 子单下有 5 台机器，其中 2 台状态为 `DONE`、3 台状态为 `RETURN_FAILED`
- **WHEN** 调用终止接口
- **THEN** 仅 3 台失败机器进入回滚流程
- **AND** 2 台已回收成功的机器在 CMDB 中不发生任何变化

#### Scenario: 子单退回失败但无失败机器

- **GIVEN** 一个子单状态为 `RETURN_FAILED`，但其下没有任何机器处于 `RETURN_FAILED` 状态
- **WHEN** 调用终止接口
- **THEN** 待回滚清单为空，不发起任何 CMDB 流转
- **AND** 子单置为 `TERMINATE`

### Requirement: 回滚幂等，跳过已回到业务内置模块的机器

回滚 MUST 幂等。系统 SHALL 通过查询机器在 CMDB 中的实际位置来判定是否需要流转：已位于业务内置模块
（空闲机 `DftModuleIdle`、故障机 `DftModuleFault` 或待回收 `DftModuleRecycle`）的机器视为已回滚，
跳过且 MUST NOT 计为失败。

内置模块的 `default` 字段非 0，自定义模块（含回收中转池）为 0，因此「机器在任一内置模块」正是
「机器不在任何自定义模块」的精确判定。

判定依据是 CMDB 实际位置，而非 `recycle_host` 表中的机器状态。系统 MUST NOT 依赖回收中转池模块的
名称或 ID 做判定——该模块由公司侧 shipper 服务维护，海垒侧无其 ID。

#### Scenario: 重新触发终止时跳过已到位机器

- **GIVEN** 上一次终止时部分机器已成功退回空闲机、剩余机器未处理，子单仍为 `RETURN_FAILED`
- **WHEN** 用户再次调用终止接口
- **THEN** 已在空闲机的机器被跳过，不重复流转
- **AND** 剩余机器完成退回
- **AND** 子单置为 `TERMINATE`

#### Scenario: 驳回场景已自动回滚的机器视为已完成

- **GIVEN** 一个 `RETURN_FAILED` 子单的机器因退回单被驳回已被系统自动回滚至业务的待回收模块
- **WHEN** 调用终止接口
- **THEN** 这些机器被识别为已离开回收中转池并跳过，不判定为失败
- **AND** 不对其发起中转池到空闲机的转移调用
- **AND** 子单正常进入 `TERMINATE`

### Requirement: 失败机器退回业务空闲机

系统 SHALL 把待回滚机器从业务的回收中转池模块转移到该业务的空闲机模块。
云主机与物理机行为 MUST 一致。

受 CMDB 接口约束，转移 MUST 按每批最多 10 台分批调用。

回滚 MUST NOT 改动机器的维护人与备份维护人：回收与中转流程从不修改 CMDB 上的这两个字段，
只将其读出存档到回收机器记录用于展示，机器退回空闲机后其值本就保持原样。

#### Scenario: 失败机器成功退回空闲机

- **GIVEN** 一个 `RETURN_FAILED` 子单下 3 台机器退回失败且滞留在业务回收中转池
- **WHEN** 调用终止接口
- **THEN** 这 3 台机器在 CMDB 中回到所属业务的空闲机模块
- **AND** 子单状态变为 `TERMINATE`
- **AND** 接口返回成功

#### Scenario: 回滚不改动维护人

- **GIVEN** 一个 `RETURN_FAILED` 子单完成回滚
- **WHEN** 检查机器在 CMDB 中的维护人与备份维护人
- **THEN** 其值与回滚前一致
- **AND** 回滚过程未对这两个字段发起任何写操作

#### Scenario: 物理机裁撤子单同样生效

- **GIVEN** 一个物理机裁撤子单在退回阶段失败
- **WHEN** 调用终止接口
- **THEN** 失败机器同样被退回原业务的空闲机模块，行为与云主机一致

#### Scenario: 超过单批上限时分批转移

- **GIVEN** 一个 `RETURN_FAILED` 子单有 25 台机器需要回滚
- **WHEN** 调用终止接口
- **THEN** 系统按每批最多 10 台分 3 批调用 CMDB 转移接口

### Requirement: 回滚失败阻断终止

执行顺序 MUST 固定为「先回滚、后置终止态」。只有回滚全部成功（含跳过的机器）才允许把子单置为
`TERMINATE` 并同步更新关联的滚服回收记录与短租回收记录。

存在任何一台机器回滚失败时，子单状态 MUST 保持 `RETURN_FAILED` 不变，关联记录 MUST NOT 更新，
接口 SHALL 返回失败及具体原因。

#### Scenario: 回滚失败时单据不进入终止态

- **GIVEN** 一个 `RETURN_FAILED` 子单在回滚过程中 CMDB 调用失败
- **WHEN** 调用终止接口
- **THEN** 接口返回失败并携带失败原因
- **AND** 子单状态仍为 `RETURN_FAILED` 而非 `TERMINATE`
- **AND** 关联的滚服回收记录与短租回收记录状态均不变更

#### Scenario: 多子单请求中某个子单回滚失败

- **GIVEN** 一次终止请求包含 2 个 `RETURN_FAILED` 子单，第 1 个回滚成功、第 2 个回滚失败
- **WHEN** 调用终止接口
- **THEN** 接口返回失败
- **AND** 第 1 个子单已置为 `TERMINATE`
- **AND** 第 2 个子单保持 `RETURN_FAILED`

### Requirement: 回滚部分成功不做反向补偿

分批转移过程中若某批失败，已成功转移的批次 MUST NOT 被退回中转池。系统依靠回滚的幂等性支持用户重新
触发终止来推进剩余机器。

回滚成功后 MUST NOT 修改 `recycle_host` 的机器状态，机器状态保持 `RETURN_FAILED`。回滚结果 MUST NOT
写入单据字段，失败详情仅通过服务端日志排查。

#### Scenario: 中途批次失败不回退已成功批次

- **GIVEN** 一个 `RETURN_FAILED` 子单有 25 台机器需回滚，分批执行时第 2 批失败
- **WHEN** 调用终止接口
- **THEN** 接口返回失败且子单保持 `RETURN_FAILED`
- **AND** 第 1 批已到空闲机的机器不被退回中转池

### Requirement: 权限校验与错误可观测

终止接口的权限校验 MUST 与改造前一致：业务视角校验业务下资源回收权限，资源视角校验自研云资源回收权限。
无权限时 MUST 在执行任何回滚之前直接拒绝。

回滚失败时系统 SHALL 记录错误日志，日志内容 MUST 包含单据号、子单号、失败机器的固资编号与失败原因，
并遵循项目日志规范携带 rid。

#### Scenario: 无权限用户被拒绝

- **GIVEN** 一个无回收权限的用户
- **WHEN** 调用终止接口
- **THEN** 返回无权限错误
- **AND** 不执行任何回滚

#### Scenario: 回滚失败可从日志定位

- **GIVEN** 回滚过程中发生失败
- **WHEN** 查看服务端日志
- **THEN** 能检索到包含单据号、子单号、失败机器固资编号与失败原因的错误日志

### Requirement: 同步执行与响应时间

由于回滚失败需要阻断终止，回滚 MUST 同步执行，MUST NOT 异步化。待回滚机器数不超过 100 台时，
终止接口 SHALL 在 30 秒内返回。

#### Scenario: 百台规模内按时返回

- **GIVEN** 一个 `RETURN_FAILED` 子单有 100 台机器需要回滚
- **WHEN** 调用终止接口
- **THEN** 接口在 30 秒内返回结果
