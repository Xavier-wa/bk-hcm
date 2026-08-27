# Coding — feat-third-party-prepaid-bill

**TAPD**: [#1069995598137371362](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137371362)

> 落码入口：page-list → comp-field-model → comp-data-list。HTTP 以 api.md 为准。  
> **模块目录必须按 page-list 规范**，对齐 `src/views/cloud-account-manage/permission-template/`（children/list/search + children/list/data-list）。禁止抄 gpu/list 的平铺 children/search。禁止沿用 `src/views/bill/bill/**` TSX。

## 目录归属（打平）

`views/bill` **不是**一级视图容器，但已经是「按菜单袋」：`account/`（旧云账号 TSX）+ `bill/`（固定月份汇总/明细/调整 TSX）。新云账号已打平到 `src/views/cloud-account-manage/`，没有继续往 `views/bill/account` 加 Vue3。

| 关联 | 结论 |
|------|------|
| 权限与预付费列表同为 **二级账号查看**（`main_account_find` / 行级 MainAccount），菜单挨着「云账单管理」、账号/产品主数据 | **业务同属账单域**；入口 IAM **不等于** 云账单的 `account_bill_find` |
| 查询模型（跨账期 vs 固定账单月）、禁止共用 Header、后续详情/导出只服务预付费、实现栈 Vue3 page-list vs 现网 TSX | **不是月度账单页的子页** |

打平原则（base.md）：功能在 `views/<原子模块>/` 自洽，菜单挂载与物理目录解耦。再塞进 `views/bill/prepaid/` 等于把袋子撑大，和 cloud-account-manage 的拆法相反。

**本期物理目录：`src/views/prepaid-bill/`**（不放 `views/bill/` 下）。Store 同步打平：`src/store/prepaid-bill/`（不建 `src/store/bill/`，不改 `src/api/bill`）。URL 仍可用 `/bill/prepaid`，与现网 `/bill/bill-manage` 并列，不表示源码从属。文档 relates: bill。

## 执行顺序

1. 字段模型 condition.ts / column.ts（单类型 getModel，无 Factory）
2. children/list/search/search.vue + children/list/data-list/data-list.vue
3. 入口 index.vue 组装请求、URL、分页、漏斗本地筛
4. Store（http 写在 store 内，不走 src/api）
5. 路由/菜单 + 详情空页
6. lint --fix

## 模块目录（硬约束）

```
src/views/prepaid-bill/
├── index.vue
├── typings.ts
├── constants.ts
├── utils.ts
├── route-config.ts
├── components/
│   ├── main-account-value.vue
│   ├── root-account-value.vue
│   └── operation-product-value.vue
└── children/
    ├── list/
    │   ├── search/
    │   │   ├── search.vue
    │   │   └── condition.ts
    │   └── data-list/
    │       ├── data-list.vue
    │       └── column.ts
    └── details/
        └── index.vue
```

同批改动：`src/store/prepaid-bill/`、`src/constants/menu-symbol.ts`、`src/views/index.ts`（`billViews = [...bill, ...prepaidBill]`，对齐 `businessViews`）。路由/侧栏只消费 `billViews`，不要在 `router/index.ts` / `useChangeHeaderTab` 再拆一份 prepaid。**不要**改 `src/store/common.ts` 给 `main_account_find` 加 path。**不要**改 `src/api/bill/index.ts`。**不要**改 `src/router/module/bill.ts` 把预付费挂成 bill-manage 的 child。**不要**调 `root_accounts/list`。**不要**用 `account_bill_find` 控本页入口。

- 单类型、单视角（资源运营），不注入 currentVendor，不建 entry-biz/rsc。
- 路由用本模块 route-config + Symbol name；在 `src/views/index.ts` 插入现网 `bill` 得到 `billViews`。现网月度账单仍走已废弃的 `src/router/module/bill.ts`，本期不整体迁移账单。
- 无新建/编辑/删除：入口不要拷贝 page-list 资产里的 create/edit/delete sideslider 与工具条「新建」。

## 改动点

### 1. 字段模型（comp-field-model）

`SearchCondition`（`@Model('bill-prepaid/search-condition')`）：

| 字段 | type | 区 | 序列化（api.md） |
|------|------|----|------------------|
| 使用时间 | `datetime` daterange，只到日 | 基础 | 一条 UI → 两条 rule：`usage_end_at gte` 开始日 + `usage_start_at lte` 结束日（重叠）；时间值 `toFilterDateTime`（RFC3339） |
| 运营产品 | `list` → `hcm-search-list` | 基础 | `product_id` `in`；`idKey: op_product_id`，`displayKey: op_product_name` |
| 二级账号 | `list` → `hcm-search-list` | 基础 | `main_account_id` `in`；名称展示、请求传 ID |
| 一级账号 | `list` → `hcm-search-list` | 高级 | `root_account_id` `in`；同上 |
| GPU 型号 | `string` | 高级 | `eq` 默认 |
| 产品名称 | `string` | 高级 | `cs` |
| 云厂商 | `enum` VendorEnum | 高级 | `in` |
| 订单月份 | 月份区间 | 高级 | `order_at gte` 月初 + `order_at lte` 月末；时间值 `toFilterDateTime` |

开始晚于结束：入口拦截，不发请求。

三个 ID 选择器走 **model + 领域组件**，对齐 `resource-plan` 运营产品的 `type: 'list'`，但数据源走本模块 store（见「复用接口」）：

- `@Column('list', { list / listGenerator, props: { idKey, displayKey } })` → `hcm-search-list`
- **不要**用 `type: 'account'`（资源账号选择器）
- **不要**抄账单 TSX 选择器和 `useOperationProducts`
- **不要**用 `resource-plan` 的 `getOpProductList`（那是 `/api/v1/woa/metas/op_products/list`，不是账单运营产品）

`TableColumn`（`@Model('bill-prepaid/table-column')`）22 列，`index` 1–22，列名跟 PRD。`sort: true` 的列走**接口排序**：DataList `@column-sort="handleSort"`（`usePage` 写 URL `sort`/`order`），入口 `getPageParams(pagination, { sort, order })` 传给 list。默认 `order_at` + `DESC`。不要对当前页本地排序。核算/单据列 `filter` 仅表格本地。

特殊列：默认仍 `display-value`。账号/产品列对齐 permission-template，**不要**入口先补名称再交给 display-value。

- `id`：`meta.display.appearance` 链接 → 详情空页路由
- 订单月份：列 id 仍是 `order_month`（与「订单时间」列的 `order_at` 区分）；展示从 `row.order_at` 取 `YYYY-MM`，与 Search 的 `order_at` 筛选同一字段。`data-list` 对 `meta.display.render` 传整行。
- `product_id` / `main_account_id` / `root_account_id`：见下方「表格 ID→名称」
- `accounting_state`：enum + `dynamic-status`（`accounted`→success 绿空心、`accounting`→ing 蓝 loading、`pending`→stop 灰空心）。**不要**用 `appearance: 'status'`——它按 `success`/`pending`/`failed` 等硬编码值选图，对不上 `accounted`/`accounting`。稿面该列的「失败」红圈不落核算态
- `settle_state`：enum + `dynamic-tag-status`（bkui Tag）。与后端对齐只有两态：`unsettled` 未定账（default）、`settled` 已定账（success）。不要「失败」。`appearanceProps` 除 `themeObject` 外透传 Tag 的 type/size/radius 等；图标优先 `#icon` slot，列模型用 `icon` / `iconObject`（按 theme 映射组件或 class）
- 数量/金额列 `number` 右对齐

**表格 ID→名称（对齐 permission-template）**

权限模板列表：列模型仍是 ID 字段，`data-list.vue` slot 出 `SecondaryAccountValue`。单元格只收 ID；`CombineRequest` 合并当页请求；store Map 缓存；展示名称，没有则 `--`。

本页抄这个形态，**不能直接复用** `SecondaryAccountValue`（云账号 `/cloud/.../accounts`，要 vendor + biz + resType）。

| 列 | 组件 | 取数 | 展示字段 |
|----|------|------|----------|
| `main_account_id` | `main-account-value.vue` | `main_accounts/list`，`id in`，每批 ≤100 | 优先 `name`，空则 `cloud_id` |
| `root_account_id` | `root-account-value.vue` | **不打** `root_accounts/list`。`parent_account_id` 就是一级账号 ID、`parent_account_name` 就是一级账号名称（二级账号详情页也这样标）。从本模块 main 缓存按 parent id 取值 | `parent_account_name` |
| `product_id` | `operation-product-value.vue` | `operation_products/list`，`op_product_ids`（≤500）+ page | `op_product_name` |

**取数时序（三个组件同一套，对齐 `user-value` / `SecondaryAccountValue`）**

1. 单元格 `value` 为空 → 直接 `--`，不请求。
2. `watchEffect`：ID 已在 store Map 里 → 不入队；未命中 → `combineRequest.add(ids)`。三个组件各用独立 `Symbol.for('prepaid-bill-*-value')`，互不混批。
3. CombineRequest 在同一 tick 把当页该列所有单元格的 ID 去重后交给 store `getXxxByIds`。
4. store：跳过已缓存 ID；剩余按上限切片（账号 100、产品 500）；`page: { count: false, start: 0, limit: 本批长度 }`；写入 Map。未返回的 ID **不写负缓存**（与 SecondaryAccountValue 一致：effect 不依赖 Map，不会死循环重打）。
5. 组件 computed 读 Map：有记录展示名称；请求结束仍没有 → 展示原始 ID（比 `--` 更可核对）。
6. 翻页后新行走同样逻辑；已缓存 ID 不打接口。入口 **不要** 在 list 返回后预拉名称。

三个组件结构相同，抽 `use-id-name-value.ts`。Search 下拉与表格 **同一 store、同一份 Map**（见下节），by-ids 先读缓存。

### 复用接口：现网场景、权限、store

| 接口 | 现网场景 | 接口鉴权 | 数据范围 |
|------|----------|----------|----------|
| `POST /main_accounts/list` | 云账单 Search 二级账号（`sub-account-selector` 直打 URL）；`useBillStore.main_accounts_list` | **二级账号查看** `MainAccount + Find` | 行级：`listAuthorized` 注入 `id in` 已授权账号；无权限返回空，不是全量 |
| `POST /root_accounts/list` | 云账单 Search 一级账号；账号申请单选一级账号 | **一级账号管理** `RootAccount + Find`（有/无，非行级） | 有权限看**全部**一级账号；无权限直接 403 |
| `POST /operation_products/list` | `useOperationProducts`：账单 Search、调账行、创建二级账号、账号详情名称翻译 | account-server **未做 IAM**，转 FinOps 目录 | 产品目录，不是按账号裁剪 |
| （对照）`prepaid_items/list` | 本页主列表 | 与二级账号同一套行级 `MainAccount + Find` | 无授权实例空列表 |
| （不要用）`/woa/metas/op_products/list` | resource-plan Search | 另一条产品线 | 与账单运营产品不是同一接口 |

页面入口和列表接口对齐 **二级账号查看**。本页**不调** `root_accounts/list`。

- **二级账号**：Search + 表格都走 `main_accounts/list`（同一 IAM，行级一致）。
- **一级账号**：从已授权 `main_accounts` 去重 `parent_account_id` / `parent_account_name`。`parent_account_id` = 一级账号 ID。
- **运营产品**：Search + 表格都走 `operation_products/list`。

#### 两个权限点，不要混用

| 前端 verify id | IAM action | 中文 | 形态 | 现网谁在用 |
|----------------|------------|------|------|------------|
| `account_bill_find` | `account_bill_manage` | 云账单-云账单管理 | **无关联资源**，有/无 | 云账单菜单 `checkAuth`；账单汇总/明细/调整接口 `AccountBill+Find` |
| `main_account_find` | `main_account_find` | 账号-二级账号查看 | **实例级**，行级隔离 | `main_accounts/list`；官方预付费列表文档写的「二级账号查看」 |

`account_bill_find` 不是 IAM 里叫 find 的独立 action，只是前端把 `{type: account_bill, action: find}` 编成的 id，auth-server `genAccountBillResource` 把它映射成 `account_bill_manage`。

官方预付费文档写的是「二级账号查看」，不是「云账单管理」。用 `account_bill_find` 控入口、接口却鉴 `main_account_find`：有二级查看、无云账单管理的人进不了页；有云账单管理、无二级查看的人能进页但列表空或 403。**本期入口跟列表接口走 `main_account_find`。**

若后端实现时改成「先 `AccountBill+Find` 再按 MainAccount 裁行」，再把入口改回 `account_bill_find`。弱依赖 TAPD `1069995598137352704` / `1069995598137350775`，联调核对。

入口形态**对齐云账单**，权限 id 用 **`main_account_find`**（不是 `account_bill_find`）：

| | 云账单 `/bill/bill-manage` | 本期预付费 `/bill/prepaid` |
|--|---------------------------|---------------------------|
| 菜单 | `checkAuth: 'account_bill_find'` | `checkAuth: 'main_account_find'` |
| `pageAuthData` path | **无**，直链不 403 | **不加**，直链不 403；**不改** `common.ts` |
| 页面 Vue 是否读 `permissionAction` | 不读 | 不读 |
| 无权限直链 | 进页，接口自己鉴 | 进页，列表接口鉴 `MainAccount + Find` |

现网 `main_account_find` 已在 `pageAuthData`（无 path），全局 verify 已有 `permissionAction.main_account_find`，业务 Vue **没有**读它。`403.tsx` / `AUTH_FIND_MAIN_ACCOUNT` **不改**。

本页：

- `checkAuth: 'main_account_find'`：无二级账号查看则不出现「预付费管理」
- **不加** path、**不拆**第二条 `pageAuthData`、**不改** `403.tsx`
- 直链无权限：进页，列表请求失败按 api.md 提示，不跳申请页
- 有权限但授权实例为空：进页，列表空态（接口 200 + 空 details）

**Store 是否与搜索下拉复用：要复用本模块 store，不是 `useBillStore`。**

对齐 permission-template：同一个 Pinia 模块里 Search 滚动 list 与表格 by-ids 是两个 action，**共享 Map**。Search 拉过的账号/产品，表格不再打接口。不把候选丢给 `useBillStore`（老杂货铺、无本页缓存）。不改 `src/api/bill`。

```
src/store/prepaid-bill/index.ts
  listPrepaidItems          # 主列表
  listMainAccounts          # Search 二级；写入 mainCache（含 parent_*）
  getMainAccountsByIds      # 表格二级；先读 mainCache
  listRootAccountsForSelect # Search 一级：main 去重 parent，不调 root_accounts
  getRootAccountLabel       # 表格一级：只读 parent 缓存
  listOperationProducts     # Search 运营产品
  getOperationProductsByIds # 表格运营产品
```

### 2. Search（comp-data-list）

复制 skill `search.vue`：`fields` + `condition`，emit `search`/`reset`，`grid-container` 4 列，全部字段铺开（不折叠高级筛选），`hcm-search-${type}`。

`hcm-search-list` 的 `listGenerator` **必须在 setup 里各创建一次**再传给控件。不要在 `getSearchCompProps` / 模板里每次渲染 `create*ListGenerator()`：`hcm-form-list` 的 `watchEffect` 依赖 generator 引用，新函数会「拉数 → 改 localList → 再渲染 → 再 create」死循环（`Maximum recursive updates exceeded`）。对齐 permission-template form 的 `computed(() => store.create…())`。

订单月份用 `hcm-search-datetime` 的 `type: 'monthrange'`（datepicker 原生月范围），不要拆成开始/结束两个框。使用时间、订单月份补 `placeholder`：`请选择日期` / `请选择月份`。

`useSearchQs` 同步 URL。`transform*` 不够用的字段用 `meta.search.filterRules`（使用时间、订单月份）。

### 3. DataList

复制 skill `data-list.vue`：`bk-table` + `usePage` + `useTableSettings`（22 列默认全开）。`remote-pagination`。`@column-sort="handleSort"`，sort/order 进 list 请求。

列渲染对齐 permission-template：默认 `display-value`；`main_account_id` / `root_account_id` / `product_id` 三列 slot 领域 Value 组件。无操作列。开头两列 `id` / `order_month` `fixed: left`，末尾两列 `vendor` / `settle_state` `fixed: right`。`settle_state` 有漏斗且贴设置齿轮：`minWidth: 160`，末列表头 `padding-right: 48px` 并去掉 head-action 负 margin。核算/单据状态枚举写在 `constants.ts`（`as const` + `_MAP`），不放进 `typings.ts`。

入口表格区走稿面「表格面板」：白底 + 边距（`table-panel`），上方 `toolbar` 先放描边「导出」+ `bkhcm-icon-download`（disabled，不接接口）。

漏斗：表格列 filter，**只滤当前页 `list`**，不改 `filter.rules`、不重发。分页 `count` 用接口总数。换页后漏斗作用在新页。名称列按行上的 ID 筛（漏斗是当前页本地，不依赖已加载完的名称）。

### 4. 入口 `index.vue`

- `getModel(SearchCondition)` / `getModel(TableColumn)`
- `watch route.query` → 组 filter（api.md）→ store 并行 count/list
- 页面壳对齐操作记录：route-config `isShowBreadcrumb` + `layout.breadcrumbs`；入口只 padding + Search + `table-panel`（toolbar + DataList），**不要**页内再写「预付费管理」标题
- 失败：Message，保留上次成功或空表
- 无 `main_account_find`：菜单隐藏；直链仍进页，不跳 `/403`（对齐云账单）
- 空授权：接口 200 + 空 `details` → 表格空态

### 5. Store（新模式，禁止 src/api）

对齐 `src/store/cloud-account-manage/`：HTTP 写在 Pinia Setup Store 里，页面只调 store action。不要往 `src/api/bill/index.ts` 加 `reqPrepaidBillList`。

```
src/store/prepaid-bill/
└── index.ts    # 主列表 + 账号/产品 Search list 与 by-ids，共享缓存
```

- 不要往 `src/api/bill/index.ts` 加方法；不要调用 `useBillStore` 做本页候选；不必挂到 `src/store/index.ts`
- Search 与表格共用本 store 的 Map，见「复用接口」
- 页面 `import { usePrepaidBillStore } from '@/store/prepaid-bill'`
- `listPrepaidItems` 发 `prepaid_items/list`（count/list 各一次），直接用真实返回。账号/产品下拉同样走真实接口。已删除 mock.ts 与 `USE_PREPAID_BILL_MOCK`。
- filter 时间走 RFC3339（`toFilterDateTime`，与 `toArray` 同在 `@/common/util`）。本页 `filterRules` 只映射字段与起止边界，不要手写 `YYYY-MM-DD HH:mm:ss`。不改通用 `convertValue`。

```ts
POST /api/v1/account/bills/prepaid_items/list
{ filter: { op: 'and', rules }, page: { count, start, limit, sort, order } }
```

- `count: true` 时 `start=0, limit=0`
- list 请求带 `sort`/`order`；默认 `order_at` / `DESC`；count 请求不带
- `rules` ≤ 10；漏斗不占 rules
- 空筛选 `rules: []`
- 账号/厂商单值也用 `in` + 数组

### 6. 路由 / 菜单

`src/views/prepaid-bill/route-config.ts` 注册独立项，由 `src/views/index.ts` 的 `billViews` 插入现网 `bill`（对齐 `businessViews`）。与「云账单管理」同级菜单、**不要**作为 bill-manage children（否则会进固定月份 Header）。`router/index.ts` 与 `useChangeHeaderTab` 只消费 `billViews`。

| | path | checkAuth |
|--|------|-----------|
| 列表 | `/bill/prepaid` | `main_account_find` |
| 详情空页 | `/bill/prepaid/detail/:id` | 同左，不进菜单 |

菜单标题「预付费管理」；icon 可复用 `bkhcm-icon-bill-manage`。

### 7. 明确不写

导出接口、详情内容、账单调整、稿面八宫格、漏斗进接口、TSX 旧页骨架。表格上方 `toolbar` 只占位「导出」按钮，不接导出接口。

## 文件

`src/views/prepaid-bill/index.vue` `src/views/prepaid-bill/typings.ts` `src/views/prepaid-bill/constants.ts` `src/views/prepaid-bill/utils.ts` `src/views/prepaid-bill/route-config.ts` `src/views/prepaid-bill/components/use-id-name-value.ts` `src/views/prepaid-bill/components/main-account-value.vue` `src/views/prepaid-bill/components/root-account-value.vue` `src/views/prepaid-bill/components/operation-product-value.vue` `src/views/prepaid-bill/children/list/search/search.vue` `src/views/prepaid-bill/children/list/search/condition.ts` `src/views/prepaid-bill/children/list/data-list/data-list.vue` `src/views/prepaid-bill/children/list/data-list/column.ts` `src/views/prepaid-bill/children/details/index.vue` `src/store/prepaid-bill/index.ts` `src/views/index.ts` `src/router/index.ts` `src/constants/menu-symbol.ts` `src/views/home/hooks/useChangeHeaderTab.ts` `src/views/home/index.tsx`
