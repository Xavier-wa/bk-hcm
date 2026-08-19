## Context

cloud-server 按「账号 + 地域」一次 HTTP 调用 hc-service 做全量 CLB 同步。hc-service 走 `ResourceSyncV2`：拉 LB 列表 → 清理已删除 → 按批（约 100 台）进入 `LoadBalancerWithListener`。每台 LB 再走 `listenerOfLoadBalancer`：`DescribeListeners` → 清理已删监听器 → 按最多 20 个监听器一批调用 `listener()`，其中 `ListenerTargets` 调 `DescribeTargets` 并与本地 RS diff。

南京灰度（约 23529 台 LB、平均约 123 监听器 / 113 在用 TG）：单次全量约 1h13m。listener 段占 85%–99%，其中 RS 同步占 listener 的 85%–96%。云 API 按 20 切批，每台约 7 次 `DescribeTargets`，一轮约 16.5 万次；生产 `load_balancer` 并发 5 × `listener` 并发 16 ≈ 80 路，相对腾讯云 `DescribeTargets` 配额 20 次/秒明显超并发。DB 侧原按在用 TG 逐个 `ListTarget`。

依据：[CLB 同步优化](https://iwiki.woa.com/p/4031600040)。`clb-sync` 分支已落地预取与 DB 批量查询，灰度曾去掉硬编码 5000 监听器上限以便压测大 LB。本设计把上限改回**可配置**，默认 **10000**，并用监听器数量作为内存护栏。

Vendor 范围：仅 **tcloud-ziyan**（自研云走腾讯云 CLB SDK）。独立入口 `Listener()`（watcher / 变更后同步指定监听器）必须保持不预取。

## Goals / Non-Goals

**Goals:**

- **时间**：每台 LB 的 `DescribeTargets` 从「监听器数 / 20」次降到 1 次（南京典型 7→1，全量约 16.5 万→2.35 万），同并发下请求率贴近 20/s 配额，减少限流重试对墙钟时间的放大。
- **时间**：本地 RS 查询从「每 TG 一次 RPC」改为「TG ID IN 批量 + 分页」，调用次数随 TG 数从线性降到 `ceil(tg/100) * pages`。
- **内存**：预取结果只覆盖「当前正在同步的那一台 LB」，并在监听器数量超过阈值时跳过预取，避免超大 LB 把整台 RS 树打进内存。
- **安全回退**：预取失败或跳过时，该 LB 本轮仍按批拉 RS，监听器/规则同步不受影响。
- **入口隔离**：`Listener()` 行为与优化前一致。

**Non-Goals:**

- 不上方案 B（把 RS / 本地目标组同步整段上移出 `listener()` 循环）。
- 不在预取前用 DB 或云 API 统计 RS 数量（`load_balancer_target` 无 `lb_id`，经 rel 再 count 成本高；云 API 统计等于先拉全量，护栏失去意义）。
- 不为此次调整 `syncConcurrent` / 云 SDK 重试策略。
- 不改 tcloud 非 ziyan 同步实现、安全组同步、孤儿 TG 级联删除。
- 不改对外 API 与删除语义。

## Decisions

### 1. 采用方案 A（预取下发），不上方案 B

`DescribeTargets` 在 `ListenerIds` 为空时返回整台 LB 全部监听器的 RS（adaptor 已支持，全量链路此前未走到）。方案 A 只在 `listenerOfLoadBalancer` 多一次全量拉取，按 listener id 建 map，分批循环不变，`ListenerTargets` 命中缓存则不再调云。

相对方案 B：不拆 `listener()` 的 RS / 本地 TG 阶段，失败面仍是「单批监听器」而非「整台 LB 结构+RS」；改动面更小，符合核心同步链路禁止 overdone 的约束。

### 2. 内存护栏用「监听器数量」，不用 RS 数量

预取占用与 RS 条数近似线性，但预取前没有廉价的 RS 计数：

| 方案 | 否决原因 |
|---|---|
| 先查 DB RS 数 | `load_balancer_target` 无 `lb_id`，需经 `target_group_listener_rule_rel` join；南京规模下额外 SQL/RPC 不划算，且本地 RS 不能代表云上即将载入的量 |
| 云 API 先 count | 没有独立 count 接口，等于先拉全量再决定是否缓存，护栏失效 |
| 监听器数量 | `DescribeListeners` 已经拿到 `len(cloudListeners)`，零额外调用 |

**1:2 假设（必须写进代码注释）**：业务上监听器与 RS 一般不超过 1:2。南京实测高 RS LB 往往监听器很少（例如 7 个 TG、1000+ RS）；监听器过万更罕见。因此 `listeners ≤ 10000` 时预估 RS ≲ 2 万，单台预取峰值可接受。超过阈值则**跳过预取**，回退按批 `DescribeTargets`（每批最多 20 个监听器的 RS），内存按批释放。

南京抽样（ziyan、以肥 TG≥50 为下界）：RS≥5000 的 LB 有 60 台，**没有 RS≥10000 的 LB**。默认 10000 在当前数据下几乎不会误伤需要加速的实例。

### 3. 阈值做成 hc-service `sync` 配置，默认 10000

不放回 `pkg/criteria/constant` 硬编码（灰度刚为压测去掉 5000 常量）。放在已有 `SyncConfig`（`yaml:"sync"`）上，与并发规则同层，运维可按环境调整而无需发版改常量。

```yaml
sync:
  concurrentRules: ...
  defaultConcurrent: 1
  # 单台 LB 预取 RS 的监听器数量上限；超过则跳过预取。
  # 监听器与 RS 一般不超过 1:2。0 或未配置时默认 10000。
  targetsPrefetchMaxListeners: 10000
```

- `trySetDefault`：字段为 0 时设为 10000，保证本地 yaml / 旧 helm 不配也能生效。
- 灰度若要再次放开，把该值调到足够大即可，不必再改代码。
- Helm `hcservice/configmap.yaml` 已 `toYaml .Values.hcservice.sync`，只需在 `values.yaml` 与 `cmd/hc-service/etc/hc_service.yaml` 补字段。

读取：`cc.HCService().SyncConfig.TargetsPrefetchMaxListeners`，仅在 `listenerOfLoadBalancer` 判断。

### 4. 预取命中 / 失败 / 跳过的语义

| 情况 | `prefetchedTargets` | `ListenerTargets` 行为 |
|---|---|---|
| 未预取（独立 `Listener()`、监听器数为 0、超阈值跳过） | `nil` | 按本批 CloudIDs 调 `listTargetsFromCloud`（每批最多 20） |
| 预取成功 | 非 nil 切片（可为空） | 直接使用，不再调云 |
| 预取 API 失败 | `prefetchListenerTargets` 返回 `nil` | 同未预取，整台 LB 回退按批拉取 |
| 预取 map 中本批部分 miss | 命中的切片 + Warn 日志 | 只同步命中的监听器 RS；miss 的本轮不拉云、不按「云上为空」删除。`ListenerTargets` 只遍历拿到的 listener targets，不会把 miss 当成空后端 |

`nil` 与空切片必须区分：Go 里 `var listenerTargets []T` 是 nil，超阈值跳过时保持 nil 才会回退。

### 5. DB 批量查询保持「在用 TG」口径

真正驱动 RS 查询的是 `target_group_listener_rule_rel` 去重后的 TG，不是 `load_balancer_target_group` 全表（南京全表 TG 约 713 万，在用约 266 万）。`listTargetsFromDB`：先按 `lb_id`（可选再按 `cloud_lbl_id`）拉 rel，再 `target_group_id IN` 每批 100、页 500。无 RS 的 TG 在 map 中保留 `nil`，与逐 TG 查询语义一致。

预取不改变 DB 路径：每批监听器仍查本批 rel + RS。方案 A 不追求把 DB 也收成整台 LB 一次。

### 6. 并发模型不变

外层 LB 批 worker 池、内层 `concurrence.BaseExec`（listener 并发）保持原配置。优化靠减少每台调用次数，而不是降并行度。预取 map 的生命周期绑定单台 LB 的 `listenerOfLoadBalancer` 栈帧，批循环结束后可被 GC；并发峰值 ≈ listener `syncConcurrent` 台 LB 的预取缓存同时存活。

## Risks / Trade-offs

- [同一台 LB 内多批监听器共用一次云结果，RS 时效差数秒] → 可接受；云上刚解绑的 RS 最多延迟一个同步周期清理。删除仍以本轮云结果为准，预取失败回退按批，不会把「没拉到」当成「云上已空」去误删（miss 只 Warn）。
- [超阈值 LB 享受不到预取加速] → 回退路径正确且这些 LB 当前数据下几乎不存在；可用配置临时抬高阈值做灰度压测。
- [1:2 被个别 LB 打破，监听器少但 RS 极多] → 监听器护栏拦不住「少监听器 + 海量 RS」。南京高 RS LB 监听器确实很少，预取一次的内存仍按该台 RS 全量。若后续出现单监听器绑数万 RS，应再加独立的 RS 体积护栏，本次不做。
- [16 路 listener 并发同时预取接近阈值的 LB] → 默认 10000 监听器 × 约 2 RS，单台数十 MB 量级，并发十几台可到数百 MB。阈值可运维下调；禁止为埋点/护栏重排控制流。
- [SDK 限流日志仍在 debug 下] → 本次不改重试封装；调用次数下降后请求率接近配额，重试应明显减少。

## Migration Plan

1. `clb-sync` 已含预取与 DB 批量；本变更补配置阈值、注释、yaml 样例与跳过路径单测。
2. 发布 hc-service 即可，cloud-server 无需改。未配字段时 `trySetDefault` 生效为 10000。
3. 回滚：回退 hc-service 镜像；或把 `targetsPrefetchMaxListeners` 设为 0 的**不能**关闭预取（0 会被默认成 10000）。关闭预取的运维手段是把阈值设为 1（几乎所有 LB 都跳过）。真正回退预取逻辑需回滚代码。
4. 观察：已有 `list targets from cloud done` 的 `cloud calls`、`lb with listener batch done`、地域级 `ResourceSyncV2 ... sync done`；超阈值应出现 `skip targets prefetch, too many listeners`。

## Open Questions

无。阈值口径（监听器数量）、默认值（10000）、配置位置（`sync.targetsPrefetchMaxListeners`）已确认。
