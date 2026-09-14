# API：agent回复交互体验优化-前端

> 本迭代**不新增、不变更** REST / SSE 协议。只改前端如何消费既有 `/agui`、`/history` 事件与消息形状。展开/收起不发会话请求。

## 1. 范围

| 项 | 说明 |
|----|------|
| 新增 REST | 无 |
| 变更 REST | 无 |
| 新增 SSE 事件 / 字段 | 无（工具说明改读 `toolCalls[].function.arguments.tool_intent`；旧 `tool-intent-*` 消息只丢弃） |
| 权限 | 不新增；入口仍认 `agent_assistant`（AC-S01） |

通道沿用现网（`src/store/chatbot/agent.ts`）：

| 通道 | 方法 | 路径 | 本单 |
|------|------|------|------|
| 流式对话 | `POST` | `/api/v1/agent/agui` | 请求体不变；说明取 `TOOL_CALL_ARGS` 里的 `tool_intent` |
| 历史回放 | `POST` | `/api/v1/agent/history` | 请求体不变（`{ sessionCode }`）；SNAPSHOT 后从各 `toolCall` 参数取 `tool_intent` |
| 停止 | `POST` | `/api/v1/agent/cancel` | 沿用；前端据 `stoppedByUser` 保持过程区展开 |

## 2. `/agui`：既有事件顺序（冻结）

一次工具调用的现网顺序（2026-09 实测，说明在 ARGS 的 JSON 里，无独立 TEXT_MESSAGE）：

```
TOOL_CALL_START     toolCallId / toolCallName（parentMessageId 前端不消费）
TOOL_CALL_ARGS      delta 一次下发完整 JSON，前后可有空格，内含 tool_intent
TOOL_CALL_END
TOOL_CALL_RESULT
```

`TOOL_CALL_ARGS` 样例：

```json
{
  "type": "TOOL_CALL_ARGS",
  "toolCallId": "functions.tool_proxy_search_tools:1",
  "delta": " {\"query\":\"腾讯自研云主机离线偏好推荐 get_biz_apply_recommend_by_static\",\"tool_intent\":\"正在查找主机离线偏好推荐工具，为用户生成可申请的主机方案\"} "
}
```

思考段沿用现网 `REASONING_*`；耗时**只能前端计时**（`REASONING_START` → `REASONING_END`），协议不带 duration，见 §2.4。

### 2.1 `TOOL_CALL_START`

| 字段 | 类型 | 必填 | 前端用法 |
|------|------|------|----------|
| `toolCallId` | string | 是 | 与 RESULT 配对 |
| `toolCallName` | string | 是 | 工具行标题里的名称；现网文案 `调用工具 {name}` |
| `mcpName` | string | 否 | 有值且该次确为 MCP 时才可用 `MCP调用：{mcpName} / {name}`；空则不要写 MCP 前缀 |
| `description` | string | 否 | 写入 `toolCalls[].function.description`，属内部详情，**不是**工具行上方的 intent 行 |

标题**视实际调用**，不写死稿面 `MCP调用：服务 / 方法`。

### 2.2 `TOOL_CALL_ARGS`（tool_intent）

| 字段 | 类型 | 必填 | 前端用法 |
|------|------|------|----------|
| `toolCallId` | string | 是 | 定位该次调用 |
| `delta` | string | 是 | 一次下发完整 JSON（前后可有空格），拼进 `function.arguments`；取出 `tool_intent` 画在工具行上方，并从展示用参数里剥掉该键 |

缺 `tool_intent` 或文案为空：不画说明行。旧 `TEXT_MESSAGE_*`（`messageId = tool-intent-{toolCallId}`）若仍下发，只丢弃、不取文案、不开气泡。

### 2.3 `TOOL_CALL_RESULT`

| 字段 | 类型 | 前端用法 |
|------|------|----------|
| `toolCallId` | string | 配对 |
| `content` | string | 交给现有 `ToolcallRender`，不新做参数面板 |
| `duration` | number | 工具行右侧耗时（稿面示例 `650ms`）；无则不画右侧时钟+数字 |

### 2.4 耗时（思考 / 工具调用）

**协议里没有耗时字段**（已核对源码）：AG-UI Go SDK 的 `ReasoningEndEvent` 只有 `{messageId}`，构造函数 `NewReasoningEndEvent(messageID)` 只收一个参数，后端 translator 全部按此签名发；`ToolCallResultEvent` 同样只有 `{messageId, toolCallId, content, role}`。

两处耗时都只能前端掐表，事件里真出现 `duration` 时优先用它（留了兜底分支，协议补字段即自动生效）：

| 耗时 | `/agui` 来源 | `/history` | 展示 |
|------|--------------|-----------|------|
| 思考 | `REASONING_START` → `REASONING_END` | **无**，见 §3 | 汇总进摘要 `思考耗时X.XXs`（小写 `s`、两位小数） |
| 单次工具调用 | `TOOL_CALL_END` → `TOOL_CALL_RESULT`，按 `toolCallId` 分别计时（缺 `TOOL_CALL_END` 时退回 `TOOL_CALL_START`） | **无**，见 §3 | 工具行右侧 `650ms`（ms），**不进摘要** |

工具耗时的起点取 `TOOL_CALL_END` 而不是 `START`：`START` → `END` 之间是模型逐 token 吐入参，算进去会把工具执行时间放大。

