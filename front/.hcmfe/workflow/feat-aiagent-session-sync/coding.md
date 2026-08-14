# coding.md — 多个会话状态同步（读一致性）

> 工作流：`feat-aiagent-session-sync`（lite: coding → test → done）
> 需求文档：`front/docs/reqs/多会话状态同步.md`（已定稿并回写 TAPD）
> 交付目标：**读一致性**（非活跃入口/标签页展示最新会话内容）。
> 本期不做：409 专门处理与冲突 UX、后端 `bk_biz_id` 类型问题（仅标注「需后端配合」）、会话列表跨端实时同步、轮询。
>
> **用户确认（2026-07-15）**：F-001、F-002、F-003 **全部本期落地**；可见/聚焦刷新加 **~300ms 去抖**；F-002 口径为「刷新后照常提交，409 交由现有错误路径，不新增冲突 UX」。
>
> **用户确认（2026-07-15，test 实测补充）**：新增 **F-004 in-flight 窗口锁定**。实测发现 F-003 只在一轮对话完整结束后才广播，动作发起到结果返回的 5-10s 窗口内其它标签页仍显示可点的卡片，存在并发重复操作风险且「同步」不实时。口径：**动作发起即广播 `session-busy`**；**仅锁「当前存在 pending 可交互卡片」的会话**（无 pending 卡片的会话不受影响）；锁定表现为**卡片顶部黄色横幅提示 + 操作按钮禁用（不折叠成「已选择」摘要）**；文本输入不锁；**全部 `sendMessage`/`regenerate`/`resendEdited` 发起时都广播**；完成后 `session-updated` 解锁并刷新；**20s 兜底自动解锁**（防发起端异常关闭卡死）。不做即时拉取（收到 busy 只锁不额外 `/history`）。

## 一、根因（基于 `feat-aiagent-session-sync` 分支现有代码复核）

1. **双入口各自独立内存态**：
   - 浮窗 `src/components/ai-assistant/index.vue:40` → `useChatbot({ sceneTag })`。
   - 全屏 `src/views/chatbot/index.vue:27` → `useChatbot()`。
   两者的 `messages / sessions / currentSessionCode` 互不共享，也无跨标签页机制。
2. **会话历史仅拉取一次**：`use-session.ts` 中只有 `switchSession`（`:118`）/ `initSessions`（`:217`）会触发 `fetchHistory`；之后停留在本地快照。
3. **无法刷新「当前会话」**：`switchSession`（`use-session.ts:119`）`if (code === currentSessionCode.value) return;` 直接早退——同一会话再次进入不会重新拉历史。
4. **`fetchHistory` 是追加式**：`use-stream.ts:496-510` 在 `MESSAGES_SNAPSHOT` 上对 `msg.messages.value.push(...)`，**从不清空**；清空动作在 `switchSession:127`（`deps.messages.value = []`）里完成。因此若直接对当前会话再次 `fetchHistory` 会**重复追加**。
5. **单一 `abortController`**：`streamChat` 与 `fetchHistory` 共用 `abortController`（`use-stream.ts:64`），二者都会 `isChatting=true`。→ **正在流式输出（`isChatting`）时不得触发刷新**，否则会顶掉进行中的流。

> 说明：同一浏览器标签内「浮窗 + 全屏」通常不会同时挂载（全屏是独立路由页，浮窗挂在业务页）。用户场景中的「同时打开」实质是**多标签页/多窗口**，或「一个标签浮窗 + 另一标签全屏」。因此跨实例一致性主要靠**可见/聚焦时刷新**（F-001/F-002）+ **可选 BroadcastChannel**（F-003）达成，而非共享单例。

## 二、总体方案

新增一个「刷新当前会话历史」的能力，并在两处入口接入「可见/聚焦」触发；提交/续跑前复用同一刷新做轻量校验。全部为 `front/` 前端改动，不涉及后端。

### 改动清单（文件级）

