# API — 账单调整列表状态与守卫

> 权限与现网账单调整相同：账单管理 = IAM `account_bill_find`（`account_bill_manage`）。  
> 本阶段只定义前端消费契约，**不改后端、不改 `docs/api-docs`**。  
> 单据：https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137371364  
> 列表字段以本文件为准（覆盖 TAPD 草稿里的推送「超时」）。

**不新开接口。** 查询、编辑、删除、批量确认仍走现网 `src/api/bill` 封装。预付费管理那套 `prepaid_items` **不调**。

## 1. 范围

| 能力 | 接口 | 本期 |
|------|------|------|
| 调账列表 | `POST /api/v1/account/bills/adjustment_items/list` | 复用；**消费新增响应字段** |
| 编辑 | `PATCH /api/v1/account/bills/adjustment_items/{id}` | 复用；请求体不变；以后端拒绝为准 |
| 批量删除 | `DELETE /api/v1/account/bills/adjustment_items/batch` | 复用 |
| 批量确认 | `POST /api/v1/account/bills/adjustment_items/confirm` | 复用；目标仍只能是未确认 |
| 金额汇总 | `POST /api/v1/account/bills/adjustment_items/sum` | 现网已有，不扩字段 |
| 导出 / 新建 | 现网已有 | 不改 |

落码：继续 `src/api/bill/index.ts` 的 `reqBillsAdjustmentList` / `updateBillsAdjustment` / `deleteBillsAdjustment` / `confirmBillsAdjustment`。扩展行类型（`src/typings/bill.ts` 或列表页本地类型）与常量映射。**不要**为预付费去改这些封装；**不要**新开导出或详情接口。

## 2. 列表（唯一数据源）

```http
POST /api/v1/account/bills/adjustment_items/list
```

请求与现网当月账单调整一致：`filter` + `page` 必填。本迭代 **不把新字段写进 `filter.rules`**（不搜来源/推送/定账）。现网仍按账期筛：

```json
{
  "filter": {
    "op": "and",
    "rules": [
      { "field": "bill_year", "op": "eq", "value": 2026 },
      { "field": "bill_month", "op": "eq", "value": 8 }
    ]
  },
  "page": { "count": false, "start": 0, "limit": 20 }
}
```

- 搜索仍只现网键：`product_id` / `main_account_id` / `updated_at`。
- count / list 仍走现网 `useTable` 并行两次，不另打无关接口（AC-P04）。
- `limit`、分页与现网表格一致，不为本期改上限。

### 2.1 新增响应字段（`data.details[]`）

现网已有字段（`id`、`state`、`cost`、`operator`、`product_name` 等）不变。本期**多读**下列字段；缺省按 §5 降级。

| 字段 | 类型 | 枚举 / 含义 | 列表怎么用 |
|------|------|-------------|------------|
| `source` | string | `manual` / `prepaid` | 守卫；**不开「来源」列** |
| `source_id` | string | prepaid = 预付费账单 ID；人工为空 | **不渲染、不跳转**；可留在行数据里 |
| `push_status` | string | `unpushed` / `pushing` / `pushed` / `failed` | 「推送状态」Tag |
| `push_fail_reason` | string | 失败原因；非 `failed` 为空 | 失败 Tag tooltip；**不单开列** |
| `settle_state` | string | `unsettled` / `settled` | 「定账状态」Tag |

无「超时」态。不要把预付费分摊里的 `accounted` 等核算枚举套到本列。

`state` 仍是调账确认态：`unconfirmed` / `confirmed`（现网「调账状态」列）。

### 2.2 前端映射

| `push_status` | 展示 | Tag theme |
|---------------|------|-----------|
| `unpushed` | 未推送 | 默认 |
| `pushing` | 推送中 | `info` |
| `pushed` | 已推送 | `success` |
| `failed` | 失败 | `danger` |

| `settle_state` | 展示 | Tag theme |
|----------------|------|-----------|
| `unsettled` | 未定账 | 默认 |
| `settled` | 已定账 | `success` |

空字符串 / `undefined` / 未知枚举：单元格 `--`，不出空 Tag。`failed` 且 `push_fail_reason` 非空才给失败 Tag 加 tooltip。

