## Why

上游系统是预付费账单（如 AWS SageMaker Training Plan Upfront Fee）的生产方，这类账单一次付费、跨多个自然月使用，与 HCM 现网「按固定账单月份查询」的账单模型不匹配：一笔账单有独立的订单时间与使用起止时间，金额需按使用周期逐月分摊后才能进入 OBS 核算。HCM 当前完全没有预付费账单的表、枚举或代码（全库检索 `account_bill_prepaid` 无匹配），属全新建设域；同时现网调账表 `account_bill_adjustment_item` 只有 `state` 一个状态字段，没有推送状态、定账状态与来源字段，OBS 推送完成后也不回写 HCM，运营无法从 HCM 侧得知某条调账是否已进 OBS。

本期建设上游系统 → HCM 的预付费账单写入链路、定账闸门、推送状态回写与跨账期查询能力，使预付费账单能被逐月分摊为「已确认」调账并随现网月度同步进入 OBS 核算，用户侧全只读、无录入、无审批。

> 口径优先级：**技术方案 > 产品需求**。冲突处已在阶段 1 逐条闭环，见需求文档附录 A 与附录 D。

## What Changes

### 新增能力

- **预付费账单主表 `account_bill_prepaid_item`**：全新表，业务唯一键 `uk_uuid_order_month(uuid, order_year, order_month)`，仅 `unsettled` / `settled` 两态（**无「失败」态**，Q-001→A），配套 table struct / DAO / data-service CRUD（含事务变体）。
- **预付费写入接口 `POST /api/v1/account/bills/prepaid_items/sync`**：批量外壳（`items` 1-100），**逐单独立事务**，失败只记在该单 `results[].message`，接口整体仍返回成功。单单处理顺序为「解码校验 → 云账号映射 → 实例级鉴权 → 业务校验 → 唯一键与双闸门判定 → 单事务落库」，**账号映射必须前置于鉴权**（鉴权实例来自映射结果）。`rmb_cost` **不入请求体**，由服务端按 `currency` 派生（`CNY` 同值，其他币种落 0）。派生调账 `res_class` 恒为 `gpu_card`、`res_sub_class` 取必填 `gpu_type`，调用方**不传**类别字段。
- **N+1 条调账生成与覆盖重建**：N 条 `increase` 落各分摊自然月，1 条 `decrease` 落**订单月份**（非当前月）；覆盖重推 = 该 `source_id` 下旧调账**整组物理删除 + 重建**，新记录回到 `confirmed` / `unpushed` / `unsettled`，主表 ID / `settle_state` / `creator` 保持不变。单事务编排落在 data-service `POST /bills/prepaid_items/sync`，account-server 不跨服务拼事务。
- **删除接口 `DELETE /api/v1/account/bills/prepaid_items/batch`**：按订单年月批量删除主单及其派生调账；可选 `main_account_cloud_ids` 收窄范围。鉴权走 `AccountBillPrepaid + Delete` 三分支；已定账或关联调账 `pushing` 时整批 `Aborted`、零变更；无匹配或无权限返回 `deleted_count=0`。
- **定账定时任务**：account-server **首次引入 cron 基础设施**（此前无任何 cron 任务）。纯时间闸门，锁定时点 = 基准月份**次月 9 日 00:00:00（Asia/Shanghai）**，双表独立判定（主表按订单月份、调账按各自账期），多副本由 `sd.IsMaster()` 去重。
- **OBS 推送状态两端打点**：推送前批量置 `pushing`，整月全部分页插入成功后统一置 `pushed` 并对该账期 `pushing` 残留兜底纠正回 `unpushed`；Flow 失败/取消置 `failed` + `push_fail_reason`。
- **跨账期查询接口**：`prepaid_items/list`（列表，详情页字段亦由此提供）与 `prepaid_items/{id}/split_items/list`（分摊与调账整合明细），均按 `ListAuthorizedInstances(MainAccount, Find)` 做二级账号行级隔离。
- **核算状态与累计核算金额派生**：主表级按条目数口径 `P/N` 派生（**调减条目不计入分子分母**），行级按单条 `push_status=pushed` 判定；**派生读模型不落库**，且后端**始终返回真实值不做遮蔽**（Q-004→A）。
- **IAM 资源类型 `AccountBillPrepaid` 与 action**：写入 `AccountBillPrepaidCreate`、删除 `AccountBillPrepaidDelete` 为两个独立权限点，均挂入「平台管理 / 云账单管理」分组；只做实例级、不设菜单级，权限维度统一为二级账号；查询侧**复用**现网 `MainAccount + Find` 不新增查询类 action。
- **同步与删除审计**：sync 一次调用写且仅写一条（首次 `create` / 重推 `update`）；删除按主单各写一条 `delete`。审计与业务写入在 data-service 同一事务内同生共死；新增 audit `res_type` **必须同时注册**到 `AuditResourceTypeEnums`（现网 `CloudCvmAuditResType` 是漏注册的反例）。

