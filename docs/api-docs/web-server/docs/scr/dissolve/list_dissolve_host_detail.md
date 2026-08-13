### 描述

- 该接口提供版本：v1.9.2.2+。
- 该接口所需权限：服务-机房裁撤。
- 该接口功能描述：分页查询裁撤主机明细。

### URL

POST /api/v1/woa/dissolve/host/detail/list

### 输入参数

| 参数名称       | 参数类型         | 必选 | 描述                                                  |
|------------|--------------|----|-----------------------------------------------------|
| bk_biz_ids | int64 array  | 否  | 业务ID列表                                              |
| project_ids | int array   | 否  | 裁撤项目ID列表                                            |
| group_ids  | int64 array  | 否  | 运维小组ID列表                                            |
| operators  | string array | 否  | 负责人列表                                       |
| modules    | string array | 否  | 裁撤模块名称列表                                            |
| inner_ips  | string array | 否  | 内网IP列表                                    |
| asset_ids  | string array | 否  | 主机固资号列表                                  |
| status     | string       | 否  | 裁撤状态，枚举值：complete（已裁撤）/incomplete（未裁撤），不传查全部       |
| expect_abolish_times | string array | 否  | 裁撤截止时间列表，格式 yyyy-MM-dd                      |
| page       | object       | 是  | 分页设置                                                |

#### page

| 参数名称  | 参数类型 | 必选 | 描述                                              |
|-------|------|----|-------------------------------------------------|
| count | bool | 是  | 是否返回总记录条数，为 true 时仅返回 count                     |
| start | uint | 否  | 记录开始位置                                          |
| limit | uint | 否  | 每页限制条数，最大 500                                   |
| sort  | string | 否 | 排序字段                                            |

### 调用示例

```json
{
  "bk_biz_ids": [100],
  "status": "incomplete",
  "page": {
    "count": false,
    "start": 0,
    "limit": 50
  }
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 1,
    "details": [
      {
        "id": "00000001",
        "asset_id": "ABC123",
        "inner_ip": "1.1.1.1",
        "device_type": "S5.LARGE",
        "module": "module-a",
        "status": "incomplete",
        "project_id": 100,
        "project_name": "2026年第一批裁撤",
        "region": "ap-guangzhou",
        "bk_biz_id": 100,
        "group_id": 200,
        "operators": ["zhangsan", "lisi"],
        "cpu_core": 64,
        "expect_abolish_time": "2026-12-31"
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int32  | 状态码  |
| message | string | 请求信息 |
| data    | object | 响应数据 |

#### data

| 参数名称    | 参数类型  | 描述     |
|---------|-------|--------|
| count   | int64 | 总记录条数  |
| details | array | 裁撤主机明细 |

#### data.details[n]

| 参数名称        | 参数类型         | 描述                                       |
|-------------|--------------|------------------------------------------|
| id          | string       | 记录ID                                     |
| asset_id    | string       | 主机固资号                                    |
| inner_ip    | string       | 内网IP                                     |
| device_type | string       | 机型                                       |
| module      | string       | 裁撤模块名称                                   |
| status      | string       | 裁撤状态，complete（已裁撤）/incomplete（未裁撤）       |
| project_id  | int          | 裁撤项目ID                                   |
| project_name | string      | 裁撤项目名称                                   |
| region      | string       | 地域ID                                     |
| bk_biz_id   | int64        | 业务ID                                     |
| group_id    | int64        | 运维小组ID                                   |
| operators   | string array | 负责人列表                                    |
| cpu_core    | int          | CPU核心数                                   |
| expect_abolish_time | string | 裁撤截止时间，格式 yyyy-MM-dd                       |
