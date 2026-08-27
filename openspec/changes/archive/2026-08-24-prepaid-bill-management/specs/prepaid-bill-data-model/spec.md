## ADDED Requirements

### Requirement: 预付费账单主表

系统 SHALL 新增表 `account_bill_prepaid_item`，字段包括 `id`（varchar(64) 主键，即预付费 ID）、`uuid`（varchar(255)，上游系统外部唯一标识）、`order_year` / `order_month`（订单月份）、`vendor`、`root_account_id` / `main_account_id`、`root_account_cloud_id` / `main_account_cloud_id`、`product_id`（取二级账号 `op_product_id`）、`resource_id` / `invoice_id`、`gpu_type`、`device_num` / `card_num`、`product_name` / `product_spec` / `region`、`usage_start_at` / `usage_end_at` / `order_at`（`VARCHAR(64)`，格式 `constant.DateTimeLayout`）、`currency`、`cost` / `rmb_cost`（decimal(38,10)，`rmb_cost` 由服务端按币种派生）、`settle_state`，以及 `creator` / `reviser` / `created_at` / `updated_at`。表 MUST 具备业务唯一键 `uk_uuid_order_month(uuid, order_year, order_month)`，以及 `idx_main_account_id` / `idx_order_year_month` / `idx_settle_state`。SQL 变更 MUST 遵循 `SQLVER=9999` / `HCMVER=v9.9.9`，文件名以 `9999` 开头并置于 `scripts/sql/`。

`usage_start_at` / `usage_end_at` 在同步契约中为非必填，两列 MUST 为 `NOT NULL DEFAULT ''`：未传入时 MUST 落空串而非 NULL 或零值时间；`order_at` MUST 保持 NOT NULL。table struct 用 `*string` 承载使用起止：覆盖重推不传时写入空串，查询响应对应字段为空串。

#### Scenario: 表结构与数据模型逐字段一致

- **GIVEN** SQL 迁移已执行
- **WHEN** 查询 `information_schema` 中 `account_bill_prepaid_item` 的表结构
- **THEN** 字段与需求文档「数据模型 → 新增主表」逐字段一致，无遗漏、无多余
- **AND** 存在唯一键 `uk_uuid_order_month(uuid, order_year, order_month)`
- **AND** 存在 `idx_main_account_id` / `idx_order_year_month` / `idx_settle_state`
- **AND** `usage_start_at` / `usage_end_at` / `order_at` 为 `VARCHAR(64)`，使用起止 `NOT NULL DEFAULT ''`

#### Scenario: 覆盖重推不传使用时间时清空原值

- **GIVEN** 一条已落库且带有使用起止时间的主单
- **WHEN** 调用方以同一业务唯一键重推且不传 `usage_start_at` / `usage_end_at`
- **THEN** 两列被更新为空串，查询响应中对应字段为空串
- **AND** `order_at` 与其余业务字段按本次请求正常刷新

#### Scenario: 唯一键在重复插入时生效

- **GIVEN** 已存在 `(uuid=U1, order_year=2026, order_month=7)` 的记录
- **WHEN** 再插入同一组合
- **THEN** 返回 `errf.RecordDuplicated`（MySQL Error 1062）
- **AND** 该唯一键支撑覆盖重推的命中判定与并发重推的冲突处理

### Requirement: 预付费主表仅两态且无失败态

系统的 `account_bill_prepaid_item.settle_state` SHALL 仅有 `unsettled`（未定账）与 `settled`（已定账）两个取值。表结构 MUST NOT 包含任何「失败」枚举值，MUST NOT 包含 `fail_reason` 或等价字段。写入失败时 MUST NOT 落库任何半成品主单记录。

#### Scenario: 表结构不存在失败态与失败原因字段

- **GIVEN** 一条合法的预付费主表记录
- **WHEN** 检查表结构与 `settle_state` 枚举定义
- **THEN** `settle_state` 取值仅为 `unsettled` 或 `settled`
- **AND** 表结构中不存在任何「失败」枚举值，也不存在 `fail_reason` 字段

### Requirement: 预付费主表 table struct 与 DAO

系统 SHALL 在 `pkg/dal/table/bill/` 新增预付费主表的 table struct（含 `TableName()` / `InsertValidate()` / `UpdateValidate()` 与字段列声明），在 `pkg/dal/dao/bill/` 新增 DAO，方法 MUST 覆盖 `CreateWithTx` / `BatchCreateWithTx` / `UpdateWithTx` / `List` / `DeleteWithTx`。批量方法 MUST 按 `constant.BatchOperationMaxLimit`（100）分片。ID MUST 由 `idgenerator` 生成。

#### Scenario: DAO 各方法按预期读写

- **GIVEN** DAO 已实现
- **WHEN** 依次调用 `CreateWithTx` / `List` / `UpdateWithTx` / `DeleteWithTx`
- **THEN** 各方法按预期完成读写
- **AND** `InsertValidate` 对缺失必填字段返回 `errf.InvalidParameter`

### Requirement: data-service 层预付费 CRUD 与事务变体

系统 SHALL 在 `pkg/api/data-service/bill` 新增请求与响应结构，在 `cmd/data-service/service/bill/` 注册 Create / Update / List / BatchDelete 路由，并 MUST 提供专用 `POST /bills/prepaid_items/sync` 接口，在 data-service 单事务内编排「主表 upsert → 旧调账删除 → N+1 调账创建 → 审计写入」。account-server MUST NOT 跨服务拼事务。client 封装 MUST 经 `pkg/client/common/request.go` 的方法完成。List 单页上限 MUST 为 `core.DefaultMaxPageLimit`（500）。BatchDelete MUST 同事务级联删除该 `source_id` 下派生调账并写删除审计。

