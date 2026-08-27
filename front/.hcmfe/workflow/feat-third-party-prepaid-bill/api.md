# API — 预付费账单列表查询展示

> 版本：`v9.9.9+`。列表接口权限（官方文档）：**二级账号查看** = IAM `main_account_find`（`MainAccount + Find`，行级）。  
> **不要**用 `account_bill_find` 控本页入口：那是另一权限点，映射 IAM `account_bill_manage`（云账单-云账单管理），与现网账单汇总/明细/调整接口一致，**和预付费列表文档不是同一个 action**。  
> 本阶段只定义前端消费契约与序列化，**不改后端、不改 `docs/api-docs`**。  
> 单据：https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137371362

以下字段、枚举、`filter`/`page` 形态以本文件为准（覆盖此前草稿里的 `settle_status` / `settled_amount` / `page.sort`）。

## 1. 范围

| 能力 | 接口 | 本期 |
|------|------|------|
| 预付费列表 | `POST /api/v1/account/bills/prepaid_items/list` | 新增消费 |
| 二级账号候选 | `POST /api/v1/account/main_accounts/list` | 复用 |
| 一级账号候选 | `POST /api/v1/account/root_accounts/list` | 复用 |
| 运营产品候选 | `POST /api/v1/account/operation_products/list` | 复用 |
| 详情 / 导出 | 无独立接口 | 不调 |

落码：Pinia store `src/store/prepaid-bill/index.ts`（`http.post` + `enableCount`，对齐 `src/store/cloud-account-manage/`）。**不要**改 `src/api/bill/index.ts`。页面 `src/views/prepaid-bill/`。

## 2. 列表查询

```http
POST /api/v1/account/bills/prepaid_items/list
```

功能：查询预付费账单列表。行级鉴权由接口完成：无授权实例返回空列表（`count=0`），不是 500、不是全量。前端不自己拼权限 ID。

### 2.1 请求体

`filter`、`page` **均必填**。

```ts
interface PrepaidBillListReq {
  filter: {
    op: 'and' | 'or';
    rules: PrepaidBillRule[];
  };
  page: {
    count: boolean; // 必填
    start?: number;
    limit?: number; // 最大 500
    sort?: string; // 列字段，如 order_at
    order?: 'ASC' | 'DESC';
  };
}

interface PrepaidBillRule {
  field: string;
  op: 'eq' | 'neq' | 'gt' | 'gte' | 'le' | 'lte' | 'in' | 'nin' | 'cs' | 'cis';
  value: string | number | string[] | number[];
}
```

- `filter.rules` 最多 **10** 条。空数组 = 查权限范围内全部（`op` 无意义）。
- 多条件默认 `filter.op = "and"`。
- 官方示例对账号、云厂商即使用 `in` + 数组（单值也用 `in`）。
- 比较时间用 `gte` / `lte`（与官方示例一致；文档亦列出 `le`，前端不混用）。**filter 里的时间值必须是 RFC3339 / TimeStdFormat**（`2006-01-02T15:04:05Z07:00`，如 `2026-07-01T00:00:00+08:00` 或 `2026-07-01T00:00:00Z`）。`YYYY-MM-DD HH:mm:ss` 是 DateTimeLayout，只用于同步写入/展示，过不了 `TimeStdRegexp`。
- list 请求带 `page.sort` + `page.order`（字段名 = 列 `id`，方向 `ASC`/`DESC`）。未点表头时默认 `order_at` + `DESC`。count 请求不带 sort。漏斗仍只筛当前页，不进 `filter.rules`。
- `page.count === true`：只返回总数，`start` 与 `limit` **必须为 0**；此时 `data.count` 有值。
- `page.count === false`：返回 `details`；文档约定此时 `count` 无业务值。前端与现网一样 **并行两次请求**（一次 count、一次 list）。
- `limit` 最大 500，超限参数错误。前端 `usePage()` 默认 10/20，`limit-list` `[10, 20, 50, 100]`。

### 2.2 可查询字段（仅这些能进 rules）

