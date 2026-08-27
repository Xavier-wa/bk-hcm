## Context

### 现状与缺口

上游系统是预付费账单（如 AWS SageMaker Training Plan Upfront Fee）的生产方。这类账单**一次付费、跨多个自然月使用**，与 HCM 现网「按固定账单月份查询」的账单模型不匹配：一笔账单有独立的订单时间与使用起止时间，金额需按使用周期逐月分摊后才能进入 OBS 核算。

已读代码核实的三条现状事实：

1. **预付费域为全新建设** —— 全库检索 `account_bill_prepaid` 无任何表、枚举或代码匹配。现有 `prepaid` 字样仅出现在 CVM / LB / EIP 的包年包月计费语境，与本期无关。
2. **调账表状态字段单薄** —— `account_bill_adjustment_item` 只有 `state`（`unconfirmed` / `confirmed`）一个状态字段（`pkg/dal/table/bill/billadjustmentitem.go:66-107`），没有推送状态、定账状态与来源字段。
3. **推送链路单向** —— 现网 OBS 调账同步只捞 `state=confirmed`（`cmd/task-server/logics/action/obs/sync/sync_adjustment.go:122`），Run 结束仅打日志（`:150-151`），全程只读 HCM、只写 OBS DB，**不回写 HCM**。运营无法从 HCM 侧得知某条调账是否已进 OBS。

### 口径优先级与已闭环决策

**技术方案 > 产品需求**。14 条冲突已在阶段 1 逐条闭环（需求文档附录 A），9 项阻塞问题 Q-001 ~ Q-009 已于 2026-08-20 由用户逐条决策（需求文档「已闭环决策」章）。本设计一律按闭环结论落地，不复议。

### ⚠️ 阶段 1 读代码推翻的三条技术方案前提

技术方案中有三条「假定成立」的前提经读代码核实**不成立**，本设计按修正后的事实展开。这三条是本文最容易被后续维护者按技术方案原文误读的地方，故前置声明：

| # | 技术方案原文 | 现网实际事实（已核实） | 本设计的处理 |
|---|---|---|---|
| P1 | §10 第 13 条「**确认**有可用索引」（指 `account_bill_adjustment_item` 上的 `(bill_year, bill_month)`） | **索引不存在**。建表仅 `PRIMARY KEY (id)`（`scripts/sql/0023_20240531_1648_bill.sql:128-149`），后续迁移均未添加 | **本期 SQL 未给调账表加该索引**。定账任务按 `(bill_year, bill_month)` 分页扫描，依赖主键回表。预付费主表本期新建了 `idx_order_year_month` / `idx_main_account_id` / `idx_settle_state` |
| P2 | §4.1「`rmb_cost` **复用现网汇率逻辑换算**」 | **没有该现网逻辑**。手工调账 `RmbCost` 是创建请求必填入参；汇率表按 `(year, month)` 存，分摊到未来月时写入时汇率不存在 | **落地口径**：请求体**不接收** `rmb_cost`，由 `deriveRMBCost` 按 `currency` 派生（`CNY` 与 `cost` 同值，其他币种落 0）。HCM 不做汇率换算 |
| P3 | §4.2 让 prepaid 走 `res_class` / `res_sub_class` 软校验 | 现网人工创建路径是**硬校验**（`validateResSubClass`） | **落地口径已取代 Q-007 → B**：调用方**不传**类别字段。派生调账 `res_class=gpu_card`，`res_sub_class` 取必填 `gpu_type`。人工路径硬校验不变 |

> P1 / P2 属技术方案事实错误，需回头修订技术方案；P3 的原软校验档已被 `gpu_type` 口径取代，不再作为本期差异留痕。

### 约束

- 对外接口路径以 `/api/v1/{server}` 开头，接口文档置于 `docs/api-docs/web-server/docs/resource/bill/`，版本标 `v9.9.9+`。
- 可枚举变量定义在 `pkg/criteria/enumor/`，固定字符串定义在 `pkg/criteria/constant/`，每个枚举提供 `Validate()`。
- SQL 文件 `SQLVER=9999` / `HCMVER=v9.9.9`，文件名以 `9999` 开头，置于 `scripts/sql/`。
- 配置项须**双写** `cmd/account-server/etc/account_server.yaml` 与 `docs/support-file/helm/values.yaml`。
- 除 data-service 外其他服务禁止直接操作 DB；client 调用统一经 `pkg/client/common/request.go` 封装。

### 干系人

| 角色 | 关注点 |
|---|---|
| 上游系统 | 写入接口契约稳定、幂等可重推、错误码语义明确 |
| 二级账号负责人 / 产品财管 | 跨账期查询、核算状态与累计核算金额 |
| 账单运营 | 调账列表上可见推送状态与定账状态；prepaid 行不可误操作 |
| 平台管理员 | 每次上游推送有审计可追溯；定账任务可手动触发 |
| IAM 权限模型评审方 | 新增资源类型 `AccountBillPrepaid` 是否成立 |

## Goals / Non-Goals

**Goals:**

- 建成上游系统 → HCM 的**幂等覆盖**写入链路：批量 `items` 逐单独立事务；一笔预付费落 1 条主表记录 + N+1 条「已确认」调账，全部在 data-service **单事务**内完成。
- 提供按订单月**批量删除**预付费主单及其派生调账的能力，受独立删除权限与定账/推送中双闸门约束。
- 用**纯时间闸门**（订单月次月 9 日 00:00:00 Asia/Shanghai）实现定账，HCM 不接受任何外部系统直接触发锁定。
- 给 OBS 推送链路补上**两端打点与兜底纠正**，使「一次整月推送结束后该账期不存在 `pushing` 残留」成为可断言的不变式。
- 提供**跨账期**查询能力（列表 + 整合明细），派生核算状态与累计核算金额，且**派生读模型不落库**。
- 把预付费契约这个**外部不可控依赖收敛到单一前置 capability**（`prepaid-sync-contract`），使 85.7% 的工时不必等上游系统排期。

