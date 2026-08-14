# API：业务视角首页 chatbot - 侧边栏会话列表

> **状态：已对齐后端现状 + MR !3129 方向** — 参考 [MR !3129](https://<GIT_HOST>/bcc/hcm/-/merge_requests/3129) 与 `docs/api-docs/web-server/docs`。  
> **路径拼装**：统一使用 `@/utils/search` 的 **`resolveBizApiPath`**（与 woa / 业务页一致）。  
> **本子需求扩展**：列表项 **`session_tag`**（标签文件夹分组）。

---

## 1. 结论摘要

| 能力 | 约定 |
|------|------|
| **列表 / 创建** | **双维度**：平台与业务路径均已提供；业务 chatbot 在有效 `bizs` 下走 **业务维度** |
| **重命名 / 删除** | **仅业务维度**（必须带 `bizs/{bk_biz_id}/`）；业务 chatbot **禁止**走平台 `sessions/{session_code}` |
| **业务 ID** | 路由 query **`bizs`**（`GLOBAL_BIZS_KEY`）→ `resolveBizApiPath(bizId)` |
| **会话标签** | 列表项 **`session_tag`**（空/缺失 = 未分组） |
| **搜索 / 置顶** | 纯前端 |
| **对话流 SSE** | 暂不变：`/api/v1/agent/agui`、`history`、`cancel` |
| **拖拽** | 下一迭代 |

实现位置：`front/src/store/chatbot/session.ts`、`front/src/hooks/chatbot/use-session.ts`。

---

## 2. 路径拼装：`resolveBizApiPath`

```246:246:hcm/front/src/utils/search.ts
export const resolveBizApiPath = (bizId?: number) => (bizId ? `bizs/${bizId}/` : '');
```

| `bizId` | 片段 | 拼入 Agent 前缀后 |
|---------|------|-------------------|
| 有值（如 `123`） | `bizs/123/` | `/api/v1/agent/bizs/123/sessions/list` |
| 无值 | `''`（空） | `/api/v1/agent/sessions/list`（**平台维度**） |

业务视角 chatbot 页应从 `route.query.bizs` 取 `bk_biz_id`，**有 biz 时 list/create/update/delete 均走业务维度**；无有效 `bizs` 时不应发起会话 CRUD。

**封装示例**（coding 参考 `store/ticket/res-sub-ticket.ts` 等写法）：

```typescript
import { resolveBizApiPath } from '@/utils/search';

const AGENT_API = '/api/v1/agent';

// 业务维度 list（bizs 有效时）
const listUrl = `${AGENT_API}/${resolveBizApiPath(bkBizId)}sessions/list`;

// 业务维度 update / delete（仅此维度）
const updateUrl = `${AGENT_API}/${resolveBizApiPath(bkBizId)}sessions/${sessionCode}`;
const deleteUrl = `${AGENT_API}/${resolveBizApiPath(bkBizId)}sessions/${sessionCode}`;
```

---

## 3. 会话接口：双维度现状

### 3.1 维度说明

| 维度 | 路径前缀 | 权限（文档） |
|------|----------|--------------|
| **平台** | `/api/v1/agent/sessions/...` | 平台-智能体助手 |
| **业务** | `/api/v1/agent/bizs/{bk_biz_id}/sessions/...` | 业务-智能体助手 |

**维度能力一览：**

- **`list` / `create`**：平台、业务**两套路径均有**。
- **`update` / `delete`**：**仅业务维度**（路径含 `bizs/{bk_biz_id}/`），无平台等价接口。

| 操作 | 平台维度 | 业务维度 | 业务 chatbot 选用 |
|------|----------|----------|-------------------|
| **list** | `POST /api/v1/agent/sessions/list` | `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list` | **业务**（有 `bizs`） |
| **create** | `POST /api/v1/agent/sessions/create` | `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create` | **业务**（有 `bizs`） |
| **update** | 无 | `PATCH /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}` | **业务**（有 `bizs`） |
| **delete** | 无 | `DELETE /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}` | **业务**（有 `bizs`） |

官方文档（MR !3129）：

| 双维度 list/create | 仅业务 update/delete |
|--------------------|----------------------|
| `service/agent/list_session.md`、`biz/agent/list_session.md` | `biz/agent/update_session.md` |
| `service/agent/create_session.md`、`biz/agent/create_session.md` | `biz/agent/delete_session.md` |

---

### 3.2 列表 `POST .../sessions/list`（业务维度）

**URL**（有 `bizs`）：

```http
POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list
```

**请求**：

```json
{
  "filter": { "op": "and", "rules": [] },
  "page": {
    "count": false,
    "start": 0,
    "limit": 100,
    "sort": "updated_at",
    "order": "DESC"
  }
}
```

**行为**：

- 过滤：**当前用户 + 当前 `bk_biz_id`**。
- `session_tag` 分组在**前端**完成，不在服务端 filter。

**响应 `details[]`（本子需求字段）**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `session_code` | string | 切换 / URL |
| `session_name` | string | 展示名；搜索匹配 |
| `session_content_count` | number | |
| `bk_biz_id` | int64 | 与路径一致 |
| `created_at` / `updated_at` | string | 排序用 `updated_at` |
| **`session_tag`** | string | 文件夹名；空 → 未分组 |

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "details": [
      {
        "session_code": "e3f4a2b1c9d8e7f6a5b4c3d2-2026032009",
        "session_name": "业务申领对话",
        "bk_biz_id": 123,
        "session_tag": "主机申领",
        "updated_at": "2026-03-20T10:30:00Z"
      }
    ]
  }
}
```

**平台维度 list**（无 `bizs` 或调试平台页时）：`POST /api/v1/agent/sessions/list`，不按 `bk_biz_id` 隔离；**业务 chatbot 默认不用**。

---

### 3.3 创建 `POST .../sessions/create`（业务维度）

**URL**（有 `bizs`）：

```http
POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create
```

**请求**：`{ "session_name": "新对话" }`

**响应 `data`**：`session_code`、`session_name`、`thread_id`、`bk_biz_id`；`session_tag` 若有则可在 create 或后续 list 带出。

**平台维度 create**：`POST /api/v1/agent/sessions/create`，`bk_biz_id` 固定 **-1**（未分配业务）；业务 chatbot **不用**。

---

### 3.4 重命名 `PATCH`（仅业务维度）

```http
PATCH /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}
```

```json
{ "session_name": "修改后的名称" }
```

响应 `data: null`。路径 **`bk_biz_id` 必填**，与当前页 `bizs` 一致。

---

### 3.5 删除 `DELETE`（仅业务维度）

```http
DELETE /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}
```

无 body；响应 `data: null`。删除当前会话后的回退逻辑沿用现网。

> 现网 `session.ts` 的 `PATCH/DELETE /api/v1/agent/sessions/{code}` 为**平台旧路径**，业务 chatbot coding 时需改为上表业务路径。

---

## 4. 对话流（本迭代不变）

| 方法 | 路径 |
|------|------|
| POST (SSE) | `/api/v1/agent/agui` |
| POST (SSE) | `/api/v1/agent/history` |
| POST | `/api/v1/agent/cancel` |

---

## 5. 前端数据模型

### 5.1 映射

| API | `ChatSession` |
|-----|---------------|
| `session_code` | `sessionCode` |
| `session_name` | `sessionName` |
| `session_content_count` | `sessionContentCount` |
| `created_at` / `updated_at` | `createdAt` / `updatedAt` |
| `session_tag` | `sessionTag` |

```typescript
export interface ChatSession {
  sessionCode: string;
  sessionName: string;
  sessionContentCount: number;
  createdAt: string;
  updatedAt: string;
  sessionTag?: string;
  messages: Message[];
}
```

### 5.2 `session.ts` 调用约定

| 方法 | URL 拼装 |
|------|----------|
| `listSessions(bkBizId)` | `` `/api/v1/agent/${resolveBizApiPath(bkBizId)}sessions/list` `` |
| `createSession(name, bkBizId)` | `` `.../${resolveBizApiPath(bkBizId)}sessions/create` `` |
| `updateSession(code, name, bkBizId)` | `` `/api/v1/agent/${resolveBizApiPath(bkBizId)}sessions/${code}` `` |
| `deleteSession(code, bkBizId)` | `` `/api/v1/agent/${resolveBizApiPath(bkBizId)}sessions/${code}` `` |

### 5.3 侧栏分组与搜索

1. 置顶：`localStorage` + list 结果。
2. 历史：`sessionTag` 空 → 未分组在上；非空 → 标签文件夹；组内 / 文件夹间按 `updatedAt`。
3. 搜索：本地过滤 `sessionName`。

---

## 6. 本迭代不涉及

服务端搜索、改 `session_tag`、文件夹 CRUD、拖拽。

---

## 7. PRD / Design 映射

| 需求 | 实现 |
|------|------|
| 业务隔离 list/create/update/delete | `resolveBizApiPath` + `bizs` |
| 重命名 / 删除 | 业务 `bizs/{id}/sessions/{code}` |
| 标签分组 | `session_tag` |
| 搜索 | 前端过滤 `session_name` |

---

## 8. 变更记录

| 日期 | 说明 |
|------|------|
| 2026-06-03 | 对齐 MR !3129；`session_tag` |
| 2026-06-03 | `resolveBizApiPath`；list/create 双维度，update/delete 仅业务维度 |