### 2.3 响应示例（节选）

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 2,
    "details": [
      {
        "id": "<ADJUST_ID>",
        "state": "confirmed",
        "source": "prepaid",
        "source_id": "<PREPAID_BILL_ID>",
        "push_status": "pushed",
        "push_fail_reason": "",
        "settle_state": "unsettled"
      },
      {
        "id": "<ADJUST_ID>",
        "state": "unconfirmed",
        "source": "manual",
        "source_id": "",
        "push_status": "failed",
        "push_fail_reason": "OBS 拒绝",
        "settle_state": "unsettled"
      }
    ]
  }
}
```

存量行可能没有上述新字段，按 §5 处理。

## 3. 写操作（路径与请求体不改）

前端守卫先挡（PRD 矩阵）；请求发出后若后端拒绝，**展示接口 `message`，列表不乐观改行**，再 `getListData()` 以服务端为准。

### 3.1 编辑

```http
PATCH /api/v1/account/bills/adjustment_items/{id}
```

请求体与现网编辑抽屉相同（账号、产品、类型、资源类别、金额、备注等）。**不要**把 `source` / `push_status` / `settle_state` 写进 body。

允许发出请求的行（与 PRD 一致）：`source === 'manual'` 且 `push_status !== 'pushing'` 且 `settle_state !== 'settled'`（含已确认）。

保存成功后刷新列表。人工已确认 + 已推送 + 未定账：刷新后预期为 `state=unconfirmed` 且 `push_status=unpushed`（AC-023，**以后端结果为准**，前端不本地改这两列）。

现网官方文档仍写「已确定不能编辑」。本期以后端实际拒绝为准，不把该旧句当成前端禁用条件。

### 3.2 删除

```http
DELETE /api/v1/account/bills/adjustment_items/batch
```

body：`{ "ids": ["..."] }`。单行删除也走该封装。可删条件与编辑相同（预付费 / 推送中 / 已定账不可删）。已确认人工行在非 pushing、未 settled 时可删。

### 3.3 批量确认

```http
POST /api/v1/account/bills/adjustment_items/confirm
```

body：`{ "ids": ["..."] }`。勾选条件必须同时满足：

- `state === 'unconfirmed'`（AC-025，已确认不能当确认目标）
- 且不是预付费、不是推送中（与编辑/删除同一套「不可勾选」）

已定账行也不可编辑/删除；批量确认仍只收未确认。

## 4. 前端守卫 vs 接口（对照）

| 条件 | 前端 | 接口（预期） |
|------|------|----------------|
| `source=prepaid` | 编辑/删除/勾选全禁 | 写操作拒绝 |
| `push_status=pushing` | 同上 | 写操作拒绝 |
| `settle_state=settled` | 编辑/删除禁 | 写操作拒绝 |
| `source=manual` 且非 pushing、未 settled | 可编辑/删除（含已确认） | 允许；编辑成功后可能变回未确认+未推送 |
| `source` 缺失 | 仅 `unconfirmed` 可操作 | 存量可能仍按「已确认不可改」拒绝 |
| 已确认 + 批量确认 | 不可勾选 | 拒绝已确认 ID |

不要为了「看起来成功」吞掉写失败。

## 5. 缺字段 / 错误码

| 场景 | 前端 |
|------|------|
| 新字段全部缺失 | 两列 `--`；操作按「来源缺失」= 仅未确认（AC-026） |
| 仅缺 `push_fail_reason` | 失败 Tag 无 tooltip |
| `source_id` 有值 | 忽略，不跳 `/bill/prepaid/detail/:id` |
| `code=0` | 按 `details` 渲染 |
| 参数 `2000001` / 解码 `2000004` / 不存在 `2000003` | 提示，不改行 |
| 业务拒绝（含已确认不可改、预付费不可改等）`2000001` / `2000006` | 提示 `message`，刷新列表 |
| 无账单管理权限 | 与现网一致：菜单/入口已控；本页不新做 403 |

本仓当前 Go 结构体尚未带这五个字段；联调前按缺字段降级，**不要**为对齐本文件去改 Go 或 `docs/api-docs`。

## 6. 调用时序

1. 打开 `/bill/bill-manage/adjust`：现网 list + sum，无第三套查询。
2. 编辑/删除/确认成功：`getListData()` + 清勾选（删除时现网已 `resetSelections`）。
3. 写失败：Message 错误，保留当前行，不本地改 `state` / `push_status`。

## 7. 与 Design / Coding 的交叉（HTTP 以本文为准）

- 两列只读 list 字段；Tag 文案/theme 见 Design §2。
- `source_id` 不进列。
- 守卫在 `src/views/bill/bill/adjust/index.tsx` 扩现网 `disabled` / `isCurRowSelectEnable`，不重做 Vue 列表。
- 常量映射可放 `src/constants/bill.ts`（现网 `BILL_ADJUSTMENT_STATE__MAP` 旁）。
- 类型扩展给 list 行，不要把 `source` 塞进新建 `create` body。
