# Coding — fix-chatbot-history-focus-flash

> 工作流：`fix-chatbot-history-focus-flash`（lite: coding → test → done）  
> 需求：`req-aiagent-host-apply-chatbot` / TAPD `#1069995598136904671`  
> 需求文档：`docs/reqs/卡片获焦闪烁.md`（已定稿回写）  
> 落码入口：无 page-*/comp-*；沿用现有 chatbot hooks（相邻实现：`feat-aiagent-session-sync`）

## 执行顺序

1. **保护窗口** — 卡片本地态写入时打点，被动刷新可跳过  
2. **无闪烁合并** — SNAPSHOT 应用改为「离屏构建 → 还原 meta → 一次赋值」  
3. **被动刷新统一入口** — 获焦 / visibility / session-updated 走同一守卫  
4. **双入口接入** — 全页 + 浮窗共用上述能力  

## 共享改动 / 提交策略

- 跨单公共改动：无（单绑）  
- 提交策略：一单一提交（前端改动）

---

## 根因（现网代码）

1. 全页 `views/chatbot/index.vue` 与浮窗 `components/ai-assistant/index.vue` 在 `visibilitychange` / `window.focus`（~300ms debounce）调用 `refreshCurrentSession()`。  
2. `refreshCurrentSession`（`use-session.ts`）→ `fetchHistory`；`MESSAGES_SNAPSHOT` 内执行 `msg.messages.value = []` 再逐条 push（`use-stream.ts`）。  
3. `captureCardClientMeta` / `restoreCardClientMeta` 在 **await fetchHistory 之后**才 restore → SNAPSHOT 处理到 restore 之间，Vue 已渲染「无 `__*` 的旧交互卡」。  
4. 保护缺口：`isChatting` 只挡流式中；卡片点击后本地 `__confirmed` / `__submitted` 等已写、agui 未再置 `isChatting` 时，获焦仍会拉 history。服务端滞后时快照无 resume → 闪回可点态。

## 总体方案（前端 only）

| 能力 | 做法 |
|------|------|
| F-001/F-003 被动刷新 | 统一 `refreshCurrentSession({ reason: 'passive' \| 'active' })`；passive 受保护窗口约束 |
| F-002 短窗口跳过 | 本地卡片运行态写入时 `touchCardActionProtect()`；窗口内 skip passive refresh |
| F-002/F-004 无闪烁 + 本地优先 | `fetchHistory` 离屏构建 nextMessages，带入 pre-captured meta 后再一次性赋给 `messages`；meta 优先于快照缺字段 |
| F-005 主动路径 | `switchSession` / 主动刷新 `reason:'active'` 不走保护跳过；仍用原子赋值避免中间帧 |

不改后端 history/agui 契约。

## 改动文件

| 文件 | 改动 |
|------|------|
| `src/hooks/chatbot/card-action-protect.ts`（新） | 保护窗口常量、`touchCardActionProtect`、`isCardActionProtected` |
| `src/hooks/chatbot/use-stream.ts` | `fetchHistory(code, onSnapshot?, opts?)`：SNAPSHOT 离屏构建 + 可选 `clientMeta` 在赋值前 restore |
| `src/hooks/chatbot/use-session.ts` | `refreshCurrentSession(opts?)`：passive + 保护窗口 skip；把 capture 的 meta 传入 fetchHistory |
| `src/hooks/chatbot/use-chatbot.ts` | 导出 `touchCardActionProtect`；`session-updated` 用 `reason:'passive'` |
| `src/components/chatbot/chat-message-list.vue` | 写 `__selectedIndex` / `__confirmedSuborders` / `__submitted` / `__selectedAccountId` 时 touch protect |
| `src/views/chatbot/index.vue` | 获焦刷新改为 passive（若 API 需显式传） |
| `src/components/ai-assistant/index.vue` | 同上；`show()` 主动打开面板可用 `active` 或保持刷新但走原子合并 |

**文件**（boundFiles）：  
`src/hooks/chatbot/card-action-protect.ts`  
`src/hooks/chatbot/use-stream.ts`  
`src/hooks/chatbot/use-session.ts`  
`src/hooks/chatbot/use-chatbot.ts`  
`src/components/chatbot/chat-message-list.vue`  
`src/views/chatbot/index.vue`  
`src/components/ai-assistant/index.vue`

## 详细实现

### 1. `card-action-protect.ts`

```ts
/** 覆盖「点完立刻切屏再回来」的常见间隔；可按手测微调 */
export const CARD_ACTION_PROTECT_MS = 8000;

let protectUntil = 0;

export const touchCardActionProtect = (): void => {
  protectUntil = Date.now() + CARD_ACTION_PROTECT_MS;
};

export const isCardActionProtected = (): boolean => Date.now() < protectUntil;
```

- 模块级即可（每个 `useChatbot` 实例一份内存态；双入口各实例各自保护，符合「本页操作保护本页」）。  
- 若浮窗/全页同页极少同时挂载；多标签各自实例独立保护正确。

### 2. `fetchHistory` 原子替换（消闪关键）

现状（闪烁源）：

