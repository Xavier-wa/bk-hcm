## 1. 表结构与模型

- [x] 1.1 新建 `scripts/sql/9999_<date>_resource_dissolve.sql`，`recycle_host_info` 加列 `expect_abolish_time varchar(32) not null default '' comment '裁撤截止时间,格式yyyy-MM-dd'`，并刷新 `hcm_version` 视图（SQLVER=9999, HCMVER=v9.9.9）
- [x] 1.2 `pkg/dal/table/dissolve/host/host.go`：`RecycleHostColumnDescriptor` 加 `{Column: "expect_abolish_time", NamedC: "expect_abolish_time", Type: enumor.String}`，结构体加 `ExpectAbolishTime *string` 字段及中文注释

## 2. data-service 落库与去重查询

- [x] 2.1 `pkg/api/data-service/dissolve/recycle_host.go`：`RecycleHostCreateReq`、`RecycleHostUpdateData` 加 `ExpectAbolishTime` 字段
- [x] 2.2 `cmd/data-service/service/dissolve/recycle-host/create.go`、`update.go`：映射 `ExpectAbolishTime` 落库
- [x] 2.3 `pkg/dal/dao/dissolve/host/host.go`：新增 DAO 方法执行 `SELECT expect_abolish_time FROM recycle_host_info <where> GROUP BY expect_abolish_time ORDER BY expect_abolish_time ASC`（where 含 `expect_abolish_time != ''` + 传入 filter），返回字符串数组
- [x] 2.4 `pkg/api/data-service/dissolve/recycle_host.go`：定义去重查询接口的 Req/Resp（filter + `expect_abolish_times []string`）与 Validate
- [x] 2.5 `cmd/data-service/service/dissolve/recycle-host/`：新增 handler 注册 `POST /dissolve/recycle_hosts/expect_abolish_time/list`，调用 2.3 DAO

## 3. client 封装

- [x] 3.1 `pkg/client/data-service/tcloud-ziyan/dissolve.go`：`DissolveClient` 新增去重查询接口封装（走 `common.Request`）

## 4. woa-server 同步填充

- [x] 4.1 `cmd/woa-server/logics/dissolve/host/sync.go` `transferDevice`：填充 `ExpectAbolishTime: cvt.ValToPtr(device.ExpectAbolishTime)`
- [x] 4.2 `sync.go` `isChange`：加入 `expect_abolish_time` 比对
- [x] 4.3 `cmd/woa-server/logics/dissolve/host/host.go` `toCreateReqs`、`toUpdateData`：透传 `ExpectAbolishTime`

## 5. woa-server 查询过滤与响应

- [x] 5.1 `cmd/woa-server/types/dissolve/types.go`：`HostDetailListReq`、`HostDetailExportListReq`、`ResDissolveReq` 加 `ExpectAbolishTimes []string`；`HostDetail` 加 `ExpectAbolishTime string`
- [x] 5.2 `cmd/woa-server/logics/dissolve/table/table.go` `buildDetailFilter`、`buildOverviewFilter`：追加 `tools.RuleIn("expect_abolish_time", req.ExpectAbolishTimes)`
- [x] 5.3 `table.go` `toHostDetails`：映射 `ExpectAbolishTime`；`ListExportHostDetail` 组装 `HostDetailListReq` 处透传 `ExpectAbolishTimes`

## 6. woa-server 查询截止时间列表接口

- [x] 6.1 `cmd/woa-server/types/dissolve/types.go`：新增请求 `{filter}` 与响应 `{expect_abolish_times}` 结构及 Validate
- [x] 6.2 `cmd/woa-server/logics/dissolve/table/table.go`（或 host logics）：新增方法，套用 `baseRules`（is_ignore=false + 排除 excludedProjectIDs）+ 可选追加 `req.Filter`，调用 data-service 去重查询接口
- [x] 6.3 `cmd/woa-server/service/dissolve/table.go`：新增 handler，含 scr 视角鉴权（`ServiceResDissolve` + `Find`）
- [x] 6.4 `cmd/woa-server/service/dissolve/service.go`：注册 `POST /dissolve/expect_abolish_time/list`

## 7. 接口文档

- [x] 7.1 更新 `docs/api-docs/web-server/docs/scr/dissolve/list_dissolve_host_detail.md`、`list_export_dissolve_host_detail.md`、`list_dissolve_table.md`：补 `expect_abolish_times` 请求参数与 `expect_abolish_time` 响应字段
- [x] 7.2 新增 `docs/api-docs/web-server/docs/scr/dissolve/list_dissolve_expect_abolish_time.md`（版本 v9.9.9+）

## 8. 验证

- [x] 8.1 `go build ./...` 编译通过，`goimports -w` 格式化受影响文件
- [ ] 8.2 自测：同步回填 → 明细/导出返回并可按 `expect_abolish_times` 过滤 → 列表接口去重升序返回
