### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：平台-智能体助手。
- 该接口功能描述：查询当前用户的 AI Agent 会话列表。系统自动在过滤条件中注入 `user = 当前用户`，确保用户只能查看自己的会话。

### URL

POST /api/v1/agent/sessions/list

### 输入参数

| 参数名称   | 参数类型   | 必选 | 描述     |
|--------|--------|----|--------|
| filter | object | 否  | 查询过滤条件 |
| page   | object | 是  | 分页设置   |

#### filter

| 参数名称  | 参数类型        | 必选 | 描述                                                                    |
|-------|-------------|----|-----------------------------------------------------------------------|
| op    | enum string | 是  | 操作符（枚举值：and、or）。如果是 and，则表示多个 rule 之间是且的关系；如果是 or，则表示多个 rule 之间是或的关系。 |
| rules | array       | 是  | 过滤规则，最多设置 5 个 rules。如果 rules 为空数组，op（操作符）将没有作用，代表查询全部数据。              |

#### rules[n] （详情请看 rules 表达式说明）

| 参数名称  | 参数类型        | 必选 | 描述                                           |
|-------|-------------|----|----------------------------------------------|
| field | string      | 是  | 查询条件 Field 名称，具体可使用的用于查询的字段及其说明请看下面 - 查询参数介绍 |
| op    | enum string | 是  | 操作符（枚举值：eq、neq、gt、gte、lt、lte、in、nin、cs、cis）  |
| value | 可变类型        | 是  | 查询条件 Value 值                                 |

##### rules 表达式说明：

##### 1. 操作符

| 操作符 | 描述                                           | 操作符的 value 支持的数据类型                            |
|-----|----------------------------------------------|-----------------------------------------------|
| eq  | 等于。不能为空字符串                                   | boolean, numeric, string                      |
| neq | 不等。不能为空字符串                                   | boolean, numeric, string                      |
| gt  | 大于                                           | numeric，时间类型为字符串（标准格式："2006-01-02T15:04:05Z"） |
| gte | 大于等于                                         | numeric，时间类型为字符串（标准格式："2006-01-02T15:04:05Z"） |
| lt  | 小于                                           | numeric，时间类型为字符串（标准格式："2006-01-02T15:04:05Z"） |
| lte | 小于等于                                         | numeric，时间类型为字符串（标准格式："2006-01-02T15:04:05Z"） |
| in  | 在给定的数组范围中。value 数组中的元素最多设置 100 个，数组中至少有一个元素  | boolean, numeric, string                      |
| nin | 不在给定的数组范围中。value 数组中的元素最多设置 100 个，数组中至少有一个元素 | boolean, numeric, string                      |
| cs  | 模糊查询，区分大小写                                   | string                                        |
| cis | 模糊查询，不区分大小写                                  | string                                        |

##### 2. 协议示例

查询 session_name 包含 "test" 且 is_temporary 为 false 的会话。

```json
{
  "op": "and",
  "rules": [
    {
      "field": "session_name",
      "op": "cs",
      "value": "test"
    },
    {
      "field": "is_temporary",
      "op": "eq",
      "value": false
    }
  ]
}
```

#### 查询参数介绍：

| 参数名称                  | 参数类型   | 描述                              |
|-----------------------|--------|---------------------------------|
| id                    | string | 会话 ID                           |
| session_code          | string | 对外会话标识                          |
| session_name          | string | 会话名称                            |
| app_name              | string | 应用名称                            |
| is_temporary          | bool   | 是否为临时会话                         |
| session_content_count | int    | 会话消息计数                          |
| creator               | string | 创建者                             |
| created_at            | string | 创建时间（格式："2006-01-02T15:04:05Z"） |
| updated_at            | string | 更新时间（格式："2006-01-02T15:04:05Z"） |

#### page

| 参数名称  | 参数类型   | 必选 | 描述                                                                                                                                                     |
|-------|--------|----|--------------------------------------------------------------------------------------------------------------------------------------------------------|
| count | bool   | 是  | 是否返回总记录条数。如果为 true，查询结果返回总记录条数 count，但查询结果详情数据 details 为空数组，此时 start 和 limit 参数将无效，且必须设置为 0。如果为 false，则根据 start 和 limit 参数，返回查询结果详情数据，但总记录条数 count 为 0 |
| start | uint32 | 否  | 记录开始位置，start 起始值为 0                                                                                                                                    |
| limit | uint32 | 否  | 每页限制条数，最大 500，不能为 0                                                                                                                                    |
| sort  | string | 否  | 排序字段，返回数据将按该字段进行排序                                                                                                                                     |
| order | string | 否  | 排序顺序（枚举值：ASC、DESC）                                                                                                                                     |

### 调用示例

#### 查询所有正式会话（第一页，每页 20 条）

```json
{
  "filter": {
    "op": "and",
    "rules": [
      {
        "field": "is_temporary",
        "op": "eq",
        "value": false
      }
    ]
  },
  "page": {
    "count": false,
    "start": 0,
    "limit": 20,
    "sort": "created_at",
    "order": "DESC"
  }
}
```

#### 查询总条数

```json
{
  "page": {
    "count": true,
    "start": 0,
    "limit": 0
  }
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "count": 0,
    "details": [
      {
        "id": "a1b2c3d4",
        "session_code": "e3f4a2b1c9d8e7f6a5b4c3d2-2026032009",
        "session_name": "我的第一个对话",
        "app_name": "hcm-agent",
        "user": "admin",
        "thread_id": "a1b2c3d4",
        "is_temporary": false,
        "session_content_count": 5,
        "extensions": null,
        "creator": "admin",
        "revisor": "admin",
        "created_at": "2026-03-20T09:00:00Z",
        "updated_at": "2026-03-20T10:30:00Z"
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

| 参数名称    | 参数类型         | 描述                                    |
|---------|--------------|---------------------------------------|
| count   | uint64       | 总记录条数。page.count 为 true 时返回实际总数，否则为 0 |
| details | object array | 会话列表，page.count 为 true 时为空数组          |

#### details[n]

| 参数名称                  | 参数类型   | 描述                                     |
|-----------------------|--------|----------------------------------------|
| id                    | string | 会话 ID（内部主键）                            |
| session_code          | string | 对外会话标识，格式为 `{md5}-{YYYYMMDDHH}`        |
| session_name          | string | 会话名称                                   |
| app_name              | string | 应用名称                                   |
| user                  | string | 会话所属用户                                 |
| thread_id             | string | 框架内部 thread ID，值与 id 相同                |
| is_temporary          | bool   | 是否为临时会话                                |
| session_content_count | int    | 会话消息计数（每次通过 /agui 接口交互后异步自增）           |
| extensions            | object | 扩展字段，暂未使用，默认为 null                     |
| creator               | string | 创建者                                    |
| revisor               | string | 最近修改者                                  |
| created_at            | string | 创建时间（格式："2006-01-02T15:04:05.000000Z"） |
| updated_at            | string | 更新时间（格式："2006-01-02T15:04:05.000000Z"） |
