### 描述

- 该接口提供版本：v9.9.9.9。
- 该接口所需权限：平台-智能体助手管理。
- 该接口功能描述：按会话从历史事件同步对话轮次到 `aiagent_run`。已存在的 `run_id` 跳过，不覆盖。按 InvocationID 切轮；状态取自 AG-UI track（`aiagent_session_track_events`）该 run 最后一次 `RUN_FINISHED` / `RUN_ERROR`，否则写入 `unknown`。`reason` 为 `history_sync`，时间戳回写为历史事件时间。

### URL

POST /api/v1/agent/eval/runs/history/sync

### 输入参数

| 参数名称 | 参数类型 | 必选 | 描述 |
|--------|--------|----|------|
| session_codes | string[] | 是 | 要同步的 `session_code` 列表。至少 1 个，最多 20 个；重复值会去重 |

### 调用示例

请求头需带网关鉴权（可用 HCM 自身 app_code）：

```
X-Bkapi-Authorization: {"bk_app_code":"<hcm_app_code>","bk_app_secret":"<hcm_app_secret>","bk_username":"<user>"}
```

```json
{
  "session_codes": [
    "xxxxxxxx-20260903",
    "yyyyyyyy-20260902"
  ]
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "session_scanned": 2,
    "session_failed": 0,
    "run_created": 35,
    "run_skipped": 12,
    "run_failed": 0
  }
}
```

### 响应参数说明

| 参数名称 | 参数类型 | 描述 |
|---------|--------|------|
| code | int32 | 状态码 |
| message | string | 请求信息 |
| data.session_scanned | uint64 | 实际命中并处理的会话数 |
| data.session_failed | uint64 | 会话不存在或拉历史失败的数量 |
| data.run_created | uint64 | 新写入的账本行数 |
| data.run_skipped | uint64 | 已存在/空 transcript/非法 run_id 跳过数 |
| data.run_failed | uint64 | 写入失败数 |

### 补充说明

- 不依赖评估总开关。会话存储未初始化时返回失败。
- 写入前按 `run_id` 查询 `aiagent_run`，已存在则跳过该轮，不覆盖。
- 状态映射：`RUN_FINISHED` → `finished`，`RUN_ERROR` → `error`，无终态事件 → `unknown`。
- `created_at` / `updated_at` 回写为该轮历史事件的起止时间，避免与账本上线后的真实轮次乱序。
- 写入后不会自动入评估队列；需要评估时再调 reeval 或等补偿窗口扫到。
