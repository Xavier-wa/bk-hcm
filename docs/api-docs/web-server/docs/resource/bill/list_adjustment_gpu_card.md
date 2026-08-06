### 描述

- 该接口提供版本：v1.9.2.8+。
- 该接口所需权限：账单查看。
- 该接口功能描述：查询指定云厂商可选的调账卡型枚举，供调账明细的资源子类下拉使用。

### URL

GET /api/v1/account/vendors/{vendor}/bills/adjustment_items/gpu_cards

### 输入参数

| 参数名称   | 参数类型   | 必选 | 描述                            |
|--------|--------|----|-------------------------------|
| vendor | string | 是  | 云厂商，枚举值：aws、gcp、huawei，其余值返回参数非法 |

### 取值来源说明

| 云厂商    | 卡型清单来源                                                                          |
|--------|---------------------------------------------------------------------------------|
| aws    | `global_config` 中 `aws_gpu_instance_types` 配置的 value 集合                           |
| gcp    | 代码内置的一级卡型清单，并上 `global_config` 中 `gcp_gpu_instance_prefixes` 配置的 value 集合         |
| huawei | 无卡型来源，固定返回空列表                                                                     |

清单结果已去重并按字典序稳定排序。运营在 `global_config` 中调整配置后无需重启服务即可生效。
配置缺失时按空来源降级，配置解析失败时忽略该来源并记录警告，两种情况均不阻断接口。

### 调用示例

```
GET /api/v1/account/vendors/gcp/bills/adjustment_items/gpu_cards
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "details": [
      "A100",
      "GB200",
      "H100",
      "TPU"
    ]
  }
}
```

华为云返回空列表：

```json
{
  "code": 0,
  "message": "",
  "data": {
    "details": []
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

| 参数名称    | 参数类型         | 描述                                     |
|---------|--------------|----------------------------------------|
| details | string array | 可选卡型清单，作为 `res_class=gpu_card` 时资源子类的取值域 |
