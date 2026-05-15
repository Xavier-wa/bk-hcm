# 首页 AI Chatbot 功能规格

## 1. 功能概述

在 HCM 首页（`/chatbot`）集成 AI 对话助手，基于 AG-UI 协议与后端 Agent 服务通信，通过 SSE 实现流式消息交互。通过权限中心（IAM）控制菜单可见性，仅授权用户可见"首页"导航入口。

## 2. 架构设计

### 2.1 技术栈

| 层级 | 技术选型 | 说明 |
|------|----------|------|
| UI 组件 | `@blueking/chat-x` | ChatInput、MessageContainer、ShortcutRender 等 |
| 状态管理 | Composable (`hooks/chatbot/use-chatbot.ts`) | 页面级状态编排 |
| API 层 | `store/chatbot/` | 参考 chat-helper 分层，session.ts + agent.ts |
| HTTP 客户端 | `@/http`（axios 封装）+ 原生 fetch（SSE） | REST 用 http，SSE 流用 fetch |
| 通信协议 | AG-UI (SSE) | 后端流式事件协议 |

### 2.2 模块划分

```
store/chatbot/                   # API 层（参考 chat-helper 的 http/module 分层）
├── session.ts                   # 会话 CRUD：listSessions / createSession / deleteSession / updateSession
└── agent.ts                     # SSE 流式：streamChat / fetchHistoryStream / cancelRun

hooks/chatbot/                   # 业务逻辑层（参考 chat-helper 模块拆分）
├── types.ts                     # ChatSession、EventType 类型定义
├── use-message.ts               # 消息 CRUD（对标 chat-helper/message）
├── use-event.ts                 # AG-UI 事件分发（对标 chat-helper/event）
├── use-session.ts               # 会话管理业务逻辑（对标 chat-helper/session）
├── use-stream.ts                # SSE 流式请求（对标 chat-helper/agent）
└── use-chatbot.ts               # 顶层编排 composable（组合上述模块、暴露给页面组件）

views/index/chatbot/             # 页面层
├── index.vue                    # 页面组件：左侧会话列表 + 右侧聊天区 + 顶部标题栏
└── AI_SPEC.md                   # 本文档
```

**分层职责**：

| 层级 | 职责 | HTTP 方式 |
|------|------|----------|
| `store/chatbot/session.ts` | 纯 API 调用，使用项目统一 `http` | `http.post` / `http.patch` / `http.delete` |
| `store/chatbot/agent.ts` | SSE 流式 API，axios 不支持 ReadableStream | 原生 `fetch` |
| `hooks/chatbot/use-session.ts` | 会话状态管理 + 调用 session API | 导入 `store/chatbot/session` |
| `hooks/chatbot/use-stream.ts` | SSE 读取 + 事件分发 + 调用 agent API | 导入 `store/chatbot/agent` |
| `use-chatbot.ts` | 顶层 composable，组合所有 hooks 暴露给 Vue 组件 | 不直接调 API（除 `sessionApi.updateSession` 用于自动标题） |

### 2.3 数据流

**发送消息（/agui）**：
```
用户输入
  → ChatInput @send-message
  → store.sendMessage(text)
  → store.addUserMessage()       // 立即展示用户消息
  → store.streamChat()           // 发起 SSE POST 请求
  → agentApi.streamChat()        // store/chatbot/agent.ts
  → reader.read() 循环
  → handleEvent() 按事件类型分发
  → appendContent()              // delta 直接追加到 message.content
  → Vue 响应式 → MessageContainer 更新 UI
  → finally: 若 tail 仍是 user 消息 → 补 "已停止生成" 占位（阻断 useMessageGroup 的 Loading 注入）
```

**切换会话 / 加载历史（/history，增量渲染 + 断点续传）**：
```
用户点击会话
  → switchSession(code)
  → abortStream()                       // 仅本地 abort；不通知后端 /cancel，让旧 run 继续跑完写入 history
  → saveCurrentSession()                // 保存当前会话本地快照
  → applySession(code) + messages = []  // 清空后由 fetchHistory 增量写入
  → isLoadingHistory = true             // 全屏 spinner
  → fetchHistory(code, onSnapshotLoaded)
  → agentApi.fetchHistoryStream()       // SSE，支持 signal 中止
  → reader.read() 循环
     ├─ MESSAGES_SNAPSHOT → push 历史消息 + 连续 user 消息之间补 "未响应或被停止" 占位
     │                     → onSnapshotLoaded() → isLoadingHistory = false（露出历史消息）
     ├─ (断点续传实时事件) → handleEvent() 按 /agui 同一路径分发
     └─ RUN_FINISHED     → reader.cancel()（后端发完不关连接）
  → finally: 若 tail 仍是 user 消息 → 补 "未响应或被停止" Error 占位
  → 写回 target.messages（竞态守卫：currentSessionCode === code 才写）
```

