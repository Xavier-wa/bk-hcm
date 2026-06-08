## Context

woa-server 已有 `ListDeviceClass` 接口，通过复用 data-service 的 `ListDistinctDeviceType` 接口分页获取数据并在应用层去重返回机型分类列表。本次变更采用完全相同的模式，新增实例族列表接口。

当前缺少 `device_family` 去重列表接口，前端无法直接枚举所有实例族。

## Goals / Non-Goals

**Goals:**
- 提供 `GET /api/v1/woa/meta/device_family/list` 接口，返回去重的实例族字符串列表
- 复用现有 data-service `ListDistinctDeviceType` 能力，不新增 data-service 代码

**Non-Goals:**
- 不支持过滤条件（需要全量列表）
- 不新增 biz 视角接口
- 不修改 data-service 层任何代码

## Decisions

**决策：在 woa-server 层做应用层去重，不走 DB DISTINCT**

复用现有 `ListDistinctDeviceType` 接口 + 分页循环 + Go map 去重，与 `ListDeviceClass` 实现模式完全对齐。避免引入新的 DAO 方法或 data-service 接口，降低变更范围。

## Risks / Trade-offs

- [风险] `woa_device_type` 表数据量增大时，分页全量扫描性能下降 → 当前表数据量较小，可接受；如未来有需要可在 data-service 层补充 DISTINCT 接口
