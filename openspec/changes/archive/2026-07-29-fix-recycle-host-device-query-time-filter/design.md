# 主机回收设备查询时间筛选修复 — 后端设计

## 1. 核心改造：`GetRecycleHostReq`（必做）

### 1.1 结构体字段变更

文件：`cmd/woa-server/types/task/recycler.go`

```diff
 type GetRecycleHostReq struct {
-    Start string `json:"start"`
-    End   string `json:"end"`
+    ReturnStart string `json:"return_start"`
+    ReturnEnd   string `json:"return_end"`
+    CreateStart string `json:"create_start"`
+    CreateEnd   string `json:"create_end"`
     // ... 其他字段保持不变
 }
```

### 1.2 Validate() 校验规则

| 校验项 | 规则 |
|--------|------|
| 成对传参 | `return_start`/`return_end` 必须同时有或同时无 |
| 成对传参 | `create_start`/`create_end` 必须同时有或同时无 |
| 日期格式 | 四个字段均为 `YYYY-MM-DD` |
| 区间合法 | `return_start <= return_end`，`create_start <= create_end` |

建议抽取私有方法 `validateRecycleHostDateRangePair(start, end, fieldName string) error`，避免重复逻辑。

### 1.3 GetFilter() 筛选逻辑

| 参数组 | 筛选字段 | 条件 |
|--------|---------|------|
| `return_start` + `return_end` | `return_time`（string） | `>= return_start` 且 `< return_end+1天` |
| `create_start` + `create_end` | `create_at`（time.Time） | `>= create_start 00:00:00` 且 `< create_end+1天 00:00:00` |

**要点**：
- 删除原 `start/end → create_at` 的逻辑。
- `return_time` 是 string 类型，边界用 `"YYYY-MM-DD"` 字符串比较（与现有格式 `2006-01-02 15:04:05` 兼容）。
- 两组条件 AND 组合。

## 2. 无需改动的层（确认即可）

| 文件 | 原因 |
|------|------|
| `cmd/woa-server/service/task/recycler.go` | Handler 只做 Decode + Validate + 调 logic，无筛选逻辑 |
| `cmd/woa-server/logics/task/recycler/recycler.go` — `GetRecycleHost()` | 直接调 `param.GetFilter()`，filter 改完即生效 |
| `cmd/woa-server/dal/task/dao/recycle_host.go` | DAO 通用 filter 查询，无需改 |
| `cmd/woa-server/dal/task/table/recycle_host.go` | `return_time` 字段已存在，模型无需改 |

## 3. 「完成时间不展示」排查结论 + 可选修复

### 3.1 排查结论（非筛选 bug，是数据写入缺口）

| 原因 | 说明 |
|------|------|
| 正常空值 | 设备未完成退回，`return_time` 为空，前端 `timeFormatter` 显示 `--` |
| 数据缺口 | 部分路径 stage 已到 DONE，但未写 `return_time` |

**未写 `return_time` 的路径**：

| 文件 | 场景 |
|------|------|
| `logics/task/recycler/transit/transit.go` — `UpdateHostInfo()` | 中转完成（转池/转 CR 模块），只更新 stage/status/update_at |
| `logics/task/recycler/dispatcher/transiting_state.go` | ResourceTypeOthers、常规 PM 中转成功后 order 直接 DONE |
| `logics/task/recycler/returner/cvm.go` — `updateReturnSuccess()` | 先改 stage=DONE，`return_time` 靠后续轮询 `updateCvmHostInfo()` 回填 |

**已写 `return_time` 的路径**：

| 文件 | 来源 |
|------|------|
| `returner/cvm.go` — `updateCvmHostInfo()` | 云 API `detail.FinishTime` |
| `returner/pm.go` — `updatePmHostInfo()` | ERP 状态=7 时写 now |

### 3.2 可选修复（若产品要求「已完成必展示完成时间」）

| 文件 | 改动 |
|------|------|
| `transit/transit.go` — `UpdateHostInfo()` | stage=DONE 时补写 `return_time = now.Format("2006-01-02 15:04:05")` |
| `dispatcher/transiting_state.go` | 同步更新 host 表 stage=DONE 时补写 `return_time`（当前只更新了 order） |
| 历史数据 | 脚本：stage=DONE 且 `return_time` 为空 → 用 `update_at` 回填（需产品确认） |

**决策**：若本次只做筛选改造，3.2 可单独立项；排查结论写进需求回复即可。

## 4. 接口文档（必做）

| 文件 | 改动 |
|------|------|
| `docs/api-docs/web-server/docs/biz/scr/list_recycle_device.md` | 废弃 `start/end`，新增 4 个时间字段及成对传参说明 |
| `docs/api-docs/web-server/docs/scr/resource-recycle/list_recycle_device.md` | 新建，管理员视角（URL: `/api/v1/woa/task/findmany/recycle/host`） |

## 5. 单元测试（建议新增）

文件：`cmd/woa-server/types/task/recycler_test.go`（新建）

| 用例 | 覆盖点 |
|------|--------|
| 仅传 return 一组 | filter 命中 `return_time` |
| 仅传 create 一组 | filter 命中 `create_at` |
| 两组都传 | AND 组合 |
| 只传 `return_start` | Validate 失败 |
| `return_start > return_end` | Validate 失败 |
| 四者均不传 | filter 无时间条件 |
| 日期格式错误 | Validate 失败 |
