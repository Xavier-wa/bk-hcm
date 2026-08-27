# 账单

> status: drafted · kind: module
> globs: `src/views/bill/**`, `src/views/prepaid-bill/**`, `src/store/prepaid-bill/**`

账单与账号账单视图。现网本目录含旧云账号 TSX 与云账单管理（固定月份）。预付费管理业务同属账单域，物理打平到 `src/views/prepaid-bill/`，不继续堆进本 glob。

## 职责

- **云账单管理**：按**固定账单月份**查询账单汇总 / 明细 / 调整。入口 `src/router/module/bill.ts` → `/bill/bill-manage`，页面 `src/views/bill/bill/`。
- **账单调整列表**：`src/views/bill/bill/adjust/index.tsx`（`/bill/bill-manage/adjust`）。「调账状态」列后是「推送状态」「定账状态」两列 Tag（`push_status` / `settle_state`）；失败原因挂在失败 Tag tooltip。`source` / `source_id` 不展示。行编辑/删除/勾选按来源、推送、定账守卫：预付费、推送中、已定账不可改；缺来源时仅未确认可操作；人工来源且非推送中、未定账时已确认也可编辑/删除。数据仍走 `reqBillsAdjustmentList`，不新开接口。
- **预付费管理**：跨账期只读列表。入口形态对齐云账单（`checkAuth` 藏菜单、`pageAuthData` 不加 path、直链不 403），权限点是 **二级账号查看**（`main_account_find`），不是 `account_bill_find`。页面不在本 glob，见 `src/views/prepaid-bill/`。
- **云账号管理（本目录旧实现）**：`src/views/bill/account/`。新 Vue 实现已打平到 [云账号管理](cloud-account-manage.md)。
- 云账单视图权限：`account_bill_find`（IAM `account_bill_manage`），只覆盖本目录的云账单管理，不覆盖预付费。

## 关键流程 / 注意事项

### 云账单管理 Header（现网）

`src/views/bill/bill/header/index.tsx`：

- 账单月份选择器（`DatePicker type=month`）+ 当月汇率。
- Tab：账单汇总 / 账单明细 / 账单调整（路由 `billSummary` / `billDetail` / `billAdjust`）。
- 查询以「当前账单月」为前提；**跨自然月的预付费账单不能挂在此 Header 下**。

### 预付费列表与详情

跨账期、不挂本模块 Header。物理目录打平到 `src/views/prepaid-bill/`，Store `src/store/prepaid-bill/`。路由 `src/views/prepaid-bill/route-config.ts`：`/bill/prepaid`（page-list）与 `/bill/prepaid/detail/:id`（page-detail 独立路由，面包屑 `back: true`）。在 `src/views/index.ts` 插入现网 `bill` 得到 `billViews`（对齐 `businessViews`）；`router/index.ts` 与 `useChangeHeaderTab` 只消费 `billViews`。独立路由，不是 `bill-manage` 的 child。

详情页：

- 主数据复用 `POST /api/v1/account/bills/prepaid_items/list`，`id eq` + `onePageParams()` + `enableCount(..., false)`，取 `details[0]`；空列表不渲染。
- 分摊 `POST /api/v1/account/bills/prepaid_items/{id}/split_items/list`，无请求体；整合表六列、不分页，账期/金额仅本地排序。
- 目录：独立路由详情在模块根 `src/views/prepaid-bill/details/`（与列表 `children/` 平级，对齐 task）。`details/index.vue` 页面壳取数；`details/children/basic-info/` 基本信息三列 Grid；`details/children/split-list/` 分摊表。
- 未定账时整单核算摘要两项显示 `--`；已定账展示真实 `accounting_state` 与 `accounted_cost`。列表页这两项不遮蔽。
- 列表工具条「导出」：`src/views/prepaid-bill/index.vue` 使用 `ExportToExcelBatchButton`，按当前 Search 分页拉同一 list 接口（单页 limit 500）本地生成 xlsx；`pick-num` 用列表 `count`，0 条不可点。列投影 `children/list/data-list/export-column.ts`，核算两列导出真实值。Store `listPrepaidItemsForExport` 带 `AbortSignal`，拉完后补账号/产品名称缓存。不新开导出接口。
- 列表筛选「GPU 型号」「产品名称」：字段 `filterRules` 调 `buildFilterRulesWithSearchSelect`（与操作记录 `res_name`、CLB 名称相同）——每条 `cs` 值是单个字符串，多关键词外层 `or`。列表/导出直接 `transformSimpleCondition`，不再包 `flattenAndRules`。


### 实现形态

现网账单汇总 / 明细 / 调整为 TSX 页（`src/views/bill/bill/**`）。新的跨账期列表不应复用该 Header，也不应以这些 TSX 页为骨架。
