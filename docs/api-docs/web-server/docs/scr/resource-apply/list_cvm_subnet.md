### 描述

- 该接口提供版本：v1.6.1+。
- 该接口所需权限：无。
- 该接口功能描述：私有子网subnet列表查询。可用IP取自CRP `getRealSubnetInfo.leftIpNum`，与生产调度选子网口径一致；CRP失败或未查询时为null，CRP明确返回0时为0。

### URL

POST /api/v1/woa/config/findmany/config/cvm/subnet

### 输入参数

| 参数名称 | 参数类型 | 必选 | 描述     |
|--------|---------|------|---------|
| region | string  | 是   | 地域     |
| zone   | string  | 是   | 可用区   |
| vpc    | string  | 是   | 私有网络  |

### 调用示例

#### 获取详细信息请求参数示例

```json
{
  "region": "ap-shanghai",
  "zone": "ap-shanghai-2",
  "vpc": "vpc-2x7lhtse"
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
        "id": "00000001",
        "region": "ap-shanghai",
        "zone": "ap-shanghai-2",
        "vpc_id": "vpc-2x7lhtse",
        "vpc_name": "VPC-IEG-SH",
        "subnet_id": "subnet-ax907buf",
        "subnet_name": "cvm_use_199",
        "enable": true,
        "comment": "",
        "available_ip_count": 123
      }
    ]
  }
}
```

**注意：** CRP 失败或未查询时，`available_ip_count` 为 `null`（不是 `0`）。CRP 明确剩余 0，或 CRP 成功但该子网未命中时，为数字 `0`。

### 响应参数说明

| 参数名称    | 参数类型       | 描述               |
|------------|--------------|--------------------|
| result     | bool         | 请求成功与否。true:请求成功；false请求失败 |
| code       | int          | 错误编码。 0表示success，>0表示失败错误  |
| message    | string       | 请求失败返回的错误信息 |
| data	     | object       | 响应数据             |

#### data

| 参数名称 | 参数类型       | 描述                    |
|---------|--------------|-------------------------|
| count   | int          | 当前规则能匹配到的总记录条数 |
| info    | object array | 私有子网信息列表           |

#### data.info

| 参数名称      | 参数类型   | 描述        |
|--------------|----------|-------------|
| id           | string   | 子网自增ID   |
| region       | string   | 地域         |
| zone         | string   | 可用区       |
| vpc_id       | string   | VPC ID      |
| vpc_name     | string   | VPC名       |
| subnet_id    | string   | 私有子网云ID |
| subnet_name  | string   | 私有子网名称  |
| enable       | bool     | 是否启用     |
| comment      | string   | 备注        |
| available_ip_count | int / null | 剩余可用IP数，取自CRP leftIpNum；CRP失败或未查询时为null，明确剩余0时为0 |
