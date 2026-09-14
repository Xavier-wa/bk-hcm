# 智能助手

> status: drafted · kind: module
> globs: `src/views/chatbot/**`, `src/components/chatbot/**`, `src/components/ai-assistant/**`, `src/hooks/chatbot/**`, `src/store/chatbot/**`

对话式 AI 助手：全页 chatbot 与业务页浮窗（ai-assistant）共用同一套会话内核（`useChatbot`），支持主机申领等场景卡片与多标签页读一致性。

## 职责

- **双入口**：`views/chatbot`（全页）与 `components/ai-assistant`（浮窗，按 `sceneTag` 过滤会话）；消息列表与交互卡在 `components/chatbot`。
- **入口鉴权**：首页菜单 `checkAuth`、默认落地、冷启动分流、浮窗显隐均认 IAM `agent_assistant`（「平台-智能体助手」）。不再读 `biz_agent_assistant` / `chatbot_access`。无平台权限时菜单和浮窗不展示；直访 chatbot 走通用申请页（先补业务访问，再申请 `agent_assistant`）。
- **会话内核**：`hooks/chatbot/use-chatbot.ts` 组装 `useSession` / `useStream` / `useMessage`；对外提供发送、续跑、刷新、跨标签同步状态。
- **过程区**（`feat-agent-reply-ux`）：同一轮 `reasoning` + 带 `toolCalls` 的助手消息收成一行摘要（`use-process-zone` + `process-zone-summary.vue`）。摘要箭头收起向右、展开向下。有过程区时整轮落进一块气泡（阴影与现网申领卡一致 `0 12px 32px`），内层 `custom-msg-card` 去掉描边投影。思考条自绘 `process-thinking.vue`（始终带耗时，点标题折叠）。工具行自绘 `process-tool-row.vue`，整行可点折叠详情。内部展开态是**单条粒度**：流式中只摊开正在跑的那条（思考条看自身 `status`，工具行看结果消息是否到达），完成即时收起，不等整轮结束；整轮结束时 `detailKey` 切 `@done` 段，流式期间的手动展开一并作废（详见 `feat-agent-reply-ux/coding.md` §2.2）。行左侧状态四态：`running` / `success` / `error` / `pending`；`pending`（黄色 `waiting`）是**需确认工具的常态**——被 `tool_confirm` 中断挂起的调用本轮就 `RUN_FINISHED`，结果落到用户确认后的新一轮里，永远配不回这条行。工具说明取自该次调用参数 `tool_intent`，画在各自工具行上方（见下「工具说明」）。全页与浮窗共用 `ChatMessageList`。
- **场景卡片**：HITL / 账号选择 / 主机申领推荐·预提单·确认提交；只读态由运行态字段（`__selectedIndex` 等）或「后续已有 user 消息」推断。HITL 澄清的 `options` 可缺省，无选项时退化为自由文本作答（见下「HITL 澄清」）。
- **主动反馈**：`chat-message-list.vue` 的 chat-x 工具栏打开 `like` / `unlike`（引用/分享/删除仍隐藏）。点赞/点踩走内置原因面板：`onAgentAction` 展示 `GET /api/v1/agent/bizs/{bk_biz_id}/config/list?config_type=agent_feedback_tag` 下发的中文标签；`onAgentFeedback` 调 `POST .../feedback/commit`（`unlike` → `dislike`，chips 中文反查英文 key）。`session_id` 用 sessions/list 的 `id`，不是 `session_code`。`run_id`：实时流取 `/agui` 的 `RUN_STARTED.runId`；历史取 SNAPSHOT 里 `activityType=RUN_STARTED` 的 `content.runId`（本期新增，不在 assistant 消息根上）。再点已选中按钮：chat-x 在 Tippy `onShow` 取消激活且不发 feedback，由消息列表捕获 `.ai-tool-btn.is-active` 后调 `DELETE .../feedback/{run_id}`。全页与浮窗共用该列表。
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

