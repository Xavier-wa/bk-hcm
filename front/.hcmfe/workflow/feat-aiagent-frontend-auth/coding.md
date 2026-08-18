# Coding — feat-aiagent-frontend-auth

> 依据：`prd.md` / `design.md` / `api.md`  
> 范围：仅 `front/`；不改后端。  
> design §3.y：无新 page-/comp- 入口；改现网 `checkAuth`、路由守卫、通用 403 申请页、浮窗显隐。

## 执行顺序

1. 预鉴权项：`id` 改为 IAM Action `agent_assistant`，并把 chatbot 路径挂上去；去掉入口对 `biz_agent_assistant` 的匹配  
2. 菜单 / 默认落地 / 冷启动落地：改读 `agent_assistant`  
3. 直访无权限去向：有业务访问 → 通用申请页申请 `agent_assistant`；无业务访问 → 先申请 `biz_access`  
4. 浮窗：改读 `agent_assistant`，无权限不渲染  

## 单据 1: 【aiagent-权限点调整】前端鉴权逻辑调整

**TAPD**: [#137216875](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137216875)  
**文件**: `src/store/common.ts` `src/views/chatbot/route-config.ts` `src/router/module/business.ts` `src/router/index.ts` `src/components/ai-assistant/index.vue` `src/views/error-pages/403.tsx`  
**改动点**: 业务首页菜单、chatbot 路由、默认落地、浮窗的能力门从 `biz_agent_assistant` 改为 IAM `agent_assistant`；直访复用现网 403 通用申请页（`params.id` 与 IAM Action ID 对齐）。

## 方案

前端入口统一用 IAM Action ID **`agent_assistant`**（「平台-智能体助手」），不再用本地别名 `chatbot_access`。这样 `permissionAction`、`checkAuth`、`403/:id`、`urlParams` 共用同一个 key。

现网 `pageAuthData`：

- `{ type: 'agent_assistant', action: 'agent_assistant', id: 'chatbot_access' }`（无 path）
- `{ type: 'agent_assistant', action: 'find', id: 'biz_agent_assistant', path: /^\/business\/chatbot/, bk_biz_id: 0 }`

守卫按 path 最具体项取 `currentFindAuthData`，chatbot 直访目前命中旧业务权限。本单把平台项改成 `id: 'agent_assistant'` 并挂上 path，同时删除 `biz_agent_assistant`。

### 1. `src/store/common.ts`

```ts
{ type: 'agent_assistant', action: 'agent_assistant', id: 'agent_assistant', path: /^\/business\/chatbot/ }
```

- 删除 `id: 'chatbot_access'` 与 `biz_agent_assistant` 整项。
- 不带 `bk_biz_id`（平台权限，auth-server `BizID==0` → IAM `agent_assistant`）。

### 2. `src/views/chatbot/route-config.ts`

- `meta.checkAuth`: `'biz_agent_assistant'` → `'agent_assistant'`。

### 3. `src/router/module/business.ts`

- 去掉 `/business` 的同步 `redirect`：redirect 在**路由匹配阶段**执行，此时 verify 还没回，`permissionAction` 为空，必然落到 host。历史代码的「冷启动 `/` 进 chatbot」因此从未生效。
- 也不能改用 `beforeEnter`：站内从 `/business/host` 再进 `/business` 时该记录被复用，vue-router 会跳过 `beforeEnter`，页面停在无组件的 `/business` 上（空白）。
- 该节点现在只作为分组，落地判定统一交给全局守卫。

### 4. `src/router/index.ts`

- 守卫分流条件由「是否冷启动（`from.path === '/'`）」改为「鉴权结果是否就绪」。verify 失败时 store 里的 `authVerifyData` 仍为 `null`，若只在冷启动拉取，之后所有站内跳转都会读到空权限、被判无 `biz_access`，用户到刷新前都出不来。
- 冷启动 verify 用模块级 `authVerifyPromise` 门闩，锁在「请求已发起」而非「结果已就绪」：重定向链上 `from` 仍是 START_LOCATION，会反复进入拉取分支，共用同一个 promise 才不会重复打 verify（AC-P01）。失败时把门闩置回 `null`，下一次导航可重试。
- 失败也要 `next()`：原来只有 `.then()`，verify 一挂导航就永远悬着、页面空白。按 api.md §4「verify 失败按无权限处理」兜底。
- `toCurrentPage` 内新增两步，都在 verify 就绪后执行：
  1. 所有 `/business*` 先校验 `biz_access`。chatbot 的鉴权项比 `biz_access` 更具体，只有平台权限时会被提前放行，必须先拦。
  2. `to.path === '/business'` 按 `agent_assistant` 落地到 chatbot / host。
  两步都先对 `to.path` 去尾斜杠：`MENU_BUSINESS_INDEX` 现在既无 `component` 也无 `redirect`，而 vue-router 默认 `strict: false`，`/business/` 能匹配到该记录；不归一化会漏过落地分流，放行到一个没有组件的路由上导致白屏。
- 无 `agent_assistant` 时：
  - 有 `biz_access` → `403` + `params.id = agent_assistant`
  - 无 `biz_access` → `403` + `params.id = biz_access`
- 修正老逻辑「进 403 且有 `biz_access` 就弹回 `/`」：该逻辑会把 AC-003 的平台权限申请页也顶掉。改为只在 `params.id` 对应权限**其实已具备**（失效链接）时才弹回，并补上 `query` 透传（原来是唯一丢 `bizs` 的重定向）。
  ⚠️ 该老 bug 的影响面大于本需求：旧逻辑会让 `/403/account_find`、`/403/rolling_server_manage` 等**所有**权限点的申请页对有 `biz_access` 的用户打不开，修复后它们都会正常停留，测试需覆盖。

### 5. `src/components/ai-assistant/index.vue`

- 改读 `permissionAction.agent_assistant`；无权限仍不渲染，不弹申请。

### 6. `src/views/error-pages/403.tsx`

- 现网 chatbot 直访仍走 `/403/:id`（pageAuthData 路径鉴权，不是新 `auth.view`）。
- 补 `urlKey === 'agent_assistant'` 的权限申请说明与功能说明。
- 底部操作区按 `urlKey` 分流：`agent_assistant` 属主动授权，不走 IAM 自助申请，隐藏「申请权限」按钮，改用 `@/components/w-name` 渲染「联系管理员申请」，点击拉起企微（账号复用 `views/chatbot/constants.ts` 的 `ASSISTANT_CONTACT.name`）。其余 `urlKey` 维持原「申请权限」按钮 + `urlParams[urlKey]`。

## 不改

- 会话 / AG-UI / history / cancel 接口
- 通用申请页框架本身（仍走现网 `/403/:id`）
- 已废弃的 `views/home/index.tsx`
- 后端 IAM / 会话鉴权

## 验收对照

| AC | 实现落点 |
|----|----------|
| AC-001 | 菜单 `checkAuth` + 浮窗 + 守卫放行均认 `agent_assistant` |
| AC-002 | 无 `agent_assistant` 时菜单/浮窗不展示 |
| AC-003 | 直访 chatbot 且有 `biz_access` → 403/`agent_assistant` |
| AC-004 | 直访且无 `biz_access` → 403/`biz_access` |
| AC-005 | 不再校验 `biz_agent_assistant`，只持有旧权限视为无平台权限 |
| AC-P01 | 仍走启动时一次 `auth/verify`，不新增请求 |

## 实现记录

已按上表改完入口鉴权，并在现网仍在用的 `403.tsx` 补了 `agent_assistant` 的申请说明与功能说明（申请按钮仍走 `urlParams.agent_assistant`）。lint 已过。

## 遗留问题（本次不改，另开单）

**`biz_access` 是「任意业务」语义，AC-004 在「对当前业务无权限」场景下不生效。**

现象：对业务 213 没有访问权限的账号，打开 `/#/business/host?bizs=213` 仍能进入主机列表页，不会跳 `403/biz_access`。

链路：

1. `store/common.ts` 的 `{ type: 'biz', action: 'access', id: 'biz_access' }` 没有 `bk_biz_id` 字段；
2. `hooks/useVerify.ts` 只对带 `bk_biz_id` 的条目下发业务 id（`if (v.bk_biz_id)`），`home/index.tsx` 的注入同样按 `hasOwnProperty('bk_biz_id')` 判断；
3. `cmd/web-server/service/auth/auth.go` 中 `BizID > 0` 才走 `Authorize`（精确），否则走 `AuthorizeAny`；
4. 结果：只要对**任意**业务有访问权限，`biz_access` 恒为 `true`，守卫据此放行。

历史成因：`biz_access` 于 2023-04-03（MR !545）加入时，`useVerify` 尚不支持下发业务 id；该能力 2023-04-05（MR !555）才补上，同期给 `cvm` 系列条目加了 `bk_biz_id: 0`，唯独漏了 `biz_access`。2024-07-01（MR !1176）改这行时只补了 `path`。

修复方向：给该条目加 `bk_biz_id: 0`，冷启动由路由守卫从 URL / localStorage 解析注入，切业务由 `home/index.tsx` 的 watcher 注入。

风险：会把所有 `/business*` 页面的准入从「有任意业务权限」收紧为「有当前业务权限」，影响面超出本需求。另外切换业务时 verify 是异步的，`business-selector` 的 `router.push` 会先用旧权限放行一次，下一次导航才拦得住，需要一并处理时序。
