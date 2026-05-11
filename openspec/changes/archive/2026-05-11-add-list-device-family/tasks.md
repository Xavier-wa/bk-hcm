## 1. woa-server Handler 实现

- [x] 1.1 在 `cmd/woa-server/service/meta/meta.go` 中新增 `ListDeviceFamily` 方法，分页调用 `ListDistinctDeviceType` 并对 `device_family` 字段去重后返回
- [x] 1.2 在 `cmd/woa-server/service/meta/service.go` 中注册路由 `GET /meta/device_family/list`

## 2. 接口文档

- [x] 2.1 新增接口文档 `docs/api-docs/web-server/docs/scr/meta/list_device_family.md`