### 工具说明（参数 `tool_intent`，重要）

- 说明文案来自该次调用 `function.arguments.tool_intent`，画在对应工具行上方；展示用参数里剥掉该键，避免详情区再画一遍。`/agui` 的 `TOOL_CALL_ARGS.delta` 与 `/history` 的 `arguments` 同形：带首尾空格的完整 JSON 字符串。
- **`/agui` 与 `/history` 的助手消息粒度不一样**，但取数不再依赖消息配对：`/agui` 每个工具各推一条助手消息；`/history` 是一条助手消息带 N 个 `toolCall`。各自从自己的 arguments 取 `tool_intent`，不会把 N 条说明糊成一句。
- 旧协议 `TEXT_MESSAGE_*` / SNAPSHOT 里 `tool-intent-{toolCallId}` 消息**只丢弃、不取文案**（原先只为工具说明服务）。过渡期与旧会话若仍下发，不能再开成独立气泡。
- 助手消息 `content` 里的正文（模型边调工具边说的话，实测恒为空或一个空格）另走 `__intent`，画在所有行之上，不与行级说明混用。

### 流式自动贴底（手写节点必须自己补，重要）

- **`ChatMessageList` 的根节点必须就是 `MessageContainer`，中间不许再套一层无样式包裹**（需要全局监听就用 `@click.capture` 等直接挂到组件上，靠属性透传落到根元素）。chat-x 的 `.ai-message-container` 用 `height: 100%` 撑满滚动父容器（全页 `.chatbot-chat-messages` / 浮窗 `.aa-messages`），多一层 auto 高度的 div 会让百分比高度回落成 auto，**滚动所有权就从 chat-x 容器转移到外层父容器**。后果有两处且都只在长对话时显现：容器内 `position: sticky; bottom: 12px` 的「停止生成 / 返回底部」条只能贴在容器自身底边（此时已在可视区外），流式贴底与过程区收起后被外层 `overflow` 裁掉一截（实测裁 14px，手动滚到真正底部又完整露出，故容易误判成偶发遮挡）；`jumpToBottom()` 改的是内层 `scrollTop`，在不可滚动的内层上直接成为空操作，只剩 `scrollIntoView` 那条路还灵。
- chat-x 的**持续跟随不是容器主动做的**：`message-container` 只在挂载时 `jumpToBottom` 两次，之后靠 Markdown 渲染器每挂载一个 token 回调一次 `toScrollBottom`。因此**凡是不走 Markdown 的手写节点（过程区思考条 / 工具行、五张场景卡）都没有任何跟随触发点**，内容一长出可视区就停在半路——本轮结束后卡片被截断、「返回底部」按钮亮着。新增手写消息节点时必须一并挂 `use-follow-scroll.ts`：内容会持续变长的用 `useFollowScroll()` + `watch(..., { flush: 'post' })`，高度在挂载时已定的（场景卡，数据来自 props）用一行 `useFollowScrollOnMount()`。
- 触发点只挂「内容长高」，**不挂展开态**：用户手动展开是为了读过程，不该被拽到底部。
- `autoScrollEnabled` **只在 wheel 事件 `deltaY < 0`（用户上滚）时置 false**，程序化滚动不会误关它，跟随里据此早退即可不抢用户翻历史的滚动。
- `toScrollBottom()` 缺省行为按距底距离二选一：>600px 瞬时贴底，否则平滑 `scrollIntoView`。**平滑分支的动画目标是发起那一刻的底部**，所以贴底调用必须等 DOM 高度定了再发，否则动画结束就停在半路。

### HITL 澄清（无选项 = 默认澄清）

