### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：无。
- 该接口功能描述：创建 AI Agent 会话，返回会话 ID、对外标识 session_code 及框架内部 thread_id。

### URL

POST /api/v1/agent/sessions/create

### 输入参数

| 参数名称         | 参数类型   | 必选 | 描述                     |
|--------------|--------|----|------------------------|
| session_name | string | 否  | 会话名称，用户可自定义；不传时默认为空字符串 |

### 调用示例

```json
{
  "session_name": "我的第一个对话"
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "a1b2c3d4",
    "session_code": "e3f4a2b1c9d8e7f6a5b4c3d2-2026032009",
    "thread_id": "a1b2c3d4",
    "session_name": "我的第一个对话"
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

| 参数名称         | 参数类型   | 描述                                            |
|--------------|--------|-----------------------------------------------|
| id           | string | 会话 ID（内部主键，8 位 36 进制字符串）                      |
| session_code | string | 对外会话标识，格式为 `{md5}-{YYYYMMDDHH}`，客户端后续操作均使用此字段 |
| thread_id    | string | 框架内部 thread ID，与 id 值相同                       |
| session_name | string | 会话名称                                          |
