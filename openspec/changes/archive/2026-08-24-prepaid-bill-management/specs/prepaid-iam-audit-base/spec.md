## ADDED Requirements

### Requirement: IAM 预付费账单资源类型与 action

系统 SHALL 在 `pkg/iam/meta/resource.go` 新增资源类型常量 `AccountBillPrepaid`，并在 `pkg/iam/sys/initial_actions.go` 新增写入 action `AccountBillPrepaidCreate` 与删除 action `AccountBillPrepaidDelete`，二者均关联 `mainaccountResource`（参照现网 `MainAccountFind`）。两个 action MUST 同时挂入 `pkg/iam/sys/initial_action_groups.go` 的「平台管理 / 云账单管理」分组（与 `AccountBillPull` / `AccountBillManage` 同组）；MUST NOT 只注册 action 而不入组，否则权限中心会将其归入「未分类」。权限 MUST 只做实例级，MUST NOT 设置菜单级权限，权限维度 MUST 统一为二级账号。写入与删除 MUST 使用独立 action，MUST NOT 复用同一权限点。

> 这是蓝鲸 IAM（权限中心）集成点：新增资源类型与 action 需在服务启动时同步至权限中心。Action 与 Action Group 是两份独立清单，漏写分组会导致权限点可见但落在「未分类」。

#### Scenario: 权限中心可见新资源类型与 action

- **GIVEN** `pkg/iam/meta/resource.go` 已新增 `AccountBillPrepaid` 且 `pkg/iam/sys/initial_actions.go` 已新增 action 并关联 `mainaccountResource`
- **WHEN** 启动服务并同步 IAM 模型
- **THEN** 权限中心可见该资源类型与 action
- **AND** 资源维度为二级账号

#### Scenario: 预付费写入与删除 action 挂在云账单管理分组

- **GIVEN** `AccountBillPrepaidCreate` 与 `AccountBillPrepaidDelete` 已在 `initial_actions.go` 注册
- **WHEN** 检查 `GenerateStaticActionGroups` 的「平台管理 / 云账单管理」分组
- **THEN** 该分组的 Actions 含 `AccountBillPrepaidCreate` 与 `AccountBillPrepaidDelete`
- **AND** 权限中心不将这两个权限点归入「未分类」

### Requirement: auth-server 预付费鉴权资源规则生成

系统 SHALL 在 `cmd/auth-server/service/auth/gen_id.go` 新增 `genAccountBillPrepaidRuleResource`，资源类型取 `sys.MainAccount`，按 `a.ResourceID` 填充实例（参照现网 `genMainAccountRuleResource`）。`meta.Create` MUST 映射到 `AccountBillPrepaidCreate`，且 `ResourceID` 为空时 MUST 返回参数错误，MUST NOT 生成空实例规则。`meta.Delete` MUST 映射到独立的 `AccountBillPrepaidDelete`；`ListAuthorizedInstances` 按 action 拉已授权实例、本身不带实例 ID，因此删除路径的 `ResourceID` 允许为空。

#### Scenario: 按 ResourceID 生成二级账号实例规则

- **GIVEN** 鉴权请求携带 `ResourceID` 为二级账号 M1
- **WHEN** auth-server 走 `genAccountBillPrepaidRuleResource`
- **THEN** 生成的资源实例规则 `type=sys.MainAccount`
- **AND** 实例 ID 为 M1

#### Scenario: Create 且 ResourceID 为空时拒绝而非误放行

- **GIVEN** 鉴权请求的 Action 为 `Create` 且 `ResourceID` 为空
- **WHEN** 走 `genAccountBillPrepaidRuleResource`
- **THEN** 返回参数错误
- **AND** 不生成空实例规则，不因实例为空而误放行

#### Scenario: Delete 映射到独立删除 action 且允许空 ResourceID

- **GIVEN** 鉴权请求的 Action 为 `Delete` 且 `ResourceID` 为空
- **WHEN** 走 `genAccountBillPrepaidRuleResource`
- **THEN** 返回 `AccountBillPrepaidDelete`
- **AND** 不复用 `AccountBillPrepaidCreate`

### Requirement: 查询侧复用现网权限不新增 action

系统的预付费查询链路 SHALL 复用现网 `ListAuthorizedInstances(MainAccount, Find)` 做行级隔离，MUST NOT 新增任何查询类 action。新增资源类型与 action 仅用于写入与删除侧鉴权。

#### Scenario: 查询侧无新增 action

- **GIVEN** 查询侧未新增 action
- **WHEN** 检查列表查询与整合明细查询的鉴权调用
- **THEN** 一律使用现网 `ListAuthorizedInstances(MainAccount, Find)`
- **AND** 不存在新增的查询类 action

