# Coding — feat-resplan-time-quantity-adjust

> 依据：`prd.md` / `design.md` / `api.md`（MR !3290，取代早期的 !3267）。  
> §3.y **无** page/comp 落码入口 → **模式借鉴** dissolve Ediatable 用法，**不** import dissolve 业务文件；本模块自建独立文件。  
> **先确认本方案，再改业务代码。** F-003 不做。

## 执行顺序

1. **P0 提交模型** — 重写 `convertToAdjust`（按 diff → `update` / `delay` / `add`）
2. **P0-A 新调整页** — 新目录落地 Ediatable；路由改挂新页；旧 `mod/` 仅保留薄兼容或删入口
3. **P0-B 反馈与预览** — 黄底 tip + 底部预览 + 提交/移除未修改/取消
4. **P1-A 行操作** — 复制/新增/移除；`add` 混装提交
5. **P1-B 部分延期下线** — 去掉列表「部分延期」；迁移引导 **待产品确认强度**（见 §迁移引导）

---

## 1. `convertToAdjust` 展开

### 1.1 现状（有问题）

```ts
// usePlanStore.convertToAdjust(original, updated)
return {
  demand_id: original.demand_id,
  adjust_type: updated.adjustType,   // 直接透传 UI 枚举 update|delay|none
  demand_source: updated.demand_source,
  original_info: map(original),
  updated_info: map(updated),
  expect_time: adjustType === delay ? updated.expect_time : undefined,
};
```

问题：

- UI 的 `AdjustType`（`update`/`delay`/`none`）被当成接口 `adjust_type` 原样提交；`none` 不应进请求。
- **无法表达组合**：改了时间又改数量时，若 UI 标成 `delay`，配置变更丢失语义；若标成 `update`，旧逻辑仍指望顶层 `expect_time` 只在 delay 时传。
- **无法表达新增**：强制有 `original` + `demand_id`。

### 1.2 目标（按 MR !3290）

提交前对**每一行**做 diff，**推导**接口枚举（与 UI 行状态解耦）：

| 行条件 | 接口 `adjust_type` | 载荷要点 |
|---|---|---|
| `isNew`（新增/复制出的空 ID 行） | `add` | 无 `demand_id`；`demand_source` + `updated_info` |
| 仅 `expect_time` 变，配置字段与基线相同 | `delay` | 仅 `demand_id` + 顶层 `expect_time` |
| 任一配置字段变（含同时改时间） | `update` | `demand_id` + `demand_source` + `original_info` + `updated_info`；时间 diff 写在 `updated_info.expect_time` |
| 无变更 | **跳过** | 不入 `adjusts[]` |

伪代码：

```ts
function convertToAdjust(row, baseline): IAdjust | null {
  if (row.isNew) {
    return { adjust_type: 'add', demand_source: '指标变化', updated_info: map(row) };
  }
  const timeChanged = row.expect_time !== baseline.expect_time;
  const configChanged = !isConfigEqual(row, baseline); // 机型/OS/CPU/内存/云盘/地域等
  if (!timeChanged && !configChanged) return null;
  if (timeChanged && !configChanged) {
    return { demand_id: row.demand_id, adjust_type: 'delay', expect_time: row.expect_time };
  }
  // 仅配置 或 组合
  return {
    demand_id: row.demand_id,
    adjust_type: 'update',
    demand_source: row.demand_source || '指标变化',
    original_info: map(baseline),
    updated_info: map(row),
  };
}
```

`handleSubmit`：`tableData.map(...).filter(Boolean)` → `{ adjusts }`。

存放建议：逻辑可留在 `usePlanStore`，或抽到新模块 `adjust-payload.ts`（优先与页面同目录，便于单测/阅读）。

---

## 2. 「取消 config/time 互斥」指什么？是否直接改原 mod？

### 2.1 互斥不只是列表入口

列表「调整 / 批量调整」只是**进页**；互斥发生在**旧调整流程内部**：

1. **Sideslider `EditPlan`（`add/`）** 顶部 Radio「调整配置 | 调整时间」（`add/type`）——二选一。
2. 选定后字段互锁：`type=time` 时配置字段 `disabled`；`type=config` 时时间字段 `disabled`（`add/basic|cvm|cbs`）。
3. 页内还有「批量延期」按钮 + Dialog，把时间调整拆成另一条路径。
4. 行上 `adjustType` 用于「已改配置的行不能再选去做延期」等互斥。

本期目标：同行可同时改时间与数量 → **整页改为可编辑表**，上述 Radio / 字段互锁 / 批量延期旁路都不再作为主路径。

列表入口「调整」**保留**，只换落地页实现。

### 2.2 不直接在旧 `mod/` 上糊；按 GPU 模块形式重构

同意：`mod` 命名随意，且会变成 Ediatable 大改。建议：

| | 路径 |
|---|---|
| **新实现（主）** | `src/views/resource-plan/cvm/adjust/` |
| **领域 Store** | `src/store/resource-plan/cvm-adjust.ts` |
| **模块路由** | `src/views/resource-plan/route-config.ts` |
| **旧 `mod/`** | 新路由上线后删除，避免双份实现 |

正式 path 改为 `/business/resource-plan/cvm/adjust`，使用具名路由 +
`routerAction`；旧 `/business/service/resource-plan-mod` 仅保留 redirect。
`/service/resource-plan/cvm/mod` 无实际 UI 调用方，连同 `/service/resource-plan/mod`
兼容 redirect 一并移除。

---

## 3–4. P0-B / P1-A

无异议，按 design 做黄底 tip、预览、操作列复制/新增/移除、`add` 提交。

---

## 5. 迁移引导：是否明确需求？怎么做？

**是明确需求**，但不在 Figma 主实现帧里：

| 来源 | 表述 |
|---|---|
| PRD 用户故事 5 / F-004 / AC-006 | 入口下线并有迁移引导 |
| design §3.6 / §6 | 提示「时间与数量可在调整同一张单完成」；稿面无独立样式 → Message/Tooltip 惯例 |

**建议实现（轻量，待你拍板）**：

- **必做**：业务侧更多菜单去掉「部分延期」；切断 `BatchPostponeSideslider` 挂载。
- **迁移引导采用 A**：
  - 去掉菜单项，不加常驻 Banner。
  - 首次进入新调整页展示一次 `Message`（localStorage 标记）：「部分延期已下线，请在本页同时调整到货时间与数量」。
  - **B**：列表页「调整」按钮旁常驻 Tooltip。
  - **C**：你确认「只要下线入口、不要引导文案」→ 改 PRD/design 纪要后按此执行。

---

## 「adjacent 复用」怎么复用？

**不是** `import` dissolve 的 `time-period-block.vue` / 业务组件。

含义：

1. **模式**：看 dissolve 如何组 `Ediatable` + `HeadColumn` + `SelectColumn` / `InputColumn` / `DateTimePickerColumn`。
2. **不直接引用业务文件**：仅使用 `@blueking/ediatable` 包内组件。
3. **本需求文件全部新建**在 `cvm/adjust/` 下（表格、行、payload），包括独立操作列，不跨业务目录引用 dissolve 或旧 `components/ediatable/operation-column.vue`。

---

## 调用链与目标目录结构

### 现状调用链

```text
列表 list/table
  ├─ 「调整」/「批量调整」
  │     → router → views/.../mod（薄包装）
  │           → components/.../mod/index.tsx
  │                 ├─ Table + useModColumn（只读 diff 展示）
  │                 ├─ 行「编辑」→ EditPlan sideslider（add/ + type Radio 互斥）
  │                 ├─ 「批量延期」Dialog
  │                 ├─ convertToAdjust → adjust API
  │                 └─ 修改预览 Panel
  └─ 「部分延期」(biz)
        → BatchPostponeSideslider
```

### 目标调用链

```text
列表 list/table
  ├─ 「调整」/「批量调整」
  │     → /business/resource-plan/cvm/adjust（具名路由）
  │           → views/resource-plan/cvm/adjust/index.vue
  │                 ├─ Ediatable（本模块自建列/行）
  │                 ├─ operation-column（项目通用）或本模块 OperationCell
  │                 ├─ buildAdjustPayload / convertToAdjust（diff → update|delay|add）
  │                 └─ 预览 + 底栏
  └─ 「部分延期」移除（+ 可选迁移引导）
```

### 建议目录

```text
src/views/resource-plan/
  route-config.ts                # 增加 CVM adjust 业务路由
  cvm/
    adjust/
      index.vue                  # 页壳：拉数、提交、预览、底栏
      data-list.vue              # Ediatable 表格与校验编排
      render-row.vue             # 单行各列 + 操作列
      adjust-payload.ts          # diff → adjusts[]
      typings.ts                 # 仅本调整视图使用的行模型

src/store/resource-plan/
  cvm-adjust.ts                  # 调整 API + API 请求/响应类型

src/components/resource-plan/resource-manage/
  list/table/index.tsx           # 跳新具名路由、去部分延期
  mod/                           # 【删除】

src/constants/menu-symbol.ts     # MENU_BUSINESS_RESOURCE_PLAN_CVM_ADJUST
src/router/module/business.ts    # 展开模块路由 + 旧 path redirect
src/router/module/service.ts     # 删除无调用方的 service mod 路由
```

### 与旧 `add/` 关系

- 调整主路径**不再**打开 `EditPlan` sideslider。
- `add/`（含 type Radio）若仍被「新建预测」等场景使用则保留；**不**为了本需求继续在调整流里走互斥 Radio。
- 「取消互斥」= 调整页不再使用该 Radio/字段 disable 机制，不是改列表上的「调整」按钮文案。

---

## 已确认

1. 按 GPU 新组织形式渐进重构 CVM；本次以 adjust 为首个垂直切片。
2. 正式 path 使用 `/business/resource-plan/cvm/adjust`，旧业务 path redirect。
3. 删除无实际调用方的 service mod 路由。
4. 局部类型统一命名 `typings.ts`；GPU 的 `types.ts` 视为例外。

---

## 实施结果（2026-07-30）

### 新实现

- `src/views/resource-plan/cvm/adjust/index.vue`：加载、变更预览、可提交条件、时间窗口校验、提交、离开保护与一次性迁移提示；样式写在 SFC `<style scoped>`，不外链 `index.scss`；依赖统一面包屑，不使用 `DetailHeader`。
- `src/views/resource-plan/cvm/adjust/data-list.vue`：Ediatable 表头、选项数据与按 `row_key` 管理的行校验引用。
- `src/views/resource-plan/cvm/adjust/render-row.vue`：同行编辑项目类型、资源类型、地域、机型、实例数、云盘、到货时间与短租返还时间。
- `src/views/resource-plan/cvm/adjust/operation-column.vue`：本模块独立的复制、新增、移除操作列。
- `src/views/resource-plan/cvm/adjust/adjust-payload.ts`：纯函数 diff，输出 `add | delay | update`；数值字段统一归一化，避免 `"10"` 与 `10` 产生假变更。
- `src/views/resource-plan/cvm/adjust/typings.ts`：仅调整视图使用的行模型与基线类型。
- `src/store/resource-plan/cvm-adjust.ts`：CVM 调整领域 API 与 API 请求/响应类型。

### 路由与入口

- `src/views/resource-plan/route-config.ts`：新增 CVM adjust 模块路由；`notMenu: true` + `layout.breadcrumbs`（新菜单未落地前必须保留 notMenu）。
- `src/constants/menu-symbol.ts`：新增 `MENU_BUSINESS_RESOURCE_PLAN_CVM_ADJUST`。
- `src/router/module/business.ts`：展开新模块路由；旧 `/business/service/resource-plan-mod` 保留 query redirect。
- `src/router/module/service.ts`：删除无调用方的 service mod 路由和兼容 redirect。
- `src/components/resource-plan/resource-manage/list/table/index.tsx`：业务调整入口改为具名路由；移除「部分延期」。
- `src/views/business/resource-plan/detail/index.tsx`：详情调整入口改为具名路由和 `routerAction`。

### 删除旧实现

- `src/components/resource-plan/resource-manage/mod/index.tsx`
- `src/components/resource-plan/resource-manage/mod/index.scss`
- `src/components/resource-plan/resource-manage/mod/useModColumn.tsx`
- `src/views/business/resource-plan/mod/index.tsx`
- `src/views/service/resource-plan/resource-manage/mod/index.tsx`
- `src/components/resource-plan/resource-manage/list/table/components/batch-postpone-sideslider/index.vue`

### 主内容区还原（对稿 `2664:45348`）

| 区块 | 稿面结论 | 落地 |
|---|---|---|
| 内容底色 | `2664:45349` = `#f5f7fa`，表格区无白色卡片 | 去掉表格外层 `Panel`（连带「调整预测需求」标题），`.cvm-adjust-main` 直接 `padding: 24px` |
| 调整预览 `2672:54071` | 白底、`padding 16/24/24`、`gap 12`、上边线 `#dcdee5` + `0 -2px 4px rgba(0,0,0,.08)`；内层浅黄 `#fdf4e8`、`padding 8px 32px`、`radius 2`；标题 14px/700/`#313238` | 独立区块，不再套 `Panel`；标题按稿改为「调整预览」 |
| 预览条目 | 行高 32、`gap 8`、12px；label `#4d4f56` 右对齐 84px；前值 `#313238`；箭头 12px `#979ba5`；后值 12px/700 `#f59500` | 标签按稿改为「CPU调整数 (核)/内存调整数 (GB)/云盘调整数 (GB)」，无变更仍显示「无变动」 |
| 操作区 `2672:53614` | 底色 `#fafbfd`、上边线 `#dcdee5`、`padding 8px 24px`、按钮 `gap 8` | 底色与上边线仅在悬浮吸底态生效 |
| 吸底 | 稿面预览 + 操作区整体贴底 | 底部区块跟随表格区在正常流中，仅 `position: sticky; bottom: 0`；内容不溢出可视区时紧贴表格下方，不强行撑到视口底部 |
| 悬浮态判定 | — | 底部区块后置 1px 哨兵 + `IntersectionObserver`；哨兵不可见即悬浮，给操作区加 `is-floating` |

### 表头对照（对稿 `2664:50066`）

稿面 15 列，顺序与文案如下（`*` = 稿面标红必填）：

