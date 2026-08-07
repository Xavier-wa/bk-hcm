## Context

自研云周期同步在 `SyncAllResource` 中先同步公共资源，再同步账号私有资源。当前公共资源 `region/zone/image` 由 `SyncPublicResource` 顺序调用，但没有接入 `detail.SyncDetail`，成功时不会写入 `account_sync_detail`。因此成功同步不会刷新最近同步时间，且从未失败过的资源不会创建状态项。

私有资源（如 vpc/subnet/security_group/load_balancer/device_type）已经在各自同步函数中调用 `ResSyncStatusSyncing/Success`，失败时由上层记录失败状态。本次设计目标是让公共资源状态记录与这些资源保持一致。

## Goals / Non-Goals

**Goals:**

- 为自研云公共资源 `region/zone/image` 写入 `syncing/sync_success/sync_failed` 状态。
- 保证公共资源同步成功后刷新最近同步时间。
- 保证 `zone` 等从未失败过的公共资源也能创建状态项并出现在账号资源状态列表中。
- 保持公共资源失败时向上返回具体资源类型和错误，便于现有调度链路感知失败。

**Non-Goals:**

- 不处理主机同步唯一键冲突问题。
- 不处理负载均衡 `lb-fake*` 脏数据问题。
- 不调整其他云厂商公共资源同步逻辑。
- 不新增对外接口或前端展示字段。

## Decisions

### Decision 1: 在 `SyncPublicResource` 编排层统一记录公共资源状态

`SyncPublicResource` SHALL 接收 `*detail.SyncDetail`，并围绕 `SyncRegion`、`SyncZone`、`SyncImage` 调用统一写入同步状态。

原因：
- 公共资源函数当前只负责调用 hc-service 同步，不持有 data-service client。
- 状态记录属于 cloud-server 的同步编排职责，放在 `SyncPublicResource` 更符合现有边界。
- 避免分别改造 `region.go`、`zone.go`、`public_image.go` 的核心同步逻辑。

替代方案：
- 在 `SyncRegion/SyncZone/SyncImage` 内部传入 `sd` 并写状态。否决：会让底层同步函数承担状态编排职责，且三个函数重复相同状态处理。

### Decision 2: 先创建 `SyncDetail` 再执行公共资源同步

`SyncAllResource` 当前在公共资源同步之后才创建 `sd`。实现时 SHALL 调整顺序，先创建 `SyncDetail`，再传入 `SyncPublicResource`。

原因：
- 公共资源需要使用同一个 `account_id/vendor/dataCli` 写入 `account_sync_detail`。
- 与后续私有资源共用同一个状态记录上下文，减少重复构造。

### Decision 3: 公共资源失败时立即记录失败并返回

每个公共资源同步 SHALL 按以下顺序处理：
1. `ResSyncStatusSyncing`
2. 执行实际同步
3. 失败时 `ResSyncStatusFailed` 后返回
4. 成功时 `ResSyncStatusSuccess`

原因：
- 与私有资源状态语义保持一致。
- 失败原因应落库，前端和排查人员可以看到明确错误。
- 当前公共资源顺序同步，失败后返回符合既有中断行为。

### Decision 4: 不改变公共资源同步范围

本次变更 SHALL 保持当前 `syncPublicResource` 的触发策略：在每个租户+vendor 同步周期内公共资源仅随第一个账号触发一次。

原因：
- 该策略由现有 `cloud-sync-biz-filter` 规范约束。
- 本次只修正状态记录，不扩大同步范围或频率。

## Risks / Trade-offs

- [Risk] 公共资源成功状态写入后，会覆盖历史失败时间，可能影响对历史问题的追溯。→ Mitigation：失败原因仍保留在日志中，状态表应反映最新一次同步结果。
- [Risk] 状态写入失败会导致公共资源同步流程返回错误。→ Mitigation：沿用私有资源现有行为，状态记录失败属于同步状态不可确认，应暴露错误。
- [Risk] 公共资源仅随第一个账号同步，状态记录会归属到触发同步的账号。→ Mitigation：保持现有行为，不在本次变更中调整公共资源按账号展示的产品口径。
- [Risk] `SyncPublicResource` 中三类资源状态处理重复。→ Mitigation：实现时可抽取小型 helper 包装公共资源同步函数，避免重复错误处理。

## Migration Plan

1. 部署后等待下一轮自研云周期同步，`region/zone/image` 将写入最新状态。
2. 对已有历史失败的 `region/image`，下一次成功同步后状态会更新为成功并刷新时间。
3. 对缺失的 `zone` 状态，下一次成功同步后会新增状态项。
4. 如需回滚，恢复 `SyncPublicResource` 签名和调用顺序即可；已写入的状态记录保留为普通历史数据。

## Open Questions

- 公共资源状态是否应长期归属触发公共资源同步的第一个账号，还是未来需要按租户/vendor 维度单独展示。
- 当前前端对缺失状态项是否需要兜底展示“未同步”或“状态异常”。
