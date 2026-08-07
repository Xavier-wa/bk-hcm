# 只同步调账策略的场景 - 接口契约

## 1. 接口概览

| 用途 | 方法 | 前端调用路径 | 后端路由 | 变更类型 |
|---|---|---|---|---|
| 创建同步记录 | POST | `/api/v1/account/bills/sync_records` | `/bills/sync_records` | **修改** |
| 全量同步内容预览 | POST | `/api/v1/account/bills/root_account_summarys/sum` | `/bills/summary_roots/sum` | 复用 |
| 调账数据汇总 | POST | `/api/v1/account/bills/adjustment_items/sum` | `/bills/adjustment_items/sum` | 复用 |

---

## 2. 创建同步记录（修改）

### 2.1 URL

```
POST /api/v1/account/bills/sync_records
```

### 2.2 Request

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `vendor` | string | 是 | 云厂商，如 `tcloud`、`huawei`、`aws`、`gcp`、`azure`、`zenlayer`、`baidu` |
| `bill_year` | int | 是 | 账单年份 |
| `bill_month` | int | 是 | 账单月份（1-12） |
| `sync_mode` | string | 否 | 同步模式；缺省值为 `full` |

#### sync_mode 枚举

| 取值 | 含义 | 说明 |
|---|---|---|
| `full` | 全量同步 | 同步当前云厂商全部账单数据 |
| `adjustment_only` | 仅同步调账数据 | 只同步已确认调账策略产生的调整数据 |

#### 请求示例

```json
{
  "vendor": "tcloud",
  "bill_year": 2026,
  "bill_month": 7,
  "sync_mode": "adjustment_only"
}
```

### 2.3 Response

与现有创建同步记录接口保持一致。

#### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "ids": ["sync_record_id_xxx"]
  }
}
```

### 2.4 错误码

| HTTP Code | 错误码 | 说明 |
|---|---|---|
| 400 | `InvalidParameter` | 参数校验失败，如 `sync_mode` 不在枚举范围内 |
| 400 | `DecodeRequestFailed` | 请求体解析失败 |
| 403 | `PermissionDenied` | 当前用户无账单同步创建权限 |
| 500 | - | 创建同步记录或触发核算失败 |

---

## 3. 全量同步内容预览（复用）

### 3.1 URL

```
POST /api/v1/account/bills/root_account_summarys/sum
```

### 3.2 Request

与现有一级账号账单汇总求和接口一致。

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `bill_year` | int | 是 | 账单年份 |
| `bill_month` | int | 是 | 账单月份 |
| `filter` | object | 是 | 过滤条件，必须包含 `vendor` 等于当前选中云厂商 |

#### 请求示例

```json
{
  "bill_year": 2026,
  "bill_month": 7,
  "filter": {
    "op": "AND",
    "rules": [
      { "field": "vendor", "op": "eq", "value": "tcloud" }
    ]
  }
}
```

### 3.3 Response

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.count` | int | 业务数量 |
| `data.cost_map.USD.Cost` | string | 美金原币金额 |
| `data.cost_map.USD.RMBCost` | string | 美金折算后的人民币金额 |
| `data.cost_map.USD.Currency` | string | 币种代码 |

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "count": 12,
    "cost_map": {
      "USD": {
        "Cost": "1234.56",
        "RMBCost": "8901.23",
        "Currency": "USD"
      }
    }
  }
}
```

### 3.4 前端展示映射（全量同步）

| 展示项 | 数据来源 | 计算/转换 |
|---|---|---|
| 人民币+美金（￥） | `cost_map.USD.RMBCost` | 直接展示 |
| 人民币 | 本期不单独展示 | — |
| 美金 | `cost_map.USD.Cost` | 直接展示 |
| 业务数量 | `count` | 直接展示 |

> 注：现网接口仅按 `USD` 维度返回 `Cost` 与 `RMBCost`。本期弹窗「全量同步」标签按 PRD 调整为「人民币+美金（￥）/ 美金 / 业务数量」，数值口径与现网保持一致。

---

## 4. 调账数据汇总（复用）

### 4.1 URL

```
POST /api/v1/account/bills/adjustment_items/sum
```

### 4.2 Request

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `filter` | object | 是 | 过滤条件，必须包含 `vendor`、`bill_year`、`bill_month` |

#### 请求示例

```json
{
  "filter": {
    "op": "AND",
    "rules": [
      { "field": "vendor", "op": "eq", "value": "tcloud" },
      { "field": "bill_year", "op": "eq", "value": 2026 },
      { "field": "bill_month", "op": "eq", "value": 7 }
    ]
  }
}
```

### 4.3 Response

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.count` | int | 调账记录条数 |
| `data.cost_map.increase.USD.Cost` | string | 调增美金原币 |
| `data.cost_map.increase.USD.RMBCost` | string | 调增美金折算人民币 |
| `data.cost_map.decrease.USD.Cost` | string | 调减美金原币 |
| `data.cost_map.decrease.USD.RMBCost` | string | 调减美金折算人民币 |

#### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "count": 5,
    "cost_map": {
      "increase": {
        "USD": {
          "Cost": "800.00",
          "RMBCost": "5760.00",
          "Currency": "USD"
        }
      },
      "decrease": {
        "USD": {
          "Cost": "200.00",
          "RMBCost": "1440.00",
          "Currency": "USD"
        }
      }
    }
  }
}
```

### 4.4 前端展示映射（仅同步调账数据）

| 展示项 | 数据来源 | 计算/转换 |
|---|---|---|
| 调账金额（人民币+美金￥） | `increase.USD.RMBCost - decrease.USD.RMBCost` | 调增减调减后的净额 |
| 调账金额（人民币） | 本期不单独展示 | — |
| 调账金额（美金） | `increase.USD.Cost - decrease.USD.Cost` | 调增减调减后的净额 |
| 调账记录条数 | `count` | 直接展示 |

> 注：调账金额合计为 0（或净额为 0）时，前端按 PRD 要求硬拦截，禁止发起同步。

---

## 5. 前端状态与接口调用关系

### 5.1 弹窗打开

1. 默认 `sync_mode = 'full'`。
2. 默认选中当前页面云厂商（如未指定则按现网默认逻辑）。
3. 调用「全量同步内容预览」接口获取展示数据。
4. 调用「一级账号账单汇总列表」接口校验是否全部确认（沿用现网逻辑）。

### 5.2 切换同步范围

- 切为 `full` → 调用「全量同步内容预览」接口。
- 切为 `adjustment_only` → 调用「调账数据汇总」接口。

### 5.3 切换云厂商

- 当前 `sync_mode` 不变。
- 根据当前 `sync_mode` 调用对应预览接口。
- 重置确认勾选状态。

### 5.4 点击同步

- 校验：云厂商已选、确认已勾选、全量时一级账号全部已确认、仅调账时调账金额合计 > 0。
- 请求参数包含 `sync_mode`。
- 成功后关闭弹窗并提示。

---

## 6. 待后端确认事项

| ID | 事项 | 建议 |
|---|---|---|
| Q-API-001 | `sync_mode` 字段在 `BillSyncRecordCreateReq` 中是否扩展为 enumor | 建议新增 `enumor.BillSyncMode` 枚举，前端按字符串 `"full"` / `"adjustment_only"` 提交 |
| Q-API-002 | 仅调账同步时，后端是否需要在创建同步记录后触发不同的核算/同步流程 | 由后端根据 `sync_mode` 分支处理，本期前端仅透传 |
| Q-API-003 | 调账数据汇总是否只需要已确认的调账项 | 建议 filter 中默认包含状态为已确认的条件，或后端接口默认只汇总已确认数据 |

---

## 7. 错误码汇总

| 错误码 | 触发场景 |
|---|---|
| `InvalidParameter` | `sync_mode` 非法、必填字段缺失 |
| `DecodeRequestFailed` | 请求体格式错误 |
| `PermissionDenied` | 无账单同步权限 |
| 业务错误（由后端返回） | 调账金额为 0 时后端若也做校验，可返回对应业务错误码；前端已做硬拦截 |
