# coding.md — agent回复交互体验优化-前端

> 工作流：`feat-agent-reply-ux`（full）  
> 依据：`prd.md` / `design.md` / `api.md`  
> 范围：仅 `front/`；不改后端。

## 单据 1: agent 回复交互体验优化（过程区）

**TAPD**: [#1069995598137818447](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137818447)

**文件**:

- `src/hooks/chatbot/types.ts`
- `src/hooks/chatbot/tool-intent.ts`
- `src/hooks/chatbot/use-event.ts`
- `src/hooks/chatbot/use-stream.ts`
- `src/hooks/chatbot/use-chatbot.ts`
- `src/hooks/chatbot/use-process-zone.ts`
- `src/components/chatbot/process-zone-summary.vue`
- `src/components/chatbot/process-thinking.vue`
- `src/components/chatbot/process-tool-row.vue`
- `src/components/chatbot/chat-message-list.vue`
- `src/hooks/chatbot/use-follow-scroll.ts`
- `src/components/chatbot/account-select-card.vue`
- `src/components/chatbot/host-apply-recommend-card.vue`
- `src/components/chatbot/host-apply-preorder-card.vue`
- `src/components/chatbot/host-apply-submit-card.vue`
- `src/components/chatbot/custom-message-card.vue`
- `src/components/ai-assistant/index.vue`

**改动点**:

- `TEXT_MESSAGE_*` / history 的 `tool-intent-{toolCallId}` 不再用于取说明：改为读工具参数 `tool_intent`，画在对应工具行上方；参数展示里剥掉该字段。旧协议遗留的 `tool-intent-*` 消息仍丢弃，避免再出气泡
- 同一轮 reasoning + 工具行收进过程区：一行摘要，流式展开、结束/历史收起、用户 Stop 保持展开
- 工具行整段手写（`src/components/chatbot/process-tool-row.vue`），整行可点折叠详情；默认展开态按单条算——该次调用在跑时摊开、拿到结果即时收起，思考条同理（见 §2.2）
- 详情区「描述 / 参数 / 返回内容」三字段常显，缺值写 `--`；`pre-wrap` 只给取值节点，不给容器（否则标记缩进被画成空行）
- 工具行状态对齐 chat-x 原生：进行中 `loading-circle` 转圈（14px）、成功 / 失败用 `circle-*`（16px）；单次耗时前端按 `toolCallId` 掐表（`TOOL_CALL_END` → `TOOL_CALL_RESULT`）
- 多出第四态 `pending`（`waiting` 字形 + `#ff9c01`）：**需确认的工具本轮拿不到结果是常态，不是异常**。`create_biz_apply` 这类工具在 `TOOL_CALL_END` 后被 `tool_confirm` 中断挂起，本轮直接 `RUN_FINISHED`；用户确认走的是 `sendMessage`，会开一条新的 user 消息即新一轮，结果落在下一轮里，永远配不回这条行（历史回放同样如此）。早先空态不画图标，标题被挤到最左，看着像丢了状态
- 思考条自绘（`process-thinking.vue`）：`思考中 (耗时 ：Xs)` / `已思考完成 (耗时 ：Xs)`，格式对齐 chat-x `1s851ms`；拿不到耗时（`/history`）时只写 `已思考完成`，不补 0；点标题条折叠
- 摘要箭头：收起 `AngleRight`、展开 `AngleDown`，20px；思考条箭头：收起 `AngleDown`、展开 `AngleUp`，16px。两处都按字形边界裁掉 1024 viewBox 的空白
- 会话卡片：`padding: 16px 24px 24px`、`gap: 16px`、`box-shadow: 0 12px 32px 0 rgba(0, 0, 0, 0.04)`；不设最大宽（与无过程区的一轮统一到 984）
- 用户气泡对齐稿面 [node 2060:20561](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/业务资源?node-id=2060-20561)：`#cddffe`、`padding: 12px 24px`、`border-radius: 16px 0 16px 16px`（右上直角，尖角朝右侧发言人）、`0 1px 1px rgba(0,0,0,.05)`、14/22。chat-x 默认是 `#e1ecff` / 8px / 4px / 无投影 / 12-20，五项全不同。字号写死而不动 `--ai-font-size`：那是 chat-x 的整档变量（`data-ai-size` small 12-20 / normal 14-24），改它会连 AI 正文和图标一起放大。气泡视觉全落在 `.ai-user-message-content`，其内 `.ai-text-content` 已被 chat-x 重置为透明无内边距
- 多行态（[node 2229:17681](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/业务资源?node-id=2229-17681)）容器样式与单行完全相同，唯一增量是**换行要保住**：稿面每行一个 `p` 且 `mb-0`，行间只有 22px 行高。chat-x 的 `text-content` 组件是 `toDisplayString(content)` 出一个纯文本节点（**不走 markdown**，所以没有 `.ai-markdown-body p` 那 10px 下边距的问题），但全链路没有 `white-space` 声明，用户 Shift + Enter 打的 `\n` 被 HTML 折叠成空格、多行糊成一段。故给 `.ai-user-message-content .ai-text-content` 开 `pre-wrap`；该文本节点是纯数据、不含模板缩进，不会重演工具行详情那次「容器开 pre-wrap 把标记缩进画成空行」
- 气泡变大后浮窗默认宽 400 不够（正文宽从 `400-40=360`@12px≈30 汉字掉到 `400-72=328`@14px≈23 汉字），默认宽改 **500** 补回容量。**但这会撞 `@container` 断点**：卡片内容盒 = 窗宽 − 48（普通轮次：`aa-messages` 左 8 + `message-group` 右 16 + 卡片自身 `8px 12px`）或 − 96（过程区轮次多 24px 左右内边距），原断点 380 在窗宽 > 428 时就失效，卡片会在仍然很窄的浮窗里跳成全页版式（账号选择的多列网格会挤）。故三处 `@container` 同步 380 → **460**（W=500 时两类卡片分别是 452 / 404，都留在紧凑版式；全页卡片约 900，不受影响）。三处共用 `.custom-msg-card` 的 `container-type`，必须一起改。副作用：手动把浮窗拖到 > 508 时普通轮次卡片会切全页版式，这是断点语义本身，非缺陷。宽度未做持久化，改默认值对所有人立即生效，`minWidth` 仍是 400
- 摘要文案 `调用N个工具，思考耗时X.XXs` / `思考耗时X.XXs`；无耗时时降级为 `调用N个工具` / `已完成思考`
- 过程区自绘节点补 `toScrollBottom` 跟随（`use-follow-scroll.ts`），否则流式输出滚不进可视区；五张场景卡同理，各挂一行 `useFollowScrollOnMount()`，否则末尾下发的卡片把内容顶出可视区后没人贴底
- chat-x 工具栏（`.ai-message-tools-container`）绝对定位到卡片下方，消除卡底恒占的空白
- 空助手消息（`content` 为空）渲染空壳并连外层条目一起 `display: none`，否则 0 高度条目仍吃两份 gap，该处间距翻倍

## 单据 2: 默认澄清交互实现

**TAPD**: [#1069995598137725247](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137725247)

**文件**:

- `src/hooks/chatbot/types.ts`
- `src/hooks/chatbot/use-stream.ts`
- `src/hooks/chatbot/use-hitl.ts`
- `src/components/chatbot/hitl-interrupt-card.vue`

**改动点**:

- `HitlInterruptValue.value.options` 放开为可选：agent 侧 `human_confirm` 的 options 本就是可选项，缺字段 / `null` / 空数组三种「无选项」都是合法的默认澄清，不是异常数据
- `parseHitlInterruptValue` 不再据 options 判 null（原先 `!Array.isArray` 与 `length === 0` 各拦一种），三种无选项一律归一为空数组；否则 `/history` 落 activity 兜底渲染成「活动消息：hitl.interrupt」
- `getHitlReadonlyState` 在无 options 时把后继 user 消息原文当答案回显；有 options 却未命中仍留空，不改既有保守判定
- 卡片 options 归一为 `?? []` 计算属性，空选项时不渲染选项区（否则空 `div` 仍占 16px `margin-bottom`）；只读态在有自由文本答案时保留输入框，无选项时去掉「其他选项：」标签、占位改「请输入您的回复...」

**要点**

单据只给了现象截图 + 一句「结合蓝鲸标准组件 + FinOps 当前实现」，无稿面、无验收标准、无 /history 口径（详见 tapd-analyst 解读）。形态经用户确认为**复用现有 HITL 卡片**（已是 bkui-vue `Input` + `Button`，满足「蓝鲸标准组件」），不引入 FinOps 参照。

坏渲染只在 `/history`：实时流 `use-event.ts` 压根不校验 options，`JSON.parse` 后直接推卡片，所以实时能出框；`/history` 的 `parseHitlInterruptValue` 却把 options 当必填。**但只放开解析器不够**——`use-hitl.ts` 的只读回填以「答案命中 options」为前提，无选项时永远命中不了，历史里会从错误文案变成空白只读卡（只剩「未选择或输入自定义」），用户当时输入的自由文本丢失。而一旦把自由文本回填出来，卡片 watch 里 `options.includes(answer)` 在 options 为 `null` 时的 TypeError 就会被激活（此前因 value 恒为 `''` 而踩不到）。四处必须连着改。

回显判据取「后继 user 消息」是安全的：HITL 卡确认走 `:on-confirm="sendMessage"`，**不带 `resumeValue`**（与主机申领 A/B/D 卡不同），澄清答案就是紧随其后的普通 user 消息，历史里不存在独立的 resume 回执可读，也就不需要 `resume_forwarded` 那套回填。

**假设**（单据未界定，已在实现中固化）：三种「无选项」同分支；只读回显仅对无选项新增，有选项的未命中行为保持不动；实时流路径不动（现状可用，复用解析器会扩大改动面）。

## 一、方案概要

工具行外观与内部详情按稿面手写，不复用 chat-x `ToolcallRender`（稿面行头/底色/字号与组件不一致）。思考条与思考正文同样手写，不走 `ReasoningMessage`（组件在无 duration 时不展示耗时、标题条样式也与稿面不一致）。过程区折叠是业务层：展示列表抽掉过程消息，插入 `__type=process.summary`；带 `toolCalls` 的助手消息改写成 `__type=process.tool`（剥掉 `toolCalls`，避免 chat-x 再画一层外壳）。

工具说明取自该次调用参数里的 `tool_intent`，挂在 `__toolCalls[].intent`，画在自绘行上方。`/agui` 与 `/history` 共用同一套拆分，不再走 `TEXT_MESSAGE_CHUNK` / 平级 `tool-intent-*` 消息配对。

## 二、实现要点

### 2.1 工具说明（参数 `tool_intent`）

- `/agui`：现网 `TOOL_CALL_ARGS.delta` 一次下发完整 JSON，前后可有空格（实测 ` {"query":"...","tool_intent":"..."} `）。trim 后 parse，取出 `tool_intent` 画在该行上方，并从展示用 arguments 里删掉该键。仍兼容逐 token 拼接。`parentMessageId` 不消费。
- 流式中 JSON 尚未闭合时，用已闭合的 `"tool_intent":"..."` 片段提前展示说明，完整 JSON 到达后再剥键
- `/history`：助手消息 `toolCalls[].function.arguments` 与 `/agui` delta 同形（实测 ` {"skill":"ziyan-cvm-apply","tool_intent":"..."} `）。SNAPSHOT 灌入时先 trim；一条消息带 N 个 toolCall 时各自从自己的参数取说明，不会糊成一句
- 缺字段或文案为空：不画说明行
- 旧协议遗留：`TEXT_MESSAGE_*` / SNAPSHOT 里 `id` 或 `messageId` 以 `tool-intent-` 开头的条目**只丢弃、不再取文案**。这套通道原先只为工具说明服务，现已改读参数，保留丢弃是为了过渡期与旧会话不冒出独立气泡

助手消息 `content` 里的正文（模型边调工具边说的话，实测 `/history` 恒为 `" "`、`/agui` 恒为 `""`）仍走 `__intent` 画在所有行之上，作为「模型确实说了话」的兜底，不与行级说明混用。

### 2.2 过程区

- 过程体：`role=reasoning`，或 `assistant` 且带 `toolCalls`（展示时改写成 `process.tool`，`role=tool` 结果并入行内后不再单独出泡）
- 中间说明 / 卡片 / 最终正文：不是过程体，收起时仍在
- 当前轮：`isChatting` 或用户 Stop（`stayProcessExpanded`）→ 展开
- 其它轮 / 历史：默认收起；只点摘要行切换
- 思考正文 / 工具行的内部折叠态与总开关解耦（点摘要行不重置内部），存取按「本轮是否仍在进行」分段（`detailKey` 的 `@live` / `@done`），默认值按**单条自身**是否在进行算，两者是两件事：
  - **默认值（单条粒度）**：思考条看 `streaming && status === Streaming`（`REASONING_END` 一到就收）；工具行看 `streaming && !toolMessage`，与行内 `resolveState` 的 running 同判据（**不能**看 `message.status`，`TOOL_CALL_END` 在入参给完时就置 Complete 了）。于是流式过程中同一时刻只摊开正在跑的那条，前一条完成即时收起，不必等整轮结束
  - **分段（整轮粒度）**：整轮结束时 key 切到 `@done` 段，流式期间的手动展开随之作废、全部落回收起；结束后用户自己点开的存在 `@done` 段里稳定保留。正文可能很长，结束后再展开总开关不该糊出一大片
  - 两条规则不冲突：默认值管「没人点过时显示什么」，分段管「点过的记多久」。手动优先——流式中点开一条已完成的，它不会再被自动收起（该条不会再变回进行中），直到整轮结束切段
  - 用户点停止后 `streaming=false`（`live` 仍为 true）：没人在跑，内部条目一律落回收起，过程区总开关仍保持展开

### 2.3 图标

- `bkhcm-icon-circle-success` / `circle-error` / `circle-time`
- 摘要箭头：收起 `AngleRight`、展开 `AngleDown`，`font-size: 20px`；思考条箭头：收起 `AngleDown`、展开 `AngleUp`，`font-size: 16px`
- bkui `Angle*` 画在 1024 viewBox 居中，左右各留大块空白：用 `overflow: hidden` 的盒子按字形实际宽度裁掉，`svg` 再 `margin-left: calc(-1em * 左缘 / 1024)` 拉回。`AngleUp` / `AngleDown` 水平边界相同（288→736），一条规则通吃、切换不跳字；`AngleRight` 左缘是 376

### 2.4 流式自动滚动

chat-x 的持续跟随不是容器主动做的：`message-container` 只在挂载时 `jumpToBottom` 两次，之后靠 **Markdown 渲染器每挂载一个 token 回调一次 `toScrollBottom`**。过程区改成手写结构后不再经过 Markdown 渲染，跟随触发点整体消失，流式输出会长到可视区外。

修法：`use-follow-scroll.ts` 用公开导出的 `useContainerScrollConsumer()` 拿到同一个滚动上下文（provider 就在 `message-container` 内，slot 内容可注入），节流 100ms 对齐 Markdown；`autoScrollEnabled === false`（用户上滚翻历史）时不抢滚动。

触发点只挂在「内容长高」上：思考正文看 `__text.length`，工具行看 intent + 各行 `args`/`result` 长度与行数，外加组件 `onMounted`（新过程条目出现 / 历史首屏）。**不看展开态**——用户手动展开是为了读过程，不该被拽到底部。

**场景卡片同属这一类**：申领方案 / 预提单 / 确认提交 / HITL / 账号选择五张卡也是手写结构、不走 Markdown，而它们多在一轮末尾才由 CUSTOM 事件下发。卡一挂载整块内容就长出可视区，却没有任何一处贴底，于是本轮结束后停在半路——卡片被截断、「返回底部」按钮亮着。五张卡各加一行 `useFollowScrollOnMount()`（`use-follow-scroll.ts` 导出）。只挂 `onMounted` 不跟内容变化：卡高在挂载时已定，数据来自 props 不异步取。

判据补充（读 chat-x `use-container-scroll`）：`autoScrollEnabled` **只在 wheel 事件 `deltaY < 0` 时置 false**，程序化滚动不会误关；`toScrollBottom` 缺省行为按距底距离二选一——超过 `INSTANT_SCROLL_DISTANCE`(600px) 瞬时 `jumpToBottom`，否则 `scrollIntoView({ behavior: 'smooth' })`。平滑分支的动画目标是发起那一刻的底部，所以贴底调用必须等 DOM 高度定了再发（`flush: 'post'` / `onMounted`），否则会停在半路。

### 2.5 工具行的状态与耗时

chat-x 原生 `ToolcallRender` 的能力对照（`toolcall-render` 组件）：状态位 `Pending`/`Streaming` 画 `bk-loading` mini 转圈、`Success`/`Complete` 画对勾、`Error` 画叉，后面缀「调用中 / 调用成功 / 调用失败」文字，末尾 `toolcall-duration` 读 `props.duration || toolCall.toolMessage?.duration` 再走 `formatDuration`。**耗时能力在组件里是有的，但值要前端自己喂**——协议不发（见 api.md §2.4）。

手写行按稿面只保留图标、不带状态文字（对勾/叉与文字语义重复）。两处实现要点：

- **状态判据不能用 `message.status`**：`TOOL_CALL_END` 只代表入参给完，此时状态已是 `Complete`，工具还在跑，旧实现会提前画对勾。改成按「有没有配到 `role=tool` 结果消息」判完成，没结果时再看本轮是否仍在流式
- **`__streaming` 与 `__live` 分开**：`__live` 含「用户 Stop 后保持展开」，用它判进行中会让停止后的行一直转圈。`__streaming` 只取 `isLastTurn && isChatting`，停止后未返回的行状态未知、状态位留空
- **耗时前端掐表**：`use-event.ts` 用 `Map<toolCallId, 起点>`，`TOOL_CALL_START` 落一次、`TOOL_CALL_END` 再覆盖一次（起点挪到入参给完，不把模型吐参数的时间算进去；缺 END 的通道自动沿用 START），`TOOL_CALL_RESULT` 结算写进结果消息的 `duration`。事件真带 `duration` 时优先用事件值。`RUN_FINISHED` / `RUN_ERROR` 清表，避免没等到结果的计时跨轮累积
- 拿不到耗时就不写 `duration` 字段（`/history` 恒缺），工具行省掉耗时段，不显示 `0ms`
- **进行中不用 `bk-loading`**：`size="mini"` 是 bkui 最小档（`.bk-spin-indicator` 16×16，8 枚 oval 铺满），比同列 `circle-*` 字形显眼，且尺寸写死在组件内、`font-size` 管不到，要更小只能 `scale` 外层。改用项目 iconfont 的 `bkhcm-icon-loading-circle` 配 `@keyframes`（同 `OrganizationSelect` 既有写法）：同一套字体度量、天然对齐，尺寸直接由 `font-size` 定，取 14px 与 16px 的 `circle-*` 视觉等重

### 2.6 工具详情的 `pre-wrap` 只能给取值节点

详情区字段之间会凭空多出一行（实测行距 68px，按 `line-height: 20` + `margin-bottom: 8` 只该有 28px）。原因是 `white-space: pre-wrap` 开在了容器 `.process-tool-detail` 上：**标记里的缩进换行也被当成真换行画**，两个 `<p>` 之间的空白文本节点就撑出一个 20px 空行；数据里自带的首尾换行（如 `tool_intent` 结尾的 `\n`）同样会显出来。

修法两层：

- 结构层：容器不再开 `pre-wrap`，只在 `.process-tool-value`（描述 / 参数）与 `.process-tool-result`（返回内容 `pre`）上开——要保留换行的只有取值本身。段间距同时从 8px 收到 4px
- 数据层：单行摘要的取值用 `inline()` 把空白压成空格，多行内容用 `squeezeBlankLines()` 去首尾、最多留一个空行

三字段改成常显、缺值写 `--`，所以详情区的 `v-if` 只看展开态，不再看「有没有内容」。

### 2.7 空助手消息不能留在列表里占位

卡内某处间距会翻倍成 32px（实测：正常 gap 16、异常处 34）。原因不是 gap 变大，而是**中间夹了一条高度为 0 的助手消息**：模型发了 `TEXT_MESSAGE_START` 开泡，正文却一直为空。它不是过程体（`isProcessBody` 不认），过程区收起也带不走它，所以两种状态下都在吃 gap。

flex 的 `gap` 只看「相邻两项之间」，与项的尺寸无关：0 高度的项照样在自己两侧各留一份 gap。只隐藏它的内容（`.ai-markdown-body`）没用，必须让 `.ai-message-item` 本身 `display: none` 才不参与 gap 计算。

修法沿用过程区那套：模板里给这类消息渲染 `.blank-message` 空壳（顶掉 chat-x 的默认渲染），样式按 `.ai-message-item:has(.blank-message)` 连外层条目一起收掉。消息仍留在列表里（chat-x 消息 `v-for` 按下标做 key，中途增删会让整轮 DOM 重建），正文一到空壳条件就不成立、自动回到默认渲染。

## 三、明确不做

独立全量按钮；结构化参数浏览器；写死 MCP 文案；套 chat-x `ToolcallRender` 改皮肤；改 `/agui` `/history`；工具名中文表。
