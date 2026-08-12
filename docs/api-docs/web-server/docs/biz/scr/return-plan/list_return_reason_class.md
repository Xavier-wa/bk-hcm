### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：按 OBS 项目类型查询退回原因大类列表。代理（proxy）CRP 系统的退回原因大类查询能力，供提交退回计划时选择 return_reason_class。

### URL

POST /api/v1/woa/bizs/{bk_biz_id}/plans/returns/reason_classes/list

### 路径参数

| 参数名称      | 参数类型   | 必选 | 描述    |
|-----------|--------|----|-------|
| bk_biz_id | int    | 是  | 业务 ID |

### 输入参数

| 参数名称        | 参数类型   | 必选 | 描述        |
|-------------|--------|----|-----------|
| obs_project | string | 是  | OBS 项目类型  |

### 调用示例

```json
{
  "obs_project": "常规项目"
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "reason_classes": [
      "成本优化&利用率提升",
      "业务下线",
      "架构调整"
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

| 参数名称           | 参数类型         | 描述        |
|----------------|--------------|-----------|
| reason_classes | string array | 退回原因大类列表  |