- agent 侧 `human_confirm` 的 `options` 是**可选项**：缺字段 / `null` / 空数组三种「无选项」都是合法的默认澄清，此时卡片只渲染问题 + 自由文本输入框（无「其他选项：」标签，占位「请输入您的回复...」），**不可**据 options 判非法。
- `parseHitlInterruptValue`（`/history`）只校验 `question`，options 归一为空数组。曾因把 options 当必填而返回 `null`，导致历史回放落 activity 兜底渲染成裸文案「活动消息：hitl.interrupt」。实时流（`use-event.ts`）本就不校验，所以该类问题只在 `/history` 显现——**改这块务必两条路径一起验**。
- 只读回显：HITL 卡确认走 `sendMessage` 且**不带 `resumeValue`**（与主机申领 A/B/D 卡不同），澄清答案就是紧随其后的普通 user 消息，历史里没有独立 resume 回执可读，也用不上 `resume_forwarded` 那套回填。故 `getHitlReadonlyState` 在**无 options 时直接回显后继 user 消息原文**；有 options 却未命中仍留空（保守判定，防把无关消息误认成自定义输入）。
- 卡片内 `options` 必须走 `?? []` 归一：watch 里 `options.includes(answer)` 在 `null` 上会抛 TypeError；空选项时选项区要 `v-if` 摘掉，否则空 `div` 仍占 16px `margin-bottom`。

### 用户气泡与 chat-x 尺寸档（覆盖点）

- 用户气泡视觉全部落在 `.ai-user-message-content`（其内 `.ai-text-content` 已被 chat-x 重置为透明无内边距）。稿面与 chat-x 默认五项全不同：底色 `#cddffe`（默认 `#e1ecff`）、`padding: 12px 24px`（默认 8/4）、`border-radius: 16px 0 16px 16px`（右上直角，尖角朝发言人）、投影 `0 1px 1px rgba(0,0,0,.05)`（默认无）、14/22（默认 12/20）。
- **字号写死、不要动 `--ai-font-size`**：那是 chat-x 的**整档**变量（`data-ai-size` small 12-20 / normal 14-24），改它会把 AI 正文和图标一起放大。只放大用户气泡就只能在该选择器里写死。
- **多行换行要自己保**：chat-x 的 `text-content` 是 `toDisplayString(content)` 出一个纯文本节点（**不走 markdown**），全链路没有 `white-space` 声明，用户 Shift + Enter 打的 `\n` 会被 HTML 折叠成空格、多行糊成一段。故给 `.ai-user-message-content .ai-text-content` 开 `pre-wrap`。该文本节点是纯数据、不含模板缩进，不会重演工具行详情那次「容器开 `pre-wrap` 把标记缩进画成空行」（见 `feat-agent-reply-ux/coding.md` §2.6）。
- 容器另加 `flex-direction: column`：chat-x 原值是 `row`，`content` 为数组时会把各段并排画。

### 浮窗宽度与卡片 `@container` 断点（联动，重要）

