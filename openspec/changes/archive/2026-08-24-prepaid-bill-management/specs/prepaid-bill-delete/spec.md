## ADDED Requirements

### Requirement: 按订单月批量删除预付费账单

系统 SHALL 提供 `DELETE /api/v1/account/bills/prepaid_items/batch` 接口，按订单年月批量删除预付费主单及其派生调账。请求体 MUST 含必填 `order_year` / `order_month`（月份取值 1-12）；`main_account_cloud_ids` 为可选二级账号云上 ID 列表，传入时 MUST 只删除这些账号下的主单，列表元素 MUST NOT 为空串。响应 MUST 返回 `deleted_count`（实际删除的主单条数）。无匹配记录时 MUST 返回 `deleted_count=0`，MUST NOT 报错。

#### Scenario: 按订单月删除未定账主单及其派生调账

- **GIVEN** 订单月 2026-07 下有 2 条 `settle_state=unsettled` 的主单，各自带 N+1 条派生调账
- **WHEN** 调用删除接口，请求体仅含 `order_year=2026` 与 `order_month=7`
- **THEN** 2 条主单及其派生调账全部被物理删除
- **AND** 响应 `deleted_count=2`

#### Scenario: 按云账号 ID 收窄删除范围

- **GIVEN** 同一订单月下账号 A、B 各有 1 条未定账主单
- **WHEN** 请求传入 `main_account_cloud_ids` 仅含账号 A 的云上 ID
- **THEN** 只删除账号 A 的主单及其派生调账
- **AND** 账号 B 的记录保持不变

#### Scenario: 无匹配记录返回零删除

- **GIVEN** 订单月 2026-07 下不存在任何预付费主单
- **WHEN** 调用删除接口
- **THEN** 返回 `deleted_count=0`
- **AND** 不返回错误

#### Scenario: 空串云账号 ID 被拒

- **GIVEN** `main_account_cloud_ids` 含空串
- **WHEN** 执行请求结构体校验
- **THEN** 返回 `errf.InvalidParameter`

### Requirement: 删除鉴权三分支

系统的删除接口 SHALL 以 `ListAuthorizedInstances(AccountBillPrepaid, Delete)` 做二级账号行级隔离，按三分支处理：`IsAny=true` 时 MUST NOT 叠加账号过滤；`IsAny=false` 且 `IDs` 非空时 MUST 叠加 `RuleIn("main_account_id", IDs)`；`IsAny=false` 且 `IDs` 为空时 MUST 直接返回 `deleted_count=0`，MUST NOT 删除全量，MUST NOT 返回 500。删除 MUST 使用独立 action `AccountBillPrepaidDelete`，MUST NOT 复用写入 action。

#### Scenario: 无删除权限时返回零删除

- **GIVEN** 调用方对任何二级账号均无 `AccountBillPrepaid + Delete` 权限
- **WHEN** 调用删除接口
- **THEN** 返回 `deleted_count=0`
- **AND** 不删除任何记录、不返回 500

#### Scenario: 只删除已授权二级账号下的主单

- **GIVEN** 调用方仅被授权二级账号 M1 的删除权限，订单月下 M1、M2 各有 1 条未定账主单
- **WHEN** 调用删除接口
- **THEN** 只删除 M1 的主单及其派生调账
- **AND** M2 的记录保持不变

### Requirement: 删除双闸门整批拒绝

系统 SHALL 在删除前执行双闸门：任一目标主单 `settle_state=settled` 时 MUST 整批拒绝并返回 `errf.Aborted`，错误信息 MUST 含「已定账，需换 uuid 或订单月份」语义；任一关联调账 `push_status=pushing` 时 MUST 整批拒绝并返回 `errf.Aborted`，错误信息 MUST 含「正在推送」语义。两种拒绝 MUST 零变更。`push_status=failed` / `pushed` / `unpushed` MUST 放行删除。

#### Scenario: 任一主单已定账则整批零变更

- **GIVEN** 目标集合中 1 条已定账、其余未定账
- **WHEN** 调用删除接口
- **THEN** 返回 `errf.Aborted`，错误信息包含已定账主单 ID
- **AND** 全部主单与调账零变更

#### Scenario: 任一关联调账推送中则整批零变更

- **GIVEN** 目标集合全部未定账，但其中 1 条主单下存在 `push_status=pushing` 的调账
- **WHEN** 调用删除接口
- **THEN** 返回 `errf.Aborted`，错误信息包含该主单 ID
- **AND** 全部主单与调账零变更

#### Scenario: 已推送或失败的调账不阻挡删除

- **GIVEN** 目标主单未定账，关联调账的 `push_status` 为 `pushed` / `unpushed` / `failed`
- **WHEN** 调用删除接口
- **THEN** 删除成功
- **AND** 主单与派生调账均被物理删除

### Requirement: 级联删除与删除审计同事务

系统 SHALL 在 data-service 单事务内完成「按 `source_id` 物理删除派生调账 → 物理删除主单 → 写删除审计」。任一步失败 MUST 整体回滚。删除按 `filter.DefaultMaxInLimit` 分片。每条被删主单 MUST 写且仅写 1 条审计：`res_type=account_bill_prepaid_item`、`action=delete`、`res_id` 为主表 ID、`detail.data` 含 `uuid`、`detail.changed` 含被删调账 ID 清单。`cmd/account-server/logics/audit/audit.go` MUST 保持空实现，删除审计 MUST 落在 data-service 事务内。

#### Scenario: 主单与派生调账同事务级联删除

- **GIVEN** 1 条未定账主单下有 4 条派生调账
- **WHEN** 删除成功
- **THEN** 主单与 4 条调账均不存在
- **AND** 不出现「主单已删但调账残留」或相反的中间态

#### Scenario: 每条主单落一条 delete 审计

- **GIVEN** 一次删除成功删掉 2 条主单
- **WHEN** 查询 audit 表
- **THEN** 新增 2 条 `action=delete`、`res_type=account_bill_prepaid_item` 的记录
- **AND** 每条 `detail.data` 含对应 `uuid`，`detail.changed` 含该单被删调账 ID

### Requirement: 删除接口 APIGW 注册与文档

系统 SHALL 在 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml` 注册 `DELETE /api/v1/account/bills/prepaid_items/batch` 资源，超时配置 MUST 为 30s，MUST 启用应用认证与 `X-Bkapi-JWT` 校验，MUST NOT 要求用户登录（与 sync 一致）。接口文档 MUST 置于 `docs/api-docs/web-server/docs/resource/bill/delete_prepaid_item.md`，版本标注 `v9.9.9+`。

#### Scenario: APIGW 已注册删除资源且超时为 30s

- **GIVEN** APIGW 资源注册已完成
- **WHEN** 查看 `bk_apigw_resources_bk-hcm.yaml`
- **THEN** 存在 `delete_prepaid_bill_item` 资源且超时配置为 30s
- **AND** 接口文档存在且版本标注为 `v9.9.9+`
