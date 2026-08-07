### 描述

- 该接口提供版本：v1.6.0+。
- 该接口所需权限：账单管理。
- 该接口功能描述：批量创建调账明细

### URL

POST /api/v1/account/bills/adjustment_items/create

### 输入参数

| 参数名称            | 参数类型                  | 必选 | 描述                    |
|-----------------|-----------------------|----|-----------------------|
| root_account_id | string                | 是  | 所属根账号id               |
| vendor          | string                | 是  | 所属厂商                  |
| items           | adjustment_item array | 是  | 调账明细列表, min=1,max=100 |

### adjustment_item

| 参数名称            | 参数类型   | 必选 | 描述                          |
|-----------------|--------|----|-----------------------------|
| root_account_id | string | 否  | 所属根账号id                     |
| main_account_id | string | 是  | 所属主账号id                     |
| product_id      | int    | 否  | 运营产品id                      |
| bk_biz_id       | int    | 否  | 业务id                        |
| bill_year       | int    | 否  | 所属年份                        |
| bill_month      | int    | 否  | 所属月份                        |
| bill_day        | int    | 是  | 所属日期                        |
| type            | string | 是  | 调账类型 枚举值（increase、decrease） |
| res_class       | string | 是  | 资源类别 枚举值（cpu、gpu_card、gpu_api、gpu_other） |
| res_sub_class   | string | 否  | 资源子类，必填性随 res_class 变化，详见下方说明 |
| currency        | string | 是  | 币种                          |
| cost            | string | 是  | 金额                          |
| memo            | string | 否  | 备注信息                        |

### res_sub_class 说明

资源子类是单列，其语义由同一条明细的 `res_class` 决定：

| res_class   | res_sub_class 必填性 | res_sub_class 语义 | 取值范围                                                 |
|-------------|------------------|------------------|------------------------------------------------------|
| cpu         | 必须为空             | -                | 传非空值返回参数非法                                           |
| gpu_card    | 必填               | GPU 卡型           | 由卡型枚举查询接口返回，见 `list_adjustment_gpu_card.md`          |
| gpu_api     | 必填               | 大模型厂商            | 由模型厂商枚举查询接口返回，见 `list_adjustment_api_brand.md`       |
| gpu_other   | 必须为空             | -                | 传非空值返回参数非法                                           |

取值域为严格比对且区分大小写，服务端不做大小写归一、不改写请求值，落库值即请求值。
华为云的两个枚举清单均为空，因此华为云的调账明细只能使用 `cpu` 与 `gpu_other`。

### 调用示例

```json
{
  "root_account_id": "00000001",
  "vendor": "huawei",
  "items": [
    {
      "root_account_id": "00000001",
      "main_account_id": "00000001",
      "product_id": 6667,
      "bk_biz_id": 1234,
      "bill_year": 2024,
      "bill_month": 6,
      "type": "increase",
      "res_class": "cpu",
      "res_sub_class": "",
      "memo": "",
      "currency": "RMB",
      "cost": "123",
      "rmb_cost": "123"
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
    "id": "00000001"
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

| 参数名称 | 参数类型   | 描述     |
|------|--------|--------|
| id   | string | 调账明细id |

