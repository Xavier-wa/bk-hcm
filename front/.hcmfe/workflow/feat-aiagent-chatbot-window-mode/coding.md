# Coding：aiagent · chatbot 聊天窗模式

> 实施前必读：本迭代 shell_only，**严格参照 `@blueking/ai-blueking`** 窗口外壳；框内对话能力**复用现网 `useChatbot()` + `@blueking/chat-x`**（与全页一致）；图标**从 ai-blueking 源码搬运**。
> 参考源：ai-blueking `bk-aidev-agent/.../packages/ai-blueking`；现网 `hcm/front/src/views/chatbot` + `src/hooks/chatbot/*`。

## 1. 总体策略

- 新建一个**页面级、可导入**的浮窗组件 `AiAssistant`，内部 = 悬浮球 + 可拖拽可缩放面板（标题栏 + 对话区）。
- **对话区不重写**：抽取现网 `views/chatbot/index.vue` 中「对话区」部分（chat-x 接线 + `useChatbot()` handlers）为可复用子组件，全页与浮窗共用。
- **每页独立实例**（D1）：浮窗各页各自 `useChatbot()`，**不做全局单例**；会话共享靠后端（同一 `bizs/{biz}/sessions` + 同一 `sessionCode`）。
- **不改后端、不改 `agent.ts`、不动 `@deprecated` 文件**。

## 2. 目录与文件（新增，kebab-case）

`src/components/ai-assistant/`（全局可复用组件目录）：

| 文件 | 职责 | 来源 |
|------|------|------|
| `index.vue` | 浮窗壳：teleport→body + 悬浮球 + 可拖拽容器 + 面板(标题栏+对话区) + 显隐/压缩状态机 | 参照 ai-blueking `ai-blueking.vue` |
| `nimbus.vue` | 悬浮球（右下停靠、可隐藏、点击开面板、拖拽 Y） | 搬运 ai-blueking `views/nimbus.vue` + `use-nimbus.ts` |
| `draggable-container.vue` | 拖拽 + 缩放容器（几何/压缩/最小尺寸/最大宽度） | 见 §4 依赖决策 |
| `use-draggable.ts` | 几何计算 + `toggleCompression`（缩小高度/恢复默认尺寸） | 搬运 ai-blueking `containers/use-draggable.ts` |
| `header.vue` | 标题栏：标题 + 操作图标（新增会话/历史/缩小高度·恢复尺寸/关闭），drag-handle | 参照 ai-blueking `components/ai-header/index.vue`；图标用项目自有 iconfont + bkui-vue（CD3），下拉用 bkui-vue 替代 tippy |
| `chat-panel.vue` | **对话区**（chat-x `MessageContainer` + `ChatInput` + HITL 卡 + `useChatbot` handlers） | 抽取自现网 `views/chatbot/index.vue` |
| `types.ts` | 组件 props/expose 类型 | 新增 |

> 图标用项目自有 iconfont（见 §6），不新建 `assets/icon/` 搬运目录；仅头像 png 复制到 `front/src/assets/image/`。
> 对话区抽取后，现网 `views/chatbot/index.vue` 改为复用 `chat-panel.vue`（保留其侧栏/首页空态/路由同步等全页专属逻辑），避免两份 chat-x 接线漂移。**此为温和重构，范围限定在对话区抽取**。

## 3. 复用现网对话内核（来自现网 hooks）

`chat-panel.vue` 复用 `useChatbot()`（`src/hooks/chatbot/use-chatbot.ts`）：

- 状态：`messages / isChatting / isLoadingHistory / sessions / currentSessionCode / currentSession`
- 方法：`sendMessage / regenerate / resendEdited / stopGeneration / switchSession / createSession / deleteSession / renameSession / goHome / initSessions / reloadSessions`
- chat-x 接线（与全页同款）：`useMessageGroup`、`messageStatus = isChatting ? Streaming : Complete`、`MessageContainer`(+HITL slot)、`ChatInput`(`:support-upload="false"` 与全页一致)、`handleSendMessage/handleStopSending/handleAgentAction/handleUserInputConfirm`。
- **浮窗模式裁剪**：不做 `views/chatbot/index.vue` 的路由同步（`watch(currentSessionCode)`/`routerAction.redirect`），改为直接 `switchSession(code)`。

### 3.1 业务上下文（biz）

- `useChatbot` 经 `useWhereAmI().getBizsId()` 取 biz：优先 `accountStore.bizs` → `?bizs=` → localStorage。
- 浮窗为全局可用：挂载页需保证 biz 上下文存在；联动跳转时 query 带 `bizs`。

### 3.2 会话定位（联动/初始化）

