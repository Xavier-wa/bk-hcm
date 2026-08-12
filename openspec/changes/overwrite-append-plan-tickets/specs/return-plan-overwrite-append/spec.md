## ADDED Requirements

### Requirement: 退回计划覆盖追加接口

系统 SHALL 提供业务视角的退回计划覆盖追加接口 `POST /api/v1/woa/bizs/{bk_biz_id}/plans/returns/tickets/overwrite_append`，在同一退回计划主单内先按筛选条件覆盖（删除）已有退回计划、再追加新的退回计划明细，复用 `return_plan_ticket` 的拆单、调度与 CRP 提单链路。系统 SHALL 在 woa-server 侧新增退回计划主单创建入口（基础子需求仅在 data-service 层提供创建能力）。HCM 本地仅持久化单据与流转状态，MUST NOT 持久化退回计划明细（明细以 CRP 为准）。接口 SHALL 异步提单，快速返回退回计划主单 ID，P95 < 1s。接口路径与命名 MUST NOT 包含 finops 字样。

#### Scenario: 仅追加退回计划（覆盖开关关闭）

- **WHEN** 请求携带结构化退回计划明细 `return_details` 且 `overwrite` 为 `false`
- **THEN** 系统经 `GetBizOrgRel(bk_biz_id)` 转换后创建 `type=add` 主单并进入提单链路，经 `submitAppendOrder` 向 CRP 新增，本地不落退回计划明细

#### Scenario: 业务转组织维度

- **WHEN** 创建退回计划主单
- **THEN** 系统调用 `GetBizOrgRel(bk_biz_id)` 得到 部门（虚拟部门）+ 规划产品 + 运营产品 并写入主单组织字段

### Requirement: 退回计划覆盖能力

当 `overwrite` 为 `true` 时，系统 SHALL 按 业务（转运营产品）+ 项目类型（`obs_projects`）+ 技术分类（`technical_classes`，不传时匹配全部）+ 计划退回时间范围（`plan_time_range`，按预计退回时间）经 CRP `queryReturnPlanItem` 查得待删除退回计划条目的 id 列表，构造为 `cancel` 条目（`Original` 含 `crp_plan_id`），经 `submitAdjustOrderForApi`（删除模式，`src=[{id}]`、`update=[]`）删除。`overwrite` 为 `true` 时 `overwrite_filter` MUST 必填，其中 `obs_projects` 与 `plan_time_range` MUST 必填。

#### Scenario: 按筛选条件覆盖已有退回计划

- **WHEN** `overwrite` 为 `true`、携带 `overwrite_filter`
- **THEN** 系统经 `queryReturnPlanItem` 按 业务(→运营产品) + 项目类型 + 技术分类 + 计划退回时间范围查得 id 后，构造 `cancel` 条目经 `submitAdjustOrderForApi` 删除模式提单

#### Scenario: 覆盖与追加拆分为不同子单

- **WHEN** `overwrite` 为 `true` 且携带 `return_details`
- **THEN** 覆盖删除条目与追加新增条目合并进同一 `type=adjust` 主单，拆单时删除条目（`cancel`）与新增条目（`add`）分属不同子单，分别对应 CRP 删除单与新增单

### Requirement: 退回计划仅覆盖不新增模式

系统 SHALL 支持仅覆盖不新增模式：当 `overwrite` 为 `true` 且不传 `return_details` 时，仅执行筛选删除、不追加新退回计划。

#### Scenario: 仅覆盖不新增

- **WHEN** `overwrite` 为 `true` 且未携带 `return_details`
- **THEN** 系统仅将命中的 CRP 退回计划条目构造为 `cancel` 条目提单，不产生 `add` 条目

### Requirement: 退回计划覆盖追加参数校验

系统 SHALL 校验入参合法性：当 `overwrite` 为 `false`（或不传）且未携带 `return_details` 时，视为参数非法并返回参数错误。追加明细中 `instance_model`+`cvm_amount` 与 `instance_type`+`core_type_name`+`core_amount` MUST 二选一。

#### Scenario: 无覆盖且无追加明细

- **WHEN** `overwrite` 为 `false` 且未携带 `return_details`
- **THEN** 系统返回参数错误（`InvalidParameter`）

### Requirement: 退回计划默认退回原因大类

当追加明细 `return_reason_class` 为空时，系统 SHALL 使用默认退回原因大类「成本优化&利用率提升」。

#### Scenario: 退回原因大类为空使用默认值

- **WHEN** 追加明细 `return_reason_class` 为空并提单
- **THEN** 系统以默认值「成本优化&利用率提升」作为该条目退回原因大类提交 CRP

### Requirement: 退回计划请求体提单人

系统 SHALL 支持请求体必填字段 `applicant` 作为 CRP 提单人，覆盖当前调用账号（如 admin）。

#### Scenario: CRP 提单人为请求体提单人

- **WHEN** 请求体携带 `applicant` 并向 CRP 提单
- **THEN** CRP 提单人为该字段值，而非调用账号 `kt.User`

### Requirement: CRP 退回计划明细查询扩展技术分类筛选

系统 SHALL 扩展 CRP `queryReturnPlanItem`（Go 方法 `QueryReturnPlan`）请求参数，新增 `technicalClass` 列表字段，用于退回计划覆盖按技术分类筛选待删除条目。

#### Scenario: 携带技术分类查询 CRP 退回计划

- **WHEN** 退回计划覆盖筛选携带 `technical_classes`
- **THEN** 系统将其作为 `technicalClass` 列表参数传入 `queryReturnPlanItem` 查询命中条目

#### Scenario: 不携带技术分类时匹配全部

- **WHEN** 退回计划覆盖筛选未携带 `technical_classes`
- **THEN** 系统不传 `technicalClass` 参数，按其余条件匹配全部技术分类的退回计划条目
