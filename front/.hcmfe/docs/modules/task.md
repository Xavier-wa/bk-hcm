# 任务管理

> status: stub · kind: module
> globs: `src/views/task/**`

异步任务（cvm/clb 等）列表与详情。

## 职责

异步任务列表与详情（cvm / clb 等）。CLB 同步任务类型枚举 `sync_load_balancer`，列表展示「同步」（`src/views/task/constants.ts`）。

## 关键流程 / 注意事项

- 列表筛「同步」→ 查询 `operations=sync_load_balancer`。
- 账号条件：`account_ids` 的 op 为 `json_overlaps`（不是 `in`），模型在 `src/model/task/search.view.ts`。
- 同步任务详情操作列（`details/children/action-list/fields.ts`）：开始/结束时间、类别（新增/修改/删除）、CLB ID（`cloud_lb_id`）、CLB VIP/域名、任务状态、失败原因。无监听器类字段。
- `sync_load_balancer` 不支持失败重执行（入口禁用）。
- 资源侧 Toast 跳进的是**业务**任务详情，业务 id 取最近一次业务选择（`getBizsId()`）。