### 2.4 会话管理（Session Management）

**参考**：`@blueking/chat-helper` 的 `use-session.ts` 和 `ai-blueking` 的 `SessionBusinessManager`。

**数据结构**：
```typescript
interface ChatSession {
  sessionCode: string;        // 会话对外标识（后端生成）
  sessionName: string;        // 会话名称
  sessionContentCount: number; // 消息数量（后端维护）
  createdAt: string;          // 创建时间 (ISO 8601)
  updatedAt: string;          // 更新时间 (ISO 8601)
  messages: Message[];        // 会话消息快照（本地缓存）
}
```

**操作方法**：

| 方法 | 说明 | API 层调用 |
|------|------|-----------|
| `initSessions()` | 页面初始化：加载列表 → 有内容则新建 / 无内容则复用 | `sessionApi.listSessions` + `sessionApi.createSession` |
| `loadSessions()` | 获取会话列表 | `sessionApi.listSessions` |
| `createSession()` | 创建新会话（复用空会话或调用后端创建） | `sessionApi.createSession` |
| `switchSession(code)` | 切换会话，通过 SSE 历史接口加载消息 | `agentApi.fetchHistoryStream` |
| `deleteSession(code)` | 删除会话 | `sessionApi.deleteSession` |
| `renameSession(code, title)` | 重命名会话（乐观更新） | `sessionApi.updateSession` |
| `saveCurrentSession()` | 将当前 `messages` 快照写回会话（本地缓存） | 无（纯本地） |

**会话初始化流程**（`initSessions`，参考 ai-blueking 的 `loadRecentSession`）：
```
页面 onMounted
  → loadSessions()                        // sessionApi.listSessions
  → 优先级判断：
    1. 列表不为空 + 最近会话有内容 → createSession()（新建空会话）
    2. 列表不为空 + 最近会话为空 → switchSession(code)（复用空会话）
    3. 列表为空 → createSession()（新建第一个会话）
```

**新建会话复用逻辑**：点击"新对话"按钮时，先检查是否存在空会话（`messages.length === 0 && sessionContentCount === 0`），有则复用，无则调用后端创建。

**自动标题**：首次发送消息时，取用户输入前 30 字作为会话标题，同步调用 `sessionApi.updateSession` 更新后端。

**布局**：
```
┌───────────────────────────────────────────────┐
│ ┌──────────┐ ┌──────────────────────────────┐ │
│ │ 开启新对话  │ │ 当前会话标题               │ │
│ ├──────────┤ ├──────────────────────────────┤ │
│ │ 今天       │ │                              │ │
│ │  会话1     │ │    MessageContainer          │ │
│ │  会话2 ··· │ │                              │ │
│ │ 昨天       │ │                              │ │
│ │  会话3     │ ├──────────────────────────────┤ │
│ │  ← 折叠   │ │    ChatInput                 │ │
│ └──────────┘ └──────────────────────────────┘ │
└───────────────────────────────────────────────┘
  240px 深色     flex: 1 浅色
  可折叠/hover 弹出
```

## 3. 已确认的需求点

### 3.1 路由与权限

- [x] Chatbot 页面路径：`/chatbot`（非根路径，避免与默认首页冲突）
- [x] `/` 根路径重定向到 `/business/host`（资源管理-主机）
- [x] 菜单可见性通过权限中心（IAM）控制：`chatbot_access` 权限（`store/common.ts` pageAuthData）
- [x] 头部导航过滤：`authVerifyData.permissionAction.chatbot_access` 控制"首页"tab 显示
- [x] IAM 权限已启用：`common.ts` 中 `chatbot_access` 使用 `agent_assistant` 类型，`home/index.tsx` 已移除 `|| true` 临时降级

### 3.2 页面布局

