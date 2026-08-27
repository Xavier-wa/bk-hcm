# 操作记录

> status: drafted · kind: module
> globs: `src/views/operation-log/**`

操作审计日志。目标原子模块的正面样板：功能自洽、菜单挂载与模块归属解耦。

## 职责

（待补充：本模块的核心职责、边界、关键文件与对外接口。）

## 关键流程 / 注意事项

- 列表筛选「资源名称」`res_name`：`children/search/condition.ts` 的 `filterRules` 调用 `buildFilterRulesWithSearchSelect`（`src/utils/search.ts`）。多关键词拆成外层 `or` + 多条 `cs` 单字符串。入口 `entry-biz.vue` / `entry-rsc.vue` 直接 `transformSimpleCondition`，不再二次摊平。
