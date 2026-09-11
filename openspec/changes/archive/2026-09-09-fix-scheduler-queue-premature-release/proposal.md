## Why

woa-server 任务调度链路的两个 informer 在把队列元素返回给调用方**之前**就释放了该元素，元素实际上从未处于 workqueue 的「处理中」状态，「同一元素同一时刻只被一个 worker 处理」这层保护完全失效。于是同一申请子单（dispatcher 侧 20 worker）／同一条生产记录（matcher 侧 60 worker）在本轮处理尚未结束时就能被再次派发，并发 worker 各自读到相同的已生产数量快照，对同一批缺口重复下单 —— 线上申请单 104398 的 SA3.2XLARGE32 子单需求 424 台、实际交付 446 台，业务方需为多出的 22 台承担 3 年包年包月费用。

## What Changes

- 申请子单从 apply informer 取出后进入「处理中」状态，本轮调度处理结束前不得被派发给第二个调度 worker；处理结束后（含提前返回与失败）必须被释放，可被下一轮轮询再次派发
- 生产记录从 generate informer 取出后进入「处理中」状态，本轮匹配处理结束前不得被派发给第二个匹配 worker；处理结束后（含提前返回与失败）必须被释放
- 队列元素的释放时机由调用方（dispatcher / matcher）在本轮处理边界上掌握，informer 不再在返回前自行释放；informer 已停机与非预期类型元素两条边界路径保持不占位、不 panic
- 上述行为使任一子单的已交付台数不超过其需求总数（`success_num ≤ total_num`），并覆盖走同一 informer 的**全部资源类型**（CVM / IDC DVM / QCLOUD DVM / PM / 升降配 CVM），因为缺陷位于与资源类型无关的通用调度层
- 交付数统计口径（按 `is_delivered = true` 的设备条数计数）与前端展示口径保持不变，不做任何调整
- 非 **BREAKING**：不新增/不修改对外接口、不改表结构、不改既有字段语义

明确不在本次范围内：交付数超发告警、存量单据超发对账清单、前端「已交付 > 总数」视觉提示、超发机器的自动回收/退还、单据 104398 已超发 22 台的处置、云梯侧多产问题、以自动化单元测试作为验收手段、`total_num`/`success_num`/`pending_num` 语义或前端展示口径的调整。

## Capabilities

### New Capabilities

- `apply-suborder-dispatch-exclusion`: 申请子单在 apply informer → dispatcher 链路上的独占派发与到期释放（对应 F-001 / R-002 / R-004，含全资源类型覆盖与停机、异常元素两条边界路径）
- `generate-record-match-exclusion`: 生产记录在 generate informer → matcher 链路上的独占派发与到期释放（对应 F-002 / R-003 / R-004）
- `suborder-delivery-count-integrity`: 子单交付台数不超过需求总数，且交付数统计口径不变（对应 R-001 / R-005，是前两项独占派发能力要达成的业务效果与不得触碰的既有口径）

### Modified Capabilities

无。既有 spec 中 `apply-suborder-retry-wakeup`、`apply-suborder-finalize-suspend-guard`、`prediction-transit-pool-reentrant-match-loop` 均描述 matcher/generator 内部的收尾与匹配判定，本次不改变它们的任何要求；本次约束的是队列元素生命周期这一正交层面，因此全部走 ADDED。

## Impact

- **受影响服务**：仅 woa-server（Service Layer）；不涉及 Access / Resource / Infrastructure 层
- **受影响代码位置**（均在 `cmd/woa-server/logics/task/`）：
  - `informer/apply/apply.go` 的 `Pop` / `Done`（`apply.go:110-131`）
  - `informer/generate/generate.go` 的 `Pop` / `Done`（`generate.go:114-135`）
  - `scheduler/dispatcher/dispatcher.go` 的 `runWorker`（`dispatcher.go:74-112`）
  - `scheduler/matcher/matcher.go` 的 `runWorker`（`matcher.go:113-174`）
- **不受影响**：`list_ticket_apply` 等对外接口字段、`ziyan_cvm_apply_suborder` / `ziyan_cvm_generate_record` / `ziyan_cvm_device_info` 表结构、云梯 CRP 与蓝鲸 CMDB 的集成方式、`generator.go` 的入口总闸与在途数统计逻辑（其失效原因是串行处理前提被破坏，而非漏算）
- **主要风险**：活性退化。元素在「处理中」状态停留更长时间，若某条路径漏掉释放，该元素在进程重启前将永不被派发，危害高于原有的重复派发缺陷；缓解措施见 `design.md`
- **依赖与迁移**：无新增依赖、无数据迁移、无配置变更；行为在进程重启后即刻生效
