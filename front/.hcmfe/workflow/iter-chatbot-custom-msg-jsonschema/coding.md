# Coding：aiagent - chatbot 自定义消息展示 - jsonschema 模式（云账号选择）

> 设计原则：**泛化现有 HITL 模式**（`use-hitl.ts` + `hitl-interrupt-card.vue` + `use-event.ts` CUSTOM 分支 + `use-stream.ts` 历史回放分支），新增并列的 `account_select.interrupt` 一套，**不重构** HITL。
> 仅消费 `CUSTOM`（`name=account_select.interrupt`）；提示语是普通 `TEXT_MESSAGE`，沿用现有文本气泡，不处理。

## 1. 变更摘要

| 文件 | 变更 | 类型 |
|------|------|------|
| `hooks/chatbot/types.ts` | 新增 `AccountSelectOption` / `AccountSelectInterruptValue` / `AccountSelectInterruptMessage` 类型 | 改 |
| `hooks/chatbot/use-event.ts` | `CUSTOM` 分支增加 `name === 'account_select.interrupt'`：解析 `value` → push 消息（`__type:'account_select.interrupt'`） | 改 |
| `hooks/chatbot/use-stream.ts` | ① `toHistoryMessage` 增加 `account_select.interrupt` 重建分支 + `parseAccountSelectInterruptValue`；② `streamChat` 支持透传 `resumeValue` | 改 |
| `hooks/chatbot/use-chatbot.ts` | `sendMessage(content, sessionTag?, resumeValue?)` 透传 `resumeValue` 给 `streamChat` | 改 |
| `store/chatbot/agent.ts` | `streamChat(sessionCode, messages, signal, resumeValue?)`：有值时 body 加 `forwardedProps:{ resumeValue }` | 改 |
| `hooks/chatbot/use-account-select.ts` | **新增**：类型守卫 `isAccountSelectMessage` / `getAccountSelectContent` / 只读态推断 `getAccountSelectReadonlyState`（并列 `use-hitl.ts`） | 新增 |
| `hooks/chatbot/vendor-display.ts` | **新增**：`vendor` → `{ name, icon }` 映射（直接 import `@/assets/image/vendor-*.svg` 文件 + `VendorMap` 中文名；`tcloud-ziyan` 复用 `vendor-tcloud.svg`） | 新增 |
| `components/chatbot/account-select-card.vue` | **新增**：厂商卡选择/收起/展开只读三态，响应式（全页横排卡 / 浮窗纵排整行），`enabled=false` 置灰 + tooltip(`reason`) | 新增 |
| `components/chatbot/chat-message-list.vue` | `#default` slot 增加 `account_select` 分支；`onConfirm` 调 `sendMessage(content, undefined, accountId)` | 改 |
| `components/chatbot/account-select-echo.vue` | **新增**：吸顶回显条（绿色对勾 + vendor 图标 + 「厂商名 - 账号名」） | 新增 |

> **吸顶回显挂载实现调整**：未改 `views/chatbot/index.vue` / `components/ai-assistant/index.vue`，改为在 `chat-message-list.vue`（全页 + 浮窗共用）内用 `position: sticky; top: 0` 将回显条固定在消息滚动区顶部——一处改动覆盖两端，符合"吸顶不随对话滚动"，churn 更小。

## 2. 数据模型（types.ts）

```typescript
export interface AccountSelectOption {
  account_id: string;
  account_name: string;
  vendor: string;       // VendorEnum 值，如 'tcloud-ziyan' / 'tcloud'
  enabled: boolean;     // false=置灰不可选
  reason: string;       // enabled=false 时的原因（tooltip）
}

export interface AccountSelectInterruptValue {
  checkpoint_id: string;
  lineage_id: string;
  value: {
    type: 'account_select.interrupt';
    options: AccountSelectOption[];
    // 注意：无 message 字段，提示语由独立 TEXT_MESSAGE 承载
  };
}

export type AccountSelectInterruptMessage = Message & {
  role: MessageRole.Assistant;
  content: AccountSelectInterruptValue;
  status: MessageStatus.Complete;
  __type: 'account_select.interrupt';
};
```