**Non-Goals（明确不做，对应需求文档「本期不包含」11 条）:**

- 前端页面与交互（预付费列表页/详情页、调账列表新增列的渲染）—— 父需求前端子单承接。
- 导出接口 —— 后端不提供，前端取数后本地导出（PRD 限 1000 条，`core.DefaultMaxPageLimit` 为 500，前端分页取两次）。
- 上游系统写入失败的**重试入口** —— 失败同步返回错误码，由调用方重推。
- 手工调账的新增/编辑/删除审计（现网本无）。
- 推送状态流转（`pushing`/`pushed`/`failed`）与定账任务状态变更审计 —— 属系统批处理而非用户操作。
- 审计查询接口与前端操作记录页展示 —— 本期审计仅供后台排查。
- OBS 推送独立告警通道 —— 复用现网 `BillSyncRecord` 失败态与既有运维告警。
- 「云账单管理」固定月份模型、三方云账单获取/解析/对账上游逻辑、OBS 侧接收处理逻辑的改造。
- 财务审批 / 确认 / 驳回流程。
- **预付费接口契约草案本身不是本 SDD 规划阶段的产出**，而是 `tasks.md` 的第一个工作项。
- 卡型未命中清单的运营对账巡检 —— `gpu_type` 若不在 OBS 厂商卡型清单内仍可能被静默置空，建议后续单独立项。

## Decisions

### D-1 主表状态：两态而非三态（Q-001 → A）

**决策**：`account_bill_prepaid_item.settle_state` 仅 `unsettled` / `settled` 两态。写入失败**不落库任何记录**，同步返回结构化错误码由上游系统凭码重推。表结构中不存在任何「失败」枚举值，也不存在 `fail_reason` 字段。

**备选与取舍**：PRD 6.2 / 4.3 列 22 / 6.7 / AC-8 / AC-14 要求三态（含「失败」）。若保留失败态，需要额外的失败原因字段、失败态的可覆盖语义、失败记录的清理策略，且会在主表沉积「僵尸主单」——一笔从未成功写入的账单在库里占着唯一键位，上游系统重推时还要先判定这条僵尸能否被覆盖。取两态后写入是「要么完整成功要么完全不存在」，唯一键位不会被失败尝试占用。代价是调用方必须自己保留重推所需的原始数据（这本就成立，上游系统是生产方）。

**落点**：`prepaid-bill-data-model`（表结构）+ `prepaid-sync-write`（失败不落库）。锚点 R-018 / AC-035 / AC-036。

### D-2 定账触发：纯时间闸门而非写入动作（Q-002 → A）

**决策**：锁定时点 = **基准月份次月 9 日 00:00:00（Asia/Shanghai）**，即 8 日整日仍可被调用方覆盖重推，9 日零点起转 `settled`。定账与调用方推了几次、是否声明「终稿」**完全无关**；HCM 不接受任何由外部系统直接触发锁定的入参。锁定日可配置，默认 8（语义为「8 日已过」）。

**判定基准双表独立**：主表按订单月份 `order_year`/`order_month`；调账条目按各自账期 `bill_year`/`bill_month`。两者**互不联动**。示例：订单月 2026-07 的主单在 2026-08-09 00:00:00 定账，其分摊到 2027-01 的调增条目要到 2027-02-09 00:00:00 才定账。

**备选与取舍**：PRD 6.1.1 / 6.8 要求「调用方写终稿即锁定」。写入动作决定锁定意味着 HCM 的数据生命周期由外部系统的一个布尔标志控制——上游系统误标终稿会提前锁死账期，漏标则永不锁定。时间闸门把锁定权收回 HCM 自己，代价是**锁定精度等于扫描周期**（最坏 1 小时误差），以及「迟到即终稿」这条固有行为：全新记录若在锁定时点之后才首次写入，会先建为 `unsettled`、再由下次扫描置 `settled`。两条代价都已被显式接受并写成 AC（AC-P04 / AC-T05）。

**调账表额外限定 `state=confirmed`**（待产品确认的口径假设）：「账已定不可再改」的前提是这笔账已经确认过，故人工录入但仍待确认的调账不参与定账。**衍生后果**：与回溯窗口叠加后，一笔在账期滑出窗口之后才被确认的人工调账将不再被任何一轮扫描捞到，长期停在 `unsettled` 因而长期可编辑可删除。不改扫描逻辑（去掉窗口即全表扫，代价大于收益，见 RK-5），处置手段是临时调大 `lookbackMonth`（配置项，重启生效）后手动触发一轮，已写入运维文档。

**落点**：`prepaid-settle-task`。锚点 R-008 / AC-015 / AC-015a / AC-015b / AC-016 / AC-T05。

### D-3 覆盖重推：整组物理删除 + 重建，而非状态转移（Q-007 无关，技术方案 §4）

**决策**：覆盖重推**不是状态转移**，而是「该 `source_id` 下旧调账整组**物理删除** + 重建 N+1 条」。新记录从各字段初始值重新开始（`state=confirmed` / `push_status=unpushed` / `settle_state=unsettled`）。删除与重建在**同一事务**内，不出现调账真空窗口。

