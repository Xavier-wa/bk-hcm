## ADDED Requirements

### Requirement: 退回计划单据列表与详情接口

系统 SHALL 提供退回计划主单列表、主单详情、子单列表、子单详情查询接口（业务视角，woa-server 入口经内部微服务调用 data-service）。列表接口 MUST 支持分页与常用条件过滤，详情接口 MUST 返回单据基本信息、流转状态与明细/CRP 单号信息。

#### Scenario: 查询主单列表

- **WHEN** 按单据 ID / 状态 / 类型 / 提单人 / 提单时间范围等条件分页查询主单列表
- **THEN** 返回匹配的主单列表，含状态、类型、组织维度、提单时间等字段，支持 count 与详情两种查询模式

#### Scenario: 查询主单详情

- **WHEN** 按主单 ID 查询详情
- **THEN** 返回主单基本信息、状态信息（含失败 `message`）及退回计划明细列表（含 add/cancel 两类条目）

#### Scenario: 查询子单列表与详情

- **WHEN** 按主单 ID 查询子单列表，或按子单 ID 查询子单详情
- **THEN** 返回子单类型、状态、项目类型、资源池、`crp_sn`/`crp_url`、`message` 等流转信息

### Requirement: 退回计划重试与终止接口

系统 SHALL 提供退回计划重试与终止接口，权限为业务-退回计划操作。重试 MUST 仅对失败态子单重新发起 CRP 提单流转。终止 MUST 将失败态单据置为终止态且终止后不可再重试，并 MUST 仅变更 HCM 本地单据状态、不撤回已提交至 CRP 的单据。

#### Scenario: 重试失败单据

- **WHEN** 对含失败子单的主单调用重试接口
- **THEN** 失败子单重新发起 CRP 提单流转，成功子单不受影响

#### Scenario: 终止失败单据

- **WHEN** 对失败态单据调用终止接口
- **THEN** 单据状态置为终止态，不可再重试，且不向 CRP 发起撤单

### Requirement: 退回原因大类查询接口

系统 SHALL 提供按 OBS 项目类型查询退回原因大类的接口，代理 CRP 的 `getReasonClassByObsProject` 能力，供提交退回计划时选择 `return_reason_class`。

#### Scenario: 按项目类型查询退回原因大类

- **WHEN** 传入 OBS 项目类型调用退回原因大类查询接口
- **THEN** 返回该项目类型下可选的退回原因大类列表

### Requirement: 创建接口异步返回

面向创建退回计划主单的接口 SHALL 在创建主单后快速返回主单号（P95 < 1s），实际的 CRP 提单与状态轮询 MUST 由后台调度异步完成。

#### Scenario: 创建后异步流转

- **WHEN** 创建退回计划主单
- **THEN** 接口在 P95 < 1s 内返回主单号，CRP 提单与轮询由后台调度器异步推进
