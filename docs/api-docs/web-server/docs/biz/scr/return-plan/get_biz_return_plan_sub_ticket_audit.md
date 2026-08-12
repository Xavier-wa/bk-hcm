### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：查询退回计划子单在 CRP 系统的审批流，包括审批状态、当前审批阶段与审批历史。退回计划不走 ITSM 流程，子单与 CRP 退回计划单据一一对应，本接口仅返回 CRP 审批信息。

### URL

GET /api/v1/woa/bizs/{bk_biz_id}/plans/returns/sub_tickets/{sub_ticket_id}/audit

### 路径参数

| 参数名称          | 参数类型   | 必选 | 描述          |
|---------------|--------|----|-------------|
| bk_biz_id     | int    | 是  | 业务 ID       |
| sub_ticket_id | string | 是  | 退回计划子单 ID   |

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
    "crp_audit": {
      "crp_sn": "RT1202506301453088428",
      "crp_url": "http://crp/order/RT1202506301453088428",
      "status": "auditing",
      "status_name": "审批中",
      "message": "如果失败，这里会写原因",
      "current_steps": [
        {
          "state_id": "",
          "name": "规划经理审批",
          "processors": [
            "lisi",
            "wangwu"
          ],
          "processors_auth": {
            "lisi": true,
            "wangwu": false
          }
        }
      ],
      "logs": [
        {
          "operator": "xxxx",
          "operate_at": "2024-11-06 12:03:12",
          "message": "同意",
          "name": "部门管理员审批"
        }
      ]
    }
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

| 参数名称      | 参数类型   | 描述                        |
|-----------|--------|---------------------------|
| id        | string | 退回计划子单ID                  |
| crp_audit | object | CRP审批信息，没有CRP审批阶段时为null   |

#### data.crp_audit

| 参数名称          | 参数类型         | 描述                                                        |
|---------------|--------------|-----------------------------------------------------------|
| crp_sn        | string       | CRP退回计划单号                                                 |
| crp_url       | string       | CRP退回计划单链接                                                |
| status        | string       | 审批状态（枚举值：init, auditing, rejected, done, failed, revoked） |
| status_name   | string       | 审批状态名称                                                    |
| message       | string       | 审批失败原因                                                    |
| current_steps | object array | 当前审批阶段                                                    |
| logs          | object array | 审批历史列表                                                    |

#### data.crp_audit.current_steps[n]

| 参数名称            | 参数类型         | 描述       |
|-----------------|--------------|----------|
| state_id        | string       | 步骤ID     |
| name            | string       | 步骤名称     |
| processors      | string array | 审批人列表    |
| processors_auth | object       | 审批人是否有权限 |

#### data.crp_audit.logs[n]

| 参数名称       | 参数类型   | 描述     |
|------------|--------|--------|
| operator   | string | 审批人    |
| operate_at | string | 处理时间   |
| message    | string | 审批结果信息 |
| name       | string | 步骤名称   |
