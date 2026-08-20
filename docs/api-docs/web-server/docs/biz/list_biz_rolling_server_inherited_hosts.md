### 描述

- 该接口提供版本：v1.9.2.14+。
- 该接口所需权限：业务访问。
- 该接口功能描述：查询业务下滚服项目可继承的固资候选，按入参机型族分组返回，每族按已使用时长由长到短最多返回 5 条。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list

#### 路径参数说明

| 参数名称      | 参数类型  | 必选 | 描述                                                    |
|-----------|-------|----|-------------------------------------------------------|
| bk_biz_id | int64 | 是  | 业务 ID |

### 输入参数

| 参数名称            | 参数类型     | 必选 | 描述                                                                                   |
|-----------------|----------|----|--------------------------------------------------------------------------------------|
| region          | string   | 是  | 云地域标识，如 `ap-guangzhou`，最大长度 128                                                      |
| device_families | []string | 是  | 机型族中文名列表，如 `标准型` / `高IO型` / `大数据型` / `计算型` / `GPU型` |

### 调用示例

#### 请求参数示例

```json
{
  "bk_biz_id": 100148,
  "region": "ap-guangzhou",
  "device_families": ["标准型", "GPU型"]
}
```

### 响应示例

#### 成功返回结果示例

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "info": [
      {
        "device_family": "标准型",
        "hosts": [
          {
            "bk_asset_id": "TC241120001357",
            "bk_host_innerip": "1.1.1.1",
            "bk_cloud_inst_id": "ins-0a1b2c3d",
            "device_type": "S5.LARGE8",
            "instance_charge_type": "PREPAID",
            "billing_start_time": "2022-11-20T10:15:30+08:00",
            "billing_expire_time": "2027-05-20T10:15:30+08:00",
            "charge_months": 10,
            "is_recommended": true
          },
          {
            "bk_asset_id": "TC250305002468",
            "bk_host_innerip": "2.2.2.2",
            "bk_cloud_inst_id": "ins-4e5f6a7b",
            "device_type": "S5.2XLARGE16",
            "instance_charge_type": "PREPAID",
            "billing_start_time": "2025-03-05T09:00:00+08:00",
            "billing_expire_time": "2027-03-05T09:00:00+08:00",
            "charge_months": 7,
            "is_recommended": false
          }
        ]
      },
      {
        "device_family": "GPU型",
        "hosts": []
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述                            |
|---------|--------|-------------------------------|
| result  | bool   | 请求成功与否。true:请求成功；false:请求失败   |
| code    | int    | 错误编码。0 表示 success，>0 表示失败错误   |
| message | string | 请求失败返回的错误信息                   |
| data    | object | 响应数据                          |

#### data

| 参数名称 | 参数类型         | 描述                                    |
|------|--------------|---------------------------------------|
| info | object array | 按机型族分组的候选列表，每个入参机型族对应一项，顺序与入参 `device_families` 一致 |

#### data.info[]

| 参数名称          | 参数类型         | 描述                                                                    |
|---------------|--------------|-----------------------------------------------------------------------|
| device_family | string       | 机型族中文名，回显入参值                                                          |
| hosts         | object array | 该机型族下的候选，最多 5 条。该族无可继承机器时为空数组 `[]`（不为 `null`、不省略该分组） |

#### data.info[].hosts[]

| 参数名称                 | 参数类型   | 描述                                                                               |
|----------------------|--------|----------------------------------------------------------------------------------|
| bk_asset_id          | string | 固资号。                                                              |
| bk_host_innerip      | string | 内网 IP。                                                               |
| bk_cloud_inst_id     | string | 云主机实例 ID                                                                         |
| device_type          | string | 机型                                                                               |
| instance_charge_type | string | 实例计费模式。PREPAID：包年包月；POSTPAID_BY_HOUR：按量计费                                         |
| billing_start_time   | string | 套餐计费起始时间，RFC3339 格式                                                             |
| billing_expire_time  | string | 套餐计费到期时间，RFC3339 格式。按量计费固资无到期时间         |
| charge_months        | int    | 剩余月数（当前时间到套餐到期时间）|
| is_recommended       | bool   | 是否为推荐项。套餐计费起始时间距当前时间已满 36 个月时为 true，同一机型族下满足条件的候选均标记为推荐项 |
