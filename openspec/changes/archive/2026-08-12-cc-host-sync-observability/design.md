# CC 主机同步消费可观测性增强 — 技术设计

## Context

增量同步链路：CC watch（cloud-server，每租户 2 协程：host / host_relation）→ 事件去重分类 → 按 (厂商, 业务/账号) 分桶、100 台一批调 hc-service → hc-service ziyan `HostWithRelRes` 五段处理（list_biz_host / list_cloud_cvm / sync_assoc_res / sync_host_data / sync_sg_relation）。cursor 存 etcd（`/hcm/event/cc/{租户}/{类型}`），仅记录已提交批次的末尾位置。

本设计基于以下**已验证事实**（代码走读 + 测试环境日志实证）：

1. cursor 是 base64 编码的 `\r` 分隔文本，第 5 字段为事件时间 unix 秒（测试环境真实 cursor 解码验证：事件时间 19:36:43，与日志时间 20:04 相差 27 分钟，合理）；
2. CC watch 是长轮询，无事件时服务端 hold 约 20 秒返回（实测 fetch_ms=21061），**不存在空转热循环**；
3. watched=false 时 CC 返回一条参考事件，其 cursor 代表 CC 当前最新位置。**注意**：参考 cursor 比本地新并不等于事件丢失——只订阅部分事件类型时（如 `WatchHostRelationEvent` 只订阅 create），被过滤掉的事件同样推进参考 cursor，故该形态无法区分空闲、过滤与失效；
4. 消费失败后 cursor 照样推进（`consumeFunc` 报错后仍 setCursor），且存在 3 条静默跳过路径：hc 批次调用失败 continue、获取厂商账号失败整桶 return、space 转换失败跳过；
5. 增量链路未开启云 API 限流重试：`SetRateLimitRetryWithRandomInterval` 的开启条件是 `RequestSource == AsynchronousTasks`，定时全量同步设置了该来源（time_sync.go），增量 watch 的 kit 未设置（NewTenantBackendKit）——限流对增量表现为整批直接失败；
6. 全量（定时任务，多 worker 并发）、增量（串行）、手工 API 三条路径共用 `HostWithRelRes`，**无互斥**，抢资源时增量（无重试）是被挤兑方；
7. hc 侧 ListCvm 被调用两次（getCVM 并发 10 + fillCloudFields 内 region 串行），属已知浪费（治理由关联变更 `ziyan-host-cvm-cache` 承担）；
8. SG 绑定关系同步对未变化的 CVM 也执行写（写 RPC 次数≈主机数），属已知写放大；
9. consume 的 for events 循环只做 JSON 解析与按 bk_host_id 去重，事件的 cursor/类型信息在循环后被丢弃，处理单位从"事件"变为"主机批次"。

## Goals / Non-Goals

**Goals:**

- 查一下（接口或日志）即可回答：消费到哪个事件了、卡在哪一段、丢了多少、是不是被限流、是不是被全量/手工抢资源；
- 观测数据同源：一处埋点，状态快照（查现在）、日志流水（查历史）、Prometheus 指标（告警与趋势，阶段二）三处输出；
- 堆积与消费慢可告警：主告警基于提交事件时间戳，成本类指标辅助归因。

**Non-Goals:**

- 不改变同步行为：不加重试、不改 cursor 推进策略、不做全量/增量互斥（治理动作属后续变更）；
- 不引入新的可观测基础设施：指标复用仓库既有 `pkg/metrics`（Prometheus），不新增组件；
- 本期指标仅覆盖 watch 消费侧（cloud-server）；hc-service 侧分段耗时指标（`hcm_res_sync_cost_seconds{vendor,resource,step,source}`）形态已预留，留待下一阶段；
- 不做性能优化本身：ListCvm 重复拉取的治理由关联变更 `ziyan-host-cvm-cache` 承担，SG 写放大收敛另立议题；
- 不做事件级进度追踪（处理单位是主机批次，事件级无意义）；
- 未下发主机（事件解析失败、classify 跳过）本期不打指标；classify 跳过点保留错误日志，不再单独计数。

## Decisions

### D1: 形态 = 结构化日志 + Prometheus 指标

日志满足历史回溯与批内细节（"卡在哪一批、哪一步"），指标回答"落后多久、单位耗时是否变高、失败多少"。两者都是轻量实现，符合"先指标后实现、不上重型方案"的共识。

> 2026-08-08 修订：日志能力已按本决策落地。阶段二追加 Prometheus 指标（见 D10），指标字段从同一处埋点（`WatchBatchTrace`）导出，兑现"字段定义可直接复用"的预设。

