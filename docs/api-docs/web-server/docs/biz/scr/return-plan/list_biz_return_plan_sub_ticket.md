### 描述

- 该接口提供版本：v1.9.2.11+。
- 该接口所需权限：业务访问。
- 该接口功能描述：业务视角查询退回计划单据对应的子单据。子单与 CRP 退回计划单据一一对应。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/returns/sub_tickets/list

### 输入参数

| 参数名称             | 参数类型         | 必选 | 描述                     |
|------------------|--------------|----|------------------------|
| ticket_id        | string       | 是  | 退回计划申请主单据ID            |
| statuses         | string array | 否  | 子单据状态列表，不传时查询全部，最多传20个 |
| sub_ticket_types | string array | 否  | 子单据类型列表，不传时查询全部，最多传20个 |
| page             | object       | 是  | 分页设置                   |

#### page

| 参数名称  | 参数类型   | 必选 | 描述                                                                                                                                                                                                        |
|-------|--------|----|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| count | bool   | 是  | 是否返回总记录条数。 如果为true，查询结果返回总记录条数 count，但不返回查询结果详情数据，此时 start 和 limit 参数将无效，且必需设置为0。如果为false，则根据 start 和 limit 参数，返回查询结果详情数据，但不返回总记录条数 count                                                                 |
| start | int    | 否  | 记录开始位置，start 起始值为0                                                                                                                                                                                        |
| limit | int    | 否  | 每页限制条数，最大500，不能为0                                                                                                                                                                                         |
| sort  | string | 否  | 排序字段，返回数据将按该字段进行排序，默认根据submitted_at(提单时间)倒序排序，枚举值为：submitted_at(提单时间)、created_at(创建时间)、updated_at(更新时间)                                                                                                    |
| order | string | 否  | 排序顺序，枚举值：ASC(升序)、DESC(降序)                                                                                                                                                                                 |

### 调用示例

```json
{
  "ticket_id": "00000001",
  "statuses": [
    "init",
    "auditing",
    "done",
    "failed"
  ],
  "sub_ticket_types": [
    "add",
    "cancel"
  ],
  "page": {
    "count": false,
    "start": 0,
    "limit": 500
  }
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "details": [
      {
        "id": "00000101",
        "status": "done",
        "status_name": "成功",
        "sub_ticket_type": "add",
        "sub_ticket_type_name": "新增",
        "obs_project": "常规项目",
        "resource_pool_name": "自研池",
        "crp_sn": "RT1202506301453088428",
        "crp_url": "http://crp/order/RT1202506301453088428",
        "message": "",
        "submitted_at": "2019-07-29 11:57:20",
        "created_at": "2019-07-29 11:57:20",
        "updated_at": "2019-07-29 11:57:20"
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述                        |
|---------|--------|---------------------------|
| code    | int    | 错误编码。 0表示success，>0表示失败错误 |
| message | string | 请求失败返回的错误信息               |
| data	   | object | 响应数据                      |

#### data

| 参数名称    | 参数类型         | 描述                                       |
|---------|--------------|------------------------------------------|
| count   | int          | 当前规则能匹配到的总记录条数，仅在 count 查询参数设置为 true 时返回 |
| details | object array | 查询返回的数据，仅在 count 查询参数设置为 false 时返回       |

#### data.details[n]

| 参数名称                 | 参数类型   | 描述                                                |
|----------------------|--------|---------------------------------------------------|
| id                   | string | 退回计划子单ID                                          |
| status               | string | 子单状态（枚举值：init, auditing, rejected, revoked, invalid, done, failed, terminated） |
| status_name          | string | 子单状态名称                                            |
| sub_ticket_type      | string | 子单类型（枚举值：add(新增，对应 CRP 新增单)、cancel(删除，对应 CRP 删除单)） |
| sub_ticket_type_name | string | 子单类型名称                                            |
| obs_project          | string | OBS项目类型                                           |
| resource_pool_name   | string | 资源池                                               |
| crp_sn               | string | CRP退回计划单号                                         |
| crp_url              | string | CRP退回计划单链接                                        |
| message              | string | 子单处理信息，失败时返回失败原因                                  |
| submitted_at         | string | 提单时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30 |
| created_at           | string | 创建时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30 |
| updated_at           | string | 更新时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30 |
