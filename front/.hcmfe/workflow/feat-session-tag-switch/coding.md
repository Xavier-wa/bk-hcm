# coding.md — 同会话内动态切换场景标签

> 工作流：`feat-session-tag-switch`（full）  
> 依据：`prd.md` / `design.md` / `api.md`  
> 范围：仅 `front/`；不改后端。

## 一、方案概要

在 SSE 流式路径的 `use-event` 中识别 `CUSTOM` / `scene.switched`，解析 `payload.to` 后更新本地 `sessions` 中当前会话的 `sessionTag`。全屏 chip（`sceneChip`）与侧栏文件夹（`sidebarSessions`）均由 `sessionTag` 派生，**无需改 UI 模板**。浮窗与全屏共用 `useChatbot`，一处生效两端数据同步。

## 二、改动清单

| 文件 | 改动 |
|------|------|
| `src/hooks/chatbot/types.ts` | 新增常量 `SCENE_SWITCHED_EVENT = 'scene.switched'`；可选 `SceneSwitchedValue` 类型 |
| `src/hooks/chatbot/use-session.ts` | 新增 `updateSessionTag(sessionCode, tag)`：就地改 `session.sessionTag`（空 tag 则 `undefined`） |
| `src/hooks/chatbot/use-event.ts` | `Custom` 分支增加 `scene.switched`：解析 value → 取 `payload.to` → 调用 deps 回调；缺 to / 解析失败则忽略 |
| `src/hooks/chatbot/use-chatbot.ts` | 用可变 deps 把 `updateSessionTag(currentSessionCode, to)` 接到 event handler（解决 event 早于 session 创建的顺序问题） |

**不改**：`views/chatbot/index.vue` 模板（chip/文件夹已派生）、浮窗组件 UI、history / `toHistoryMessage`（本期不做回放）。

## 三、实现细节

### 3.1 解析 `value`（兼容 string / object）

```ts
const parseSceneSwitchedPayload = (value: unknown): { to?: string; from?: string } | null => {
  try {
    const root = typeof value === 'string' ? JSON.parse(value) : value;
    if (!root || typeof root !== 'object') return null;
    const payload = (root as { payload?: unknown }).payload;
    if (!payload || typeof payload !== 'object') return null;
    const to = String((payload as { to?: unknown }).to ?? '').trim();
    const from = String((payload as { from?: unknown }).from ?? '').trim();
    return { to: to || undefined, from: from || undefined };
  } catch {
    return null;
  }
};
```

- 无有效 `to` → return，不更新  
- 不校验 `from`  
- 解析失败 → `console.warn` 后忽略，不打断流式

### 3.2 `updateSessionTag`

```ts
const updateSessionTag = (sessionCode: string, tag: string) => {
  const session = sessions.value.find((s) => s.sessionCode === sessionCode);
  if (!session) return;
  session.sessionTag = tag.trim() || undefined;
};
```

- 更新当前流对应会话：`currentSessionCode`（流式事件属于当前会话）
- 列表中该项 tag 变更后，`sidebarSessions` computed 自动重归文件夹；`sceneChip` 自动换文案

### 3.3 event ↔ session 接线

`useChatbot` 内：

```ts
const eventDeps = {
  onSceneSwitched: (_to: string) => {},
};
const eventModule = useEventHandler(messageModule, eventDeps);
// ... sessionModule 创建后
eventDeps.onSceneSwitched = (to: string) => {
  const code = sessionModule.currentSessionCode.value;
  if (!code) return;
  sessionModule.updateSessionTag(code, to);
};
```

### 3.4 浮窗 sceneTag 过滤副作用

浮窗 `useChatbot({ sceneTag: 'host_apply' })` 列表按场景过滤。会话 tag 切到 `resource_query` 后，该项可能不再出现在浮窗列表——符合「浮窗无外显标签、数据已更新」；全屏可见正确文件夹。

## 四、图标 / 组件

- 无新增图标（design §3.x）
- 无新 page/comp skill 落码；复用既有派生 UI

## 五、自测要点（编码后）

- 流式中收到 `to=resource_query`：chip + 文件夹同步  
- 连续多次切换：以最后一次为准  
- 未知 `to`：展示原值  
- 缺 `to`：标签不变、对话继续  
- 浮窗：无新标签 UI，会话数据已更新  

## 六、boundFiles

- `src/hooks/chatbot/types.ts`
- `src/hooks/chatbot/use-session.ts`
- `src/hooks/chatbot/use-event.ts`
- `src/hooks/chatbot/use-chatbot.ts`

## 七、落码结论（2026-08-06）

已按上文方案落地并通过 `bkdevbuddy_lint`：`SCENE_SWITCHED_EVENT` / `updateSessionTag` / `use-event` 解析分支 / `use-chatbot` 可变 deps 接线。未改 `views/chatbot/index.vue`（chip/文件夹仍由 `sessionTag` 派生）。
