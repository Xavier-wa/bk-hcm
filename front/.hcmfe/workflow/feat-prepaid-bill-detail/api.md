# API — 预付费账单详情与分摊明细

> 版本：`v9.9.9+`。权限（官方文档）：**二级账号查看** = IAM `main_account_find`，与列表页同一点。  
> 本阶段只定义前端消费契约与序列化，**不改后端、不改 `docs/api-docs`**。  
> 单据：https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137371365  
> 列表契约见同需求另一迭代 `feat-third-party-prepaid-bill/api.md`；主数据字段、枚举与本文冲突时以**列表 api.md** 为准。

## 1. 范围

| 能力 | 接口 | 本期 |
|------|------|------|
| 详情主数据 | `POST /api/v1/account/bills/prepaid_items/list` | 按 `id` 过滤单条，**不新开详情 GET** |
| 分摊与调账整合明细 | `POST /api/v1/account/bills/prepaid_items/{id}/split_items/list` | 新增消费 |
| 运营产品 / 二级账号名称补齐 | 列表已有 store 缓存与 by-ids | 复用，不新开接口 |
| 导出 / 调整守卫 | — | 不调 |

落码：继续写在 Pinia `src/store/prepaid-bill/`（`http.post`）。**不要**改账单公共 API 封装、common store。页面 `src/views/prepaid-bill/details/`。

## 2. 详情主数据（复用 list）

```http
POST /api/v1/account/bills/prepaid_items/list
```

只发 **一次 list**，不要并行 count。`id` 可进 `filter.rules`（见列表 api.md §2.2）。

分页用项目工具，不要手写 `{ start: 0, limit: 1 }`：

- `onePageParams()`（`@/utils/search`，即「取一条」的 page）
- 再 `enableCount(..., false)` 补上 `page.count = false`

```ts
import { enableCount, onePageParams } from '@/utils/search';

http.post('/api/v1/account/bills/prepaid_items/list', enableCount({
  filter: {
    op: 'and',
    rules: [{ field: 'id', op: 'eq', value: id }],
  },
  page: onePageParams(),
}, false));
```

等价请求体：

```json
{
  "filter": {
    "op": "and",
    "rules": [{ "field": "id", "op": "eq", "value": "<PREPAID_BILL_ID>" }]
  },
  "page": { "count": false, "start": 0, "limit": 1 }
}
```

- 路径参数来自路由 `:id`，与列表首列 `id` 相同。
- `details[0]` 即详情主数据；字段与列表 22 列同一套 JSON（`id`、`order_at`、`resource_id`、`product_id`、`main_account_id`、`root_account_id`、`gpu_type`、`device_num`、`card_num`、`cost`、`accounted_cost`、`accounting_state`、`currency`、`product_name`、`product_spec`、`region`、`usage_start_at`、`usage_end_at`、`invoice_id`、`vendor`、`settle_state` 等）。订单月份展示从 `order_at` 取 `YYYY-MM`，与列表/Search 同一字段。
- **无失败原因字段**。主表 `settle_state` 仅 `unsettled` / `settled`。AC-019 仅当联调出现失败原因字段时才渲染，不为此发明请求。
- `details` 空（无权限、不存在、越权）：**不渲染**主信息与分摊表（AC-020）。不要把空列表画成成功空字段。
- 名称补齐：`product_id` / `main_account_id` / `root_account_id` 复用列表 store 缓存与 by-ids，不在详情再拉全量候选。

### 2.1 整单核算摘要（前端规则，不是接口字段）

摘要读主数据的 `accounting_state`、`accounted_cost`（展示用原币；`accounted_rmb_cost` 本期不单独占格）。

| 主数据 `settle_state` | 核算状态 | 累计核算金额 |
|----------------------|----------|--------------|
| `settled` | 真实值：`pending` 待核算 / `accounting` 核算中 / `accounted` 已完成 | `accounted_cost` 绝对值字符串 |
| `unsettled` | `--` | `--` |

列表页仍如实展示这两项；**只有详情页**按上表遮蔽。

币种：基本信息「优惠后总价」展示为 `{cost} {currency}`，不单独占 Descriptions 格。

## 3. 分摊与调账整合明细

```http
POST /api/v1/account/bills/prepaid_items/{id}/split_items/list
```

