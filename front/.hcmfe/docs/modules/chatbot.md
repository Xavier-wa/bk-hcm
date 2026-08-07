# 智能助手

> status: drafted · kind: module
> globs: `src/views/chatbot/**`, `src/components/chatbot/**`, `src/components/ai-assistant/**`, `src/hooks/chatbot/**`, `src/store/chatbot/**`

对话式 AI 助手：全页 chatbot 与业务页浮窗（ai-assistant）共用同一套会话内核（`useChatbot`），支持主机申领等场景卡片与多标签页读一致性。

## 职责

- **双入口**：`views/chatbot`（全页）与 `components/ai-assistant`（浮窗，按 `sceneTag` 过滤会话）；消息列表与交互卡在 `components/chatbot`。
- **会话内核**：`hooks/chatbot/use-chatbot.ts` 组装 `useSession` / `useStream` / `useMessage`；对外提供发送、续跑、刷新、跨标签同步状态。
- **场景卡片**：HITL / 账号选择 / 主机申领推荐·预提单·确认提交；只读态由运行态字段（`__selectedIndex` 等）或「后续已有 user 消息」推断。
- **多入口读一致性**（`feat-aiagent-session-sync`）：
  - F-001：可见/聚焦刷新当前会话（~300ms 去抖）
  - F-002：带 `resumeValue` 的续跑前先 `refreshCurrentSession`
  - F-003：`BroadcastChannel('hcm-chatbot-sync')` 在写入完成后广播，其它同会话实例刷新
  - F-004：动作发起广播 `session-busy` 锁定他处 pending 卡片；完成后 `session-updated` 解锁并刷新

## 关键流程 / 注意事项

### 历史刷新

- `fetchHistory` 对 `MESSAGES_SNAPSHOT` 为**替换式**（先清空再填），支撑无闪烁刷新。
- `refreshCurrentSession` 守卫：无选中会话 / `isChatting` / 空会话 → 跳过；刷新失败静默保留本地快照。
- 刷新前后用 `card-client-meta.ts` 捕获/恢复卡片运行态（按 `messageId`），避免 F-002 刷新冲掉刚写入的选中下标。

### 跨标签同步规则（重要）

- **`session-busy`**：只锁卡片，**不**附带/应用 cardMeta（未提交编辑不跨标签传播）。
- **`session-updated`**：仅广播**已提交只读态**最终数据（卡片后已有 user 消息才算提交）：方案 `__selectedIndex`、预提单 `__confirmedSuborders`、确认提交 `__submitted`、账号 `__selectedAccountId`。
- 预提单组件本地 C 弹窗草稿（`edited=true`）在切标签触发的 refresh 中**不得**被服务端原始值覆盖。
- 历史回放兜底：`resume_forwarded` 宽松匹配回填；未命中**不**强行写选中下标 0。

### 停止生成提示（重要）

- 用户主动点「停止」→ `stopGeneration()` 置 `stoppedByUser=true` + abort + `cancelRun`。
- `streamChat` 的 `finally`（仅当前流）据 `stoppedByUser` 调 `ensureStopTip('已停止生成')`：末条为**空内容助手气泡**（工具调用后正文未产出即中断）→ 直接填充文案消除空白气泡；否则追加一条。提示气泡统一用 `MessageStatus.Error` 渲染，复用与「未响应或被停止」一致的提示图标。正常 `RunFinished` 不插入提示。
- 仅覆盖用户主动停止；`abortStream`（会话切换本地中断）与 `fetchHistory`（history 断点续传，文案「未响应或被停止」）不受影响。
- **`fetchHistory` 与 `isChatting`**：纯历史 SNAPSHOT 加载**不**置 `isChatting`（避免切回对话页 / 拉历史时闪现「停止生成」）。仅在 SNAPSHOT **之后**出现断点续传实时事件（非 `RunFinished`）时才置 `isChatting=true`。`abortStream` 只要存在 `abortController` 即可 abort（不依赖 `isChatting`），保证快速切会话仍能打断历史拉取。

### 关键边界

- 不做 409 专门冲突 UX；不做轮询；会话列表跨端实时同步非本模块本期范围。
- 浮窗与全页各自独立 `useChatbot()` 实例，跨实例一致性靠可见刷新 + BroadcastChannel，而非共享单例。
- **输入框占位**：`ChatInputBox` 覆盖 `@blueking/chat-x` 默认四行引导，仅展示「通过 Shift + Enter 进行换行输入」（Skill / Prompt / 工具与 MCP 能力未开放，不在占位中提示）。
- **首页大卡片连点**：`handleBigCardClick` 用同步 `isBigCardSending` 互斥（`isChatting` 有 createSession/streamChat 异步窗口），模板以 `is-disabled` class + `pointer-events: none` 禁用（`div` 上 `:disabled` 无效）。
- **浮窗权限**：`AiAssistant` 与业务「首页」菜单一致，校验 `biz_agent_assistant`；无权限不渲染 teleport 浮窗入口，`show` / `toggle` / 深链 `initSessions` 亦早退。

## 关键文件

| 路径 | 说明 |
|------|------|
| `components/chatbot/chat-input-box.vue` | 输入框封装；自定义 `placeholder`；场景 chip 让位 |
| `hooks/chatbot/use-session.ts` | `refreshCurrentSession`；刷新前后 card meta 恢复 |
| `hooks/chatbot/use-stream.ts` | 替换式 history；`resume_forwarded` 回填 |
| `hooks/chatbot/card-client-meta.ts` | 卡片运行态捕获/恢复/已提交快照/宽松匹配 |
| `components/chatbot/chat-message-list.vue` | 下发 `locked` 给交互卡 |
| `components/chatbot/custom-message-card.vue` | 锁定黄条横幅（非只读折叠） |
| `components/ai-assistant/index.vue` / `views/chatbot/index.vue` | F-001 可见/聚焦刷新 |