## 3. 下行解析（use-event.ts）

`CUSTOM` 分支与现有 `hitl.interrupt` 并列：

```typescript
case EventType.Custom: {
  const name = event.name as string;
  if (name === 'hitl.interrupt') { /* 现状不动 */ }
  else if (name === 'account_select.interrupt') {
    const raw = JSON.parse(event.value as string) as AccountSelectInterruptValue;
    msg.messages.value.push({
      role: MessageRole.Assistant,
      content: raw,
      id: genId(), messageId: genId(),
      status: MessageStatus.Complete,
      __type: 'account_select.interrupt',
    } as Message);
  }
  break;
}
```

## 4. 上行回写（resumeValue 透传链）

现状：`sendMessage(content)` → `streamChat([{role,content}])` → `agentApi.streamChat(sessionCode, messages)` → body `{ sessionCode, messages }`。

改为可选透传 `resumeValue`（**不破坏现有调用**——HITL `:on-confirm="sendMessage"`、首页大/小卡片仍按位置参数工作）：

- `use-chatbot.ts`：`sendMessage(content, sessionTag = sceneTag, resumeValue?)` → `streamChat([{role:'user',content}], resumeValue)`
- `use-stream.ts`：`streamChat(userMessages, resumeValue?)` → `agentApi.streamChat(sessionCode.value, userMessages, signal, resumeValue)`
- `agent.ts`：

```typescript
export const streamChat = (sessionCode, messages, signal?, resumeValue?: string) =>
  fetch(`${getBaseUrl()}/api/v1/agent/agui`, {
    method: 'POST', headers: sseHeaders(), credentials: 'include', signal,
    body: JSON.stringify({
      sessionCode, messages,
      ...(resumeValue ? { forwardedProps: { resumeValue } } : {}),
    }),
  });
```

- 卡片点选确认：`onConfirm(accountId)` → `sendMessage(交互文案, undefined, accountId)`。
  - `content` 取交互文案（如「我选择 自研云 - ieg-ziyan」，便于对话气泡可读 + 兜底）；`resumeValue` 取 `account_id`。
  - 兜底：用户不点选直接手动输入走原 `sendMessage`，无 `resumeValue`，后端用 `content` 当 accountID。

## 5. 卡片渲染（account-select-card.vue）

参考 `hitl-interrupt-card.vue` 的 props 形态（`content` / `onConfirm` / `readonly` / `readonlyValue`）。

**Props**：`content: AccountSelectInterruptValue`、`onConfirm: (accountId: string) => void`、`readonly?: boolean`、`readonlyValue?: string`（已选 account_id）。

**三态**（对齐 design §2.1~§2.3）：
1. **选择态**：横排厂商卡，每张 = vendor 图标 + 厂商名（加粗）+ 账号名（弱色）。
   - `enabled=false`：置灰、不可点、无 hover、hover **tooltip 展示 `reason`**（bkui `Popover`/`v-bk-tooltips`）。
   - 选中：蓝框 + 右上角对勾角标；单选。底部「确认」按钮（未选禁用）。
2. **收起态**（确认后）：整行「✓ 您已选择【厂商名】- 账号名」+ 行尾 chevron `⌄`，点击展开。
3. **展开只读态**：收起摘要行（`⌃`）+ 下方厂商卡只读回显（已选项保留角标，无 hover、不可切换）。

**响应式（design §0.1）**——同一份 options，容器宽窄自适应：
- 宽（全页）：`.options` flex-wrap 横向卡片，超出换行。
- 窄（浮窗）：纵向整行铺满，右侧预留附加信息位（配额本迭代无数据→不展示，QA4）。
- 用容器查询/`ResizeObserver` 或 CSS `@container`/flex-wrap 自适应（优先纯 CSS flex-wrap + min-width，避免 JS 测量）。