**备选与取舍**：备选是「逐条 diff 后增量更新」。diff 方案需要在旧组与新组之间做账期级匹配，分摊月数变化（3 条改 4 条）时要处理新增/删除/修改三类边，且要为每条决定 `push_status` 是否回退——复杂度远高于收益。整组重建天然满足父需求评论「未定账时调用方修改数据后所有调账转为未推送需重新推送」的语义。代价是调账 ID 在重推后全部变化，因此整合明细接口必须断言「返回的调账编号全部为新建条目」（AC-SP06），审计的 `detail.changed` 必须记录被删除的旧组清单（AC-028 / AC-A03）。

覆盖重推时主表 ID 不变；`settle_state` 与 `creator` MUST 保持首次入库值，只刷新业务字段。

**落点**：`prepaid-sync-write`。锚点 R-007 / AC-002 / AC-013 / AC-014。

### D-4 处理顺序：账号映射**前置于**鉴权

**决策**：sync handler 的处理顺序为「① 解码与基础字段校验 → ② 云账号映射 → ③ 实例级鉴权 → ④ 业务校验 → ⑤ 唯一键与双闸门判定 → ⑥ 调用 data-service 单事务落库」。②③ 顺序**不可调换**。映射键为 `(main_account_cloud_id, vendor)`。

**理由**：鉴权实例来自映射结果。调用方传入的是 `main_account_cloud_id` + `vendor`，而鉴权需要的是 `main_account_id`。`root_account_cloud_id` 原样落库、不参与映射。AC-032 显式断言「映射不到时在**鉴权之前**返回 `errf.RecordNotFound`」。

**落点**：`prepaid-sync-write`。锚点 R-016 / AC-031 / AC-032。

### D-5 核算派生：条目数口径、调减不计入、不落库、不遮蔽（Q-003 → A + Q-004 → A）

**决策**：

- 主表级派生：记 `P` = `push_status=pushed` 且 `type=increase` 的条目数，`N` = 该单下 `type=increase` 的条目总数（**分母不含调减条目**）。`P = 0` → 待核算；`0 < P < N` → 核算中；`P = N` → 已完成。
- 累计核算金额 = `push_status=pushed` 且 `type=increase` 的分摊金额之和（**不含**调减条目）。
- 行级派生（整合明细）：**单条判定**，该条 `push_status=pushed` 即「已核算」，否则「未核算」，不做 P/N 聚合。
- **派生读模型不落库**，主表不存在「核算状态」或「累计核算金额」的物理列（AC-L06）。
- 后端**始终返回真实值**，`settle_state=unsettled` 期间也如实反映，不做任何按定账状态的遮蔽或置零。

**备选与取舍**：PRD 6.6.1 用「金额 + 时间维度」判定（累计=总额但当前月 ≤ 最晚分摊月仍算核算中）。时间维度会让同一份数据在不同查询时刻给出不同状态，无法写成稳定的 AC，且与 PRD 自身的 T5「初稿期调账也正常推送」矛盾。落库派生值的备选被否决是因为它会引入第二个真相源——`push_status` 一变就要同步刷主表，覆盖重推时更要重算，而查询期实时聚合的成本在逐月分摊的量级下可忽略。可见性遮蔽交给前端，因为「未定账期间是否展示」是展示决策而非数据事实（未定账期间调账已恒为 `confirmed` 并被整月 Flow 正常推送，确实会产生 `pushed` 条目，遮蔽会掩盖真实数据）。

**落点**：`prepaid-bill-query`（S7 主表级 + S8 行级同 capability，避免两处派生真相源）。锚点 R-011 / AC-021 / AC-022 / AC-022a / AC-L06 / AC-SP02。

### D-6 S7 + S8 合并为单一查询 capability

**决策**：列表查询（S7）与整合明细查询（S8）落在同一个 `prepaid-bill-query` capability。

**理由**：两者共用鉴权模式（`ListAuthorizedInstances(MainAccount, Find)` 三分支）与「核算 = `push_status=pushed`」口径基线。拆成两个 capability 会让核算口径出现两处定义，一旦一侧改动另一侧极易漂移。两者的接口契约本身相互独立（路由、入参、出参不交叉），合并不会造成契约耦合。

**落点**：`prepaid-bill-query`。

### D-7 account-server 首次引入 cron 基础设施

**决策**：account-server 新增 `initCronTask`（`cron.Init` + `cron.Register`），写法参照现网 cron 注册写法；新建 `cmd/account-server/task/bill_settle.go` 实现 `croncore.Task` 四方法（`pkg/cron/core/task.go:30-38`）。

**多副本去重**：`Do(kt)` **首行**做 `sd.IsMaster()` 检查，非 master 直接返回。`pkg/cron/core/scheduler.go` 的调度器**不做 leader 选举**，多副本去重必须由任务自身负责——现网 cron 任务的 master 检查先例。

**备选与取舍**：account-server 此前无任何 cron 任务，现有后台任务是 BillManager / SyncController / ExchangeRateController 三个裸 goroutine（`cmd/account-server/service/service.go:227-236`）。备选是再起一个裸 goroutine + `time.Ticker`。选 cron 是因为 cron 包自带手动触发 API 契约（`GetURL()`）、注册可见性与统一的调度日志，而裸 goroutine 三样都要自己造。代价是引入一个 account-server 此前没有的基础设施，启动路径新增一处初始化——因此 AC-T02 专门做了「`initCronTask` 已执行、任务已注册」的正向断言。