| field | 类型 | 说明 |
|-------|------|------|
| `id` | string | 预付费账单 ID |
| `uuid` | string | 外部唯一标识 |
| `order_year` | int | 订单年份 |
| `order_month` | int | 订单月份 |
| `vendor` | string | 云厂商 |
| `root_account_id` | string | 一级账号 ID |
| `main_account_id` | string | 二级账号 ID |
| `root_account_cloud_id` | string | 一级账号云上 ID |
| `main_account_cloud_id` | string | 二级账号云上 ID |
| `product_id` | int | 运营产品 ID |
| `resource_id` | string | 资源 ID |
| `invoice_id` | string | 发票 ID |
| `gpu_type` | string | GPU 型号 |
| `device_num` | int | 数量（台） |
| `card_num` | int | 数量（卡） |
| `product_name` | string | 产品名称 |
| `product_spec` | string | 产品规格 |
| `region` | string | 地域 |
| `usage_start_at` | string | 使用开始时间 `2006-01-02 15:04:05` |
| `usage_end_at` | string | 使用结束时间 `2006-01-02 15:04:05` |
| `order_at` | string | 订单时间 `2006-01-02 15:04:05` |
| `currency` | string | 币种 `RMB` / `USD` |
| `cost` | string | 优惠后总价 |
| `rmb_cost` | string | 优惠后总价人民币金额 |
| `settle_state` | string | 定账状态 `unsettled` / `settled` |
| `creator` | string | 创建者 |
| `reviser` | string | 更新者 |
| `created_at` | string | 创建时间 RFC3339 |
| `updated_at` | string | 修改时间 RFC3339 |

**不能**作为 filter field（响应才有）：`accounting_state`、`accounted_cost`、`accounted_rmb_cost`。

### 2.3 Search → filter 序列化

界面选名称，请求只传 ID。非法起止（开始日晚于结束日、开始月晚于结束月）不发请求。全空 `rules: []`。

| 筛选项 | UI | 请求 | 转换规则 |
|--------|----|------|----------|
| 使用时间 | 日期起止（只到日） | `usage_end_at` `gte` 开始日 `00:00:00`；`usage_start_at` `lte` 结束日 `23:59:59` | **区间重叠**（与官方示例「年内使用」相同），不是「完全落在区间内」。时间值走 RFC3339 |
| 运营产品 | 名称下拉 | `product_id` `in` `[id]` | 候选见 §4 |
| 二级账号 | 名称下拉 | `main_account_id` `in` `[id]` | 传名称会参数错误 |
| 一级账号 | 名称下拉 | `root_account_id` `in` `[id]` | 同上 |
| GPU 型号 | 文本 | `gpu_type` `eq` 或 `cs` | 默认 `eq` |
| 产品名称 | 文本 | `product_name` `cs` | 包含 |
| 云厂商 | 枚举 | `vendor` `in` `[vendor]` | `VendorEnum` |
| 订单月份 | **月份区间** | `order_at` `gte` / `lte` | 见下方，**不要**再用 `order_year`+`order_month` 拼区间 |

无单独的「订单时间」筛选项。高级筛选「订单月份」UI 仍是起止月份；**传参改走 `order_at`**。

#### 订单月份 UI → `order_at`

| UI | 请求 |
|----|------|
| 开始月 `YYYY-MM` | `{ field: "order_at", op: "gte", value: "YYYY-MM-01T00:00:00+08:00" }` |
| 结束月 `YYYY-MM` | `{ field: "order_at", op: "lte", value: "<该月最后一天>T23:59:59+08:00" }` |

只选单月：起止都是该月，仍发两条 `order_at`。不要把月份区间展开成多组 `order_year`/`order_month`（会占满 10 条 rules）。`order_year` / `order_month` 仅当需要精确 EQ 单月且不走 UI 区间时才用；本期 Search 不走它们。

#### 列头漏斗（当前页本地筛选，不进接口）

核算状态、单据状态两列漏斗：**只过滤当前页 `details`**，不写进 `filter.rules`，不重发 list。换页 / 重新 Search 后，漏斗作用在新的当前页上。

| 列 | 本地过滤 field | 选项 |
|----|----------------|------|
| 核算状态 | `accounting_state` | `pending` / `accounting` / `accounted` |
| 单据状态 | `settle_state` | `unsettled` / `settled`；（UI 可有「失败」，当前页无该值则筛空） |

漏斗清空后恢复当前页全量。分页 `count` 仍是接口总数，不受漏斗影响。

