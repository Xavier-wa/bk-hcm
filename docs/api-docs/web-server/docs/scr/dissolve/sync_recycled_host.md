### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：平台-机房裁撤管理。
- 该接口功能描述：同步裁撤主机信息。

### URL

POST /api/v1/woa/dissolve/recycled_host/sync

### 输入参数

无

### 调用示例

```json
{}
```

### 响应示例

```json
{
  "result":true,
  "code":0,
  "message":"success",
  "data": null
}
```

### 响应参数说明

| 参数名称    | 参数类型      | 描述               |
|------------|-------------|--------------------|
| result     | bool        | 请求成功与否。true:请求成功；false请求失败 |
| code       | int         | 错误编码。 0表示success，>0表示失败错误  |
| message    | string      | 请求失败返回的错误信息 |
| data	     | object      | 响应数据             |