| 文件 | 改动 | 对应功能点 |
|------|------|-----------|
| `src/hooks/chatbot/use-stream.ts` | `fetchHistory` 收到 `MESSAGES_SNAPSHOT` 时先清空再填充，使其变为「替换式」，支持无闪烁刷新 | 支撑 F-001/F-002 |
| `src/hooks/chatbot/use-session.ts` | 新增 `refreshCurrentSession()`；`SessionDeps` 增加 `isChatting` 依赖用于守卫 | F-001/F-002 |
| `src/hooks/chatbot/use-chatbot.ts` | 组装时把 `streamModule.isChatting` 传入 `useSession`；导出 `refreshCurrentSession`；`sendMessage` 在 `resumeValue` 场景下先 `await refreshCurrentSession()` | F-002 |
| `src/components/ai-assistant/index.vue` | 面板 `panelVisible` false→true 时刷新；面板打开期间监听 `visibilitychange`/`focus`（~300ms 去抖）刷新 | F-001（浮窗） |
| `src/views/chatbot/index.vue` | 监听 `visibilitychange`/`focus`（~300ms 去抖）刷新当前会话 | F-001（全屏） |
| `src/hooks/chatbot/use-chatbot.ts` + 两入口 | BroadcastChannel 广播/接收写完成事件触发刷新 | F-003（本期落地） |
| `src/hooks/chatbot/use-chatbot.ts` | 动作发起广播 `session-busy`、完成广播 `session-updated`（三入口 try/finally）；维护 `remoteBusySessions` 集合 + 20s 兜底定时器；导出 `isCurrentSessionRemoteBusy` | F-004 |
| `src/components/chatbot/chat-message-list.vue` | 注入 `isCurrentSessionRemoteBusy`，将 `locked` 传给各交互卡片 | F-004 |
| `src/components/chatbot/{hitl-interrupt-card,account-select-card,host-apply-recommend-card,host-apply-preorder-card,host-apply-submit-card}.vue` | 新增 `locked` prop：黄色横幅提示 +（在未只读时）禁用操作按钮/选项 | F-004 |

## 三、详细实现方案

### 3.1 `use-stream.ts`：`fetchHistory` 改为「替换式」（关键前置）

在 `MESSAGES_SNAPSHOT` 分支，**push 前先清空一次**，使 `fetchHistory` 幂等可复用（切换会话场景 `switchSession` 已提前清空，此处再清空为幂等无副作用；刷新场景则实现「旧内容保留到快照到达后一次性替换」，避免闪烁）：

```ts
if (e.type === EventType.MessagesSnapshot) {
  const items = (e.messages as Record<string, unknown>[]) || [];
  msg.messages.value = []; // 新增：快照到达即以服务端全量为准替换，支持无闪烁刷新
  let lastRecommend: HostApplyRecommendMessage | undefined;
  // ...原有逻辑不变（改为 push 到已清空列表）
}
```

- 风险控制：`switchSession` 调用前已 `deps.messages.value = []`，此处再清空无影响；`initSessions`→`switchSession` 同理。仅改变「刷新」这一新路径的观感。
- 竞态：快照替换发生在单次 `readSSE` 内，且 `refreshCurrentSession` 会加会话码守卫（见 3.2）。

### 3.2 `use-session.ts`：新增 `refreshCurrentSession()`

`SessionDeps` 增加只读依赖 `isChatting: Ref<boolean>`（来自 stream 模块），用于守卫。

```ts
// 刷新「当前选中会话」的历史，使本地快照与服务端一致（读一致性核心）。
// 守卫：无选中会话 / 正在流式输出 / 空会话 → 跳过，避免无谓请求与打断进行中的流。
const refreshCurrentSession = async () => {
  const code = currentSessionCode.value;
  if (!code) return;                 // 首页空态
  if (deps.isChatting.value) return; // 不打断进行中的对话/续跑
  const target = sessions.value.find((s) => s.sessionCode === code);
  if (!target) return;
  if (target.sessionContentCount === 0 && target.messages.length === 0) return; // 空会话无需刷新

  try {
    // 不预清空 messages：由 fetchHistory 在快照到达时一次性替换（3.1），避免闪烁
    await deps.fetchHistory(code, () => {
      /* 快照已替换，无需额外处理；不切 isLoadingHistory 以保持静默刷新 */
    });
    if (currentSessionCode.value === code) {
      target.messages = [...deps.messages.value]; // 竞态守卫：仅当仍是同一会话才写回
    }
  } catch (err) {
    // 刷新失败：保留原有本地内容，静默降级（不弹错、不清空）
    console.warn('[Session] refreshCurrentSession failed, keep local snapshot:', err);
  }
};
```

