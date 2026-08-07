## Why

裁撤主机当前未记录「裁撤截止时间」，运营无法按截止时间维度筛选裁撤主机、也无法快速拿到全部可选的截止时间用于前端筛选项。裁撤系统 `ListDeviceV2` 已返回 `expectAbolishTime`，需要落库并贯穿查询链路，补齐这一运营诉求。

## What Changes

- `recycle_host_info` 表新增 `expect_abolish_time` 字段（字符串，格式 `yyyy-MM-dd`，默认空串）。
- 同步任务从裁撤系统 `DeviceV2.expectAbolishTime` 填充该字段，并纳入变更比对（diff/isChange）。
- data-service 的裁撤主机创建/更新接口与表模型、client 封装贯穿新增字段。
- 新增「查询裁撤截止时间列表」能力：woa-server 提供 `POST /api/v1/woa/dissolve/expect_abolish_time/list`，请求支持可选 `filter` 过滤表达式（如按 `project_id` 过滤），返回按 `expect_abolish_time` 去重并升序排列的全量数组；对应 data-service 新增一个 GROUP BY 去重查询接口（上层禁止直连 DB）。
- 裁撤明细 `POST /dissolve/host/detail/list`、导出明细 `POST /dissolve/host/detail/export/list`、总览 `POST /dissolve/table/list` 三个接口新增 `expect_abolish_times` 数组过滤条件，明细/导出接口响应新增 `expect_abolish_time` 字段。
- 相关接口文档（scr 管理员视角）同步更新，并新增列表接口文档。

## Capabilities

### New Capabilities
<!-- 无新增 capability，均为对现有裁撤能力的扩展 -->

### Modified Capabilities
- `dissolve-host-crud`: `recycle_host_info` 新增 `expect_abolish_time` 字段，data-service 创建/更新接口支持写入；新增按 `expect_abolish_time` GROUP BY 去重升序返回的查询接口。
- `dissolve-host-sync`: 同步 SHALL 从 `DeviceV2.expectAbolishTime` 填充 `expect_abolish_time`，并纳入变更比对。
- `dissolve-host-query`: 明细/导出/总览三接口新增 `expect_abolish_times` 过滤，明细/导出响应新增 `expect_abolish_time`；新增 woa-server 查询裁撤截止时间列表接口。

## Impact

- **SQL**: 新增 `scripts/sql/9999_<date>_resource_dissolve.sql`（SQLVER=9999, HCMVER=v9.9.9）。
- **表模型**: `pkg/dal/table/dissolve/host/host.go`。
- **DS 协议/服务**: `pkg/api/data-service/dissolve/recycle_host.go`、`cmd/data-service/service/dissolve/recycle-host/`（create/update + 新增 group-by 查询）、`pkg/dal/dao/dissolve/host/host.go`。
- **client**: `pkg/client/data-service/tcloud-ziyan/dissolve.go`。
- **woa-server**: `logics/dissolve/host/{host.go,sync.go}`、`logics/dissolve/table/table.go`、`service/dissolve/{service.go,table.go,host.go}`、`types/dissolve/types.go`。
- **文档**: `docs/api-docs/web-server/docs/scr/dissolve/`（更新 3 篇 + 新增 1 篇，版本 v9.9.9）。
- **兼容性**: 字段有默认值、过滤条件为可选，无破坏性变更。
