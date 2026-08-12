# 设计：suspend 覆盖守卫、已足额交付时的直接收尾

## Context

主机申请子单会永久停在 `stage=RUNNING / status=MATCHING`。根因分析见 proposal.md，本文只复述两个直接决定实现方式的事实：

1. 子单离开 MATCHING 的唯一通路是 `UpdateApplyOrderStatus`，而它只能由 informer 唤醒。informer 捞取条件是 `status=Success AND is_matched=false AND updated_at ∈ ~10s 滑动窗口`。三个条件一旦不再同时成立，无任何机制能让子单离开 MATCHING。
2. `matcher.go:347` 的 suspend 覆盖在 `calcApplyOrderStatus` 已算出 DONE 之后无条件生效，会把已交付满额的单误判为 TERMINATE/SUSPEND；而被人工重新拉起后，`retryMatchDevice` 因全部设备已交付而空转，不再产生任何可唤醒 informer 的生产记录。

关键工程事实（已实现路径，非新建能力）：

- 补收尾作用于整张子单：`UpdateApplyOrderStatus` 按 `suborder_id` 重新统计全部设备，与从哪条生产记录进入无关。
- `matchDevice` 才是重跑生产链路的入口：初始化、磁盘检查、交付三步都在 `matchDevice` 内部。其中 `RunDiskCheck` 对已交付设备没有任何早返回，会对存量机器再次发起云侧磁盘检查，失败还带 3 次 × 180s 重试；`ProcessInitStep`/`DeliverDevice` 虽按 `IsInited`/`IsDelivered` 早返回，但绕过 `matchDevice` 后这些都不必再论证。
- matcher 不 import generator、generator 不 import matcher，无循环依赖；`scheduler.go` 里 matcher 先于 generator 构造，`Generator.SetMatcher` 照抄 `Dispatcher.SetGenerator` 的注入模式。
- `retryMatchDevice` 由 `GenerateCVM`/`UpgradeCVM`/`MatchPM` 三处调用，改为接收整张 `order`。

## Goals / Non-Goals

**Goals:**

- 已交付满额的子单不再被 suspend 覆盖误判为 TERMINATE/SUSPEND。
- 用户在界面上点「启动」后，已交付满额的子单能流转到 DONE，且不重复执行初始化/磁盘检查/交付。

**Non-Goals:**

- 不修复三张存量卡死单（102851-1、102956-1、103123-2），需人工解卡。
- 不做周期性检测、告警、自动恢复、失败留痕。
- 不改 informer、recoverer 的既有行为，不改 `retryFailedDevices`。
- 不新增表、不改表结构、不引入 metric 组件、不新增外部依赖。

## Decisions

### D1. suspend 守卫：加 `matchedCnt < TotalNum`，但跳过时必须告警，不静默

`matcher.go:347` 改为：

```go
if isSuspend && matchedCnt < int(order.TotalNum) && suspendCnt+matchedCnt >= int(order.TotalNum) {
```

条件不满足（即已交付满额但存在 Suspend 记录）时，**不静默跳过**，而是另起一个同级 `if` 打 warning，带上子单号、挂起记录的 generateID、已交付/需求台数。

**为什么保留核对信号**：Suspend 的语义是「云梯可能真的开了机器，只是我们没拿到 task_id」。改成 DONE 后单据关闭，若不留信号，幽灵机器会持续计费且无归属。守卫修复的是「状态判错」，不是「幽灵机器消失」。

**为什么只打日志、不写 `remark`**：`remark` 是用户备注，字段上限 255 字符，写机器标记需要幂等替换 + 长度兜底 + 用户内容保护三套逻辑，为一条提示引入的复杂度远超收益，且机器标记与用户文本混在一列本身是污染。告警日志已足够支撑人工核对。

**为什么不动 326–342 行的循环**：该循环在守卫之前，Suspend 记录仍会被 `updateGenerateFailed` 改写为 Failed 并写上 "check YunTi" 的 message。地雷照常清除、审计痕迹照常保留，守卫只影响终态取值，不影响这条链上的其他副作用。

**为什么不在守卫里直接调收尾**：守卫改变的是 `calcApplyOrderStatus` 算出的结果**不被覆盖**，后续 `updateApplyOrderToDb` 照常执行，自然落成 DONE。无需额外调用，改动面最小。

### D2. 重试收尾：在 `retryMatchDevice` 原过滤为空且无在途批次时，直接调用 `FinalApplyStep`

不修改原有 `!IsDelivered` 过滤循环，仅在其产出空集合时介入。收尾的前置条件照抄 matcher 的收尾口径——已交付数量达子单需求，且无在途批次：

已交付台数在原过滤循环里顺带累加；调用方已算好的 `generatingCount` 作为「无在途批次」的早退保险一并传入：

