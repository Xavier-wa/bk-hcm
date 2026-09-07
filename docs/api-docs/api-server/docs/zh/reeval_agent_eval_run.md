### 描述

- 该接口提供版本：v1.9.3.1。
- 该接口所需权限：平台-智能体助手管理。
- 该接口功能描述：对指定 `run_id` 手动触发一次模型评估。该 run 已有评估行时必须传 `overwrite=true`，否则拒绝覆盖。

### URL

POST /api/v1/agent/eval/runs/{run_id}/reeval

### 路径参数

| 参数名称 | 参数类型 | 必选 | 描述 |
|--------|--------|----|------|
| run_id | string | 是 | AG-UI runId |

### 输入参数

| 参数名称 | 参数类型 | 必选 | 描述 |
|--------|--------|----|------|
| overwrite | bool | 是 | 是否覆盖已有评估行。已有评估行时必须为 `true`可以覆盖，否则报错 |

### 调用示例

请求头需带网关鉴权（可用 HCM 自身 app_code）：

```
X-Bkapi-Authorization: {"bk_app_code":"<hcm_app_code>","bk_app_secret":"<hcm_app_secret>","bk_username":"<user>"}
```

```json
{
  "overwrite": true
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": null
}
```

### 响应参数说明

| 参数名称 | 参数类型 | 描述 |
|---------|--------|------|
| code | int32 | 状态码 |
| message | string | 请求信息 |
| data | null | 入队成功无返回体 |

### 补充说明

- 请求体必须包含 `overwrite`，空 body 或不传该字段会校验失败。
- 评估总开关关闭时返回失败。
- 评估队列满或提交等待超时返回过于频繁。