> 2026-08-08 再修订：曾实现 etcd 批次记录（批首/批尾写 `/hcm/observe/cc_watch/`）与消费位置查询接口（`QueryCCWatchStatus`，见 D9）。指标落地后这两路成为重复出口（批级统计由 `hcm_cc_watch_*` 与日志承载、消费位置由 `committed_event_timestamp` 承载），已整套移除：写入侧、读取侧、API 类型、admin 路由与 handler。"是否落后、是否卡住"由指标 + 日志回答。

### D2: 卡住判定落在"批"这一层，批粒度信息交给日志与指标

排查只需要先回答两个问题：还在动吗、卡在哪一批。"还在动吗"由 `committed_event_timestamp` 指标（消费位置）回答；"卡在哪一批、哪一步"由批粒度日志回答——批首一条（事件数、事件时间范围、fetch 耗时），批尾一条汇总（耗时、主机数、解析失败数、下发批数、步骤耗时）。

批内更细的进度（每个 (厂商, 隔离空间) 下发批、批内每 100 台的小批）同样只进日志：每个下发批开始与结束各一条，卡住时最后一条 `patch start` 就是当前正在处理的批；小批只在超过阈值（30s）时打一条 warn，把拖慢整批的那一批捞出来。日志均带批级 rid，凭它可以把 cloud-server 与 hc-service 的日志串成一条线。

- biz 只放 ID 不做名称解析，保持日志轻量；
- 跳过的 hostIDs 只在跳过点的错误日志里全文记录。

### D3: 日志与指标字段同源

B/C 组日志字段与 `hcm_cc_watch_*` 指标来自同一处埋点（`WatchBatchTrace`），避免两套口径。B-1（首尾 cursor 事件时间）、B-3（事件类型分布）在日志流水与指标中同时存在：日志看单批明细，指标看趋势与告警。

### D4: 外部调用明细统一机制（CC / 云 API / DS 三类，逐次一条）

一个 helper 在调用点记录：目标类型、接口、阶段标签、region、耗时、条数、错误类型（云限流单独分类）。不按阶段拆分观测项，避免遗漏调用点（fillCloudFields 的重复 ListCvm、删除入口的 DS 删除、getVpcMap 的 DS 查询等都能覆盖）。阶段标签使每次调用可归因到五段之一。

注：`ziyan-host-cvm-cache` 落地后，fillCloudFields 的云调用仅剩缓存 miss 补拉一条路径，统一机制照常覆盖该调用点，观测设计无需随之调整。

### D5: 放弃心跳对比，watched=false 分支不加日志

原设计想用"参考 cursor 是否比本地新"区分静默与断档。落地前复核发现该判据不成立：`WatchHostRelationEvent` 只订阅 create 事件，链上的 update/delete 会被 CC 跳过并返回更新后的参考 cursor，参考更新是这条流的常态，据此告警必然误报；而"被过滤"与"被丢失"在 cursor 上无法区分。

因此 watched=false 分支只提交参考 cursor 并刷新 `committed_event_timestamp` 指标，不输出日志（该分支约 20 秒触发一次，日志无信息量）。"是否掉队"由该指标结合最近一次成功消费批次的日志判断。

### D6: 全量/增量撞车判断零代码化

hc 日志带来源字段（RequestSource 本来就在 header 中传递），排查时在日志平台按 (账号, 业务) 聚合时间窗看重叠即可。不维护内存/分布式计数（多副本下不准确且变重）。

### D7: 事件级进度不做，粒度到"批次"

for events 循环只解析去重（毫秒级），真正耗时在批处理。记录"批首尾 cursor + 批内小批进度"即可完整回答"消费到哪"。

### D8: 观测记录命名统一为 `*Trace`，统一挂 `kt.Ctx` 传递，形态按两侧需要各异

两侧埋点载体都是"每批一条记录，边处理边填充，批结束渲染一行日志"，命名与方法集统一、传递方式统一，仅数据形态按各自需要不同：

**命名与传递统一**：公共机制置于 `pkg/tools/timing`（包名 `timing`，遵循包名小写无分隔符约定），提供加锁的步骤耗时收集与汇总渲染、nil 安全降级。两侧具体类型以 `Trace` 为统一后缀、按追踪对象命名：cloud-server 为 `WatchBatchTrace`（一轮 watch 迭代：拉取 + 消费 + 分发），hc-service 为 `SyncHostTrace`（一批主机同步）。两者都挂 `kt.Ctx`（`kit.NewSubKitWithCtx`），埋点取用走 `FromCtx`，取不到返回 nil——未埋种子的入口行为不变。种子点各一处：cloud-server 在 watch 循环每轮迭代新建 kt 后挂载，hc-service 在 `HostWithRelRes` 入口挂载。

**形态与方法集按两侧需要各异**（同源思路，非同一接口）：

