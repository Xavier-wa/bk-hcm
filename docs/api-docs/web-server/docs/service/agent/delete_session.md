### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：无。
- 该接口功能描述：删除指定 AI Agent 会话。系统校验会话归属后执行删除，不允许删除其他用户的会话。

### URL

DELETE /api/v1/agent/sessions/{session_code}

### 路径参数

| 参数名称         | 参数类型   | 必选 | 描述                           |
|--------------|--------|----|------------------------------|
| session_code | string | 是  | 会话对外标识，由创建接口返回的 session_code |

### 调用示例

```
DELETE /api/v1/agent/sessions/e3f4a2b1c9d8e7f6a5b4c3d2-2026032009
```

无请求体。

### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": null
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述                 |
|---------|--------|--------------------|
| code    | int32  | 状态码                |
| message | string | 请求信息               |
| data    | null   | 删除操作无返回数据，固定为 null |
