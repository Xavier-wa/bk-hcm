## 1. 前置验证（阻塞后续实现，先做完再动代码）

> 实现已采用防御性写法规避这四项的阻塞：新查询不带 `bk_cloud_id` 过滤、
> 已在内置模块的主机不会走到转移调用、`IN` 已按 `pkg.BKMaxInstanceLimit` 分片、未预先并发。
> 以下验证仍需在真实环境执行，用于确认假设并决定是否需要调整。

- [ ] 1.1 验证 OQ-2：查询若干物理机裁撤子单的机器在 CMDB 中的 `bk_cloud_id`，确认是否全部为 0。
      新增的 `listHostsInBizInternalModule` 已不带该过滤，此项用于确认无需为物理机特殊处理
- [ ] 1.2 验证 OQ-3：在测试环境对一台已不在回收中转池的机器调用 `HostsCrTransit2Idle`，记录返回行为。
      结果决定 D1 的幂等判定是正确性依赖（报错）还是纯性能优化（返回成功）
- [ ] 1.3 验证 OQ-4：实测 `HostsCrTransit2Idle` 单批耗时，估算 100 台（10 批）总耗时是否满足
      AC-P01 的 30 秒；超出则在 `transferHost2BizIdle` 外层实现批次间并发，或与需求方协商放宽
- [ ] 1.4 验证 `ListBizHost` 的 `bk_asset_id IN [...]` 单次元素上限，确认
      `pkg.BKMaxInstanceLimit` 分片值合适

## 2. CMDB 能力补齐与回滚方法实现

- [x] 2.1 在 `pkg/thirdparty/api-gateway/cmdb/ziyan.go` 新增 `GetBizInternalModuleIDs`，
      一次返回业务的全部内置模块 ID（空闲机/故障机/待回收），
      实现方式对照现有 `GetBizRecycleModuleID`；同步在 `ZiyanCmdbClient` 接口声明
- [x] 2.2 在 `returner` 包新增 `listHostsInBizInternalModule(kt, bizID, assetIDs)`：
      按 2.1 拿到的模块 ID 调 `ListBizHost(BkModuleIDs=[内置模块], bk_asset_id IN assetIDs)`，
      返回已在内置模块的固资编号集合，用于幂等判定
- [x] 2.3 在 `returner` 包新增 `getReturnFailedHosts(kt, suborderID)`：
      查 `recycle_host` 表 `suborder_id + status = RETURN_FAILED`，
      分页沿用 `pkg.BKMaxInstanceLimit` 模式；查询条件抽为 `returnFailedHostFilter` 便于单测
- [x] 2.4 ~~新增 `restoreHostOperators`~~ **已移除**（实现阶段查证）：`setHostOperator` 全局只被
      驳回链路 `rollbackTransit` 调用，回收与中转流程从不修改 CMDB 维护人，只读出来存档用于展示。
      没有任何值被清掉，「恢复」只是把原值再写一遍。design.md D4 已改写，需求文档新增 R-011
- [x] 2.5 新增导出方法 `Returner.RollbackReturnFailedHosts(kt, order) error`，编排：
      取失败机器 → 查已在内置模块的机器并求差集 → 差集为空则直接返回 nil →
      分批 10 台调 `HostsCrTransit2Idle`。
      CMDB 部分拆为 `rollbackHostsToBizIdle(kt, order, hosts)`，与 DB 查询解耦以便单测。
      **转移后复查已移除**（design.md D3 改写）：其提供新主机 ID 的职责随 2.4 消失，
      剩余的「不信任返回码」属防御性设计，代价是每次回滚多一次 CMDB 往返，不划算
- [x] 2.6 在 2.5 中实现失败路径：任一步失败即返回 error 并记录错误日志，
      日志内容包含单据号、子单号、失败机器固资编号、失败原因与 rid；
      已成功的批次不做反向补偿。批次间并发待 1.3 结论决定，当前保持串行

## 3. 终止流程接入

- [x] 3.1 在 `recycler.terminateOrder`（`recycler.go:1722`）的 per-order 状态 switch 中新增
      `case table.RecycleStatusReturnFailed`，于 `UpdateRecycleOrder` **之前**执行
      `r.returner.RollbackReturnFailedHosts(kt, order)`，
      失败则 `return err`，不置终止态、不更新滚服与短租记录
- [x] 3.2 确认 `TerminateRecycleOrder`（`recycler.go:1678`）中那份重复的状态校验 switch
      保持纯只读校验，不在其中挂载回滚，避免双重执行
