# API — 预付费账单查询结果导出

> 本阶段只定义前端消费契约与序列化，**不改后端、不改 `docs/api-docs`**。  
> 单据：https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137371363  
> 列表契约见同需求另一迭代 `feat-third-party-prepaid-bill/api.md`。字段、枚举、`filter`/`page` 与本文冲突时以**列表 api.md** 为准。  
> 本迭代**不新开导出接口**。

## 1. 范围

| 能力 | 接口 | 本期 |
|------|------|------|
| 导出拉数 | `POST /api/v1/account/bills/prepaid_items/list` | **复用列表**，按当前 Search filter 分页拼装全量 |
| 名称补齐 | 列表已有 store 缓存与 by-ids | 复用，不新开接口 |
| 后端独立导出 / 详情 / 分摊 / 调整 | — | 不调 |

落码：继续写在 Pinia `src/store/prepaid-bill/`（`http.post` + `rollRequest`）。**不要**改 `src/api/bill/index.ts`，不要调 `useBillStore`。页面工具条 `src/views/prepaid-bill/index.vue`。

## 2. 导出拉数（复用 list）

```http
POST /api/v1/account/bills/prepaid_items/list
```

与列表同一接口、同一鉴权（IAM `main_account_find`，行级）。无授权实例不会出现在文件中（AC-013），前端不另拼权限 ID。

### 2.1 请求体

与列表相同：`filter`、`page` 均必填。filter 序列化、时间 RFC3339、`order_at` 月份区间、账号/产品 `in` + ID，全部复用列表页当前 Search 已生成的 `filter`（`buildPrepaidBillFilter`），**不要另写一套**。

导出分页与列表页展示分页不同：

| 项 | 列表页 | 导出 |
|----|--------|------|
| 目的 | 当前页 + 总数 | 按当前 filter 拉全量行 |
| `page.count` | 并行两次：一次 `true`（start/limit=0）、一次 `false` | 只发 list（`count=false`）；总数用列表已有 `pagination.count` |
| `page.limit` | `usePage()` 10/20/50/100，最大仍 500 | **固定 500**（接口上限）。禁止照抄申领主机样板里的 `limit: 5000` |
| `page.start` | 当前页 | 由 `rollRequest` 按 0、500、1000… 递增 |
| `page.sort` / `order` | 表头排序；默认 `order_at` `DESC` | 与当前列表一致，保证文件行序与列表默认/当前排序一致 |
| 列头漏斗 | 只滤当前页，不进 filter | **不进导出**。导出行数 = 接口 `count`（Search 命中），不是漏斗后的当前页条数 |

组件 `pick-num` 与 `rollReqUseTotalCount.total` 都用 `pagination.count`。`count === 0` 时按钮 disabled，本接口不调用。

`filter.rules` 仍最多 10 条；时间值必须 RFC3339（列表已用 `toFilterDateTime`）。不要改通用 `convertValue`。

### 2.2 分页拼装（前端）

对标申领主机「导出全部」：`rollRequest(...).rollReqUseTotalCount`，并把组件传入的 `AbortSignal` 交给 http 配置，才能「终止导出」。

```ts
rollRequest({ httpClient: http, pageEnableCountKey: 'count' }).rollReqUseTotalCount(
  '/api/v1/account/bills/prepaid_items/list',
  {
    filter, // 当前 Search 的同一份
    page: { sort: page.sort, order: page.order }, // start/limit/count 由 rollRequest 写入
  },
  {
    limit: 500,                 // 接口上限，不是 5000
    total: pagination.count,    // 已知总数，不再每页 count
    listGetter: (res) => res.data.details,
    countGetter: (res) => res.data.count,
  },
  { signal },
);
```

- 返回值：`IPrepaidBillItem[]`，交给 `ExportToExcelBatchButton` 的 `request`。
- 中止：`signal.aborted` 后不再写文件（组件已处理 abort）。
- 不设 1000 条业务截断；条数上限走组件默认 `maxExportNum`（约 45 万）与单文件拆分（约 15 万）。

### 2.3 名称补齐（导出前，不是新接口）

list 行上账号/产品只有 ID。Excel 要名称，与列表展示一致。`exportFormatter` 是同步函数，必须在 `request` 返回前补齐缓存：