- `WatchBatchTrace` 为强类型结构（`Track` / `AddStep` / 桶统计 / 批末 `flushMetrics`）：要同时渲染日志汇总行与导出批级指标，扁平步骤列表无法表达桶维度的统计。
- `SyncHostTrace` 为强类型结构（`Track` / `Set*` / `logSummary`），步骤耗时委托 `timing.Collector`：hc 侧本期只输出日志，无需 etcd/API 契约；穿入 `cvm-rel-manager` 的统计用该包内的通用 `RelSyncStats`（基于 `timing`），不引入 ziyan 专用类型。外部调用明细（CC/云 API/DS 逐次一条）仍属待办，本期总览只聚合 steps。

两者**不跨进程流动**：`WatchBatchTrace` 中的 hc 调用耗时只是一个标量，hc 内部五段明细由 `SyncHostTrace` 在 hc 进程内记录与输出，两者靠已验证不变的 rid（header 透传）在日志平台关联。摘要随同步响应回传的方案本期不做。

并发安全：cloud-server 当前分发为串行，但后续若并行化小批调用，ctx 携带的 `WatchBatchTrace` 天然被各 goroutine 共享；hc-service 的 `getCVM`（并发 10）与 `syncCvmRelRes`（并发 20）本就并发写入，故写方法一律加锁；并发路径的过程日志带 region / resType。

### D9: 不做状态查询接口，消费位置由指标承载

请求经 api-server 反向代理以**轮询**方式落到任意 cloud-server 实例，而 watch 仅在 master 运行——任何内存态查询接口都要解决"请求落到非 master"的转发问题（master 地址推导、转发客户端、环路防护等），代价明显超出接口本身的价值。

> 2026-08-08 修订：本决策曾落地为只读查询接口（`QueryCCWatchStatus`，读 etcd cursor key + 批次记录，任意实例可答）。指标（D10）落地后，消费位置由 `committed_event_timestamp` 指标承载（告警与趋势比一次性查询更有用），接口与批次记录已整套移除（admin 路由/handler、`ListWatchStatus`、API 类型、`capability.EtcdCli` 透传）。`pkg/serviced` 零改动。

替代方案：①内存快照 + 转发至 master——代价过高，否决；②只读接口读 etcd——已实践后随指标落地移除；③批级记录落 etcd——已实践后移除，指标与日志足够覆盖。

### D10: watch 消费侧指标（阶段二）：trace 同源导出、相除项同点采集

阶段一的 `WatchBatchTrace` 已是批级唯一采集入口；指标作为它的**第二个出口**接入：metric 自增/记录收在 trace 的方法内部，深层调用点对 metric 无感知，日志与指标永远同源（同一份计数，不会两套口径）。

**指标清单**（namespace `hcm`，subsystem `cc_watch`，全名如 `hcm_cc_watch_events_total`）：

| 指标 | 类型 | Label | 采集时机 |
|---|---|---|---|
| `cc_watch_committed_event_timestamp_seconds` | Gauge | tenant, res_type | 每次 cursor 提交成功后（有效批末尾、watched=false 参考 cursor 提交），取所提交 cursor 解码出的事件时间；置空或解码失败不更新 |
| `cc_watch_processing_start_timestamp_seconds` | Gauge | tenant, res_type | consume 开始时置为本批开始时间，consume 返回后置 0（含消费失败路径） |
| `cc_watch_consume_cost_seconds` | Histogram | tenant, res_type | 批末 flush，仅 consume 段耗时（不含 fetch） |
| `cc_watch_events_total` | Counter | tenant, res_type, event_type | 批末 flush，按事件类型分别计数 |
| `cc_watch_hosts_total` | Counter | tenant, operation | 批末 flush，去重后主机数（upsert / delete） |
| `cc_watch_hosts_sync_total` | Counter | tenant, operation, result(success/failed) | 每个 100 台子批 hc 调用返回后即时采集 |

**采集时机纪律**：凡是要相除的两个采集项 MUST 同点采集——consume_cost、events_total、hosts_total 同在批末 flush（per-host / per-event 均耗的分子分母同属"已完成批次"集合，避免短窗口错位：主机数去重后即知，但耗时要批末才知，提前打会让均值先低后高）；sync_total 自成比率（成功率 = success/(success+failed)），分子分母同在子批返回处即时采集，换取慢批中途的失败可见性。两组之间 MUST NOT 混算（成功率不除以 hosts_total，均耗不除以 sync_total）。

**语义要点**：

