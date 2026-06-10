# API：aiagent · chatbot 聊天窗模式

> 本迭代为 shell_only：**不新增、不修改任何后端接口**。聊天窗复用现网 `views/chatbot` 已有的会话 REST + SSE，框内能力（输入/上传/消息渲染）直接走 `@blueking/chat-x`，与全页一致。
> 本文档作用：固化「复用的现网接口契约」与「跨形态联动的前端参数契约」，作为 coding 依据。

## 1. 复用的现网后端接口（不改动）

来源：`src/store/chatbot/session.ts`（REST）、`src/store/chatbot/agent.ts`（SSE）。

### 1.1 会话 REST（业务维度，axios `@/http`）

前缀：`/api/v1/agent/{resolveBizApiPath(bkBizId)}sessions`（业务视角带 `bizs/{bk_biz_id}/`）。

| 方法 | 路径 | 用途 | 请求体 | 响应 |
|------|------|------|--------|------|
| POST | `{prefix}/list` | 会话列表 | `{ filter:{op:'and',rules:[]}, page:{count:false,start,limit,sort:'updated_at',order:'DESC'} }` | `{ data: SessionApiItem[] }` |
| POST | `{prefix}/create` | 新建会话 | `{ session_name, session_tag? }` | `{ data: { id, session_code, thread_id, session_name, session_tag?, bk_biz_id? } }` |
| PATCH | `{prefix}/{sessionCode}` | 重命名 | `{ session_name }` | — |
| DELETE | `{prefix}/{sessionCode}` | 删除 | — | — |

`SessionApiItem`（snake_case）：`id / session_code / session_name / thread_id / is_temporary / session_content_count / session_tag? / bk_biz_id? / created_at / updated_at`。

### 1.2 对话 / 历史 / 取消（SSE，原生 fetch）

平台级路径（非业务前缀），`credentials: 'include'`，头部 `getCommonHeaders()`。

| 方法 | 路径 | 用途 | 请求体 | 响应 |
|------|------|------|--------|------|
| POST | `/api/v1/agent/agui` | 主对话流 | `{ sessionCode, messages:[{role,content}] }` | SSE（AG-UI 事件，`data: {type,...}`） |
| POST | `/api/v1/agent/history` | 历史回放流 | `{ sessionCode }` | SSE（`MESSAGES_SNAPSHOT` + 续播事件） |
| POST | `/api/v1/agent/cancel` | 停止生成 | `{ sessionCode }` | 200/404/400 |

SSE 解析与事件语义沿用现网 `hooks/chatbot/use-stream.ts` / `use-event.ts`（AG-UI：`TEXT_MESSAGE_*`/`TOOL_CALL_*`/`RUN_*`/`CUSTOM:hitl.interrupt` 等）。

> 说明：`agent.ts` 现网用 `BK_HCM_AJAX_URL_PREFIX` 拼接 SSE 路径，属既有实现。本迭代**复用现网会话/对话逻辑，不在本需求内改造该实现**（如需整改另立任务，避免越界）。

## 2. 跨形态联动的前端参数契约（核心，无新接口）

联动本身**不依赖新后端接口**，靠「全页 → 主机申领页」携带会话标识 + 目标页用现网 `history` 接口加载会话实现。

### 2.1 触发与传参

- 触发：全页对话方案卡「添加到配置清单」。
- 跳转：用 `routerAction.redirect` 跳到主机申领页（路由 name 用 Symbol，见 coding）。
- 传参（建议 query）：

| 参数 | 含义 | 必填 | 说明 |
|------|------|------|------|
| `sessionCode` | 来源会话 code | 是 | 目标页据此自动打开聊天窗并加载该会话 |
| `autoOpenChat` | 是否自动唤起聊天窗 | 否（默认按 sessionCode 存在即开） | 防御性开关，便于复用 |

> 具体 query key 命名在 coding 阶段与主机申领页落地时最终确认；保持与 HCM 现有路由传参风格一致。

### 2.2 目标页加载流程

1. 主机申领页挂载聊天窗组件（页面级实例，D1）。
2. 读取 `sessionCode` → 自动唤起面板。
3. 用现网 `POST /api/v1/agent/history` 加载该会话历史，进入续播/可继续状态。
4. 异常降级（D2）：`sessionCode` 不存在/无权限 → 唤起空面板/新会话并提示。

### 2.3 会话共享前提

- 聊天窗与全页**共享同一会话空间**（同一 `bizs/{bk_biz_id}/sessions` + 同一 `agui/history`）。
- 因此全页创建的会话，聊天窗用同一 `sessionCode` 即可加载，无需额外同步接口。

## 3. 权限

- 复用现网 chatbot 权限（业务视角 `biz_agent_assistant` / `AUTH_ACCESS_BIZ` 体系）。
- 本迭代不新增后端鉴权点；前端按现网方式处理。

## 4. 本迭代 API 结论

- 后端：**零新增、零修改**。
- 前端：仅新增「联动跳转携带 `sessionCode` + 目标页自动加载」的**前端约定**，全部建立在现网接口之上。
