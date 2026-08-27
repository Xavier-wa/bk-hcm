
### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：二级账号查看。
- 该接口功能描述：查询预付费账单的整合明细。

### URL

POST /api/v1/account/bills/prepaid_items/{id}/split_items/list

### 路径参数

| 参数名称 | 参数类型   | 必选 | 描述                          |
|------|--------|----|-----------------------------|
| id   | string | 是  | 预付费账单 ID |

### 输入参数

无。仅需路径参数，请求体可为空。

### 调用示例

```
POST /api/v1/account/bills/prepaid_items/00000001/split_items/list
```

### 响应示例

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
        "memo": "预付费账单一次性扣减，uuid: prepaid-uuid-0001"
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int    | 状态码  |
| message | string | 请求信息 |
| data    | object | 响应数据 |

#### data

| 参数名称    | 参数类型   | 描述     |
|---------|--------|--------|
| count   | uint64 | 分摊行总数  |
| details | array  | 月度分摊行 |

#### data.details[n]

| 参数名称          | 参数类型   | 描述                                     |
|---------------|--------|----------------------------------------|
| adjustment_id | string | 调账编号                                   |
| bill_year     | int    | 账期年份                                   |
| bill_month    | int    | 账期月份                                   |
| accounted     | bool   | 行级核算状态，该条 push_status 为 pushed 时为 true |
| type          | string | 调账类型 increase/decrease                 |
| cost          | string | 调账金额，一律为正数，正负语义由 type 承载               |
| rmb_cost      | string | 调账金额的人民币值                              |
| currency      | string | 币种 RMB/USD                             |
| res_class     | string | 资源类别                                   |
| res_sub_class | string | 资源子类                                   |
| push_status   | string | 推送状态 unpushed/pushing/pushed/failed    |
| settle_state  | string | 定账状态 unsettled/settled                 |
| memo          | string | 备注                                     |

