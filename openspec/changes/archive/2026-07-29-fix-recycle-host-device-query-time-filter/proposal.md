## Why

主机回收设备查询页面的时间筛选器前端文案标注为「完成时间」，但后端 `GetRecycleHostReq.GetFilter()` 将时间参数接到了 `create_at` 字段上，而「完成时间」实际对应的是 `RecycleHost.return_time` 字段。同时原接口仅通过 `start/end` 传时间范围，未指明是哪个时间字段，且不支持同时按创建时间和完成时间筛选。需要修正筛选字段并拆分出独立的创建时间/完成时间两组筛选条件。

## What Changes

- `GetRecycleHostReq` 废弃原有 `Start/End string` 字段，新增 `ReturnStart/ReturnEnd/CreateStart/CreateEnd` 四个独立时间字段。
- `Validate()` 增加成对传参校验（`return_start`/`return_end` 必须同有或同无，`create_start`/`create_end` 同理）、日期格式校验、区间合法性校验；抽取私有方法 `validateRecycleHostDateRangePair` 避免重复。
- `GetFilter()` 删除原 `start/end → create_at` 的逻辑，改为两组独立筛选：`return_start/end → return_time`（string 比较）和 `create_start/end → create_at`（time.Time 比较），两组条件 AND 组合。
- 配套更新 woa-server 业务视角和管理员视角接口文档中 `start/end` → `return_start/end/create_start/create_end` 的入参描述。
- 「完成时间不展示」排查结论：`return_time` 字段已存在，部分设备为空是因为未走退回流程（正常空值）或中转路径未写 `return_time`（数据缺口）；本次不做写入修复，排查结论记录在案。

## Capabilities

### New Capabilities

- `recycle-host-device-query-time-filter`: 规范主机回收设备查询接口的时间筛选行为，支持独立的完成时间和创建时间两组筛选条件。

### Modified Capabilities

- `recycle-host-query`: `GetRecycleHostReq` 废弃 `start/end`，新增 `return_start/end/create_start/create_end` 四字段及成对校验；`GetFilter()` 筛选逻辑从单一 `create_at` 拆分为 `return_time` + `create_at` 两组 AND 组合。

## Impact

- **Affected code**:
  - `cmd/woa-server/types/task/recycler.go` — `GetRecycleHostReq` 结构体、`Validate()`、`GetFilter()` 三处改造；新增 `validateRecycleHostDateRangePair` / `buildReturnTimeFilter` / `buildCreateAtTimeFilter` 三个私有 helper。
- **Unchanged layers** (确认无需改动):
  - `cmd/woa-server/service/task/recycler.go` — Handler 只做 Decode + Validate + 调 logic，无筛选逻辑。
  - `cmd/woa-server/logics/task/recycler/recycler.go` — `GetRecycleHost()` 直接调 `param.GetFilter()`，filter 改完即生效。
  - `cmd/woa-server/dal/task/dao/recycle_host.go` — DAO 通用 filter 查询，无需改。
  - `cmd/woa-server/dal/task/table/recycle_host.go` — `return_time` 字段已存在，模型无需改。
- **APIs/dependencies**:
  - 接口 URL 不变，入参字段名变更（`start/end` → `return_start/end` + `create_start/end`），前端需同步适配。
  - 不新增外部依赖。
- **Docs**:
  - `docs/api-docs/web-server/docs/biz/scr/list_recycle_device.md` — 更新入参。
  - `docs/api-docs/web-server/docs/scr/resource-recycle/list_recycle_device.md` — 更新入参。
