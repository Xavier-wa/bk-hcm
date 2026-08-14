# Coding：aiagent - 资源管理首页 chatbot - 默认主内容区

## 变更摘要

| 文件 | 变更 |
|------|------|
| `views/chatbot/constants.ts`（新增） | 大卡片 `BIG_CARDS`、小卡片 `PROMPT_CHIPS`、`SESSION_TAG_NAME` 映射 + `resolveSessionTagName` |
| `store/chatbot/session.ts` | `createSession(bkBizId, sessionName, sessionTag?)` —— 带 tag 时请求体加 `session_tag` |
| `hooks/chatbot/use-session.ts` | `createSession(sessionTag?)` 透传；带 tag 时**不复用**无 tag 空会话，直接新建 |
| `hooks/chatbot/use-chatbot.ts` | `sendMessage(content, sessionTag?)` 透传给 `createSession` |
| `views/chatbot/index.vue` | 欢迎区 + 大卡片栅格 + 小卡片排（分段翻页）+ 点击交互 + 文件夹名映射 |

## 行为说明

1. **首页空态判定**：`isHomeEmpty = !currentSessionCode && isEmpty`。
   - 首页空态 → 展示「欢迎区 + 大卡片栅格」+ 输入框上方「小卡片排」。
   - **已选中会话但无消息** → `chat-blank` 空白（不回退默认内容，design §5）。
2. **欢迎区**：`@/assets/image/cloud-assistant.svg` 云朵图标 + 「海垒 AI 助手」+ 功能 tips。
3. **大卡片（点击直发）**：`handleBigCardClick` → `sendMessage(card.prompt, card.sessionTag)`，复用现有惰性建会话/SSE；发送中禁用。响应式栅格 `repeat(auto-fit, minmax(280px,1fr))`，数量由 `BIG_CARDS` 驱动（默认 4）。
4. **小卡片（注入待发）**：`handlePromptChipClick` → 编辑器内注入**默认词文本** + `focus()`，场景 **chip 由 `ChatInput` 的 `#input-header` 插槽自渲染**（可点 × 移除），记录 `pendingChip`；用户点发送时 `handleSendMessage` 取 `pendingChip.sessionTag` 上报并清空。数量由 `PROMPT_CHIPS` 驱动（默认 10）。
   - ⚠️ **为何不用组件 tag 节点**：经 chat-x 源码确认，`ChatInput` 的 v-model 外部更新会用 `docToString` 把内容拍平成纯文本再 `ReplaceAll` 回填（tag 节点序列化为 `@label`），**无法通过 v-model 注入 chip**；组件也未暴露编辑器内插入 tag 的 API。故场景 chip 改为业务侧自绘。
   - **chip 两列布局（贴稿面）**：`.scene-chip` 绝对定位在编辑器左列（`top:6px;left:8px`，相对 chat-x `.chat-input`(position:relative) 定位，对齐 `.ai-slash-input` 8px 内边距）；`.has-scene-chip` 下给 `:deep(.ai-slash-input)` 设 **`padding-left: calc(var(--scene-chip-w) + 16px)`**（左偏移8 + chip宽 + 间距8），让**所有行**（含换行、空态光标、placeholder）整体右移到右列，与首行对齐——区别于早期 `text-indent` 仅缩进首行导致换行回左、空态光标落在 chip 前的问题。`--scene-chip-w` 由 `watch(sceneChip)` 后 `nextTick` 实测 `sceneChipRef.offsetWidth` 写入。⚠️ 依赖 chat-x 编辑器内部类名 `.ai-slash-input`，升级可能需复核。
5. **场景标识**：通过 `create_session` 的 `session_tag` 入参传递（**非**发消息字段）。目前仅 `host_apply`（主机申领大卡片 #1/#4、主机申领小卡片 #1）。
6. **会话分组联动**：带 `session_tag` 创建的会话，列表返回同字段 → 自动归入标签文件夹（上一迭代能力）；侧栏文件夹名经 `resolveSessionTagName` 映射（`host_apply` → 主机申领）。
7. **小卡片分段翻页**：`recomputeChipSegments` 按视口宽度累计 chip 宽度动态分段，`ResizeObserver` 监听容器变宽重算；`‹`/`›`（bkui `AngleLeft/AngleRight`）切段，单段放得下不显示箭头。每次进入首页空态重新绑定监听（v-if 重挂载）。

## 图标映射（design §3.x → 实现）

