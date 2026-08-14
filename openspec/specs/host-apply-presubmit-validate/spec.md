# Capability: host-apply-presubmit-validate

## Purpose

定义 woa-server 业务视角提单前只读校验接口：与人工提单同源的前置校验链、实时容量校验，以及结构化校验结果（区分业务不通过与系统异常）。

## Requirements

### Requirement: 提单前只读校验接口

woa-server SHALL 提供业务视角的提单前只读校验接口 `POST /api/v1/woa/bizs/{bk_biz_id}/task/check/apply`，入参复用 `types.ApplyReq`，`bk_biz_id` 取自 path 并覆盖请求体。该接口 MUST 只做校验，MUST NOT 落库、MUST NOT 创建 ITSM 工单、MUST NOT 产生任何副作用。

接口 MUST 与人工提单 `CreateBizApplyOrder` 执行同源的前置校验链，校验项按既有顺序为：结构校验（`input.Validate()`）→ 业务权限（`AuthorizeWithPerm(Biz, Create)`）→ 需求类型校验（机型/绿通/DA 前缀/裁撤可申请额度）→ 预测内/外余量校验 → GPU 计费时长校验 → 实时容量校验。

#### Scenario: 全部校验通过

- **GIVEN** 一个结构合法、用户有业务创建权限、需求类型与预测/GPU/容量均满足的申领请求
- **WHEN** 调用 `check/apply` 接口
- **THEN** 返回 HTTP 200 且结果为 `{pass: true, reason: ""}`
- **AND** 系统未写入任何提单/工单数据

#### Scenario: 结构校验不通过

- **GIVEN** 一个缺失必填字段或字段非法的申领请求
- **WHEN** 调用 `check/apply` 接口
- **THEN** 返回参数校验错误，调用方据此提示用户修正参数

#### Scenario: 无业务创建权限

- **GIVEN** 当前用户对目标 `bk_biz_id` 无主机申领创建权限
- **WHEN** 调用 `check/apply` 接口
- **THEN** 返回权限不足错误（IAM/BlueKing 权限中心鉴权失败）

### Requirement: 校验逻辑与人工提单同源

为避免"校验"与"创建"两条路径行为漂移，系统 SHALL 将 `createApplyOrder` 中的只读前置校验抽取为共享聚合方法，供创建路径与校验接口共同调用。GPU 计费时长校验 `VerifyCvmGPUChargeMonth` SHALL 被暴露为可在校验链复用的只读校验。抽取过程 MUST NOT 改变人工提单 `CreateBizApplyOrder` 的对外行为与异步生产流程。

#### Scenario: 创建与校验调用同一聚合校验

- **GIVEN** 同一份申领请求
- **WHEN** 分别经过创建路径与校验接口
- **THEN** 两者执行相同的需求类型校验与预测余量校验逻辑，对相同输入给出一致的通过/拒绝判定

#### Scenario: 校验前完成机型信息填充

- **GIVEN** 一个 CVM 类子单，机型族/核数尚未填充
- **WHEN** 进入 GPU 计费时长校验与容量校验之前
- **THEN** 系统先完成机型信息填充（与底层创建路径一致的顺序），再执行后续校验

### Requirement: 实时容量校验

校验接口 SHALL 新增实时容量校验，逐 CVM 类子单按 `spec`（region/zone/device_type/vpc/subnet/charge_type/require_type）构造容量查询参数，调用 `Capacity().GetCapacity`（底层为 CRP/云梯实时 `QueryCvmCapacity`）获取各可用区 `MaxNum`。容量查询 MUST 为只读（`DisableUpsertDB=true`，不写库存快照）。是否纳入预测口径与子单需求类型一致（`IgnorePrediction = !requireType.NeedVerifyResPlan()`）。

容量判定口径 MUST 与现有异步生产（`generator`）一致：

- 需求类型满足 `NotNeedVerifyCapacity()`（如小额绿通）→ 跳过容量校验。
- 单可用区 → 该 zone `MaxNum >= replicas`。
- 分 Campus / 多可用区（`zone == CvmSeparateCampus` 或候选 zone 数量大于 1）→ `Σ(候选 zone MaxNum) >= replicas`。

#### Scenario: 单可用区容量充足

- **GIVEN** 子单指定单一可用区且 `replicas` 不超过该 zone 实时 `MaxNum`
- **WHEN** 执行容量校验
- **THEN** 该子单容量校验通过

#### Scenario: 容量不足

- **GIVEN** 子单需求台数超过可用容量（单区 `MaxNum` 或多区 `MaxNum` 之和）
- **WHEN** 执行容量校验
- **THEN** 返回 `{pass: false, reason}`，原因说明可申领容量不足、需要 X 台、可用 Y 台

#### Scenario: 分 Campus 多可用区按各 zone 求和

- **GIVEN** 子单为分 Campus 或指定多个候选可用区
- **WHEN** 执行容量校验
- **THEN** 以各候选 zone 实时 `MaxNum` 之和作为有效容量与 `replicas` 比对

#### Scenario: 免容量校验的需求类型

- **GIVEN** 子单需求类型满足 `NotNeedVerifyCapacity()`
- **WHEN** 执行容量校验
- **THEN** 跳过容量校验，不调用 CRP 实时接口

### Requirement: 结构化校验结果

校验接口 SHALL 返回结构化结果类型 `{pass: bool, reason: string}`，以区分"业务不通过"与"系统异常"：业务校验未通过 MUST 返回 HTTP 200 且 `pass=false` 并携带可读 `reason`；下游/系统异常（DB、CRP、预测服务调用失败等）MUST 以 error 形式返回，不得伪装为业务不通过。聚合校验内部 SHALL 将业务类失败（如预测校验失败、参数非法、容量不足）归一为 `{pass:false, reason}`，其余异常透出为 error。

#### Scenario: 业务不通过返回 200 与原因

- **GIVEN** 预测余量不足或容量不足等业务约束未满足
- **WHEN** 调用 `check/apply` 接口
- **THEN** 返回 HTTP 200 且 `{pass: false, reason}`，调用方据 `reason` 提示用户调整规格或数量

#### Scenario: 系统异常返回 error

- **GIVEN** CRP 实时容量接口或预测服务调用失败
- **WHEN** 调用 `check/apply` 接口
- **THEN** 返回 error（非 `pass=false`），调用方据此提示系统异常、可重试
