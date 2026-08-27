# prepaid-sync-write

## Purpose

预付费 sync 写入全链路：映射、鉴权、双闸门、N+1 调账生成与单事务覆盖重建。

## Requirements

### Requirement: sync 接口处理顺序

系统 SHALL 按固定顺序处理批量请求中的**每一个**订单：① 解码与基础字段校验（整批）→ ② 云账号映射 → ③ 实例级鉴权 → ④ 业务校验 → ⑤ 唯一键与双闸门判定 → ⑥ 调用 data-service 单事务落库。步骤 ② 与 ③ 的顺序 MUST NOT 调换。映射字段为 `(main_account_cloud_id, vendor)`。接口整体 MUST 返回成功，逐单结果放在 `results` 中，成功项携带预付费账单 ID，失败项 `success=false` 且 `message` 给出原因。

> 契约字段、DTO 与校验分档由 `prepaid-sync-contract` 定义，本 capability MUST NOT 重新定义任何字段、类型、约束或错误码。

#### Scenario: 首次写入落主表与 N+1 条调账

- **GIVEN** 不存在 `(uuid=U1, 订单月=2026-07)` 的预付费记录
- **WHEN** 调用方调用 sync 推送该单（含 3 条分摊明细）
- **THEN** 主表新增 1 条 `settle_state=unsettled` 的记录
- **AND** 调账表新增 4 条 `source=prepaid`、`source_id` 为该主表记录 ID 的条目
- **AND** 接口返回该预付费 ID

### Requirement: 云账号映射与实例级鉴权

系统 SHALL 由调用方传入的 `(main_account_cloud_id, vendor)` 在调用方所属租户内映射出二级账号，取其 `main_account_id`、`root_account_id` 与 `op_product_id`。`root_account_cloud_id` MUST 按请求原样落库，MUST NOT 参与映射。租户维度 MUST NOT 由请求体指定：`main_account` 表已开启多租户，租户过滤条件由 DAO 按 Kit 中的租户自动注入。`op_product_id` SHALL 作为主表与调账的 `product_id`；`bk_biz_id` MUST 留空，以满足调账表 `InsertValidate` 的「`bk_biz_id` 或 `product_id` 二选一」约束。随后 SHALL 以 `main_account_id` 为实例执行 `AccountBillPrepaid + Create` 鉴权。映射不到唯一二级账号时 MUST 返回 `errf.RecordNotFound` 并在鉴权之前返回。鉴权不通过时 MUST NOT 泄露账号存在性以外的信息。

> 支持的云厂商由 `enumor.Vendor` 约束；预付费账单的实际生产方为 AWS 场景（如 SageMaker Training Plan Upfront Fee），映射逻辑对全部受支持厂商一致。

#### Scenario: 映射成功后以二级账号为实例鉴权

- **GIVEN** 调用方传入的 `(main_account_cloud_id, vendor)` 在调用方租户内能唯一映射到二级账号 M1
- **WHEN** 推送
- **THEN** 鉴权以 M1 为实例执行
- **AND** 落库的 `main_account_id` 为 M1，`root_account_id` 与 `product_id` 均由 M1 推导得出（`product_id` = M1 的 `op_product_id`）
- **AND** `bk_biz_id` 为空

#### Scenario: 映射失败在鉴权之前返回

- **GIVEN** `(main_account_cloud_id, vendor)` 在调用方租户内映射不到任何二级账号
- **WHEN** 推送
- **THEN** 返回 `errf.RecordNotFound`
- **AND** 在鉴权之前即返回，不因鉴权实例为空而报 500 或误放行

#### Scenario: 无写入权限时被拒且无数据落库

- **GIVEN** 调用方的调用身份对目标二级账号无 `AccountBillPrepaid + Create` 权限
- **WHEN** 调用 sync
- **THEN** 返回权限错误
- **AND** 无任何数据落库

### Requirement: 唯一键判定与定账闸门

系统 SHALL 以 `(uuid, order_year, order_month)` 为业务唯一键判定命中。命中且主表 `settle_state=unsettled` 时 SHALL 执行覆盖重推。命中且主表 `settle_state=settled` 时 MUST 返回 `errf.Aborted`，错误信息 MUST 包含「已定账，需换 uuid 或订单月份」语义，主表与调账表数据 MUST 零变更，且 MUST NOT 产生任何「失败」态主单记录。

#### Scenario: 未定账时重推成功

