# 实施任务

## 1. 公共基础（cloud-server）

- [x] 1.1 实现 cursor 解码工具：base64 解码 + `\r` 分隔解析，提取事件时间（第 5 字段）；解析失败降级返回原值与"未知"标记。事件类型分布来自 watch 返回的 `Events[].EventType`，不从 cursor 解码
- [x] 1.2 实现通用机制包 `pkg/tools/timing`（D8）：加锁的步骤耗时收集与汇总渲染、nil 安全降级，供 `WatchBatchTrace` / `SyncHostTrace` 共同构建
- [x] 1.3 实现 `WatchBatchTrace`（强类型，方法集 `Track`/`AddStep`/`Summary`）：含批次/计数/桶统计字段，同时渲染日志汇总行与 etcd 记录；种子点在 watch 循环每轮迭代新建 kt 后经 `NewSubKitWithCtx` 挂载
- [x] 1.4 ~~批首、批尾各写一次 etcd 观测记录~~（已随指标落地整套移除，见 6.6；批级统计改由 `hcm_cc_watch_*` 指标与日志承载）
- [x] 1.5 ~~实现单一只读查询接口（D9）~~（已随指标落地整套移除，见 6.6；消费位置改由 `committed_event_timestamp` 指标承载）

## 2. cloud-server watch 拉取观测（B 组 + A-3）

- [x] 2.1 watch 循环日志增强：批首/批尾 cursor 及解码事件时间、事件条数、事件类型分布、watched 标记、拉取耗时
- [x] 2.2 watched=false 分支：不加日志，只更新 cursor 提交时间。原计划的「参考 cursor 跳变告警」已放弃 —— `WatchHostRelationEvent` 只订阅 create，链上的 update/delete 会被 cc 跳过并返回更新的参考 cursor，跳变属正常现象，告警必然误报；host 流的落后程度由 cursor lag 直接回答
- [x] 2.3 拉取失败：日志记录连续失败次数（恢复清零）+ 原始错误。**不做错误分类** —— 原始错误本身已可读，分类是有损重编码；且 `errors.Is(err, context.DeadlineExceeded)` 在本链路恒为 false（watch kit 无 deadline，超时由 ResponseHeaderTimeout 产生，不是 DeadlineExceeded），链节点失效则已由紧邻的 `strings.Contains` 分支体现

## 3. cloud-server 消费分发观测（C 组）

- [x] 3.1 桶级日志：桶开始一条（厂商/隔离空间/主机数）、桶结束一条（追加已完成与失败小批数/耗时/状态），另有事件数→去重后主机数一条。小批只在单批耗时超过 30s 时打 warn，不逐批打日志
- [x] 3.2 分发耗时拆解：consume 整批耗时 + classifyHost 子步耗时（CC 查业务归属、DS 查厂商）
- [x] 3.3 失败跳过计数（当前只覆盖 ziyan 链路）：ziyan hc 批次失败、获取账号失败整桶跳过、space 转换失败，加上厂商无关的 classifyHost 跳过、厂商不支持、delete 失败，均带 hostIDs。其余厂商的 `update*Host` 暂不埋点，仅通过 `upsertHostByDiffSpace` 的桶级日志覆盖（主机数/耗时/状态）
- [x] 3.4 delete 路径单列日志：删除台数/耗时/hostIDs
- [x] 3.5 埋点同步更新 `WatchBatchTrace`（etcd 记录与日志同源）

## 4. hc-service ziyan 同步观测（D 组）

