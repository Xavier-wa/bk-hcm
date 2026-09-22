# 资源运营一级视图（容器·待拆解）

> status: drafted · kind: module
> globs: `src/views/resource/**`
> ⚠️ 这不是原子模块，而是一级视图 `/resource/` 的**菜单容器**（体量最大）——「按菜单笼统归一」的现状刻画。原子原则要打破它。

## 现状（聚合了什么）

- `accountmanage/` — 账号管理 → 应拆为账号管理原子模块。
- `recyclebin-manager/` — 回收站 → 应拆为 recyclebin 原子模块。
- `resource-manage/` — 资源纳管：内部含 host/vpc/subnet/安全组/cvm 等**多种 IaaS 资源类型**（`resource-manage/children/detail/*`），每种资源类型都是一个潜在原子模块。
- 安全组列表页 `resource-manage/children/manage/security-manage.vue` 由业务视角与资源接入共用。三个 tab 都用 `ResourceSearchSelect`（`option-common.ts` 只登记字段和 children）；`type` / `filterRules` 与默认回填由页面挂到 URL `filter` + `useSearchQs`。业务视角安全组 tab 通过 `exclude` 去掉使用业务/管理业务。GCP 防火墙条件不含云厂商，与资源接入一致。
- 安全组克隆弹窗 `resource-manage/children/dialog/clone-security/index.vue` 支持「目标地域」下拉（默认源安全组地域，选项来自 `useRegionStore.getRegionList`，提交透传 `target_region`）；克隆入口仅对 TCLOUD 开放（`plugin/security-group/show-clone.plugin.ts`），成功提示携带目标地域名。
- `NoPermission.tsx` — 旧无权限页（已废弃，见 auth 模块，改用路由守卫 + 权限页）。

## 目标形态

账号管理、回收站、各 IaaS 资源类型各自**打平为原子模块**；资源纳管本身可能保留为「多资源类型」的聚合入口，但各资源类型的字段/列表/详情应走 [model](model.md) 的多云/多资源 Factory 模式。散落对照见 [base](base.md) 的「逻辑特性映射表」与「一级视图容器」表。

## 注意事项

- **只迁不增**：新资源/新功能不要继续无序堆进容器。
- IaaS 资源的列表/详情/表单遵循 [model](model.md) 的字段模型 + 通用组件模式。
- 改造老代码不删老文件，新建迁移并加 `@deprecated`。

## 负载均衡入口与分配弹窗（外部更新）

资源 tab「负载均衡」挂 `src/views/load-balancer/entry-rsc.vue`（双入口壳）；CLB 列表在 `load-balancer-manage.vue`，独占集群列表在负载均衡模块 `exclusive-cluster/`。

资源分配弹窗 `src/views/resource/resource-manage/children/dialog/batch-distribution/`：工具栏「批量分配」读表格勾选；`open(rows)` 用入参行打开同一弹窗（不改勾选）。一条时标题为「{资源名}分配」。独占集群类型 `exclusive_clusters`，body key 为 `cluster_ids`（与现网 store / `assign/bizs` 一致，不是 `exclusive_cluster_ids`）。

- 资源接入各 `children/manage/*-manage.vue` 的 toolbar 在 `isResourcePage` 时取 `justify-content-end`：内容溢出会往**左**溢并被裁掉（先没的是 `toolbar-prefix` 里的 slot 前缀），不是常见的右侧溢出，也无法滚动看到。所以这一行必须留一个可压缩项——右侧搜索框容器给 `min-width: 0`、搜索框 `max-width: 100%` 加可用下限；同时给非搜索的直接子元素 `flex-shrink: 0`，否则收缩量会按 flex-basis 摊到按钮上把文字压掉。注意 slot 内容带的是父组件 scope id，子组件的 `> *` 选不中，需要在各自组件里自管（如 `resource-subtype-switch` 自带 `flex-shrink: 0`）。
- 账号「资源状态」页 `accountInfo/component/resourceStatus/`：表格「资源名称」列直接用 `RESOURCE_TYPES_MAP` 转译 `sync_details` 返回的 `res_name`，后端上新资源类型时前端只需在该 map 补一行（独占集群是 `load_balancer_exclusive_cluster`）。轮询回调**不能**吃 watch 回调的账号形参：轮询只创建一次，账号会被闭包永久固定，而 `resourceAccount` 是 `useResourceAccount` 异步取详情后才写进 store、离开 resource 页前又不会 `clear()`，结果切账号后首次请求对、后续轮询全打到上一个账号。账号只能在 `getList` 内现取；轮询用 `useTimeoutPoll`，watch 里 `reset()` + `resume()` 重置轮次。
- 资源侧 CLB 同步复用业务侧同一套 `use-clb-sync-feedback`（成功 Toast、空 id、`2000002`、跳任务详情）。
- 跳转进的是业务任务管理详情；`bizs` 取最近一次业务选择，没有单独的资源任务入口。
