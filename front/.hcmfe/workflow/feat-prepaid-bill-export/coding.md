# Coding — feat-prepaid-bill-export

**TAPD**: [#1069995598137371363](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137371363)

> 落码入口：page-list 只换列表入口工具条；导出按钮 adjacent 用 ExportToExcelBatchButton；列从列表 TableColumn 投影，不改列表列模型。HTTP 以 api.md 为准。不要改账单公共 API 封装、common store、通用 convertValue。

## 执行顺序（已完成）

1. Store：`listPrepaidItemsForExport`（`rollReqUseTotalCount` + `limit: 500` + `AbortSignal`；拉完后 by-ids 补名称缓存）
2. 导出列：新建投影文件，把列表 22 列投影成 ExportColumn（exportFormatter 出名称/枚举/金额）
3. 列表入口：替换列表页禁用占位为 ExportToExcelBatchButton（showIcon + showConfirmDialog + outline 透传）
4. lint --fix

## 文件

- `src/store/prepaid-bill/index.ts`
- `src/views/prepaid-bill/children/list/data-list/export-column.ts`
- `src/views/prepaid-bill/index.vue`

## 改动点

### Store

新增 `listPrepaidItemsForExport(filter, { sort, order, total }, signal)`：

- 接口仍是 `POST /api/v1/account/bills/prepaid_items/list`
- `rollRequest({ httpClient: http, pageEnableCountKey: 'count' }).rollReqUseTotalCount(..., { limit: 500, total, listGetter: res => res.data.details }, { signal })`
- **不要** `limit: 5000`；**不要**走现有 `listPrepaidItems`（那会改 `listLoading`、并行 count）
- 拉完后 `Promise.all`：`getMainAccountsByIds` / `getRootAccountsByIds` / `getOperationProductsByIds`，保证 formatter 同步读缓存
- filter 由页面传入当前 Search 的 `buildPrepaidBillFilter(...)`，与列表同一份

### 导出列

不要改列表列模型。新建投影文件，22 列顺序/列名与 TableColumn 一致：

| 列 | formatter |
|----|-----------|
| 订单月份 | `formatOrderMonthFromAt(row.order_at)`（与列表同一字段） |
| 运营产品 / 二级账号 / 一级账号 | 读 store 缓存名称，miss 则 ID |
| 优惠后总价 / 累计核算金额 | `formatAmount`；核算**不**按 `settle_state` 改 `--` |
| 核算状态 / 单据状态 / 云厂商 | `ACCOUNTING_STATE_MAP` / `SETTLE_STATE_MAP` / `BILL_VENDORS_MAP` |
| 其余 | 原值；空用 `--` 与列表空态一致 |

名称 format 与 Value 组件相同：二级 `name || cloud_id || id`；一级 `name || id`；产品 `op_product_name || id`。

### 列表入口

替换禁用按钮：

```vue
<ExportToExcelBatchButton
  outline
  show-icon
  show-confirm-dialog
  :request="exportAllRequest"
  :columns="exportColumns"
  filename="预付费账单"
  text="导出"
  name="预付费账单"
  :pick-num="pagination.count"
  :disabled="pagination.count === 0"
/>
```

- `outline` 透传到根 `bk-button`（组件未声明该 prop），对齐稿面描边；**不改**通用组件
- `showIcon` 已是 `bkhcm-icon-download`（design §3.x）
- `exportAllRequest(signal)`：用当前 `condition` + `route.query` 的 sort/order + `pagination.count`
- 列头漏斗不进 request
- 去掉 tooltip「导出能力本期未接入」

## 明确不做

- 勾选导出、CSV、详情/分摊导出
- `maxExportNum: 1000`
- 改造 ExportToExcelBatchButton 内部逻辑
- 改列表筛选、列模型、通用 Search convertValue