**扫描窗口取舍**：调账表是现网存量表，数据量远大于预付费主表，扫描限定账期**回溯 3 个月**（Q-107）。这是**有意的性能取舍**：超出窗口且仍 `unsettled` 的调账不会被扫描到（AC-T06 显式断言），需在运维文档中说明。

**落点**：`prepaid-settle-task`。锚点 AC-T01 / AC-T02 / AC-T03 / AC-T06 / AC-P03。

### D-8 推送后回写：整月全成功才统一置位，不逐页回写

**决策**：

- **推送前**：SyncController 创建调账 Flow 时（`cmd/account-server/logics/bill/sync_controller.go:511-527`），批量将该 `(vendor, 年, 月)` 下 `state=confirmed` 的调账置 `pushing`。
- **推送后**：`SyncAdjustmentAction.Run` 在**所有分页全部插入成功后**（`sync_adjustment.go:150` 返回前）统一回写：本次实际处理的 ID 集合置 `pushed`，该账期内其余 `pushing` 残留置回 `unpushed`。
- **失败**：Flow 失败或取消时（`sync_controller.go:541-544`）置 `failed` + `push_fail_reason`，随后按现网既有行为重建 Flow（调账回到 `pushing`）。本期不引入重试计数与重试上限。

**为什么不能逐页回写**：`clean` 是插入前**一次性**完成的。中途失败时 OBS 数据本就不完整，此时把已插入页标成 `pushed` 会产生「HCM 说已推送、OBS 实际缺数据」的错账。AC-O01 专门断言这一点。

**为什么 `failed` 放行调用方覆盖**：推送失败后，调用方必须能重新推送修正数据，否则该账期会被永久锁死。这是有意设计，写成 R-003 的「`failed` 放行」与 AC-005 / AC-O03。

**不变式**：一次整月推送结束后，该账期内不存在 `pushing` 残留 —— 要么 `pushed`，要么被兜底纠正回 `unpushed`，要么 `failed`。

**落点**：`bill-adjustment-push-status`。锚点 R-009 / R-010 / AC-017 ~ AC-020 / AC-O01 ~ AC-O05。

### D-9 IAM：写入侧新增资源类型，查询侧复用

**决策**：

- `pkg/iam/meta/resource.go` 新增资源类型常量 `AccountBillPrepaid`（现有账单类为 `AccountBill` `:131-138`、`AccountBillThirdParty`）。
- `pkg/iam/sys/initial_actions.go` 新增写入 action `AccountBillPrepaidCreate` 与删除 action `AccountBillPrepaidDelete`，均关联 `mainaccountResource`（参照 `MainAccountFind` `:819-826`）。写入与删除使用独立权限点，不复用。
- `cmd/auth-server/service/auth/gen_id.go` 新增 `genAccountBillPrepaidRuleResource`，资源类型取 `sys.MainAccount`，按 `a.ResourceID` 填实例（参照 `genMainAccountRuleResource` `:845-864`）。`Create` 映射写入 action 且 ResourceID 必填；`Delete` 映射删除 action，供 `ListAuthorizedInstances` 按 action 拉已授权实例，允许 ResourceID 为空。
- **只做实例级，不设菜单级**，权限维度统一为二级账号。
- **查询侧复用**现网 `ListAuthorizedInstances(MainAccount, Find)`，**不新增查询类 action**（AC-I04）。

**与父需求评论的张力**：父需求评论说「权限完全复用当前逻辑」，技术方案 §9 要求新增资源类型。结论是**查询侧确为复用、写入侧为新增**——写入是一个 HCM 此前不存在的动作（外部系统写账单主数据），没有可复用的现网 action。这条需 IAM 权限模型评审确认，是 `prepaid-iam-audit-base` 的开工前置（见 Open Questions OQ-2）。

**落点**：`prepaid-iam-audit-base`。锚点 R-013 / AC-I01 ~ AC-I04。

### D-10 审计：一次调用一条，事务内同生共死，res_type 必须注册

**决策**：审计对象是「调用方调用本接口做了一次调账同步」或「按订单月删除预付费」这类操作本身，不是逐条调账的资源 CRUD 审计。**一次 sync 调用写且仅写一条**审计（首次 `create` / 重推 `update`），即使本次内容与上次完全一致也照记。删除按被删主单各写一条 `delete`。

写入位置在 data-service 的**同步/删除事务内**，与业务写入同生共死，用 `dao.Audit.BatchCreateWithTx`。因此 `cmd/account-server/logics/audit/audit.go` 保持现有空实现（AC-A05）。

**⚠️ 已知坑（现网有反例）**：新增枚举必须**同时注册**到 `pkg/criteria/enumor/audit.go` 的 `AuditResourceTypeEnums`（`:69-98`），否则 `AuditTable.CreateValidate` 的 `ResType.Exist()` 校验会报 `resource type not support`。现网 `CloudCvmAuditResType`（`:60-61` 定义、`:69-98` **未注册**）即为此坑的现成反例。AC-A01 专门做防回归断言。

`res_id` 取主表 `id`（与 audit 表 `varchar(64)` 对齐，`pkg/dal/table/audit/audit.go:61`）；`res_name` / `cloud_res_id` 留空——调用方的 `uuid` 放 `detail`，不借用 `cloud_res_id` 的「云上资源ID」语义（AC-A04）。

**落点**：`prepaid-iam-audit-base`（res_type 注册 + 构造封装）+ `prepaid-sync-write`（事务内集成）。锚点 R-014 / AC-027 / AC-028 / AC-A01 ~ AC-A05。