```go
genIDs := make([]string, 0)
// deliveredCnt 口径与 matcher 的收尾判定保持一致，均按设备 IsDelivered 计数
deliveredCnt := uint(0)
for _, device := range devices {
    if !device.IsDelivered {
        genIDs = append(genIDs, device.GenerateId)
        continue
    }
    deliveredCnt++
}

genIDs = utils.StrArrayUnique(genIDs)

// 无未交付设备时进入收尾分支：已交付数量已达子单需求且无在途生产批次，说明子单只差一次收尾。
// 收尾直接调用 matcher.FinalApplyStep，而不是再重置一条生产记录去唤醒 informer，
// 因为后者会把初始化/磁盘检查/交付整条链路重跑一遍，而其中磁盘检查对已交付设备并不幂等。
// 已交付数量不足时不介入，收尾由在途批次完成后触发。
if len(genIDs) == 0 && deliveredCnt >= order.TotalNum && generatingCount == 0 {
    return g.finalizeDeliveredOrder(kt, order)
}
```

`finalizeDeliveredOrder` 负责判断该不该补收尾，三个分支按顺序：

- 存在 Init/Handling 记录：还有批次在途，跑完自然触发收尾，不介入；
- 已存在 `Success 且 is_matched=false` 的记录：informer 下一轮会捞到它收尾，此处再执行会导致同一子单并发收尾（dispatcher 20 worker、matcher 60 worker 均无跨单据互斥），不介入；
- 其余情况从 `Success 且 is_matched=true` 的记录中任取一条作为 `FinalApplyStep` 入参。收尾按 `suborder_id` 重新统计全部设备，作用于整张子单，挑中哪条记录无差别。无候选时打 warning 返回。

```go
// finalizeDeliveredOrder 子单设备已足额交付但仍停在 MATCHING 时补一次收尾。
// 收尾作用于整张子单，与传入哪条生产记录无关，任取一条成功记录满足 FinalApplyStep 入参即可。
func (g *Generator) finalizeDeliveredOrder(kt *kit.Kit, order *types.ApplyOrder) error {
    records, err := g.getOrderGenRecords(kt, order.SubOrderId)
    if err != nil {
        logs.Errorf("failed to get generate records, subOrderID: %s, err: %v, rid: %s",
            order.SubOrderId, err, kt.Rid)
        return err
    }

    var genRecord *types.GenerateRecord
    for _, item := range records {
        // 还有批次在途，跑完自然触发收尾，此处不重复执行
        if item.Status == types.GenerateStatusInit || item.Status == types.GenerateStatusHandling {
            return nil
        }
        // informer 下一轮会捞到这条未匹配记录并收尾，此处再执行会并发
        if item.Status == types.GenerateStatusSuccess && !item.IsMatched {
            return nil
        }
        if item.Status == types.GenerateStatusSuccess && genRecord == nil {
            genRecord = item
        }
    }

    if genRecord == nil {
        logs.Warnf("no success generate record to finalize, subOrderID: %s, rid: %s", order.SubOrderId, kt.Rid)
        return nil
    }

    logs.Infof("finalize delivered suborder directly, subOrderID: %s, generateID: %s, rid: %s",
        order.SubOrderId, genRecord.GenerateId, kt.Rid)
    return g.matcher.FinalApplyStep(kt, genRecord, order)
}
```

**为什么直接调 `FinalApplyStep` 而不是经 informer 重跑整条链路**：informer 唤醒的唯一手段是把一条已完成记录的 `is_matched` 重置为 `false`，让其再走一遍 `matchHandler → matchDevice → FinalApplyStep`。这条链路里 `RunDiskCheck` 对已交付设备没有早返回，会再次发起云侧磁盘检查并 3 次 × 180s 重试；即便 init/deliver 幂等，磁盘检查这一条就足以排除该路径。直接调 `FinalApplyStep` 绕过 `matchDevice`，初始化/磁盘检查/交付一次都不重跑，副作用面从"逐个论证三步幂等"收敛为"只收尾一次"。

**为什么传一条成功记录而不是不传**:`FinalApplyStep` 的第一步是 `setGenerateRecordMatched(genRecord.GenerateId)`，需要一个 `GenerateId` 入参；收尾本身按 `suborder_id` 统计设备，与该记录的 `IsMatched` 无关，传入一条已成功记录只是满足签名，且对已 matched 记录重写 `is_matched=true` 是幂等 SET。

**终态判断不离开 matcher**：子单最终是不是 DONE 仍由 `FinalApplyStep → UpdateApplyOrderStatus → calcApplyOrderStatus` 决定，generator 不复制这份判断，也不绕过完成通知（`notifyApplyDone`/`checkAndNotifyDelivery`），避免在 generator 里手写一份"直接 UPDATE stage/status"而丢掉通知与 rolling 核数。

**新增依赖 generator → matcher 安全**：两者原本互不 import，无循环依赖；`scheduler.go` 中 matcher 先于 generator 构造，注入 `SetMatcher` 后 generator 持有其指针，与 `Dispatcher.SetGenerator` 完全同形。