| # | 表头 | 必填 | 稿宽 | 对应字段 |
|---|---|---|---|---|
| 1 | 期望到货时间 | * | 188（左固定） | `expect_time` |
| 2 | 项目类型 | * | 129 | `obs_project` |
| 3 | 城市 | * | 129 | `region_id` |
| 4 | 可用区 | | 129 | `zone_id` |
| 5 | 资源类型 | * | 129 | `demand_res_type` |
| 6 | 机型类型 | * | 128 | `device_class` |
| 7 | 机型规格 | * | 166 | `device_type` |
| 8 | 实例数量 | * | 120 | `remained_os` |
| 9 | CPU总核数 (核) | | 128 | `remained_cpu_core` |
| 10 | 内存总量 (GB) | | 128 | `remained_memory` |
| 11 | 云盘类型 | * | 129 | `disk_type` |
| 12 | 云盘容量 /实例 (GB) | * | 143 | `disk_per_size` |
| 13 | 云盘总量 (GB) | | 118 | `remained_disk_size` |
| 14 | 单实例磁盘IO(MB/s) | | 143 | `disk_io` |
| 15 | （空表头） | | 112（右固定） | 操作列 |

落地差异与原因：

- **删除「预测ID」列，且不设左固定列**：稿面 Ediatable 帧宽 2019、直接溢出 1600 画板画满 15 列，没有滚动态、没有固定列阴影，即稿面未表达任何列固定。操作列沿用 `fixed="right"`（改造前既有，也是 ediatable README 的范式，横向 2019px 下保证增删行随时可达）。
- **额外保留「短租退回日期」列**（稿面无此列，放操作列前）：原表单 `components/resource-plan/add/basic/index.tsx` 的 `isShowShortRentalTime` 只在 `obs_project === '短租项目' && resourceType === 'cvm'` 时渲染该字段，且 `required` + 「不能早于期望到货日期」。表格形态无法按行隐藏列，等价实现为**非短租行禁用**、短租行必填并带同一条日期先后校验。表头文案随原表单用「短租退回日期」（`typings/plan.ts` 注释里的「短租返还日期」不是 UI 文案）。
  - 原逻辑里的 `type === AdjustType.none` 分支不迁移——那是已拆除的 config/time 互斥机制。
- **`HeadColumn` 宽度改用 `:width`**：组件内部 `styles` 只读 `props.width`（`min-width` 仅参与拖拽计算），原来全列 `:min-width` 实际都落到默认 120px。
- **表头批量编辑图标不做**：稿面「期望到货时间」表头带批量编辑 icon，属 F-003 表头批量改日期新交互，design 阶段已明确跳过。
- **`云盘类型` / `云盘容量 /实例` 只加稿面红星，不加必填校验**：遵循「UI/UX 对稿、逻辑与原代码一致」——存量行 `disk_type` 可能为空，加硬校验会挡住纯延期提交。

### 表格滚动

- `.bk-ediatable` 自带 `overflow-x: scroll` 且不限高，本身不会产生纵向滚动条。
- 因此移除 `data-list.vue` 外层多余的 `overflow-x: auto`，避免嵌套滚动容器；纵向滚动统一交给页面主内容区 `.view-warp`。
**ediatable 不支持表头吸顶，且纯 CSS 补不上**：README 属性表只有列固定 `fixed: left|right`，`Ediatable` 唯一 prop 是 `theadList`，无 height/maxHeight；DOM 为 `div.bk-ediatable > table > thead`，样式里没有任何 `thead`/`th` 的 `position: sticky`。更关键的是 `.bk-ediatable` 写了 `overflow-x: scroll`，按 CSS Overflow 规范另一轴的 `visible` 会被计算成 `auto`，所以它本身就是滚动容器——`th { position: sticky; top: 0 }` 只会相对它生效，而它高度自适应从不纵向滚动，等于空转。把横向滚动挪到外层同样无解：外层也会变成滚动容器，且 ediatable 靠 `tableOuterRef` 的 scroll/clientWidth 算固定列，挪走会直接废掉固定列。

**已落地：限高内滚 + 表头吸顶（用户确认，且要求底部吸底交互不变）**

- 不用 `calc(100vh - Npx)` 魔数，改成一条 `flex: 0 1 auto` + `min-height: 0` 的压缩链：`.cvm-adjust-page`（`height: 100%` 撑满 `.view-warp`）→ `.cvm-adjust-main` → `.cvm-adjust-data-list` → `.bk-ediatable`。**不撑高、只压缩**，所以行少时表格保持自然高度、底部区块紧跟其后（跟随内容流，与原交互一致）；行多时表格被压到剩余空间内滚动。
- `.bk-ediatable` 本来就是滚动容器，补上高度约束就能纵向滚；`th` 加 `position: sticky; top: 0`。`border-collapse: collapse` 下 sticky 表头的边框会随内容滚走，改用 `inset` 阴影补上下两条线；右固定表头提 `z-index` 保证同时吸顶吸右且压在普通表头之上。
- **吸底交互的等价保持**：页面不再整体滚动，原来靠哨兵 IntersectionObserver 判「悬浮态」会永远为假，操作区的底色与上边线将不再出现。因此把触发条件扩成 `isTableScrollable || isBottomStuck`——`data-list.vue` 用 ResizeObserver 比对 `.bk-ediatable` 的 `scrollHeight/clientHeight` 并 `scrollable-change` 上报。语义不变：**操作区悬于可滚动内容之上时才有底色与上边线**，只是「可滚动内容」从页面换成了表格。哨兵观察器保留，兜住视口过矮导致页面仍会滚动的情况。

未采用的替代方案：
- **B 克隆吸顶表头**：主内容区里额外渲染一份 `thead` 做 sticky，同步 `scrollLeft` 与列宽。保留页面级滚动，但要对付 ediatable 的拖拽改列宽，双份 DOM，成本最高。
- **C 给上游提能力**：推组件库加 `maxHeight` + sticky thead。
- **D 不做**：靠分页/控制行数规避。

### 期望到货时间「可申领周期」提示

对稿 `2664:50066`：`期望到货时间` 单元格右侧有一个描边 ⓘ 图标（取色 `#979BA5`，14px），hover 出提示。文案与新建预测页 `components/resource-plan/add/basic/index.tsx` 一致：「注意：日期落在{年}年{月}月W{周}，需要在{周起}~{周止}之间申领，超过{月止}将无法申领」。

提示同时挂两处：**单元格右侧 ⓘ hover**（选完面板即关，靠它承接）+ **日期面板 footer 插槽**（打开面板时可见当前日期的约束）。文案关键日期按原实现标红（`#ea3636` / 11px），拆成 segments 渲染而非拼接长字符串，避免模板里跨行文本节点被 whitespace condense 插入多余空格。面板 `append-to-body` 会 teleport 到 body，但插槽内容编译在本组件作用域、带的是本组件 scope id，所以 `<style scoped>` 依然命中。

**「点确定才关闭」用 `type="datetime"` 拿到**（第一轮误判为不可做，此处更正）：

确定栏是否渲染由 `isConfirm = !!slots.trigger || type === 'datetime' || type === 'datetimerange' || multiple` 决定。第一轮只盯着 `trigger` 插槽这条路（它会整个替换默认输入框、且 ediatable 不转发），漏了 **`type` 这条路是白送的**：

- `typeValueResolver.datetime` 的 `formatter/parser` 与 `date` **实现完全一致**，值的进出只取决于 `format`。所以 `type="datetime" + format="yyyy-MM-dd"` 对 v-model 零影响。
- `onSelectionModeChange('datetime')` 走 `/^date/` 分支归一到 `'date'`，面板 `currentView` 仍是日历，不会冒出时间列；时间面板只有点「时间」按钮才切。
- `onPick` 里 `state.visible = visible` 被 `if (!isConfirm)` 挡住，**选日期不再关面板**；而 `emitChange()` 在 gate 之外无条件调用，所以选完立刻能拿到值去刷新 footer 提示。关闭只剩「确定」(`onPickSuccess`) 和点外部 (`handleClose`) 两条路，都不丢值。
- `:clearable="false"` 会让确认栏只剩「确定」，`base_confirm` 的「清除」由同一个 `clearable` 控制。
- 代价：`type="datetime"` 会带出一个「时间」切换按钮（渲染在 `confirm` 插槽**之外**，插槽替换不掉）。用 `ext-popover-cls="expect-time-picker-dropdown"` 给弹层挂类名，再用非 scoped 样式块隐藏 `.bk-picker-confirm-time`——面板 teleport 到 body，scoped 选择器够不到，必须走这个组合。

**`confirm` 插槽不是开关**：`confirmSlot = hasConfirm ? { confirm: $slots.confirm } : {}` 只是把插槽透传给 panel，panel 里 `this.confirm ? createVNode(base_confirm, {...}, this.$slots) : ''` —— `base_confirm` 整体渲不渲染仍由 `isConfirm` 决定，`hasConfirm` 全程不参与那个 gate。所以它只能改确认区长什么样，不能让确认区出现。（且 ediatable 只转发 `footer`，`confirm` 也传不进去。）

**已知顺序**：dropdown 子节点是 `[header?, panel, footer?]`，而确认栏在 panel **内部**底部，是 float/clear 布局，跨层级重排不安全。所以视觉顺序为 日历 → 确定栏 → 提示条，提示落在按钮下方。

### 调整预览区（对稿 2672:54072）

- **浅黄底 `#fdf4e8` 是「有变更」的强调态，不是常驻底色**：`is-changed` 由 `previewItems.some(changed)` 驱动，三项全无变动时不上色。
- **箭头换成 bkui `<ArrowsRight />`**：设计稿是 12×12 细线箭头（`#979BA5`），原先用的 `bkhcm-icon-right-shape` 是实心三角，形状不符。
- **bkui 图标不能用 `width`/`height` prop 调尺寸**（踩过坑）：`bkIcon` 把 props 拼成 `"width: ".concat(width, "; height: ...")` 塞进 svg 的 style，传 `width="14"` 生成的是 `width: 14`——**CSS 长度缺单位即非法声明**，被丢弃后回落到内置的 `width: 1em`。所以尺寸只能靠 `font-size` 控。`fill="#979BA5"` 倒是合法能生效，但为统一起见改成 CSS `color`（图标内置 `fill: currentColor`）。期望到货时间那个 ⓘ 之前也踩了这个坑（写了 `width="14"` 实际是 1em），一并修正。
- 其余尺寸对齐设计：容器 `padding: 8px 32px` + `border-radius: 2px`，行高 32px、`gap: 8px`，标签 84px 右对齐 `#4d4f56`，调整前 `#313238`，调整后 12px Bold `#f59500`。标签用 `min-width` 而非设计稿的固定 `width`，避免中文标签在 12px 下溢出。

### 单元格内 ⓘ 提示的位置约定

- **下拉列**：`bk-select` 的箭头 `.angle-down` 是 `right: 4px` 的 20px 方块（左边缘 24px），ⓘ 一律排在它**左侧**——`right: 28px`，输入框 `padding-right: 46px`（28 + 图标 14 + 4）。
- **日期列**：无箭头，ⓘ 直接 `right: 12px`，`padding-right: 32px`。
- **与校验错误图标互斥**：ediatable 的 `.input-error` / `.select-error` 也定位在右侧。互斥**必须用 CSS 兄弟选择器**盯 `.is-error` 类，不能用组件的 `@error` 事件——`SelectColumn.getValue()`（提交时整表校验）只设 `errorMessage`、**不 emit `error`**，走事件会漏掉提交触发的错误态，两个图标叠在一起。
- 为让兄弟选择器成立，`originTip` 的 `v-bk-tooltips` 要挂在**组件本身**（落到组件根元素 `.bk-ediatable-select`）而不是外层 `<td>`，ⓘ 作为其兄弟节点绝对定位；这样指针移到 ⓘ 上会触发组件根的 mouseleave，两个 tooltip 天然互斥。

### 项目类型（obs_project）与现网对齐

现网调整态走的是 `add/basic` + `ObsProjectSelector`（老 mod 页把 `add/index.tsx` 当 sideslider 用，`type=AdjustType.config`）。逐条核对结论：

| 旧行为 | 处理 |
|---|---|
| `filterable` 可搜索 | **不是差异**：ediatable `SelectColumn` 内部写死 `filterable: ""` 传给 `bk-select`，所有下拉列本来就可搜索 |
| `disabled={type === AdjustType.time}` | **不带**：新设计把改时间/改配置合并成一张表，`adjust_type` 由 `convertRowToAdjust` 在提交时推导，页面上不存在"当前是延期态"这个概念 |
| `showShortRentalProject={resourceType !== 'cbs'}` | **不适用**：新页只受理 CVM（`demand_res_type` 规则即 `val === 'CVM'`），条件恒真 |
| `showRollingServerProject={is931Business}` | **带上**：`data-list` 用 `getBizsId() === 931` 算一次下发，行内过滤 |
| `showTips`（改造复用/轻量云徙红字） | **带上**：改用单元格右侧 ⓘ hover，表格里没有红字的纵向空间 |

**滚服项目（产品收简）**：非 931 业务下，存量行与新增行的下拉都**不出现**「滚服项目」，也不把当前值补回选项。其它已下线的项目类型仍补回以保证回显。不再做 tip / 行内校验 / 提交前拦截——只做选项过滤。

### 短租退回日期：现网调整态其实不可见

`isShowShortRentalTime = obs_project === '短租项目' && resourceType === 'cvm' && type === AdjustType.none`——最后一个条件意味着**只在新建态渲染**，而调整态 `type` 恒为 `config`/`time`，所以现网调整时这个字段根本不出现。但 `usePlanStore.convertToAdjust` 的 `mapDetailToAdjustInfo` 里有 `return_plan_time: detail.return_plan_time`，原值仍会原样进 payload。

由此现网存在两个隐性缺陷：调整时把项目类型**改成**短租项目 → 退回日期无处可填，提交空值；**从**短租项目改走 → 旧值残留一并提交。新表按设计稿显式化了这一列，第一个缺陷不复存在（有必填校验）；第二个暂与现网一致（`mapRowToAdjustInfo` 只要有值就带上），未做清空。

### 城市 / 可用区与现网对齐

交互逐条对齐，两处有意偏离：

