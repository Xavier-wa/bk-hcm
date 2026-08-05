# Coding：chatbot 小优化集合

> 工作流：`feat-aiagent-chatbot-patch1`（lite）  
> 本工作流承载多个 chatbot 小优化，按任务分小节。

## 执行顺序

1. `1069995598136109206` 默认提示词修改（已完成）
2. `1069995598136110822` 停止生成后前端提示（已完成；历史单，当前绑定已不含）
3. `1069995598136381200` 切换窗口时闪现「停止生成」按钮（编码完成）
4. `1069995598136401202` 首页大卡片快速点击重复请求（防抖）（编码完成）
5. `1069995598136402240` 主机申请浮窗入口缺少权限控制（编码完成）

## 共享改动 / 提交策略

- 均落在 `hooks/chatbot` / `components/chatbot` / `components/ai-assistant` / `views/chatbot`，同分支按单据分 commit。
- 三张新单已分别提交并合入 `env-devhk-frontend`；TAPD 已追至 `for_test`。

---

# 任务一：默认提示词修改

> 需求文档：`docs/reqs/默认提示词.md`  
> TAPD：1069995598136109206

## 1. 目标

将 Chatbot 输入框空态占位提示从组件库默认四行，收敛为仅保留换行提示：

```
通过 Shift + Enter 进行换行输入
```

去掉：
- 输入 "/" 唤出 Skill
- 输入 "\\" 唤出 Prompt
- 输入 "@" 唤出 工具和 MCP

全页 Chatbot 与业务浮窗 AI 助手共用同一输入组件，一处修改两端生效。

## 2. 改动点

| 文件 | 改动 |
|------|------|
| `src/components/chatbot/chat-input-box.vue` | 向 `ChatInput` 传入 `placeholder`，覆盖 `@blueking/chat-x` 默认四行文案 |

可选（若希望文案集中管理）：

| 文件 | 改动 |
|------|------|
| `src/views/chatbot/constants.ts` | 新增 `INPUT_PLACEHOLDER` 常量，供 `ChatInputBox` 引用 |

**建议**：常量放在 `chat-input-box.vue` 同文件或 `constants.ts` 均可；优先在 `chat-input-box.vue` 内定义常量，改动最小、作用域清晰。

## 3. 实现细节

```vue
<!-- chat-input-box.vue -->
const INPUT_PLACEHOLDER = '通过 Shift + Enter 进行换行输入';

<ChatInput
  ...
  :placeholder="INPUT_PLACEHOLDER"
/>
```

- `ChatInput` 已支持 `placeholder` prop（见 `@blueking/chat-x` 文档 / dist 默认值）。
- 不改动 `prompts` / `skills` / `resources` 等能力开关；仅改展示文案。
- 不影响 `PROMPT_CHIPS` / `BIG_CARDS`。

## 4. 验证点（编码后自测）

1. 全页 `/chatbot` 首页空态：输入框占位仅一行换行提示。
2. 全页进入会话后：占位文案一致。
3. 业务页浮窗 AI 助手：占位文案与全页一致。
4. 小卡片注入 / 大卡片直发行为不变。

## 5. 非目标

- 不实现 `/` Skill、`\\` Prompt、`@` MCP 功能。
- 不改欢迎区 tips、大/小卡片文案。
- 不升级 `@blueking/chat-x` 版本。

---

# 任务二：停止生成后前端提示

> 需求文档：`docs/reqs/停止提示.md`  
> TAPD：1069995598136110822

## 1. 目标

用户主动点击「停止」后，页面始终展示可见的「已停止生成」提示，消除空白气泡。

## 2. 根因

`hooks/chatbot/use-stream.ts`：
- `ToolCallStart` / `TextMessageStart` 会 push 一条 `content: ''` 的流式助手消息。
- `stopGeneration()` abort 后，`streamChat` 的 `finally` 把该流式消息置 `Complete`；若内容为空 → 空白气泡。
- 兜底 `ensureAssistantTail('已停止生成')` 仅在末条为 user 时补提示，空助手消息场景不触发 → 无提示。

## 3. 改动点

| 文件 | 改动 |
|------|------|
| `src/hooks/chatbot/use-stream.ts` | 新增用户主动停止标记；`streamChat` finally 中据此保证「已停止生成」可见 |

## 4. 实现细节