- **GIVEN** `(uuid=U1, 订单月=2026-07)` 的主表记录 `settle_state=unsettled`
- **WHEN** 调用方重推
- **THEN** 写入成功，HTTP 200

#### Scenario: 已定账时拒绝写入且零变更

- **GIVEN** 同一记录已被定账任务置为 `settle_state=settled`
- **WHEN** 调用方用同一 `(uuid, 订单月)` 重推
- **THEN** 返回 `errf.Aborted`，错误信息包含「已定账，需换 uuid 或订单月份」语义
- **AND** 主表与调账表数据零变更
- **AND** 主表不产生任何「失败」态记录

### Requirement: 推送中闸门

系统 SHALL 在写入前检查该 `source_id` 下的调账推送状态：存在任意一条 `push_status=pushing` 时 MUST 拒绝写入并返回 `errf.Aborted`，主表与调账表数据 MUST 零变更。`push_status=failed` MUST 放行 —— 推送失败后 调用方必须能重新推送修正数据，否则该账期会被永久锁死。

#### Scenario: 组内仅有 unpushed 与 failed 时放行

- **GIVEN** 某预付费单下 4 条调账的 `push_status` 分别为 `unpushed` / `failed` / `unpushed` / `failed`
- **WHEN** 调用方重推
- **THEN** 写入成功

#### Scenario: 组内存在 pushing 时拒绝

- **GIVEN** 该单下任意 1 条调账 `push_status=pushing`
- **WHEN** 调用方重推
- **THEN** 接口拒绝并返回 `errf.Aborted`
- **AND** 主表与调账表数据零变更

### Requirement: N+1 条调账生成结构

系统 SHALL 为一笔预付费生成 N+1 条调账：N 条 `type=increase` 落各分摊自然月，金额为调用方传入的该月分摊额；1 条 `type=decrease` 落**订单月份**（非写入操作发生的自然月），金额为调用方传入的优惠后总价。字段取值 SHALL 为：`bill_day` 沿用现网默认 1；`state` 恒为 `confirmed`；`push_status` 初始 `unpushed`；`settle_state` 初始 `unsettled`；`source` = `prepaid`；`source_id` = 预付费账单 ID；`operator` / `creator` 取 JWT 解析出的调用方；`res_class` 恒为 `gpu_card`、`res_sub_class` 取该单的 `gpu_type`，两者均不由上游系统指定；`rmb_cost` 由服务端按 `currency` 派生（`CNY` 时与 `cost` 同值，其他币种落 0），HCM MUST NOT 做汇率换算。

#### Scenario: 调减条目账期为订单月份

- **GIVEN** 订单月 = 2026-07，分摊明细覆盖 2026-08 / 2026-09 / 2026-10
- **WHEN** 调用方推送
- **THEN** 调账表产生 4 条：账期 2026-08 / 09 / 10 的 3 条 `type=increase`，以及账期 2026-07（订单月）的 1 条 `type=decrease`
- **AND** 4 条的 `bill_day` 均为 1

#### Scenario: 不存在账期为当前自然月的调减条目

- **GIVEN** 订单月 = 2026-07 且写入操作发生在其他自然月
- **WHEN** 查询该单下的调账
- **THEN** 不存在任何账期等于「写入操作发生的自然月」但不等于订单月的调减条目
- **AND** 调减条目账期恒等于订单月份

#### Scenario: prepaid 调账四字段出生态

- **GIVEN** 调用方推送成功
- **WHEN** 查询生成的 N+1 条调账
- **THEN** 全部为 `state=confirmed`、`push_status=unpushed`、`settle_state=unsettled`、`source=prepaid`

#### Scenario: 调增总额等于调减总额

- **GIVEN** 优惠后总价 = 3000.00，分摊明细为 1000.00 / 1000.00 / 1000.00
- **WHEN** 调用方推送
- **THEN** 写入成功
- **AND** 生成的调增总额 3000.00 等于调减总额 3000.00

### Requirement: 覆盖重建

系统的覆盖重推 SHALL 实现为「该 `source_id` 下旧调账整组**物理删除** + 重建 N+1 条」，而不是状态转移。新记录 MUST 从各字段初始值重新开始（`confirmed` / `unpushed` / `unsettled`）。删除与重建 MUST 在同一事务内完成，MUST NOT 出现调账真空窗口。主表记录 ID、`settle_state` 与 `creator` MUST 保持首次入库值，其余业务字段被整体更新。

#### Scenario: 重推后旧组被物理删除且总数为新组条数

