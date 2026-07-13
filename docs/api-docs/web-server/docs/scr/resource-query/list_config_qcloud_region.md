### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：无。
- 该接口功能描述：qcloud地域配置信息查询。

### URL

GET /api/v1/woa/config/find/config/qcloud/region

### 输入参数

无

### 调用示例

无

### 响应示例

#### 获取详细信息返回结果示例

```json
{
  "result":true,
  "code":0,
  "message":"success",
  "data":{
    "count":2,
    "info":[
      {
        "id":1,
        "region":"ap-shanghai",
        "region_cn":"华东区域(上海)",
        "is_recommended": true
      },
      {
        "id":2,
        "region":"ap-guangzhou",
        "region_cn":"华南地区(广州)"
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
| info    | object array | 区域配置信息详情列表        |

#### data.info

| 参数名称        | 参数类型  | 描述         |
|----------------|---------|--------------|
| id	         | int	   | 可用区配置信息实例ID，系统内部管理ID |
| region         | string  | 可用区，英文   |
| region_cn      | string  | 可用区，中文   |
| is_recommended | bool    | 是否推荐地域   |
