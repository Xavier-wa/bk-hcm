## Why

离线任务 `apply_recommend_offline` 从历史已交付设备聚合推荐数据时，未过滤当前已不支持的主机申请项目类型（`require_type`）。创建推荐数据会走 `RequireType.Validate()`，历史脏值（已下线类型、0 等）导致整批创建失败；写入又是「先删后建、任一分组失败即整轮中止」，结果是失败分组推荐被删空、后续分组停更。需要在聚合前把不在代码枚举内的记录剔掉，消除这一类已知触发源。

## What Changes

- **F-001**：离线聚合生成推荐数据前，过滤掉 `require_type` 不在后端代码枚举内的历史设备记录。判定口径 Q1=A：受支持集合为 1/2/3/6/7/8/9（常规 / 春节保障 / 机房裁撤 / 滚服 / 小额绿通 / 春保资源池 / 短租），与创建推荐数据时的校验同源。
- **F-002**：过滤后无候选的用户/业务分组本轮不写入、不产生占位空行；其历史推荐行按既有过期清理规则处理。
- **F-003**：被过滤记录逐条输出 Warn 日志（含记录 ID 与项目类型取值，带 rid）。
- 覆盖读取推荐表的在线链路（`ApplyRecommendTop`、`ApplyRecommendByStatic`）的结果正确性验证（不改接口契约）。

本期不包含（与需求文档逐条一致）：

- 不做离线任务的写入容错与写入保护（Q3=A：只做过滤；先删后建、任一分组失败即整轮中止的现状另立需求）。
- 不过滤滚服项目 `require_type=6`（Q2=A：滚服在受支持集合内；写入侧与读取侧口径差异已知且本期不处理）。
- 不清理 `ziyan_cvm_device_info` 源表中项目类型已不支持的历史记录。
- 不改在线读取侧的过滤逻辑，只做写入侧源头治理。
- 不做项目类型的替换/降级兜底。
- 不以运营配置作为判定口径（Q1=A 已选定代码枚举）。
- 不改动推荐算法本身（Top-K 排序、去重键、用户/业务补足策略均不变）。
- 不新增或调整表结构、不调整对外接口契约。
- `ApplyRecommendByPlan` 链路：候选来自实时预测余量而非推荐表，不受本需求影响。

## Capabilities

### New Capabilities

（无。本需求是既有 `apply-recommend` 能力的增量过滤，不引入新 capability。）

### Modified Capabilities

- `apply-recommend`: 离线聚合前增加「不支持的项目类型」过滤；空分组不占位；被过滤记录逐条 Warn。不改读取侧、不改算法、不改表结构/接口。

## Impact

- Affected specs: `apply-recommend`
- Affected layer: 仅 woa-server 离线聚合逻辑（Service Layer），不改 data-service / 在线接口 / 前端。
- Affected code（预期，实现阶段核实）:
  - `cmd/woa-server/logics/applyrecommend/logics.go`：`collectCounts` 聚合前过滤循环
  - 判定复用 `pkg/criteria/enumor/woa_ziyan.go` 的 `RequireType.Validate()`（不改枚举本身）
- 不改：表结构、对外接口、读取侧 `recommend.go`、`ApplyRecommendByPlan`、源表数据、推荐算法参数。