- 用 `initSessions(targetCode?)`：先 `loadSessions()`，若 `targetCode` 在列表则 `switchSession(targetCode)`。
- 约束：`switchSession` 仅对**已在列表**的会话生效，故必须先 `initSessions()`。
- 历史经现网 `POST /api/v1/agent/history` 加载（`use-stream.ts fetchHistory`）。

## 4. 依赖决策（需用户确认，见 §9）

ai-blueking 外壳依赖两个项目未安装的库：

| 能力 | ai-blueking 用 | 项目现状 | 建议 |
|------|----------------|----------|------|
| 拖拽 + 缩放面板 | `vue-draggable-resizable@^3` | 无（仅 `vue-draggable-plus`，不适用） | **方案 A（推荐，最忠实）**：新增 `vue-draggable-resizable` 依赖；**方案 B**：用原生 pointer 事件自研轻量拖拽/缩放（无新依赖，工作量更大） |
| 标题栏下拉/tooltip | `vue-tippy` + `tippy.js` | 无（有 bkui-vue） | **用 bkui-vue 的 `Popover`/`bkTooltips` 替代**，不新增 tippy 依赖 |

> 团队规范：不轻易加依赖。拖拽缩放是本需求核心交互，"完全参照 ai-blueking" 倾向方案 A；最终由用户在 §9 拍板。

## 5. 壳层关键实现（**完全复刻 ai-blueking 源码**）

> 用户强约束：悬浮球交互、窗口背景色等细节需**完全复刻 ai-blueking**，逐项对照源码实现（除图标改用项目自有资源 CD3、下拉/tooltip 改用 bkui-vue 不引 tippy 外，视觉与交互保持一致）。

- **根层**：`teleport to="body"` → `.ai-assistant`(`position:fixed; inset:0; z-index:10000; pointer-events:none`)；子层悬浮球/面板 `pointer-events:auto`（复刻 `ai-blueking.vue` `.ai-blueking-v2`）。
- **面板几何**（复刻 `containers/use-draggable.ts`）：默认宽 400、右对齐、视口满高；min 400×400；max 宽 = min(80% 视口, 1000)。
- **缩小高度/恢复默认尺寸**：`toggleCompression()` 压缩态停靠右下、高度 ~800，再次点击恢复初始几何（标题栏右1图标，D4）。
- **拖拽手柄**：标题栏 `.drag-handle`；拖拽/缩放时给页面 iframe 加 `pointer-events:none`（复刻 ai-blueking）。
- **状态机**：`panelVisible`（开/关）、`nimbusMinimized`（球锚定）、`isCompressed`（高度压缩）三 ref + show/hide/toggleCompression（简化 ai-blueking ComponentManager 为本组件内 ref）。

### 5.1 悬浮球交互（完全复刻 `views/nimbus.vue` + `use-nimbus.ts`）
- **默认位置**：`top = innerHeight - 48 - 40`、`left = innerWidth - 48 - 16`（右下）；window resize 时 Y 相对底部、X 贴右。
- **尺寸**：48×48 圆，白底+阴影，内层 `#f0f5ff` wrapper，头像 `avatar.png`（CD3 复制到项目）。
- **点击 vs 拖拽**：mousedown→mouseup 时长 >200ms 视为拖拽（拖拽后不触发 click）；点击 → 开面板。
- **拖拽**：仅 Y 轴可拖（沿用 ai-blueking `:parent="true"` 行为，用 CD1 的 vue-draggable-resizable）；面板打开时球拖拽失活（`:active="!isPanelShow"`）。
- **hover 态**（见附图）：显示 `Cmd + I` 快捷键提示 + "最小化，将缩成锚点" 的 minus 按钮；点击 minus → `nimbusMinimized` 切换，锚定态 `transform: translateX(26px)` 半隐到右侧边缘。
- **快捷键**：`Cmd/Ctrl + I` 切换面板显隐（复刻 `utils` isTogglePanelShortcut）。
- **tooltip 实现**：用 bkui-vue tooltip 等效呈现（不引入 tippy）；视觉对齐 ai-blueking。

### 5.2 背景色/视觉（完全复刻 `ai-blueking.vue` 样式）
- 面板 `.ai-assistant-panel`：圆角 12、阴影 `0 2px 12px 0 rgb(0 0 0 / 20%)`。
- 背景：欢迎态 `linear-gradient(...) , #fff`，有消息态（`has-messages`）纯 `#fff`——**取值逐一对照 ai-blueking `.ai-blueking-panel` 源码**。
- 头部 48px；对话区高度 `calc(100% - 48px)`。
- CSS 类名用 kebab-case（项目规范），但**颜色/渐变/阴影/间距数值与 ai-blueking 源码一致**。

## 6. 图标（用项目自有图标，CD3）

不搬运 ai-blueking 图标字体。标题栏/悬浮球图标用项目自有资源（类名已核对存在）：

