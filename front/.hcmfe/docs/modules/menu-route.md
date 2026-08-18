# 菜单与路由

> status: drafted · kind: module
> globs: `src/router/**`、`src/constants/menu-symbol.ts`
> 横切关注点，从 base 拆出。散落的模块路由见各 `views/<模块>/route-config.ts`（不纳入本模块 glob，避免与业务模块重叠）。

## 职责

统一管理应用的一级视图划分、路由注册、菜单结构与页面跳转，并保证「菜单」与「路由」解耦、「功能模块」独立于二者。

## 整体架构

三个一级视图，各对应一组路由集合（在 `views/index.ts` 汇总）：

| 一级视图 | 路由前缀 | 注册数组 | 说明 |
|---|---|---|---|
| 业务资源 | `/business/:bizId/` | `businessViews` | 需要 bizId 路径参数 |
| 工作台 | `/service/` | `serviceViews` | 个人/公共事务 |
| 资源运营 | `/resource/` | `resourceViews` | 资源管理、账单等 |

## 关键文件

- `src/constants/menu-symbol.ts` — 所有菜单/路由的 Symbol 常量（导航一律用 Symbol，禁止硬编码路径字符串）。
- `src/router/index.ts` — 路由实例、全局守卫（`beforeEach` 内含视图级鉴权，见 auth 模块）。`/business/chatbot` 命中平台权限 `agent_assistant`（比 `biz_access` 的 `/^\/business/` 更具体）。无 `agent_assistant` 时：有 `biz_access` 则进通用申请页申请 `agent_assistant`；无 `biz_access` 则先申请业务访问。默认首页分流：`/` → `/business`，由**全局守卫**在 verify 就绪后按 `agent_assistant` 落到 chatbot 或 host；直链 `/business/host` 不拦截。`/business` 节点不能用 `redirect`（匹配阶段鉴权未回）也不能用 `beforeEnter`（站内跳转记录复用会被跳过，停在无组件的 `/business`）。`/403/:id` 仅在该权限确实缺失时停留，已具备时才弹回 `/`。
- `src/router/meta.ts` — 路由 meta 配置类 `Meta`；面包屑用 `layout.breadcrumb.show`。
- `src/router/utils/action.ts` — `routerAction`（跳转唯一入口，见下）。
- `src/router/utils/history-storage.ts` — 自定义历史栈，支撑 `history`/`back` 智能返回。
- `src/router/module/*` — **老路由配置，已废弃**，迁移到各 `views/<模块>/route-config.ts`，待整体删除。
- `views/<模块>/route-config.ts` — 去中心化的模块路由定义（当前仓库 11 处）。
- `common/menu-service.ts` / `components/layout/menu.vue` — 规则描述的目标菜单方案，**当前仓库尚未落地**（还没有该文件）；菜单是否展示应由此独立控制，而非路由 meta。

## 关键约定

### 路由跳转统一走 routerAction

**禁止**直接调用 `router.push/replace/back/go` 或 `useRouter()` 实例方法。一律用 `@/router/utils/action.ts` 的 `routerAction`：

- 普通跳转 `routerAction.redirect(to)`；替换 `redirect(to, { replace: true })`。
- 后退 `routerAction.back()`；新窗口 `routerAction.open(to)`。
- 记入历史供目标页返回 `redirect(to, { history: true })`；跳转后刷新 `{ reload: true }`。

理由：统一封装历史记录管理、`_f` 标志注入、后续可扩展跳转前鉴权/埋点，降低与 vue-router 的耦合。

### route-config.ts 编写要点

- `path` 用相对路径，不以 `/` 开头；`name` 用 Symbol。
- `meta` 用 `...new Meta({ ... })` 展开；`owner` 指向一级视图 Symbol，`activeKey` 指菜单高亮项，`menu.relative` 指面包屑上级。
- **不使用** `notMenu` / `isShowBreadcrumb` / `icon`（均已废弃）；是否入菜单由菜单定义控制，不注册即不展示。

### 多视角入口约定

同一模块需在多个一级视图呈现时，用 `entry-biz.vue` / `entry-rsc.vue` / `entry-srv.vue` 三个入口，分别 `provide('isBusinessPage'|'isResourcePage'|'isServicePage', true)`，子组件 `inject` 感知视角实现差异化。`constants.ts` / `typings.ts` 模块内闭环，跨模块复用应抽到 `common/`、`utils/`。

## 注意事项

- `useWhereAmI`（正则匹配路径判断场景）脆弱、待废弃；嵌套路径（如 `/resource/bill/`）需手动加优先匹配规则。目标是改用路由 meta（如 `owner`）判定场景。
- 改造老模块**不删除老文件**，新建迁移并在老文件加 `@deprecated`。
- 本文档提炼自 `.cursor/rules/fe-menu-route-architecture.mdc`；新建/改造模块的完整 Checklist 以该规则为准。
