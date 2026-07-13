# Coding 方案 — 资源预测/申请单详情页返回列表后轮询未停止

- TAPD: https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598135691832
- 分支: feat-ticketdetail-pollreset
- 模式: lite (coding → test → done)

## 一、问题定位

页面 `front/src/views/ticket/children/resource-plan/detail/index.vue`（申请单详情页）在单据状态为 `init` / `auditing` 时开启 30s 定时轮询（`autoFlushTask = useTimeoutPoll(getResultData, 30000)`）。用户从详情页返回单据列表页后，轮询仍在继续触发 `getResultData` 请求。

### 根因（两处叠加）

1. **`use-timeout-poll.ts` 的 `pause()` 不清定时器**
   钩子内 `onScopeDispose(pause)` 在组件卸载时只把 `isActive` 置为 `false`，并未 `clearTimeout` 已挂起的定时器。

2. **`getResultData()` 结尾无条件 `resume()`（关键）**
   `getResultData` 是轮询回调本身，其结尾会根据状态调用 `autoFlushTask.resume()`。当用户离开页面时若正好有一次 `getResultData` 请求在途（await 未结束），组件卸载后该 Promise 继续 resolve，结尾的 `resume()` 会把已被置为 `false` 的 `isActive` 重新置 `true` 并重启定时器 —— **轮询在卸载后被复活**。

> 子组件 `components/resource-plan/applications/detail/ticket-audit/index.vue` 已在 `onUnmounted` 调 `reset()`，但它①仅在 `isTicketAuditDetailShow` 为真时才渲染；②无法拦截「在途请求 resolve 后再 resume」这一时序，因此不足以修复。

## 二、修复方案（治本：改公共 hook `use-timeout-poll.ts`）

> 方案变更记录：初版计划在两个页面各自加 `isUnmounted` 守卫（不动公共 hook）。经与需求方确认，改为**治本方案**：直接在公共 hook 内根治「作用域销毁后仍被在途请求 resume 复活」的问题，让所有 20+ 处调用方统一受益，页面不再各自打补丁。

### 2.1 `src/hooks/use-timeout-poll.ts`（核心修复）

1. 新增 `let isDisposed = false;` 标记作用域是否已销毁。
2. `resume()` 开头加守卫：`isDisposed` 为真直接 `return`（no-op），防止组件卸载后在途请求 resolve 时把轮询复活。
3. 作用域销毁回调由 `onScopeDispose(pause)` 改为：
   ```ts
   onScopeDispose(() => {
     isDisposed = true;
     reset(); // 清定时器 + isActive=false + times 归零
   });
   ```
   即卸载时**彻底清理**（原 `pause()` 只置 `isActive=false`、不清定时器）。

**兼容性**：对所有调用方均为纯改进——正常生命周期内 `isDisposed` 恒为 `false`，`resume/pause/reset` 行为不变；仅在「组件已卸载」后才拦截 resume 并清理残留定时器。不改变正常轮询语义。

### 2.2 申请单详情页 `resource-plan/detail/index.vue`

路由页，返回列表即卸载，卸载场景已由公共 hook 统一兜底，**无需页面改动**（初版加的守卫已全部回退）。

### 2.3 子单详情 `sub-ticket/sub-ticket-detail.vue`（常驻 sideslider）

该组件是常驻 sideslider，**关闭时 `isShow=false` 但组件不卸载**，此场景 hook 的作用域销毁兜底覆盖不到，故仍需页面自身处理「关闭但未卸载」：

1. `getResultData` 轮询判定加 `isShow.value` 守卫：sideslider 关闭后不再 `resume`（拦截关闭瞬间在途请求 resolve 复活轮询）。
2. `handleClose`（sideslider `@hidden`）中调用 `autoFlushTask.reset()`，关闭即停轮询。

> 卸载场景（父详情页返回列表导致本组件一并卸载）由公共 hook 兜底，页面无需再加 `onBeforeUnmount`。
> `open()` 中 `getResultData()` 为异步、先于同步的 `isShow.value = true` 让出，`isShow` 守卫不会误伤正常打开流程。

## 三、影响面 / 兼容性

- 改公共 hook `use-timeout-poll.ts` + 子单详情页；申请单详情页无需改动。
- 正常轮询行为不变（init/auditing 仍会轮询；状态终态仍会 reset）。
- 任意使用该 hook 的组件卸载后，均不再被在途请求复活轮询（治本）。
- 子单 sideslider 关闭（未卸载）后同样停止轮询。

## 四、遗留待确认

- `detail/index.vue` 轮询间隔当前为 `3000`（3s），疑似本地调试残留（原值 `30000`/30s）。此改动非本次修复引入，建议恢复为 `30000` 后再提测。
