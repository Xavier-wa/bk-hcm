## Why

预算提报会一次性写入未来 3 年预测，明细会出现当前日历尚未开放的项目类型（如 `2029春节保障`）。HCM 按当前时间滚动计算合法集合，覆盖追加会在入参或落库被拒，预算账与预测账对不齐。本期放宽 **resPlan 校验**：只认已知形态、不卡年份；页面下拉仍按当前时间窗口给选项。

## What Changes

- `ValidateResPlan()` 改为只校已知形态：固定五类 + `YYYY春节保障` / `YYYY机房裁撤`（`YYYY` 为四位数字）。覆盖追加、页面 / 简易提单、DAL、列表筛选共用这一处，不再沿链路堆 skip。
- 给用户选择的接口（`ListObsProject`）继续用 `GetObsProjectMembersForResPlan()`，下拉只出当前时间窗口内的类型。
- 退回计划覆盖追加与 `ListReturnReasonClass` 的项目类型从更严的 `Validate()` 改为 `ValidateResPlan()`。
- dispatch 回包删除对 CRP `ProjectName` 的校验，信任原值；出站仍按字符串透传。
- 非法形态（空值、`2029春保`、`foobar` 等）仍拒绝，不静默改写成窗口内邻近年份。

不包含：按期望到货时间动态算窗口、改 meta 枚举对外展示、任意字符串放行、CRP 侧项目类型主数据改造。若 CRP 自身不认未来年份，按 R-005 停下来重评，不在 HCM 打补丁。

## Capabilities

### New Capabilities

- `overwrite-append-obs-project`：resPlan 项目类型校验按已知形态放宽年份；选择列表仍按当前时间窗口；CRP 回包项目类型信任。

### Modified Capabilities

（无：既有覆盖追加 capability 尚未归档到 `openspec/specs/`。）

## Impact

- 校验中枢：`pkg/criteria/enumor/woa_ziyan.go` 的 `ValidateResPlan()`。
- 退回计划：`ticket_overwrite_append.go`、`ListReturnReasonClassReq` 改调 `ValidateResPlan()`。
- dispatch：`apply_change.go` 删除 `ProjectName.ValidateResPlan()`。
- 选择列表：`GetObsProjectMembersForResPlan()` / `ListObsProject` 不改。
- 接口路径与字段名不变；权限沿用现网覆盖追加。
- 无表结构变更。