- [x] Chatbot 页面隐藏左侧菜单栏（`useWhereAmI` 中 `/chatbot` 映射为 `Senarios.index`）
- [x] 首页 header 菜单高亮"首页"tab（`useChangeHeaderTab` 中 `case 'chatbot'` → `topMenuActiveItem = 'index'`）
- [x] 左侧固定会话列表侧边栏（240px，深色背景） + 右侧聊天区
- [x] 顶部当前会话标题栏
- [x] 聊天区域占满右侧可用空间，上方消息列表 + 下方输入框

### 3.3 UI 组件集成

- [x] 使用 `@blueking/chat-x` 的 `ChatInput` 和 `MessageContainer`
- [x] 手动引入 `@blueking/chat-x/dist/index.css`（因为项目直接依赖 chat-x 而非 ai-blueking，样式不会自动打包）
- [x] `:deep()` 处理 scoped 样式穿透（message-group 居中、message-tools hover 效果等）

### 3.4 后端接口对接

**所有接口统一使用 `sessionCode` 作为会话标识，`threadId`/`runId` 由后端内部管理，前端不再生成和传递。**

| 接口 | 方法 | 路径 | API 层文件 | HTTP 方式 |
|------|------|------|-----------|----------|
| 会话列表 | POST | `/api/v1/agent/sessions/list` | `session.ts` | `http.post` |
| 创建会话 | POST | `/api/v1/agent/sessions/create` | `session.ts` | `http.post` |
| 删除会话 | DELETE | `/api/v1/agent/sessions/{code}` | `session.ts` | `http.delete` |
| 重命名会话 | PATCH | `/api/v1/agent/sessions/{code}` | `session.ts` | `http.patch` |
| 聊天 | POST | `/api/v1/agent/agui` | `agent.ts` | `fetch` (SSE) |
| 历史回放 | POST | `/api/v1/agent/history` | `agent.ts` | `fetch` (SSE) |
| 取消推理 | POST | `/api/v1/agent/cancel` | `agent.ts` | `fetch` |

- [x] REST API 使用项目统一 `http` (`@/http`)，自动处理 CSRF、错误码、登录态
- [x] SSE 流式接口使用原生 `fetch`（axios 不支持 ReadableStream 逐块读取）
- [x] `sessionCode` 由后端 `POST /sessions/create` 生成
- [x] 历史消息通过 SSE 流返回 `MESSAGES_SNAPSHOT` 事件（非 JSON 数组）
- [x] 历史接口支持断点续传：`MESSAGES_SNAPSHOT` 之后可能继续下发实时事件流直到 `RUN_FINISHED`（与 `/agui` 同构，前端按增量渲染）
- [x] 停止生成仅由用户点击"停止"触发：`stopGeneration()` = 前端 `AbortController.abort()` + 后端 `POST /cancel` 双重中止。切换 / 新建会话时使用 `abortStream()`，仅本地 abort、不通知后端（让后端 run 自然跑完，结果会进入 history）
- [x] `streamChat` / `fetchHistoryStream` 均支持 `signal` 参数，可被上述两种 abort 同时中断

### 3.5 认证与用户标识

- [x] 后端通过 Cookie 用户体系确认身份
- [x] `X-Bkapi-User-Name` header 用于 cancel 接口定位进行中的 Run

### 3.6 AG-UI 事件处理

已支持的完整事件类型（对齐 `@blueking/chat-helper` AGUIProtocol）：