- 浮窗默认宽在 `components/ai-assistant/index.vue` 的 `DraggableContainer :default-width`（当前 **500**，`minWidth` 仍 400，**不做持久化**——改默认值对所有人立即生效）。
- **改浮窗宽度必须同步复核卡片的 `@container` 断点**，两者是耦合的。卡片内容盒 = 窗宽 − 48（普通轮次：`aa-messages` 左 8 + `message-group` 右 16 + 卡片自身 `8px 12px`）或 − 96（过程区轮次多 24px 左右内边距）。断点定在 380 时，窗宽 > 428 就已失效，卡片会在仍然很窄的浮窗里跳成全页版式（账号选择的多列网格会挤）。故断点随默认宽 400→500 同步 380 → **460**（W=500 时两类卡片分别是 452 / 404，都留在紧凑版式；全页卡片约 900，不受影响）。
- **三处 `@container` 必须一起改**：`custom-message-card.vue` / `account-select-card.vue` / `host-apply-recommend-card.vue` 共用 `.custom-msg-card` 上声明的 `container-type`，漏改一处就只有那张卡跳版式。
- 已知断点语义（非缺陷）：手动把浮窗拖到 > 508 时普通轮次卡片会切全页版式。

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
- **数据盘可为空**：主机申领的数据盘是可选项——后端 `ResourceSpec.ValidateDisk()` 允许总块数为 0，`data_disk` 列可为 NULL，下发 CRP 与云 API 前均有 `len()` 守卫。但 woa-server `recommend.go` 的推荐方案会固定塞 1 块 500G 高性能云盘，因此 C 调整弹窗（`host-apply-adjust-dialog.vue`）必须提供增删行能力，否则用户无法申领不挂盘的机器。总块数上限 20（对齐后端 `constant.DataDiskTotalNum`）。空数组在预提单表格 / 确认提交卡 / 规格展示（`host-apply-display.ts`）均渲染为 `--` 或隐藏该行。**注意「零块」与「空行」是两件事**：零块合法，但已存在的行三个字段都不能为空——输入框清空后 `Number('')` 会静默变成 0，而后端逐行按 `[DataDiskMinSize, DataDiskMaxSize] = [10, 32000]` 且必须是 10 的倍数校验（`len(DataDisk)==0` 时才跳过逐行检查），因此弹窗必须带 `rules.data_disk` 前置拦截，否则用户要走到「确认方案」之后才看到后端拒单。该校验与容量上下限、标签说明文案统一复用自研云申领表单的既有规则与常量（`CVM_DATA_DISK_INFO`），不另造一份以免上下限漂移。

## 关键文件

| 路径 | 说明 |
|------|------|
| `components/chatbot/chat-input-box.vue` | 输入框封装；自定义 `placeholder`；场景 chip 让位 |
| `hooks/chatbot/use-session.ts` | `refreshCurrentSession`；刷新前后 card meta 恢复 |
| `hooks/chatbot/use-stream.ts` | 替换式 history；`resume_forwarded` 回填 |
| `hooks/chatbot/card-client-meta.ts` | 卡片运行态捕获/恢复/已提交快照/宽松匹配 |
| `hooks/chatbot/use-hitl.ts` | HITL 消息识别、内容提取、只读态与答案回显判定 |
| `components/chatbot/hitl-interrupt-card.vue` | 澄清卡：选项单选 + 自由输入；无选项时纯自由输入 |
| `components/chatbot/chat-message-list.vue` | 下发 `locked` 给交互卡；过程区摘要与浅底；点赞/点踩接 commit/delete |
| `components/chatbot/process-zone-summary.vue` | 过程区一行摘要（展开/收起入口） |
| `components/chatbot/process-thinking.vue` | 过程区自绘思考条（始终展示耗时） |
| `components/chatbot/process-tool-row.vue` | 过程区自绘工具行（行头 + 描述/参数/返回，整行折叠） |
| `hooks/chatbot/use-process-zone.ts` | 过程消息折叠、摘要文案、工具行改写 |
| `hooks/chatbot/tool-intent.ts` | 从工具参数 `tool_intent` 拆出说明；丢弃旧 `tool-intent-*` 消息 |
| `hooks/chatbot/use-follow-scroll.ts` | 手写节点的流式贴底（过程区跟随 + 场景卡挂载贴底） |
| `hooks/chatbot/use-agent-feedback.ts` | 标签配置、commit/delete、本会话取消映射 |
| `store/chatbot/feedback.ts` | Agent 反馈 HTTP 封装 |
| `hooks/chatbot/use-event.ts` | AG-UI 事件分发；实时消息打 `__runId` |
| `components/chatbot/custom-message-card.vue` | 锁定黄条横幅（非只读折叠）；`.custom-msg-card` 上的 `container-type` 是三张卡 `@container` 断点的宿主 |
| `components/ai-assistant/index.vue` / `views/chatbot/index.vue` | F-001 可见/聚焦刷新；前者 `DraggableContainer :default-width` 定浮窗默认宽（与卡片断点联动） |
