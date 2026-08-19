## Why

南京地域 `tcloud-ziyan` 单次全量 CLB 同步约 2.35 万台 LB，实测耗时约 1h13m，超过 cloud-server 调用 hc-service 的 HTTP 超时（原 50 分钟）。灰度计时表明瓶颈在监听器同步，其中 RS 同步占 85%–96%：云侧 `DescribeTargets` 按监听器每批 20 个切批调用，叠加生产并发约 80 路，远超腾讯云 20 次/秒配额，限流重试被监听器数量放大；DB 侧原按在用目标组逐个 `ListTarget`（N+1）。

同时，方案选择「不传 ListenerIds、一次拉整台 LB 的 RS」后，单台 LB 的预取结果会常驻内存直至该 LB 同步结束。监听器与 RS 一般不超过 1:2，用监听器数量作为预取上限即可在不额外查 RS 的前提下兜住内存。阈值必须可配，默认 10000。

调查与方案依据：[CLB 同步优化](https://iwiki.woa.com/p/4031600040)。关联 TAPD story 136401493。

## What Changes

- **云 API 预取（方案 A）**：仅在 LB 全量同步路径 `listenerOfLoadBalancer` 中，`DescribeListeners` 之后对整台 LB 调用一次 `DescribeTargets`（不传 ListenerIds），按 listener id 建内存索引，后续每批 20 个监听器从缓存切片，零额外云调用。预取失败回退按批拉取。
- **内存安全阈值**：当该 LB 云上监听器数量超过配置阈值时，**跳过预取**，回退原按批 `DescribeTargets`。阈值走 hc-service `sync` 配置，默认 **10000**。代码注释写明：监听器与 RS 一般不超过 1:2，故用监听器数量近似约束预取占用。
- **DB N+1 消除**：`listTargetsFromDB` 按 `target_group_listener_rule_rel` 取出在用 TG 后，按 TG ID 批量 IN 查询 RS（每批 `CloudResourceSyncMaxLimit=100`，分页 500），不再按目标组逐个 RPC。
- **入口隔离**：独立监听器同步入口 `Listener()`（watcher / 变更后同步）继续传 `prefetchedTargets=nil`，行为与预取前一致。
- **不做**：不按 DB/云 API 预查 RS 数量再决定是否预取；不上方案 B（把 RS 同步整段上移）；不改 tcloud 非 ziyan 拷贝；不为此次优化调整 `syncConcurrent`。

无 **BREAKING** 变更：对外 API、同步结果语义、删除链路不变。

## Capabilities

### New Capabilities

- `clb-sync-rs-prefetch`：tcloud-ziyan CLB 全量同步时，单台 LB 一次预取全部监听器 RS 以降低 `DescribeTargets` 调用次数；并以可配置的监听器数量阈值跳过超大 LB 的预取，保证内存占用安全。
- `clb-sync-rs-db-batch`：tcloud-ziyan CLB 同步查本地 RS 时，按在用目标组 ID 批量 IN 查询，消除按目标组逐个 `ListTarget` 的 N+1。

### Modified Capabilities

无。`openspec/specs/` 中没有 CLB 资源同步行为的既有 spec。

## Impact

**受影响服务**：hc-service（ziyan CLB 资源同步）。cloud-server 仍按账号+地域发起全量同步，本变更不改调用契约。data-service 的 `ListTarget` 接口不变，仅调用次数下降。

**受影响代码**：

| 位置 | 改动 |
|---|---|
| `cmd/hc-service/logics/res-sync/ziyan/load_balancer_listener.go` | `listenerOfLoadBalancer` 预取、阈值跳过、缓存切片；`Listener()` 不预取 |
| `cmd/hc-service/logics/res-sync/ziyan/load_balancer_target_group.go` | `listTargetsFromCloud` 支持空 CloudIDs 一次拉全量；`listTargetsFromDB` 批量 IN |
| `pkg/cc/types.go` 的 `SyncConfig` | 新增预取监听器数量阈值配置，默认 10000 |
| `cmd/hc-service/etc/hc_service.yaml`、`docs/support-file/helm/values.yaml` | 同步配置样例补阈值字段 |
| `cmd/hc-service/logics/res-sync/ziyan/load_balancer_target_group_test.go` | 预取、缓存切片、DB 批量查询、阈值跳过的单测 |

**不涉及**：DB schema、对外 HTTP API、依赖版本、tcloud（非 ziyan）同步实现、安全组同步、孤儿目标组级联删除。

**风险**：

- 预取使同一台 LB 内多批监听器共用一次云结果，RS 时效差一个同步周期内的数秒；云上刚解绑的 RS 最多延迟到下一轮才清理，不会误删。
- 超阈值 LB 仍走按批拉取，该台耗时与优化前相当，但南京实测无 RS≥10000 的 LB，监听器过万更罕见。
- 预取失败回退按批拉取，不中断该 LB 的监听器/规则同步。
