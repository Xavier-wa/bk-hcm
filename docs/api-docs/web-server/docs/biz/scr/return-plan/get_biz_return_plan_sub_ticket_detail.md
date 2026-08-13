### 描述

- 该接口提供版本：v1.9.2.11+。
- 该接口所需权限：业务访问。
- 该接口功能描述：获取退回计划子单据详情。子单与 CRP 退回计划单据一一对应，本接口返回子单的流转状态与关联的 CRP 单号信息。

### URL

GET /api/v1/woa/bizs/{bk_biz_id}/plans/returns/sub_tickets/{id}

### 路径参数

| 参数名称      | 参数类型   | 必选 | 描述          |
|-----------|--------|----|-------------|
| bk_biz_id | int    | 是  | 业务 ID       |
| id        | string | 是  | 退回计划子单 ID   |

### 输入参数

无

### 调用示例

无

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "00000101",
    "ticket_id": "00000001",
    "sub_ticket_type": "add",
    "sub_ticket_type_name": "新增",
    "status": "done",
    "status_name": "成功",
    "obs_project": "常规项目",
    "resource_pool_name": "自研池",
    "crp_sn": "RT1202506301453088428",
    "crp_url": "http://crp/order/RT1202506301453088428",
    "message": "",
    "submitted_at": "2019-07-29 11:57:20",
    "created_at": "2019-07-29 11:57:20",
    "updated_at": "2019-07-29 11:57:20"
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

| 参数名称                 | 参数类型   | 描述                                                |
|----------------------|--------|---------------------------------------------------|
| id                   | string | 退回计划子单ID                                          |
| ticket_id            | string | 所属退回计划主单ID                                        |
| sub_ticket_type      | string | 子单类型（枚举值：add(新增，对应 CRP 新增单)、cancel(删除，对应 CRP 删除单)） |
| sub_ticket_type_name | string | 子单类型名称                                            |
| status               | string | 子单状态（枚举值：init, auditing, rejected, revoked, invalid, done, failed, terminated） |
| status_name          | string | 子单状态名称                                            |
| obs_project          | string | OBS项目类型                                           |
| resource_pool_name   | string | 资源池                                               |
| crp_sn               | string | CRP退回计划单号                                         |
| crp_url              | string | CRP退回计划单链接                                        |
| message              | string | 子单处理信息，失败时返回失败原因                                  |
| submitted_at         | string | 提单时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30 |
| created_at           | string | 创建时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30 |
| updated_at           | string | 更新时间，格式为YYYY-MM-DD HH:MM:SS，例如2024-01-01 13:59:30 |