### 现网改造

- **BREAKING（行为变更）**：`account_bill_adjustment_item` 新增 `source` / `source_id` / `push_status` / `push_fail_reason` / `settle_state` 五字段与 `(bill_year, bill_month)` 索引，存量刷安全默认值（`manual` / `pushed` / `settled`）。
- **BREAKING（偏离 PRD v2.12 十三节）**：`checkAdjustmentUnconfirmed` 拆分为来源守卫与状态守卫；`source=manual` 的已确认调账在 `push_status ≠ pushing` 且 `settle_state ≠ settled` 时**放开编辑与删除**，编辑后回退 `state`→`unconfirmed`、`push_status`→`unpushed`。**批量确认行为不变**（仍拒绝已确认）。详见 design 的「显式偏离 #1」。
- **取代原软校验口径**：调用方**不再传入** `res_class` / `res_sub_class`，服务端固定 `gpu_card` + `gpu_type`。人工路径 `validateResSubClass` 硬校验保持不变。详见 design 的 D-13。
- 调账 list 接口响应体新增 `push_status` / `settle_state` / `source` 三字段（新增字段，向后兼容）。
- `enumor.CurrencyCode` 补 `Validate()` 方法（现网无此方法）。

## Capabilities

按限界上下文划分 9 个 capability；查询域承接 S7 + S8 两张卡（共用鉴权模式与「核算 = `push_status=pushed`」口径基线）；删除接口为编码阶段相对原 8 域新增的对外能力，单独成 capability，避免塞进写入域后把 预付费 sync 契约与运营删除契约缠在一起。

### New Capabilities

- `prepaid-sync-contract`（契约域，S1）：sync 批量字段级契约、请求/响应 DTO 与 validator tag、R-015 硬校验、`gpu_type` 必填与类别由服务端填充、`rmb_cost` 按币种派生、错误码语义、`CurrencyCode.Validate()`、APIGW 资源注册与接口文档。
- `prepaid-bill-data-model`（数据域，S2）：预付费主表建表与唯一键及查询索引、table struct 与 DAO、data-service CRUD 与专用 sync 事务接口、调账表新增 5 字段与存量刷值、三个新枚举。时间字段以 `VARCHAR(64)` 存 `YYYY-MM-DD HH:mm:ss`，使用起止未传入时落空串。
- `prepaid-iam-audit-base`（权限审计域，S3）：IAM 资源类型 `AccountBillPrepaid` 与写入/删除两个 action、auth-server `genAccountBillPrepaidRuleResource`、audit `res_type` 新增并注册、同步与删除审计记录构造与事务内写入封装。
- `prepaid-sync-write`（写入域，S4）：sync handler 全链路、云账号映射与实例级鉴权、N+1 生成与覆盖重建、调用 data-service sync 单事务落库、审计事务内集成。
- `prepaid-bill-delete`（删除域）：按订单月批量删除、删除鉴权三分支、定账/推送中双闸门整批拒绝、data-service 级联删除与删除审计、APIGW 与接口文档。
- `prepaid-settle-task`（定账域，S5）：account-server cron 基础设施、定账任务双表独立判定与时间闸门、master 检查、手动触发接口、配置项双写。
- `bill-adjustment-push-status`（推送域，S6）：推送前置 `pushing`、推送后置 `pushed` 与残留兜底纠正、失败置 `failed`、data-service 批量更新接口。
- `prepaid-bill-query`（查询域，S7 + S8）：基于 filter 表达式的列表查询、整合明细查询与账期匹配、实例级行过滤三分支、主表级与行级核算派生（含 `accounted_rmb_cost`）。
- `bill-adjustment-guard`（现网调账域，S9）：守卫拆分、放开 manual 已确认编辑删除与状态回退、批量确认行为不变、调账 list 响应补 3 字段、批量操作整批原子拒绝。