| 事件 | 处理 |
|------|------|
| **运行生命周期** | |
| `RUN_STARTED` | 无特殊处理 |
| `RUN_ERROR` | 创建错误消息 |
| `RUN_FINISHED` | 兜底设置 streaming 消息为 Complete |
| **文本消息** | |
| `TEXT_MESSAGE_START` | 创建 assistant 消息（status=Streaming） |
| `TEXT_MESSAGE_CONTENT` | delta 直接追加到消息内容 |
| `TEXT_MESSAGE_END` | 设 status=Complete |
| `TEXT_MESSAGE_CHUNK` | 兼容旧协议，直接追加内容 |
| **思考** | |
| `THINKING_START` | 创建 reasoning 消息（content=[]） |
| `THINKING_TEXT_MESSAGE_START` | content 数组 push 空字符串 |
| `THINKING_TEXT_MESSAGE_CONTENT` | delta 追加到最后一个 content 项 |
| `THINKING_TEXT_MESSAGE_END` | 无特殊处理 |
| `THINKING_END` | 设置 duration + Complete |
| **工具调用** | |
| `TOOL_CALL_START` | 创建带 toolCalls 的 assistant 消息 |
| `TOOL_CALL_ARGS` | 追加 function.arguments |
| `TOOL_CALL_END` | 设置对应工具调用消息为 Complete |
| `TOOL_CALL_RESULT` | 创建 Tool 角色消息 |
| `TOOL_CALL_CHUNK` | 预留（兼容旧协议） |
| **步骤** | |
| `STEP_STARTED` / `STEP_FINISHED` | 预留 |
| **历史快照** | |
| `MESSAGES_SNAPSHOT` | 用于历史接口，解析 messages 数组并增量 push 到 `messages`，同时对 SNAPSHOT 中连续 user 消息（上次被中断）每条补一条 "未响应或被停止" 占位。后续的实时事件（断点续传）复用普通分发路径 |
| **状态/自定义** | |
| `STATE_DELTA` / `STATE_SNAPSHOT` / `ACTIVITY_DELTA` / `ACTIVITY_SNAPSHOT` / `CUSTOM` / `RAW` | 预留 |

### 3.7 会话管理

- [x] 左侧侧边栏固定展示会话历史列表（可折叠，hover 弹出）
- [x] 会话按日期分组（今天、昨天、3 天前、一周前、更早），基于本地时区午夜零点计算
- [x] "开启新对话"按钮（复用空会话或调用后端创建，复用时自动移至列表顶部）
- [x] 会话操作菜单（重命名、删除）
- [x] 点击会话切换，通过 SSE `/api/v1/agent/history` 加载历史消息（**增量渲染**：`MESSAGES_SNAPSHOT` 到达即关闭全屏 spinner、先把历史消息铺出来，后续断点续传的实时事件继续追加）
- [x] 后端加载失败时降级到本地快照
- [x] 切换会话竞态守卫：`switchSession` 在 await 回收时用 `currentSessionCode === code` 守住，避免旧流 resolve 后写回错误会话；`useStream` 内部用局部 `controller === abortController` 守护状态收尾
- [x] 首条消息自动设为会话标题（截取前 30 字）+ 同步更新后端
- [x] `isLoadingHistory` 状态支持 UI 展示加载中
- [x] 页面加载时通过 `initSessions` 自动初始化会话

### 3.8 已知问题与经验

- **CORS 跨域**：后端 `/api/v1/agent/agui` 返回 `Access-Control-Allow-Origin: *`，与 `credentials: 'include'` 冲突。需后端修改为返回具体 origin。
- **历史接口 SSE 格式（增量 + 断点续传）**：`fetchHistory` 与 `streamChat` 同构，按增量方式处理——`MESSAGES_SNAPSHOT` 到达即 push 历史消息并通过 `onSnapshotLoaded` 回调让页面尽快渲染；后续的实时事件（断点续传场景下 SNAPSHOT 之后会跟随未完成 run 的实时事件流）复用 `event.handleEvent` 分发；收到 `RUN_FINISHED` 时前端主动 `reader.cancel()` 断开（后端发完事件不会关闭 SSE 连接）。后续可能改为普通 REST 接口返回 JSON 数组（对齐 chat-helper 的 `getMessages`）。
- **末尾 User 消息兜底（Loading 注入阻断）**：`@blueking/chat-x` 的 `useMessageGroup` 仅依据消息列表末尾消息的 role 决定是否注入 `LOADING_MESSAGE_ID`（"请求中..."），与 `messageStatus` prop 无关。因此每条流结束时若 tail 仍是 user，必须补一条 assistant 占位阻断 Loading 注入：
  - `/agui`（`streamChat` finally）：文案 **"已停止生成"**，`Complete` 状态
  - `/history`（`fetchHistory` finally）：文案 **"未响应或被停止"**，`Error` 状态
  - 连续停止导致 SNAPSHOT 中出现相邻 user 消息时，**每个未响应的 user** 都补一条 "未响应或被停止"（不含 tail，tail 可能被 resume 的实时事件继续填充，由 finally 根据最终状态决定）
  - `stopGeneration`（用户点击停止）= `abort()` + `/cancel`；`abortStream`（会话切换 / 新建）只做本地 `abort()`。两者都**不直接写占位**，文案由各自流的 finally 按场景决定
