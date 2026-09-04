### 描述

- 该接口提供版本：v9.9.9.9。
- 该接口所需权限：平台-智能体助手管理。
- 该接口功能描述：运营看板评估结果分页。

### URL

POST /api/v1/agent/eval/dashboard/eval_results/list

### 输入参数

| 参数名称 | 参数类型 | 必选 | 描述 |
|--------|--------|----|------|
| from | string | 是 | 周期起点，RFC3339。评估按 `aiagent_run_eval.created_at` 过滤 |
| to | string | 是 | 周期终点，RFC3339 |
| scene | string | 否 | 场景过滤。枚举值：`host_apply`、`resource_query`、`chat` |
| query_like | string | 否 | 问句模糊匹配，匹配账本 `aiagent_run_eval.query`，不区分大小写 |
| passed | bool | 否 | 按查询时计算的通过结果过滤 |
| has_redline | bool | 否 | 是否命中红线 |
| run_id | string | 否 | 精确匹配评估行 `run_id` |
| session_id | string | 否 | 精确匹配评估行 `session_id` |
| users | string array | 否 | 匹配评估行 `user IN (...)`。空数组或不传表示不过滤，最多 100 个 |
| bk_biz_ids | int64 array | 否 | 匹配评估行 `bk_biz_id IN (...)`。空数组或不传表示不过滤，最多 100 个 |
| page | object | 否 | 分页。缺省 `start=0, limit=20` |

#### page

| 参数名称 | 参数类型 | 必选 | 描述 |
|--------|--------|----|------|
| start | uint32 | 否 | 起始偏移，从 0 开始 |
| limit | uint | 否 | 每页条数，缺省 20 |
| sort | string | 否 | 排序字段。枚举值：`created_at`、`quality_score`。不传按 `created_at` |
| order | string | 否 | 升降序。枚举值：`ASC`、`DESC`。不传按倒序 |

### 调用示例

```json
{
  "from": "2026-08-16T00:00:00+08:00",
  "to": "2026-08-23T23:59:59+08:00",
  "scene": "resource_query",
  "query_like": "广州",
  "passed": false,
  "page": {
    "start": 0,
    "limit": 20,
    "sort": "created_at",
    "order": "DESC"
  }
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "count": 2,
    "details": [
      {
        "created_at": "2026-08-20 10:00:00",
        "run_id": "6f6b35e6-9ee9-11f1-852b-525400225955",
        "session_id": "thread-abc",
        "user": "admin",
        "bk_biz_id": 100,
        "scene": "resource_query",
        "query": "查一下广州的 CVM",
        "quality_score": 40,
        "passed": false
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
| count | uint64 | 过滤后的总条数（不是本页条数） |
| details | object array | 当前页。无数据时为 `[]` |

#### details[n]

| 参数名称 | 参数类型 | 描述 |
|--------|--------|------|
| created_at | string | 评估时间 |
| run_id | string | AG-UI runId |
| session_id | string | 评估行会话 ID |
| user | string | 发起用户，与账本 `aiagent_run.user` 一致 |
| bk_biz_id | int64 | 业务 ID，与账本 `aiagent_run.bk_biz_id` 一致 |
| scene | string | 账本场景 |
| query | string | 本轮用户首句，可能为空 |
| quality_score | int | 质量分 |
| passed | bool | 查询时计算的通过结果 |