- 新增局部标记 `stoppedByUser`（`stopGeneration()` 中置 `true`）。
- 新增 `ensureStopTip(content)`（提示气泡统一置 `MessageStatus.Error`，复用与「未响应或被停止」一致的提示图标）：
  - 末条为**空内容助手气泡**（`role===Assistant` 且 `typeof content==='string'` 且 `!content.trim()`，如工具调用后正文未产出即被中断）→ 原地填充「已停止生成」并置 `Error` 态，消除空白气泡；
  - 其余情况（**正文已渲染**、卡片消息、或末条为 user）→ **追加一条独立的「已停止生成」助手气泡**（`Error` 态）。
- `streamChat` 的 `finally`（仅当前流 `abortController === controller`）中：
  - 先将当前流式消息置为 `Complete`（保持现状）。
  - 若 `stoppedByUser` → 调 `ensureStopTip('已停止生成')`。
  - 重置 `stoppedByUser = false`。
- 仅覆盖用户主动停止（`stopGeneration`）；`abortStream`（会话切换本地中断）与 `fetchHistory`（history 断点续传，用「未响应或被停止」）不变。

> 验收补充（正文已渲染场景）：SSE 已开始渲染正文后点停止，末条助手气泡已有内容，此时**不**原地覆盖，而是追加一条独立「已停止生成」气泡，避免吞掉已生成正文。

## 5. 验证点（编码后自测）

1. 工具调用后、正文未产出即点停止：空白气泡变为「已停止生成」，且文案前带提示图标（同「未响应或被停止」）。
2. 正文流式输出中点停止：展示「已停止生成」，文案前带提示图标。
3. 浮窗与全页表现一致。
4. 正常生成结束（未点停止）不出现多余「已停止」提示。
5. 会话切换 / 新建会话不误插入「已停止」提示。

## 6. 非目标

- 不改后端 cancel run 逻辑。
- 不调整 history 断点续传文案。
- 不新增系统提示样式（沿用助手气泡）。

---

# 任务三：切换窗口时闪现「停止生成」按钮

> TAPD：1069995598136381200  
> 标题：AI对话-切换窗口时闪现「停止生成」按钮

## 1. 目标

从其他页面切回对话页 / 重新拉取会话历史时，若**没有**正在进行的生成（含断点续传），输入区**不得**出现「停止生成」按钮或发送位停止态。

真正在流式生成（`streamChat`）或 history **断点续传**时，停止按钮行为保持不变。

## 2. 根因

`ChatInputBox` / `ChatMessageList` 用 `isChatting` → `MessageStatus.Streaming` 控制停止按钮。

`fetchHistory`（`use-stream.ts`）在请求一开始就 `isChatting = true`，而全页/浮窗的 `ChatInputBox` **始终渲染**（仅消息区在 `isLoadingHistory` 时显示 loading）。因此：

1. 切回带 session 的对话页 → `initSessions` → `switchSession` → `fetchHistory`
2. SNAPSHOT 到达后关 loading（`isLoadingHistory=false`），但 SSE 尚未 `RUN_FINISHED` / `finally`
3. 这段窗口 `isChatting===true` → 输入区闪现「停止生成」
4. 即使没有断点续传（纯历史快照）也会闪

会话切换时消息区有「加载会话历史...」遮罩，观感上不明显；切页回对话时更易被感知。

## 3. 改动点

| 文件 | 改动 |
|------|------|
| `src/hooks/chatbot/use-stream.ts` | `fetchHistory`：仅在 SNAPSHOT **之后**出现续传实时事件时才置 `isChatting=true`；`abortStream` 改为只要存在 `abortController` 即可 abort（不依赖 `isChatting`，保证会话快速切换仍能打断历史拉取） |

## 4. 实现细节

```ts
// fetchHistory
const controller = new AbortController();
abortController = controller;
// 不再在入口处 isChatting = true
let snapshotDone = false;

await readSSE(reader, (e) => {
  if (e.type === EventType.MessagesSnapshot) {
    // ...现有 SNAPSHOT 处理...
    snapshotDone = true;
    onSnapshotLoaded?.();
    return;
  }
  // SNAPSHOT 后的非 RunFinished 事件 = 断点续传，此时才进入「生成中」
  if (snapshotDone && e.type !== EventType.RunFinished && !isChatting.value) {
    isChatting.value = true;
  }
  event.handleEvent(e);
  if (e.type === EventType.RunFinished) return true;
});

// finally：保持现有收尾（Complete / ensureAssistantTail / isChatting=false）
```

```ts
// abortStream：会话切换 / goHome 需能打断「尚未置 isChatting」的历史拉取
const abortStream = () => {
  abortController?.abort();
};
```

约束：