### D-11 契约先行：字段口径以实现与 `prepaid-sync-contract` 为准

**决策**：`prepaid-sync-contract` 是 sync 接口契约的**唯一真相源**。编码阶段已按如下口径落地，后续按 Additive 治理——只允许新增字段，禁止删改既有字段：

- 请求为批量 `items`（1-100），逐单独立事务，响应 `results` 与请求顺序对齐，失败原因只放 `message`。
- 账号映射字段为 `(main_account_cloud_id, vendor)`；`root_account_cloud_id` 原样落库，不参与映射。
- `rmb_cost` 不入请求，按 `currency` 派生。
- `gpu_type` 必填；`res_class` / `res_sub_class` 不入请求。
- `usage_start_at` / `usage_end_at` 非必填；`device_num` / `card_num` 非必填，只拒绝负数。

发调用方确认并标记冻结版本号仍为流程项（tasks 1.5），不阻塞已落地代码。

**落点**：`prepaid-sync-contract`。锚点 AC-C01 / AC-C02。

### D-12 ⚠️ 显式偏离 #1：放开 `source=manual` 已确认调账的编辑与删除（Q-005 → B）

> **这是对 PRD v2.12 十三节的显式偏离，不是实现 bug。** 记录在此是为了让评审者、测试与后续维护者能一眼看到「这里和 PRD 写的不一样，是有意的」。

| 项 | 内容 |
|---|---|
| **PRD 原文** | 十三节「明确不涉及内容」：「不涉及『账单调整』页手工录入/确认流程的改造；调账状态字段及其『仅已确认参与 OBS 推送』逻辑保持不变」 |
| **本期实际做法** | `source=manual` 的调账在 `push_status ≠ pushing` **且** `settle_state ≠ settled` 时**可编辑、可删除**；编辑后 `state` 回退 `unconfirmed`、`push_status` 回退 `unpushed`（使其重新走「确认 → 推送」链路） |
| **偏离依据** | 用户 2026-08-20 对 Q-005 选择 **B** + 技术方案 §10 第 2/3/4 条 |
| **Agent 原建议** | **A（不放开）**。理由：本需求的 prepaid 调账本就禁止人工编辑，放开 manual 不是本需求的必要条件，且改变运营既有的「确认即冻结」约束。**用户选择 B，按 B 执行，不再复议。** |
| **风险等级** | **中高**。改变现网行为 + 偏离 PRD |

**技术实现**：拆分 `checkAdjustmentUnconfirmed`（`cmd/account-server/service/bill/billadjustment/bill_adjustment.go:372-402`）为两类守卫：

- **来源守卫**：`source=prepaid` 一律拒绝编辑与删除（新增，本需求必需）；
- **状态守卫**：`push_status=pushing` 拒绝、`settle_state=settled` 拒绝（新增），**仅接入编辑与删除路径**；批量确认仍拒绝 `confirmed`（**现网行为保留**），且**不判定** `push_status` / `settle_state` —— 存量调账已刷 `settled`，把状态守卫接进确认路径会让存量待确认调账永久不可确认。

**守卫矩阵（改造后）**

| 操作 | `source=prepaid` | `push_status=pushing` | `settle_state=settled` | `state=confirmed` |
|---|---|---|---|---|
| 编辑 | 拒绝 | 拒绝 | 拒绝 | **放行**（现网拒绝 → 本期改变） |
| 删除 / 批量删除 | 拒绝 | 拒绝 | 拒绝 | **放行**（现网拒绝 → 本期改变） |
| 批量确认 | 拒绝 | — | — | 拒绝（**现网行为保留**） |

**4 个受影响调用点（必须全量回归，一个都不能漏）**

| # | 调用点 | 行号 | 回归要点 |
|---|---|---|---|
| 1 | `UpdateBillAdjustmentItem` | `:251` | 未确认可编辑（现网行为不回归）；已确认可编辑且编辑后 `state` 回退 `unconfirmed`、`push_status` 回退 `unpushed`；`pushing` / `settled` / `prepaid` 三种情况被拒 |
| 2 | `BatchConfirmBillAdjustmentItem` | `:297` | 未确认可确认；已确认**仍被拒**（**不得因守卫拆分被误放开**——这是拆分最易出错点）；`prepaid` 被拒 |
| 3 | `DeleteBillAdjustmentItem` | `:322` | 单条删除：未确认与已确认均可删；`pushing` / `settled` / `prepaid` 被拒 |
| 4 | `BatchDeleteBillAdjustmentItem` | `:355` | 批量删除：混合 ID 列表（含被拒项）时**整批拒绝且零变更**，不允许部分成功 |

**风险缓解（四道）**

1. **`settled` 守卫兜住全部存量历史数据** —— 存量调账刷 `settle_state=settled`，放开动作**不波及现网历史数据**。这是本次放开风险可控的**关键前提**，须专项验证（AC-034a）。
2. **`pushing` 守卫**防止推送过程中被改。
3. **批量操作整批原子拒绝**，不允许部分成功（AC-034）。
4. **上线前须知会运营** —— 放开改变了运营既有的「确认即冻结」约束（需求文档「约束条件 → 依赖限制」已列）。

**已知连带影响**：编辑已推送（`pushed`）的调账会把 `push_status` 打回 `unpushed`，该条目会在下次整月同步时重新进 OBS。需验证 OBS 侧 clean + 全量插入**不产生重复行**（AC-034b）。

