# cc-host-sync-watch-observability 规格

涉及服务层：cloud-server（service 层，watch 消费侧）、hc-service（resource 层，ziyan 同步侧）。
蓝鲸生态集成点：蓝鲸 CMDB watch API（`/event/watch/resource/{host|host_relation}`）、CMDB 主机查询接口（ListBizHost / ListHost / FindHostBizRelations）。
云厂商：TCloudZiyan（自研云）为主；cloud-server watch 分发侧覆盖全部厂商（Aws/Azure/Gcp/HuaWei/TCloud/Other/TCloudZiyan）。

## ADDED Requirements

### Requirement: watch 拉取观测日志

cloud-server 每轮 watch 循环 SHALL 输出结构化日志，字段包含：租户、资源类型、批首/批尾 cursor 及其解码事件时间（cursor 为 base64 编码、`\r` 分隔文本，第 5 字段为事件时间 unix 秒）、事件条数、事件类型分布、watched 标记、拉取耗时。此外：

- watched=false 时 MUST NOT 输出日志，仅提交参考 cursor 并刷新 `committed_event_timestamp` 指标（该分支约 20 秒一次，日志无信息量）；
- 拉取失败时 MUST 记录连续失败次数（恢复后清零）与最近一次原始错误，MUST NOT 再做错误分类（原始错误已可读，分类属有损重编码）。

#### Scenario: 正常批次

- **WHEN** 拉取到 N 条事件
- **THEN** 日志包含首尾事件时间、条数、增/删/改分布、拉取耗时

#### Scenario: 参考 cursor 更新不误报

- **WHEN** watched=false 且参考 cursor 事件时间新于本地 cursor
- **THEN** 不告警。`WatchHostRelationEvent` 只订阅 create，链上的 update/delete 会被 CC 跳过并给出更新的参考 cursor，这属于正常现象；无法据此区分"被过滤"与"被丢失"

#### Scenario: 连续失败可见

- **WHEN** 拉取连续失败（当前实现无退避，失败即重试）
- **THEN** 日志中连续失败次数递增，可区分偶发抖动与完全停滞

### Requirement: 消费分发观测日志

cloud-server 消费分发侧 SHALL 输出：

- 桶级日志：每个 (厂商, 隔离空间) 桶开始时一条（厂商、隔离空间、桶内主机数），结束时一条（追加已完成/失败小批数、本桶耗时、状态），以及本轮事件数与去重后主机数；
  桶开始日志是卡住时定位"卡在哪个桶"的唯一依据（最后一条 start 即当前处理中的桶）；
- 主机小批（每 100 台一次 hc 调用）MUST NOT 逐批打日志，仅在单批耗时超过阈值（30s）时输出一条 warn 并带 hostIDs，避免事件风暴期日志量随主机数线性膨胀；
- 分发耗时：consume 整批耗时，classifyHost 子步耗时（CC 查业务归属、DS 查厂商两段拆开）；
- 失败跳过：ziyan 链路上所有"未处理即跳过"路径 MUST 计数并记录 hostIDs，包括 hc 调用失败跳过、获取厂商账号失败整桶跳过、space 转换失败跳过（cursor 仍推进的现状行为不变，仅观测）；
  本期只做 ziyan 链路，其余厂商的 `update*Host` 保持零改动，其观测由桶级日志（主机数/耗时/状态）覆盖；
- delete 路径单列：删除台数、耗时、hostIDs。

#### Scenario: 下发批级日志定位卡点

- **WHEN** 某 (厂商, 隔离空间) 下发批按 100 台分批调用 hc-service
- **THEN** 下发批开始与结束各落一条日志；批次卡住时，最后一条 `patch start` 即当前正在处理的批，回答"当前批进行到哪"

#### Scenario: 慢批可见

- **WHEN** 某 100 台小批的 hc 调用耗时超过阈值
- **THEN** 输出 warn 日志带耗时与 hostIDs，凭 rid 到 hc-service 侧继续下钻

#### Scenario: hc 批次失败跳过留证

- **WHEN** 某小批 hc 调用失败被跳过
- **THEN** 日志记录失败主机数与 hostIDs 全文，作为事件丢失的直接证据

#### Scenario: 整桶跳过留证

- **WHEN** 获取厂商账号失败导致整桶未处理
- **THEN** 记录该桶主机数与原因，不留观测盲区

#### Scenario: 删除风暴可见

- **WHEN** 一批事件包含删除
- **THEN** delete 日志单列台数与耗时，不与 upsert 混杂

### Requirement: hc-service 同步观测日志

hc-service ziyan 同步 SHALL 输出三类日志：

1. 批次总览（每批一条）：五段耗时（list_biz_host / list_cloud_cvm / sync_assoc_res / sync_host_data / sync_sg_relation）、回退标记（path=full|host_only，CC 查不到主机或云上无 CVM 时仅走 Host 路径）、账号、业务ID、主机数、cc hosts/cvms/regions、db 增删改、rid。
   **已落地**。入口（增量 upsert / 增量 delete / 全量）与来源（RequestSource）仍为待办；