### Requirement: 审计资源类型新增并注册

系统 SHALL 在 `pkg/criteria/enumor/audit.go` 新增 audit `res_type` 常量 `account_bill_prepaid_item`，并 MUST 同时将其注册进 `AuditResourceTypeEnums`。未注册会导致 `AuditTable.CreateValidate` 的 `ResType.Exist()` 校验报 `resource type not support` —— 现网 `CloudCvmAuditResType` 即为「已定义但未注册」的反例，本要求为该坑的防回归约束。

#### Scenario: 新 res_type 通过 Exist 与 CreateValidate 校验

- **GIVEN** 新增 res_type `account_bill_prepaid_item`
- **WHEN** 调用 `ResType.Exist()`
- **THEN** 返回 true，即该常量已追加进 `AuditResourceTypeEnums`
- **AND** 构造一条该 res_type 的 audit 记录并调用 `AuditTable.CreateValidate` 时不返回 `resource type not support`

### Requirement: 同步审计记录构造

系统 SHALL 提供同步审计记录的构造能力，产出的 audit 记录字段取值为：`res_type` = `account_bill_prepaid_item`；`res_id` = 预付费账单主表 `id`（与 audit 表 `varchar(64)` 对齐）；`action` 首次写入为 `create`、覆盖重推为 `update`、批量删除为 `delete`；`operator` = `kit.User`（APIGW JWT 解析出的调用方）；`source` = `kt.GetRequestSource()`；`vendor` / `account_id` 分别为云厂商与 `main_account_id`；`bk_biz_id` 不填取默认值 -1。sync 的 `detail.data` MUST 含本次生成的 N+1 条调账清单（调账 ID、账期、`type`、金额）与主数据关键字段（`uuid`、订单月份、优惠后总价、币种）。`detail.changed` 在覆盖重推或删除时 MUST 含被整组删除的旧调账清单，首次写入时为空。`res_name` 与 `cloud_res_id` MUST 留空，调用方的 `uuid` MUST 放在 `detail` 中而非借用 `cloud_res_id` 的「云上资源ID」语义。删除审计按主单各写一条，`detail.data` 含 `uuid`。

#### Scenario: 首次写入构造 create 审计

- **GIVEN** 首次写入场景的输入（主数据 + 4 条新调账、无旧调账）
- **WHEN** 调用审计构造函数
- **THEN** 产出记录 `action=create`、`res_type=account_bill_prepaid_item`、`res_id` 为预付费 ID、`operator` 为 `kit.User`、`source` 为 `kt.GetRequestSource()`、`bk_biz_id` 为 -1
- **AND** `detail.data` 含 4 条调账清单与主数据关键字段，`detail.changed` 为空

#### Scenario: 覆盖重推构造 update 审计并记录被删旧组

- **GIVEN** 覆盖重推场景的输入（新 5 条调账 + 旧 4 条被删调账）
- **WHEN** 调用审计构造函数
- **THEN** 产出记录 `action=update`
- **AND** `detail.changed` 含被删除的旧 4 条调账清单

#### Scenario: 删除按主单各写一条 delete 审计

- **GIVEN** 一次批量删除成功删掉 2 条主单
- **WHEN** 查询 audit 表
- **THEN** 新增 2 条 `action=delete`、`res_type=account_bill_prepaid_item` 的记录
- **AND** 每条 `detail.data` 含对应 `uuid`

#### Scenario: res_name 与 cloud_res_id 留空

- **GIVEN** 一条构造完成的审计记录
- **WHEN** 检查 `res_name` 与 `cloud_res_id`
- **THEN** 两者均为空
- **AND** 调用方的 `uuid` 出现在 `detail` 中而非 `cloud_res_id`

### Requirement: 审计写入事务内封装

系统 SHALL 提供在 data-service 同步事务内写入审计的封装，使用 `dao.Audit.BatchCreateWithTx(kt, tx, audits)` 传单元素切片（audit DAO 无单条的 `CreateWithTx`）。审计写入 MUST 与业务写入同生共死。`cmd/account-server/logics/audit/audit.go` MUST 保持现有空实现，MUST NOT 在 account-server 侧新增预付费审计实现。

#### Scenario: account-server 审计逻辑保持空实现

- **GIVEN** `cmd/account-server/logics/audit/audit.go`
- **WHEN** 检查代码
- **THEN** 保持现有空实现
- **AND** 未在 account-server 侧新增预付费审计实现

#### Scenario: 审计写入不成为事务耗时瓶颈

- **GIVEN** 单事务内含 1 条 audit 写入
- **WHEN** 执行 N=36 的 sync 事务
- **THEN** audit 写入不成为耗时瓶颈
- **AND** 整体事务耗时留在 sync 接口 P99 < 2s 的预算内