| 点 | 现网 | 新表 |
| --- | --- | --- |
| 切城市清可用区 | `watch(region_id)` 无条件清空 | 加 `oldRegionId !== undefined` 守卫，只在用户切换时清 |
| `clearable` | 城市可清空（清空后触发必填报错） | 城市不可清空，可用区保留 clearable |
| `popoverOptions.boundary: 'parent'` | 有 | 不传 |
| 下拉 loading | 城市、可用区都有 | 只给可用区加 |

`boundary: 'parent'` 不需要跟：bk-select 的弹层默认 teleport 到 body，表格的 overflow 裁切不到它。

`watch(regions/zones)` 那层「选项晚到时补 name」的兜底也没跟：行数据本身带 `region_name`/`zone_name`，用户改城市时选项早已就位，条件触发不到。

**逐行 onMounted 拉选项会放大 N 倍请求**——现网是单条侧滑表单，`getZones`/`getDeviceTypes` 各发一次；整表 100 行就是各 100 次，且同城市、同规格的行完全重复。修法沿用 `getAvailableTime` 那套：在调整模块自己的 store 里按 key 缓存 **Promise**（缓存 Promise 而非结果，顺带把并发请求合并成一次），不动共享的 `resourcePlan.ts`，避免影响其他页面的数据新鲜度。

loading 只补可用区：城市是首屏拉的、有表格 loading 遮罩盖着；可用区是切完城市现拉的，那段空列表用户直接可见。`SelectColumn` 没有 `loading` prop，但 `$attrs` 会透传到内部 bk-select，直接传即可。

**回填 name 的 watch 必须处理「找不到」分支**。原写法 `if (zone) zone_name = zone.zone_name`，手动清空可用区时 `find` 落空，`zone_id` 空了而 `zone_name` 留着旧值。`zone_name` 不在 `CONFIG_KEYS` 里，diff 与 payload 都不受影响，坏的是单元格的「调整前为」tooltip——它读 `originTip('zone_name')`，判定未变更就不弹，于是格子高亮了却没有原值提示（高亮读的是 `zone_id`）。改成 `zone?.zone_name ?? ''`，与现网 `handleChooseZone` 的 `zone?.zone_name || ''` 一致。

**字典下线的存量可用区跟现网一样清掉**。现网 `watch(zones)` 在选项到达后拿 `zone_id` 去 `find`，找不到就把 `zone_id`/`zone_name` 一并清空。这里在 `loadZones` 成功分支后调 `dropOrphanZone()` 复刻；**只在请求成功后校正**，请求失败时选项为空并不代表存量值失效，否则一次接口抖动就会抹掉用户数据。

副作用是这类行一打开就带「已修改」高亮、且不会被「移除未修改」清掉。这是如实反映——该行提交上去的可用区确实变成了空值，比现网静默丢弃更可见。

### 资源类型（demand_res_type）与 CBS 行

编码期一度把选项砍成只剩 CVM、并在提交前拦截 CBS 行，依据是「后端不受理纯 CBS 的 CVM 调整」。**这条依据不成立，已撤销**：`usePlanStore.convertToAdjust` 的 `mapDetailToAdjustInfo` 里 `demand_res_types` 起手就是 `['CBS']`，只有 `remained_cpu_core > 0 || remained_memory > 0` 才追加 `'CVM'`，走的是同一个 `plans/resources/demands/adjust` 端点，现网对纯 CBS 行照常提交。

现网真正的约束是**不让改**：

```jsx
<bk-radio-group modelValue={props.resourceType} disabled={props.type !== AdjustType.none}>
```

`AdjustType.none` 是新建态，调整态恒为 `config`/`time`，所以这组 radio 在调整页恒禁用。落到整表就是**存量行只读、新增行可选**，两态都对得上。

设计稿（`2664:50069`）把这一列画成带下拉箭头 + 必填星号 + 新增行「请选择」，与「新增行可选」一致；存量行只读会显示为灰态，这点与稿有出入，取现网逻辑。

**CBS 行的字段矩阵**照现网补齐——现网 CBS 时整个「CVM云主机信息」Panel 不渲染（`props.resourceType === 'cvm' ? <Panel/> : ''`），云盘部分另走一套：

| 列 | 现网 CBS 调整态 | 新表 |
| --- | --- | --- |
| 机型类型 / 机型规格 / 实例数量 | 面板不渲染 | `-` |
| CPU总核数 / 内存总量 | 面板不渲染 | `-` |
| 云盘类型 | 可编辑 | 可编辑 |
| 云盘容量/实例 | **不出现** | `-` |
| 云盘总量 | **可直接编辑** | CBS 行可编辑，CVM 行仍只读派生 |
| 项目类型 | 过滤掉短租项目 | 同 |

云盘总量这条是唯一与设计稿冲突的：稿里它是只读派生列（无星号、新增行灰 `0`）。但稿中的 CBS 行是照抄 CVM 行的数字——容量/实例 100、总量 5100、实例数量 `-`，`5100 ÷ 100 = 51` 对不上任何一列——等于没有真正定义 CBS 行，所以这里取现网。

**上表只适用于存量 CBS 行。**整表把现网的新建态与调整态并到了一起，而 CBS 的云盘填法在这两态下是两套：

| | 现网新建态 CBS | 现网调整态 CBS |
| --- | --- | --- |
| 云磁盘容量/块 | 必填 | 不出现 |
| 所需数量（块） | 必填 > 0 | 不出现 |
| 云盘总量 | 只读 = 所需数量 × 容量/块 | 直接填写，必填 |

差异根因是 `总量 = 单块容量 × 数量` 里的「数量」，CBS 没有实例概念：新建时由「所需数量」提供，调整时该字段不出现、只好直接填总量。

新表的分界因此是 `isCbsAdjustRow = isCbsOnly && !is_new`：

- **新增 CBS 行**走新建态那套，块数**复用「实例数量」列**（表里与设计稿都没有「所需数量」列，且该列对 CBS 本就空置），总量继续派生。单元格挂 tooltip「CBS 预测在此填写所需云盘块数」，校验文案切成「所需数量应大于0」。
- **存量 CBS 行**走调整态那套：实例数量与容量/块显示 `-`，总量直填。
- 机型类型/机型规格/CPU/内存两态都是 `-`，因为现网的「CVM云主机信息」Panel 在两态下都不渲染。

块数不会污染提交：`mapRowToAdjustInfo` 只在 `demand_res_types` 含 `'CVM'` 时才写 `info.cvm`，CBS 行解析为 `['CBS']`，`cvm.os` 根本不下发。

配套的两处联动：`syncDeviceDerived` 只对**存量** CBS 行直接 return（新增行仍要派生总量），否则会把用户填的总量按 `os=0` 冲成 0；`watch(demand_res_type)` 切换时清空 `obs_project`，对齐现网 `handleUpdateResourceType`（短租项目在 CBS 下不可选，不清会留下非法值）。

### SelectColumn 清空后显示 "true"（ediatable 的 prop 声明坑）

现象：切换城市自动清空可用区、或手动清空可用区后，单元格显示字符串 `true` 而不是 placeholder。

根因在 Vue 的布尔属性转型规则，不在我们的代码：

```js
// @blueking/ediatable select-column
"modelValue": { type: [Boolean, Number, String, Array] }
```

```js
// @vue/runtime-core resolvePropValue
if (opt[shouldCast]) {
  if (isAbsent && !hasDefault) value = false;
  else if (opt[shouldCastTrue] && (value === '' || value === hyphenate(key))) value = true;
}
```

`shouldCastTrue = stringIndex < 0 || booleanIndex < stringIndex`，这里 Boolean 下标 0、String 下标 2，条件成立。于是**传空串进去会被当成「布尔属性简写」转成 `true`**。实测：

```
传入空串 -> 子组件收到: <i>true / boolean</i>
```

之后 `localValue = true` 一路传到 bk-select，`handleSetSelectedData` 见 `true` 是 truthy 就建了条 selected，`handleGetLabelByValue` 在 optionsMap / listMap / selectedCacheMap 里都查不到，最后 `|| tmpValue` 兜底回原值，`selectedLabel.join(',')` 就成了 `"true"`。

**影响面比报出来的大**：`obs_project`、`region_id`、`zone_id`、`demand_res_type`、`device_class`、`device_type`、`disk_type` 七个下拉都吃这条规则，而 `createEmptyAdjustRow` 把它们全初始化成空串——所以「新增一行」时这些格子本来也全是 `true`。切机型规格联动清空 `device_type` 同理。ediatable 其余列不受影响（`input-column` 的 modelValue 只声明了 `default: ""`，`date-time-picker-column`/`tag-input-column` 没声明 type）。

修法是加一层空值适配，行数据仍存空串，只在下发给组件时把空串换成 `null`：

```ts
const selectModel = (field: SelectField) =>
  computed<string | null>({
    get: () => localData.value[field] || null,
    set: (val) => { (localData.value as Record<SelectField, string>)[field] = val ?? ''; },
  });
```

不能改用 `undefined`：SelectColumn 内部 `watch(modelValue, v => { if (v === void 0) return; ... })` 会直接忽略这次更新，旧标签会残留在格子里。`null` 则能正常走到 `selected = []`（`allowEmptyValues` 默认空数组，不含 null），显示 placeholder。

适配器要在 setup 里建好常量，不能写在模板表达式里，否则每次渲染都会重建 computed。

### 评审期限制接入调整页

调整页此前完全没接 `useDeadlineRestrict`（老 mod 页也没有，它的 `disabledDate` 只在批量延期弹窗里挡过去日期）。缺口是：列表的 `shouldDisableRow` 只锁住 `expect_time >= 起始日` 的行，能进调整页的行仍可把日期**改到**锁定年份里去。现按与新建页一致的口径补上：

- `useDeadlineRestrict()` 放在 `data-list.vue` 取一份再按 props 下发。**不能放 `render-row.vue`**：该 Hook 自带 `onMounted` 拉 `report_deadline`，按行实例化会按行数重复请求。
- `disabled-date` 只挂 `expect_time`，不挂 `短租退回日期`——与新建页一致（那边 `getDisabledDate` / `getReturnDisabledDate` 是分开的）。
- 新建页 `getDisabledDate` 里还有本周/13 周那套周次限制，调整页**没有照搬**：调整页的可申领区间是靠 `getAvailableTime` 在提交前校验 + 面板内提示承接的，两套口径不混。
- 文案原样复用新建页那句（`预算评审期间，不允许提交 {year} 及之后的预测；…「修改需求」入口`）。后半句指向的是**单据详情页**的 `MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY`（复用 add 页、`deadlineEnabled=false`），与本调整页不是同一入口，所以并不矛盾——调整的是预测需求，不是单据。
- 该模块不走 i18n（全模块都是裸中文），故用模板串拼接而非 `t()`。
- 只放在**面板 footer**，没放进单元格 ⓘ：这条提示与所选日期无关，不属于「这个日期落在第几周」的解释；且 ⓘ 只有拉到可申领区间才出现，评审期用户还没选日期时根本看不到。触发点本来就是「打开面板发现日期禁选」，此时 footer 正好可见。

落地要点：

- **按日期缓存**：`store/resource-plan/cvm-adjust.ts` 的 `getAvailableTime` 改为缓存 Promise（失败即剔除）。否则整表逐行 watch 会对同一日期重复发请求；`validateExpectTimes` 也顺带复用缓存。
- **两个 tooltip 不打架**：`originTip('expect_time')`（调整前为…）从 `<td>` 下移到内层包裹 div，ⓘ 作为**兄弟节点**绝对定位在右侧。这样指针移到图标上会触发包裹 div 的 mouseleave，两个 tooltip 天然互斥。
- **footer 不能撑宽面板**：单日期面板宽度由日历格子撑出，`.bk-date-picker-dropdown` 绝对定位取子元素 max-content，长文案会把整个面板顶宽。提示用 `width: 0` 退出固有宽度计算、再 `min-width: 100%` 回填（配 `box-sizing: border-box`），面板宽度就只由日历决定。`white-space: nowrap` 只在快捷侧栏上，不会继承过来。
- **`<template #footer>` 必须常驻，不能挂 `v-if`**（踩过坑）：`bk-date-picker` 里 `hasFooter = computed(() => !!slots.footer)`，而 `slots` 是非响应式的普通对象——这个 computed **没有任何响应式依赖，求值一次后永久缓存**。首帧提示尚未拉回、插槽不存在时 `hasFooter` 会被永久钉死为 `false`，之后无论提示怎么变都不再渲染 footer。改为常驻插槽 + 内层 `v-if` 控制内容；未选日期时按原实现给「请先选择期望到货日期」。
- **与错误图标不冲突**：ediatable 的 `.input-error` 也定位在 `right: 0`，但 `expect_time` 只有「空值」一条规则，空值时不出提示，二者互斥。
- `padding-right: 32px` 加在 `.bk-date-picker-editor` 上（该 class 在原生 `<input>` 上），给图标让位。
- 异步竞态：`await` 回来后比对当前 `expect_time`，过期结果丢弃。

### 变更高亮被输入框白底盖住（修复）

现象：改了「期望到货时间」后单元格高亮 `#fdf4e8` 不出现。`<td>` 上的 `.is-changed` 其实生效了，是被单元格内控件的不透明白底盖住：

| 列类型 | 覆盖来源 | 原本是否可见 |
|---|---|---|
| 日期（`DateTimePickerColumn`） | `.bk-date-picker-editor` 是原生 `<input>`，bkui 只设了 border/padding，项目 `reset.css` 也没清背景 → 吃浏览器 UA 默认白底 | ✗ |
| 数字输入（`InputColumn`） | ediatable `.bk-ediatable-input { background: #fff }` + bkui `.bk-input--text { background-color: white }` | ✗ |
| 下拉（`SelectColumn`） | 默认态 ediatable 一路写了 `background: transparent` / `inherit`；但 `.bk-ediatable-select:hover` 是 `#fafbfd` | 默认 ✓ / hover ✗ |
| 纯文本（`TextPlainColumn`） | 未设背景 | ✓ |

**hover 态同样要保持高亮**（对稿要求）。ediatable 三类可编辑列的 hover 都会刷 `#fafbfd`：`.bk-ediatable-select:hover`、`.bk-ediatable-input .input-box input:hover`、`.bk-ediatable-time-picker .bk-date-picker .bk-date-picker-editor:hover`。

