## Tasks

### 后端改动

- [x] `cmd/woa-server/types/task/recycler.go` — `GetRecycleHostReq` 结构体字段替换（废弃 `Start/End`，新增 `ReturnStart/ReturnEnd/CreateStart/CreateEnd`）
- [x] `cmd/woa-server/types/task/recycler.go` — 抽取私有方法 `validateRecycleHostDateRangePair`，实现成对传参/日期格式/区间合法性校验
- [x] `cmd/woa-server/types/task/recycler.go` — `Validate()` 接入新的校验逻辑
- [x] `cmd/woa-server/types/task/recycler.go` — `GetFilter()` 删除原 `start/end → create_at` 逻辑，改为 `return_start/end → return_time` + `create_start/end → create_at` 两组 AND 组合
- [x] `docs/api-docs/web-server/docs/biz/scr/list_recycle_device.md` — 更新入参描述
- [x] `docs/api-docs/web-server/docs/scr/resource-recycle/list_recycle_device.md` — 新建管理员视角接口文档
- [x] `cmd/woa-server/types/task/recycler_test.go` — 新建单元测试文件，覆盖筛选逻辑和校验用例

