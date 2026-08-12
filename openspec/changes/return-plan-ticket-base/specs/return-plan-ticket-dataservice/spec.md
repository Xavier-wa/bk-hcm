## ADDED Requirements

### Requirement: 退回计划单据数据模型

系统 SHALL 新增退回计划主单表 `return_plan_ticket` 与子单表 `return_plan_sub_ticket`，采用主单 + 子单两表结构。主单 MUST 内含 `status` 与 `message` 字段（不单独设状态表）；子单 MUST 与 CRP 退回计划单一一对应并记录 `crp_sn`/`crp_url`。系统 MUST NOT 新增退回计划明细本地表，退回计划明细以 CRP 为准。

#### Scenario: 主单表结构

- **WHEN** 创建退回计划主单
- **THEN** 主单记录包含单据类型（add/adjust/cancel）、明细 JSON（仅用于展示与拆单）、提单人、业务与组织维度（bk_biz、运营产品、规划产品、虚拟部门）、`status`、`message`、备注、提单时间及审计字段

#### Scenario: 子单表结构

- **WHEN** 创建退回计划子单
- **THEN** 子单记录包含父单 ID、子单类型（add/cancel）、子单明细 JSON、拆单维度（业务与组织维度、项目类型、资源池）、`status`、`crp_sn`/`crp_url`、`message`、提单时间及审计字段

#### Scenario: 不落退回计划明细本地表

- **WHEN** 退回计划单据完成流转
- **THEN** HCM 本地仅存在主单与子单及其流转状态，不存在退回计划明细的本地持久化表

### Requirement: 退回计划单据 data-service CRUD 接口

系统 SHALL 通过 data-service 提供主单/子单两表的 CRUD 原子接口，并在 `pkg/client/data-service` 中封装对应 client。非 data-service 服务 MUST NOT 直连数据库，所有单据读写 MUST 经 data-service。

#### Scenario: 通过 data-service 读写单据

- **WHEN** woa-server 需要创建、更新、查询主单或子单
- **THEN** 通过 data-service client 调用对应原子接口完成，不直接操作数据库

#### Scenario: 子单状态更新

- **WHEN** 调度器推进子单状态或写入 CRP 单号
- **THEN** 经 data-service 更新子单 `status`、`crp_sn`/`crp_url`、`message` 等字段
