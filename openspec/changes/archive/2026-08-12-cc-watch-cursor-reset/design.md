# Design: cc-watch-cursor-reset

## Context

CC 主机增量同步链路：cloud-server（仅 master）每租户拉起 2 条常驻 watch 流（host / host_relation，cursor 相互独立），单流内严格串行——每轮循环从 etcd 重读 cursor（`getEventCursor`）→ `ResourceWatch` 长轮询拉一批事件 → 消费 → 提交本批最后一个 cursor（`setEventCursor`，消费失败也照样提交）。cursor 存储在 etcd：`/hcm/event/cc/{tenantID}/{resType}`，写操作通过 watcher 持有的 lease 完成。

现状缺口：消息堆积落后太久时，没有应急手段让消费位置跳过积压；失败批次不重试，被跳过的变更只能等 6h 全量同步补齐。已有的唯一自愈逻辑是 CC 返回事件链节点不存在（1103007）时自动把 cursor 置空（从当前时间重新 watch），但该逻辑只在「cursor 已死」时触发，覆盖不了「cursor 活着但落后太多」的场景，也无法人工干预。

约束：

- 「cursor 为空 = 从当前时间起 watch」是 **CC 侧的判定**，HCM 不做任何时间换算，只是把空串原样下发（`param.Cursor = cursor`，`StartFrom` 全项目从未赋值）。这一语义与 1103007 自愈逻辑共用同一条写路径。
- cursor 单写者语义必须保持：watch 流串行提交，不能出现多写者并发竞争 etcd。
- admin 路径下接口直连、不走网关、系统内部调用无需鉴权（与现有 `/admin/system/init` 一致）。
- CC 增量同步只覆盖 `tcloud-ziyan` / `other` 两个厂商的主机；补偿范围与此对齐。

## Goals / Non-Goals

**Goals:**

- 提供 admin 接口 `POST /api/v1/cloud/admin/system/cc_sync/watch/cursor/reset`，支持按 tenant + resType 把消费位置重置到最新，跳过全部积压。
- 写入 reset flag（复用 watcher 的 lease 写 etcd），下一轮 watch 循环消费 flag 并将 cursor 置空，无需重启、无需通知机制。
- 重置成功后异步触发该租户下 ziyan/other 的主机补偿同步，补齐被跳过的主机变更。
- 补偿与定时全量 / 主动按账号同步的互斥：对齐主动按账号触发的范式（DB cvm syncing + 账号锁），进行中的账号跳过，不报错、不影响定时全量正常进行。
- 重置前后 Warn 级审计日志；响应回显重置结果。

**Non-Goals:**

- **本期不实现按指定时间/指定事件重置**（方案见「后续演进」一节，留待后续立项）。
- 不改定时全量编排、不加租户级全量锁、不引入新的 etcd 锁命名空间。
- 不补偿非主机资源，不补偿 ziyan/other 以外的厂商。
- 不改变正常 watch 消费流程、不引入 cursor 多写者、不为 watch 循环加通知/中断机制。
- 不做失败批次重试、不做积压自动检测与自动重置（本期只提供人工应急入口）。
- 不新增「查询当前 cursor」接口。
- 主动触发的补偿同步不做 master 校验（与主动按账号触发一致；定时全量仍只在 master 执行）。

## Decisions

### 1. 接口落在 admin 路径，复用 watcher 写 cursor

- 路由注册在 `cmd/cloud-server/service/admin/`，与现有 `Init` 同级：`POST /cc_sync/watch/cursor/reset`（handler 文件名 `cc_watch_cursor.go`）。
- `Watcher` 新增导出方法 `ResetEventCursor(kt, resType)`，目标租户取 `kt.TenantID`，内部先 `getEventCursor` 读旧值（审计用）、再写 reset flag key（lease 写 etcd）。
- watcher 实例通过 `capability.Capability` 注入 admin 服务（装配点在 `service.go`，与 `NewWatcher` 同一处）。

备选方案：admin 侧自建 etcd client 重写 cursor 写逻辑。放弃——会复制 lease 管理逻辑，且两处写路径易不一致；复用 watcher 能保证「cursor 写入只有一套实现」。

### 2. 生效机制：写 reset flag，不直接写 cursor