- [x] 3.3 ~~核对 `dispatcher.GetReturn()`~~ 该访问器已标注 `Deprecated: 不应该对外暴露`，
      改为给 `recycler` 结构体新增 `returner *returner.Returner` 字段，在 `New` 中赋值
      已有的 `moduleReturner`；design.md D2 已同步更正
- [x] 3.4 确认 `rollbackTransit`（`returner.go:460`）及驳回链路零改动

## 4. 测试

> 单测位于 `returner/rollback_test.go`，用嵌入 `cmdb.Client` 接口的轻量 fake 打桩，
> 未实现的方法保持 nil，一旦被调用即 panic，可反证回滚流程未越界调用其他 CMDB 接口。

- [x] 4.1 单测：`TestReturnFailedHostFilter` 校验查询条件保留 `status = RETURN_FAILED`，
      缺失该条件会把公司侧已回收成功的机器一并纳入回滚
- [x] 4.2 单测：`TestRollbackHostsToBizIdle_AllSettled` 覆盖幂等分支——
      全部已在内置模块时不发起任何转移调用
- [x] 4.3 单测：错误路径与筛选逻辑——`_TransferFailed`、`_ModuleQueryFailed`、`_ListFailed`
      （后两者断言查询失败时不发起任何转移）、`_PartiallySettled`（断言只转移未到位的机器）、
      `TestFilterUnsettledAssets`。原 `_RecheckNotLanded` 随转移后复查一并移除
- [x] 4.4 ~~单测：状态分流~~ **不适用**（已决策）。回滚是 `terminateOrder` 状态 switch 中的单一
      `case RecycleStatusReturnFailed`，`TRANSIT_FAILED`、`RETURN_PLAN_FAILED`、`DETECT_FAILED`、
      `REJECTED` 不触发回滚由结构保证；`terminateOrder` 依赖全局 `dao.Set()` 与 `rsLogic`/`srLogic`，
      该包无集成测试脚手架，改由 4.10 手工回归覆盖
- [ ] 4.5 联调：构造云主机 `RETURN_FAILED` 子单，验证 AC-001、AC-002
- [ ] 4.6 联调：构造物理机裁撤 `RETURN_FAILED` 子单，验证 AC-014
- [ ] 4.7 联调：模拟 CMDB 调用失败，验证 AC-003（子单保持 `RETURN_FAILED`、滚服与短租记录不变更）
- [ ] 4.8 联调：25 台分批场景第 2 批失败后重新终止，验证 AC-004 与 AC-005 的幂等重入
- [ ] 4.9 联调：驳回后机器已在待回收模块的子单执行终止，验证 AC-006 跳过且不被阻断
- [ ] 4.10 回归：`TRANSITING`/`RETURNING` 仍拒绝终止（AC-009）；`DONE`/`TERMINATE` 拒绝重复终止
      （AC-010、AC-011）；无权限用户被拒（AC-013）
- [ ] 4.11 性能：100 台规模实测终止接口耗时，验证 AC-P01

## 5. 收尾

- [x] 5.1 `go build ./cmd/... ./pkg/...`、`go vet`、`gofmt` 均通过，日志格式符合项目规范
      （`err: %v` 在前、`rid: %s` 在末尾）。golangci-lint 本地未安装，需在 CI 复核
- [x] 5.2 回写 `docs/reqs/回收退回失败终止回滚.md`：已补充 `RETURN_PLAN_FAILED` 状态与状态机图、
      修正 R-007 的幂等判定方案（改为「已落在业务内置模块」并给出等价性论证）、
      落实 Q-001 与 Q-002 结论（Q-001 升级为避免死锁的必要选择）、新增 R-011（回滚不改维护人）
      与 AC-016/AC-017、按范围收窄更新工时 24→17 人时（RICE 40→60，Effort 跨档至 2 人天）、
      补第 6 轮澄清记录
- [x] 5.3 确认无接口文档变更需求（接口路径与请求参数未变，无新增对外接口）
- [x] 5.4 回写 TAPD 需求 1069995598136403168：需求描述同步为收敛后版本，预估工时 24→17、
      价值规模 40→60；追加评论说明四处编码阶段查证结论、连带移除的复查逻辑与量化影响，
      并挂出 Q-003 待确认及联调需实测的 3 项（AC-017 维护人、物理机 `bk_cloud_id`、AC-P01 性能）