- 导出 `refreshCurrentSession`。
- 说明：**不**设置 `isLoadingHistory=true`（静默刷新），避免每次聚焦/可见都全屏 loading 抖动。旧消息在快照到达前保持可见，到达后原子替换。

### 3.3 `use-chatbot.ts`：接线 + F-002

1. 组装 `useSession` 时传入 `isChatting`：

```ts
const sessionModule = useSession({
  messages: messageModule.messages,
  sessionCode: streamModule.sessionCode,
  getBkBizId: getBizsId,
  abortStream: streamModule.abortStream,
  fetchHistory: streamModule.fetchHistory,
  isChatting: streamModule.isChatting, // 新增
  sceneTag,
});
```

2. F-002：`sendMessage` 中，仅在**续跑/提交（存在 `resumeValue`）**时于发起前做一次轻量校验刷新（普通首轮发送不加，以免徒增延迟）：

```ts
const sendMessage = async (content, sessionTag = sceneTag, resumeValue?, forwardedProps?) => {
  // F-002：续跑/提交前确保当前会话为最新（读一致性的一部分；不做 409 专门处理）
  if (resumeValue && sessionModule.currentSession.value) {
    await sessionModule.refreshCurrentSession();
  }
  // ...原有逻辑不变
};
```

3. `return` 中导出 `refreshCurrentSession: sessionModule.refreshCurrentSession`。

- **F-002 边界说明（本期口径）**：刷新后**照常发起**续跑请求；若服务端因他处已推进而返回 409，沿用现有错误渲染路径（`use-stream.ts:429-439` / `fetchHistory` 的 tail 占位），**不**新增定向冲突提示。刷新的价值在于：他处已推进时，本处会先把过期卡片更新为最新态（例如已提交/已消费），减少用户对着过期卡片继续操作。「刷新后若卡片已不可提交则主动拦截提交」属 409 处理，本期不做。
- 代价：每次续跑前多一次 `/history` 往返（轻量）。此为用户明确要求，接受该延迟。

### 3.4 `ai-assistant/index.vue`（浮窗）接入 F-001

- 取用 `refreshCurrentSession`（从 `chatbot` 解构）。
- **面板 false→true**：`show()` 已存在；在 `panelVisible` 由隐藏变可见时刷新。用 `watch(panelVisible, ...)` 或在 `show()` 内 `ensureSessions()` 之后调用 `refreshCurrentSession()`（首次 `initSessions` 时当前无选中会话，`refreshCurrentSession` 会自守卫跳过，无冲突）。
- **标签页聚焦 / 页面可见**：`onMounted` 注册 `document` 的 `visibilitychange`、`window` 的 `focus`；回调中**仅当 `panelVisible.value` 为真**时 `refreshCurrentSession()`（面板收起时不刷新，省请求）。`onBeforeUnmount` 注销。

```ts
const { /* ... */ refreshCurrentSession } = chatbot;

const handleVisibilityRefresh = () => {
  if (document.visibilityState !== 'visible') return;
  if (!panelVisible.value) return;
  refreshCurrentSession();
};

onMounted(() => {
  window.addEventListener('keydown', handleKeydown);
  document.addEventListener('visibilitychange', handleVisibilityRefresh);
  window.addEventListener('focus', handleVisibilityRefresh);
});
onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown);
  document.removeEventListener('visibilitychange', handleVisibilityRefresh);
  window.removeEventListener('focus', handleVisibilityRefresh);
});

const show = () => {
  panelVisible.value = true;
  ensureSessions();
  refreshCurrentSession(); // 面板重新可见时刷新当前会话
};
```

### 3.5 `views/chatbot/index.vue`（全屏）接入 F-001

- 取用 `refreshCurrentSession`。
- 全屏页常驻可见，主要覆盖「标签页切走再切回 / 窗口重新聚焦」：`onMounted` 注册 `visibilitychange`/`focus`，回调 `document.visibilityState === 'visible'` 时 `refreshCurrentSession()`；`onBeforeUnmount` 注销（复用现有 `onBeforeUnmount`）。