- **会话切换竞态**：增量渲染下 `fetchHistory` 会直接写入共享的 `msg.messages`，若快速切换 A→B→C，旧流 resolve 时可能污染新会话。两层守卫：`useStream` 内部用局部 `const controller = new AbortController()` + `finally` 里 `if (abortController === controller)` 判别；`switchSession` 在 await 后用 `if (currentSessionCode.value === code)` 判别，避免写回错误的 `target.messages` 或 `isLoadingHistory`。
- **日期分组时区**：会话列表按日期分组时，必须使用 `new Date().setHours(0, 0, 0, 0)` 获取本地午夜零点，不能用 `Date.now() % 86400000`（UTC 午夜），否则 UTC+8 等正偏移时区下当日凌晨会话会被错误归入"昨天"。
- **空会话名称兜底**：后端 `session_name` 可能为空字符串，前端 `toSession` 映射时需 `|| '新对话'` 兜底，避免列表出现空白项。
- **复用空会话排序**：`createSession` 复用已有空会话时，需将其从原位置移至数组首位（`splice` + `unshift`），确保 UI 显示在列表顶部。
- **`abortStream` vs `stopGeneration`**：两种中断职责严格区分——`abortStream()` 仅用于会话切换 / 新建场景（本地 abort 避免旧 SSE 继续写入共享 messages），不通知后端；`stopGeneration()` 仅由用户主动点击"停止"触发（abort + `/cancel`）。会话管理路径绝不调用 `stopGeneration`，避免因页面操作把后端还在正常跑的 run 取消掉。两者内部都先判断 `isChatting.value`，避免空流或重复 abort。

## 4. 相关文件变更清单

| 文件 | 变更说明 |
|------|----------|
| `src/store/chatbot/session.ts` | 新增：会话 CRUD API（使用项目 `http`） |
| `src/store/chatbot/agent.ts` | SSE 流式 API + cancel（使用原生 `fetch`）；`fetchHistoryStream` 增加 `signal` 参数以支持切换会话 / 停止生成时中断 |
| `src/hooks/chatbot/types.ts` | ChatSession 对齐后端响应结构（sessionCode / sessionName / sessionContentCount） |
| `src/hooks/chatbot/use-message.ts` | 消息 CRUD（无变更，消息由前端管理） |
| `src/hooks/chatbot/use-event.ts` | AG-UI 事件分发（无变更） |
| `src/hooks/chatbot/use-session.ts` | 会话管理业务逻辑，直接导入 `store/chatbot/session`；`switchSession` 改造为配合增量渲染（清空 messages → 由 fetchHistory 写入 → SNAPSHOT 回调关 spinner），并加会话切换竞态守卫 |
| `src/hooks/chatbot/use-stream.ts` | SSE 流式业务逻辑，直接导入 `store/chatbot/agent`；`fetchHistory` 改为增量渲染并处理 /history 断点续传；新增 `ensureAssistantTail` / `fillSnapshotUserGaps` 统一 tail / gap 占位；`streamChat` 与 `fetchHistory` 各自 finally 按场景决定占位文案；拆分 `abortStream`（仅本地 abort，供会话切换使用）与 `stopGeneration`（abort + `/cancel`，仅用户点击停止触发） |
| `src/hooks/chatbot/use-chatbot.ts` | 顶层编排 composable（组合所有 hooks，暴露给页面组件） |
| `src/views/index/chatbot/index.vue` | onMounted 调用 initSessions，import 路径更新 |
| `src/views/index/route-config.ts` | 路由配置（path: `/chatbot`） |
| `src/router/index.ts` | `/` 重定向改为 `/business/host` |
| `src/router/header-config.ts` | 首页导航 path 改为 `/chatbot` |
| `src/store/common.ts` | pageAuthData 增加 `chatbot_access` 权限项（待 IAM 就绪启用） |
| `src/views/home/index.tsx` | 头部菜单增加 `chatbot_access` 权限过滤 + `isNeedSideMenu` 排除首页 |
| `src/views/home/hooks/useChangeHeaderTab.ts` | 增加 `case 'chatbot'` → 首页 tab 高亮 + 清空左侧菜单 |
| `src/hooks/useWhereAmI.ts` | `/chatbot` 映射为 `Senarios.index` |
