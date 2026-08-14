# API：aiagent-同会话内动态切换场景标签

> 本迭代**不新增** REST 接口。前端在既有 Agent SSE（AG-UI）流中识别新的 `CUSTOM` 事件，并依赖既有会话列表字段 `session_tag` 做刷新后校验。

## 1. 范围

| 项 | 说明 |
|----|------|
| 新增 REST | 无 |
| 变更 REST | 无（假设 list/create 已返回/接受 `session_tag`，见 §3） |
| 新增 SSE 事件 | `CUSTOM` + `name=scene.switched` |
| 历史回放 | **本期不消费** history / `MESSAGES_SNAPSHOT` 中的该事件（PRD Q-002） |

## 2. SSE：`scene.switched`

### 2.1 识别

```
event.type === 'CUSTOM' && event.name === 'scene.switched'
```

通道：当前会话流式 SSE（与现有 `use-stream` / `use-event` 一致）。**仅流式过程处理**。

### 2.2 逻辑载荷（产品/联调约定）

```json
{
  "type": "CUSTOM",
  "timestamp": 1785483780052,
  "name": "scene.switched",
  "value": {
    "nodeId": "scene_dispatch",
    "payload": {
      "from": "host_apply",
      "to": "resource_query"
    },
    "timestamp": "2026-07-31T07:43:00.001649644Z"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 固定 `scene.switched` |
| `value` | object \| string | 是 | 见下「线格式」 |
| `value.nodeId` | string | 否 | 编排节点标识；前端可忽略 |
| `value.payload.from` | string | 否 | 切换前场景标识；**不校验**是否等于当前标签 |
| `value.payload.to` | string | 是* | 切换后场景标识；缺失/空则**忽略整事件** |
| `value.timestamp` | string | 否 | 后端时间戳；前端可忽略 |

\* 业务语义必填：无有效 `to` 时前端不更新标签。

### 2.3 线格式说明

现网其它 `CUSTOM`（如 `hitl.interrupt`）在 `use-event` 中对 `event.value` 做 `JSON.parse(string)`。联调时按实际下发二选一适配：

| 形态 | 处理 |
|------|------|
| `value` 为 **JSON 字符串** | `JSON.parse` 后取 `payload`（与现网一致） |
| `value` 已是 **对象** | 直接读 `payload` |

`api.md` 以逻辑字段为准；coding 阶段按联调实测分支解析。

### 2.4 前端消费契约（非协议字段）

收到有效 `to` 后：

1. 更新本地该会话的 `sessionTag` / `session_tag` 为 `to`
2. 全屏：派生更新聊天区 chip 文案与侧栏文件夹归组
3. 浮窗：仅更新会话数据，无标签 UI
4. **不**向后端回传确认；**不**因此中断当前 run / 文本流

### 2.5 已知场景标识（展示映射，非接口枚举）

| `session_tag` / `to` | 展示名（前端常量） |
|----------------------|-------------------|
| `host_apply` | 主机申领 |
| `resource_query` | 资源查询 |
| 其他 | 展示原始字符串 |

后端可下发未映射值；前端不得因未知而丢弃事件。

## 3. 既有 REST：会话 `session_tag`（刷新校验）

路径前缀：`/api/v1/agent/{bizPath}/sessions`（见 `store/chatbot/session.ts`）。

| 接口 | 与本需求关系 |
|------|----------------|
| `POST .../sessions/list` | 列表项含 `session_tag?`；刷新后应用新值（AC-005 / R-003） |
| `POST .../sessions/create` | 创建时可带 `session_tag`；本迭代不改创建协议 |

**假设（需联调）**：后端在下发 `scene.switched` 的同时（或之前）已将会话 `session_tag` 持久化为 `payload.to`。若 list 仍返回旧值 → 记「需后端配合」，前端仍完成本地即时更新。

## 4. 错误与忽略

| 情况 | 前端行为 |
|------|----------|
| `name` 非 `scene.switched` | 沿用既有 CUSTOM 分流；本迭代不处理 |
| 无 `to` / `to` 为空 | 忽略事件，不改标签，不报错打断对话 |
| `value` 无法解析 | 忽略事件（建议打 warn 日志），不打断对话 |
| `from` 与当前标签不一致 | 仍以 `to` 更新 |

## 5. 本期不做

- 为场景切换新增 REST
- history / snapshot 中重建 `scene.switched`（除非后续升格 Q-002）
- 前端调用接口主动 PATCH `session_tag`（除非联调证明必须且另开需求）

## 6. 与 PRD 映射

| PRD | API |
|-----|-----|
| F-001 / AC-001~003 / AC-006 | §2 SSE 识别与 `payload.to` |
| F-004 浮窗数据侧 | §2.4（无新接口） |
| AC-005 刷新后标签 | §3 list `session_tag` |
| Q-002 history | §1 / §5 明确不做 |
