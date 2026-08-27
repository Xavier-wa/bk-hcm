# CC 主机同步消费可观测性增强

## Why

ziyan 自研云主机通过 CC（蓝鲸 CMDB）watch 能力做增量同步。当变更事件多或某个环节消费慢时，数据变更不及时（目前延迟已达数小时），且整个消费过程是盲盒：

- 不知道当前消费到 CC 的哪个事件（出现"4 点的变更没更新、实际在消费 6 点事件"和"6 点了还在消费 4 点事件"两类 case 时无法快速定位）；
- 找不到消费瓶颈点，无法针对性优化；
- 消费失败时 cursor 照样推进，事件被静默跳过，丢了多少无感知；
- 真产生堆积时，缺少调整消费位置的配套观测手段。

本变更分两阶段落地观测能力：**阶段一（已完成）**用**结构化日志字段**解开盲盒；**阶段二（本次追加）**在 watch 消费侧补充 **Prometheus 指标**，让"落后 / 消费慢 / 失败"可告警、可看趋势——日志回答"卡在哪一批、哪一步"，指标回答"落后多久、单位耗时是否变高、失败多少"。hc-service 侧指标形态预留，留待后续阶段。

> 2026-08-08 修订：阶段一曾含 etcd 批次记录与消费位置查询接口（`QueryCCWatchStatus`），指标落地后成为重复出口，已整套移除（见设计 D1/D9 修订注记）。

## What Changes

在 cloud-server（watch 消费侧）和 hc-service（ziyan 同步侧）增加观测输出，**不改变任何同步逻辑行为**：

- **A. 消费位置（指标）**：每 (租户, 类型) 已提交 cursor 解码出的事件时间，以 `hcm_cc_watch_committed_event_timestamp_seconds` Gauge 暴露——`time() - 值 > 阈值` 即落后告警；无事件时参考 cursor 同样刷新，空闲不误报。
- **B. watch 拉取（日志）**：每批首尾 cursor 的事件时间、拉取失败的原始错误与连续失败次数、事件类型分布（增/删/改）。
- **C. 消费分发（cloud-server 日志）**：桶级日志（桶开始/结束各一条，含厂商/隔离空间/主机数/小批数/耗时）、分发耗时拆解（classifyHost 两段外部调用）、**失败跳过计数（覆盖 ziyan 链路全部静默跳过路径，带 hostIDs）**、超阈值慢批 warn、delete 路径单列。
- **D. hc-service 同步（日志）**：批次总览（五段耗时 + 入口 + 来源 + 回退标记）、外部调用明细统一机制（CC / 云 API / DS 逐次记录，云限流错误单独分类）、sync_assoc_res / sync_host_data / sync_sg_relation 三段汇总。
- **E. 并发抢资源判断（零代码）**：通过日志按 (账号, 业务) 聚合时间窗，判断全量/增量/手工是否撞车，不新增任何埋点。
- **F. 逃生门配套（日志）**：cursor 重置审计（旧值/新值/原因/触发方/时间），为后续"调整 token 从指定事件消费"提供审计基础。
- **G. watch 消费侧指标（Prometheus，阶段二）**：5 个指标（提交事件时间戳 / consume 耗时 / 事件数 / 去重主机数 / 子批同步成败），全部从 `WatchBatchTrace` 字段同源导出，深层调用点零新增，详见设计 D10。

关键设计约束：

- cursor 为 base64 编码文本（`\r` 分隔），第 5 字段是事件时间 unix 秒，已在测试环境验证可解码；
- CC watch 为长轮询（hold 约 20 秒），无需空转/轮询 QPS 防护；
- 增量链路当前未开启云 API 限流重试（`RequestSource` 为空），限流表现为整批失败，观测按此事实设计。

## Capabilities

### New Capabilities

- `cc-host-sync-watch-observability`: CC 主机增量同步（watch 消费）链路的可观测能力，覆盖 cloud-server watch 消费侧与 hc-service ziyan 同步侧的位置水位、拉取健康度、分发进度、同步分段耗时、失败跳过计数与 cursor 重置审计。

### Modified Capabilities

（无）

## Impact

- **cloud-server**：`cmd/cloud-server/service/watch/bkcc/`（watch 循环、consume、classify、桶级日志、指标 flush）。
- **hc-service**：`cmd/hc-service/logics/res-sync/ziyan/`（HostWithRelRes 五段）、`cmd/hc-service/service/sync/tcloud-ziyan/`（入口日志）。
- **公共包**：新增 `pkg/tools/timing` 通用机制包，供两侧的 `WatchBatchTrace`（cloud-server）与 `SyncHostTrace`（hc-service）共同构建；`pkg/metrics` 新增 `CCWatchSubSys` subsystem 常量与 `cc_watch.go` 指标定义（阶段二）；`pkg/serviced` 等核心包零改动。
- **etcd**：无新增 key（消费位置复用既有 cursor key `/hcm/event/cc/`）。
- **外部依赖**：无新增依赖；指标复用仓库既有 `pkg/metrics`（Prometheus client），`/metrics` 暴露与采集链路已在线上就绪。
- **行为兼容性**：纯观测输出（日志 + 指标），不改变同步流程、错误处理与 cursor 推进行为。
- **关联变更**：`ziyan-host-cvm-cache`（ListCvm 重复拉取治理）。本变更 D 组外部调用明细是其收益验证手段（fill 阶段云调用次数应归零，仅剩缓存 miss 补拉）。
- **多服务层影响**：涉及 service 层（cloud-server）与 resource 层（hc-service）两层，均已标注。
