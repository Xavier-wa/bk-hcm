### 描述

- 该接口提供版本：v1.9.2.11+。
- 该接口所需权限：业务-退回计划操作。
- 该接口功能描述：终止退回计划单据。将处于失败态的单据置为终止态，终止后不可再重试。该接口仅变更 HCM 本地单据状态，不撤回已提交至 CRP 的退回计划单据。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/returns/tickets/{ticket_id}/terminate

### 路径参数

| 参数名称      | 参数类型   | 必选 | 描述        |
|-----------|--------|----|-----------|
| bk_biz_id | int    | 是  | 业务 ID     |
| ticket_id | string | 是  | 退回计划主单 ID |

### 输入参数

无

### 调用示例

无

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": null
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述                        |
|---------|--------|---------------------------|
| code    | int    | 错误编码。 0表示success，>0表示失败错误 |
| message | string | 请求失败返回的错误信息               |
| data	   | object | 响应数据                      |

#### data

无
