### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：账单查看。
- 该接口功能描述：查询指定云厂商可选的调账大模型厂商枚举，供调账明细的资源子类下拉使用。

### URL

GET /api/v1/account/vendors/{vendor}/bills/adjustment_items/api_brands

### 输入参数

| 参数名称   | 参数类型   | 必选 | 描述                            |
|--------|--------|----|-------------------------------|
| vendor | string | 是  | 云厂商，枚举值：aws、gcp、huawei，其余值返回参数非法 |

### 取值来源说明

| 云厂商    | 模型厂商清单                                    |
|--------|-------------------------------------------|
| aws    | claude、gemini、jina、kimi                   |
| gcp    | claude、gemini、jina、kimi                   |
| huawei | 无 OBS API 资源分类，固定返回空列表                     |

清单取账单上报侧归并后的四值，`veo`、`imagen`、`lyria` 在上报侧已归并为 `gemini`，因此不出现在本清单中。

### 调用示例

```
GET /api/v1/account/vendors/aws/bills/adjustment_items/api_brands
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "details": [
      "claude",
      "gemini",
      "jina",
      "kimi"
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

| 参数名称    | 参数类型         | 描述                                    |
|---------|--------------|---------------------------------------|
| details | string array | 可选模型厂商清单，作为 `res_class=gpu_api` 时资源子类的取值域 |