- `streamChat` 仍在入口置 `isChatting=true`（用户主动发送）。
- `stopGeneration` 仍以 `isChatting` 为门闩（纯历史加载阶段不应可点停止 / 不应调 cancel）。
- 纯 SNAPSHOT + `RunFinished`（无续传）全程不置 `isChatting`，无停止按钮闪现。
- 有续传时，首个 SNAPSHOT 后实时事件再亮停止按钮，行为正确。

## 5. 验证点（编码后自测）

1. 打开有历史的会话 → 离开到其他业务页 → 再切回对话页：加载过程与加载完成后均**不**闪「停止生成」。
2. 会话列表间切换：加载历史时输入区保持发送态（非停止态）；续传中仍显示停止。
3. 空会话 / 首页空态：无停止按钮。
4. 正常发送生成中：停止按钮正常；点停止后「已停止生成」提示仍正常（任务二）。
5. 快速连续切换会话：旧 history 请求被 abort，无串会话 / 无残留 isChatting。

## 6. 非目标

- 不改 `isLoadingHistory` UI / 不隐藏输入框。
- 不引入独立 `isLoadingHistory`→UI 映射到 chat-x（根因在 `isChatting` 误用）。
- 不做多入口可见性刷新（属另一需求 `多会话状态同步`）。

---

# 任务四：首页大卡片快速点击重复请求（防抖）

> TAPD：1069995598136401202  
> 标题：AI对话-首页大卡片快速点击重复请求（防抖）

## 1. 目标

首页大卡片（「自研云主机申请」「资源查询」）快速连点时只触发**一次**发送，不产生重复请求。

## 2. 根因

`handleBigCardClick` 仅用 `isChatting` 门闩，但 `isChatting` 要等 `sendMessage` →（可能 `createSession`）→ `streamChat` 入口才置 true。连点落在该异步窗口内会多次进入 `sendMessage`。

另：大卡片是 `div`，`:disabled` / CSS `&:disabled` 对非表单元素基本无效，视觉禁用不可靠。

## 3. 改动点

| 文件 | 改动 |
|------|------|
| `src/views/chatbot/index.vue` | 同步 `isBigCardSending` 互斥锁；模板用 `is-disabled` class；CSS 改 `.is-disabled` |

## 4. 实现细节

```ts
const isBigCardSending = ref(false);

const handleBigCardClick = async (card: BigCard) => {
  if (isChatting.value || isBigCardSending.value) return;
  isBigCardSending.value = true;
  try {
    pendingChip.value = null;
    await sendMessage(card.prompt, card.sessionTag || '');
  } finally {
    isBigCardSending.value = false;
  }
};
```

```vue
<div
  class="big-card"
  :class="{ 'is-disabled': isChatting || isBigCardSending }"
  @click="handleBigCardClick(card)"
>
```

```scss
.big-card {
  &.is-disabled {
    cursor: not-allowed;
    opacity: 0.6;
    pointer-events: none;
  }
}
```

说明：用**进行中互斥**（非时间窗 debounce），更贴合「请求进行中不可重复触发」。

## 5. 验证点

1. 首页空态快速连点同一大卡片：只发一次请求 / 只建一条用户消息。
2. 快速交替点两张大卡片：仍只生效第一次。
3. 正常单击：行为不变。
4. 生成结束后可再次点击大卡片（需先回到首页空态或相应入口）。

## 6. 非目标

- 不改小卡片 / 输入框发送防重（本期仅大卡片）。
- 不改 `sendMessage` 全局锁（若后续仍有其它入口竞态再补）。

---

# 任务五：主机申请浮窗入口缺少权限控制

> TAPD：1069995598136402240  
> 标题：AI对话-主机申请浮窗入口缺少权限控制

## 1. 目标

浮窗入口与业务「首页」菜单权限一致：无 `biz_agent_assistant` 时不显示悬浮球 / 面板。

## 2. 根因

- 菜单：`route-config` `checkAuth: 'biz_agent_assistant'` → 无权限隐藏。
- 浮窗：`service-apply/cvm` 无条件挂载 `AiAssistant`，组件内无权限判断。

## 3. 改动点

| 文件 | 改动 |
|------|------|
| `src/components/ai-assistant/index.vue` | 读取 `authVerifyData.permissionAction.biz_agent_assistant`；无权限不渲染 teleport；`show`/`toggle`/`initSessions` 早退 |

## 4. 验证点

1. 无 `biz_agent_assistant`：首页菜单不可见，主机申请页无浮窗入口。
2. 有权限：浮窗正常；深链 `sessionCode` 唤起正常。
3. 快捷键 Cmd/Ctrl+I：无权限不唤起。