直接写 cursor 存在竞态：reset 将 cursor 置空后，master 节点在途的 watch 轮次结束时会把当轮 cursor 提交回 etcd，覆盖刚写入的空值，导致 reset 白做。

改为写独立的 reset flag key（`/hcm/event/cc-reset/{tenantID}/{resType}`），cursor 的写入权始终只在 watch 循环手里：

- `ResetEventCursor` 只写 reset flag，不碰 cursor key。
- watch 循环每轮迭代开头先 `needResetCursor`：若 flag 存在则删除 flag 并将 cursor 置空，然后正常从空 cursor 开始 watch。
- 正在执行中的当轮消费不被打断，reset 在当前批次完成后下一轮生效。

这样 reset 与 watch 循环之间无需互斥，也无竞态。

### 3. 本期只重置到最新，校验因此收敛

调用方不传 cursor，接口语义固定为「置空 → CC 从当前时间起 watch」。因此本期校验只有两项，均失败返回 `InvalidParameter`：

- `resource` 白名单：`host` / `host_relation`（当前 watch 流仅有的两种）；
- 目标租户存在性：复用 `logics/tenant.CheckTenantExist`，与 watch 拉起 goroutine 使用同一数据源。

**目标租户取请求头 `X-Bk-Tenant-Id`（即 `kt.TenantID`），body 不再传 `tenant_id`**：直连场景（`defaultParser` / `FromHeader`）下该 header 就是调用方自己设置的普通请求头，没有任何系统侧强制覆写，调用方调用时把目标租户写进 header 即可，body 重复传一份没有增量信息。header 租户缺失或格式非法会在 `kt.Validate()` 阶段被拦截，存在性再由 `CheckTenantExist` 兜底，不会落到 `/hcm/event/cc/system/{resType}` 这类无 watch goroutine 的 key 上。

由于不接受外部传入的 cursor，cursor 本地解码校验、事件时间「过期窗口」校验、以及 CC `ResourceWatch` 活性探测**本期全部不需要**——写入的空值与 1103007 自愈路径完全一致。

### 4. 主机补偿同步：对齐主动按账号触发，给定时全量让路

重置会跳过积压事件，因此重置成功后必须补齐被跳过的主机变更。最终方案收敛如下。

**范围**：

- 只同步主机（`SyncHostResource`），不同步其他资源。
- 只覆盖 CC 增量涉及的厂商：`tcloud-ziyan`、`other`。
- 编排入口是通用能力 `SyncHostsByTenant(kt, vendors, cliSet)`，目标租户取 `kt.TenantID`，厂商集合由调用方传入，不写死在函数内。

**触发方式**：

- handler 内 `go SyncHostsByTenant(asyncKt, ...)` 异步触发，接口立即返回 `pre_cursor`；`asyncKt` 由请求 kit 派生（`NewSubKitWithCtx(context.Background()).WithAsyncSource()`），脱离请求 ctx 避免响应返回后同步被中断。
- 主动触发，不做 `IsMaster()` 校验（与主动按账号触发一致）。

**互斥（核心决策）**：

补偿只关心「主机是否在同步」，不抢账号级锁，避免被同账号其他资源（如条件同步 CLB/SG）误伤：

| 检查项 | 来源 | 命中时行为 |
|---|---|---|
| DB `account_sync_detail` 中该账号 `res_name=cvm` 且 `res_status=syncing` | 定时全量 / 主动全量 / 兜底写入的同一条记录 | 跳过该账号 |
| `lock.Manager.TryLock(lock.ResKey(accountID, cvm))` | 主机补偿专用锁，与账号级 `lock.Key(accountID)` 隔离 | 跳过该账号 |

因此：

- **兜底给定时/主动让路**：DB 中 cvm 已在 syncing 时，补偿跳过该账号，不抢、不报错、不影响正在进行的同步。
- **不被其他资源误伤**：账号级锁被条件同步或其他资源同步占用时，只要 cvm 未在 syncing，补偿正常触发。
- **不引入租户级锁 / 不改定时全量**：定时全量本身无锁的现状保持不变；补偿不试图「公平抢锁拦全量」。
- **DB 状态冲突可覆盖**：补偿同步写入的也是同一条 `account_sync_detail` 记录，覆盖行为与主动触发一致，暂时不管，只要不影响定时全量正常进行、不因冲突报错返回即可。

**并发模型**（`sync_host.go`）：