| 语义 | 落地 |
|------|------|
| 新建会话 | `<i class="hcm-icon bkhcm-icon-chat-plus" />` |
| 历史会话 | `<i class="hcm-icon bkhcm-icon-lishijilu" />` |
| 缩小高度 | `<i class="hcm-icon bkhcm-icon-zoomout" />` |
| 恢复默认尺寸 | `<i class="hcm-icon bkhcm-icon-fullscreen" />` |
| 关闭 | bkui-vue `<close-line />`（`bkui-vue/lib/icon`） |
| 悬浮球头像 | 复制 ai-blueking `assets/images/avatar.png` → `front/src/assets/image/`，组件 import 引用 |

输入区图标来自 chat-x，无需处理。

## 7. 跨形态联动（US-7 / AC-11）

### 7.1 目标侧（本迭代构建）
- CVM 服务申领页（`views/service/service-apply/cvm/index.tsx`）挂载 `AiAssistant`（页面级实例）。
- 进入页面读取 query `sessionCode`（+ `bizs`）：有则**自动唤起面板** → `initSessions(sessionCode)`。
- 异常（会话不存在/无权限，D2）：唤起空面板/新会话并提示。

### 7.2 源侧（CD5：本迭代不做）
- 「添加到配置清单」源侧按钮触发跳转**本迭代不实现**；目标侧支持从 query `sessionCode` 自动唤起（手动 URL 或后续源侧按钮均可驱动）。
- 后续接源侧时再调查 agent 渲染内容的按钮触发通道（`on-agent-action` 新增 action 或自定义消息渲染）。

## 8. 约束与不做项

- 不改后端接口、不改 `src/store/chatbot/agent.ts`（含 `BK_HCM_AJAX_URL_PREFIX`）。
- 不动 `@deprecated`（`router/module/`、`views/home/` 等）。
- 路由跳转一律 `routerAction`，name 用 Symbol。
- 表单状态（如有）用 `reactive(initialState)`，禁 `useFormModel`。
- CSS kebab-case、禁工具类间距；bkui-vue 复杂组件查 MCP 文档。
- 不引入划词（AiSelection）。

## 9. 已确认决策（用户逐项确认）

- CD1 **拖拽缩放**：方案 A，新增依赖 `vue-draggable-resizable`（ai-blueking 同款，最忠实；核心交互）。
- CD2 **挂载范围**：本迭代**只接入 CVM 服务申领页** `front/src/views/service/service-apply/cvm/index.tsx`（路由 `/business/service/service-apply/cvm?bizs=`，自研云账号用 `front/src/plugin-handler/bcc/service-apply-cvm.ts` 的 `ApplicationForm`）；跑通后再推广。
- CD3 **图标（用项目自有图标，不搬运 ai-blueking 字体）**（类名已核对存在于 `front/src/assets/iconfont/style.css`）：
  - 新建会话：`hcm-icon bkhcm-icon-chat-plus`
  - 历史会话：`hcm-icon bkhcm-icon-lishijilu`
  - 缩小高度：`hcm-icon bkhcm-icon-zoomout`
  - 恢复默认尺寸：`hcm-icon bkhcm-icon-fullscreen`
  - 关闭：bkui-vue 图标 `<close-line />`
  - 悬浮球头像：复制 ai-blueking `.../assets/images/avatar.png` → `front/src/assets/image/`（单数目录），组件内引用。
- CD4 **联动目标页**：CVM 服务申领页（同 CD2）。
- CD5 **源侧「添加到配置清单」按钮**：本迭代**只做目标侧自动唤起 + 加载会话**；源侧按钮触发通道待调查 agent 渲染内容后另接。
- CD6 **复刻保真度**：悬浮球交互（hover 的 `Cmd+I` 提示 + 最小化锚点 minus、点击/拖拽区分、锚定半隐、快捷键）与窗口背景色/渐变/阴影等视觉细节，**完全复刻 ai-blueking 源码**（详见 §5.1/§5.2）；仅图标(CD3)、tooltip/下拉(bkui-vue) 为等效替换。

## 10. 实施分期（建议）

1. **P1 壳层**：`nimbus` + `draggable-container` + `header` + `index.vue`（含状态机、项目自有图标、头像 png 复制），对话区先占位。
2. **P2 对话区抽取**：抽 `chat-panel.vue`，浮窗接入 `useChatbot`；现网全页改用 `chat-panel`。
3. **P3 联动目标侧**：CVM 服务申领页挂载 `AiAssistant` + query `sessionCode` 自动唤起 + 加载会话。
4. 每组改动后跑 `hcmfe_lint --fix`。

> CD5：源侧「添加到配置清单」按钮本迭代不做。
