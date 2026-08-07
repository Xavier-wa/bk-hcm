## Context

批量分配主机时，cloud-server 会连带分配关联资源（磁盘 / EIP / 网卡），并为所有资源创建审计记录。当前调用链为：

```
cvm/assign.go → audit.ResBizAssignAudit(diskIDs...) → data-service CreateAudit
disk/assign.go → audit.ResBizAssignAudit(diskIDs...) → data-service CreateAudit  
eip/assign.go → audit.ResBizAssignAudit(eipIDs...) → data-service CreateAudit
nic/assign.go → audit.ResBizAssignAudit(nicIDs...) → data-service CreateAudit
```

`ResBizAssignAudit`、`ResDeliverAudit`、`ResCloudAreaBindAudit` 三个公共方法直接将全部资源 ID 组装到单个请求中调用 data-service。data-service 侧 `CloudResourceAssignAuditReq.Validate()` 校验 `len(req.Assigns) > BatchOperationMaxLimit(100)` 时拦截返回错误。

99 台主机若每台有系统盘 + 数据盘，磁盘总数轻松超过 100，触发拦截。

## Goals / Non-Goals

**Goals:**

- 在 `ResBizAssignAudit`、`ResDeliverAudit`、`ResCloudAreaBindAudit` 内部按 `BatchOperationMaxLimit`(100) 分批调用 data-service。
- 修正 data-service 侧三处 `shuold` → `should` 拼写错误。
- 所有关联资源类型自动受益，调用方无需修改代码。
- 保持现有错误处理语义：任一批次失败返回错误，已成功批次不回滚。

**Non-Goals:**

- 不调整 `BatchOperationMaxLimit` 的值（仍为 100）。
- 不优化其他审计类型（Update/Delete/Operation）的分批逻辑——当前无已知超限场景。
- 不改造 `BatchResOperationAudit` 的分批逻辑——由调用方自行控制分批传入。
- 不新增对外 API 或数据库 schema 变更。
- 不做前端错误提示优化（可作为后续独立需求）。

## Decisions

### Decision 1: 在公共审计方法内部统一做分批

`ResBizAssignAudit`、`ResDeliverAudit`、`ResCloudAreaBindAudit` SHALL 在方法内部对资源 ID 列表按 `constant.BatchOperationMaxLimit`(100) 分批，每批构造独立的请求调用 data-service。

原因：
- 三个方法是所有关联资源审计的统一入口，内部分批后所有调用方自动受益。
- 避免在每个调用方（disk/eip/nic assign）重复编写分批逻辑。
- 与 data-service 侧 `createAudit` 已有的 `slice.Split(auditOpts, constant.BatchOperationMaxLimit)` 分批模式对齐。

替代方案：
- 在各调用方（如 `disk/assign.go`）分别做分批后再调用 `ResBizAssignAudit`。否决：会导致分批逻辑分散在多处，违反 DRY 原则，且容易遗漏新的调用方。

### Decision 2: 分批粒度为资源 ID 数量

每批最多包含 100 个资源 ID，每个资源 ID 对应一条审计记录。分批仅按切片索引切割，不考虑资源类型混合情况。

原因：
- data-service 侧校验的是 `len(req.Assigns)`，与资源类型无关。
- 简单按索引分批实现最简洁，性能开销最小。

### Decision 3: 错误处理保持现有语义

任一批次调用失败时立即返回错误，已成功的批次不回滚。这与现有单次调用的行为一致——如果单次调用失败也不会回滚任何操作。

原因：
- 审计记录是异步可重试的，不需要严格的事务一致性。
- 引入回滚机制会增加复杂度，且收益不大。

### Decision 4: 拼写修正同步进行

三处 `shuold` → `should` 拼写错误 SHALL 在本次变更中一并修正。其中 `CloudResourceOperationAuditReq.Validate()` 中的 `"assign shuold"` 改为更准确的 `"operations should"`。

原因：
- 拼写错误误导用户，修复成本低。
- `OperationAudit` 的错误信息用 `operations` 比 `assign` 更准确。

## Risks / Trade-offs

- [Risk] 分批调用会增加整体耗时，批次越多耗时越长。→ Mitigation：单批次调用 < 1 秒，总耗时与批次数量线性相关，实际场景中关联资源通常不超过 2-3 批，额外耗时 < 100ms。
- [Risk] 某一批次失败时，已成功批次不会回滚，可能导致审计记录不完整。→ Mitigation：与现有行为一致；审计记录用于追溯，少量缺失不影响核心功能。
- [Risk] `ResCloudAreaBindAudit` 当前调用量较小，分批可能过度设计。→ Mitigation：统一在公共方法内分批，代码模式一致，维护成本更低。

## Migration Plan

1. 修改 `cmd/cloud-server/logics/audit/audit.go` 中三个公共方法，增加分批逻辑。
2. 修改 `pkg/api/data-service/audit/audit.go` 中三处拼写错误。
3. 本地编译验证 + 单元测试（如有）。
4. 使用「主机数 ≤ 100，但关联磁盘 > 100」的数据集进行回归验证。
5. 部署后观察批量分配主机的成功率。

如需回滚，恢复两个文件的原始版本即可，无数据迁移影响。

## Open Questions

- `BatchResOperationAudit` 是否需要同步优化分批逻辑？→ 经分析，该接口由调用方自行控制分批传入，当前无已知超限场景，暂不纳入本期范围。
