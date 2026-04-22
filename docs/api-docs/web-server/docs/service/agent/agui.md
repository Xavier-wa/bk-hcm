### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：无。
- 该接口功能描述：AI Agent 主聊天接口，基于 AG-UI 协议，以 SSE（Server-Sent Events）流式返回 Agent 推理过程与回复内容。

### URL

POST /api/v1/agent/agui

### 输入参数

| 参数名称           | 参数类型   | 必选 | 描述                                                           |
|----------------|--------|----|--------------------------------------------------------------|
| sessionCode    | string | 是  | 会话对外标识，由创建会话接口返回。服务端自动解析为内部 threadId 并注入 runId               |
| messages       | array  | 是  | 消息历史列表，包含本次对话的上下文                                            |
| forwardedProps | object | 否  | 透传给 Agent 的额外属性，可传 `modelName` 指定模型（需在服务端 allowedModels 列表中） |
| state          | object | 否  | 任意状态载荷，透传给 Agent                                             |
| tools          | array  | 否  | 可用工具列表                                                       |
| context        | array  | 否  | 上下文条目列表                                                      |

#### messages[n]

| 参数名称       | 参数类型   | 必选 | 描述                                   |
|------------|--------|----|--------------------------------------|
| id         | string | 是  | 消息唯一标识                               |
| role       | string | 是  | 消息角色（枚举值：user、assistant、tool、system） |
| content    | string | 否  | 消息内容                                 |
| toolCalls  | array  | 否  | 工具调用列表（assistant 消息）                 |
| toolCallId | string | 否  | 对应的工具调用 ID（tool 消息）                  |

#### forwardedProps

| 参数名称      | 参数类型   | 必选 | 描述                                                |
|-----------|--------|----|---------------------------------------------------|
| modelName | string | 否  | 指定使用的 AI 模型名称，需在服务端配置的 `allowedModels` 列表中，否则返回错误 |

### 调用示例

```json
{
  "sessionCode": "e3f4a2b1c9d8e7f6a5b4c3d2-2026032009",
  "messages": [
    {
      "id": "msg-001",
      "role": "user",
      "content": "帮我查询一下当前有哪些云账号"
    }
  ],
  "forwardedProps": {
    "modelName": "deepseek-v3"
  }
}
```

### 响应说明

响应为 SSE 流，`Content-Type: text/event-stream`。每个事件格式如下：

```
data: {"type":"<事件类型>", ...事件字段}
```

#### SSE 事件类型说明

| 事件类型                          | 描述                             |
|-------------------------------|--------------------------------|
| RUN_STARTED                   | Run 开始，携带 `threadId` 和 `runId` |
| TEXT_MESSAGE_START            | 文本消息开始，携带消息 `id` 和 `role`      |
| TEXT_MESSAGE_CONTENT          | 文本消息内容片段（打字机效果逐 token 推送）      |
| TEXT_MESSAGE_END              | 文本消息结束                         |
| THINKING_TEXT_MESSAGE_START   | 推理内容开始（推理模型，如 deepseek-r1）     |
| THINKING_TEXT_MESSAGE_CONTENT | 推理内容片段                         |
| THINKING_TEXT_MESSAGE_END     | 推理内容结束                         |
| TOOL_CALL_START               | 工具调用开始，携带工具名称                  |
| TOOL_CALL_ARGS                | 工具调用参数片段                       |
| TOOL_CALL_END                 | 工具调用结束                         |
| TOOL_CALL_RESULT              | 工具调用结果                         |
| STEP_STARTED                  | Agent 步骤开始                     |
| STEP_FINISHED                 | Agent 步骤结束                     |
| RUN_FINISHED                  | Run 正常结束                       |
| RUN_ERROR                     | Run 异常结束，携带 `message` 错误信息     |

### 响应示例

```
data: {"type":"RUN_STARTED","threadId":"a1b2c3d4","runId":"x9y8z7w6"}

data: {"type":"TEXT_MESSAGE_START","messageId":"resp-001","role":"assistant"}

data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"resp-001","delta":"当前"}

data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"resp-001","delta":"共有 3 个云账号："}

data: {"type":"TEXT_MESSAGE_END","messageId":"resp-001"}

data: {"type":"RUN_FINISHED","threadId":"a1b2c3d4","runId":"x9y8z7w6"}
```