- 单台均耗 `rate(cc_watch_consume_cost_seconds_sum[5m]) / rate(cc_watch_hosts_total[5m])` 为主口径：一条事件 detail 只对应一个 bk_host_id（host 为单 `Host`、host_relation 为单 `HostTopoRelation`），去重后主机数 ≤ 事件数恒成立，per-event 均耗仅作重复变更率的辅助解释，两线同高可辅证消费变慢；
- consume_cost 不含 fetch：长轮询等待（无事件 hold 约 20s）与流量成反比，混入会在空闲期制造"消费慢"假阳性；fetch 耗时保留在批末 summary 日志，不进指标；
- committed_event_timestamp 覆盖 watched=false 的参考 cursor 提交（其语义已由 D5 论证可信）：空闲期时间戳随参考 cursor 刷新不误报，堆积/卡住时停在真实事件时间上，`time() - 值 > 阈值` 即告警；
- processing_start_timestamp 与 committed_event_timestamp 互补，补齐"卡在单次下游调用"这一盲区：该场景下 committed 停更（游标未提交）、hosts_sync_total 不涨（子批未返回）、consume_cost 未 Observe（批未结束），三者与"无事件"表现完全相同；本指标在消费期间持续非零，`> 0` 即在处理、`time() - 值` 即已卡时长。埋点并入 trace 已有的 `Track(stepConsume)`，合成 `TrackConsume()`：两个 consumeFunc 各一行 `defer tr.TrackConsume()()` 同时完成"步骤计时 + 处理中标记"，defer 覆盖消费失败与 panic 路径；res_type 在 trace 构造时固化（与 tenant 同源），不下沉到各 vendor 分支；
- 未下发主机（解析失败、classify 跳过、整桶跳过）本期不打指标——`hosts_total - hosts_sync_total` 的大窗口差值可作"未下发"趋势参考，短窗不做；classify 跳过点保留错误日志，不再单独计数；
- label 基数：tenant 有界（租户列表低频变化）、res_type 2 值、operation 2 值、event_type 3 值、result 2 值，全部低基数；MUST NOT 把 host_id / biz_id 放入 label。

**告警分工**：committed_event_timestamp 为主告警（是否落后，承接"主动发现消费过慢→触发 cursor 重置兜底"）；consume_cost 分布与单台均耗为归因辅助（单位成本是否变高）；sync_total 失败率承接"失败跳过"方向。

**hc-service 侧预留**：分段耗时指标定为泛化形态 `hcm_res_sync_cost_seconds{vendor, resource, step, source}`（ziyan 传常量 vendor=tcloud-ziyan、resource=host，step 复用 `SyncHostTrace` 的 stepXxx 常量，source 取 `kt.GetRequestSource()`），接入时只需 `timing.Collector` 补一个按名聚合的 `Steps()` 访问器，在 `logSummary` 同点 flush。本期不实现。

## Risks / Trade-offs

- cursor 格式是 CC 服务端约定，非本仓库契约，存在 CC 版本升级改格式的可能 → 解码逻辑做防御：解析失败降级为返回原值、事件时间标记"未知"，不影响主流程；首尾小数字段（1/2/4）含义找 CC 团队确认后补充文档。
- 日志量增加（每桶首尾各一条 + 每次外部调用一条明细）→ 明细日志控制在 V(n) 级别或按字段精简；桶日志的条数取决于一批事件涉及的 (厂商, 隔离空间) 数，量级可控；小批只在超阈值时打 warn。
- `WatchBatchTrace` / `SyncHostTrace` 挂在 ctx 上属隐式依赖，漏挂种子点会静默退化为无观测（编译期无法拦截）→ 种子点每侧各仅一处且在最外层循环/入口，评审时重点核对；nil 安全降级保证漏挂不影响业务行为。
- 观测只能揭示"丢了多少"，不能阻止丢失（失败仍跳过、cursor 仍推进）→ 两个治理决策留待后续变更：①失败后 cursor 是否继续推进（跳过+全量兜底 vs 不推进防丢但怕毒丸卡死）；②增量链路是否开启限流重试（全量已开启，模式现成）。
- 指标（D10）由 watch master 单点产生，master 切换后由新实例的序列接续（`process_name`/`host` 常量标签随实例变化）→ 告警表达式按 tenant/res_type 聚合、不带实例标签，切换窗口内 committed_event_timestamp 短暂停更属预期语义（停更本身即"未推进"信号）。

## Migration Plan

纯新增日志字段与 Prometheus 指标，无数据迁移、无行为变更，可直接灰度上线后观察日志量。批次记录曾使用的 etcd key 前缀 `/hcm/observe/cc_watch/` 随租约到期（≤1h）自动消失，无需清理。

## Open Questions

- cursor 首尾字段（1/2/4）的确切语义：待与 CC 团队确认；
- 桶日志的粒度：已定为每桶首尾各一条 Info。原方案"每小批一条"在事件风暴期日志量随主机数线性上涨，且需要在分发循环里逐行插日志、侵入核心路径；小批只保留超阈值（30s）的 warn。