- **GIVEN** 已存在 `(uuid=U1, 订单月=2026-07)` 且 `settle_state=unsettled`，其下有 4 条调账
- **WHEN** 调用方用同一 uuid 与同一订单月但把分摊改为 4 条重推
- **THEN** 主表记录 ID、`settle_state` 与 `creator` 保持不变，其余业务字段被整体更新
- **AND** 调账表中原 4 条被物理删除、新增 5 条
- **AND** 查询该 `source_id` 下调账总数为 5 而非 9

#### Scenario: 重推后已推送条目回到未推送

- **GIVEN** 某单下 4 条调账已全部 `push_status=pushed`，且主表仍 `unsettled`
- **WHEN** 调用方重推
- **THEN** 旧 4 条被物理删除
- **AND** 新条目 `push_status` 全部回到 `unpushed`

### Requirement: 单事务落库与失败不落库

系统 SHALL 调用 data-service `POST /bills/prepaid_items/sync`，由 data-service 在单个事务内完成「主表 upsert → 删除该 `source_id` 下旧调账 → 批量创建 N+1 条调账 → 写同步审计」四步。account-server MUST NOT 跨服务拼事务。任一步失败 MUST 整体回滚，MUST NOT 产生「主表有记录但调账不全」的中间态。批量创建 MUST 按 `constant.BatchOperationMaxLimit`（100）分片。校验失败或事务回滚时 MUST NOT 在主表新增任何记录，MUST NOT 留下「失败」态半成品主单。同一 `(uuid, 订单月)` 并发重推 SHALL 依赖唯一键与事务，冲突返回 `errf.RecordDuplicated` 由调用方重试。失败单 MUST 只影响其 `results` 项，MUST NOT 中断整批后续单据。

#### Scenario: 事务在创建新调账阶段失败时整体回滚

- **GIVEN** 某单下 4 条调账已全部 `push_status=pushed`，主表仍 `unsettled`
- **WHEN** 重推事务在创建新调账阶段失败
- **THEN** 事务回滚
- **AND** 旧 4 条调账仍完整存在且 `push_status=pushed`
- **AND** 主表字段未被更新

#### Scenario: 失败路径不留半成品主单

- **GIVEN** 一次会触发校验失败或事务回滚的推送（分摊金额不平衡 / 云账号映射失败 / 事务中途异常）
- **WHEN** 推送
- **THEN** 返回对应错误码
- **AND** 主表不新增任何记录，不留下「失败」态半成品主单
- **AND** 重新查询该 `(uuid, 订单月)` 应为「不存在」

#### Scenario: 并发重推由唯一键收敛

- **GIVEN** 同一 `(uuid, 订单月)` 的两个 sync 请求并发到达
- **WHEN** 两者同时进入落库阶段
- **THEN** 其中一个成功、另一个返回 `errf.RecordDuplicated`
- **AND** 最终该 `source_id` 下调账条目数恰为 N+1，无重复组

### Requirement: 同步审计事务内集成

系统 SHALL 在业务写入的同一事务内写且仅写一条同步审计记录，即使本次内容与上次完全一致也照记。首次写入 `action=create`，覆盖重推 `action=update`。事务回滚时 MUST NOT 新增任何审计记录。

#### Scenario: 首次写入落一条 create 审计

- **GIVEN** 调用方首次推送某单
- **WHEN** 写入成功
- **THEN** audit 表新增 1 条 `res_type=account_bill_prepaid_item`、`action=create`、`res_id` 为预付费 ID 的记录
- **AND** `detail.data` 含本次 N+1 条调账清单与主数据关键字段

#### Scenario: 幂等重推照记且回滚不写审计

- **GIVEN** 调用方用完全相同的内容再推一次
- **WHEN** 写入成功
- **THEN** audit 表再新增 1 条 `action=update` 的记录
- **AND** `detail.changed` 含被删除的旧调账清单
- **AND** 若写入事务回滚，则 audit 表不新增任何记录

### Requirement: sync 接口写入性能

系统的 sync 接口 SHALL 在 N=36、并发 5 的场景下满足 P99 < 2s。调用方调用峰值 QPS ≤ 5 SHALL 作为容量设计基线。分摊明细条数不设上限；单事务内的调账写入按 `constant.BatchOperationMaxLimit`（100）分片，避免单批过长。

#### Scenario: 压测满足 P99 与零错误率

- **GIVEN** N=36 的 sync 请求、并发 5
- **WHEN** 用 wrk 压测 5 分钟
- **THEN** P99 < 2s
- **AND** 错误率为 0
