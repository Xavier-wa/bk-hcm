# 实现任务清单

> 按**波次**分组，阶段 4 按波次分批实现。波次内零相互依赖可并行，跨波次不可逆序。
>
> | 波次 | 卡 | capability | 子单 | 工时 |
> |---|---|---|---|---|
> | Wave 1 | S1 契约 | `prepaid-sync-contract` | `1069995598137350526` | 15h |
> | Wave 1 | S2 数据底座 | `prepaid-bill-data-model` | `1069995598137350683` | 23.5h |
> | Wave 1 | S3 IAM 审计 | `prepaid-iam-audit-base` | `1069995598137350775` | 15h |
> | Wave 2 | S4 写入接口 | `prepaid-sync-write` | `1069995598137351347` | 24h |
> | Wave 2 | S4b 删除接口 | `prepaid-bill-delete` | （编码阶段相对原规划新增） | — |
> | Wave 2 | S5 定账任务 | `prepaid-settle-task` | `1069995598137351656` | 21.5h |
> | Wave 2 | S6 OBS 打点 | `bill-adjustment-push-status` | `1069995598137352154` | 18h |
> | Wave 2 | S7 列表查询 | `prepaid-bill-query`（列表段） | `1069995598137352704` | 16h |
> | Wave 2 | S9 现网调账改造 | `bill-adjustment-guard` | `1069995598137353698` | 23h |
> | Wave 3 | S8 整合明细 | `prepaid-bill-query`（明细段） | `1069995598137353407` | 12h |
>
> **合计原规划 168h（21 人天）；删除接口为编码阶段新增对外能力。**
>
> 流程项未完成：预付费契约 Additive 冻结确认（1.5）、IAM 评审确认（3.1/3.14）、若干压测与上线收口。
> ★ **一个上线闸门**：第 2 组的 SQL 迁移（含存量刷值）必须先于第 6 组与第 8 组上线，否则存量保护前提不成立。

## 1. Wave 1 · S1 契约域（prepaid-sync-contract，15h）

