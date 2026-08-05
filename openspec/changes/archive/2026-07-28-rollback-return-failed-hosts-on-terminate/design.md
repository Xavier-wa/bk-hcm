# Design: 回收单退回失败时通过「终止」把失败机器退回业务空闲机

## Context

资源回收子单状态机（`cmd/woa-server/logics/task/recycler/dispatcher/`）的真实形态比需求文档描述的多一层：

```
TRANSITING ──TransitSuccess──▶ RETURNING ──ReturnSuccess──▶ RETURNING_PLAN ──▶ DONE
     │                             │                             │
TransitFailed                 ReturnFailed                 ReturnPlanFailed
     ▼                             ▼                             ▼
TRANSIT_FAILED               RETURN_FAILED               RETURN_PLAN_FAILED
                          （本次唯一回滚触发点）      （机器已回收成功，仅成本预测失败）
```

`RETURN_FAILED` 是本次唯一的回滚触发点：机器已中转成功、统一停在业务的回收中转池模块
「CR_IEG_资源服务系统专用退回中转勿改勿删」，公司侧回收失败后滞留在此。`RETURN_PLAN_FAILED` 语义上
机器已被公司侧回收成功，不能回滚。

现状约束：

- 终止入口 `TerminateRecycleOrder` 与内部 `terminateOrder` 各有一份**重复的**状态校验 `switch`
  （`recycler.go:1700` 与 `recycler.go:1725`），回滚只应挂载在其中一处。
- 已有回滚链路 `returner.rollbackTransit`（`returner.go:460`）服务于「退回单被驳回」场景，
  终点是**待回收**模块而非空闲机，且存在三个问题：所有机器统一取 `hosts[0].Operator`；
  `setHostOperator` 把 `operator` 与 `bk_bak_operator` 写成同一个值；整个函数只 `logs.Warnf` 不返回
  error，四步中任一步失败都被静默吞掉。
- 海垒侧**没有**回收中转池的 module ID。中转时只提供源模块「待回收」的 ID
  （`transit/cvm.go:93`），目标中转池由公司侧 shipper 接口 `hosts_to_cr_transit` 内部维护。
- `recycler` 结构体不直接持有 returner，但可经 `r.dispatcher.GetReturn()` 访问
  （现有 `RecoverReturnCvm` 即此用法，`recycler.go:224`）。

## Goals / Non-Goals

**Goals:**

- 终止接口在 `RETURN_FAILED` 子单上按机器粒度把退回失败的机器退回业务空闲机模块。
- 回滚幂等：用户因失败重试时，已到位的机器不重复流转。
- 回滚未完全成功时阻断终止，杜绝「单据已终止但机器还卡在中转池」的静默失败。
- 非 `RETURN_FAILED` 子单的终止行为与改造前完全一致。

**Non-Goals:**

- 不改「退回单被驳回自动回滚」现有链路的行为与终点。
- 不做中转阶段失败（`TRANSIT_FAILED`）的回滚。
- 不做自动重试、自动回退。
- 不新增表、字段、状态，不改接口协议，不改前端。
- 不改 `recycle_host.status`——回滚成功后仍为 `RETURN_FAILED`。
- 不放宽 `TRANSITING` / `RETURNING` 不允许终止的现有约束。

## Decisions

### D1：幂等判定「是否已到终点」，而非「是否还在起点」

需求文档 R-007 表述为「机器已不在回收中转池时跳过」。按字面实现需要 `SearchModule` 硬编码中文模块名
「CR_IEG_资源服务系统专用退回中转勿改勿删」反查 module ID，再 `FindHostBizRelations` 逐台比对。
该模块名由公司侧维护，改名即静默失效，是生产判定不可接受的耦合。

改为判定机器是否已落在业务的**内置模块**：`GetBizInternalModule(bizID)` 一次返回该业务全部内置模块，
按 `Default` 字段取全部三个内置模块——空闲机 `DftModuleIdle`(1)、故障机 `DftModuleFault`(2)、
待回收 `DftModuleRecycle`(3)，再用 `ListBizHost(BkModuleIDs=[内置模块], bk_asset_id IN 候选)`
查出已在内置模块的机器直接跳过。