## 3. 响应

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 0,
    "details": []
  }
}
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `code` | int | 状态码 |
| `message` | string | 请求信息 |
| `data.count` | int | 总数，仅 `page.count=true` 时有值 |
| `data.details` | array | 列表项，`count=false` 时返回 |

### 3.1 `details[n]` 与 22 列

| 列 | JSON | 说明 |
|----|------|------|
| 预付费 ID | `id` | 详情路由用 |
| 订单月份 | `order_at` | 展示 `YYYY-MM`（与 Search 同一字段；列 id 仍为 `order_month`） |
| 资源 ID | `resource_id` | |
| 运营产品 | `product_id` | 名称见 §3.4 |
| 二级账号名称 | `main_account_id` | 名称见 §3.4 |
| 一级账号名称 | `root_account_id` | 名称见 §3.4 |
| GPU 型号 | `gpu_type` | |
| 数量(台) | `device_num` | |
| 数量(卡) | `card_num` | |
| 优惠后总价(不含税) | `cost` | 字符串；不脱敏。另有 `rmb_cost` 本期列表不单开列 |
| 累计核算金额 | `accounted_cost` | 已推送调增分摊之和，不含调减。另有 `accounted_rmb_cost` |
| 核算状态 | `accounting_state` | 见 §3.2 |
| 币种 | `currency` | `RMB` / `USD` |
| 产品名称 | `product_name` | OFS 产品名，不是运营产品 |
| 产品规格 | `product_spec` | |
| Region | `region` | |
| 使用开始时间 | `usage_start_at` | `2006-01-02 15:04:05` |
| 使用结束时间 | `usage_end_at` | 同上 |
| 订单时间 | `order_at` | 同上 |
| 发票 ID | `invoice_id` | |
| 云厂商 | `vendor` | |
| 单据状态 | `settle_state` | 见 §3.3 |

其它：`uuid`、`*_cloud_id`、`creator`、`reviser`、`created_at`、`updated_at` 列表不展示。无失败原因字段。

### 3.2 核算状态 `accounting_state`

始终展示真实值，不按定账状态遮蔽。

| 展示 | 枚举 |
|------|------|
| 待核算 | `pending` |
| 核算中 | `accounting` |
| 已完成 | `accounted` |

### 3.3 单据状态 `settle_state`

| 展示 | 枚举 |
|------|------|
| 未定账 | `unsettled` |
| 已定账 | `settled` |

无第三态「失败」。

### 3.4 名称补齐

响应只有 ID。表格不在入口预补齐，对齐 permission-template：data-list 对 ID 列 slot 出领域 Value 组件（CombineRequest + store 缓存）。Search 候选与表格 by-ids 共用 §4 三个现网接口。优先：若联调发现列表已带 `*_name` 则组件可直接展示该字段。

## 4. 复用现网接口

三个接口同时服务 **Search 候选** 和 **表格名称补齐**（列表响应只有 ID）。

| 用途 | 接口 | Search | 表格 |
|------|------|------|------|
| 运营产品 | `POST /api/v1/account/operation_products/list` | 名称下拉，请求 `product_id` | `op_product_ids` by-ids |
| 二级账号 | `POST /api/v1/account/main_accounts/list` | 名称下拉，请求 `main_account_id`；接口自带行级授权 | `id in` by-ids |
| 一级账号 | **不调** `root_accounts/list` | 已授权 `main_accounts` 去重 `parent_account_id/name`，请求 `root_account_id` | 读同一份 parent 缓存 |

数据源在 `src/store/prepaid-bill/` 内 Search 与表格共享缓存。不改 `src/api/bill/index.ts`，不调用 `useBillStore`，不调 `root_accounts/list`。`parent_account_id` 即一级账号 ID。运营产品请求字段 `op_product_id` → 预付费 filter 用 `product_id`。入口与列表接口对齐 `main_account_find`，不用 `account_bill_find`。

## 5. 错误码与前端行为

| 场景 | 前端 |
|------|------|
| `code=0` 且空列表 | 列表空态（无授权也是空态，不是申请页） |
| 单页 >500 / 筛选非法 / 解 body 失败 | 提示错误，保留当前筛选，不清空成全量 |
| 其它失败 | 提示可重试，保留上次成功结果或空表 |
| 无 `main_account_find`（二级账号查看） | 菜单 `checkAuth` 隐藏（对齐云账单）；直链可进页、不跳 `/403`；列表接口自行鉴权，失败按「其它失败」提示 |