| 语义 | 实现 |
|------|------|
| 欢迎区云朵 | `@/assets/image/cloud-assistant.svg` |
| 主机申领 | `bkhcm-icon-host-application` |
| GPU/库存 | `bkhcm-icon-host-inventory` |
| 主机回收 | `bkhcm-icon-host-recycle` |
| 服务器(多台) | `bkhcm-icon-host-multi` |
| 预测提单/调整 | `bkhcm-icon-resource-plan` |
| CLB | `bkhcm-icon-loadbalancer` |
| 安全组 | `bkhcm-icon-security-group` |
| 小卡片翻页 | bkui `AngleLeft` / `AngleRight` |

## 卡片文案（合理默认，可微调）

- 见 `views/chatbot/constants.ts`；GPU 库存描述稿面截断处补全为「…及使用率」。

## 已知边界 / 未实现

- 场景 chip 为业务侧 `#input-header` 插槽自绘（非编辑器内 tag 节点），点 × 仅清场景标识、保留已输入文本。
- `host_apply` 之外的场景暂无 `session_tag`（落入未分组），后续补 `SESSION_TAG_NAME` 即联动文件夹。
- 卡片后台配置管理（非本迭代）。

## 稿面对齐微调（二次调整）

1. **面包屑改为全局**：移除主内容区自定义 header（原显示会话标题）及 `currentTitle`，改由全局面包屑承载。`route-config.ts` 将 `layout.breadcrumbs` 设为 `{ show: true, back: false }`，复用 `home/breadcrumb.tsx` 展示路由 `title`「首页」（与任务管理一致），首页无返回箭头。
2. **底部联系人**：`.chatbot-chat` 末尾新增 `.chatbot-footer`「有任何问题可联系 @小助手」，`@小助手` 用 `@/components/w-name`（`ASSISTANT_CONTACT` 配置 `name`/`alias`，点击拉起企微）。⚠️ `name` 暂用「小助手」占位，待确认真实企微账号后替换（见 `constants.ts` TODO）。
3. **背景/间距/留白（响应式）**：
   - 主内容区背景 `--main-bg` 由 `#fff` 改 `#f5f7fa`（卡片/输入框保持白色形成层次）；小卡片 chip 与翻页按钮背景改白 + 边框，避免与灰底融合。
   - 上边距收紧：欢迎标题 `margin-top` 16→12px，大卡片 `margin-top` 32→20px。
   - 左右留白随视口收缩：`.home-default` 横向内边距改 `clamp(16px, 4vw, 40px)`；大卡片 `max-width` 720→960px、列 `minmax(280→360px)`（宽屏 2 列、窄屏自适应降列）。

## 稿面对齐微调（三次调整）

1. **小卡片排宽度对齐输入框**：chat-x `.chat-input` 为 `width:100% / max-width:1000px` 居中，宽屏下窄于外层。`.prompt-chips` 改为外层 `padding:0 16px` + 内层 `.prompt-chips-inner`（`width:100%; max-width:1000px; margin:0 auto`），与输入框同宽同居中。
2. **翻页改单按钮（右侧）**：移除左右双按钮，改为右侧单按钮 `toggleChipSegment`：未到末段显示 `>` 前进一段，到末段显示 `<` 回到首段（`isChipAtEnd` 控制图标 `AngleRight/AngleLeft`）。按钮样式按稿面：28×28 圆角方形（radius 6）、浅灰底 `#f0f1f5`、灰色箭头。
3. **小卡片 icon 颜色**：`.prompt-chip-icon` 固定 `#699df4`。
4. **场景 chip 去 icon + 只读回显**：scene-chip 移除图标。新增 `sceneChip` 计算属性——首页点小卡片 → 可关闭待发 chip；进入带 `session_tag` 的会话 → 按 `resolveSessionTagName` 回显**只读** chip（无 × 关闭）。`has-scene-chip` 让位与宽度实测改由 `sceneChip` 驱动。

## 验证建议

- 有 `bizs` 时进入 chatbot 首页：展示欢迎区 + 4 大卡片 + 小卡片排。
- 点大卡片「申领 10 台主机」→ 直接发送、进入会话，侧栏该会话归入「主机申领」文件夹。
- 点小卡片「主机申领」→ 输入框出现 tag chip + 默认词、发送按钮变蓝；点发送后会话归入「主机申领」文件夹。
- 点其他小卡片（如「CLB申领」）→ 注入待发；发送后落入未分组。
- 窄屏/拖拽改变宽度：大卡片列数自适应；小卡片放不下出现 `‹`/`›`，点击分段翻页。
- 切到已有空会话 → 主内容区空白（不展示默认内容）；点「新对话」→ 回到首页空态重新展示。
