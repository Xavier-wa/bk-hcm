# API：Chatbot HITL 中断交互（通用选项列表）

## 1. 接口概述

本次为前端渲染与消息归一化增强，**无新增后端 API**。主要变化：

1. 支持历史流 `MESSAGES_SNAPSHOT` 中的 `role: "activity" + activityType: "CUSTOM"` 消息。
2. 对 `content.name === "hitl.interrupt"` 的活动消息进行结构化解析，并复用现有 HITL 卡片渲染。
3. 历史场景下 HITL 卡片按只读展示，结合其后续 `user` 消息推断是否命中预置选项。

## 2. 消息格式（输入）

### 2.1 实时 CUSTOM 事件（已有）

```typescript
interface CustomEvent {
  type: 'CUSTOM';
  name: 'hitl.interrupt';
  value: string; // JSON 字符串
}
```

### 2.2 历史快照 activity 消息（本次重点支持）

```typescript
interface SnapshotActivityMessage {
  id: string;
  role: 'activity';
  activityType: 'CUSTOM';
  content: {
    name: 'hitl.interrupt' | string;
    value: unknown; // 可能是 JSON 字符串或对象
  };
}
```

### 2.3 HITL 结构体

```typescript
interface HitlInterruptValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    question: string;
    options: string[];
  };
}
```

## 3. 前端归一化输出（消息模型）

对于可识别的 `hitl.interrupt`，前端统一转换为：

```typescript
interface HitlInterruptMessage {
  role: 'assistant';
  content: HitlInterruptValue;
  __type: 'hitl.interrupt';
  status: 'complete';
}
```

说明：`__type` 为前端渲染标记，仅用于 `MessageContainer` slot 内分支判断。

## 4. 历史只读回显规则

历史 HITL 卡片不再可交互，回显规则如下：

1. 找到当前 HITL 消息的下一条消息。
2. 若下一条不是 `user`，则保持非只读（按普通消息展示链路）。
3. 若下一条是 `user`：
   - 当 `user` 文本 **命中 `options`**：只读 + 高亮该选项。
   - 未命中：只读 + 不回填输入框内容（按“未命中/其它”处理）。

## 5. 降级与容错

- `activityType !== 'CUSTOM'`：按普通消息流程处理。
- `content.name !== 'hitl.interrupt'`：降级为文本 `活动消息：{name}`（或 `活动消息`）。
- `value` 解析失败或结构不合法（缺少 `question/options`）：不走 HITL 卡片，降级文本。
- `options` 中非字符串项会被过滤；若过滤后为空则视为非法。

## 6. API 清单（无变更）

| 接口        | 方法 | 路径                    | 变更 |
| ----------- | ---- | ----------------------- | ---- |
| Chat SSE    | POST | `/api/v1/agent/agui`    | 无   |
| History SSE | POST | `/api/v1/agent/history` | 无   |
| Cancel Run  | POST | `/api/v1/agent/cancel`  | 无   |

## 7. 相关实现文件

- `src/hooks/chatbot/use-stream.ts`：历史 `MESSAGES_SNAPSHOT` 归一化（含 `activity/CUSTOM` 解析）。
- `src/hooks/chatbot/use-event.ts`：实时 `CUSTOM` 事件处理。
- `src/hooks/chatbot/types.ts`：`HitlInterruptValue` / `HitlInterruptMessage`。
- `src/views/index/chatbot/index.vue`：HITL 识别 + 历史只读状态推导。
- `src/views/index/chatbot/children/hitl-interrupt-card.vue`：通用选项卡片（可编辑/只读双态）。