**确认后本地态**：`isConfirmed` 后转收起态（live 流可靠，自身记住所选 option，无需依赖历史推断）。

## 6. 吸顶回显（account-select-echo.vue + 两处容器）

- design §2.4/§3.4：选择云账号后，**主内容区/窗口内容区顶部**常驻一条回显条（不随滚动），内容「厂商名 - 账号名」。
- 数据来源：`use-account-select.ts` 派生 `selectedAccountEcho`——取**当前会话内最后一条已确认**的 account_select 选择（live：卡片确认后的选中项；history：见 §7）。
- 挂载点：全页 `views/chatbot/index.vue` 消息列表上方；浮窗 `components/ai-assistant/index.vue` 内容区上方。仅"云账号已选"语义出现时显示。

## 7. 历史回放（use-stream.ts toHistoryMessage）

- 增加分支：`role==='activity' && activityType==='CUSTOM' && content.name==='account_select.interrupt'` → `parseAccountSelectInterruptValue(content.value)` 重建消息（`__type` 同上）。
- 只读态（design §3.5）：
  - **已提交** → 收起态摘要「您已选择…」；需知道所选 account_id。
  - **未提交即中断** → 只读空态（展示选项不可操作）。
- ✅ **QA2 已用真实 `/history` 核对**：CUSTOM 持久化结构同实时；所选账号在「下一条 user 消息 `content`」+「`ACTIVITY_DELTA` `/resume`」两处。
  - `getAccountSelectReadonlyState` 按下一条 user 消息 `content` 命中 `account_id`/`account_name` 还原已选账号；前端生成的 `content` 含 `account_name`，round-trip 还原正确（已实测）。
  - 无后续 user 消息（未提交中断）→ 保持可交互（对齐 HITL）。

## 8. vendor 映射（vendor-display.ts）

- **直接 import svg 文件**（遵循 QD4：用图标文件，非复用 `vendorProperty` 代码对象）：
  `vendor-tcloud.svg` / `vendor-aws.svg` / `vendor-azure.svg` / `vendor-gcp.svg` / `vendor-huawei.svg`。
- 映射：`vendor` → `{ name, icon }`。`name` 用 `VendorMap`（`tcloud-ziyan`→自研云、`tcloud`→腾讯云、`aws`→亚马逊云…）；`icon`：`tcloud-ziyan` 复用 `vendor-tcloud.svg`，其余按 vendor 取。
- 未知 vendor 兜底：`name` 用原值、`icon` 用默认（不报错）。

## 9. 不在本次实现 / 已知边界

- **多选 / 自定义输入**：框架预留，本迭代云账号仅单选（QD3）。
- **配额展示**：真实 payload 无 quota 字段 → 不展示（QA4）；后端加字段后前端按数据驱动展示。
- **单账号跳过**：由 agent 侧决定是否下发选择步骤（前端只渲染收到的 options）；前端不做"只有 1 个就跳过"的额外逻辑（agent 不下发即不展示）。
- **历史已提交态的账号还原**：依赖 QA2，未确认前为 best-effort。

## 10. 验证建议（手测，详见 test 阶段）

- 收到 `account_select.interrupt`：提示语气泡 + 厂商卡一排（全页横排 / 浮窗纵排）。
- `enabled=false` 卡片置灰、不可选、hover 显示 `reason` tooltip。
- 点选可用账号 → 蓝框+角标 → 确认 → 收起摘要行 + 顶部吸顶回显；`/agui` 请求体带 `forwardedProps.resumeValue=account_id`。
- 点收起态整行 → 展开只读回显（不可切换）→ 再收起。
- 刷新/切会话回放历史：已提交渲染收起态（QA2 确认前 best-effort），未提交渲染只读空态。
- lint 通过。
```