**落点**：`bill-adjustment-guard`。锚点 R-017 / AC-033 / AC-034 / AC-034a / AC-034b / AC-034c / AC-G03。

### D-13 ⚠️ 原软校验档已取消：类别由服务端按 `gpu_type` 填充

> **取代 Q-007 → B。** 编码阶段不再让 调用方传入 `res_class` / `res_sub_class`，因此也不再做软校验 Warn 放行。

| 项 | 内容 |
|---|---|
| **被覆盖的结论** | 原 Q-007 → B「调用方传入类别、HCM 只记 Warn」 |
| **本期实际做法** | 请求体 MUST NOT 含 `res_class` / `res_sub_class`。N+1 条派生调账固定 `res_class=gpu_card`，`res_sub_class` 取该单必填 `gpu_type` |
| **仍保留的差异** | 人工录入路径继续走 `validateResSubClass` **硬校验**；prepaid 路径不经过该函数 |

**已知残留代价**：若 `gpu_type` 不在 OBS 厂商卡型清单内，OBS 侧 `splitAdjustmentResSubClass` 的 default 分支仍会把 GPU 卡型字段静默置空。这不再是「软校验放行」，而是 GPU 型号与 OBS 清单不对齐的下游行为，建议后续立项对账巡检。

**落点**：`prepaid-sync-contract` + `prepaid-sync-write`（填充）+ `bill-adjustment-guard`（人工硬校验不变）。

### D-14 层次归属与调用链

| 层 | 位置 | 改动性质 |
|---|---|---|
| api-server | 路由透传、`gwparser.Parse` 身份解析 | 新增路由（sync / delete / list / split / 手动定账） |
| account-server | `service/bill/billprepaid/`（sync、delete、query、settle）；`service/bill/billadjustment/` 守卫改造；`logics/bill/sync_controller.go`；`task/bill_settle.go`；`service/service.go` 新增 `initCronTask` | 新增 + 现网改造 |
| task-server | `logics/action/obs/sync/sync_adjustment.go` 推送后回写 | 现网改造 |
| data-service | 预付费 Create / Update / List / BatchDelete；专用 `POST /bills/prepaid_items/sync` 单事务编排；级联删除；调账 `push_status` / `settle_state` 批量更新 | 新增 |
| auth-server | `genAccountBillPrepaidRuleResource`：`Create` → `AccountBillPrepaidCreate`（ResourceID 必填）；`Delete` → `AccountBillPrepaidDelete`（允许空 ResourceID） | 新增 |
| pkg | `dal/table/bill`、`dal/dao/bill`、`api/{account-server,data-service,core}/bill`、`client`、`criteria/enumor/{bill,audit}.go`、`iam/{meta,sys}` | 新增 + 扩展 |
| SQL | `scripts/sql/9999_20260820_1500_account_bill_prepaid_item.sql` | 建表 + 5 字段 + 存量刷值；主表索引；**未**建调账表 `(bill_year, bill_month)` 索引 |
| 配置 | `cmd/account-server/etc/account_server.yaml` 的 `billSettle` + helm `values.yaml` | **双写** |
| 文档 | `docs/api-docs/web-server/docs/resource/bill/`（sync / delete / list / split，版本 `v9.9.9+`）、`bk_apigw_resources_bk-hcm.yaml` | 新增 |

**单事务边界**：account-server 不跨服务拼事务。写入调用 data-service `SyncBillPrepaidItem`；删除调用 data-service `BatchDeleteBillPrepaidItem`（内部 AutoTxn 级联删调账 + 主单 + 审计）。

**时间字段存储**：主表 `usage_start_at` / `usage_end_at` / `order_at` 为 `VARCHAR(64)`，格式 `constant.DateTimeLayout`。使用起止列为 `NOT NULL DEFAULT ''`，未传入或覆盖重推清空时落**空串**（不是 NULL、不是零值时间）。table struct 用 `*string` 承载使用起止。

**批量与分片**：批量创建按 `constant.BatchOperationMaxLimit`（100）分片；查询单页上限 `core.DefaultMaxPageLimit`（500）；删除 ID 按 `filter.DefaultMaxInLimit` 分片。

### D-15 删除接口：按订单月整批、双闸门、独立权限

**决策**：提供 `DELETE /api/v1/account/bills/prepaid_items/batch`，按 `order_year` + `order_month` 批量删除，可选 `main_account_cloud_ids` 收窄。鉴权使用 `ListAuthorizedInstances(AccountBillPrepaid, Delete)` 三分支（与查询同构，但 action 独立）。无权限或无匹配返回 `deleted_count=0`。

**双闸门与 sync 对齐**：任一目标主单 `settled` 或任一关联调账 `pushing` → 整批 `errf.Aborted`、零变更。`failed` 不阻挡删除。

**级联**：data-service 同事务删除该 `source_id` 下全部派生调账与主单，并为每条主单写 `action=delete` 审计。

**落点**：`prepaid-bill-delete` + `prepaid-iam-audit-base`。

## Risks / Trade-offs

