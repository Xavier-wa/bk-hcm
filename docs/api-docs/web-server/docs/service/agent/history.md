### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：无。
- 该接口功能描述：AI Agent 历史消息回放接口，基于 AG-UI 协议，以 SSE 流式返回指定会话的完整历史消息快照。需服务端配置 MySQL
  session 存储后方可使用。

### URL

POST /api/v1/agent/history

### 输入参数

| 参数名称        | 参数类型   | 必选 | 描述                                             |
|-------------|--------|----|------------------------------------------------|
| sessionCode | string | 是  | 会话对外标识，由创建会话接口返回。服务端自动解析为内部 threadId 并注入 runId |

### 调用示例

```json
{
  "sessionCode": "e3f4a2b1c9d8e7f6a5b4c3d2-2026032009"
}
```

### 响应说明

响应为 SSE 流，`Content-Type: text/event-stream`。依次返回以下事件：

| 事件类型              | 描述                                                      |
|-------------------|---------------------------------------------------------|
| RUN_STARTED       | Run 开始                                                  |
| MESSAGES_SNAPSHOT | 历史消息快照，携带 `messages` 数组，包含该会话的完整消息历史（已还原为标准 Message 列表） |
| RUN_FINISHED      | Run 正常结束                                                |
| RUN_ERROR         | 加载历史失败时返回，携带 `message` 错误信息                             |

#### MESSAGES_SNAPSHOT 事件中的 messages[n]

| 参数名称       | 参数类型   | 描述                                   |
|------------|--------|--------------------------------------|
| id         | string | 消息唯一标识                               |
| role       | string | 消息角色（枚举值：user、assistant、tool、system） |
| content    | string | 消息内容                                 |
| toolCalls  | array  | 工具调用列表（assistant 消息）                 |
| toolCallId | string | 对应的工具调用 ID（tool 消息）                  |

### 响应示例

```
data: {"type":"RUN_STARTED","threadId":"a1b2c3d4","runId":"x9y8z7w6"}

data: {"type":"MESSAGES_SNAPSHOT","messages":[{"id":"msg-001","role":"user","content":"帮我查询一下当前有哪些云账号"},{"id":"resp-001","role":"assistant","content":"当前共有 3 个云账号：..."}]}

data: {"type":"RUN_FINISHED","threadId":"a1b2c3d4","runId":"x9y8z7w6"}
```

### 补充说明

- 如果调用该接口时，后台仍有进行中的 LLM 调用。该接口会在返回完已有历史记录后，继续同步输出最新的调用结果，直到本次对话完全结束。（返回的 SSE 事件类型和  `agui` 接口相同）