**为什么取全部三个内置模块**（实现阶段确认）：CMDB 中内置模块的 `Default` 字段非 0，而自定义模块
（含回收中转池）的 `Default` 为 0。因此「机器在任一内置模块」正是「机器不在任何自定义模块（含中转池）」
的精确判定，不存在误跳过。只取空闲机与待回收会让落在故障机的机器重蹈下述死锁。

**为什么把「待回收」也算作已回滚**（对应需求文档 Q-001）：「退回单被驳回」时 `rollbackTransit` 把机器
送到**待回收**模块。若只认空闲机，这些机器会被判定为「未到位」而进入回滚，然后对一台已经不在中转池的
机器调用 `HostsCrTransit2Idle`——源模块不匹配，大概率报错，反而**永久阻断**该单据的终止。
把两个内置模块都视为「已离开中转池、已回到业务可见范围」，语义正确且避开这个死锁。

代价：驳回场景下机器最终停在「待回收」而非 R-003 要求的「空闲机」。这是需求文档 Q-001 本身在问的取舍，
已确认接受——即「驳回场景保持现状不动」。

**备选方案**：直接调 `HostsCrTransit2Idle` 不做任何判定，赌公司侧接口对已在空闲机的机器返回成功。
未采纳——该接口行为未经验证，且失败会阻断终止，赌注太大。若后续 spike 证明它天然幂等，D1 可退化为
纯粹的性能优化而非正确性依赖。

### D2：新写 `RollbackReturnFailedHosts`，不复用也不修改 `rollbackTransit`

新方法挂在 `returner.Returner` 上（与其复用的 `transferHost2BizIdle` 等 CMDB 辅助方法同处一个包），
落在独立文件 `returner/rollback.go`，不与 764 行的 `returner.go` 混在一起。

调用方式：给 `recycler` 结构体新增 `returner *returner.Returner` 字段，在 `New` 中直接赋值已创建的
`moduleReturner`。**不走** `r.dispatcher.GetReturn()`——该访问器已标注
`Deprecated: 不应该对外暴露`（`dispatcher.go:83`），新代码不应依赖它。

不复用：`rollbackTransit` 终点是待回收模块、无返回值、维护人处理有缺陷，三点都与本需求冲突。
不修改：它服务于驳回场景，属于「不引入回归」明确点名不许动的链路。两者共享底层
`transferHost2BizIdle` / `setHostOperator` 等原子方法即可。

### D3：以转移接口返回码为准，不做转移后复查（实现阶段修订）

原设计要求转移后再查一次 CMDB 确认机器到位，理由是不信任公司侧接口的返回码。评审后移除，原因有二：

其一，该复查的另一半价值——提供流转后变化的新主机 ID——随 D4 的调整一起消失了。
其二，`transferHost2BizIdle` 本身已正确返回错误（不同于 `rollbackTransit` 的静默吞错），
「接口返回成功但实际未生效」是一个未观测到的故障模式，为它付出每次回滚多一次 CMDB 往返的代价不划算。

结果：单次回滚的 CMDB 调用从「2 次查询 + N/10 次转移 + 1 次属性更新」降为「1 次查询 + N/10 次转移」，
直接缓解 AC-P01 的耗时压力。

### D4：不改动维护人（实现阶段修订）

原设计要求回滚后恢复每台机器的维护人与备份维护人。查证后移除：`setHostOperator` 全局**只被
`rollbackTransit`（驳回链路）调用**，回收与中转流程从不修改 CMDB 上的维护人，只是把它读出来存进
`recycle_host.Operator` / `BakOperator` 用于展示。既然没有任何东西被清掉，「恢复」就只是把原值
再写一遍。

残留风险：公司侧 `hosts_to_cr_transit` 是外部接口，是否会作为副作用清空维护人未经实测。
需求文档 AC-017 即为此验证点，若联调发现会清空，需重新引入恢复逻辑。

### D5：执行时序与挂载点

