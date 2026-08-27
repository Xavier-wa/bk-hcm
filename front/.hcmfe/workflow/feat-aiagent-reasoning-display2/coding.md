# Coding — feat-aiagent-reasoning-display2

**TAPD**: [#1069995598137285011](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137285011) aiagent-reasoning展示处理优化
**需求文档**: `front/docs/reqs/思考记录展示优化.md`

## 现状诊断

线上前后不一致的**根因不是「后端多吐事件」，而是前端事件名停留在已废弃的 thinking 系列**：

- `src/hooks/chatbot/types.ts` 的 `EventType` 只枚举了 `THINKING_*`，agui 实际下发的是 `REASONING_*`
- `src/hooks/chatbot/use-event.ts` 的 switch 因此全部落到 `default: break`，实时流的推理事件被静默丢弃
- history 回放走的是另一条路（`use-stream.ts` 的 `toHistoryMessage`），后端快照里已带 `role: 'reasoning'`，被通用分支正常还原成 chat-x 的 `ReasoningMessage`

所以「聊的时候没有、刷新后冒出来」= 实时流丢事件 + history 正常展示。

另外发现现有实现的两个隐患，本次一并处理：

1. `getCurrentStreamingMessage()` 是 `findLast(status === Streaming)`。推理消息与正文消息可能同时处于 streaming，靠「最后一条 streaming 且 role 是 reasoning」定位推理消息并不稳。
2. `RUN_FINISHED` / `RUN_ERROR` 只收尾**一条** streaming 消息。若推理没收到 END 事件，会永远卡在「思考中」。

## 组件能力核对（已查证，不需要自研 UI）

`@blueking/chat-x` 的 `ReasoningMessage`（`MessageRender` 按 `role=reasoning` 自动挂载）：

| 输入 | 表现 |
|------|------|
| `status = streaming` | 标题「思考中」+ `AiLoading` 动画，内容区展开 |
| `status = complete` | 标题「已思考完成」 |
| `status = complete` + `duration` 真值 | 标题「已思考完成 (耗时：x.xs)」，并**自动折叠一次**（内部 `watch` 命中后 `stop()`，之后完全由用户控制） |
| `status = error` | 标题「思考失败」，内容区换成 `CommonErrorContent` |
| `content` | `string[]`，每项独立渲染为一段 `MarkdownContent` |

结论：本期只需把实时流事件正确翻译成这条消息的 `content` / `status` / `duration`，UI 零改动。

## 改动清单

**文件**: `src/hooks/chatbot/types.ts` `src/hooks/chatbot/use-event.ts` `src/views/chatbot/AI_SPEC.md`

### 1. `types.ts` — 事件枚举直接替换

删除 5 个 `Thinking*` 成员，替换为 reasoning 系列并补齐两个非主链路事件（需求已确认**不做双兼容**）：

```ts
ReasoningStart = 'REASONING_START',
ReasoningMessageStart = 'REASONING_MESSAGE_START',
ReasoningMessageContent = 'REASONING_MESSAGE_CONTENT',
ReasoningMessageEnd = 'REASONING_MESSAGE_END',
ReasoningEnd = 'REASONING_END',
ReasoningMessageChunk = 'REASONING_MESSAGE_CHUNK',
ReasoningEncryptedValue = 'REASONING_ENCRYPTED_VALUE',
```

### 2. `use-event.ts` — 推理事件分发改造

**用闭包变量显式跟踪当前推理消息**，不再依赖 `getCurrentStreamingMessage()` 猜：

```ts
// 当前进行中的推理消息与其开始时刻（一轮对话可能有多个推理阶段，逐个复位）
let currentReasoning: Message | null = null;
let reasoningStartedAt = 0;
```

各事件处理：

| 事件 | 处理 |
|------|------|
| `REASONING_START` | 新建 `role=Reasoning` / `content=[]` / `status=Streaming` 消息并 push；记录 `currentReasoning` 与 `reasoningStartedAt = Date.now()` |
| `REASONING_MESSAGE_START` | 有 `currentReasoning` 才 `content.push('')` 开启新一段 |
| `REASONING_MESSAGE_CONTENT` | 有 `currentReasoning` 且已有段落才把 `delta` 追加到最后一段；`delta` 非字符串则忽略 |
| `REASONING_MESSAGE_END` | 空实现（段落边界由下一次 `MESSAGE_START` 划分） |
| `REASONING_END` | 结算：`duration` 优先取事件下发值（需为非负有限数），否则 `Date.now() - reasoningStartedAt`；置 `status=Complete`；清空 `currentReasoning` |
| `REASONING_MESSAGE_CHUNK` | 便捷合并事件，一条即一整段：有 `currentReasoning` 时 push 整段；无则新建一条推理消息承载（见下方待确认项 A） |
| `REASONING_ENCRYPTED_VALUE` | 显式 `break`，加密思维链不展示、不落内容 |

**异常收尾**：`RUN_FINISHED` / `RUN_ERROR` 分支增加对 `currentReasoning` 的兜底结算（补 `duration`、置 `Complete`、清空），保证「只有开始没有结束」时不卡在「思考中」。抽成一个 `finalizeReasoning()` 内部函数供 `REASONING_END` 与两个 run 分支复用。

`RUN_ERROR` 时的推理消息按**完成**收尾而不是 `Error`：`error` 会把标题变成「思考失败」并用 `CommonErrorContent` 覆盖内容区，而实际上此前已产出的推理正文是有效的；错误本身由紧随其后的 assistant 错误气泡承载。

**`TEXT_MESSAGE_CHUNK` 防护**：该分支同样用 `getCurrentStreamingMessage()` 取消息并对 `content` 做字符串 `+=`。改动前推理消息从未被创建，这条路径不可能命中推理消息；改动后推理消息会真实进入 streaming，若此时来一条 `TEXT_MESSAGE_CHUNK`，`findLast` 就会取到推理消息并把数组 content 字符串化。因此加一个 `role !== Reasoning` 守卫，命中时另起一条 assistant 消息（对应 AC-007）。

### 3. `AI_SPEC.md` — 事件表同步

把「思考」小节的 5 行 `THINKING_*` 换成 7 行 `REASONING_*`（含 CHUNK 与 ENCRYPTED_VALUE 的处理说明），并补一句耗时由前端统计。

### 4. 升级 `@blueking/chat-x` 0.0.42 → 0.0.50（编码期追加）

**文件**: `package.json` `package-lock.json`

原计划「UI 零改动」，但联调发现需升级组件库。用户要求升到 0.0.42，查证后 0.0.50 为当前最新稳定版，遂直接升到 0.0.50。升级带来两个必须处理的连带问题，见下方 5、6。

### 5. 修复推理消息卡在「思考中」（编码期追加）

**文件**: `src/hooks/chatbot/use-event.ts`

`startReasoning()` 先构造字面量对象 push 进 `msg.messages.value`，再把**同一个原始对象**赋给 `currentReasoning`。`messages` 是 `ref<Message[]>` 的深响应式数组，push 后数组里存的是代理，而闭包持有的是原始对象——后续 `finalizeReasoning()` 改 `status` / `duration` 都写在原始对象上，不触发依赖收集，视图始终停在「思考中」，多轮推理就并排堆出多个「思考中」。

修法：push 后用 `getMessageByMessageId(messageId)` 回查拿到响应式代理再持有。

```ts
msg.messages.value.push({ /* ... */ messageId, status: MessageStatus.Streaming } as unknown as Message);
// 数组里存的是原始对象，必须回查取到响应式代理再持有，否则后续改动不触发视图更新
const message = msg.getMessageByMessageId(messageId) as Message;
currentReasoning = message;
```

### 6. 收敛消息悬浮工具栏（编码期追加）

**文件**: `src/components/chatbot/chat-message-list.vue`

升级后消息 hover 冒出 7 个操作（复制/引用/重新生成/分享/点赞/不满意/删除），仅「复制」「重新生成」有实现。根因是新版把工具栏容器类名加了 `ai-` 前缀（`.message-tools-container` → `.ai-message-tools-container`），且工具图标不再有 `ai-cite-icon` / `ai-share-icon` 这类区分类名（统一为 `ai-common-icon`），原先按这些选择器隐藏按钮的 scoped 样式整段失配。

| 位置 | 手段 | 说明 |
|------|------|------|
| AI 消息 | `MessageContainer` 的 `messageTools` / `updateTools` props | 新版支持按 `id` 与内置列表合并，`hidden: true` 的项被 `.filter(e => !e.hidden)` 真正剔除；`updateTools` 清空后分隔线（`v-if="updateTools.length > 0"`）一并消失 |
| 用户消息 | CSS 按位置隐藏第 2、4 个 | `UserMessage` 内部写死 `message-tools={CONST_USER_MESSAGE_TOOLS}`（复制/引用/编辑/删除）、`update-tools={[]}`，`MessageContainer` 与 `ChatContainer` 均未暴露透传入口（已通过 chat-x MCP 文档 + dist 产物双向确认）；`ToolBtn` 根节点统一为 `.ai-tool-btn` 无法按 id 区分，只能按位置 |

同时删除三段已彻底失效的 scoped 样式：`.message-wrapper`、`.message-tools-hover` 两个类在新版 dist 中已不存在，`.message-tools-container` 及按图标类名隐藏的规则也匹配不到任何元素。

> **维护风险**：用户消息侧的 `nth-child(2)` / `nth-child(4)` 依赖 `CONST_USER_MESSAGE_TOOLS` 的顺序，组件升级若调整该列表需同步修改。已在代码注释中标注。

## 不改动的部分

- `use-stream.ts` 的 history 回放路径：`role='reasoning'` 已被通用分支正常还原，无需改。history 快照不带 `duration`，因此回放时标题是「已思考完成」（无耗时）且默认展开——这是**改动前的既有行为**，本期不动，避免扩大范围。
- `ReasoningMessage` 组件本身。（组件库整体升级见第 4 项；样式仅动 `chat-message-list.vue` 的工具栏隐藏规则，见第 6 项）

## 已定项

**A. `REASONING_MESSAGE_CHUNK` 在没有 `REASONING_START` 时自建推理消息**（用户确认）。
需求文档 F-004 写的是「便捷合并事件正常解析并展示」，F-001 异常处理写的是「先收到增量、没收到开始 → 忽略」，两条对 CHUNK 的指向不一致。结论取**自建**：CHUNK 语义上自成一段完整正文，不属于「半截增量」，因此不套用 CONTENT 的忽略规则。自建的消息同样进入 `currentReasoning` 跟踪，由 `REASONING_END` 或 run 收尾结算。

## 待确认项

**B. 线上是否真的下发 END 系列**（需求文档 Q-001）。
本方案对缺失 END 已有兜底（run 收尾结算），无论下发与否都能正确收口，不阻塞编码。

## 验收对照

| 验收项 | 覆盖方式 |
|--------|----------|
| AC-001 实时流出现「思考中」并流式追加 | `REASONING_START` + `MESSAGE_START` + `MESSAGE_CONTENT` |
| AC-002 结束后「已思考完成（耗时）」并自动折叠 | `REASONING_END` 结算 `duration`，组件自动折叠一次；实际生效还依赖改动 5 的响应式修复 |
| AC-003 与 history 回放一致 | 两端都产出同形态的 `role=reasoning` 消息 |
| AC-004 多次推理各自独立成条 | 每个 `REASONING_START` 新建消息，`REASONING_END` 复位 |
| AC-005 加密思维链不展示 | `REASONING_ENCRYPTED_VALUE` 显式忽略 |
| AC-006 未覆盖事件静默忽略 | `default: break`（既有行为） |
| AC-007 不影响既有消息类型 | 改动只落在推理分支与 run 收尾，不触碰工具调用 / 卡片 / CUSTOM |
| AC-008 无推理事件时表现同改动前 | 无 `REASONING_START` 则不产生任何推理消息 |
