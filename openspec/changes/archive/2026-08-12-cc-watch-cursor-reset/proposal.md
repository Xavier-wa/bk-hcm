# Proposal: cc-watch-cursor-reset

## Why

CC 主机增量同步（watch 事件消费）在事件堆积、消费落后太久时，当前没有任何应急手段让消费位置跳过积压：失败的批次不重试、cursor 照样提交，中间被跳过的主机变更只能等下一个事件或 6 小时一轮的全量同步才补上，导致「主机没同步」类问题恢复慢、不可控。

现状唯一的自愈逻辑，是 CC 返回事件链节点不存在（1103007）时把 cursor 置空、从当前时间重新 watch；它只在「cursor 已死」时触发，覆盖不了「cursor 活着但落后太多」的场景，且无法人工干预。

需要一个「重置 cursor」的兜底能力：堆积落后太久时，由运维把消费位置跳到最新，并异步触发主机补偿同步，补齐被跳过的主机增量变更。属应急操作，会跳过中间未消费事件。

## What Changes

- 新增 admin 接口 `POST /api/v1/cloud/admin/system/cc_sync/watch/cursor/reset`（同现有 admin 路径，直连、不走网关、系统内部调用无需鉴权）。
- 请求参数：body 仅 `resource`（resType），目标租户取请求头 `X-Bk-Tenant-Id`（直连场景由调用方在 header 中指定），二者限定重置范围。**本期只支持「重置到最新」一种目标**：写入 etcd reset flag（`/hcm/event/cc-reset/{tenant}/{resType}`），watch 循环下一轮消费 flag 并将 cursor 置空，CC 在 cursor 为空时从当前时间起 watch，跳过全部积压。
- 校验：resource 白名单（host / host_relation）+ tenant 存在性（`logics/tenant.CheckTenantExist`）。本期不接受调用方传入 cursor，因此无需 cursor 格式校验与 CC 活性探测。
- 写入与生效：复用 watcher 的 lease 写 reset flag 到 etcd；watch 循环每轮迭代开头检查并消费 flag，将 cursor 置空后正常 watch。不直接覆盖 cursor key，避免与在途轮次的 cursor 提交竞态；无需重启 master。
- **主机补偿同步（异步）**：重置成功后 `go SyncHostsByTenant(kt, [ziyan, other], cliSet)`，只同步主机、只覆盖 CC 增量涉及的两个厂商。互斥对齐「主动按账号触发」：逐账号查 DB `account_sync_detail` 中 cvm 是否 `syncing` + 抢 `lock.Key(accountID)`，进行中的账号跳过、不报错、不影响定时全量。
- 观测与审计：重置前后打 Info 级审计日志（旧/新 cursor、tenant、resType、rid）；响应回显 `pre_cursor`。

**本期不做**：

- **按指定时间/指定事件重置消费位置**：技术方案已在 design 中记录（接口内用 `StartFrom` 向 CC 探测、把时间换算成 cursor 后再写 etcd），留待后续立项实现。
- **补偿其他云厂商 / 非主机资源**：CC 增量同步只涉及 ziyan/other 主机，其他厂商与资源类型不在本期范围。
- **给定时全量加锁 / 引入租户级补偿锁**：不改定时全量编排；补偿与定时全量的互斥靠账号级 DB 状态 + 账号锁，冲突时补偿跳过该账号。

## Capabilities

### New Capabilities

- `cc-watch-cursor-reset`: CC watch 事件消费 cursor 的管理员应急重置能力，覆盖重置到最新的接口、参数校验、etcd 写入生效机制、主机补偿同步与审计日志。

### Modified Capabilities

（无）

## Impact

- **代码**：
  - `cmd/cloud-server/service/admin/reset_cursor.go`：重置接口 handler + 异步触发主机补偿；
  - `cmd/cloud-server/service/watch/bkcc/watcher.go`：向 admin 链路暴露 `ResetEventCursor`（写 reset flag），watch 循环消费 flag 并重置 cursor，不改动 watch 消费主流程；
  - `cmd/cloud-server/service/watch/bkcc/host.go`：watch 循环每轮开头检查并消费 reset flag；
  - `cmd/cloud-server/service/capability/` + `cmd/cloud-server/service/service.go`：装配 watcher 到 admin 服务；
  - `cmd/cloud-server/logics/tenant/`：新增 `CheckTenantExist`（目标租户由调用方显式指定，与身份租户无关）；
  - `cmd/cloud-server/service/sync/sync_host.go`：通用入口 `SyncHostsByTenant`（按租户 + 厂商集合同步主机），账号并发 + DB/锁互斥；
  - `cmd/cloud-server/service/sync/{tcloud-ziyan,other}/sync_all_resource.go`：新增 `SyncHostResource`（仅同步主机）；
  - `pkg/api/cloud-server/cc-sync/`：重置接口请求/响应协议。
- **外部系统**：etcd（cursor 写入 + 账号锁）、data-service（`account_sync_detail` 状态查询与同步状态写入）。本期不新增 CC 调用。
- **行为**：该接口为应急操作，**会跳过中间未消费事件**；重置成功后异步触发 ziyan/other 主机补偿。进行中的账号自动跳过，不打断定时全量。不影响正常 watch 消费流程（cursor 单写者语义不变，reset 通过 flag 通知，不直接写 cursor key）。
- **依赖**：不引入新的第三方依赖。