| # | 风险 | 缓解 |
|---|---|---|
| RK-1 | **预付费契约 Additive 冻结尚未完成**（tasks 1.5）。字段口径已按代码落地，冻结后只允许新增字段 | → 契约收敛到 `prepaid-sync-contract`；后续字段改动走影响评估 |
| RK-2 | **放开 manual 已确认调账编辑删除**（D-12），运营原本受「确认即冻结」保护，误改已推送条目会触发该条重新进入推送链路。**中高风险** | → 四道缓解：`settled` 守卫兜住全部存量（AC-034a 专项验证）+ `pushing` 守卫 + 批量整批原子拒绝（AC-034）+ 上线前知会运营；4 个调用点全量回归（AC-G03），其中 `BatchConfirm` 的「已确认仍被拒」是拆分最易出错点，单列 AC-034c |
| RK-3 | **`gpu_type` 不在 OBS 卡型清单内会静默置空**（D-13 残留）。HCM 按 `gpu_card` + `gpu_type` 落库成功，OBS 同步 default 分支把 GPU 字段置空 | → 建议后续立项运营对账巡检（不在本期）；人工路径硬校验不变 |
| RK-4 | **现网调账表 DDL + 存量刷值**。加 5 字段 + 刷值，回归范围覆盖全部调账读写路径。本期 **未** 给调账表加 `(bill_year, bill_month)` 索引 | → 存量刷**安全默认值**（`manual` / `pushed` / `settled`）：刷 `pushed` 使历史调账不被推送打点重新置 `pushing`（AC-O05）；刷 `settle_state=settled` **只覆盖 `state='confirmed'` 的行**（AC-034a），待确认保持 `unsettled`。刷值与建字段须在同一 SQL 文件内完成；存量量大时分批 UPDATE |
| RK-5 | **定账任务回溯窗口 3 个月是有意的性能取舍**（Q-107）。超窗口且仍 `unsettled` 的调账不会被扫描到。两个已知触发场景：任务连续停机超过窗口；人工调账在账期滑出窗口之后才被确认（`state=confirmed` 条件的衍生后果，见 D-2） | → AC-T06 显式断言该行为；两个场景与处置步骤均写入运维文档：`lookbackMonth` 为配置项，临时调大 → 重启生效 → 手动触发一轮补齐 → 调回默认值；如运营不接受需改全量扫描并重新评估 AC-P03 |
| RK-6 | **锁定精度 = 扫描周期**（最坏 1 小时误差），且存在「迟到即终稿」行为 | → 已写成 AC-P04 / AC-T05，属被接受的固有行为而非缺陷；扫描间隔可配置 |
| RK-7 | **account-server 首次引入 cron**，启动路径新增初始化；多副本若漏做 master 检查会重复执行 | → `Do(kt)` 首行 `sd.IsMaster()`（现网先例 `sync_device_type_physical_rel.go:84-87`）；AC-T01 断言非 master 直接返回（日志可证）；AC-T02 正向断言 `initCronTask` 已执行并注册 |
| RK-8 | **IAM 权限模型评审可能改为纯复用**（不新增资源类型，附录 A 冲突 #9） | → 若评审结论改变，`prepaid-iam-audit-base` 范围需回退且 `prepaid-sync-write` 的写入鉴权语义需改。建议 Wave 1 启动**同时**发起评审，不要等到写入域开工才发现分叉（见 OQ-2） |
| RK-9 | **audit res_type 漏注册**会在运行期报 `resource type not support`（现网 `CloudCvmAuditResType` 即为反例） | → AC-A01 防回归断言：`ResType.Exist()` 返回 true 且 `CreateValidate` 不报错 |
| RK-10 | **`prepaid-sync-write` 恰为 24h 上限**，若契约冻结后调用方提出字段改动有超阈值风险 | → 契约冻结后重新核对工时；若超 24h，可把「同步审计事务内集成」（约 2h）回归到 `prepaid-iam-audit-base` 消费侧处理 |
| RK-11 | **虚假独立**：数据域的存量刷值是现网调账域（AC-034a）与推送域（AC-O05）存量保护的前提 | → 上线顺序**必须按波次**，`bill-adjustment-guard` 不可先于 `prepaid-bill-data-model` 上线。已在 `tasks.md` 波次编排中固化 |
| RK-12 | **覆盖重推后调账 ID 全部变化**（D-3 的固有代价） | → 整合明细接口断言「返回的调账编号全部为新建条目，不出现已删除的旧编号」（AC-SP06）；审计 `detail.changed` 记录被删旧组清单（AC-A03） |

## Migration Plan

### 部署顺序（须严格按波次，不可乱序）

```
Wave 1（三卡零依赖并行）
├─ prepaid-sync-contract   ── 字段口径已按代码落地；发调用方确认冻结仍为流程项
├─ prepaid-bill-data-model ── SQL 迁移（建表 + DDL + 存量刷值 + 主表索引）★ 上线闸门
└─ prepaid-iam-audit-base  ── 【外部里程碑：IAM 权限模型评审】

Wave 2
├─ prepaid-sync-write
├─ prepaid-bill-delete
├─ prepaid-settle-task
├─ bill-adjustment-push-status
├─ prepaid-bill-query（列表段）
└─ bill-adjustment-guard

Wave 3
└─ prepaid-bill-query（整合明细段）
```

★ **`prepaid-bill-data-model` 的 SQL 迁移是 `bill-adjustment-guard` 与 `bill-adjustment-push-status` 的硬前置**（存量刷值是它们存量保护的前提，见 RK-11）。

### SQL 迁移步骤