```ts
const handleVisibilityRefresh = () => {
  if (document.visibilityState === 'visible') refreshCurrentSession();
};
onMounted(() => {
  initSessions(readRouteSessionCode() || undefined);
  document.addEventListener('visibilitychange', handleVisibilityRefresh);
  window.addEventListener('focus', handleVisibilityRefresh);
});
onBeforeUnmount(() => {
  navObserver?.disconnect();
  chipsResizeObserver?.disconnect();
  chipsResizeObserver = null;
  document.removeEventListener('visibilitychange', handleVisibilityRefresh);
  window.removeEventListener('focus', handleVisibilityRefresh);
});
```

### 3.6 F-003 跨标签页广播（本期落地）

- 在 `use-chatbot.ts` 内建一个 `BroadcastChannel('hcm-chatbot-sync')`（需判空降级：`typeof BroadcastChannel !== 'undefined'`，SSR/老浏览器无此 API 时静默跳过）。
- **发**：一次对话/续跑写完成后（`sendMessage` 末尾，会话内容已 +1）`postMessage({ type: 'session-updated', sessionCode })`。
- **收**：`onmessage` 时，若 `data.sessionCode === currentSessionCode.value` 且 `!isChatting.value` → `refreshCurrentSession()`（复用 3.2 的守卫，天然幂等安全）。
- hook 作用域通过 `onScopeDispose` 关闭 channel（`close()`），避免泄漏。
- 价值：同机多标签页/多窗口近实时同步，无需等待聚焦；不采用轮询。

### 3.7 F-004 in-flight 窗口锁定（本期落地）

**问题**：3.6 的 `session-updated` 只在一轮对话/续跑**完整结束**后才广播。动作发起（选择方案/确认预提单/确认提交/账号选择）到结果返回通常 5-10s，这段窗口内其它标签页既未收到任何信号、卡片也未只读，用户可在他处对**同一张过期卡片**再次点击，导致重复 resume（并发/409）且「同步」明显滞后。

**方案**：在 `session-updated`（完成态）之外，新增 `session-busy`（发起态）广播，让其它持有相同会话且**存在 pending 可交互卡片**的实例在窗口期立即锁定该卡片。

- **广播（发起端，`use-chatbot.ts`）**：`sendMessage` / `regenerate` / `resendEdited` 三入口，在**会话已存在**（有 `sessionCode`）后、`streamChat` 之前 `postMessage({ type: 'session-busy', sessionCode })`；在各自 `finally` 中 `postMessage({ type: 'session-updated', sessionCode })`（原 F-003 仅 `sendMessage` 末尾广播，现统一收敛到三入口 `finally`，确保异常/中断也能解锁）。
  - 发起端自身无需靠广播锁定：点击时 `handleSelectPlan`/`handleConfirmPreorder`/`handleSubmitConfirm` 等已就地写入 `__selectedIndex`/`__confirmedSuborders`/`__submitted` 使本地卡片即时只读；且 BroadcastChannel 不回投自身。
- **接收（`use-chatbot.ts`）**：维护 `remoteBusySessions = ref<Set<string>>`。
  - 收到 `session-busy` → 将 `sessionCode` 加入集合，并设置 **20s** 定时器兜底移除（防发起端崩溃/关闭导致长期卡死）。
  - 收到 `session-updated` → 从集合移除并清除该会话定时器；若为当前会话且空闲则 `refreshCurrentSession()`（沿用 3.6）。
  - 导出 `isCurrentSessionRemoteBusy = computed(() => remoteBusySessions.value.has(currentSessionCode.value))`。
  - `onScopeDispose` 关闭 channel 时一并清理所有定时器。
- **锁定表现（卡片，经 `chat-message-list.vue` 下发 `locked`）**：
  - `chat-message-list.vue` 注入 `isCurrentSessionRemoteBusy`，把它作为 `:locked` 传给 5 张交互卡片。
  - 各卡片新增 `locked` prop：**仅当自身仍可交互（未 readonly）时** `locked` 才生效，即「只锁当前 pending 卡片」——历史只读卡片天然不受影响，无 pending 卡片的会话即使标记 busy 也无可见变化。
  - 生效表现：**不套用 readonly 折叠**（避免误显示绿色对勾「您已选择…」摘要），改为在卡片顶部展示黄色横幅「**该会话正在其它页面操作中，最新状态稍后自动同步…**」，并把操作按钮/选项置灰禁用。host-apply 系列复用 `CustomMessageCard` 的 `banner` 插槽属性；HITL/账号选择卡片各自加等价提示与禁用。
