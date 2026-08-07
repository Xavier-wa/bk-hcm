## Why

自研云公共资源 `region/zone/image` 同步成功时未写入 `account_sync_detail`，导致成功同步后最近同步时间不刷新、从未失败过的资源（如可用区）不展示状态项。需要补齐公共资源状态记录，使账号资源同步状态展示与实际同步结果一致。

## What Changes

- 为自研云公共资源同步补充 `syncing/sync_success/sync_failed` 状态记录。
- 让 `region/zone/image` 成功同步后刷新资源同步状态和最近同步时间。
- 让 `zone` 等从未失败过的公共资源也能创建并展示同步状态项。
- 保持主机、负载均衡等已有资源同步流程不变；其失败原因另行排查，不纳入本次行为改造。

## Capabilities

### New Capabilities

- `ziyan-public-resource-sync-status`: 规范自研云公共资源 `region/zone/image` 的同步状态记录与展示行为。

### Modified Capabilities

- 无。

## Impact

- Affected code:
  - `cmd/cloud-server/service/sync/tcloud-ziyan/sync_all_resource.go`
  - `cmd/cloud-server/service/sync/tcloud-ziyan/sync_public_resource.go`
  - 可能涉及 `region.go`、`zone.go`、`public_image.go` 的函数签名或调用方式调整。
- Affected systems:
  - cloud-server 自研云周期同步流程
  - data-service `account_sync_detail` 资源同步状态记录
  - 前端账号资源状态展示数据
- APIs/dependencies:
  - 不新增对外 API。
  - 不新增外部依赖。
