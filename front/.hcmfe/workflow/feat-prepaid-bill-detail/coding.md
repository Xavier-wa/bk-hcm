# Coding — feat-prepaid-bill-detail

**TAPD**: [#1069995598137371365](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137371365)

> 落码入口：page-detail → 独立路由详情壳；comp-field-model + comp-detail 基本信息；分摊表 comp-data-list。HTTP 以 api.md 为准。  
> 独立路由详情跟 task 模块：放在模块根 details/，不要塞进列表 children/details。不要改账单公共 API 封装、common store。

## 执行顺序（已完成）

1. Store：getPrepaidItem（onePageParams + enableCount false）+ listSplitItems
2. 详情字段模型 field.ts（单类型 getModel；group 由字段声明，组件按 getPropertiesByGroup 出面板）
3. basic-info.vue（GridContainer column=3；分组走 getPropertiesByGroup）
4. 分摊列模型 + 无分页 DataList（账期/金额本地排序，金额按数值）
5. details/index.vue 页面壳：route params 取数、单据 Tag、摘要遮蔽、空态
6. 目录调整为模块根 details/（对齐 task）
7. 去掉 mock：删除 mock.ts，store 全部走真实接口
8. lint --fix

## 文件

- src/views/prepaid-bill/details/index.vue
- src/views/prepaid-bill/details/children/basic-info/basic-info.vue
- src/views/prepaid-bill/details/children/basic-info/field.ts
- src/views/prepaid-bill/details/children/split-list/column.ts
- src/views/prepaid-bill/details/children/split-list/data-list.vue
- src/views/prepaid-bill/route-config.ts
- src/store/prepaid-bill/index.ts
- src/views/prepaid-bill/typings.ts
- src/views/prepaid-bill/constants.ts
- src/views/prepaid-bill/utils.ts

HTTP 写在预付费 Pinia store。

## 改动点

- 独立路由详情目录对齐 task：views/prepaid-bill/details/ 与列表 children/ 平级；页面壳 details/index.vue；基本信息与分摊表在 details/children/。
- 基本信息：GridContainer 指定 3 列自适应，不手切字段；面板标题用字段 group，组件不写死分组名。订单月份从 `order_at` 取 `YYYY-MM`，与列表/Search 同一字段。
- 路由组件改为模块根 details 页面壳，面包屑 back 已开，页内不再做返回按钮。
- 标题旁 ID 与单据状态 Tag 用 Teleport 插入 #breadcrumbHead（| ID + Tag），不要页内 detail-header。
- 主数据：list 接口 + onePageParams + enableCount(..., false)，只取第一条；空数组不渲染。
- 分摊：split_items/list，无 body；表头排序只排本次已加载行；调账金额按 Number(cost) 本地排。
- 未定账摘要占位 `--`；已定账展示核算 Tag 与累计金额（已完成 success，核算中 info）。
- 调账金额绝对值；类型 increase/decrease 文案为增加/减少，列用动态 Tag（增加 success、减少 danger）。
- 已去掉 mock 文件与开关；列表、详情、分摊、账号/产品下拉全部走真实接口，失败直接抛错。