- **不做**：收到 busy 不主动 `/history`（用户明确不需要更进一步）；文本输入不锁（用户选 cards_only）；真正的并发 409 仍交由现有错误路径兜底（客户端锁定把窗口从「5-10s」压到「一次 BroadcastChannel 投递延迟」，无法根除同一毫秒级双击，最终一致性由后端 409 保证）。

```ts
// use-chatbot.ts（示意）
const remoteBusySessions = ref<Set<string>>(new Set());
const busyTimers = new Map<string, ReturnType<typeof setTimeout>>();
const BUSY_TTL = 20_000;

const markBusy = (code: string) => {
  const next = new Set(remoteBusySessions.value);
  next.add(code);
  remoteBusySessions.value = next;
  clearTimeout(busyTimers.get(code));
  busyTimers.set(code, setTimeout(() => clearBusy(code), BUSY_TTL));
};
const clearBusy = (code: string) => {
  clearTimeout(busyTimers.get(code));
  busyTimers.delete(code);
  if (!remoteBusySessions.value.has(code)) return;
  const next = new Set(remoteBusySessions.value);
  next.delete(code);
  remoteBusySessions.value = next;
};
const isCurrentSessionRemoteBusy = computed(() => remoteBusySessions.value.has(sessionModule.currentSessionCode.value));
```

## 四、边界与验收对齐

- AC-001：他处已提交 → 本处切回/聚焦触发 `refreshCurrentSession`，快照替换为最新（已提交态卡片）。✓（F-001）
- AC-002：续跑前 `await refreshCurrentSession()`。✓（F-002）
- AC-003：`isChatting` 守卫，刷新跳过，不打断当前流。✓
- AC-004：无 `currentSessionCode` 守卫，首页空态不发请求。✓
- AC-005：浮窗与全屏均接入相同 `refreshCurrentSession`。✓
- AC-006（F-004）：A 发起动作（选择/确认/提交）瞬间，B（同会话、有 pending 卡片）立即出现黄色横幅并禁用按钮，无法在窗口期重复点击；A 完成后 B 横幅消失并刷新为最新态。✓
- AC-007（F-004）：A 异常关闭/崩溃未发 `session-updated` 时，B 的锁最多 20s 自动解除。✓

## 五、不做项（标注）

- **409 冲突专门处理 / 冲突 UX**：out-of-scope，后续增强。F-004 的 in-flight 锁定属于**客户端并发规避**（大幅缩短可重复点击窗口），非服务端 409 的定向处理；毫秒级同时双击仍由后端 409 兜底。
- **后端 `bk_biz_id` 类型不兼容**：后端问题，**需后端配合**，前端本期不改。
- **会话列表跨端实时增删改排序同步**：非本期核心。
- **轮询**：明确不采用。

## 六、风险与回归关注点

1. `fetchHistory` 改为替换式后，需回归 `switchSession` 首次进入、深链直达、`deleteSession` 后自动切换等路径消息是否正常（预期无影响，因这些路径本就先清空）。
2. 续跑前刷新增加一次 `/history` 往返；关注 HITL/预提单/方案推荐卡片 resume 交互是否仍正确（刷新→替换→addUserMessage→streamChat 顺序）。
3. `visibilitychange`/`focus` 可能高频触发；除 `isChatting`/`currentSessionCode`/空会话三重守卫外，**本期已加 ~300ms 去抖**（`lodash/debounce`）避免高频 alt-tab 场景重复拉取。
4. 严格遵循 `fe-conventions`（`<script setup lang="ts">`、`useTemplateRef`、`defineModel` 等）与 `fe-no-backend-edit`（仅改 `front/`）。

## 七、实现后动作（待方案确认后执行，本轮不做）

1. 按上述改动实现。
2. `bkdevbuddy lint --fix`。
3. 进入 test 阶段，按 `wf-test-checklist` 产出手测清单。
