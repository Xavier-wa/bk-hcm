## Why

自研云主机申领、申领修改页面的子网下拉只有 ID 和名称，用户无法判断所选子网能否装下本次申领量，常出现提单后才发现 IP 不足。可用 IP 是随开机/退机实时变化的高频数据，MySQL `subnet` 表不存该字段，6 小时一轮的同步快照也无参考价值，必须实时查询，且必须与生产调度 `getCvmSubnet` 使用同一口径（CRP `leftIpNum`），否则「显示有 IP、提单却说不足」比不展示更糟。

## What Changes

- 改造现有接口 `POST /api/v1/woa/config/findmany/config/cvm/subnet`：请求不变，响应每项的 `available_ip_count` 取自 CRP `getRealSubnetInfo.leftIpNum`（方案 A），与调度器同源。
- woa-server logics 新增独立方法 `GetAllSubnetWithAvailIP`，对齐 `left_ip.SyncLeftIP`：先 `GetAllSubnet` 只取清单，再单独调 CRP，按 `cloud_id` 左连接。`GetAllSubnet` 方法体不增加 CRP。`GetSubnetList` 不引入 CRP，仅把赋值改成指针以适配共用结构。
- `NewSubnetOp` 注入的 CRP 客户端由未使用的 `thirdCli.OldCVM` 改为 `thirdCli.CVM`，与调度器 / 实际提单同一地址。
- 将共用结构 `Subnet.AvailableIpCount` 改为 `*uint64`（JSON 不带 `omitempty`）：下拉接口 CRP 命中时写数字（含明确的 0），CRP 失败或未查询时为 `null`，便于前端展示 `-`；单项未命中仍为数字 `0`。下拉出口以 CRP 覆盖，不以 `GetSubnetList` 上可能存在的 TCloud 值为准。
- 配置列表 `POST .../cvm/subnet/list` 不引入 CRP；成功路径 JSON 仍是数字，不把下拉的 `null` 语义扩过去。
- 更新申领侧子网列表 API 文档。

不涉及：前端改动、数据库变更、子网同步逻辑、hc-service / cloud-server / data-service、公有云 `subnets/with/ip_count`、新外部依赖、新接口。

## Capabilities

### New Capabilities

- `ziyan-cvm-subnet-available-ip`: 自研云 CVM 子网下拉接口返回与调度同源的实时可用 IP（CRP `leftIpNum`）。CRP 失败或未查询时字段为 `null`，明确剩余 0 时为数字 `0`；失败不阻断清单返回。

### Modified Capabilities

（无现有 spec 级行为变更）

## Impact

- **服务层（仅 woa-server）**：`cmd/woa-server/logics/config/subnet.go`、`cmd/woa-server/service/config/subnet.go`、`cmd/woa-server/types/config/types.go`。`AvailableIpCount` 改为指针以区分未取到与剩余 0。不改 Access / Resource 层，不改前端。
- **外部依赖**：复用已有 `pkg/thirdparty/cvmapi.QueryRealCvmSubnet`（CRP `/capacity/api` `getRealSubnetInfo`），必须走 `thirdCli.CVM`（`cvm.host`）。下游 CRP / data-service / hc-service 的请求响应契约不变。
- **API**：
  - 下拉 `POST /api/v1/woa/config/findmany/config/cvm/subnet`：`available_ip_count` 改为 CRP `leftIpNum`（`*uint64`）；失败或未查询为 `null`。请求不变。
  - 配置列表 `POST .../cvm/subnet/list`：不引入 CRP；因共用 `Subnet` 结构，赋值改为指针，成功时 JSON 仍是数字。现网前端不读该字段。
  - 调度 / 容量 / `left_ip` 走 `GetAllSubnet`，不读 `AvailableIpCount`，行为不变。
- **文档**：`docs/api-docs/web-server/docs/scr/resource-apply/list_cvm_subnet.md`。