### Modified Capabilities

无。`openspec/specs/` 下现有 `bill-adjustment-res-sub-class` / `bill-adjustment-obs-sync-res-class` 等账单类 spec 覆盖的是 `res_sub_class` 字段口径，与本期改造的守卫、推送状态、来源字段无需求级交叉；本期对现网行为的改动全部落在新建的 `bill-adjustment-guard` 与 `bill-adjustment-push-status` 两个 capability 内，requirement 文本已显式写明「相对现网行为的变化」。

## Impact

### 受影响服务与代码

| 层 | 位置 | 改动性质 |
|---|---|---|
| api-server | 路由透传、`gwparser.Parse` 身份解析 | 新增路由 |
| account-server | `service/bill/` 新增 prepaid 包（sync / delete / list / split / 手动定账）；`service/bill/billadjustment/` 守卫改造；`logics/bill/sync_controller.go`；`task/bill_settle.go`（新建）；`service/service.go` 新增 `initCronTask` | 新增 + 现网改造 |
| task-server | `logics/action/obs/sync/sync_adjustment.go` 推送后回写 | 现网改造 |
| data-service | `service/bill/` 预付费 CRUD、专用 sync 事务接口、级联删除、调账批量更新 `push_status` / `settle_state` | 新增 |
| auth-server | `service/auth/gen_id.go` 新增 `genAccountBillPrepaidRuleResource` | 新增 |
| pkg | `dal/table/bill`、`dal/dao/bill`、`api/{account-server,data-service,core}/bill`、`client`、`criteria/enumor/{bill,audit}.go`、`iam/{meta,sys}` | 新增 + 扩展 |
| SQL | `scripts/sql/9999_20260820_1500_account_bill_prepaid_item.sql` | 建表 + 主表索引 + 5 字段 + 存量刷值（**未**建调账账期索引） |
| 配置 | `cmd/account-server/etc/account_server.yaml` + `docs/support-file/helm/values.yaml` | 双写 |
| 文档 | `docs/api-docs/web-server/docs/resource/bill/`（版本 `v9.9.9+`）、`docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml` | 新增 |

### 外部依赖与阻塞

- **预付费接口契约 Additive 冻结（Q-009→A）**：字段口径已按实现落地（批量 `items`、`gpu_type`、服务端派生 `rmb_cost`）。发调用方确认并标记冻结版本号仍为流程项，冻结后只允许新增字段。
- **IAM 权限模型评审**：`prepaid-iam-audit-base` 的开工前置。代码已按新增 `AccountBillPrepaid` 与两个独立 action 落地；若评审结论改为纯复用，鉴权语义需回退。
- **运营知会**：放开已确认调账编辑删除改变了运营既有的「确认即冻结」约束，上线前需知会到位。

### 本期不包含（防蔓延边界，11 条）

前端页面与交互 · 导出接口（前端本地导出）· 失败重试入口 · 手工调账的增删改审计 · 推送状态流转与定账任务审计 · 审计查询接口与前端操作记录页 · OBS 推送独立告警 · 云账单管理固定月份模型与 OBS 侧接收逻辑改造 · 财务审批/确认/驳回流程 · 预付费接口契约草案本身 · 卡型未命中清单的运营对账巡检。
