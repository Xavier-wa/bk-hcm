
### 描述

- 该接口提供版本：v1.6.0+。
- 该接口所需权限：账单管理。
- 该接口功能描述：编辑调账明细，已确定的调账明细不能编辑，该接口不能确认调账明细

### URL

PATCH /api/v1/account/bills/adjustment_items/{id}

### 输入参数

| 参数名称            | 参数类型   | 必选 | 描述                          |
|-----------------|--------|----|-----------------------------|
| main_account_id | string | 否  | 所属主账号id                     |
| product_id      | int    | 否  | 运营产品id                      |
| bk_biz_id       | int    | 否  | 业务id                        |
| type            | string | 否  | 调账类型 枚举值（increase、decrease） |
| res_class       | string | 否  | 资源类别 枚举值（cpu、gpu_card、gpu_api、gpu_other） |
| res_sub_class   | string | 否  | 资源子类，必填性随 res_class 变化，详见下方说明 |
| currency        | string | 否  | 币种                          |
| cost            | string | 否  | 金额                          |
| memo            | string | 否  | 备注信息                        |

### res_sub_class 说明

资源子类是单列，其语义由该条明细更新后的 `res_class` 决定：

| res_class   | res_sub_class 必填性 | res_sub_class 语义 | 取值范围                                                 |
|-------------|------------------|------------------|------------------------------------------------------|
| cpu         | 必须为空             | -                | 非空返回参数非法                                             |
| gpu_card    | 必填               | GPU 卡型           | 由卡型枚举查询接口返回，见 `list_adjustment_gpu_card.md`          |
| gpu_api     | 必填               | 大模型厂商            | 由模型厂商枚举查询接口返回，见 `list_adjustment_api_brand.md`       |
| gpu_other   | 必须为空             | -                | 非空返回参数非法                                             |

校验规则：

- 请求未携带 `res_sub_class` 时沿用该记录已有的资源子类参与校验；携带该字段（包括传空字符串）视为显式赋值。
- **显式置空要求**：把 `res_class` 由 `gpu_card` / `gpu_api` 改为 `cpu` / `gpu_other` 时，必须在同一请求中把 `res_sub_class`
  显式传为空字符串，否则返回参数非法。服务端不会自动清空该字段。
- 取值域按记录所属云厂商判定（更新接口的 URL 不含云厂商），严格比对且区分大小写，落库值即请求值。

### 调用示例

```json
{
  "main_account_id": "0000001"
}
```

把资源类别由 `gpu_card` 改为 `cpu` 时需同时显式置空资源子类：

```json
{
  "res_class": "cpu",
  "res_sub_class": ""
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data":null
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int    | 状态码  |
| message | string | 请求信息 |
