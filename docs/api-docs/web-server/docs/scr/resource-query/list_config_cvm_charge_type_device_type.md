### 描述

- 该接口提供版本：v1.6.11+。
- 该接口所需权限：无。
- 该接口功能描述：查询计费模式及机型配置信息。

### URL

POST /api/v1/woa/config/findmany/config/cvm/charge_type/device_type

### 输入参数

| 参数名称      | 参数类型 | 必选 | 描述                            |
|--------------|--------|-----|---------------------------------|
| bk_biz_id    | int	| 是  | CC业务ID                         |
| require_type | int	| 是  | 需求类型。1: 常规项目; 2: 春节保障; 3: 机房裁撤 |
| region       | string | 是  | 地域                             |
| zone         | string | 否  | 可用区，若为空则查询地域下所有可用区支持的机型 |
| suborder_id  | string | 否  | 主机申请子订单ID                  |

### 调用示例

#### 获取详细信息请求参数示例

```json
{
  "bk_biz_id": 3,
  "require_type": 1,
  "region": "ap-shanghai",
  "zone": "ap-shanghai-2",
  "suborder_id": "1001-1"
}
```

### 响应示例

#### 获取详细信息返回结果示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "count": 2,
    "info": [
      {
        "charge_type": "PREPAID",
        "available": false,
        "device_types": [
          {
            "device_type": "SK1.LARGE16",
            "available": false,
            "remain_core":10
          },
          {
            "device_type": "I2.2XLARGE8",
            "available": false,
            "remain_core":10
          }
        ]
      },
      {
        "charge_type": "POSTPAID_BY_HOUR",
        "available": true,
        "device_types": [
          {
            "device_type": "SK1.LARGE16",
            "available": true,
            "remain_core":10
          },
          {
            "device_type": "I2.2XLARGE8",
            "available": false,
            "remain_core":10
          }
        ]
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型         | 描述                         |
|---------|--------------|----------------------------|
| result  | bool         | 请求成功与否。true:请求成功；false请求失败 |
| code    | int          | 错误编码。 0表示success，>0表示失败错误  |
| message | string       | 请求失败返回的错误信息                |
| data	   | object array | 响应数据                       |

#### data

| 参数名称  | 参数类型         | 描述             |
|-------|--------------|----------------|
| count | int          | 当前规则能匹配到的总记录条数 |
| info  | object array | 机型详情列表         |

#### data.info

| 参数名称         | 参数类型         | 描述                                        |
|--------------|--------------|-------------------------------------------|
| charge_type	 | string       | 计费模式 (PREPAID:包年包月，POSTPAID_BY_HOUR:按量计费) |
| available    | bool         | 是否可用                                      |
| device_types | object array | 机型配置信息列表                                  |

#### data.info[i].device_types[i]

| 参数名称      | 参数类型 | 描述      |
|--------------|--------|-----------|
| device_type  | string | 机型       |
| available    | bool   | 是否可用    |
| remain_core  | int    | 剩余核心数  |
