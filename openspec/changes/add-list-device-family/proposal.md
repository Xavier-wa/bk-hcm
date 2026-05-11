## Why

资源申请流程中，前端需要按实例族（device_family）对机型进行筛选分组，但当前 woa-server 只提供了机型分类（device_class）的列表接口，缺少实例族列表接口，导致前端无法直接获取去重后的实例族枚举值。

## What Changes

- 在 woa-server 新增 `GET /api/v1/woa/meta/device_family/list` 接口
- 复用已有的 `ListDistinctDeviceType` data-service 接口，在 woa-server 层分页获取数据并对 `device_family` 字段去重后返回
- 新增对应接口文档

## Capabilities

### New Capabilities

- `list-device-family`: 获取 woa_device_type 表中 device_family 字段的去重列表，供前端筛选使用

### Modified Capabilities

## Impact

- `cmd/woa-server/service/meta/meta.go`：新增 `ListDeviceFamily` handler
- `cmd/woa-server/service/meta/service.go`：注册新路由
- `docs/api-docs/web-server/docs/scr/meta/list_device_family.md`：新增接口文档