修复：只在 `&.is-changed, &.is-new` 作用域下把这几层置为 `transparent`，特异性（(0,5,0) ~ (0,7,0)）刻意压过上述 hover 规则；**只改 `background-color`，hover 的 `border-color: #a3c5fd` 反馈保留**。错误与禁用态用 `:not()` 排除交回组件自身——注意下拉的禁用类名是 `is-disable`（少一个 d），与日期的 `is-disabled` 不同。非高亮单元格不加任何覆盖，保持组件原样。

## 机型类型 / 机型规格与现网对齐

对照 `add/cvm/index.tsx`：选项源（`getDeviceClasses()` 无参全量）、`filterable`、必填校验、切换类型后清空规格并重拉规格列表、CBS 行不出现，均已一致。写法上现网用 `if (oldVal)` 判真值来决定是否清空 `device_type`，我们用 `oldClass !== undefined`——差别只在「从空值首次选中」，此时 `device_type` 本就是空的，行为等价。现网的 `disabled={type === AdjustType.time}` 不适用，整表已取消时间/配置互斥。

提交载荷确认无缺口：`info.cvm` 不含 `device_class`，与现网真正的提交函数 `usePlanStore.mapDetailToAdjustInfo` 一致（同样只发 `res_mode / device_type / os / cpu_core / memory`，后端按 `device_type` 反推）。`device_class` 在现网只存在于表单内部模型，用于驱动机型规格下拉。它仍留在 `CONFIG_KEYS` 参与 diff——理论上「只改类型不改规格」会算变更但载荷无差异，实际不会发生，因为改类型必然清空规格，必填校验会卡住提交。

机型规格另有两处已修：

- **机型信息提示挤掉了「调整前为」**：原绑定 `deviceTypeTip ? { content: deviceTypeTip } : originTip('device_type')`，CVM 行只要选了规格 `deviceTypeTip` 必然非空，`originTip` 成了走不到的死分支。改为与项目类型、期望到货时间同构——右侧 ⓘ 承接补充说明，单元格 tooltip 留给「调整前为」。文案补回现网首项 `core_type`，整体沿用现网 `所选机型为{core_type}，CPU为{n}核，内存为{n}G[，GPU卡为{n}张]`。顺带把 `.obs-project-cell` 提为共享的 `.select-tip-cell`，两处下拉 ⓘ 用同一套定位。
- **CPU 小数校验缺失**：现网在 `os` 上有第二条规则「请调整实例数为整数」（单机核数为小数的机型乘上实例数后总核数可能不是整数），我们只校验了 `> 0`，能提交出小数总核数。已补同名规则。实现上直接用「单机核数 × 实例数」而非派生出的 `remained_cpu_core`——派生走 watch，change 触发校验时派生值可能还没落库。现网另有一条挂在 `cpu_core` 上的同条件规则，只是多高亮一个字段，表格里该列只读，不重复挂。

两处差异按讨论定档：

1. **`clearable` 保持与现网不同**：现网必填下拉也带 `clearable`，重构统一为「必填不可清空、非必填可清空」。表格形态下错误提示比表单拥挤，不给用户清出一个必然报错的空值。
2. **`loading` 补齐**：`data-list` 并行拉取的四份字典（项目类型/城市/机型类型/云盘类型）共用一个 `optionsLoading` 下发各行；机型规格按行异步拉取，用行内 `isDeviceTypeLoading`。`SelectColumn` 未声明 `loading` prop，但其渲染函数用 `mergeProps(..., $attrs, ...)` 把 `$attrs` 透传给内层 `bk-select`，所以直接绑 `:loading` 有效。字典拉取补了 `try/catch/finally`，错误提示由 http 层统一弹出，这里只保证 loading 收尾。

## 实例数量与现网对齐

`type=number`、`clearable`、`> 0` 与 CPU 整数两条规则均已一致（`InputColumn` 的 `clearable` 默认 `true`、`precision` 默认 `0`，不必显式声明）。现网 `onChange` 里的 `val || 0` 空值归一无需照搬：`normalizeNumber` 已把 `''` 与 `NaN` 收敛到 `0`，清空输入框不会产生假 diff，载荷侧 `Number(row.remained_os) || 0` 结果也相同。

两处有意偏离：

- **`min` 保持 1（现网 0）**：数字微调器探不到 0，用户按不出一个必然报错的值；清空时仍会触发「实例数量应大于0」，最终约束一致。
- **不补单位「台」（现网有 `suffix`）**：设计稿表头就是无单位的「实例数量」，且该列在 CBS 行语义是「块」不是「台」，写死单位会冲突。

修掉一处误导提示：tooltip 原按 `isCbsOnly` 分支，而 `isCbsOnly` 同时覆盖新增与存量 CBS 行——存量 CBS 行这一格渲染的是只读 `-`，hover 却提示「CBS 预测在此填写所需云盘块数」。改为仅新增 CBS 行（`isCbsOnly && !isCbsAdjustRow`）显示该提示，存量行回落到「调整前为」。

## 云盘类型与现网对齐

对照 `add/cbs/index.tsx`：选项 value/label（`disk_type` / `disk_type_name`）、`filterable`、`clearable`、`loading` 均一致。必填按此前结论只保留表头星号、不加硬校验，以免存量数据提交不了。

修掉两处：

- **清空云盘类型时 `disk_type_name` 残留**：原来是 `if (found)` 才写回，与之前修过的 `zone_name` 同类问题，残留旧值会让「调整前为」提示失真。改为 `found?.disk_type_name ?? ''`。现网 `handleUpdateDiskType` 里 `diskType.disk_type_name` 没有可选链，清空时 `diskType` 为 `undefined` 会抛 TypeError，这个写法不照搬。
- **切换云盘类型后未重置 `disk_io`**：现网无条件回落到 15。不同盘型 IO 上限不同（高性能云盘 150 / SSD 260），不重置会留下超限值。已对齐，并把新增行里硬编码的 `disk_io: 15` 一并抽为 `DEFAULT_DISK_IO`。

关联但属于下一列：`disk_io` 的 `disabled={!disk_type}` 与 `max`（`CLOUD_PREMIUM` 150、其余 260）联动、以及 `> 0` 校验，我们目前都没有，留到「单实例磁盘IO」处理。

### 云盘容量列在 CBS 行的语义

现网该字段的 label 按资源类型切换：`isCVM ? '云磁盘容量/实例' : '云磁盘容量/块'`，校验文案同理。表格表头全表共享、无法按行切换，且该差异只影响**新增 CBS 行**（存量 CBS 行这一格是只读 `-`）。处理方式与「实例数量」列一致：表头按设计稿保持 `云盘容量 /实例 (GB)`，新增 CBS 行加单元格 tooltip「CBS 预测在此填写单块云盘容量」。

> 这是「一列两语义」的通用处理约定：CVM/CBS 混排的表格里，凡现网按资源类型切 label 的字段，表头取设计稿措辞，差异用行级 tooltip 承接。目前涉及「实例数量」（台 / 块）与「云盘容量」（每实例 / 每块）两列。

### 云盘容量的逻辑对齐

值的来源已与现网一致：接口不返回单盘容量，`demandDetailToAdjustRow` 用 `Math.floor(remained_disk_size / remained_os)` 反推，同现网 `convertToPlanTicketDemand`。

修掉一处派生依赖过宽：

- **改机型规格会静默改掉云盘总量**。现网是两条独立派生 —— `calcCpuAndMemory` 跟 `[device_type, os]`，`calcDiskSize` 跟 `[disk_num, disk_per_size, os]`，机型规格**不在**云盘总量的依赖里。我们原本合成了一个 `syncDeviceDerived` 跟 `[device_type, remained_os, disk_per_size]`，任一变化就把两条派生全跑一遍。叠加上面的 floor 反推（丢余数），存量行「总量 5105 / 实例 10」反推出 510，用户只改机型规格就会被重算成 5100，并作为变更项提交 —— 现网不会。已拆成 `syncCpuAndMemory` / `syncDiskSize` 两个 watch，依赖与现网逐条对应。

补齐一条校验：

- 现网挂了 `value >= 0`，文案按资源类型切（`云磁盘容量/实例` / `云磁盘容量/块`）。此前定的「这列不加硬校验」针对的是**必填**，`>= 0` 不属于必填语义（空值归一为 0 照样通过），不会卡住存量数据，故补上，并把 `diskPerSizeRef` 纳入 `getValue()` 的收集，使其在提交时也生效。

## 云盘总量与现网对齐

读写分支与现网一致：存量 CBS 行直填（现网「编辑CBS时仅可调整云盘总量」），其余由 `syncDiskSize` 派生只读。

- **空值显示**：原为 `remained_disk_size || '-'`，0 时显示 `-`，与上一轮 CPU/内存统一的口径冲突，也与现网（派生态 `<span>{disk_size} GB</span>`，0 就显示 0）冲突。且该分支只覆盖 CVM 行与**新增** CBS 行，后者的总量是「块数 × 单块容量」的有效派生值，本就不该是 `-`。已改为与 CPU/内存同写法：`|| 0` + `is-empty` 占位色。
- **校验**：现网 `rules` 里没有 `disk_size` 条目，只有 form-item 的 `required`。我们挂的 `Number(val) > 0`（「云盘总量应大于0」）比现网严，但与现网给 `disk_num`（CBS 块数）挂 `> 0` 的意图一致，保留。

### 列级说明用 HeadColumn memo，不占单元格 tooltip

现网用 form-item `description` 承接列说明（云盘总量：CBS「需要的云磁盘总量」/ CVM「所有实例的系统盘，数据盘总容量」）。表格形态下的等价物是 `HeadColumn` 的 `memo` prop —— 表头文字加虚线下划线 + hover tooltip，无需占用单元格 tooltip（那里留给「调整前为」），也不必像项目类型/机型规格那样在单元格里塞 ⓘ 图标。

> 约定补充：**列级**说明（对整列恒成立）走 `HeadColumn memo`；**行级**说明（只对某些行成立，如新增 CBS 行的「块」语义）走单元格 tooltip；**值相关**的补充信息（机型配置、项目类型说明）走单元格右侧 ⓘ。混排表头无法按行切文案时，memo 里把两种资源类型的口径并列写全。

按此把现网 form-item `description` 全量迁完，共三处：

| 列 | memo |
| --- | --- |
| 实例数量 | CVM 为所需实例数（台）；CBS 为需要的云磁盘块数（块） |
| 云盘容量 /实例 (GB) | CVM 为每个实例的云盘容量；CBS 为每块云盘的容量 |
| 云盘总量 (GB) | CVM 为所有实例的系统盘、数据盘总容量；CBS 为需要的云磁盘总量 |
| 单实例磁盘IO(MB/s) | 磁盘IO吞吐需求，无特殊要求填写15；高性能云盘上限150，SSD云硬盘上限260 |

前两列的双语义此前只由新增 CBS 行的单元格 tooltip 承接，存量行与 CVM 行看不到；「单实例磁盘IO」原本整条说明都漏了，而它正是默认值 15 与盘型上限的出处（对应切换云盘类型重置 `disk_io` 的处理）。

## 可用区/机型规格重选后回显 id 而非名称

现象：切换城市清空可用区后重新选一个，单元格显示 `zone_id`；重新开合一次下拉又变回名称。

bk-select 解析回显文案的顺序（`handleGetLabelByValue`）是：

```js
已挂载选项.get(v)?.optionName || list属性映射[v] || 已解析缓存[v] || v
```

最后一档 `|| v` 就是直接把值当文案。而 ediatable 的 SelectColumn 有这么一行：

```js
list: slots.optionRender ? props.list : []
```

**只有检测到 `optionRender` 插槽时才把 `list` 透传给 bk-select**，否则传空数组、改用默认插槽自己渲染 `<Option>`。于是第二档恒空；第三档（`_e`）是从当前 selected 反推的自缓存，值被清空过就一并没了。剩下第一档依赖选项此刻是否挂载，取不到就落到显示 id——重新开合下拉让选项重新注册，computed 重算，文案才回来。

修法是给出 `optionRender` 插槽以恢复 `list` 这一档，与挂载时机无关。代价是选项内的搜索关键字高亮（`.is-keyword`）没了，故插槽里自行套上 `bk-select-option-item` 保留省略号样式。

范围只给**选项列表会在交互中重新拉取**的两列：可用区（随城市）、机型规格（随机型类型）——它们都是「换上游 → 重新请求 → 清空当前值」，恰好同时打掉第二三档。项目类型/城市/机型类型/云盘类型的选项由 `data-list.vue` 一次性加载后不再变，没有暴露条件，故不动，避免无谓地牺牲高亮。

## 短租退回日期与现网对齐

现网这个字段在**调整态根本不渲染**：

```js
const isShowShortRentalTime = computed(() =>
  obs_project === '短租项目' && resourceType === 'cvm' && type === AdjustType.none);
```

三条与关系，`AdjustType.none` 即新建态。调整时字段不出现，但值仍随 payload 提交，用户改不了。我们按设计稿做成常驻列，由「是否短租项目」控制可编辑/只读。现网的 `resourceType === 'cvm'` 条件天然满足——`obsProjectSelectList` 已把短租项目从 CBS 行滤掉，选不中短租即恒只读。

按现网新建态补齐四处：

- **未选到货日期时禁用**：现网 `disabled={!expect_time}`。并入 `readonlyFields.return_plan_time`，灰底随之一致。
- **可选范围下界**：现网 `getReturnDisabledDate` 禁用早于 `max(今天, 期望到货日期)` 的日期。我们原先只有「不早于期望到货日期」的校验兜底——存量行的到货日期若已过期，现网仍要求不早于今天，我们放得过去。已补 `disabled-date`，规则保留作二次兜底。
- **快捷按钮**：面板 footer 补「1/2/3 个月」，与现网 `handleReturnTimeWithMonth` 同口径（1 个月记 30 天，从期望到货日期起算），样式照搬 `.date-range-btns`。
- **「请先选择期望到货日期」提示**：现网写在字段下方，表格没有这个位置；禁用态面板不展开、footer 也提示不到，故回退到单元格 tooltip，仅在没有「调整前为」可显示时占用。

> tooltip 分工再补一条：单元格 tooltip 的**回退位**可以承接「当前不可操作的原因」，前提是与「调整前为」互斥（`originTip` 在无变更时返回 `{ disabled: true }`，据此判断）。

## 无云盘的存量行：整组云盘列置只读

