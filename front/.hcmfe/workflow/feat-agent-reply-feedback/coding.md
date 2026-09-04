# Coding — feat-agent-reply-feedback

选型真相：`design.md` §3.x / §3.y / §8。落码入口 = **无**（adjacent），跟 `chat-message-list.vue` + chat-x，不走 page-* / comp-*。接口契约见同目录 `api.md`（MR `https://<GIT_HOST>/bcc/hcm/-/merge_requests/3376`）。

## 执行顺序

1. 打开现网已隐藏的 👍 / 👎，接 chat-x 原因面板与提交回调
2. 接入三个业务接口：标签配置、提交覆盖、再点取消删除
3. 实时流把 AG-UI `runId` 打到助手消息；历史 SNAPSHOT 用新增的 RUN_STARTED activity 恢复每轮 `run_id`

## 共享改动 / 提交策略

- 只改 chatbot 共用消息列表与会话内核；全页与浮窗一起生效
- 不改后端、不改 `docs/api-docs/**`

---

## 单据 1: 用户主动反馈-前端实现

**TAPD**: [#1069995598137584175](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137584175)

**文件**: `src/components/chatbot/chat-message-list.vue`、`src/hooks/chatbot/use-agent-feedback.ts`、`src/hooks/chatbot/agent-feedback.ts`、`src/store/chatbot/feedback.ts`、`src/hooks/chatbot/use-event.ts`、`src/hooks/chatbot/use-stream.ts`、`src/hooks/chatbot/types.ts`、`src/hooks/chatbot/use-session.ts`

**改动点**:

- `updateTools` 去掉 `like` / `unlike` 的 `hidden: true`（保留 `delete` 隐藏）
- `GET .../config/list?config_type=agent_feedback_tag`：chips 用 map 的中文 value；提交 `tags` 反查 key。配置空则只出补充说明
- `POST .../feedback/commit`：`onAgentFeedback` 上报 `session_id`（sessions/list 的 `id`，不是 `session_code`）、`run_id`、`reaction`（`unlike`→`dislike`）、tags、comment（截断 500 字）
- `DELETE .../feedback/{run_id}?session_id=`：再点已选中 👍/👎。chat-x 在 Tippy `onShow` 里取消激活且不发 `onAgentFeedback`，也不保证走 `onAgentAction`；改为在列表捕获阶段识别 `.ai-tool-btn.is-active` 后删除
- 实时 `/agui`：`RUN_STARTED.runId` 写入消息 `__runId`
- 历史 `/history`：SNAPSHOT 内 `activityType=RUN_STARTED|RUN_FINISHED` 的 `content.runId` 标出每轮边界（不渲染这些 activity），打到该轮可见消息。assistant 消息根上没有 `runId`
- 不手写 👍/👎 按钮或原因面板

### 与 PRD / Design 的口径差

- 点踩四条不再写死 TAPD 文案，改由标签配置下发（用户指定 MR 三个接口）
- chat-x 在面板 **提交后** 才激活按钮；因此 commit 走面板提交，不在首次点击瞬间空提交

### 需后端配合 / 组件限制

- chat-x `MessageTools` 无初始选中 prop，刷新后实心图标不能靠 `feedback/list` 还原；本迭代未接 list 做 UI 恢复
- commit 失败时 chat-x 仍会把按钮置为激活（回调在激活之后，且类型为 sync）

### 明确不做

- 项目 iconfont、新页面、3 秒撤销、改后端、运营看板