#### Scenario: 事务变体支持单事务内多步编排

- **GIVEN** data-service 已提供专用 sync 接口
- **WHEN** 写入域调用 `SyncBillPrepaidItem`，请求含主表数据与 N+1 条调账
- **THEN** 四步在同一 data-service 事务内执行，任一失败整体回滚

#### Scenario: 列表单页超上限被拒

- **GIVEN** 一个请求单页 501 条的 List 调用
- **WHEN** 调用 data-service 的 List 接口
- **THEN** 返回 `errf.InvalidParameter`（单页上限为 500）

### Requirement: 调账表新增五个字段

系统 SHALL 为 `account_bill_adjustment_item` 新增五个字段：`source`（varchar(32)，`manual` 人工录入 / `prepaid` 预付费生成）、`source_id`（varchar(64)，预付费账单 ID，人工录入为空）、`push_status`（varchar(32)，`unpushed` / `pushing` / `pushed` / `failed`）、`push_fail_reason`（varchar(255)）、`settle_state`（varchar(32)，`unsettled` / `settled`）。同步 MUST 更新 `pkg/dal/table/bill/billadjustmentitem.go` 的 struct 与字段列，以及 `pkg/api/core/bill` 的核心结构。

#### Scenario: 调账表五个新字段存在

- **GIVEN** SQL 迁移已执行
- **WHEN** 查询 `account_bill_adjustment_item` 表结构
- **THEN** 存在 `source` / `source_id` / `push_status` / `push_fail_reason` / `settle_state` 五个字段

### Requirement: 调账表存量数据刷安全默认值

系统 SHALL 在新增五个字段的同时为现网存量数据刷安全默认值：`source=manual`、`source_id` 为空、`push_status=pushed`、`push_fail_reason` 为空、`settle_state=settled`。刷值 MUST 与建字段在同一 SQL 迁移文件内完成，避免出现「字段已加但值为空」的窗口期。存量数据量大时 MUST 分批 UPDATE，避免长事务锁表。

刷 `settle_state=settled` MUST 限定 `state='confirmed'`：`settled` 单向不可逆且被守卫拦住编辑与删除，若把 `state=unconfirmed` 的待确认存量调账也刷成 `settled`，运营将永久无法确认这批调账且没有任何恢复手段。待确认的存量行 SHALL 保持列默认值 `unsettled`。

> 刷 `push_status=pushed` 使历史调账不被推送打点重新置 `pushing`；刷 `settle_state=settled` 使历史已确认调账仍被 `settled` 守卫拦住不可编辑删除 —— 这是现网调账守卫放开风险可控的关键前提。

#### Scenario: 存量记录全部刷上安全默认值且无空值

- **GIVEN** 现网存量调账数据
- **WHEN** 迁移完成后抽样查询
- **THEN** 全部存量记录的 `source=manual`、`push_status=pushed`
- **AND** `state=confirmed` 的存量记录 `settle_state=settled`
- **AND** `source_id` 与 `push_fail_reason` 为空
- **AND** 不存在任何这五个字段为 NULL 的存量行

#### Scenario: 待确认的存量调账不被刷成已定账

- **GIVEN** 现网存在 `state=unconfirmed` 的存量调账
- **WHEN** 迁移完成后查询这批记录
- **THEN** 其 `settle_state` 仍为 `unsettled`
- **AND** 运营仍可正常确认这批调账

#### Scenario: 存量刷值不阻塞现网调账读写

- **GIVEN** 调账表为现网量级数据
- **WHEN** 执行存量刷值迁移
- **THEN** 分批执行且单批不产生超过约定阈值的锁等待
- **AND** 迁移期间现网调账 list 接口可正常响应

### Requirement: 调账表账期扫描不新增专用索引

本期 SQL MUST NOT 为 `account_bill_adjustment_item` 新建 `idx_bill_year_month`。定账任务按 `(bill_year, bill_month)` 分页扫描，依赖现网主键回表。预付费主表定账走新建的 `idx_order_year_month`。

#### Scenario: 本期迁移不含调账表账期索引

- **GIVEN** 迁移已执行
- **WHEN** 查询 `account_bill_adjustment_item` 的索引
- **THEN** 不存在名为 `idx_bill_year_month` 的索引
- **AND** 预付费主表存在 `idx_order_year_month`

### Requirement: 三个新枚举定义

系统 SHALL 在 `pkg/criteria/enumor/bill.go` 新增 `BillAdjustmentSource`（`manual` / `prepaid`）、`BillAdjustmentPushStatus`（`unpushed` / `pushing` / `pushed` / `failed`，共 4 态，MUST NOT 包含「超时」态）、`BillSettleState`（`unsettled` / `settled`）。每个枚举 MUST 提供 `Validate()` 方法。`settle_state` 两张表 MUST 共用同一 `BillSettleState` 定义，取值与含义完全一致，但两表彼此独立判定。

> `enumor.CurrencyCode.Validate()` 属校验关注点，由契约域实现，本 capability 不重复实现。

#### Scenario: push_status 枚举仅四态且拒绝超时

- **GIVEN** 三个新枚举已定义
- **WHEN** 对 `BillAdjustmentPushStatus` 逐值调用 `Validate()`
- **THEN** `unpushed` / `pushing` / `pushed` / `failed` 四值通过
- **AND** `timeout` 等其他取值返回错误

#### Scenario: 两表共用同一 settle_state 枚举定义

- **GIVEN** `settle_state` 被预付费主表与调账表共用
- **WHEN** 检查代码中的枚举引用
- **THEN** 两处引用同一 `BillSettleState` 定义
- **AND** 不存在重复的枚举声明
