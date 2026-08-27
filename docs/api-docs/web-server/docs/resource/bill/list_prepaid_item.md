
### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：二级账号查看。
- 该接口功能描述：查询预付费账单列表。

### URL

POST /api/v1/account/bills/prepaid_items/list

### 输入参数

| 参数名称   | 参数类型   | 必选 | 描述                     |
|--------|--------|----|------------------------|
| filter | object | 是  | 查询过滤条件                 |
| page   | object | 是  | 分页设置，单页上限 500，超限返回参数错误 |

#### filter

| 参数名称  | 参数类型        | 必选 | 描述                                                              |
|-------|-------------|----|-----------------------------------------------------------------|
| op    | enum string | 是  | 操作符（枚举值：and、or）。如果是and，则表示多个rule之间是且的关系；如果是or，则表示多个rule之间是或的关系。 |
| rules | array       | 是  | 过滤规则，最多设置10个rules。如果rules为空数组，op（操作符）将没有作用，代表查询全部数据。            |

#### rules[n] （详情请看 rules 表达式说明）

| 参数名称  | 参数类型        | 必选 | 描述                                          |
|-------|-------------|----|---------------------------------------------|
| field | string      | 是  | 查询条件Field名称，具体可使用的用于查询的字段及其说明请看下面 - 查询参数介绍  |
| op    | enum string | 是  | 操作符（枚举值：eq、neq、gt、gte、le、lte、in、nin、cs、cis） |
| value | 可变类型        | 是  | 查询条件Value值                                  |

#### 查询参数介绍：

| 参数名称                  | 参数类型   | 描述                                       |
|-----------------------|--------|------------------------------------------|
| id                    | string | 预付费账单 ID                                 |
| uuid                  | string | 外部唯一标识                              |
| order_year            | int    | 订单年份                                     |
| order_month           | int    | 订单月份                                     |
| vendor                | string | 云厂商                                      |
| root_account_id       | string | 一级账号 ID                |
| main_account_id       | string | 二级账号 ID                 |
| root_account_cloud_id | string | 一级账号云上 ID                                |
| main_account_cloud_id | string | 二级账号云上 ID                                |
| product_id            | int    | 运营产品 ID                                  |
| resource_id           | string | 资源 ID                                    |
| invoice_id            | string | 发票 ID                                    |
| gpu_type              | string | GPU 型号                                   |
| device_num            | int    | 数量（台）                                    |
| card_num              | int    | 数量（卡）                                    |
| product_name          | string | 产品名称                                     |
| product_spec          | string | 产品规格                                     |
| region                | string | 地域                                       |
| usage_start_at        | string | 使用开始时间，格式 2006-01-02 15:04:05            |
| usage_end_at          | string | 使用结束时间，格式 2006-01-02 15:04:05            |
| order_at              | string | 订单时间，格式 2006-01-02 15:04:05              |
| currency              | string | 币种 RMB/USD                               |
| cost                  | string | 优惠后总价                              |
| rmb_cost              | string | 优惠后总价的人民币金额                              |
| settle_state          | string | 定账状态 unsettled/settled                   |
| creator               | string | 创建者                                      |
| reviser               | string | 更新者                                      |
| created_at            | string | 创建时间，标准格式：2006-01-02T15:04:05Z           |
| updated_at            | string | 修改时间，标准格式：2006-01-02T15:04:05Z           |

#### page

| 参数名称  | 参数类型   | 必选 | 描述                                   |
|-------|--------|----|--------------------------------------|
| count | bool   | 是  | 是否只返回总数，为 true 时 start 与 limit 必须为 0 |
| start | uint32 | 否  | 记录开始位置                               |
| limit | uint32 | 否  | 每页限制条数，最大 500                        |

### 调用示例

查询 2026 年内使用、云厂商为 aws、订单月份为 2026-07 的预付费账单：

```json
{
  "filter": {
    "op": "and",
    "rules": [
      {
        "field": "usage_end_at",
        "op": "gte",
        "value": "2026-01-01 00:00:00"
      },
      {
        "field": "usage_start_at",
        "op": "lte",
        "value": "2026-12-31 23:59:59"
      },
      {
        "field": "vendor",
        "op": "in",
        "value": ["aws"]
      },
      {
        "field": "main_account_id",
        "op": "in",
        "value": ["00000001"]
      },
      {
        "field": "order_year",
        "op": "eq",
        "value": 2026
      },
      {
        "field": "order_month",
        "op": "eq",
        "value": 7
      }
    ]
  },
  "page": {
    "count": false,
    "start": 0,
    "limit": 10
  }
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 0,
    "details": [
      {
        "id": "00000001",
        "uuid": "prepaid-uuid-0001",
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
        "creator": "syncer",
        "reviser": "syncer",
        "created_at": "2026-07-20T10:00:00Z",
        "updated_at": "2026-07-20T10:00:00Z"
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

| 参数名称    | 参数类型  | 描述                              |
|---------|-------|---------------------------------|
| count   | int   | 总数，仅 page.count 为 true 时有值      |
| details | array | 列表项                             |

#### data.details[n]

| 参数名称                  | 参数类型   | 描述                                                              |
|-----------------------|--------|-----------------------------------------------------------------|
| id                    | string | 预付费账单 ID                                                        |
| uuid                  | string | 调用方外部唯一标识                                                     |
| order_year            | int    | 订单年份                                                            |
| order_month           | int    | 订单月份                                                            |
| vendor                | string | 云厂商                                                             |
| root_account_id       | string | 一级账号 ID                                                         |
| main_account_id       | string | 二级账号 ID                                                         |
| root_account_cloud_id | string | 一级账号云上 ID                                                       |
| main_account_cloud_id | string | 二级账号云上 ID                                                       |
| product_id            | int    | 运营产品 ID                                                         |
| resource_id           | string | 资源 ID                                                           |
| invoice_id            | string | 发票 ID                                                           |
| gpu_type              | string | GPU 型号                                                          |
| device_num            | int    | 数量（台）                                                           |
| card_num              | int    | 数量（卡）                                                           |
| product_name          | string | 产品名称                                                            |
| product_spec          | string | 产品规格                                                            |
| region                | string | 地域                                                              |
| usage_start_at        | string | 使用开始时间，格式 2006-01-02 15:04:05，与写入格式一致                          |
| usage_end_at          | string | 使用结束时间，格式 2006-01-02 15:04:05，与写入格式一致                          |
| order_at              | string | 订单时间，格式 2006-01-02 15:04:05，与写入格式一致                            |
| currency              | string | 币种 RMB/USD                                                      |
| cost                  | string | 优惠后总价（不含税）                                                      |
| rmb_cost              | string | 优惠后总价的人民币金额                                                     |
| settle_state          | string | 定账状态 unsettled/settled                                          |
| accounting_state      | string | 核算状态 pending/accounting/accounted               |
| accounted_cost        | string | 累计核算金额，为已推送调增条目的分摊金额之和，不含调减        |
| accounted_rmb_cost    | string | 累计核算金额的人民币值                                                     |
| creator               | string | 创建者                                                             |
| reviser               | string | 更新者                                                             |
| created_at            | string | 创建时间，标准格式：2006-01-02T15:04:05Z                                  |
| updated_at            | string | 修改时间，标准格式：2006-01-02T15:04:05Z                                  |

