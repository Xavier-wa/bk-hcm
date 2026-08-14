### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：平台-智能体助手。
- 该接口功能描述：查询AI Agent 就绪状态接口，通过这接口判断Agent是否可以对外服务。

### URL

GET /api/v1/agent/readiness

### 输入参数
无


### 响应示例

```json
{
    "code": 0,
    "message": "ok",
    "data": {
        "ready": true,
        "skill_ready": true,
        "prompt_ready": true
    }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int32  | 状态码  |
| message | string | 请求信息 |

#### data

| 参数名称    | 参数类型         | 描述                                    |
|---------|--------------|---------------------------------------|
| ready         | boolean       | Agentserver就绪状态|
| skill_ready   | boolean       | SKILL加载状态 |
| prompt_ready  | boolean       | Prompt加载状态 |
