## Context

裁撤主机数据从裁撤系统（caiche）经 woa-server 同步任务拉取，补全业务字段后经 data-service 落库到 `recycle_host_info`。查询侧由 woa-server 的总览/明细/导出接口统一读本地表。裁撤系统 `ListDeviceV2` 返回的 `DeviceV2.ExpectAbolishTime`（`pkg/thirdparty/caiche/caiche_response.go:78`）当前未被使用。

本次需在整条链路补齐「裁撤截止时间」字段，并新增一个按截止时间去重的列表查询接口，同时为三个已有查询接口加入截止时间过滤。

约束：
- 除 data-service 外，其他服务禁止直连 DB；woa-server 的 group by 去重必须走 data-service。
- 表模型的 filter 字段会用 `RecycleHostColumns.ColumnTypes()` 做校验，过滤/返回字段必须先登记到列描述符。
- 字段格式为 `yyyy-MM-dd` 字符串，可能为空串（裁撤系统未给出截止时间时）。

## Goals / Non-Goals

**Goals:**
- `recycle_host_info` 新增 `expect_abolish_time` 字段并贯穿「同步 → 落库 → 查询/返回」。
- 提供按 `filter` 过滤、按 `expect_abolish_time` GROUP BY 去重升序返回的列表接口。
- 明细/导出/总览三接口支持 `expect_abolish_times` 过滤，明细/导出返回该字段。

**Non-Goals:**
- 不做业务视角（biz）接口，仅管理员（scr）视角，与现有 dissolve 接口保持一致。
- 不改变裁撤状态四态/两态转换、CPU 核数统计等既有逻辑。
- 不对 `expect_abolish_time` 做字符串日期格式的强校验（沿用裁撤系统原值，空串按空处理）。

## Decisions

### 1. 字段类型与存储
- DB：`varchar(32) NOT NULL DEFAULT ''`，与同表 `region` 等字符串字段风格一致，避免 NULL 处理复杂度。
- 表模型：`ExpectAbolishTime *string`，与该表其余可空字段（指针）保持一致，配合 `cvt.ValToPtr/PtrToVal`。
- 列描述符登记 `{Column: "expect_abolish_time", Type: enumor.String}`，使其可用于 filter 与字段选择。

### 2. 同步填充与变更比对
- `transferDevice` 增加 `ExpectAbolishTime: cvt.ValToPtr(device.ExpectAbolishTime)`。
- `isChange` 增加 `expect_abolish_time` 比对项，保证裁撤系统更新截止时间后本地能触发 update。
- **理由**：不加入 diff 比对会导致同步后旧记录的截止时间无法更新。

### 3. GROUP BY 去重查询走 data-service 新接口（而非内存去重）
- data-service 新增 `POST /dissolve/recycle_hosts/expect_abolish_time/list`，DAO 执行：
  `SELECT expect_abolish_time FROM recycle_host_info <where> GROUP BY expect_abolish_time ORDER BY expect_abolish_time ASC`，`where` 含 `expect_abolish_time != ''` + 传入 filter。
- woa-server 新增 `POST /api/v1/woa/dissolve/expect_abolish_time/list`，请求 `{filter}`（可选 filter 表达式），构造最终 filter（套用与查询侧一致的 `baseRules`：`is_ignore=false` + 排除 `excludedProjectIDs`；`req.Filter` 非空则以 AND 追加），调用 DS 接口。
- **备选**：复用现有 List 分页拉全量后在 woa 内存去重排序。**否决**：全表扫描 + 全量传输开销大，DB 侧 GROUP BY 更高效且语义清晰。

### 4. 请求/响应契约
- 新接口请求：`{ "filter": filter.Expression }`（可选，透传标准 filter 表达式，可按 `project_id`/`bk_biz_id`/`group_id`/`region`/`abolish_phase`/`expect_abolish_time` 过滤），比 `project_ids` 单一维度更通用。
- 新接口响应：`{ "expect_abolish_times": ["2021-12-31", ...] }`（升序、去重、去空、全量）。
- 三接口过滤参数：`expect_abolish_times []string`，为空则不加过滤；`buildDetailFilter`/`buildOverviewFilter` 追加 `tools.RuleIn("expect_abolish_time", req.ExpectAbolishTimes)`。
- `HostDetail` 增加 `expect_abolish_time` 字段，`toHostDetails` 映射；`ListExportHostDetail` 组装 `HostDetailListReq` 时透传 `ExpectAbolishTimes`。

### 5. 字段贯穿点（落库）
- DS proto `RecycleHostCreateReq` / `RecycleHostUpdateData` 加 `ExpectAbolishTime`；`create.go`/`update.go` 映射；woa `toCreateReqs`/`toUpdateData` 透传。

## Risks / Trade-offs

- [裁撤系统截止时间格式不为 `yyyy-MM-dd`] → 直接透传原值不解析，去重列表按字符串排序；若后续需严格日期序，可在 DS 层用日期解析排序（本次不做）。
- [总览接口加截止时间过滤改变统计口径] → 语义上为「仅统计选定截止时间的主机」，属预期行为；文档中明确说明。
- [老数据 `expect_abolish_time` 为空串] → 去重列表已 `!= ''` 过滤排除；过滤查询传空串时按精确匹配空串处理（RuleIn），符合直觉。

## Migration Plan

1. 执行 SQL 加列（默认空串，无需数据回填）。
2. 发布 data-service（新字段 + 新查询接口）、woa-server（同步填充 + 查询过滤 + 新接口）。
3. 触发一次裁撤主机同步，回填存量记录的 `expect_abolish_time`。
4. 回滚：接口与字段均为增量、可选，回滚代码即可；新列可保留不影响旧逻辑。

## Open Questions

- 无（`filter` 过滤语义、group by 实现方式、明细是否暴露、视角范围均已确认）。