- [x] 4.0 实现 `SyncHostTrace`（构建在 `pkg/tools/timing` 之上）：在 `HostWithRelRes` 入口挂载种子并以 defer `logSummary` 兜底输出总览（失败提前返回的路径同样输出已记录步骤）；方法集为 `Track` + `Set*` + `logSummary`（与 `WatchBatchTrace` 同源思路，非同一接口）。`cvm-rel-manager` 内用通用 `timing`/`RelSyncStats`，不引入 ziyan 专用类型
- [ ] 4.1 外部调用明细统一 helper：目标类型（CC/云API/DS）、接口、阶段标签、region、耗时、条数、错误类型（云限流单独分类），接入全部调用点（含 fillCloudFields 的 ListCvm、删除入口的 DS 删除、getVpcMap/getSubnetMap 的 DS 查询）；`getCVM`（并发 10）与 `syncCvmRelRes`（并发 20）等并发路径记录 MUST 带 region / resType 标签
- [x] 4.2 批次总览日志：五段耗时（list_biz_host / list_cloud_cvm / sync_assoc_res / sync_host_data / sync_sg_relation）+ 回退标记（path=full|host_only）+ 账号/业务/主机数/cc hosts/cvms/regions/db 增删改/rid。**尚缺**：入口（upsert/delete/全量）、来源（RequestSource）
- [x] 4.3 sync_assoc_res 步骤耗时：按资源类型聚合为 sync_assoc_vpc / sync_assoc_subnet / sync_assoc_sg（ziyan 关联资源本体仅此三种，无 Disk/EIP）；另有 region×资源类型的过程 Info。**尚缺**：云拉取次数、DB 写行数细字段
- [x] 4.4 sync_host_data 子步耗时：list_cc_host / fill_cloud_field / read_host_db / create_host_db / update_host_db / delete_host_db 已进 steps；diff 三态数量经 SetHostWrite 进总览。**尚缺**：独立分段汇总日志
- [x] 4.5 sync_sg_relation 步骤耗时：整段 Track + region 级 `RelSyncStats` 过程 Info。**尚缺**：对比主机数/DS 读次数/写 RPC 次数等细字段进总览
- [x] 4.6 hc-service 指标泛化：`pkg/metrics/res_sync.go` 新增 `hcm_res_sync_cost_seconds{vendor,resource,step,source}`；`source` 由 `RequestSource` 推导（`asynchronous_tasks`→full，其余→incremental）；启动注册 `EnsureResSyncMetric`
- [x] 4.7 trace 基座抽象：`res-sync/common/trace.go` 新增 `ResSyncTrace`（步骤采集 + source 推导 + `FlushMetrics(err)` 白名单导出，带 result 维度），纯构造不挂 ctx（YAGNI，埋点均在入口函数内可见）；ziyan `SyncHostTrace` 改为组合内嵌基座（调用点零改动），只保留日志字段与 `logSummary`；主机环节常量收敛为 `common.ResSyncStepHost/RelRes/HostRel`
- [x] 4.8 tcloud 试点接入：全量 `CvmWithRelRes` 埋点（rel_res/host/host_rel 三段各一对 start/AddStep 合并计时，失败提前返回时段耗时不记、total 由 defer 兜底）；增量 `CvmCCInfo` 仅报 total（只更新 CC 字段，无关联资源环节）
- [x] 4.9 source 语义修正：弃用 `RequestSource` 推导（`AsynchronousTasks` 是云异步操作重试语义，async 任务交付后的按条件同步会误标 full），改为 `ResourceSync`/`ResourceSyncV2` 框架入口统一 `MarkFullSyncSource` 打标记，一处覆盖所有 vendor 全量入口；watch 增量/操作后回调/手工条件同步天然为 incremental
- [x] 4.10 source 补漏 + resource 口径修正：`SyncCvmCCInfo` 全量入口不走框架（非云资源同步不适配框架机制），service 入口手动 `MarkFullSyncSource` 补 full 标记（纯 ctx value，零业务侵入）；`CvmCCInfo` 埋点 resource 由 `cvm` 改为 `cvm_cc_info`，与 sync_detail 资源类型口径一致，避免与云镜像同步（分钟级）混分布导致分位数/告警失真
- [x] 4.11 埋点推广至 aws/azure/gcp/huawei：按 tcloud 样本接入 `CvmWithRelRes` 三段（rel_res/host/host_rel，NI 同步按位置并入相邻段：azure 入 rel_res、gcp/huawei 入 host）与 `CvmCCInfo` total；四个厂商 `SyncCvmCCInfo` service 入口同步补 full 标记

## 5. 审计与验证

- [x] 5.1 cursor 重置审计日志：链节点失效自动重置路径记录旧值/新值/原因/触发方/时间
- [ ] 5.2 测试环境验证：用真实 cursor 验证解码正确性；模拟失败批次验证跳过计数；核对五段耗时总和≈总耗时
- [ ] 5.3 多副本验证：非 master 实例查询结果与 master 一致；master 切换后消费位置仍正确、新 master 首批完成后记录被覆盖；确认 etcd 观测 key 的租约随写入续期、停写后自动过期
- [ ] 5.4 上线后观察日志量，必要时将外部调用明细降为 V(n) 级别

## 6. watch 消费侧指标（Prometheus，阶段二，见 D10）

- [x] 6.1 `pkg/metrics` 新增 `CCWatchSubSys = "cc_watch"` subsystem 常量
- [x] 6.2 新增 `pkg/metrics/cc_watch.go`：6 个指标定义与注册（`sync.Once` 防重，参照 `pkg/metrics/http_request.go` 样板）
- [x] 6.3 `WatchBatchTrace` 扩展：创建时固化 metric label（tenant）；新增 `AddSyncResult`（子批 hc 调用返回后即时自增 sync_total）；新增 `flushMetrics`（批末导出 consume_cost / events_total / hosts_total）；`timing.Collector` 补 `Cost(name)` 聚合访问器
- [x] 6.4 埋点接入：子批 RPC 返回处一行 `tr.AddSyncResult`；批末一行 `tr.flushMetrics`；cursor 提交成功后刷新 committed_event_timestamp（有效批末尾与 watched=false 参考 cursor 两处）
- [ ] 6.5 验证（待测试环境）：`/metrics` 暴露正确 label；构造失败子批验证 result=failed 计数；观察空闲期 committed_event_timestamp 随参考 cursor 刷新不误报；核对单台均耗 PromQL 分子分母同点（本地编译/vet/gofmt 已通过）
- [x] 6.6 代码收敛：指标定义迁至 `pkg/metrics/cc_watch.go`（按 subsystem 分文件的既有惯例）；删除低价值 `AddSkip` 及衍生死代码；整套移除 etcd 批次记录与查询接口（`putWatchRecord` / `status.go` / `pkg/api/cloud-server/watch` / admin 路由与 handler / `capability.EtcdCli` 透传 / `CursorDetail.AgeSeconds`）
- [x] 6.7 补齐启动注册与"处理中"指标：`app.go` 增 `metrics.EnsureCCWatchMetric()`（对齐 `EnsureCLBSubmitMetric` 惯例，避免惰性注册导致 `/metrics` 短暂缺元信息）；新增 `processing_start_timestamp_seconds` Gauge，与 `Track(stepConsume)` 合并为 `tr.TrackConsume()`（两个 consumeFunc 各一行 defer，res_type 随 trace 构造固化，`flushMetrics` 同步去掉入参），用于区分"卡在单次下游调用"与"无事件"
