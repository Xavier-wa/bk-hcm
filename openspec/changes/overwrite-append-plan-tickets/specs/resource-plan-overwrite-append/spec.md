## ADDED Requirements

### Requirement: 资源预测覆盖追加接口

系统 SHALL 提供业务视角的资源预测覆盖追加接口 `POST /api/v1/woa/bizs/{bk_biz_id}/plans/resources/tickets/overwrite_append`，在同一资源预测主单内先按筛选条件覆盖（删除）已有预测、再追加新的预测明细，复用现有 `res_plan_ticket` 的拆单、调度与 CRP 提单链路。接口 SHALL 校验业务-资源预测操作权限（`biz_resource_plan_operate`）。接口 SHALL 异步提单，快速返回资源预测主单 ID（`data.id`），P95 < 1s。接口路径与命名 MUST NOT 包含 finops 字样。

#### Scenario: 仅追加提单（覆盖开关关闭）

- **WHEN** 请求携带结构化预测明细 `demands` 且 `overwrite` 为 `false`（或不传）
- **THEN** 系统复用 `res_plan_ticket` 逻辑创建 `type=add` 主单并进入提单链路，流转完成后落本地 `res_plan_demand`，接口返回主单 ID

#### Scenario: 接口命名不含 finops

- **WHEN** 检查接口路径、请求/响应字段命名
- **THEN** 均不出现 finops 字样

### Requirement: 资源预测覆盖能力

当 `overwrite` 为 `true` 时，系统 SHALL 按 业务（`bk_biz_id`）+ 项目类型（`obs_projects`）+ 技术分类（`technical_classes`，不传时匹配全部）+ 期望交付时间范围（`expect_time_range`，按 `expect_time` 筛选）匹配本地 `res_plan_demand`，并将命中的明细转为 `cancel` 条目（`Original` 填命中明细数据），走 CRP 删除审批流程。`overwrite` 为 `true` 时 `overwrite_filter` MUST 必填，其中 `obs_projects` 与 `expect_time_range` MUST 必填。

#### Scenario: 按筛选条件覆盖已有预测并追加

- **WHEN** `overwrite` 为 `true`、携带 `overwrite_filter` 与追加 `demands`
- **THEN** 系统先按 业务 + 项目类型 + 技术分类 + `expect_time` 范围匹配本地 `res_plan_demand` 生成 `cancel` 条目，再将追加明细生成 `add` 条目，二者合并进同一 `type=adjust` 主单

#### Scenario: 覆盖筛选无技术分类时匹配全部

- **WHEN** `overwrite` 为 `true` 且 `overwrite_filter.technical_classes` 为空
- **THEN** 系统按 业务 + 项目类型 + 时间范围匹配全部技术分类的本地预测明细

### Requirement: 资源预测仅覆盖不新增模式

系统 SHALL 支持仅覆盖不新增模式：当 `overwrite` 为 `true` 且不传 `demands` 时，仅执行筛选删除、不追加新预测。

#### Scenario: 仅覆盖不新增

- **WHEN** `overwrite` 为 `true` 且未携带 `demands`
- **THEN** 系统仅将命中明细转为 `cancel` 条目提单（`type=cancel` 主单），不产生 `add` 条目

### Requirement: 资源预测覆盖追加参数校验

系统 SHALL 校验入参合法性：当 `overwrite` 为 `false`（或不传）且未携带 `demands` 时，视为参数非法并返回参数错误；携带 `demands` 时 `demand_class` MUST 必填。

#### Scenario: 无覆盖且无追加明细

- **WHEN** `overwrite` 为 `false` 且未携带 `demands`
- **THEN** 系统返回参数错误（`InvalidParameter`）

### Requirement: 资源预测跳过 ITSM 审批

系统 SHALL 支持 `skip_itsm` 开关：当 `skip_itsm` 为 `true` 时，跳过 ITSM 审批，主单直接进入拆单阶段。

#### Scenario: 跳过 ITSM 直接拆单

- **WHEN** 请求携带 `skip_itsm=true` 并提单
- **THEN** 系统不创建 ITSM 审批单，主单直接进入拆单与 CRP 提单阶段

#### Scenario: 不跳过 ITSM 走正常审批

- **WHEN** 请求 `skip_itsm` 为 `false`（或不传）
- **THEN** 系统按现有资源预测提单流程创建 ITSM 审批单

### Requirement: 资源预测请求体提单人

系统 SHALL 支持请求体必填字段 `applicant` 覆盖调用账号（`kt.User`，可能为 admin）作为主单提单人。该改造 MUST 仅作用于覆盖追加接口，现有创建/调整/取消接口保持使用 `kt.User`。

#### Scenario: 使用请求体提单人提单

- **WHEN** 请求体携带 `applicant`
- **THEN** 主单 `applicant` 及后续 CRP 提单人为该字段值，而非调用账号 `kt.User`