| 列 | 行字段 | 复用 store |
|----|--------|------------|
| 运营产品 | `product_id` | `getOperationProductsByIds` |
| 二级账号名称 | `main_account_id` | `getMainAccountsByIds` |
| 一级账号名称 | `root_account_id` | `getRootAccountsByIds` |

缓存 miss 时与列表相同：展示 ID 或空，不要为此改 list 契约。

## 3. 响应与导出列

响应形态与列表相同：`data.details[]` 字段见列表 api.md §3.1。导出 **22 列**，列名/顺序/取值与列表一致；核算两列**不按** `settle_state` 改成 `--`。

| 列 | JSON | 导出文案 |
|----|------|----------|
| 预付费 ID | `id` | 原值 |
| 订单月份 | `order_at` | `formatOrderMonthFromAt` → `YYYY-MM`，与列表同一字段 |
| 资源 ID | `resource_id` | 原值 |
| 运营产品 | `product_id` | 缓存名称，不是 ID |
| 二级账号名称 | `main_account_id` | 缓存名称 |
| 一级账号名称 | `root_account_id` | 缓存名称 |
| GPU 型号 | `gpu_type` | 原值 |
| 数量(台) | `device_num` | 原值 |
| 数量(卡) | `card_num` | 原值 |
| 优惠后总价(不含税) | `cost` | `formatAmount`，与列表相同 |
| 累计核算金额 | `accounted_cost` | `formatAmount`，真实值，不遮蔽 |
| 核算状态 | `accounting_state` | 中文：待核算 / 核算中 / 已完成 |
| 币种 | `currency` | `RMB` / `USD` |
| 产品名称 | `product_name` | 原值 |
| 产品规格 | `product_spec` | 原值 |
| Region | `region` | 原值 |
| 使用开始时间 | `usage_start_at` | 列表展示形态（接口 DateTimeLayout） |
| 使用结束时间 | `usage_end_at` | 同上 |
| 订单时间 | `order_at` | 同上 |
| 发票 ID | `invoice_id` | 原值 |
| 云厂商 | `vendor` | `BILL_VENDORS_MAP` 中文 |
| 单据状态 | `settle_state` | 未定账 / 已定账 |

`ExportColumn`：`label` / `field` / `exportFormatter`。不要把表格 `type: selection` 之类列带进文件。

## 4. 错误码与前端行为

| 场景 | 前端 |
|------|------|
| `pagination.count === 0` | 按钮 disabled，不调接口、不下载 |
| `code=0` 分页拉全量成功 | 本地 xlsx + Toast「导出成功」 |
| 用户终止 | 中止后续分页，不下载 |
| 单页 >500 / 筛选非法 / 解 body 失败 | 弹窗失败原因，不留下半成品文件 |
| 中途某页失败 | 同上 |
| 超过组件 `maxExportNum` | 组件现成「请筛选后再导出」，不发分页请求 |
| 无 `main_account_find` | 与列表相同：菜单隐藏；直链失败按其它失败提示 |

参数错误 `2000001`；解失败 `2000004`；内部异常 `2000006`。

## 5. 调用示例

当前 Search 与列表完全相同的 body，仅 `page` 改为导出分页。例如已筛选云厂商 aws、二级账号、订单月份 2026-07～2026-09 时，第一页：

```json
{
  "filter": {
    "op": "and",
    "rules": [
      { "field": "vendor", "op": "in", "value": ["aws"] },
      { "field": "main_account_id", "op": "in", "value": ["00000001"] },
      { "field": "order_at", "op": "gte", "value": "2026-07-01T00:00:00+08:00" },
      { "field": "order_at", "op": "lte", "value": "2026-09-30T23:59:59+08:00" }
    ]
  },
  "page": { "count": false, "start": 0, "limit": 500, "sort": "order_at", "order": "DESC" }
}
```

第二页 `start: 500`，以此类推，直到凑齐 `pagination.count` 条。

## 6. 明确不调

- 无 `.../prepaid_items/export` 或任何下载 URL
- 不分摊 `split_items/list`
- 不 `root_accounts/list`
- 不改通用 Search `convertValue`
