### 描述

- 该接口提供版本：v1.9.3.1。
- 该接口所需权限：平台-智能体助手管理。
- 该接口功能描述：运营看板单轮详情。返回账本问句/场景/用户/终态、评估结论、九维分数，以及评估窗口内各轮 transcript。

### URL

GET /api/v1/agent/eval/runs/{run_id}

### 输入参数

| 参数名称 | 参数类型 | 必选 | 描述 |
|--------|--------|----|------|
| run_id | string | 是 | 路径参数，AG-UI runId |
| include_trace | string | 否 | Query。传 `true` 时返回 `eval.eval_trace`。缺省不返回 |

### 调用示例


GET /api/v1/agent/eval/runs/6f6b35e6-9ee9-11f1-852b-525400225955?include_trace=false


### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "run_id": "6f6b35e6-9ee9-11f1-852b-525400225955",
    "session_code": "e3f4a2b1c9d8e7f6a5b4c3d2-2026032009",
    "user": "admin",
    "scene": "resource_query",
    "status": "finished",
    "query": "查一下广州的 CVM",
    "summary": "本轮先给结论，工具结果与回复一致。",
    "eval": {
      "process_score": 86,
      "outcome_score": 80,
      "quality_score": 83,
      "passed": true,
      "redlines": [],
      "reason_code": "ok",
      "dims": {
        "faithfulness": 4,
        "conclusion_first": 4
      },
      "summary": "本轮先给结论，工具结果与回复一致。"
    },
    "window": [
      {
        "run_id": "6f6b35e6-9ee9-11f1-852b-525400225955",
        "role": "target",
        "brief": "列出广州运行中的 CVM",
        "query": "查一下广州的 CVM",
        "transcript": {
          "items": [
            {
              "type": "user",
              "text": "查一下广州的 CVM"
            },
            {
              "type": "assistant",
              "text": "广州目前有 3 台运行中的 CVM。"
            }
          ]
        }
      }
    ]
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

| 参数名称 | 参数类型 | 描述 |
|--------|--------|------|
| run_id | string | AG-UI runId |
| session_code | string | 所属会话对外 ID |
| user | string | 发起用户 |
| scene | string | 场景。枚举值：`host_apply`、`resource_query`、`chat`、`unsupported` |
| status | string | 账本状态。枚举值：`running`、`finished`、`error`、`cancel`、`unknown` |
| query | string | 本轮用户首句 |
| summary | string | 评估结论。无 eval 时为空 |
| eval | object / null | 评估块。有 run 无 eval 时为 `null`，不返回 404 |
| window | object array | 评估窗口下钻 |

### 补充说明

- run 不存在返回 `RecordNotFound`。run 存在但无 eval 仍返回账本，`eval` 为空。
- 默认不要传 `include_trace=true`。