- [x] 1.1 产出预付费 sync 接口字段级契约草案：按技术方案 §3.1 直译并统一 snake_case，与预付费主表字段逐字段对齐（**本变更的第一个工作项，其余任务均可与之并行，但第 4 组必须等其冻结**）
- [x] 1.2 在契约中明确三条约束：调用方不得传负数；`type` 由条目角色固定不由金额正负判定；`rmb_cost` 不入请求体、由服务端按 `currency` 派生（`CNY` 同值，其他币种落 0），HCM 不做汇率换算
- [x] 1.3 编写错误码语义表：`InvalidParameter` / `RecordNotFound` / `Aborted` / `RecordDuplicated` 各自的触发场景
- [x] 1.4 产出 Mock 请求样例两组（N=1 与 N=36），供调用方在契约确认前自测
- [ ] 1.5 发调用方确认并标记冻结版本号；建立契约 Additive 治理约定（只允许新增字段，禁止删改既有字段，变更须走影响评估）
- [x] 1.6 在 `pkg/api/account-server/bill` 定义 `PrepaidItemSyncReq` / `PrepaidSplitItem` / `PrepaidItemSyncResp`，金额字段用 decimal 语义承载不得用浮点
- [x] 1.7 分摊明细条数不设上限，仅要求 `validate:"required,min=1,dive"`
- [x] 1.8 为 `enumor.CurrencyCode` 新增 `Validate()` 方法（现网无此方法），写法参照同文件 `BillAdjustmentResClass.Validate()`
- [x] 1.9 实现 R-015 硬校验：`currency` 枚举、`vendor`、三个时间字段解析与先后关系、分摊月份范围、主数据 `cost` > 0、分摊金额不要求为正、`device_num`/`card_num` 不得为负、分摊合计 = 总价、分摊明细至少 1 条；每项不通过返回 `errf.InvalidParameter` 并指明字段
- [x] 1.10 提供「`currency` 与二级账号所属 summary root 的 `currency` 一致」的可复用比对函数，供写入域在映射完成后调用
- [x] 1.11 取消原软校验档：请求不接收 `res_class` / `res_sub_class`；派生调账固定 `res_class=gpu_card`，`res_sub_class` 取必填 `gpu_type`
- [x] 1.12 在 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml` 注册 sync 资源，超时配置 30s，启用 `X-Bkapi-JWT` 校验
- [x] 1.13 在 `docs/api-docs/web-server/docs/resource/bill/` 编写 sync 接口文档，版本标 `v9.9.9+`
- [x] 1.14 单测覆盖硬校验反例与 `gpu_type` 必填、批内唯一键重复、N=101 在 validator 层即被拒不进业务逻辑

## 2. Wave 1 · S2 数据域（prepaid-bill-data-model，23.5h）

- [x] 2.1 编写 `scripts/sql/9999_*.sql`（`SQLVER=9999` / `HCMVER=v9.9.9`）建表 `account_bill_prepaid_item`，字段按需求文档「数据模型 → 新增主表」逐字段落地
- [x] 2.2 建业务唯一键 `uk_uuid_order_month(uuid, order_year, order_month)`
- [x] 2.3 确认表结构不含任何「失败」枚举值、不含 `fail_reason` 或等价字段（`settle_state` 仅 `unsettled` / `settled` 两态）
- [x] 2.4 在**同一 SQL 文件内**为 `account_bill_adjustment_item` 加 5 字段（`source` / `source_id` / `push_status` / `push_fail_reason` / `settle_state`）并刷存量默认值（`manual` / 空 / `pushed` / 空 / `settled`），存量量大时分批 UPDATE 避免长事务锁表。刷 `settled` **必须限定 `state='confirmed'`**：待确认存量行保持 `unsettled`，否则叠加 `settled` 守卫后运营将永久无法确认这批调账
- [x] 2.5 主表新建 `idx_main_account_id` / `idx_order_year_month` / `idx_settle_state`。**本期不建**调账表 `idx_bill_year_month`；定账任务按账期分页扫描
- [x] 2.6 在 `pkg/criteria/enumor/bill.go` 新增 `BillAdjustmentSource`（`manual`/`prepaid`）、`BillAdjustmentPushStatus`（4 态，不含「超时」）、`BillSettleState`（`unsettled`/`settled`），每个提供 `Validate()`
- [x] 2.7 确认 `settle_state` 两表共用同一 `BillSettleState` 定义，无重复枚举声明
- [x] 2.8 在 `pkg/dal/table/bill/` 新增预付费主表 table struct（`TableName()` / `InsertValidate()` / `UpdateValidate()` / 字段列声明）
- [x] 2.9 在 `pkg/dal/dao/bill/` 新增预付费主表 DAO，覆盖 `CreateWithTx` / `BatchCreateWithTx` / `UpdateWithTx` / `List` / `DeleteWithTx`，批量按 `constant.BatchOperationMaxLimit`（100）分片，ID 由 `idgenerator` 生成
- [x] 2.10 同步更新 `pkg/dal/table/bill/billadjustmentitem.go` 的 struct 与字段列、`pkg/api/core/bill` 的核心结构，纳入 5 个新字段
- [x] 2.11 在 `pkg/api/data-service/bill` 新增请求响应结构，在 `cmd/data-service/service/bill/` 注册预付费 Create / Update / List / BatchDelete 路由，List 单页上限 `core.DefaultMaxPageLimit`（500）
- [x] 2.12 提供 data-service 专用 `POST /bills/prepaid_items/sync`，在单事务内编排「主表 upsert → 旧调账删除 → N+1 调账创建 → 审计写入」；覆盖重推保持主表 ID / `settle_state` / `creator`
- [x] 2.13 在 `pkg/client/data-service` 封装 client 调用，经 `pkg/client/common/request.go` 的方法完成
- [ ] 2.14 迁移后验证：表结构逐字段比对（含 VARCHAR 时间列与主表三个二级索引）；存量 `confirmed` 行刷值抽样无 NULL；唯一键重复插入返回 `errf.RecordDuplicated`

## 3. Wave 1 · S3 权限审计域（prepaid-iam-audit-base，15h）⛔ 前置：IAM 权限模型评审

- [ ] 3.1 发起 IAM 权限模型评审：确认新增 `AccountBillPrepaid` 资源类型成立（而非纯复用现网 `AccountBill`）。**评审未通过前本组不得开始编码**
- [x] 3.2 在 `pkg/iam/meta/resource.go` 新增资源类型常量 `AccountBillPrepaid`
- [x] 3.3 在 `pkg/iam/sys/initial_actions.go` 新增写入 action `AccountBillPrepaidCreate` 与删除 action `AccountBillPrepaidDelete`，均关联 `mainaccountResource`（参照 `MainAccountFind`）；只做实例级，不设菜单级
- [x] 3.3.1 将两个 action 挂入 `pkg/iam/sys/initial_action_groups.go` 的「平台管理 / 云账单管理」分组，避免权限中心归入「未分类」
- [x] 3.4 在 `cmd/auth-server/service/auth/gen_id.go` 新增 `genAccountBillPrepaidRuleResource`，资源类型取 `sys.MainAccount`，按 `a.ResourceID` 填实例（参照 `genMainAccountRuleResource`）；`Create` 映射写入 action，`Delete` 映射独立删除 action
- [x] 3.5 `Create` 且 `ResourceID` 为空时返回参数错误，不生成空实例规则（防止实例为空而误放行）；`Delete` 允许空 `ResourceID` 以支持 `ListAuthorizedInstances`
- [x] 3.6 确认查询侧未新增任何 action，统一复用 `ListAuthorizedInstances(MainAccount, Find)`
- [x] 3.7 在 `pkg/criteria/enumor/audit.go` 新增 res_type 常量 `account_bill_prepaid_item`
- [x] 3.8 **将该常量同时注册进 `AuditResourceTypeEnums`**（现网 `CloudCvmAuditResType` 是「已定义未注册」的反例，漏注册会在运行期报 `resource type not support`）
- [x] 3.9 实现同步审计记录构造：`res_id` 取主表 `id`；`action` 首次 `create` / 重推 `update`；`operator` = `kit.User`；`source` = `kt.GetRequestSource()`；`vendor` / `account_id` 分别取云厂商与 `main_account_id`；`bk_biz_id` 取 -1
- [x] 3.10 构造 `detail.data`（本次 N+1 条调账清单 + 主数据关键字段）与 `detail.changed`（重推时被删旧组清单，首次为空）
- [x] 3.11 `res_name` 与 `cloud_res_id` 留空，调用方的 `uuid` 放 `detail` 而非借用 `cloud_res_id` 语义
- [x] 3.12 封装事务内审计写入，用 `dao.Audit.BatchCreateWithTx(kt, tx, audits)` 传单元素切片（audit DAO 无单条 `CreateWithTx`）
- [x] 3.13 确认 `cmd/account-server/logics/audit/audit.go` 保持现有空实现，不在 account-server 侧新增预付费审计实现
- [ ] 3.14 验证：`ResType.Exist()` 返回 true 且 `AuditTable.CreateValidate` 不报 `resource type not support`；权限中心可见新资源类型与 action 且维度为二级账号

## 4. Wave 2 · S4 写入域（prepaid-sync-write，24h）

- [x] 4.1 契约口径已按 `prepaid-sync-contract` 落地（批量 `items`、`gpu_type`、服务端派生 `rmb_cost`）；发调用方冻结版本号见 1.5
- [x] 4.2 在 account-server `service/bill/` 新增 prepaid 包与 sync handler，注册 `POST /api/v1/account/bills/prepaid_items/sync`；api-server 侧加路由透传
- [x] 4.3 按固定顺序实现处理链：解码校验 → 云账号映射 → 实例级鉴权 → 业务校验 → 唯一键与双闸门判定 → 单事务落库（**映射与鉴权顺序不可调换**）
- [x] 4.4 实现云账号映射：由 `(main_account_cloud_id, vendor)` 在调用方租户内查二级账号（租户过滤由 DAO 按 Kit 自动注入），取 `main_account_id` / `root_account_id` / `op_product_id`；`root_account_cloud_id` 按请求原样落库；`product_id` 取 `op_product_id`，`bk_biz_id` 留空以满足调账表「二选一」约束
- [x] 4.5 映射不到唯一二级账号时返回 `errf.RecordNotFound` 并**在鉴权之前**返回
- [x] 4.6 以 `main_account_id` 为实例做 `AccountBillPrepaid + Create` 鉴权；鉴权失败不泄露账号存在性以外的信息、无数据落库
- [x] 4.7 调用契约域的 `currency` 比对函数完成「与二级账号一致」校验（依赖 4.4 的映射结果）
- [x] 4.8 实现唯一键判定与定账闸门：`settle_state=settled` 时返回 `errf.Aborted`，错误信息含「已定账，需换 uuid 或订单月份」语义，两表零变更、不产生失败态主单
- [x] 4.9 实现推送中闸门：该 `source_id` 下存在任一 `push_status=pushing` 时拒绝并返回 `errf.Aborted`、零变更；`failed` 放行
- [x] 4.10 实现 N+1 生成：N 条 `increase` 落各分摊自然月、1 条 `decrease` 落**订单月份**（非当前自然月）；`bill_day`=1、`state`=`confirmed`、`push_status`=`unpushed`、`settle_state`=`unsettled`、`source`=`prepaid`、`source_id`=预付费 ID、`operator`/`creator` 取 JWT 调用方
- [x] 4.11 实现覆盖重建：旧组**物理删除** + 重建 N+1 条，主表 ID / `settle_state` / `creator` 不变，其余业务字段整体更新，删除与重建同事务无真空窗口
- [x] 4.12 调用 data-service `POST /bills/prepaid_items/sync` 完成单事务落库：主表 upsert → 删旧调账 → 批量创建 N+1（按 100 分片）→ 写审计，任一失败整体回滚
- [x] 4.13 集成审计事务内写入（调用第 3 组的构造与写入封装），保证一次调用写且仅写一条、幂等重推照记、回滚不写审计
- [x] 4.14 校验失败或事务回滚时不在主表新增任何记录、不留失败态半成品主单
- [x] 4.15 并发重推依赖唯一键 + 事务，冲突返回 `errf.RecordDuplicated`，最终该 `source_id` 下条目数恰为 N+1 无重复组
- [ ] 4.16 压测验证：N=36、并发 5、wrk 5 分钟，P99 < 2s 且错误率为 0

## 4b. Wave 2 · S4b 删除域（prepaid-bill-delete）

- [x] 4b.1 定义 `PrepaidItemDeleteReq` / `PrepaidItemDeleteResp`：必填 `order_year` / `order_month`，可选 `main_account_cloud_ids`（`dive,required` 拒绝空串）
- [x] 4b.2 注册 `DELETE /api/v1/account/bills/prepaid_items/batch`，鉴权走 `ListAuthorizedInstances(AccountBillPrepaid, Delete)` 三分支；无权限返回 `deleted_count=0`
- [x] 4b.3 实现双闸门：任一主单 `settled` 或任一关联调账 `pushing` 整批 `errf.Aborted`、零变更
- [x] 4b.4 data-service `BatchDeleteBillPrepaidItem` 同事务级联删除派生调账与主单，并为每条主单写 `action=delete` 审计
- [x] 4b.5 在 APIGW 注册删除资源（超时 30s，应用认证 + JWT），编写 `delete_prepaid_item.md`（版本 `v9.9.9+`）
- [x] 4b.6 单测覆盖定账拒绝、推送中拒绝、鉴权三分支与筛选条件构造

## 5. Wave 2 · S5 定账域（prepaid-settle-task，21.5h）

- [x] 5.1 在 `cmd/account-server/service/service.go` 新增 `initCronTask`（`cron.Init` + `cron.Register`），写法参照现网 cron 注册（account-server 此前无任何 cron 任务）
- [x] 5.2 新建 `cmd/account-server/task/bill_settle.go`，实现 `croncore.Task` 四方法
- [x] 5.3 `Do(kt)` **首行**做 `sd.IsMaster()` 检查，非 master 直接返回并打日志（调度器不做 leader 选举，多副本去重由任务自负）
- [x] 5.4 实现纯时间闸门判定：锁定时点 = 基准月份次月 9 日 00:00:00（Asia/Shanghai），8 日整日仍可覆盖；不接受任何外部系统直接触发锁定的入参
- [x] 5.5 实现双表独立判定：主表按 `order_year`/`order_month`，调账按各自 `bill_year`/`bill_month`，两者不联动
- [x] 5.6 扫描条件仅捞 `unsettled`，批量置 `settled`；`settled` 单向不可逆、不重复更新已定账记录。调账表额外限定 `state=confirmed`，待确认的人工调账不参与定账（**待产品确认的口径假设**，见 design/spec 留痕）
- [x] 5.7 调账表扫描限定账期回溯窗口（默认 3 个月）；本期不依赖 `idx_bill_year_month`。超窗口的 `unsettled` 条目不被扫描（有意的性能取舍）。**已知缺口**：账期滑出窗口后才被确认的人工调账将永不定账（5.6 口径的衍生后果），不改扫描逻辑，处置手段见 5.11
- [x] 5.8 实现 `GetURL()` 手动触发路径并在 account-server 注册为 POST 接口，仅管理员可调用。手动触发走**不带 master 门禁**的 `RunOnce`（master 判定只留在 cron 的 `Do`），否则请求落到非 master 副本会静默返回成功；接口为同步长耗时操作，已在接口文档中写明超时取舍
- [x] 5.9 配置项**双写** `cmd/account-server/etc/account_server.yaml` 与 `docs/support-file/helm/values.yaml`：锁定日（默认 8）、扫描间隔（默认 1 小时）、回溯窗口（默认 3 个月），两处键名一致
- [x] 5.10 编写手动触发接口的接口文档，版本标 `v9.9.9+`
- [x] 5.11 在运维文档中说明「回溯窗口 3 个月」的行为边界与「锁定精度 = 扫描周期（最坏 1 小时误差）」，并给出超窗口漏扫的两个触发场景（长时间停机 / 迟确认）与处置步骤（调大 `lookbackMonth` → 重启生效 → 手动触发一轮 → 调回默认值）
- [ ] 5.12 验证：多副本仅 master 执行；启动日志可证 `initCronTask` 已执行且任务已注册；8 日边界与 9 日零点行为；「迟到即终稿」（锁定时点后首次写入先 `unsettled` 再由下轮置 `settled`）
- [ ] 5.13 性能验证：回溯 3 个月账期单次扫描耗时 < 5min 且无慢查询告警；锁定时点刚过的记录在 1 小时内被置 `settled`

## 6. Wave 2 · S6 推送域（bill-adjustment-push-status，18h）★ 上线须晚于第 2 组

- [x] 6.1 在 data-service 新增调账 `push_status` / `push_fail_reason` 批量更新接口，按 `BatchOperationMaxLimit` 分片（其他服务禁止直接操作 DB）
- [x] 6.2 在 `cmd/account-server/logics/bill/sync_controller.go` 创建调账 Flow 前，批量将该 `(vendor, 年, 月)` 下 `state=confirmed` 的调账置 `pushing`；`unconfirmed` 不置位
- [x] 6.3 在 `cmd/task-server/logics/action/obs/sync/sync_adjustment.go` 的 `Run` 返回前，**所有分页全部插入成功后**统一回写：本次处理 ID 集合置 `pushed`
- [x] 6.4 同一回写点做兜底纠正：该账期内其余 `pushing` 残留置回 `unpushed`
- [x] 6.5 **确保不逐页回写**：中途分页失败时前面已插入页不得被置 `pushed`（`clean` 是插入前一次性完成的，此时 OBS 数据本就不完整）
- [x] 6.6 在 `sync_controller.go` 的 Flow 失败/取消分支置 `failed` 并写 `push_fail_reason`
- [x] 6.7 置 `failed` 后按现网既有行为重建 Flow（调账回到 `pushing`），不引入重试计数与重试上限
- [x] 6.8 不新建独立告警通道，复用现网 `BillSyncRecord` 失败态与既有运维告警
- [ ] 6.9 验证不变式：一次整月推送结束后该账期不存在 `pushing` 残留（只可能是 `pushed` / `unpushed` / `failed`）
- [ ] 6.10 验证存量保护：存量调账已刷 `pushed` 后，SyncController 正常创建当期 Flow 不会把历史账期记录重新置 `pushing`
- [ ] 6.11 验证 `gpu_type` 不在清单内的 prepaid 调账在 OBS 侧 GPU 卡型字段为空串且同步不报错
- [ ] 6.12 性能验证：置位与回写均分片批量执行无单条循环更新，整月同步总耗时相比改造前无显著劣化

## 7. Wave 2 · S7 查询域（prepaid-bill-query 列表段，16h）

- [x] 7.1 在 account-server 实现 `POST /api/v1/account/bills/prepaid_items/list`，支持跨账期查询不要求指定固定账单月份
- [x] 7.2 请求采用 `filter` 表达式（`core.ListWithoutFieldReq`），可筛选字段为预付费主表列，字段名与单页上限由 data-service DAO 统一校验
- [x] 7.3 接口文档给出按使用时间筛选的区间重叠写法（`usage_end_at gte 起点` + `usage_start_at lte 终点`）与订单月份须 `order_year` + `order_month` 同时给出的说明；账号类筛选只接收 ID
- [x] 7.4 实现实例级行过滤三分支：`IsAny=true` 不叠加过滤；`IDs` 非空叠加 `RuleIn("main_account_id", IDs)`；两者皆空直接返回空列表（count=0，不返回全量、不返回 500）
- [x] 7.5 实现主表级核算派生：`P` = `pushed` 且 `increase` 的条目数，`N` = `increase` 条目总数（**调减条目不计入分子分母**）；`P=0` 待核算 / `0<P<N` 核算中 / `P=N` 已完成
- [x] 7.6 实现累计核算金额 = `pushed` 且 `increase` 的分摊金额之和（不含调减）；不引入时间维度判定
- [x] 7.7 派生值**不落库**：主表不存在「核算状态」或「累计核算金额」物理列
- [x] 7.8 后端始终返回真实值，`settle_state=unsettled` 期间也如实反映，不做按定账状态的遮蔽或置零
- [x] 7.9 单页上限 `core.DefaultMaxPageLimit`（500），超限返回 `errf.InvalidParameter`
- [x] 7.10 确认详情页字段由本接口提供，不另开详情接口
- [x] 7.11 编写列表接口文档，版本标 `v9.9.9+`
- [ ] 7.12 性能验证：主表 10 万条、调账表现网量级，wrk 100 并发单页 500，P99 < 1s

## 8. Wave 2 · S9 现网调账域（bill-adjustment-guard，23h）★ 上线须晚于第 2 组

- [x] 8.1 将 `cmd/account-server/service/bill/billadjustment/bill_adjustment.go` 的 `checkAdjustmentUnconfirmed` 拆分为**来源守卫**（`source=prepaid` 拒绝编辑删除）与**状态守卫**（`pushing` 拒绝、`settled` 拒绝，**仅接入编辑与删除路径**）；批量确认只保留 `confirmed` 拒绝 + 新增 `prepaid` 拒绝，**不得接入状态守卫**
- [x] 8.2 守卫判定复用已有 list 查询结果，不新增额外 DB 查询
- [x] 8.3 改造 `UpdateBillAdjustmentItem`（调用点 `:251`）：`source=manual` 且 `push_status ≠ pushing` 且 `settle_state ≠ settled` 时**放开已确认编辑**（⚠️ 偏离 PRD 十三节）
- [x] 8.4 编辑成功后同步回退 `state` → `unconfirmed`、`push_status` → `unpushed`
- [x] 8.5 改造 `DeleteBillAdjustmentItem`（调用点 `:322`）与 `BatchDeleteBillAdjustmentItem`（调用点 `:355`）：**放开已确认删除**，叠加 `pushing` / `settled` / `prepaid` 守卫
- [x] 8.6 批量删除实现**整批原子拒绝**：ID 列表含任一被拒项时整批拒绝零变更，不允许部分成功
- [x] 8.7 `BatchConfirmBillAdjustmentItem`（调用点 `:297`）**行为不变**：仍拒绝 `confirmed`（不能重复确认），新增 `prepaid` 拒绝，**不判定 `push_status` / `settle_state`**（存量已刷 `settled`，接入状态守卫会让存量待确认调账永久不可确认）。**这是守卫拆分最易出错点，须单独验证不被误放开**
- [x] 8.8 调账 list 接口响应体新增 `push_status` / `settle_state` / `source` 三字段，老字段结构与语义不变（向后兼容）
- [x] 8.9 确认编辑删除的 `AccountBill` + `Update`/`Delete` 鉴权不受守卫改造影响
- [x] 8.10 确认人工路径 `validateResSubClass` 硬校验**保持不变**，不因预付费路径改为服务端填充 `gpu_card` + `gpu_type` 而放松
- [ ] 8.11 **4 处调用点全量回归**：逐点验证行为与守卫矩阵一致，无遗漏调用点
- [ ] 8.12 专项验证**存量数据保护**：存量已刷 `settle_state=settled`，历史已确认调账仍被 `settled` 守卫拒绝编辑删除（本次放开风险可控的关键前提）
- [ ] 8.13 验证**编辑回退的连带影响**：回退为 `unconfirmed`+`unpushed` 的调账重新确认并触发整月同步后，OBS 侧经 clean + 全量插入不出现重复行、金额为编辑后新值
- [x] 8.14 更新调账 list 接口文档，版本标 `v9.9.9+`
- [ ] 8.15 性能验证：新增三字段判定后编辑/删除/批量确认接口响应时间无显著劣化

## 9. Wave 3 · S8 查询域（prepaid-bill-query 明细段，12h）

- [x] 9.1 实现 `POST /api/v1/account/bills/prepaid_items/{id}/split_items/list`，以月度分摊行为主行
- [x] 9.2 通过账期（`bill_year` / `bill_month`）匹配关联调账记录，无错位无重复关联
- [x] 9.3 每行返回调账编号、账期、行级核算状态、调账类型、调账金额、备注；字段清单以 PRD 5.2 为准，可一并返回补充字段供前端按需取用
- [x] 9.4 实现行级核算状态**单条判定**：该条 `push_status=pushed` 即已核算，否则未核算（不用主表级 P/N 聚合口径）
- [x] 9.5 复用第 7 组的实例级鉴权模式，越权请求他人预付费明细返回空或权限错误、不泄露数据
- [x] 9.6 `{id}` 不存在时返回 `errf.RecordNotFound`
- [ ] 9.7 验证覆盖重推后一致性：返回的调账编号全部为新建条目，不出现已物理删除的旧编号
- [x] 9.8 编写整合明细接口文档，版本标 `v9.9.9+`

## 10. 上线收口

- [ ] 10.1 按波次顺序上线：第 2 组 SQL 迁移（含存量刷值）必须先于第 6 组与第 8 组，否则存量保护前提不成立
- [ ] 10.2 SQL 迁移在低峰窗口执行，迁移后逐项验证表结构、存量刷值抽样、索引命中
- [ ] 10.3 确认 `account_server.yaml` 与 helm `values.yaml` 两处配置键名一致且已随发布生效
- [ ] 10.4 **知会运营**：放开 `source=manual` 已确认调账的编辑与删除，改变了既有的「确认即冻结」约束
- [ ] 10.5 回头修订技术方案：§10 第 13 条索引口径（本期未给调账表新建账期索引）；§4.1 `rmb_cost` 改为服务端按币种派生；§4.2 软校验档改为 `gpu_card` + `gpu_type`
- [ ] 10.6 确认回滚不可逆残留已知悉：已置 `settled` 的记录单向不可逆，回滚后仍拒绝调用方覆盖；SQL 新增字段与索引不建议 drop（会丢失存量刷值信息）
