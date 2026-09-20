# 资源运营一级视图（容器·待拆解）

> status: stub · kind: module
> globs: `src/views/resource/**`

一级视图 /resource/ 容器，聚合账号/回收站/资源纳管，待拆解。

## 职责

一级视图 `/resource/` 容器。资源 tab「负载均衡」挂 `src/views/load-balancer/entry-rsc.vue`（双入口壳）；CLB 列表在 `load-balancer-manage.vue`，独占集群列表在负载均衡模块 `exclusive-cluster/`。

资源分配弹窗 `src/views/resource/resource-manage/children/dialog/batch-distribution/`：工具栏「批量分配」读表格勾选；`open(rows)` 用入参行打开同一弹窗（不改勾选）。一条时标题为「{资源名}分配」。独占集群类型 `exclusive_clusters`，body key `exclusive_cluster_ids`。

## 关键流程 / 注意事项

- 资源侧 CLB 同步复用业务侧同一套 `use-clb-sync-feedback`（成功 Toast、空 id、`2000002`、跳任务详情）。
- 跳转进的是业务任务管理详情；`bizs` 取最近一次业务选择，没有单独的资源任务入口。

## 明细目录

<!-- bkdevbuddy:toc:start -->
<!-- bkdevbuddy:toc:end -->

（上方标记块由 `bkdevbuddy docs deepen` 按 `modules/<id>/` 目录自动生成，**请勿手工编辑**；拆出 topic 后 deepen 一次即出现链接。）

## 明细约定

优先在本文件用章节深化。**本轮只碰模块的一块、且本文件已有实质章节**时，优先拆 `modules/<id>/<topic>.md`（并行改同一模块时能少撞正文）；某块稳定超 80–120 行是兜底阈值。topic 文件首行写 `# 标题`、紧随一行摘要（TOC 据此渲染）。拆出后勿复制正文。