现网调整态的数据先过 `convertToPlanTicketDemand`，其中 `cbs = detail.remained_disk_size > 0 ? {…} : undefined`，而 `add/cbs/index.tsx` 的渲染条件是 `props.planTicketDemand.cbs && (<Panel title="CBS云磁盘信息">…)`。所以**云盘剩余量为 0 的存量预测，调整时整个 CBS 面板不渲染**，云盘类型 / 云盘容量·实例 / 云盘总量 / 单实例磁盘IO 四个字段一起消失。新建态走默认 `cbs: { disk_io: 15, … }`，面板必定渲染。

表格是扁平的、四列恒在，不加处理就能给一个本来没有云盘的预测补上云盘 —— 现网做不到。已按同口径置只读并显示 `-`：

- `hasNoDisk = !is_new && Number(baseline.remained_disk_size) === 0`，判定取 **baseline** 而非当前值，否则用户改动后条件会自己翻转。
- 四列的 `readonlyFields` 与模板分支都接上 `hasNoDisk`，`syncDiskSize` 也提前返回，避免派生把 0 又写一遍。

> CVM 侧有对称的 `cvm = remained_os > 0 ? {…} : undefined`，但 CVM 面板的渲染条件写的是 `resourceType === 'cvm'`，没用上这个 guard，故不镜像。

### ⏳ 待现网确认（联调发现，暂不改）

联调实测两条 CVM 预测均为 `total_disk_size: 0` / `remained_disk_size: 0`，`hasNoDisk` 命中，四列全显示 `-`。暴露两点疑问：

1. 若 `disk_size = 0` 在 CVM 预测里很普遍，这四列会近乎恒为 `-`，观感上像功能坏了。
2. 界面显示「无云盘」，但提交入参仍带原值：`"cbs":{"disk_type":"CLOUD_PREMIUM","disk_io":15,"disk_size":0}`（`mapRowToAdjustInfo` 对 CVM 行恒发 `cbs`，取的是行数据里 API 原值）。**与现网 `mapDetailToAdjustInfo` 行为一致**，即显示与提交不一致这点现网也有，不是本次引入。

需在现网核对的点：拿一条 `remained_disk_size = 0` 的 CVM 预测走「调整 → 编辑」，看侧滑里**有没有「CBS云磁盘信息」面板**。

- 没有面板 → 现状 `-` 只读即为对齐，保持不动。
- 有面板 → 说明 `cbs` 三元的推断有误（例如列表与详情字段口径不同），需要改回可编辑/回显真实值。

顺带可在 F12 里看一眼现网提交的 `cbs`，确认是否也带 `CLOUD_PREMIUM` / `15`。

## 单实例磁盘IO与现网对齐

除上面的 `hasNoDisk` 外，这一列原本只有 `type=number :min=0`，漏了三处：

- **上限**：现网 `max={disk_type === 'CLOUD_PREMIUM' ? 150 : 260}`。已补 `diskIoMax`，常量 `PREMIUM_DISK_TYPE` 落在 `typings.ts`。
- **未选盘型时禁用**：现网 `disabled={!disk_type}` —— IO 上限取决于盘型，没选盘型时无从校验。已并入 `readonlyFields.disk_io`。
- **校验**：现网 `value > 0`（「单实例磁盘IO应大于0」）。已补规则并把 `diskIoRef` 纳入 `getValue()` 收集。该规则比 `>= 0` 严，但能命中它的行恰好就是现网会校验的行（无云盘的行已被 `hasNoDisk` 挡在只读态），无回归。

> 加了 memo 的表头因超过 printWidth 会被 prettier 拆成多行，Vue 的 `whitespace: condense` 只压缩不裁剪，槽内文本会变成 `" 实例数量 "`。此处无害：`.th-cell` 是 `inline-block`，`.title-memo` 的文本处在行首行尾，可折叠空白按 CSS Text 规则被移除，虚线下划线仍贴合文字。若将来把 memo 用在非 `inline-block` 容器里，需重新确认。

## CPU 总核数 / 内存总量与现网对齐

两列结构相同，一并处理。单位写在表头（`CPU总核数 (核)` / `内存总量 (GB)`），现网写在值后（`{n} 核`），设计稿采表头写法。小数校验按上一轮结论只挂在实例数量上。

修掉两处：

- **机型规格被清空后派生值不归零**：`syncDeviceDerived` 原本是 `if (device && !isCbsOnly)` 才写回，改机型类型会连带清空机型规格，此时 `device` 为 `undefined`，CPU/内存就停在上一个机型的旧数上。现网 `calcCpuAndMemory` 用 `perCpuCore = deviceType?.cpu_core || 0` 无条件写回，等于归零。已改为 `(device?.cpu_core ?? 0) * os`。必填校验本就会拦住提交，所以不是数据问题，但展示会误导。
- **无值时显示 `-` 而非 `0`**：设计稿逐格取色确认，CBS 行是 `-`，新增行是 `#C4C6CC` 占位色的 `0`；现网同样渲染 `0 核`。已改为 CBS 行 `-`、其余显示数值（含 0），0 时套 `.is-empty` 占位色。

顺带修掉一处整表性差异：设计稿的表格正文统一 `#313238`（抽样期望到货时间、项目类型、城市、机型规格、实例数量、磁盘IO 与只读的资源类型、云盘总量，含禁用态全部一致），而 ediatable / bkui 默认用次级文字色 `#63656e`。改法是在 `.adjust-cell` 上定色，再让 `.bk-ediatable-text-plain` / `.bk-ediatable-select` / `input` 三类 `color: inherit`，省得逐个组件覆盖；占位符走 `::placeholder`，bkui 默认就是设计稿的 `#C4C6CC`，不受影响。

## 只读态底色统一

设计稿（2664:50066）整表逐列采样，底色只有两个值：`#FAFBFC` 只读、`#FFFFFF` 可编辑，前者与 ediatable 的 `#fafbfd` 是同一色（PNG 取样差 1）。逐列结果也反过来印证了此前按现网对齐的判断：资源类型存量行灰、新增行白；机型类型/机型规格/实例数量仅 CBS 行灰；CPU 总核数与内存总量三种行态全灰。

改前只读有两套并行机制，且都不表达「只读」：

| 机制 | 用在哪 | 灰底来源 |
|---|---|---|
| `:disabled` | 资源类型（存量行）、短租退回日期（非短租） | `.bk-ediatable-select.is-disable` / `.bk-ediatable-time-picker.is-disabled` |
| 换 `TextPlainColumn` | CPU 总核数、内存总量、云盘总量（非存量 CBS）、CBS 行的机型/规格/实例数量/容量 | `.bk-ediatable-text-plain.default-display` |

后者的 `default-display` 触发条件是 `!props.data`，语义是「没传数据的空占位」；我们一直用默认插槽传内容、`data` 恒为 undefined，才碰巧一直灰。一旦改用 `:data` 传值灰底立刻消失。

改法：`readonlyFields` 作为唯一判定源，同时驱动组件 `:disabled` 与 `<td>` 的 `is-readonly` 类；底色只由 td 决定，`TextPlainColumn` 的 `default-display` 灰底显式置透明。td 还要补一层 `background-color: #fff` 基准——ediatable 的 `td` 和 `SelectColumn` 都不画背景（`.bk-ediatable-input` 有 `#fff`、日期是原生 input、右固定列有 `.bk-ediatable-right-fixed-column { background-color: #fff }`），缺这层时只有下拉列会透出页面底色，看起来像灰底。`-` 与派生数值仍由 `TextPlainColumn` 承载——塞进 disabled 的数字输入框只会显示空白。

高亮优先于只读：`is-changed` / `is-new` 写在 `is-readonly` 之后，同优先级下压过灰底。这修掉了一个真实缺陷——CPU 总核数、内存总量、云盘总量是跟机型与实例数量联动的派生列，改完机型后值确实变了，但 `#fafbfd` 盖住了 `#fdf4e8`，看不到变化提示；新增行的绿底同样被盖。为此把控件置透明的名单从 `:not(.is-error, .is-disable)` 放宽为只排除错误态，让高亮行里的 disabled 控件（如新增非短租行的短租退回日期）也透出行底色。

新增行的「云盘总量」设计稿画的是白底可编辑，判定为漏改，仍按只读派生处理：同为派生的 CPU/内存在新增行保持灰底，现网新建态该值也是 `容量/实例 × 实例数量` 派生。存量 CBS 行的云盘总量设计稿是灰底，但设计稿未建模 CBS 行（第 2 行数值是 CVM 行拷贝），按现网保持可编辑白底。

### 关键契约

1. 新增/复制行提交 `adjust_type: add`，不传 `demand_id` / `original_info`。
2. 仅到货时间变化提交 `adjust_type: delay`，只传 `demand_id` / 顶层 `expect_time`。
3. 配置变化或时间与配置组合变化提交 `adjust_type: update`，传 `original_info` / `updated_info`。
4. CVM 行保持现网映射：`demand_res_types = ['CVM', 'CBS']`，同时传 `cvm` 与 `cbs`。
5. 资源类型存量行只读、新增行可选 CVM/CBS，与现网调整态/新建态一致。
6. F-003 自定义日期选择组件未实现；沿用 `DateTimePickerColumn`，并保留原有可申领时间范围校验（提交前拦截 + 单元格 ⓘ 提前提示）。

### 验证

- changed frontend ESLint：通过。
- adjust Vue/SCSS Stylelint：通过。
- `pnpm --dir "front" run build:bcc`：通过；仅保留项目既有资源体积 warning。
- payload 行为矩阵：覆盖 unchanged / delay / update / add、数值字符串归一化、CVM+CBS 映射、短租返还日期与到货时间去重校验。

## 提交前校验：现状核实与整表校验拦截

疑问是「ediatable 的列只在 change/blur/clear 时跑 validator，从头到尾没被碰过的必填项会不会绕过提交」。逐层核实结论如下。

### 现状：机制本身是通的

`@blueking/ediatable` 六个列组件（input / select / date-time-picker / tag-input / text-plain / checkbox）的 `getValue()` 是同一个写法：

```js
getValue() {
  return validator(modelValue.value).then(() => modelValue.value);
}
```

`useValidtor` 的 `validator` 只依赖**传入的当前值**，与「是否交互过」无关；命中失败规则时写 `state.error/message` 并 **reject**，根元素随之带上 `is-error`（下拉是 `is-error` + `is-disable`，注意少一个 d）并渲染右侧错误图标 + tooltip。

因此 `render-row.getValue()` → `data-list.validate()` → `handleSubmit` 这条链**确实能覆盖未触碰的必填项**，方向 2（点击时整表校验）在改动前就已经是现状：`canSubmit` 只看加载失败 / 存在未修改数据 / 无可提交变更，**从不**因填写项校验置灰。

带 `:rules` 的列与 `getValue()` 的 ref 收集也已全覆盖，逐列核对无遗漏：

| 列 | rules | 收集到 `getValue()` |
|---|---|---|
| 期望到货时间 / 项目类型 / 城市 / 资源类型 / 机型类型 / 机型规格 / 实例数量 / 云盘容量·实例 / 云盘总量 / 单实例磁盘IO / 短租退回日期 | 有 | 有 |
| 可用区 | 无（表头即非必填） | 无（无需） |
| 云盘类型 | 无（表头有星号但按前述结论不加硬校验，避免挡住存量行纯延期） | 无（无需） |
| CPU总核数 / 内存总量 | 无（只读派生列） | 无（无需） |

`v-if` 换成 `TextPlainColumn` 的列（CBS 行的机型/规格/实例数量、无云盘行的四列）ref 自然为 null，被 `filter(Boolean)` 跳过，符合预期。

### 真实缺口一：ediatable 的 `rules` 是非响应式的（已修）

```js
// select/input/date-time-picker 三个组件都是这一行
const { message: errorMessage, validator } = useValidtor(props.rules);
```

`props.rules` **只在 setup 时读一次**，`validator` 闭包持有的永远是首帧那个数组。父组件之后换 `rules` 引用完全无效。

这条踩在了短租退回日期上，原写法 `:rules="requireReturnPlanTime ? returnPlanTimeRules : []"` 因此产生两个对称缺陷：

| 场景 | 首帧 rules | 结果 |
|---|---|---|
| 新增行（`obs_project` 为空）→ 改成短租项目 | `[]` | **必填校验永不生效，可提交空的短租退回日期** |
| 存量短租行 → 改走别的项目类型 | 两条规则 | 规则仍在，而单元格已按 `readonlyFields` 置为禁用；若值不满足规则则**提交被永久卡死在一个改不动的格子上** |

修法是把「是否短租项目」搬进 validator 内部由闭包实时判定，`rules` 数组本身变成常量（不再需要 computed）：

```ts
const returnPlanTimeRules = [
  { validator: (val: string) => !requireReturnPlanTime.value || Boolean(val), message: '短租项目请填写短租退回日期' },
  {
    validator: (val: string) =>
      !requireReturnPlanTime.value || !dayjs(val).isBefore(dayjs(localData.value.expect_time), 'day'),
    message: '短租退回日期不能早于期望到货日期',
  },
];
```

规则顺序保证第二条只在「必填且已填」时才实际比较日期，语义与原先一致。

同一条机制在实例数量与云盘容量的 `message` 上还有两处残留：`isCbsOnly ? … : …` 同样定格在首帧，新增行 CVM→CBS 切换后报错文案会停在 CVM 版措辞。值判定不受影响（`> 0` / `>= 0` 对两态相同），但既然是已知错误行为就一并修了。修法不必把条件搬进 validator——`useValidtor` 的 `getRuleMessage` 本来就支持函数式 `message`：

```js
const getRuleMessage = (rule) => (typeof rule.message === 'function' ? rule.message() : rule.message);
```

改成 `message: () => (isCbsOnly.value ? '所需数量应大于0' : '实例数量应大于0')`，求值推迟到校验时。两列的 rules 一并提到 script 里作为常量（`osRules` / `diskPerSizeRules`），与 `returnPlanTimeRules` 同一形态。

> `osRules` 里的 `isCpuCoreInteger` 声明在文件下方，**不能直接引用**——rules 是 setup 期求值的常量数组，直接写会踩 TDZ。包一层 `(val) => isCpuCoreInteger(val)` 把查找推迟到调用时。

