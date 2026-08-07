# Change: 回收单退回失败时通过「终止」把失败机器退回业务空闲机

## Why

资源回收在「退回」阶段调用公司侧接口（云主机走云梯 CRP、物理机走 ERP）失败后，子单置为 `RETURN_FAILED`，
失败的机器滞留在业务的回收中转池模块「CR_IEG_资源服务系统专用退回中转勿改勿删」——既不在业务可用范围内，
公司侧也没有真正回收，处于两不管状态。当前「终止」只把单据状态改成 `TERMINATE`，不做任何资源流转，
每次线上出问题都要研发直接改生产数据库把机器恢复回业务模块，有误操作风险且无审计。

本次让终止接口在 `RETURN_FAILED` 子单上按机器粒度回滚失败机器到业务空闲机模块，回滚不成功则阻断终止，
消除人工改库。对应 TAPD 需求 1069995598136403168，需求文档见 `docs/reqs/回收退回失败终止回滚.md`。

## What Changes

- 终止逻辑 `TerminateRecycleOrder` / `terminateOrder` 按子单状态分流：仅 `RETURN_FAILED` 触发回滚，
  其余状态（`DETECT_FAILED` / `REJECTED` / `TRANSIT_FAILED` / `RETURN_PLAN_FAILED` / 已提单 / 未提单）
  行为与改造前完全一致。
- 新增回滚方法（**不复用** `returner.rollbackTransit`）：筛出子单下 `status = RETURN_FAILED` 的机器，
  分批 10 台调 `HostsCrTransit2Idle` 转到业务空闲机模块。
- 幂等判定改为「是否已到终点」而非「是否还在起点」：用 `GetBizInternalModule` 取业务的全部内置模块 ID
  （空闲机 / 故障机 / 待回收），`ListBizHost` 按 `bk_module_ids` + `bk_asset_id` 查出已在内置模块的
  机器直接跳过。内置模块的 `default` 非 0、自定义模块（含中转池）为 0，故「在内置模块」即「不在中转池」。
  避免硬编码中文模块名去反查回收中转池（海垒侧无该模块 ID，中转池由公司侧 shipper 接口维护）。
- **不改动维护人**：`setHostOperator` 全局只被驳回链路调用，回收与中转流程从不修改 CMDB 上的维护人，
  只读出来存档用于展示，因此无需恢复。回滚成功以 `HostsCrTransit2Idle` 的返回码判定。
- 执行顺序固定为「先回滚、后置终止态」。回滚存在失败则返回错误，子单保持 `RETURN_FAILED`，
  关联的滚服/短租回收记录状态一并不更新。
- 新增 `GetBizInternalModuleIDs` 辅助方法（对照现有 `GetBizRecycleModuleID` 实现）。
- 行为变更：终止在 `RETURN_FAILED` 场景下由「必然成功」变为「可能失败」。接口路径与请求参数不变，
  前端零改动（**非** BREAKING）。

## Capabilities

### New Capabilities

- `recycle-terminate-rollback`: 资源回收单终止时对退回失败机器的回滚能力——按机器粒度识别 `RETURN_FAILED`
  机器、幂等地退回业务空闲机模块，回滚不成功则阻断单据进入终止态。

### Modified Capabilities

<!-- 现有 specs 中无覆盖资源回收终止流程的能力，本次以新能力形式引入 -->

## Impact

- **终止逻辑（核心）**：`cmd/woa-server/logics/task/recycler/recycler.go` 的 `TerminateRecycleOrder`
  与 `terminateOrder`（两处存在重复的状态校验 switch，回滚只在一处挂载）
- **回滚实现**：`cmd/woa-server/logics/task/recycler/returner/` 下新增方法
- **CMDB 客户端**：`pkg/thirdparty/api-gateway/cmdb/ziyan.go` 新增 `getBizIdleModuleID`；
  复用 `GetBizInternalModule`、`ListBizHost`、`HostsCrTransit2Idle`、`UpdateHosts`
- **常量**：复用 `pkg/thirdparty/api-gateway/cmdb/constvar.go` 的 `DftModuleIdle`
- **零改动**：前端、接口协议、数据库表结构与字段、「退回单被驳回自动回滚」现有链路
- **不改动机器状态**：回滚成功后 `recycle_host.status` 仍为 `RETURN_FAILED`（需求方确认，见 R-010）

## Open Questions

- **OQ-1**：现有 `getHostIDByAsset` 的 `ListBizHost` 查询硬挂 `bk_cloud_id = 0` 过滤
  （`returner.go` 注释 "support bk_cloud_id 0 only"）。AC-014 要求物理机路径同样生效，需确认物理机
  是否全部落在 `bk_cloud_id = 0`。现有 `rollbackTransit` 虽被 `pm.go` 调用，但它静默吞错，
  不能作为物理机路径已验证的证据。
- **OQ-2**：性能。每子单 2 次 `ListBizHost` + N/10 次 `HostsCrTransit2Idle` + 1 次 `UpdateHosts`；
  100 台需 10 次转移调用，公司侧单次 3 秒即撞上 AC-P01 的 30 秒上限。需实测决定是否批次间并发或放宽 AC-P01。
- **OQ-3**：`ListBizHost` 的 `bk_asset_id IN [...]` 单次元素数量上限未确认（拟一次传 100 个固资编号）。
- **OQ-4**（承接需求文档 Q-003）：回滚失败阻断终止后，若机器因公司侧原因长期无法退回，用户将始终无法
  终止该单据。是否需要强制终止（跳过回滚）的兜底手段。