1. 低峰窗口执行 `scripts/sql/9999_20260820_1500_account_bill_prepaid_item.sql`（`SQLVER=9999` / `HCMVER=v9.9.9`）。
2. 建 `account_bill_prepaid_item` 表 + 唯一键 `uk_uuid_order_month` + `idx_main_account_id` / `idx_order_year_month` / `idx_settle_state`。时间列 `usage_start_at` / `usage_end_at` / `order_at` 为 `VARCHAR(64)`，使用起止 `NOT NULL DEFAULT ''`。
3. `account_bill_adjustment_item` 加 5 字段并在同一文件内刷存量：`source=manual`；`state='confirmed'` 的行刷 `push_status=pushed` 且 `settle_state=settled`；待确认行保持列默认 `unpushed` / `unsettled`。
4. **本期不建**调账表 `idx_bill_year_month`。定账任务按账期分页扫描，预付费主表走 `idx_order_year_month`。
5. 验证：表结构逐字段比对、存量刷值抽样 `confirmed` 行无 NULL、主表按订单月查询命中 `idx_order_year_month`。

### 配置项落地（双写）

`cmd/account-server/etc/account_server.yaml` 与 `docs/support-file/helm/values.yaml` **两处键名必须一致**（AC-T04）：锁定日（默认 8）、扫描间隔（默认 1 小时）、回溯窗口（默认 3 个月）。

### 上线前置

- ⛔ 发调用方确认契约冻结版本（流程项，不阻塞已落地代码）。
- ⛔ IAM 权限模型评审通过（权限审计域开工前置；代码已按新增资源类型落地）。
- ⛔ **知会运营**：放开 manual 已确认调账编辑删除改变了既有的「确认即冻结」约束。
- 定账任务回溯窗口 3 个月、调账表无 `(bill_year, bill_month)` 专用索引，须写入运维说明。

### 回滚策略

| 层 | 回滚方式 |
|---|---|
| 代码 | 按 capability 粒度回滚。查询域、定账域、推送域、现网调账域相互独立，可单独回滚 |
| 定账任务 | 关闭 cron 注册即停止定账，已置 `settled` 的记录**不可逆**（R-008 单向不可逆），回滚后这些记录仍拒绝调用方覆盖 —— 这是回滚的**不可逆残留**，须知悉 |
| 现网调账守卫 | 回滚守卫改造即恢复「确认即冻结」。回滚前已被编辑并回退为 `unconfirmed`+`unpushed` 的记录需人工复核 |
| SQL | 新增字段与索引可保留（向后兼容，老代码不读这些列）。**不建议 drop 字段** —— 存量刷值信息会丢失，重新上线时需重刷 |
| 调账 list 接口 | 响应体为**新增字段**，向后兼容，前端老版本忽略即可 |

## Open Questions

### 待外部闭环（不阻塞本 SDD 规划，阻塞下游编码）

| ID | 问题 | 状态 | 阻塞对象 |
|---|---|---|---|
| OQ-1 | **上游系统接口字段级契约 Additive 冻结**（口径已按代码落地，发调用方确认并打版本号仍为流程项） | 🟡 代码已实现；tasks 1.5 未完成 | 不阻塞编码；阻塞对外契约承诺 |
| OQ-2 | **IAM 权限模型评审**：新增 `AccountBillPrepaid` 与独立删除 action 是否成立 | 🟡 代码已按新增落地 | 若结论改变，写入/删除鉴权语义需回退 |
| OQ-3 | **运营知会**：放开 manual 已确认调账编辑删除，改变既有操作约束 | 🟡 上线前须完成 | 上线闸门，不阻塞编码 |

### 非阻塞假设（按建议假设推进，编码阶段如有异议再回头确认）

| ID | 问题 | 采用的假设 | 落在 capability |
|---|---|---|---|
| Q-101 | `push_status` 是否需要「超时」态 | **不需要**，4 态即可（超时在现网表现为 Flow 失败，已被 `failed` 覆盖） | data-model / push-status |
| Q-102 | OBS 推送失败是否要独立告警 | **本期不做**，复用现网 `BillSyncRecord` 失败态与既有运维告警 | push-status |
| Q-103 | 「`currency` 与该二级账号一致」的比对基准 | 比对该二级账号所属 summary root 的 `currency`（现网创建路径即取 `summaryRoot.Currency`，`bill_adjustment.go:129`） | sync-contract |
| Q-104 | 每月 2 号在 HCM 侧有判定作用吗 | **无**（假设 A-04）。HCM 只认 8 日闸门 | settle-task |
| Q-105 | 高级筛选是「订单时间」还是「订单月份」；账号类筛选传 ID 还是名称 | 按**订单月份**（与唯一键一致）；账号类筛选**只接收 ID**，名称由前端转换 | bill-query |
| Q-106 | 整合表字段以 PRD 5.2 还是「已闭环 #2」为准 | 以 **5.2** 为准，后端可一并返回 #2 字段供前端按需取用 | bill-query |
| Q-107 | 定账任务扫描调账表的账期回溯窗口 | **3 个月**（有意的性能取舍，AC-T06 显式断言） | settle-task |
| Q-108 | sync 接口是否需要限流 | 依赖 **APIGW 侧限流**，HCM 不自建 | sync-contract |
| Q-109 | 同一 `(uuid, 订单月)` 并发重推如何处理 | 依赖**唯一键 + 事务**，冲突返回 `errf.RecordDuplicated` 由调用方重试 | data-model / sync-write |

### 需回头修订的上游文档

| 文档 | 修订点 |
|---|---|
| 技术方案 §10 第 13 条 | 「**确认**有可用索引」→ 本期未给调账表新建该索引，定账扫描按账期分页 |
| 技术方案 §4.1 | 「`rmb_cost` 复用现网汇率逻辑换算」→「请求不传 `rmb_cost`，服务端按币种派生」 |
| 技术方案 §4.2 | 软校验档 → 服务端固定 `gpu_card` + `gpu_type` |