2. 外部调用明细（统一机制，每次调用一条）：目标类型（CC / 云 API / DS）、接口名、阶段标签、region（云 API）、耗时、返回或影响条数、错误类型（云 API 限流错误 MUST 单独分类）。
   **待办**；
3. 分段步骤耗时（进总览 steps）：sync_assoc_res 下仅三种关联资源本体——sync_assoc_vpc / sync_assoc_subnet / sync_assoc_sg（ziyan 不同步 Disk/EIP）；sync_host_data 子步 list_cc_host / fill_cloud_field / read_host_db / create|update|delete_host_db；sync_sg_relation 整段耗时。
   region × 资源类型的过程 Info 与 RelSyncStats 已有；云拉取次数、写 RPC 次数等细字段进总览仍为待办。

#### Scenario: 五段耗时定位慢段

- **WHEN** 处理一批 100 台主机
- **THEN** 总览日志包含五段耗时，慢在哪一段直接可读

#### Scenario: 重复云调用可归因

- **WHEN** 同一批主机在 getCVM 与 fillCloudFields 各调用一次云 ListCvm
- **THEN** 两条云调用明细分别携带各自阶段标签，重复调用浪费直接可见

注：本场景描述治理前现状；关联变更 `ziyan-host-cvm-cache` 落地后，fillCloudFields 的云调用仅剩缓存 miss 补拉，阶段标签同样适用，本观测能力无需调整。

#### Scenario: 云 API 限流单独分类

- **GIVEN** 增量链路未开启限流重试（RequestSource 为空），限流即整批失败
- **WHEN** 云 API 返回限流错误
- **THEN** 调用明细的错误类型标记为限流，与超时等其他错误区分

#### Scenario: 回退路径可解释

- **WHEN** CC 查不到主机或云上无 CVM，仅执行 Host 同步
- **THEN** 总览标记回退路径，sync_assoc_res 与 sync_sg_relation 耗时为 0 的原因可解释

#### Scenario: 全量增量撞车可事后判断

- **GIVEN** 全量、增量、手工三条路径共用 HostWithRelRes 且无互斥
- **WHEN** 排查消费慢
- **THEN** 可按 (账号, 业务) + 来源字段在日志平台聚合时间窗，判断是否撞车（不新增埋点）

### Requirement: watch 消费侧 Prometheus 指标

cloud-server SHALL 上报 watch 消费侧指标（namespace `hcm`、subsystem `cc_watch`），全部 MUST 从 `WatchBatchTrace` 同源导出，深层调用点对指标无感知：

| 指标 | 类型 | Label | 采集时机 |
|---|---|---|---|
| `cc_watch_committed_event_timestamp_seconds` | Gauge | tenant, res_type | 每次 cursor 提交成功后（含 watched=false 参考 cursor），取所提交 cursor 解码的事件时间；置空或解码失败不更新 |
| `cc_watch_processing_start_timestamp_seconds` | Gauge | tenant, res_type | consume 开始时置为本批开始时间，consume 返回后置 0（含消费失败路径） |
| `cc_watch_consume_cost_seconds` | Histogram | tenant, res_type | 批末 flush，仅 consume 段耗时（不含 fetch 长轮询等待） |
| `cc_watch_events_total` | Counter | tenant, res_type, event_type | 批末 flush |
| `cc_watch_hosts_total` | Counter | tenant, operation | 批末 flush，去重后主机数 |
| `cc_watch_hosts_sync_total` | Counter | tenant, operation, result(success/failed) | 每个 100 台子批 hc 调用返回后即时采集 |

凡是要相除的采集项 MUST 同点采集：consume_cost / events_total / hosts_total 同在批末 flush；sync_total 的成功率 MUST 由 success/(success+failed) 自闭合计算，MUST NOT 与批末的 hosts_total 相除。未下发主机（事件解析失败、classify 跳过、整桶跳过）MUST NOT 计入 sync_total。label MUST 保持低基数，MUST NOT 使用 host_id / biz_id。

#### Scenario: 落后告警

- **WHEN** `time() - hcm_cc_watch_committed_event_timestamp_seconds` 持续超过阈值
- **THEN** 判定消费未推进（拉取失败或某批卡住），触发告警，可衔接 cursor 重置兜底

#### Scenario: 空闲不误报

- **GIVEN** watched=false 的参考 cursor 提交同样刷新该时间戳（与 etcd 中已提交 cursor 的位置语义一致）
- **WHEN** 某租户长期无主机事件
- **THEN** 时间戳随参考 cursor 刷新，不产生误报

#### Scenario: 区分卡住与无事件

- **GIVEN** 某批卡在单次 hc-service 调用上，此时 committed_event_timestamp 停更、hosts_sync_total 不增长、consume_cost 尚未 Observe，与"无事件"表现一致
- **WHEN** 查询 `hcm_cc_watch_processing_start_timestamp_seconds > 0`
- **THEN** 判定当前确有批次在处理中，`time() - 值` 即已卡时长；空闲时该值为 0，不与卡住混淆