- 厂商串行遍历调用方传入的 `vendors`。
- 单厂商内账号分页拉取，页内用 `errgroup` + `constant.SyncConcurrencyDefaultMaxLimit` 并发。
- 单账号内：查 DB → 抢锁 → `SyncHostResource` → 解锁；跳过/失败不中断其他账号。

**为何不复用 `SyncAllResource` / 不加「只同步主机」参数**：另开 `SyncHostResource` 入口，避免给全量路径加分支、保持全量调用点零改动；本期只改 ziyan/other 两个厂商的实现。

**废弃的候选方案（曾讨论，不采用）**：

1. 租户级 `FullSyncKey` / `CompensateSyncKey`：与账号级锁域不相交，拦不住主动按账号路径；且定时全量无锁，探测式抢锁语义脆弱。
2. 给定时全量加锁：改动核心链路，超出本期范围。
3. 补偿触发全量（所有资源）：过度，CC 增量只涉及主机。

### 5. 审计与回显

- 重置前、后各打一条 Warn 级日志，包含旧 cursor、tenant、resType、rid。
- 响应回显 `pre_cursor`（重置前 cursor；空表示重置前已处于「从当前时间起」）。
- 主机补偿始终异步触发；逐账号跳过细节落在同步编排内的 Info/Warn 日志，不回灌到接口响应。`ResetWatchCursorResult.Msg` 字段预留（当前始终为空，表示已触发）。

## 后续演进：按指定时间重置（本期不实现）

运维视角更自然的入参是时间而非 base64 cursor。CC 的 watch 协议确实支持按时间（`bk_start_from`），但**不能与 cursor 同时使用**——同款协议实现中该约束是硬校验：

```go
// pkg/dal/watch/watch.go
// use either StartFrom or Cursor.
if w.StartFrom != 0 && len(w.Cursor) != 0 {
    return errors.New("bk_start_from and bk_cursor can not use at the same time")
}
```

因此**不建议**让 watch 主循环支持 `StartFrom`：`watchCCEvent` 的 `param` 是在 for 循环外创建、每轮复用的同一个结构体，一旦某轮设了 `StartFrom` 而未清零，下一轮 cursor 有值即触发上述互斥错误，整条 watch 流会持续报错卡死。

推荐方案（后续实现时采用）：**时间只作为接口入参，在 admin 接口内部换算成 cursor**——收到时间 T 后，接口自身发起一次 `ResourceWatch(StartFrom: T, Cursor: "")` 探测，从返回结果中取出该时间点对应的 cursor，再把 **cursor** 写入 etcd。这样：

- etcd 与 watch 循环始终只有 cursor 一种语义，零改动、零冲突；
- 时间→cursor 的换算交给唯一持有事件链的 CC，同时天然完成活性校验；
- 无需在 HCM 侧维护「过期窗口」常量。

实现时需明确的精度问题：探测返回 `watched=true`（T 之后已有堆积）时，API 只能拿到事件级 cursor，拿不到「T 这一刻」的位置；存 `Events[0].Cursor` 意味着下一轮从该事件之后开始，首条事件不会被增量消费到（由随后的主机补偿补齐）。`watched=false` 时 CC 返回的是定位节点，不存在该问题。

## Risks / Trade-offs

- 跳过中间未消费事件，数据在窗口期内不一致 → 接口定位应急操作；审计日志留痕；重置后异步主机补偿缩短补齐窗口。
- 补偿与定时全量可能对同一账号并发写 `account_sync_detail`（后写覆盖）→ 与主动按账号触发的现有行为一致，可接受；补偿侧跳过已 syncing 的账号，尽量给定时让路。
- 账号锁 TTL（20min）无 KeepAlive，长任务中途过期是既有常态 → 补偿沿用同一锁语义，不在本期单独加固。
- 重置写入与在途轮次的 cursor 提交存在竞态 → 已通过 reset flag 机制解决：reset 只写 flag key，cursor 写入权始终只在 watch 循环手里，无竞态。
- 置空语义与现有 1103007 自愈逻辑完全一致（同一条写路径、同一个值），无新增风险。

## Open Questions

- 是否需要配套「查询当前 cursor」接口（本期不做；如有需要后续单独立项）。
- 补偿与定时全量并发写同一条同步状态记录的长期治理（本期沿用主动触发的覆盖语义，暂不处理）。
