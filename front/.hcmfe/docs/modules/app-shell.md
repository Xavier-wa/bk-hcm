# 门户框架/首页/错误页/通知

> status: drafted · kind: module
> globs: `src/views/home/**`, `src/views/error-pages/**`, `src/views/notice/**`, `src/views/index.ts`

应用外壳与通用页面：顶部导航与全局业务选择器、错误页、全局通知、views 根入口。账号管理顶栏左侧菜单由 `src/views/home/hooks/useChangeHeaderTab.ts` 按 header id 切换；`bill` 时用 `views/index.ts` 的 `billViews`（现网 `bill` 插入预付费 route-config）。

## 职责

- `src/views/home/index.tsx`：外壳布局（顶栏 + 侧边菜单 + 内容区）。**注意：`fe-deprecated` 规则把它标为已废弃，但 `app.vue` 至今仍在渲染它**，改动前不要按"废弃文件"对待；规则与现状的出入需另行收敛。
- `src/views/home/business-selector/**`：全局业务选择器，**全局业务上下文（`bizs`）的唯一产生方**。它决定业务视角下所有页面读到的业务 id，因此是跨模块接缝，不是普通展示组件。
- `src/views/error-pages/**`、`src/views/notice/**`：403/404 等异常页与全局通知条。

## 关键流程 / 注意事项

### 全局业务上下文的初始化与传播

选择器在 `onMounted` 里拉取有权限业务列表，然后按固定优先级决定本次生效的业务，写入三处（Vuex `accountStore` / localStorage / URL query）：

1. **页面业务** `bkBizId`（`PAGE_BIZ_KEY`）—— 页面数据自身的业务归属，如单据所属业务；
2. **URL 业务** `bizs`（`GLOBAL_BIZS_KEY`）—— 链接携带的全局业务；
3. **localStorage 业务** —— 上次选择的业务；
4. 都取不到时回落到有权限业务列表首项。

`bkBizId` 排在最前是为了避免外链进入时"顶部业务与页面数据不一致"：外链里的 `bizs` 可能是分享者当时的业务，而 `bkBizId` 是单据的固有身份，二者冲突时必须以后者为准，否则页面按错误业务发请求（历史缺陷 TAPD 1069995598137480239）。

时序上有一个容易踩的坑：选择器的初始化是**异步**的（要等业务列表接口返回），而业务页面组件的 `onBeforeMount` 早于它完成。因此**页面组件不能在挂载早期读全局业务做一致性校验或跳转**——那时读到的往往还是旧值。需要按页面业务发请求的页面，应直接用自己的 `bkBizId`，见 [ziyan-scr](ziyan-scr.md) 与 [base](base.md) 中 `getBusinessApiPath(bizId?)` 的说明。

### 手动切换业务时的页面归宿

`handleChange` 里除了写入新业务，还要处理"当前页面属于旧业务"的情况：

- 资源详情页（路由名含 `BusinessDetail`）→ 回到该资源的列表页；
- 声明了 `bkBizId` 的页面（单据详情等）→ 跳到 `route.meta.activeKey` 对应的列表页，并**从 query 中摘掉 `bkBizId`**（它是原页面数据的身份，不应带进新业务）；
- 其余页面 → 停留在当前页（或 `meta.rootRoutePath`）并触发 `reload`。

### 已移除的版本号兜底

历史上存在 `GLOBAL_BIZS_VERSION` / `bizs_version` 机制：版本号变化时清空缓存业务并**直接 return**，导致该次初始化连 URL 里的 `bizs` 都不再读取，外链业务被静默丢弃。由于版本号是写死的常量、从不更新，这段逻辑长期只在"首次访问"时生效且弊大于利，已整体移除。localStorage 中残留的 `bizs_version` 键无人引用，无需清理。**不要再引入类似的"版本号即清缓存"兜底**；缓存失效应由具体场景显式处理。