#### Scenario: 单台主机均耗

- **WHEN** 计算 `rate(cc_watch_consume_cost_seconds_sum[5m]) / rate(cc_watch_hosts_total[5m])`
- **THEN** 得到已完成批次的每台去重主机平均消费耗时；分子分母同点采集，无短窗错位

#### Scenario: 同步失败率告警

- **WHEN** `rate(cc_watch_hosts_sync_total{result="failed"}[5m]) / rate(cc_watch_hosts_sync_total[5m])` 超过阈值
- **THEN** 判定下游同步失败占比异常，结合带 rid 的失败日志定位

#### Scenario: 事件陡增可见

- **WHEN** `rate(cc_watch_events_total[5m])` 突增或 event_type 占比异常
- **THEN** 可区分"事件变多"与"消费变慢"两类落后原因

### Requirement: hc-service 资源同步 Prometheus 指标

hc-service SHALL 上报泛化的资源同步指标（namespace `hcm`、subsystem `res_sync`），按 `vendor / resource / step / source / result` 五个 label 聚合，同厂商不同资源（或后续其他 vendor）复用同一指标族，由 trace 同源导出：

| 指标 | 类型 | Label | 采集时机 |
|---|---|---|---|
| `res_sync_cost_seconds` | Histogram | vendor, resource, step, source(full/incremental), result(success/failed) | 同步请求结束（入口 defer 阶段）按已记录 step 导出 |

step 只覆盖大块环节，且**由各资源自行定义**（主机同步各 vendor（ziyan/tcloud/aws/azure/gcp/huawei）统一为 host/rel_res/host_rel，其他资源如 disk/eip 的环节不同，不统一枚举），不承载细粒度排障步骤，避免与日志 steps 重复；**`source` 由全量框架标记推导**：`ResourceSync`/`ResourceSyncV2` 框架入口（`handler.go`）统一打 full 标记（`common.MarkFullSyncSource`），watch 增量、操作后回调、手工按条件同步均不经过框架，天然为 incremental；**不走框架的全量入口（`SyncCvmCCInfo` 一族，非云资源同步不适配框架机制）MUST 在 service 入口手动调用 `MarkFullSyncSource` 补标记**。MUST NOT 用 `RequestSource` 推导——`AsynchronousTasks` 语义是云批量异步操作的超限重试，async 任务交付后触发的按条件同步也携带它，会误标 full。`result` 区分成败，失败样本不混入成功分布。label MUST 低基数，MUST NOT 使用 account_id / biz_id / host_id。

`resource` 与同步详情（sync_detail）的资源类型口径一致：云资源镜像同步用云资源类型（如 `cvm`），**CC 业务归属对齐用独立资源类型 `cvm_cc_info`**——两者量级差异大（分钟级 vs 秒级），混在同一 resource 下分位数与告警阈值均失真。`MarkFullSyncSource` 与指标上报为纯观测横向逻辑（ctx value + 内存 Observe），MUST NOT 改变任何业务行为。

通用机制 MUST 收敛到 `res-sync/common` 的 `ResSyncTrace` 基座（步骤采集、source 推导、白名单导出）；vendor 有自有观测外壳时（如 ziyan 的日志字段）SHALL 组合内嵌基座而非复制机制代码。基座不挂 ctx（当前埋点均在入口函数内可见，YAGNI）；同名 step 多次记录由 `timing.Collector` 聚合（适配多段 mgr.Sync 合并计时的场景）。

#### Scenario: 全量/增量耗时分开看

- **WHEN** 查询 `histogram_quantile(0.9, rate(hcm_res_sync_cost_seconds_bucket{vendor="tcloud-ziyan",resource="cvm",step="total"}[5m])) by (source)`
- **THEN** 全量（定时）与增量（watch 消费）两条链路的整体耗时分布可直接对比

#### Scenario: 定位消费瓶颈段

- **WHEN** 某 `source="incremental"` 同步整体变慢
- **THEN** 按 `step` 拆分 `host / rel_res / host_rel`，慢在大块环节直接可读

#### Scenario: 跨 vendor 对比整体耗时

- **WHEN** 查询 `histogram_quantile(0.9, rate(hcm_res_sync_cost_seconds_bucket{resource="cvm",step="total"}[5m])) by (vendor, source)`
- **THEN** ziyan 与 tcloud 的主机云镜像同步耗时同面板可比；CC 归属对齐（`resource="cvm_cc_info"`）独立成序列，不与云镜像同步互相稀释

### Requirement: cursor 重置审计

cursor 被重置时 SHALL 输出审计日志：旧值、新值、原因、触发方（自动/人工）、时间。

#### Scenario: 链节点失效自动重置

- **WHEN** CC 返回事件链节点不存在错误，cursor 被自动重置为空
- **THEN** 审计日志记录旧 cursor、新值、原因（当前仅有一条 error 日志，无审计）

#### Scenario: 人工调整预留

- **WHEN** 后续通过管理能力人工调整 cursor（从最新或指定事件开始消费）
- **THEN** 审计日志记录操作人、旧值、新值、原因
