### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：预付费账单同步（二级账号实例级）。
- 该接口功能描述：批量推送预付费账单主数据与逐月分摊明细。

### URL

POST /api/v1/account/bills/prepaid_items/sync

### 输入参数

| 参数名称  | 参数类型               | 必选 | 描述                    |
|-------|--------------------|----|-----------------------|
| items | prepaid_item 数组    | 是  | 预付费账单列表，条数取值 1-100    |

### prepaid_item

| 参数名称                  | 参数类型                    | 必选 | 描述                              |
|-----------------------|-------------------------|----|---------------------------------|
| uuid                  | string                  | 是  | 外部唯一标识，与订单年月共同构成业务唯一键           |
| order_year            | int                     | 是  | 订单年份                            |
| order_month           | int                     | 是  | 订单月份，取值 1-12                    |
| vendor                | string                  | 是  | 云厂商                             |
| root_account_cloud_id | string                  | 是  | 一级账号的云上账号 ID，须与二级账号所属一级账号一致     |
| main_account_cloud_id | string                  | 是  | 二级账号的云上账号 ID                    |
| resource_id           | string                  | 否  | 资源 ID                           |
| invoice_id            | string                  | 否  | 发票 ID                           |
| gpu_type              | string                  | 是  | GPU 型号                          |
| device_num            | int                     | 否  | 数量（台），传入时须大于 0                  |
| card_num              | int                     | 否  | 数量（卡），传入时须大于 0                  |
| product_name          | string                  | 否  | 产品名称                            |
| product_spec          | string                  | 否  | 产品规格                            |
| region                | string                  | 否  | 地域                              |
| usage_start_at        | string                  | 否  | 使用开始时间，格式 `YYYY-MM-DD HH:mm:ss` |
| usage_end_at          | string                  | 否  | 使用结束时间，格式 `YYYY-MM-DD HH:mm:ss` |
| order_at              | string                  | 是  | 订单时间，格式 `YYYY-MM-DD HH:mm:ss`   |
| currency              | string                  | 是  | 币种，枚举值（CNY、USD），须与二级账号所属汇总账号币种一致 |
| cost                  | string                  | 是  | 优惠后总价（不含税），须大于 0                |
| split_items           | prepaid_split_item 数组   | 是  | 逐月分摊明细                          |

### prepaid_split_item

| 参数名称          | 参数类型   | 必选 | 描述                                        |
|---------------|--------|----|-------------------------------------------|
| bill_year     | int    | 是  | 分摊账期年份                                    |
| bill_month    | int    | 是  | 分摊账期月份，取值 1-12                            |
| cost          | string | 是  | 该月分摊额，N 条合计须严格等于主数据 cost            |
| memo          | string | 否  | 备注                                        |

### 调用示例

```json
{
  "items": [
    {
      "uuid": "prepaid-mock-n1",
      "order_year": 2026,
      "order_month": 7,
      "vendor": "aws",
      "root_account_cloud_id": "123456789012",
      "main_account_cloud_id": "123456789012",
      "gpu_type": "H100",
      "device_num": 1,
      "card_num": 8,
      "product_name": "SageMaker Training Plan",
      "region": "us-east-1",
      "usage_start_at": "2026-08-01 00:00:00",
      "usage_end_at": "2026-08-31 23:59:59",
      "order_at": "2026-07-15 10:00:00",
      "currency": "USD",
      "cost": "1000.0000000000",
      "split_items": [
        {
          "bill_year": 2026,
          "bill_month": 8,
          "cost": "1000.0000000000"
        }
      ]
    }
  ]
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "results": [
      {
        "uuid": "prepaid-mock-n1",
        "order_year": 2026,
        "order_month": 7,
        "id": "00000001",
        "success": true,
        "message": ""
      },
      {
        "uuid": "prepaid-mock-n2",
        "order_year": 2026,
        "order_month": 7,
        "id": "",
        "success": false,
        "message": "prepaid item has been settled, please use a new uuid or order month, id: 00000002, uuid: prepaid-mock-n2"
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

| 参数名称    | 参数类型                | 描述                     |
|---------|---------------------|------------------------|
| results | sync_result 数组      | 逐单写入结果，顺序与请求 items 一致  |

#### sync_result

| 参数名称        | 参数类型   | 描述                          |
|-------------|--------|-----------------------------|
| uuid        | string | 外部唯一标识，与请求一致                |
| order_year  | int    | 订单年份，与请求一致                  |
| order_month | int    | 订单月份，与请求一致                  |
| id          | string | 预付费账单 ID，该单失败时为空串           |
| success     | bool   | 该单是否写入成功                    |
| message     | string | 该单的失败原因，成功时为空串              |