其余内联 rules 均为静态（无三元、无随行状态变化的闭包），定格无害，保持原样。

### 云盘类型：无条件必填（已补）

先按「只在新增时必填」实现过一版，核实现网后**推翻**——`required` 挂在 form-item 上是无条件的，不分 `AdjustType`：

```tsx
<bk-form-item label={t('云盘类型')} property='disk_type' required>
```

真正决定校不校验的是**面板渲不渲染**：`add/cbs/index.tsx` 的 render 是 `props.planTicketDemand.cbs && (<Panel …>)`，而调整态的 demand 先过 `convertToPlanTicketDemand`，其中 `cbs = detail.remained_disk_size > 0 ? {…} : undefined`。所以现网口径是：

| 场景 | `cbs` | 面板 | 云盘类型 |
|---|---|---|---|
| 新建 | 默认对象 | 渲染 | 必填 |
| 调整，云盘剩余量 > 0 | 有 | 渲染 | **必填** |
| 调整，云盘剩余量 = 0 | `undefined` | 不渲染 | 不校验 |

第三行正是表格里的 `hasNoDisk`——渲染 `TextPlainColumn`、拿不到 ref，天然不参与校验。所以规则不需要任何条件，无条件必填就完全对齐：

```ts
const diskTypeRules = [{ validator: (val: string) => Boolean(val), message: '请选择云盘类型' }];
```

> 先前「历史数据可能没有云盘类型，加必填会让它们提交不了」的顾虑不成立：`remained_disk_size > 0` 而 `disk_type` 为空的行，现网打开侧滑同样会被拦。差异只在触发面——现网只校验用户点开过的行，表格提交时全行都校验；但这对所有必填列（项目类型/城市/机型…）一视同仁，口径一致。

`hasNoDisk` 的判定用 `Number(baseline.remained_disk_size) === 0`，而现网谓词是 `!(x > 0)`；`demandDetailToAdjustRow` 里该字段是 `Number(detail.remained_disk_size) || 0`，恒为非负数值，两者等价。

并补 `diskTypeRef` 纳入 `getValue()` 收集——此前这一列没有 ref，即使有规则也只在交互时给行内反馈、提交不拦截。

### 真实缺口二：校验失败只有一句笼统提示（已修）

改动前 `handleSubmit` 只是 `catch` 掉 `validate()` 的 reject 后弹「请完善表格必填项后再提交」。表格横向 2019px、纵向内部滚动，出错行可能既不在视口内也不在滚动区内，用户拿不到定位信息。

改法（对应用户给的方向 2，且**不把校验并入 `canSubmit`**）：

- `data-list.validate()` 改为返回 `IAdjustValidateResult { valid, invalidRowIndexes }`，不再 reject。
- 逐行 `catch` 收集结果，**不用 `Promise.all` 的快速失败**：出错行序号要收全，Message 才能报总数。（顺带说明：即使用快速失败，所有行的 validator 也早已被同步启动，标红本来就是全的，丢掉的只是序号信息。）
- 新增 `scrollToInvalidCell()`：先 `scroller.scrollIntoView({ block: 'nearest' })` 让表格进入视口（视口过矮时页面自身仍会滚动），再在 `.bk-ediatable` 内部按两个方向归位，并让出 **sticky 表头高度**与**右固定列宽度**。横向目标取行内第一个 `.is-error` 元素，纵向等价。
  - 右固定列宽取 `th.is-right-fixed`，该 class 的条件是 `isMinimize && isFixedRight && !isScrollToRight` —— 恰好只在固定列真的遮住内容时存在，取不到时按 0 处理正好正确。
  - 量尺寸前 `await nextTick()`，避免按错误态渲染前的布局定位。
- `validate()` 开头补 `await nextTick()`：刚新增的行此刻可能还没有 ref，否则会被 `filter(Boolean)` 当成「不存在的行」静默跳过，而它会照常进 `add` 载荷。
- `index.vue` 按出错行数给提示：单行报「第 N 行…」，多行报「共 N 行…，已定位到第 M 行…」。

`submitDisabledReason` / `canSubmit` 的职责边界在注释里写死：**只收数据层面不允许提交的情形**（加载失败 / 存在未修改数据 / 无可提交变更），填写项校验一律交给点击时的整表校验。理由是表格形态下长期置灰只能给一句笼统 tooltip，用户无从知道是哪一行哪一列；让用户点一下、由校验把出错单元格标红并滚动到位，定位成本更低。

### 未改动的两处，留作决策

1. **`disk_io` 在禁用态仍会被校验**：`getValue()` 不看 `disabled`。`readonlyFields.disk_io = hasNoDisk || !disk_type`，而规则是 `> 0`。理论上「存量行 `disk_type` 为空 + `remained_disk_size > 0` + `disk_io <= 0`」会让提交卡在一个禁用格子上。留着不改的依据：`demandDetailToAdjustRow` 里 `disk_io` 来自接口且现网新建/调整都必填，这种数据形状基本不存在；且用户还有一条出路——云盘类型此时可选，选中后 `watch(disk_type)` 会把 `disk_io` 回落到 15 解锁（代价是多一项变更）。若确认线上存在 `disk_io = 0` 的存量数据，应把 `isReadonly('disk_io')` 一并折进该规则的 validator（与短租退回日期同一手法）。
2. **云盘类型表头有星号但无校验**：沿用前述「不加必填校验，以免存量数据提交不了」的结论，未动。若要补，需先确认存量行 `disk_type` 为空的比例。

### 变更文件

- `data-list.vue`：`validate()` 返回结构化结果 + 出错行滚动定位；新增 `scrollToInvalidCell()`。未触碰 max-height / sticky 表头相关样式。
- `index.vue`：`handleSubmit` 消费新结果并给可定位的 Message；`submitDisabledReason` 补职责边界注释。
- `typings.ts`：新增 `IAdjustValidateResult`（`<script setup>` 不能 `export` 类型，故落在 typings）。
- `render-row.vue`：**仅 2 处**——`returnPlanTimeRules` 定义、短租退回日期的 `:rules` 绑定。与并行进行的可用区下拉显示改动无重叠。

### 验证

- `npx eslint src/views/resource-plan/cvm/adjust/`：通过。
- `npx stylelint "src/views/resource-plan/cvm/adjust/*.vue"`：通过。
- `npx prettier --config ./.prettierrc.js --check src/views/resource-plan/cvm/adjust/`：通过。
- `vue-tsc` 本仓缺 `typescript/lib/tsc`，不可用，未纳入门禁。

### 收口待沉淀约定（promotions）

流程完成后应总结以下项目约定（本次评审纠正）：

1. **子页路由必须 `notMenu: true`**：新菜单体系未落地前，模块内非一级菜单页（如 adjust/detail）沿用 GPU 写法，写 `notMenu: true` + `menu.relative` + `layout.breadcrumbs`。
2. **统一面包屑与 `DetailHeader` 互斥**：已配置 `layout.breadcrumbs.show/back` 时，页面内不要再挂 `DetailHeader`。
3. **无特殊情况不外链 `index.scss`**：组件样式写在 SFC `<style scoped lang="scss">`；仅跨多文件复用或体积过大时再拆独立样式文件。
4. **`Panel` 只在稿面确有白色卡片时使用**：内容区底色为 `#f5f7fa` 且稿面无卡片时，直接用容器 padding，不要为了「有个标题」而套 `Panel`。
5. **吸底操作区的底色与上边线是悬浮态样式**：`position: sticky; bottom: 0` + 哨兵 `IntersectionObserver` 判定悬浮，非悬浮态不加底色/上边线；跟随吸底但非操作区的区块（如调整预览）不套用操作区底色。
6. **吸底 ≠ 撑满视口高度**：底部区块留在正常流里跟随内容，靠 `sticky` 在内容溢出时自然悬浮；不要用 `min-height: 100%` + `flex: 1` 把它顶到视口底部。
7. **编辑表格的只读态要有唯一判定源**：用一份 `readonlyFields` 同时驱动组件 `:disabled` 与单元格底色，不要依赖 `TextPlainColumn.default-display` 这类「空占位」样式顺带出灰底；单元格底色统一挂在 `<td>` 上，行级高亮写在只读之后压过它。
8. **ediatable 的 `:rules` 非响应式，条件校验必须写进 validator**：`useValidtor(props.rules)` 只在 setup 读一次，切换 rules 数组无效。凡「某条规则是否生效」依赖行内其它字段的场景，都要把条件折进 validator 由闭包实时判定，而不是在模板里换数组。另一半是 `getValue()` 不看 `disabled`，禁用态的格子照样跑规则，所以「不可编辑时不该校验」也要写进 validator。
9. **可编辑表格的提交拦截用「点击时整表校验」，不用长期置灰**：`canSubmit` 只承担数据层面不允许提交的情形（加载失败、无变更等）并配 tooltip；填写项校验在点击时整表跑一遍，逐行 catch 收集出错行、滚动定位到第一个 `.is-error` 单元格（让出 sticky 表头与固定列），Message 报出范围。整表校验前先 `await nextTick()`，否则刚新增、还没拿到 ref 的行会被静默跳过。

## 联调：接口调用清单与现网比对

现网「进入调整页 + 打开调整侧滑」共 11 个请求，逐个核对后**我们用了 9 个、少发 2 个**，无新增请求。

| 现网请求 | 我们 | 用处 / 不用的原因 |
|---|---|---|
| `POST bizs/{id}/plans/resources/demands/list` | ✅ | `adjustStore.listDemands`，按 `demand_ids` + `expect_time_range` 拉待调整行，映射为表格行与 `baseline` 快照 |
| `POST plans/demands/available_times/get` ×2 | ✅ | 两处：「期望到货时间」的 ⓘ 可申领周期提示 + 日期面板 footer；提交前 `validateExpectTimes` 校验新/改过的日期。按日期做 Promise 缓存，同日期只发一次（现网这里重复发了 2 次） |
| `GET config/find/config/apply/stage` | ❌ | **现网的副作用请求**：mod 页用了 `useScrColumns('planDemandModColumns')`，而 `useScrColumns` 顶部无条件调 `useApplyStages()`，后者 `onMounted` 即发请求。但其产物 `transformApplyStages(row.stage)` 只被主机申领类列消费（`use-scr-columns` L247/L422），调整列表不含 `stage` 字段。我们手写 Ediatable 列、不经过 `useScrColumns`，故不发——少发一个无用请求 |
| `POST meta/zone/list` | ✅ | `adjustStore.getZones(regionId)`，城市决定可用区选项；按 `regionId` 缓存 Promise |
| `GET meta/region/list` | ✅ | 城市列选项，`data-list.vue` 预载全表共用 |
| `GET plan/demand_source/list` | ✅ | 「变更原因」列的选项，`data-list.vue` 预载。现网只在新建态（`AdjustType.none`）渲染该表单项，我们按产品结论把新增行放开为下拉，故这个请求由「副作用」变成了真实消费方 |
| `GET meta/obs_project/list` | ✅ | 项目类型列选项，预载 |
| `GET meta/device_class/list` | ✅ | 机型类型列选项，预载 |
| `POST meta/device_type/list` | ✅ | `getDeviceTypes(deviceClass)`，机型规格选项，并提供 `cpu_core` / `memory` 供派生 CPU/内存；按 `deviceClass` 缓存 |
| `GET meta/disk_type/list` | ✅ | 云盘类型列选项，预载 |
| `GET plans/resources/tickets/report_deadline` | ✅ | `useDeadlineRestrict()`，评审期判定，驱动 `expect_time` 的 `disabled-date` 与面板 footer 的评审期提示；`data-list.vue` 调一次，props 下发全行 |

### 更正：评审期限制在现网调整态本就生效

此前记录成「调整页首次引入截止期限制」，不准确。现网调整侧滑复用 `add/basic`，其中：

```ts
const isModifyTicketScene = computed(() => route.name === MENU_BUSINESS_RESOURCE_PLAN_CVM_MODIFY);
const deadlineEnabled = computed(() => !isModifyTicketScene.value);
```

调整走的不是 modify 路由 → `enabled = true`，限制是**开着**的（只有「修改单据」场景才关）。所以我们接 `useDeadlineRestrict()` 属于**对齐**，不是新增能力。旧 mod 页自身没引用该 Hook，`report_deadline` 是侧滑内的 `add/basic` 发的。

### 调整单详情：新增预测打 NEW 标（Figma 2714:17065）

TAPD / PRD 的 F-002 只写到「调整内新增可混编」，**没有写 NEW 标**；判定与样式以设计稿单据详情帧为准，落点是审批/单据详情的「资源预测」表，不是调整编辑页（编辑页继续用整行绿底）。

判定（已与产品确认）：

```ts
ticketType === 'adjust' && !row.original_info
```

依据：提交 `add` 不带 `original_info`，详情 `TicketDemandItem` 注释也写了「新增单为 null；调整单为调整前快照」。纯「新增」单据不打——整单语义已是新增。实现挂在机型列右侧（`applications/detail/list/index.vue`），样式对齐稿面 Tag（`#2caf5e` / 高 16 / 字 10）。

### 新增行的「变更原因」：新增可选、存量只读（产品已定）

设计稿的调整表格没有这一列，但 `demand_source` 在 `add` 下是必填项，产品结论是**新增行给下拉、历史行只读**。落地成表格最后一列（操作列之前），形态与「资源类型」列完全同构——同样是 `is_new` 决定可编辑性，同样用 `readonlyFields` 统一驱动 `disabled` 与灰底：

```ts
demand_source: !localData.value.is_new,
```

其余属性对齐现网 `add/basic` 的那个下拉：`clearable={false}` + 预载 loading + 不加必填规则。不加规则不是漏了——新增行默认值是 `'指标变化'` 且不给清空入口，它取不到空值；`mapRowToAdjustInfo` 侧还有 `row.demand_source || DEFAULT_DEMAND_SOURCE` 兜底。

`demand_source` 不进 `CONFIG_KEYS`：它是「本次为什么改」的元信息，不是被比较的配置项；存量行只读也决定了它不可能产生 diff。

**存量行展示历史真值，取不到就 `-`**：`demands/list` 目前不返回 `demand_source`。回填时**不做兜底**（`detail.demand_source || ''`），存量行渲染成 `TextPlainColumn`，空值显示 `-`：

```ts
// demandDetailToAdjustRow
demand_source: detail.demand_source || '',
```