参数错误对应 `2000001 InvalidParameter`；解失败 `2000004`；内部异常 `2000006`。

## 6. 调用示例

官方示例：2026 年内使用、云厂商 aws、指定二级账号、订单月份 2026-07（EQ 年/月）。本期 Search 的「订单月份」区间**不采用**这种 `order_year`+`order_month`，而用 `order_at` 转换；使用时间重叠规则与此示例相同。

```json
{
  "filter": {
    "op": "and",
    "rules": [
      { "field": "usage_end_at", "op": "gte", "value": "2026-01-01 00:00:00" },
      { "field": "usage_start_at", "op": "lte", "value": "2026-12-31 23:59:59" },
      { "field": "vendor", "op": "in", "value": ["aws"] },
      { "field": "main_account_id", "op": "in", "value": ["00000001"] },
      { "field": "order_year", "op": "eq", "value": 2026 },
      { "field": "order_month", "op": "eq", "value": 7 }
    ]
  },
  "page": { "count": false, "start": 0, "limit": 10 }
}
```

本期 Search 等价写法（订单月份 2026-07～2026-09 → `order_at`）：

```json
{
  "filter": {
    "op": "and",
    "rules": [
      { "field": "usage_end_at", "op": "gte", "value": "2026-08-01T00:00:00+08:00" },
      { "field": "usage_start_at", "op": "lte", "value": "2026-08-01T23:59:59+08:00" },
      { "field": "vendor", "op": "in", "value": ["aws"] },
      { "field": "main_account_id", "op": "in", "value": ["<ACCOUNT_ID>"] },
      { "field": "order_at", "op": "gte", "value": "2026-07-01T00:00:00+08:00" },
      { "field": "order_at", "op": "lte", "value": "2026-09-30T23:59:59+08:00" }
    ]
  },
  "page": { "count": false, "start": 0, "limit": 20 }
}
```

count 请求：

```json
{
  "filter": { "op": "and", "rules": [] },
  "page": { "count": true, "start": 0, "limit": 0 }
}
```

## 7. 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 0,
    "details": [
      {
        "id": "00000001",
        "uuid": "ofs-uuid-0001",
        "order_year": 2026,
        "order_month": 7,
        "vendor": "aws",
        "root_account_id": "00000001",
        "main_account_id": "00000002",
        "root_account_cloud_id": "123456789012",
        "main_account_cloud_id": "210987654321",
        "product_id": 3456,
        "resource_id": "res-0001",
        "invoice_id": "inv-0001",
        "gpu_type": "H100",
        "device_num": 2,
        "card_num": 16,
        "product_name": "GPU 裸金属",
        "product_spec": "8*H100",
        "region": "us-east-1",
        "usage_start_at": "2026-08-01 00:00:00",
        "usage_end_at": "2026-10-31 23:59:59",
        "order_at": "2026-07-20 10:00:00",
        "currency": "USD",
        "cost": "3000",
        "rmb_cost": "21000",
        "settle_state": "unsettled",
        "accounting_state": "accounting",
        "accounted_cost": "1000",
        "accounted_rmb_cost": "7000",
        "creator": "ofs",
        "reviser": "ofs",
        "created_at": "2026-07-20T10:00:00Z",
        "updated_at": "2026-07-20T10:00:00Z"
      }
    ]
  }
}
```

## 8. Mock

已去掉。列表与账号/产品下拉全部走真实接口，不再保留 `USE_PREPAID_BILL_MOCK` 与 mock.ts。

## 9. 与 Design / Coding 的交叉（HTTP 以本文为准）

- 表头排序：UI 可点的列把 `sort`/`order` 写入 list 请求；默认订单时间倒序。
- 列头漏斗（核算 / 单据）：UI 可挂；**按当前页数据本地筛选，先不按接口**。
- 订单月份：UI 月份区间 → `order_at` 起止时间。
- **Coding 目录硬约束**：views 打平为 `src/views/prepaid-bill/`；store 同步打平为 `src/store/prepaid-bill/`（HTTP 写在 store 内）。不要放进 `src/views/bill/`，不要改 `src/api/bill/index.ts`。单类型无 Factory。禁止沿用 `src/views/bill/bill/**` TSX 骨架。