- 路径 `id`：预付费账单 ID（与路由、主数据 `id` 相同）。
- **无请求体**，不要传 `filter` / `page` / sort。
- 权限：二级账号查看。越权以后端为准；前端不渲染他人明细。
- 一次返回全部行（产品最多 37 行）。`data.count` 为行数；前端**不分页**。
- 账期、调账金额表头排序：**只排本次 `details`**，不重发请求。

### 3.1 响应

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 2,
    "details": [
      {
        "adjustment_id": "00000101",
        "bill_year": 2026,
        "bill_month": 8,
        "accounted": true,
        "type": "increase",
        "cost": "1000",
        "rmb_cost": "7000",
        "currency": "USD",
        "res_class": "gpu_card",
        "res_sub_class": "H100",
        "push_status": "pushed",
        "settle_state": "unsettled",
        "memo": null
      },
      {
        "adjustment_id": "00000104",
        "bill_year": 2026,
        "bill_month": 7,
        "accounted": false,
        "type": "decrease",
        "cost": "3000",
        "rmb_cost": "21000",
        "currency": "USD",
        "res_class": "",
        "res_sub_class": "",
        "push_status": "unpushed",
        "settle_state": "unsettled",
        "memo": "预付费账单一次性扣减，uuid: ofs-uuid-0001"
      }
    ]
  }
}
```

### 3.2 `details[n]` 与六列

| 列 | JSON | 展示 |
|----|------|------|
| 调账编号 | `adjustment_id` | 原文 |
| 账期 | `bill_year` + `bill_month` | `YYYY-MM`（月补零） |
| 核算状态 | `accounted` | `true` → 已核算；`false` → 未核算。**不要**用整单 `accounting_state` 三态 |
| 调账类型 | `type` | `increase` → 增加；`decrease` → 减少（稿面「调减」落地用「减少」） |
| 调账金额 | `cost` | **绝对值字符串**，右对齐，不拼 `+`/`-`；增减只看 `type` |
| 备注 | `memo` | `null` / 空 → 空或 `-` |

`accounted` 口径（后端约定，前端只读布尔）：该行 `push_status=pushed` 时为 `true`。缺 `accounted` 时不要前端猜，联调对齐。

### 3.3 响应有、本期表不展示

| 字段 | 说明 |
|------|------|
| `rmb_cost` | 人民币金额 |
| `currency` | 行币种 `RMB` / `USD` |
| `res_class` / `res_sub_class` | 资源类别 / 子类 |
| `push_status` | `unpushed` / `pushing` / `pushed` / `failed`；列用 `accounted`，不单独出推送列 |
| `settle_state` | 行级 `unsettled` / `settled`；不是标题区整单单据状态 |

### 3.4 本地排序

| 列 | 排序键 |
|----|--------|
| 账期 | `(bill_year, bill_month)` |
| 调账金额 | `Number(cost)`（已是绝对值） |

默认顺序 = 接口 `details` 原序。订单月全额调减是否单独成行由**后端返回**保证（AC-017）；前端不补造调减行。

## 4. 调用时序

1. 进页后用路由 `id` **并行**拉主数据 list 与 split_items。
2. 主数据失败或 `details` 空：提示 / 不渲染；分摊表也不展示他人数据（若 split 已返回则丢弃）。
3. 主数据成功、split 失败：基本信息可展示；表区错误提示，不虚构行。
4. `count=0` 且 `details=[]`：空表。

## 5. 错误码与前端行为

| 场景 | 前端 |
|------|------|
| `code=0` 且主数据 `details` 空 | 不展示该账单字段与分摊（越权/不存在） |
| `code=0` 且分摊 `details` 空 | 空表 |
| 参数错误 `2000001` / 解码 `2000004` | 提示错误，不画成功空壳 |
| 其它失败 `2000006` 等 | 提示；主失败不渲染页；仅 split 失败则保住主信息 |
| 无 `main_account_find` | 与列表相同：菜单隐藏；直链可进页、不跳 `/403`；接口自行鉴权 |

## 6. Mock

已去掉。列表、详情、分摊、账号/产品全部走真实接口，不再保留 `USE_PREPAID_BILL_MOCK` 与 mock.ts。

## 7. 与 Design / Coding 的交叉（HTTP 以本文为准）

- 主数据不新开 GET；list `id eq` + `limit 1`。
- 分摊无 body；排序只本地。
- 金额绝对值；类型 `increase`/`decrease`。
- 摘要遮蔽只发生在详情读 `settle_state` 之后。
- HTTP 写在 `src/store/prepaid-bill/`，不改 `src/api/bill/index.ts`。