挂在 `terminateOrder` 的 per-order 循环内、`UpdateRecycleOrder` 之前：

```
for order := range orders:
    ├─ 现有状态 switch（TRANSITING/RETURNING 拒绝，DETECTING 取消调度）
    ├─ order.Status == RETURN_FAILED ?
    │     └─ 是 → RollbackReturnFailedHosts(kt, order)
    │              失败 → return err（不置终止态，关联记录也不更新）
    ├─ UpdateRecycleOrder(stage=TERMINATE, status=TERMINATE)
    ├─ rsLogic.UpdateReturnedStatusBySubOrderID(滚服)
    └─ srLogic.UpdateReturnedStatusBySubOrderID(短租)
```

`TerminateRecycleOrder` 里那份重复的状态校验保持只读校验语义，不在其中挂载回滚。

### D6：多子单遇错中断，沿用现有行为

`terminateOrder` 当前就是遇错 `return`，前面已处理的子单保持已终止。不改为「独立处理 + 汇总返回」，
这回答了需求文档的 Q-002：默认接受现有行为。

### D7：回滚部分成功不做反向补偿

分批转移时中途失败，已到空闲机的批次保持原样。本次终止判定为失败，依赖 D1 的幂等性支持用户重新触发
终止推进剩余机器。

## Risks / Trade-offs

- **物理机路径未经验证** → 现有 `getHostIDByAsset` 的 `ListBizHost` 硬挂 `bk_cloud_id = 0` 过滤
  （`returner.go:545` 注释 "support bk_cloud_id 0 only"）。AC-014 要求物理机同样生效，而
  `rollbackTransit` 虽被 `pm.go` 调用却静默吞错，不能证明该路径跑通过。**实现前必须用物理机子单实测**，
  必要时去掉该过滤条件。
- **性能可能撞上 AC-P01** → 每子单 2 次 `ListBizHost` + N/10 次 `HostsCrTransit2Idle` + 1 次
  `UpdateHosts`；100 台需 10 次转移调用，公司侧单次 3 秒即触及 30 秒上限。缓解：批次间并发，
  或实测后与需求方协商放宽 AC-P01。
- **`bk_asset_id IN [...]` 上限未知** → 拟一次传 100 个固资编号。缓解：按 `pkg.BKMaxInstanceLimit`
  分片查询，与既有代码保持一致。
- **回滚成功但机器状态仍是 `RETURN_FAILED`** → 页面显示这批机器「退回失败」，实际已回到空闲机可用。
  信息误导但不影响功能（幂等判定依据是 CMDB 实际位置而非 DB 状态）。需求方已确认接受（R-010）。
- **长期无法退回的机器会永久阻断终止** → 若某台机器因公司侧原因始终转不回来，用户将无法终止该单据。
  当前无兜底手段，对应需求文档 Q-003，未决。
- **同一批机器被并发终止** → 两个用户同时点终止，可能重复调用 CMDB 转移。缓解：D1 的幂等判定使重复
  调用无害；未引入分布式锁。

## Migration Plan

无数据迁移、无 DDL、无配置变更。单点代码改造，随 woa-server 常规发布。

回滚策略：改造集中在 `terminateOrder` 的一个分支内，非 `RETURN_FAILED` 子单不经过新代码路径，
回退只需还原该分支。已经通过新逻辑退回空闲机的机器不受版本回退影响（CMDB 侧状态已落地）。

## Open Questions

- ~~**OQ-1（承接 Q-001）**：把「待回收」模块也视为「已回滚、跳过」是否可接受~~
  **已确认接受**，驳回场景保持现状不动，见 D1。
- **OQ-2**：物理机是否全部落在 `bk_cloud_id = 0`（阻塞 AC-014）。
- **OQ-3**：`HostsCrTransit2Idle` 对「已不在中转池」的机器的实际行为（决定 D1 是正确性依赖还是性能优化）。
- **OQ-4**：AC-P01 的 30 秒是否需要放宽，或必须做批次并发。
- **OQ-5（承接 Q-003）**：是否需要强制终止（跳过回滚）的兜底手段。