兜底只留在提交侧（`convertRowToAdjust` 的 add / update 分支与 `mapRowToAdjustInfo` 各自的 `|| DEFAULT_DEMAND_SOURCE`）。这样后端哪天在 list 里补上这个字段，前端不用改一行就直接显示真值；而在此之前也不会把「拿不到历史值」伪装成「变更原因是指标变化」。

存量行不用 disabled 的 `SelectColumn` 而用 `TextPlainColumn`，是因为空值下前者会显示「请选择」占位符——对一个不可编辑的字段是误导。灰底仍由 `readonlyFields` 统一给（与「机型类型」在 CBS 行的处理同构：条件渲染出文本 + `readonlyFields` 给底色）。

### 请求数随行数放大（已知，暂不优化）

`getZones` / `getDeviceTypes` 目前是**按单键请求 + 按键缓存**，各自只传一个元素的数组：

```ts
resourcePlanStore.getZones([regionId]);
resourcePlanStore.getDeviceTypes([deviceClass]);
```

于是整表载入时会并发 K 个可用区请求 + M 个机型请求（K/M 为去重后的城市数 / 机型类型数）。现网是一次只开一个侧滑、按需各发一次，总量相当但被摊开了；批量调整上限 100 行时我们会在 mount 瞬间打出几十个请求。

两个接口的入参本就是数组（`region_ids` / `device_classes`），合并成各一次请求是可行的改法：整表去重后各发一次，拿到结果再按键拆回现有的 `zonesCache` / `deviceTypesCache`，行内消费方无需改动。

**当前状态**：暂不优化。按键缓存已经消掉了「N 行同城市/同机型重复请求」这一层，剩下的放大量与去重后的基数成正比；等实际批量场景压出问题再动。此处留作性能问题的首查点。

## 二轮修复：校验错误态残留 & 移除行的下限

### ediatable 的校验错误态：只进不出，唯一出口是再跑一遍 validator

「短租退回日期」面板 footer 的 1/2/3 个月快捷按钮选中日期后，单元格已有的「短租项目请填写短租退回日期」报错不消失（要等面板收起或提交时的整表校验）；而在日历里正常点选日期能即时消除。读 `node_modules/@blueking/ediatable/vue3/index.es.min.js` 后确认这是机制问题，不是时序偶发。

`useValidtor(rules)` 的全部状态就是一个 `reactive({ loading, error, message })`，**没有任何 watch 或对外的 clear 方法**：

```js
function useValidtor(rules) {
  const state = reactive({ loading: false, error: false, message: '' });
  const validator = (targetValue) => {
    state.error = false, state.message = '';   // 清错误的唯一时机：进入 validator 的第一行
    // …逐条跑 rules，失败即 state.error = true; state.message = getRuleMessage(rule); 并 reject(message)
  };
  return { ...toRefs(state), validator };
}
```

要点三条：

1. **红框/红图标由 `message` 驱动**，不是 `error`：`class="{ 'is-error': Boolean(errorMessage) }"`。
2. **错误态只在 `validator()` 被调用的那一刻清零**。没有别的路径会清它。
3. **`validator` 失败会 reject**（reject 的是 message 字符串），调用方必须自己 catch。

于是各列组件的「错误态什么时候刷新」就等价于「validator 有哪些调用入口」，三个列组件差异很大：

| 列组件 | validator 的调用入口 | 程序化写值（改 `v-model` 绑的行字段）能否刷新错误态 |
|---|---|---|
| `DateTimePickerColumn` | DatePicker 的 `change`、`open-change(false)`（面板收起）、`expose.getValue()` | **不能**。值走 `:model-value` 单向下发，写值不触发 `change` |
| `InputColumn` | `change` / `blur` / Enter / `clear`、`expose.getValue()` | **不能**。且 `blur` 分支里还有 `if (modelValue.value)` 与 `oldInputText` 短路 |
| `SelectColumn` | `handleSelect` / `handleRemove`、内部 `watch(modelValue, …, { immediate: true })`、`expose.getValue()` | **能，但仅限写入真值**：watch 里 `if (typeof value !== 'object' && value)` 才跑 validator，写空串/null 直接 return |

三个组件 `expose` 的都只有 `getValue()`（`InputColumn` 多一个 `focus()`），**没有 `validate` / `clearValidate` / `resetValidate`**。`getValue()` 的实现就是 `validator(当前值).then(() => 当前值)`——所以它同时是「取值」「重校验」「清错误」，也是外部唯一能碰到校验态的把手。

据此修法：程序化写值后 `await nextTick()` 再调该列的 `getValue()`，并吞掉 reject。抽成 `render-row.vue` 的一个小工具：

```ts
const revalidateCell = async (cell?: { getValue?: () => Promise<unknown> } | null) => {
  await nextTick();
  await cell?.getValue?.().catch(() => false);
};
```

`nextTick()` 不能省：列组件的值是 `useModel(props, 'modelValue')`，`localData.value.xxx = …` 只是改了行数据，要等 `render-row` 重渲染把新 prop 传下去，`getValue()` 读到的才是新值；少这一帧就会拿旧值去校验，错误态原地复现。`.catch()` 也不能省：`getValue()` 校验失败必 reject，不接就是 unhandled rejection（这里本来就不关心结果，只要它跑一遍）。

两处调用点：

```ts
const setReturnTimeByMonth = async (month: number) => {
  if (!localData.value.expect_time) return;
  localData.value.return_plan_time = dayjs(localData.value.expect_time).add(month * 30, 'day').format('YYYY-MM-DD');
  await revalidateCell(returnTimeRef.value);
};
```

```ts
// watch(disk_type)
localData.value.disk_io = DEFAULT_DISK_IO;
await revalidateCell(diskIoRef.value);
```

第二处是同类问题的另一例：`disk_io` 规则是 `> 0`，用户填了 0 拿到报错后去换云盘类型，`watch(disk_type)` 把它回落到 15（一定合规），但 `InputColumn` 收不到 `change`，红框会一直留着。

### 其余程序化写值路径的排查结论

`render-row.vue` 里代码直接改行字段的地方全过了一遍：

| 写值路径 | 目标字段 | 该字段有校验规则？ | 结论 |
|---|---|---|---|
| footer 快捷按钮 `setReturnTimeByMonth` | `return_plan_time` | 有（必填 + 不早于到货日期） | **已修** |
| `watch(disk_type)` 回落 IO | `disk_io` | 有（`> 0`） | **已修** |
| `watch(demand_res_type)` 清项目类型 | `obs_project` | 有（必填） | 不需要。写的是空串，`SelectColumn` 的 watch 对空值 return，但清空后必填规则依然失败——残留的错误态恰好仍是正确的；反之若写入真值，那个 watch 自己会重校验 |
| `watch(device_class)` 清机型规格 | `device_type` | 有（必填） | 同上 |
| `watch(region_id)` / `watch(zone_id)` / `dropOrphanZone` | `zone_id` / `zone_name` / `region_name` | 无 | 无影响 |
| `syncCpuAndMemory` | `remained_cpu_core` / `remained_memory` | 无（`TextPlainColumn` 派生列） | 无影响 |
| `syncDiskSize` | `remained_disk_size` | **仅存量 CBS 行有**（`isCbsAdjustRow` 分支的 `InputColumn`） | 不需要。`syncDiskSize` 开头就 `if (isCbsAdjustRow || hasNoDisk) return`，恰好与「该格子有规则」互斥；且 `isCbsAdjustRow` 运行期不会翻转（`demand_res_type` 对存量行只读） |
| `watch(disk_type)` 写盘型名 | `disk_type_name` | 无 | 无影响 |

**另有一类残留没动，留作决策**：值没变、但**规则结果**因别的字段变化而翻转。`return_plan_time` 的两条规则都依赖 `requireReturnPlanTime`（项目类型）与 `expect_time`，所以「退回日期早于到货日期」的报错在用户把到货日期改早、或把项目类型改走短租之后就不成立了，但那一格不会自己重校验。没顺手修的原因是它不对称：补上重校验等于在用户没碰过的格子上**主动**标红（例如把到货日期改晚，存量退回日期立刻变红），这是行为新增而非缺陷修复，需要产品确认。当前口径下它仍会被提交前的整表校验纠正，不会误放行也不会误拦截。
### 同一机制的第二面：键入过程中错误态不会消

上面讲的是「程序化写值不消错」。同一机制还有一个更常见的面：**用户手打也不消错**。现象是「实例数量」格子已经聚焦、输入框里是 `1`、报错却还挂着「实例数量应大于0」。

成因链三段，全部落在 `InputColumn` 上（`node_modules/@blueking/ediatable/vue3/index.es.min.js`）：

1. **它确实绑了 `input`，但那个 handler 不校验**。模板上一次性绑了五个：`onBlur / onChange / onFocus / onInput / onKeydown`，而 `handleInput` 全文只有两行：

```js
const handleInput = (value) => {
  isBlur.value = false;
  modelValue.value = value;   // 只写值，不碰 validator
};
```

2. **`bk-input` 的 `change` 是原生 change 语义**。`bkui-vue/lib/input/index.js` 里 `handleChange` / `handleInput` 由同一个 `eventHandler(eventName)` 生成并绑在原生 `change` / `input` 事件上（`eventListener = { onInput, onChange, … }`），所以 `change` 要失焦或回车才触发。键入过程中只有 `input`。
3. **`InputColumn` 内部一个 `watch` 都没有**（对比 `SelectColumn`：它有 `watch(modelValue, …, { immediate: true })`，写入真值时会自己重校验，所以下拉列不吃这个坑）。

于是 validator 的入口仍是 change / blur / Enter / clear / `getValue()`，键入既不触发也没有 watch 兜，错误态就滞留到失焦。

**错误最初是谁造的**：`bk-input` 在 `type="number"` 下把值过一遍 `handleNumber`，里头有 `Math.max(newVal, props.min)`，而 `Number('') === 0`——所以**清空也会被钳到 `min`**：

| 列 | `min` | 清空后落到 | 规则 | 失焦能否造出错误 |
|---|---|---|---|---|
| 实例数量 `remained_os` | 1 | 1 | `> 0` | **不能**（钳制后恒 ≥ 1） |
| 云盘容量/实例 `disk_per_size` | 0 | 0 | `>= 0` | 不能（0 合规） |
| 云盘总量 `remained_disk_size` | 0 | 0 | `> 0` | 能 |
| 单实例磁盘IO `disk_io` | 0 | 0 | `> 0` | 能 |

所以截图那格的错误**只可能来自提交前的整表校验**（`getValue()`）：新增行 `createEmptyAdjustRow` 的 `remained_os` 初值是 `0`（存量行若接口返回 `remained_os: 0` 同理），用户点一次「提交调整」→ 全表标红 → 回到格子里键入 `1` → 值合法了、红框还在。截图里派生的「CPU总核数」已经跟着变成 12，恰好印证 `handleInput` 把值写进去了、只是没校验。

排除掉的其它可能：

- **不是第二条规则在报错**：`isCpuCoreInteger` 的消息是「请调整实例数为整数」，截图是第一条的动态消息（CVM 版措辞）。
- **不是 `handleClear`**：清除图标的渲染条件是 `clearable && modelValue && type !== 'number'`，数字列根本不渲染它。
- **不是 `min="1"` 本身有问题**：它反而让这一列失焦时造不出错误，正因如此才能断定错误来自整表校验。
- **不是动态 `message` 被覆盖**：`getRuleMessage` 每次校验时才求值，措辞与当前行状态一致。

#### 修法：「只降不升」的重校验

目标是「错误态随输入即时消除」，**不是**「每敲一个键校验一次」——后者会把中间态立刻标红（把 62 全选删掉的瞬间就飘红），比现在更聒噪。所以只做单向的：**已经出错的格子，输入使其合法就立即消错；没出错的格子，键入过程中不主动造错**（仍由 change / blur / 整表校验产生）。

难点是外部读不到校验态：三个列组件 expose 的只有 `getValue()`，而 `getValue()` 会无差别重跑规则、可能凭空造错。两条候选路径：

- **`@error` 事件**：`InputColumn` 在 change / blur / Enter / clear 里都 `emits('error', …)`，但 **`getValue()` 不 emit**——恰好漏掉本例这个唯一来源。否决。
- **读根元素的 `.is-error`**：`class="{ 'is-error': Boolean(errorMessage) }"` 就是用户看到的那个状态本身，三种列组件同名同义；`data-list.vue` 的 `scrollToInvalidCell` 早就在用这个信号定位出错单元格，不算新增耦合。采用。

`$el` 能拿到是因为 Vue 的 expose 代理对 `publicPropertiesMap` 里的键做了透传（`$el: (i) => i.vnode.el`，`runtime-core` 3.5.39），`expose({ getValue })` 不会把它挡掉。

```ts
const clearResolvedError = (cell?: IEditableCell | null) => {
  const el = cell?.$el;
  if (!(el instanceof HTMLElement) || !el.classList.contains('is-error')) return;
  revalidateCell(cell);
};
```

安全性来自 validator 自己的形状：进门先 `state.error = false; state.message = ''` 再逐条重判，所以对一个**已经出错**的格子重跑，结果只有「消错」或「维持同一条错误」两种，永远不会变出新错误。门禁在 watch 回调里同步读（默认 `pre` 时机，DOM 还是错误态），随后 `revalidateCell` 的 `await nextTick()` 才让新值到位。

触发点不走 DOM 事件，而是 watch 我们自己的行字段——`handleInput` 每次键入都会写 `modelValue`，也就是写到我们的 `v-model` 上，所以字段变化就是「用户在打字」的等价信号。四列共用一个 watch：

```ts
watch(
  () => [
    localData.value.remained_os,
    localData.value.disk_per_size,
    localData.value.remained_disk_size,
    localData.value.disk_io,
  ],
  () => {
    [osRef.value, diskPerSizeRef.value, diskSizeRef.value, diskIoRef.value].forEach((cell) => clearResolvedError(cell));
  },
);
```

这样不需要往 `InputColumn` 上挂事件（挂 `@input` 要走 attrs 透传，而它的 `$attrs` 同时落在根 `div` 和内部 `bk-input` 上，一次键入会触发两遍），也不需要在我们这侧另存一份「哪格出过错」的状态（会与真实校验态漂移）。