无工具：摘要只有 `思考耗时X.XXs`。有工具：`调用N个工具，思考耗时X.XXs`。无思考且无工具：不渲染过程区。

### 2.5 工具调用状态

协议没有独立的状态字段，状态从事件序列推：

| 状态 | 判据 |
|------|------|
| 进行中 | 没有配到 `role=tool` 的结果消息，且本轮仍在流式 |
| 成功 | 配到结果消息且无错 |
| 失败 | 助手消息或结果消息 `status=Error`，或结果消息带 `error` |
| 未知（不画图标） | 没有结果消息且本轮已结束（用户 Stop / 异常中断 / 历史回放） |

**不能用助手消息的 `status` 判进行中**：`TOOL_CALL_END` 只代表入参给完，此时 `status` 已是 `Complete`，工具还在跑。唯一可靠的完成信号是配到结果消息。

## 3. `/history`：既有消息形状（冻结）

`MESSAGES_SNAPSHOT` 替换式灌入（现网 `fetchHistory`）。工具说明与 `/agui` 同字段：`toolCalls[].function.arguments` 是带首尾空格的完整 JSON 字符串。

样例：

```json
{
  "toolCalls": [
    {
      "id": "functions.skill_load:0",
      "type": "function",
      "function": {
        "name": "skill_load",
        "arguments": " {\"skill\":\"ziyan-cvm-apply\",\"tool_intent\":\"正在加载腾讯自研云主机申领技能，准备为用户推荐主机申请方案\"} "
      }
    }
  ]
}
```

| 顺序（常见） | 形状 | 前端必须做的事 |
|--------------|------|----------------|
| 先 | 助手消息带 `toolCalls[]`（如上） | 渲染样式化工具行；trim 后 parse `arguments`，取出该次调用自己的 `tool_intent` 画在行上方，并从展示用参数里剥掉 |
| 后（遗留） | `{ id: "tool-intent-{toolCallId}", content: "…" }` | **丢弃**，不再按 id 配对取文案；避免旧会话冒出独立气泡 |

缺 `tool_intent` 或文案为空：不画说明行。

历史回放按**已结束**处理：过程区默认收起（AC-009）。不新增 history 请求；打开会话仍走现网一次 `fetchHistory`。

**快照里没有任何时间信息**（已核对 `trpc-agent-go/server/agui` v1.8.0，go.mod 锁定版本）：reduce 器对 `ReasoningEndEvent` 直接 `return nil`，重放出的思考消息是 `Message{id, role, content}`、工具结果是 `Message{id, role, content, toolCallId}`，都不带 duration；逐条消息的时间戳索引（`rawEvent.messages[id].timestamp`）要到该库 v1.9.0 之后才有。因此**历史的思考耗时与工具耗时都拿不到，也无法反推**——省掉耗时段，不补 0（详见 §5）。

## 4. 前端消费契约（非协议字段）

| 行为 | 是否打接口 |
|------|------------|
| 点击摘要行展开/收起 | **否**（AC-P01，100ms 内完成） |
| 流式中过程区展开、结束后自动收起 | 否；看本地 `isChatting` / `RunFinished` |
| 用户 Stop 保持展开并结算摘要 | 只走现网 `cancel` + abort，不另发会话接口 |
| 中间说明（工具之间的叙述） | 普通助手文本，不是工具参数 `tool_intent`，不随过程区收走 |

同一轮浅底、摘要文案、iconfont（`circle-success` / `circle-error` / `circle-time`）均为前端展示，无接口字段。

## 5. 错误与忽略

| 情况 | 前端行为 |
|------|----------|
| 无 `tool_intent` 或文案为空 | 只画工具行，不报错 |
| history 里仍夹着 `tool-intent-*` 消息 | 丢弃该条，说明仍从对应 `toolCall` 参数取 |
| `mcpName` 为空 | 标题用现网 `调用工具 {name}` |
| `duration` 缺失（`/history` 恒缺） | 工具行不画耗时；思考条标题只写「已思考完成」；摘要省掉「思考耗时X.XXs」，只剩「调用N个工具」，两者都缺时写「已完成思考」。**禁止补 0 显示成 `0ms` / `0.00s`** |
| `TOOL_CALL_*` 缺 `toolCallId` | 跳过该次调用，不打断对话 |

不新增错误码。会话 403 / 鉴权与现网一致。

## 6. 本期不做

- 新增或改 `/agui`、`/history`、`/cancel` 的 path、body、事件类型
- 让后端重排 history 或补 `tool-intent`
- 前端工具名中文映射表
- 因展开/收起多打会话接口
- 新权限点

## 7. 与 PRD 映射

| PRD | API 结论 |
|-----|----------|
| F-005 / AC-006 / R-006 | §2.2 参数 `tool_intent`；画在工具行上方，不上气泡 |
| AC-007 / AC-007b | §3 history 从各 `toolCall` 参数取；缺省不画行 |
| F-006 / R-004 / R-005 | §2.4 摘要与 duration |
| AC-P01 | §4 展开/收起不发接口 |
| AC-S01 | §1 权限不变 |
| R-007 / AC-010 | §2.3 结果仍给现有 `ToolcallRender` |
| R-008 | §6 不改协议、不做中文表 |
