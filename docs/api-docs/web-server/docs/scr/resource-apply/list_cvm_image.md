### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：无。
- 该接口功能描述：CVM下的镜像列表查询。

### URL

POST /api/v1/woa/config/findmany/config/cvm/image

### 输入参数

| 参数名称 | 参数类型      | 必选 | 描述     |
|--------|--------------|------|---------|
| region | string array | 是   | 地域     |

### 调用示例

#### 获取详细信息请求参数示例

```json
{
  "region": ["ap-shanghai"]
}
```

### 响应示例

#### 获取详细信息返回结果示例

```json
{
  "result":true,
  "code":0,
  "message":"success",
  "data":{
    "count":1,
    "info":[
      {
        "image_id": "img-evitcbqz",
        "image_name": "Tencent Linux release 2.6 (Final)",
        "is_recommended": true
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型       | 描述               |
|------------|--------------|--------------------|
| result     | bool         | 请求成功与否。true:请求成功；false请求失败 |
| code       | int          | 错误编码。 0表示success，>0表示失败错误  |
| message    | string       | 请求失败返回的错误信息 |
| data	     | object array | 响应数据             |

#### data

| 参数名称 | 参数类型       | 描述                    |
|---------|--------------|-------------------------|
| count   | int          | 当前规则能匹配到的总记录条数 |
| info    | object array | 镜像信息列表              |

#### data.info

| 参数名称        | 参数类型   | 描述       |
|----------------|----------|-----------|
| image_id       | string   | 镜像ID     |
| image_name     | string   | 镜像名称    |
| is_recommended | bool     | 是否推荐镜像 |