**与上一轮那两处 `revalidateCell` 的分工**（两套机制不打架，方向不同）：

| | 触发条件 | 是否可能造错 | 用途 |
|---|---|---|---|
| `revalidateCell`（无条件） | 代码显式调用 | 会（无差别重跑） | 程序化写值：footer 快捷按钮、切盘型回落 IO |
| `clearResolvedError`（有门禁） | 字段值变化 + 该格已出错 | 不会 | 键入过程中即时消错 |

程序化那两处**不能**改成靠这个 watch：写入同一个值时字段不变、watch 不触发（例如 footer 连点同一个月份），而那两处本就该无条件重跑。

#### 覆盖范围

| 列 | 组件 | 结论 |
|---|---|---|
| 实例数量 `remained_os` | `InputColumn` | **已纳入** watch |
| 云盘容量/实例 `disk_per_size` | `InputColumn` | **已纳入**（规则 `>= 0`，失焦造不出错误，但整表校验可以） |
| 云盘总量 `remained_disk_size` | `InputColumn`（仅存量 CBS 行） | **已纳入**；非该形态时 ref 为 null，门禁直接 return |
| 单实例磁盘IO `disk_io` | `InputColumn` | **已纳入** |
| 项目类型 / 城市 / 资源类型 / 机型类型 / 机型规格 / 云盘类型 | `SelectColumn` | 不需要。内部 `watch(modelValue)` 在写入真值时自己重校验；写空值时必填规则依然失败，残留的错误仍是正确的 |
| 期望到货时间 / 短租退回日期 | `DateTimePickerColumn` | 不需要。面板点选走 `change`、面板收起走 `open-change(false)`，两个入口都会重跑；唯一绕过的 footer 快捷按钮已由 `revalidateCell` 兜住 |
| CPU总核数 / 内存总量 等 | `TextPlainColumn` | 无规则、不参与校验 |

> **未实机验证**：本仓 `dev` 是 `bk-cli-service-webpack dev`，页面需要后端会话与 `planIds` 才能进；也没有 jsdom / vitest 之类可以单独挂载 `InputColumn` 的运行时（团队约定不新增依赖）。以上结论全部由源码推导，路径已在回报里逐段给出行为依据。

### 移除行的下限：至少留一行（不是「至少留一条历史预测」）

> **本节推翻了上一版**。上一版按「调整页不支持纯新增」写成了「历史行至少留一条」，并把由此产生的死锁记成「有意为之」。**产品最终结论相反：允许纯新增**，那一版的约束与结论已整体撤掉，别再按它改回去。

最终口径三条：

1. **允许纯新增**：可以把全部历史行移除，只提交新增行。调整单里混装 `add` 本来就是支持的（`convertRowToAdjust` 对 `is_new` 行直接产出 `adjust_type: 'add'`，不依赖任何 `demand_id`）。
2. **未修改数据仍必须先移除才能提交**：`submitDisabledReason` 里那条不动。
3. 于是「移除未修改 → 只剩新增行 → 提交」是必须走得通的**主路径**。

#### 真正的约束是「表格不能被清空」

撤掉历史行约束后剩下的唯一死角与 `is_new` 无关：**`+`（新增一行）按钮长在行操作列 `operation-column.vue` 上**，`data-list.vue` 的 `#data` 插槽只有一个 `v-for`、没有空态占位行，`index.vue` 底部只有「提交调整 / 移除未修改 / 取消」三个按钮，也没有调用 `data-list` 暴露出来的 `addRow()`。所以 `tableData` 一旦为空，页面上就**一个新增入口都没有**，用户只能刷新页面。

这才是后来人需要记住的那条：**行数下限的理由是「保住新增入口」，不是「保住调整语义」**。若哪天在表格外面补了空态占位或独立的「新增一行」按钮，这条下限就可以一并放开。

于是两处入口都退成「表里至少留一行」：

**行操作列**——不再按行区分，`data-list.vue` 里一个共享 computed 同时喂按钮置灰与实际拦截（这点是上一版保留下来的改进，别退回成两处各写一份 `length <= 1`）：

```ts
const canRemoveRow = computed(() => tableData.value.length > 1);

const removeRow = (index: number) => {
  if (!canRemoveRow.value) return;
  tableData.value.splice(index, 1);
};
```

文案回到 `'至少保留一行'`，`operation-column.vue` 的 `removeTip` prop 与 `render-row.vue` 的透传一并删掉（不再有第二种触顶原因，参数化没有收益）。

**「移除未修改」按钮**——判定改成「移除后表格会为空」，即未修改行数 == 总行数（未修改行必然是历史行，`isRowUnchanged` 对 `is_new` 恒返回 false，所以这个等号成立就意味着「全表都是未修改的历史行、没有新增行」）：

```ts
const removeUnchangedDisabledReason = computed(() => {
  if (unchangedCount.value === 0) return '不存在未修改数据';
  if (unchangedCount.value === tableData.value.length) return '移除后表格将为空，请先新增一行';
  return '';
});
```

tooltip 给的是**下一步动作**而不是限制本身：此时用户想继续（走纯新增），正确动作就是先 `+` 新增一行、再点移除。沿用页面既有的「不可用原因 computed + 空串表示可用」形态（与 `submitDisabledReason` 同构）。

顺带记一个**同类但更早存在的空表路径**：`loadData()` 在 `planIds` 为空时会 `tableData.value = []`。那是入参异常（正常入口一定带 `planIds`），不由移除操作产生，本次未处理；但它和「表格清空」是同一个失效面——真要兜住，正确的做法是补空态占位 + 外部新增入口，而不是继续在移除侧加限制。

#### 两个交互点的可达状态自查

`H` = 历史行（`!is_new`），`N` = 新增行。「提交调整」的 tooltip 取 `submitDisabledReason`，「移除未修改」取 `removeUnchangedDisabledReason`。

| # | 表格构成 | 提交调整 | 移除未修改 |
|---|---|---|---|
| 1 | H 若干，改了一部分，无 N | **置灰** `存在未修改数据，请先移除未修改后再提交` | **可点**（移除后还剩已改的 H） |
| 2 | H 若干，全部未改，无 N | **置灰** `存在未修改数据，请先移除未修改后再提交` | **置灰** `移除后表格将为空，请先新增一行` |
| 3 | H 若干，全部未改，有 N | **置灰** `存在未修改数据，请先移除未修改后再提交` | **可点** → 移除后 H 清空、只剩 N，提交转为可点 |
| 4 | H 若干，改了一部分，有 N | **置灰** `存在未修改数据，请先移除未修改后再提交` | **可点** |
| 5 | H 已清空，只剩 N | **可点**（payload 全是 `add`） | **置灰** `不存在未修改数据` |
| 6 | 表里只剩 1 行（无论 H 还是 N） | 按上面各行的规则 | 该行的行内移除按钮置灰 `至少保留一行`；「移除未修改」若命中会被 #2 的分支挡住 |
| — | H 只剩 1 条但另有 N | 同上 | 该 H 可移除（`length > 1`），移完进入 #5 |

情形 2 是上一版的死锁场景，现在**不再是死锁**：表里有行 → `+` 可用 → 新增一行即进入情形 3；或者改动任一历史行进入情形 1。两条出路都在页面上。

#### 情形 3 主路径的链路核对

逐个读过一遍，链路上没有隐性拦截：

| 环节 | 结论 |
|---|---|
| `confirmRemoveUnchanged` | `filter((row) => !isRowUnchanged(row))`，`isRowUnchanged` 对 `is_new` 恒 false → 新增行一条不漏 |
| `rowRefs` 清理 | 被移除行卸载时 Vue 以 `setRef(ref, null, …, isUnmount = true)` 调函数式 ref，`value = isUnmount ? null : refValue` → 走到 `setRowRef(key, null)` 的 `delete` 分支（Vue 3.5.39 `runtime-core`）。函数式 ref 在重渲染时不会被置 null，且各行 key 互不相同，不会误删存活行。`validate()` 本身也只按当前 `tableData` 的 `row_key` 查表，即使残留也进不了校验 |
| `submitDisabledReason` | 移除后 `unchangedCount === 0`；`buildAdjustPayload` 非空 → 返回空串，按钮转可点 |
| `buildAdjustPayload` / `convertRowToAdjust` | `is_new` 行进 `add` 分支：`{ adjust_type: 'add', demand_source, updated_info }`，不带 `demand_id` / `original_info`。全表皆 `add` 时 `adjusts` 就是纯 add 列表，无需 `original_info`，也不受「一条历史行都没有」影响 |
| 提交前整表校验 | `data-list.validate()` 照常校验新增行的必填项。空白新增行会被拦并标红——这是**正确**拦截（用户还没填），不是死角 |
| `validateExpectTimes` | `collectExpectTimesToValidate` 对 `is_new` 行只要 `expect_time` 非空就纳入可申领周期校验，与新建预测口径一致 |
| 100 条上限 | 与行来源无关，纯新增同样受限 |

> ~~`cloneRowAsNew` 会连 `expect_time` 一起复制~~ → **已改**：复制时清空 `expect_time` / `return_plan_time`（见下方「收口补充」）。

### 变更文件（本轮）

- `render-row.vue`：新增 `revalidateCell()` 与 `clearResolvedError()`（含 `IEditableCell` 类型）；`setReturnTimeByMonth` 改 async 并重校验退回日期列；`watch(disk_type)` 改 async 并重校验 IO 列；新增四个数字输入列的值 watch 用于键入时即时消错。（`removeTip` 透传已随本轮回改删除）
- `data-list.vue`：新增共享的 `canRemoveRow` computed，`removeRow` 与模板 `:removeable` 共用它。
- `index.vue`：新增 `removeUnchangedDisabledReason` / `canRemoveUnchanged`，「移除未修改」按钮的 `disabled` 与 tooltip 改吃它。
- `operation-column.vue`：净无改动（`removeTip` prop 加了又撤）。
- `adjust-payload.ts` / `typings.ts`：无改动。

### 验证（本轮）

- `npx eslint --no-eslintrc -c .eslintrc.js --ext .vue,.ts src/views/resource-plan/cvm/adjust/`：通过。
- `npx stylelint "src/views/resource-plan/cvm/adjust/*.vue"`：通过（未改样式）。
- `npx prettier --config ./.prettierrc.js --check src/views/resource-plan/cvm/adjust/`：通过。
- `vue-tsc` 本仓缺 `typescript/lib/tsc`，与本次改动无关，未纳入门禁。

---

## 收口补充（2026-08-05，coding→test 前）

相对 07-30 基线后的交互收口与范围纠偏，进入 test 前对齐文档。

### 范围纠偏

| 项 | 结论 |
|---|---|
| F-003 预测专用日历（周选择视觉 / 预测内外条） | **仍不做** |
| 表头批量改日期（稿 `2664:50073` / `2664:47705`） | **要做**（曾误归入 F-003 豁免；design.md 已改回） |
| F-004 首次进页 Message 迁移引导 | **去掉**（仅下线列表「部分延期」入口；与 TAPD AC-006「给引导」口径不一致，按产品当场确认） |

### 本轮落地

1. **表头批量改期望到货时间**（`data-list.vue`）  
   - `HeadColumn #append` + `BatchUpdatePopConfirm` + `DatePicker`  
   - `type=datetime` 仅为面板确定栏；`append-to-body` + `z-index` 防被 PopConfirm 遮挡  
   - 日历 footer「13周后」（与行内同套路：mouseup stop、绝对定位到确定行居中）  
   - 确定后写**全部行** `expect_time`；短租退回日若早于新到货日则清空  
   - 禁选与行内一致：评审期非本年 + 非本周过去日  

2. **行内期望到货**（`render-row.vue`）  
   - 非本周过去日禁选、「13周后」footer、可申领 tip  

3. **复制**（`adjust-payload.ts` `cloneRowAsNew`）  
   - 清空 `expect_time` / `return_plan_time`，其它配置原样  

4. **取消 / 返回**  
   - `routerAction.back` + `HistoryStorage.peek/pop` + UTF-8 安全序列化；取消跳过离开二次确认防双弹  

5. **工程清理**  
   - 删除空目录 `mod/`、`batch-postpone-sideslider/`；保留旧 path `/business/service/resource-plan-mod` → 新调整页 redirect  
   - 去掉 `index.vue` 首次 Message  

### 关键文件（收口）

- `src/views/resource-plan/cvm/adjust/data-list.vue`
- `src/views/resource-plan/cvm/adjust/render-row.vue`
- `src/views/resource-plan/cvm/adjust/adjust-payload.ts`
- `src/views/resource-plan/cvm/adjust/index.vue`
- `src/views/resource-plan/cvm/adjust/operation-column.vue`
- `src/store/resource-plan/cvm-adjust.ts`
- `src/views/resource-plan/route-config.ts`
- `src/constants/menu-symbol.ts`
- `src/router/module/business.ts`
- `src/router/module/service.ts`
- `src/router/utils/action.ts`
- `src/router/utils/history-storage.ts`
- `src/router/hooks/use-back.ts`
- `src/components/resource-plan/resource-manage/list/table/index.tsx`
- `src/views/business/resource-plan/detail/index.tsx`

---

## 增量：F-005 `demand_class` 一致性（2026-08-06）

### 目标

1. 列表批量调整禁止跨 `demand_class` 混选  
2. 混编新增继承历史 `demand_class`；历史原样回填  
3. 纯新增可选手选 `demand_class`，提交顶层必传  

### 落地

| 点 | 文件 | 行为 |
|---|---|---|
| 列表混选 | `list/table/index.tsx` | 允许勾选混合用途；混选时禁用「批量调整」并用 tip 说明（对齐截止期禁用态）；不弹 Message、不自动剔勾 |
| 表格列 | `data-list.vue` + `render-row.vue` | 「预测用途」列：历史/混编只读展示；纯新增可按行编辑；不一致时提交按钮禁用 + tip |
| 新增继承 | `data-list.vue` | `addRow` / `copyRow` 写入 `resolveDemandClass` |
| 载荷 | `adjust-payload.ts` + `cvm-adjust.ts` | `buildAdjustPayload` 带顶层 `demand_class`；`demand_class` 移出行内配置 diff |

> 契约增量（顶层 `demand_class`）只记在本工作流 `api.md`；**不要**改仓库根 `docs/api-docs/**`（后端范围，见 `fe-no-backend-edit`）。
