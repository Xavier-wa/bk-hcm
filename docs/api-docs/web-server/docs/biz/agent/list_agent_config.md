### 描述

- 该接口提供版本：v9.9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：在指定业务下，按配置类型查询 Agent 可对前端下发的 `global_config`。本期用于拉取用户反馈的点赞 / 点踩标签文案（`config_type=agent_feedback_tag`），`config_value` 为英文枚举到中文文案的 map。该接口对 `config_type` 做白名单校验，未放行的类型直接返回参数非法，不能用来读取凭据类配置。标签配置本身是全局的，路径中的 `bk_biz_id` 只做业务访问鉴权。

### URL

GET /api/v1/agent/bizs/{bk_biz_id}/config/list

### 路径参数

| 参数名称      | 参数类型  | 必选 | 描述      |
|-----------|-------|----|---------|
| bk_biz_id | int64 | 是  | 蓝鲸业务 ID |

### 输入参数

| 参数名称        | 参数类型   | 必选 | 描述                                                |
|-------------|--------|----|---------------------------------------------------|
| config_type | string | 是  | 配置类型。查询参数。本期仅支持 `agent_feedback_tag`，其余值返回参数非法 |

### 调用示例

```
GET /api/v1/agent/bizs/123/config/list?config_type=agent_feedback_tag
```

### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "details": [
      {
        "config_key": "like",
        "config_value": {
          "accurate": "回答准确",
          "complete": "内容完整",
          "professional": "专业清晰",
          "solved": "解决了问题",
          "well_formatted": "格式清晰",
          "other": "其他"
        }
      },
      {
        "config_key": "dislike",
        "config_value": {
          "factual_error": "事实错误",
          "reasoning_error": "推理错误",
          "incomplete": "内容不完整",
          "unprofessional": "内容不专业",
          "calculation_error": "计算错误",
          "harmful": "违法有害",
          "format_error": "格式错误",
          "garbled": "乱码错误",
          "duplicated": "内容重复",
          "chart_expected": "需画图但生成文本",
          "other": "其他"
        }
      }
    ]
  }
}
```

配置缺失时返回空列表，不报错：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "details": []
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

| 参数名称    | 参数类型         | 描述                         |
|---------|--------------|----------------------------|
| details | object array | 该 `config_type` 下的配置行，不分页 |

#### details[n]

| 参数名称         | 参数类型   | 描述                                                                 |
|--------------|--------|--------------------------------------------------------------------|
| config_key   | string | 配置键。`agent_feedback_tag` 下枚举值：`like`（点赞标签）、`dislike`（点踩标签）                 |
| config_value | object | 标签 map。key 为英文枚举（提交反馈时的 `tags` 取值），value 为中文文案（前端 chips 展示） |

### 错误码说明

| code    | 描述                                          |
|---------|---------------------------------------------|
| 2000001 | 请求参数错误，例如路径参数 `bk_biz_id` 为空、为 0 或负数，或 `config_type` 不在白名单 |
| 2000006 | 查询配置失败                                      |
| 2000012 | 用户无指定业务的「业务访问」权限                            |

### 补充说明

- `agent_feedback_tag` 的 `config_value` 是扁平 map，不包含「针对内容 / 针对格式」分组。
- 配置缺失或 `config_value` 为空时，前端可退化为只展示文本框、不展示 chips。
- 提交反馈时的 `tags` 必须是当前 `reaction` 对应那份 map 的 key。
