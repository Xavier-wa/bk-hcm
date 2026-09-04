# API — 用户主动反馈（前端消费契约）

来源：工蜂 MR `https://<GIT_HOST>/bcc/hcm/-/merge_requests/3376`（接口文档分支）。本迭代 api 阶段已跳过，契约在 coding 落地前补记于此。路径均相对 web-server，前缀 `/api/v1/agent`。

路径中的 `bk_biz_id` 做业务访问鉴权；反馈标签配置本身是全局的。`session_id` 使用 `GET /api/v1/agent/bizs/{bk_biz_id}/sessions/list` 的 `details[].id`（如 `000002p9`），**不是** `session_code`。

## 1. 拉取反馈标签

`GET /api/v1/agent/bizs/{bk_biz_id}/config/list?config_type=agent_feedback_tag`

- 权限：业务访问。
- `config_type` 本期仅允许 `agent_feedback_tag`。
- `data.details[]`：`config_key` 为 `like` | `dislike`；`config_value` 为英文 key → 中文文案的扁平 map。
- chips 展示 **value（中文）**；提交 `tags` 必须用对应 **key**。
- `details` 为空：不展示 chips，可只填补充说明。
- 错误：`2000001` 参数非法 / `2000006` 查询失败 / `2000012` 无业务访问权限。

## 2. 提交点赞 / 点踩

`POST /api/v1/agent/bizs/{bk_biz_id}/feedback/commit`

Body：

| 字段 | 必选 | 说明 |
|------|------|------|
| session_id | 是 | 当前用户、当前业务下的会话主键，取 sessions/list 的 `id`，不是 `session_code` |
| run_id | 是 | 本轮 AG-UI `runId` |
| reaction | 是 | `like` \| `dislike` |
| tags | 否 | 当前 reaction 对应标签 map 的 key，可空 |
| comment | 否 | 手输文本，上限 500 字（按字） |

- 按 `run_id` **覆盖更新**（upsert），赞/踩互斥；改判会清空旧 tags/comment。
- 点赞瞬间可不带 tags；chat-x 实际在原因面板提交时才上报（可带 tags/comment）。
- `tags` 跨套或未知 key → `2000001`，不会静默丢弃。
- 成功 `data.id` 为反馈表主键。

## 3. 取消点赞 / 点踩

`DELETE /api/v1/agent/bizs/{bk_biz_id}/feedback/{run_id}?session_id=`

- query 的 `session_id` 同样是 sessions/list 的 `id`，不是 `session_code`。

- 再点一次 chat-x **已选中** 的 👍/👎 时调用（组件内部取消激活，不触发 `onAgentFeedback`）。
- 只能删自己的、且属于当前业务/会话的反馈。
- 错误：`2000001` / `2000003` 会话或反馈不存在 / `2000006` / `2000012`。

## 4. 回查已提交反馈（同 MR，选中态恢复）

`GET /api/v1/agent/bizs/{bk_biz_id}/feedback/list?session_id=`

- query 的 `session_id` 同样是 sessions/list 的 `id`，不是 `session_code`。

- 只返回当前用户该会话下已评价的 run（最多 500 条）。
- 用于恢复按钮选中态。chat-x `MessageTools` **没有**初始选中 prop，刷新/切会话后实心图标无法还原；本迭代不调用此接口做 UI 恢复。

## 5. `run_id` 从哪来

- **`/agui`**：SSE 事件 `RUN_STARTED` 的 `runId`（`RUN_FINISHED` 会再带一次）。后续 `TEXT_MESSAGE_*` 通常不带；前端记住本轮 id 打到助手消息 `__runId`。
- **`/history`**：不要用外层那条历史请求自己的 `RUN_STARTED`。本期 SNAPSHOT 把每一轮边界写进 `messages[]` 的 activity（`history.md` 的 `messages[n]` 表尚未更新）：
  - `{ role: "activity", activityType: "RUN_STARTED", content: { runId, threadId } }`
  - `{ role: "activity", activityType: "RUN_FINISHED", content: { runId, threadId } }`
  - 中间的 assistant / 卡片 / tool 消息根上**没有** `runId`。前端解析 marker、不渲染，并把当前 `runId` 打到该轮可见消息上。

## 6. 前端映射

| 交互 | 接口 |
|------|------|
| 打开原因面板 | `onAgentAction` 返回 config 中文 values；异步拉 config |
| 面板提交 | `commit`：`session_id` = sessions/list `id`；`unlike` → `dislike`；labels → keys；`comment` 截断 500 字 |
| 再点已选中按钮 | `delete`（捕获阶段识别 chat-x 已激活的 👍/👎；组件取消激活不走 `onAgentFeedback`） |
| 实时流消息 | `/agui` 的 `RUN_STARTED.runId` 写入消息 `__runId` |
| 历史消息 | SNAPSHOT 中 `activityType=RUN_STARTED` 的 `content.runId` 打到该轮可见消息 |
