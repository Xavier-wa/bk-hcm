# Coding 方案 — 负责人展示 hover 中文名未加载

- TAPD: https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598135774216
- 模式: lite（coding → test → done）

## 1. 问题现象

在表格（如「机房裁撤 / 裁撤明细」等，凡是用 `UserValue` 渲染「负责人 / 维护人」等用户字段的列）中：

- 单元格文本能正确显示 `英文名(中文名)`，例如 `abc(中文名)`；
- 但鼠标 hover 出现的 overflow tooltip 却显示 `abc(--)`，中文名变成 `--`。

## 2. 根因分析

### 2.1 UserValue 的中文名是异步解析的

`front/src/components/display-value/user-value.vue`：

- `displayValue` 依赖 `userStore.userList`，逐个 username 查找：
  - 命中 → `英文名(中文名)`；
  - 未命中 → `英文名(--)`。
- 组件首次渲染时用户数据往往还没拉到，先渲染 `英文名(--)`；
- `watchEffect` 里通过 `CombineRequest` 批量请求用户信息，请求回来后 `userStore.userList` 更新，`displayValue` 响应式地变成 `英文名(中文名)`，单元格文本随之刷新。

所以「单元格文本」是响应式、最终正确的。

### 2.2 表格的原生 show-overflow-tooltip 抓取的是首帧 innerText，且只创建一次

各列表（如 `data-list.vue`、`useTable` 等）在 `bk-table` 上统一开启了 `show-overflow-tooltip`。

bkui-vue 表格单元格（`table/components/table-cell`）的 tooltip 逻辑（见 `node_modules/bkui-vue/lib/table/index.js`）：

- `getContentValue()` 读取 `target.innerText`（单元格 DOM 文本）；
- `resolveOverflowTooltip()` 在组件渲染 / resize 时执行，内部 `defaultContent = getContentValue()` 把当时的 innerText 固化进一个闭包 `content()`；
- 且 `if (bkEllipsisIns === null) { createInstance(...) }` —— tooltip 实例**只创建一次**，之后不再用新的 innerText 更新 content。

因此：首帧渲染时 innerText 还是 `英文名(--)`，被固化为 tooltip 内容；之后 `UserValue` 把单元格文本刷新成 `英文名(中文名)`，但 **tooltip 的缓存内容不再更新**，于是 hover 永远显示 `英文名(--)`。

### 2.3 对照：string-value 为什么没问题

`front/src/components/display-value/string-value.vue` 在需要 tooltip 时使用了 `<bk-overflow-title type="tips">` 包裹文本：

```vue
<bk-overflow-title class="full-width" resizeable type="tips" v-if="display?.showOverflowTooltip">
  {{ displayValue }}
</bk-overflow-title>
```

`bk-overflow-title` 的 tooltip 内容 `contentText` 是**响应式 computed**（默认取默认插槽内容），随 `displayValue` 变化而更新；同时它在内部用 `.text-ov`（`width:100%` 的 reference + `text-overflow:ellipsis`）裁剪文本，单元格根节点自身不再溢出，从而**抑制了表格原生的那一份会缓存的 tooltip**。

`string-value` 由于值是同步的，即使走原生表格 tooltip 也不会出错，所以现网没暴露问题；`UserValue` 因为异步，才暴露了原生 tooltip 缓存首帧的缺陷。

## 3. 修复方案（组件层统一修，用户已确认）

改动文件：`front/src/components/display-value/user-value.vue`（仅此 1 个文件）。

参照 `string-value.vue` 的做法，把「非 appearance 分支」的纯文本渲染改为用 `<bk-overflow-title type="tips">` 包裹，让 hover tooltip 由响应式的 `displayValue` 驱动：

```vue
<template>
  <template v-if="!appearance">
    <bk-overflow-title class="full-width" resizeable type="tips">
      {{ displayValue }}
    </bk-overflow-title>
  </template>
  <component v-else :is="appearanceComps[appearance]" :display-value="displayValue" :value="value" />
</template>
```

要点：

- 只改「无 appearance」的默认展示分支；`wxwork-link` 等 appearance 分支保持不变。
- 不额外传 `content` prop，直接用默认插槽 —— `bk-overflow-title` 的 `contentText` computed 会取插槽内容并保持响应式，`displayValue` 一旦解析到中文名，tooltip 同步更新。
- `.full-width`（`src/style/size.scss` 全局类）+ `resizeable`：与 `string-value` 完全一致，保证在表格单元格里正确裁剪、并抑制表格原生的缓存 tooltip，避免出现两份 tooltip。

## 3.1 为什么不会出现「双 tooltip」（关键论证）

bkui-vue 表格 `show-overflow-tooltip`（默认 `mode:'auto'`）是否弹 tooltip，取决于对单元格根节点 `.cell` 调用 `hasOverflowEllipsis(el)`（`el.offsetWidth < el.scrollWidth`）。`.cell` 的 CSS：

```css
.cell {
  padding: 0 16px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

- 改造前：文本是 `.cell` 的直接文本节点，超宽 → `hasOverflowEllipsis(.cell)=true` → 原生 tooltip 触发（缓存首帧 `(--)`）。
- 改造后：文本被移入 `bk-overflow-title` 内层 `.text-ov`（`width:100%` 容器内裁剪），`.cell` 最宽子元素恰为 100% → `.cell` 不再溢出 → `hasOverflowEllipsis(.cell)=false` → **原生 tooltip 被自动禁用**，仅剩 `bk-overflow-title` 那份响应式 tooltip。

结论：「把文本搬进 `bk-overflow-title`」本身就是抑制原生 tooltip 的手段，二者不会并存。这也是 `string-value.vue` 在同类表格里一直不出现双 tooltip 的原因。

佐证：`cloud-secret/.../secret-detail-sideslider/index.vue` 已给 `UserValue` 传 `:display="{ showOverflowTooltip: true }"`，但当前 `UserValue` 未消费此 prop —— 说明该支持本属既定设计。

补充（既有测量策略，不改动）：`bk-overflow-title` 用 `ResizeObserver` 观察外层容器判定 `isShowTips`，挂载时按首帧内容测一次。负责人列通常多人必然溢出，`(--)` 与中文名两态都溢出，tooltip 均会出现；且内容响应式，弹出时必为解析后的中文名。

## 4. 影响面与风险

- 影响所有使用 `<display-value :property>`（type=user）或直接使用 `UserValue` 的位置（列表页 + 详情页 InfoList + `application-list` 等）。
- `bk-overflow-title` 仅在文本溢出时才弹 tooltip，未溢出时无 tooltip，与原「未溢出不弹」的表现一致，不会平白多出 tooltip。
- 需回归验证的场景：
  1. 列表页负责人列 hover（核心修复点）：中文名正确、无 `(--)`、无双重 tooltip；
  2. 详情页 / InfoList 中 user 字段展示：文本、换行、宽度不因 `full-width` 出现布局异常；
  3. `wxwork-link` appearance（如企微跳转）分支不受影响。
- 不新增依赖，不改动表格组件本身，改动范围最小（单文件、模板局部）。

## 5. 验证方式

- 本地 `bkdevbuddy_lint` 自动修复规范；
- test 阶段产出手测清单（P0：列表页负责人 hover 中文名；P1：详情页 user 字段；P2：企微 appearance）。