```ts
msg.messages.value = [];
// ... push ...
// restore 在 await 外
```

改为：

```ts
const next: Message[] = [];
// 原 push 逻辑改为 next.push(...)
restoreCardClientMeta(next, opts?.clientMeta ?? new Map());
msg.messages.value = next; // 一次赋值；列表项已带 __* 字段
```

- `switchSession` 路径：仍可先清空 loading 区；SNAPSHOT 同样原子赋值。  
- `onSnapshotLoaded` 在赋值后调用。  
- **本地优先**：`restoreCardClientMeta` 只在目标字段空时写入现有实现已是「有 meta 则写」；确保 capture 含刚写入的 `__*`。若快照侧已有 resume_forwarded，与 meta 一致则幂等。

### 3. `refreshCurrentSession`

```ts
type RefreshOpts = { reason?: 'passive' | 'active' };

const refreshCurrentSession = async (opts: RefreshOpts = {}) => {
  const reason = opts.reason ?? 'passive';
  // 既有守卫：无 code / isChatting / 空会话
  if (reason === 'passive' && isCardActionProtected()) return;

  const clientMeta = captureCardClientMeta(deps.messages.value);
  await deps.fetchHistory(code, undefined, { clientMeta });
  // restore 已在 fetchHistory 内完成；此处仅写回 session 快照
  if (currentSessionCode.value === code) {
    target.messages = [...deps.messages.value];
  }
};
```

- 获焦 / visibility / session-updated：`reason: 'passive'`（默认）。  
- 将来若有「用户点刷新」：`reason: 'active'`。  
- `sendMessage` 里 resume 前刷新：保持现有调用；建议 `active` 或单独保留（属发起前一致性，不在保护 skip 之列——若与保护冲突：resume 前刷新应带着刚 touch 的 meta 做原子合并，**不要 skip**，否则可能丢服务端新气泡。口径：**保护窗口只跳过「无用户主动意图」的 passive 刷新**；resume 前刷新不算 passive skip）。

### 4. 卡片写入点 touch

在 `chat-message-list.vue`：

- `handleSelectPlan`：设 `__selectedIndex` 后 `touchCardActionProtect()`  
- `handleConfirmPreorder`：设 `__confirmedSuborders` 后 touch  
- `handleSubmitConfirm`：设 `__submitted` 后 touch  
- 账号选择写入 `__selectedAccountId` 处同样 touch  

先 touch 再 `sendMessage`，保证紧随其后的 focus 被 skip。

### 5. 入口

- `views/chatbot/index.vue` / `ai-assistant/index.vue`：`handleVisibilityRefresh` → `refreshCurrentSession({ reason: 'passive' })`（若默认已是 passive 可只改 session 层）。  
- `ai-assistant` `show()`：面板打开拉取用 `active`（用户显式打开，应对齐服务端；仍走原子合并，无闪）。  
- `use-chatbot` `session-updated`：`refreshCurrentSession({ reason: 'passive' })` 后仍 `restoreCardClientMeta(remoteMeta)`（广播已提交态；与本地 protect meta 合并时：remote 与 local 并存，restore 两次即可——先 refresh 内 local meta，再 remote committed meta）。

### 6. 边界与回归

| 场景 | 期望 |
|------|------|
| 点确认后立刻 alt-tab 再点回 | 保护窗口内不发 history；无闪 |
| 保护窗口外获焦 | 拉 history，但原子合并，不出现可点旧卡中间帧 |
| 服务端仍无结果 | 本地 `__*` 保留，不回退 |
| 冷启动 / 切会话 | 正常拉历史 |
| 跨标签 session-updated | 同类保护 + 合并；草稿不跨签（既有 captureCommitted） |
| isChatting | 仍 skip（既有） |

## 验收映射

| AC | 实现落点 |
|----|----------|
| AC-001/002 | 双入口 passive + protect + 原子合并 |
| AC-003/004 | clientMeta 本地优先 + session-updated passive |
| AC-005 | switchSession / show active 不受 protect skip |
| AC-006 | debounce 保留 + protect skip |
| AC-P01 | 禁止 `messages=[]` 中间态 |

## 风险

- 保护窗口过短仍闪 → 手测调 `CARD_ACTION_PROTECT_MS`。  
- 保护窗口过长可能短暂挡住跨签已提交同步 → 可接受；窗口结束后下次 passive 仍会合并。  
- messageId 对不上 → 沿用现 metaKey；对不上时不得清空已有 `__*`（原子路径下旧 messages 不被中间清空暴露）。

## 状态

- [x] 实现  
- [x] `bkdevbuddy_lint`（通过）  
- [ ] 手测 AC-001~006  

### 改动点（落地）

- 新增 `card-action-protect.ts`：8s 保护窗  
- `fetchHistory`：离屏构建 + 同步帧内 restore clientMeta  
- `refreshCurrentSession({ reason })`：passive 受保护窗 skip；active 不 skip  
- 卡片写入点 touch；全页/浮窗获焦 passive；浮窗 show active；session-updated / 续跑前刷新按 reason 区分  