**为什么改 `retryMatchDevice` 的签名**：三个调用点（`GenerateCVM`/`UpgradeCVM`/`MatchPM`）都已持有完整 `order`，改为 `retryMatchDevice(kt, order, existDevices, generatingCount)`，收尾需要整张 `order`（`TotalNum`、`SubOrderId` 及下游 `FinalApplyStep` 所需字段）。

### D3. D2 是 D1 的前置，两者必须在同一变更中合入

D2 触发的收尾会重新执行 `UpdateApplyOrderStatus`。如果此时该子单仍存在 Suspend 生产记录，347 行的原逻辑会把刚算出的 DONE 覆盖回 TERMINATE/SUSPEND，D2 白做。因此守卫（D1）必须先于或与收尾（D2）同时生效，不能单独合入 D2。

对 102851-1 这张具体单据，原 Suspend 记录已在首次收尾时被 326–341 循环改为 Failed，重跑时 `isSuspend=false`，守卫不触发。但这是个案运气，不构成放松该约束的理由。

## Risks / Trade-offs

- **[D2 触发并发收尾]** → 通过 `finalizeDeliveredOrder` 第二分支挡掉：已存在 `Success 且 is_matched=false` 记录时不重复执行。极端场景（两个 dispatcher worker 同时处理同一子单）下仍可能各自补一次收尾，但 `GenerateCVM` 的 `scheduledCount >= TotalNum` 早返回发生在锁单之后，同一子单不会被并发 dispatch，实际窗口不存在。
- **[D2 重复触发预测扣减]** → 已被线上数据证伪为既有问题而非本次引入：`res_plan_demand_changelog` 中 `type=expend` 记录存在大量「同一子单+同一预测单」数十行重复（最高单条 57 行、累计 -273.6 万核），且行数与该子单生产记录条数同量级，源头是 `UpdateApplyOrderStatus` 按 `suborder_id` 全量统计、`FinalApplyStep` 每条生产记录跑一次，多批次单在正常流程里就重复追加。该链路自 2025-08 起近休眠（近 11 个月仅 12 行）。本次改动与之同类且不加剧，结论见「已知遗留」。
- **[D1 把本应人工核对的 TERMINATE 变成了自动 DONE]** → 通过 warning 日志保留核对信号；同时「满额交付即 DONE」在业务语义上本就是正确的终态，TERMINATE 才是误判。
- **[核对信号只在日志，无人看则丢失]** → 接受该权衡：本次不做告警通道与周期性检测（见 Non-Goals），日志可由既有日志平台按关键字 `skip suspend override` 配置监控。
- **[generator 新增对 matcher 的依赖]** → 无循环依赖、照抄现有 setter 注入，已在 D2 论证；若后续 generator 与 matcher 需要解耦，可将 `finalizeDeliveredOrder` 上移为 scheduler 层的薄编排。

## Open Questions

### D1. 预测扣减链路失效是否需要单独跟进

`expend` 类型记录自 2025-08-20 起停止写入，而 `append`/`adjust` 正常。属本次排查副产品，与本变更无代码耦合，需业务方确认是否单独立项。

### D2. B 的兜底分支是否需要配置开关

倾向不加：行为符合用户点击「启动」时的预期，且回滚成本低。待实现前确认。

## 已知遗留

### `AddMatchedPlanDemandExpendLogs` 非幂等，正常多批次单即在重复追加

该函数对 `res_plan_demand_changelog` 裸 `INSERT`，无唯一键冲突处理、无「该子单是否已写过」的前置判断。因 `UpdateApplyOrderStatus` 按 `suborder_id` 全量统计设备、而 `FinalApplyStep` 每条生产记录跑一次，多批次单在正常流程里每个收尾批次都会把已交付核数再扣一遍（平坦的 N 倍重复，非递增）。线上数据已证实：单张子单同一预测单最高 57 行、累计 -273.6 万核，changelog 行数与生产记录条数同量级。这是系统自始已有的行为，与本变更无关，本次不处理。如需修复应单独提单，并注意 `addMatchedPlanDemandExpendLogs` 中 `inserts := make([]..., len(demandIDs))` 按 demandIDs 长度分配却按 Details 下标填充，demand 被删时会插入全零行的独立隐患。

## Migration Plan

纯小范围条件调整，无数据迁移、无接口变更：

1. 合入 D1（suspend 守卫）与 D2（直接收尾），两者在同一变更中，无顺序依赖。
2. 回滚：D1 恢复原 if 条件并删除告警块；D2 恢复 `retryMatchDevice` 原过滤逻辑、移除 `finalizeDeliveredOrder` 与 `SetMatcher` 注入，均无数据残留需清理。

三张存量卡死单不在本变更范围内，单独人工解卡。
