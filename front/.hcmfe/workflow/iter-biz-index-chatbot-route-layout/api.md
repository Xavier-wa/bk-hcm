# API：业务视角首页 chatbot - 路由与布局

## 1. 结论

**本子需求不新增、不修改后端 HTTP 接口。** 页面迁移至 `/business/index` 后，继续调用现有 Agent / Session API；权限仍走 IAM `agent_assistant`（前端 `chatbot_access`）。

---

## 2. 复用接口清单

实现位置：`front/src/store/chatbot/`（与现网 `/chatbot` 页相同）。

### 2.1 会话 CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agent/sessions/list` | 会话列表；body：`filter` + `page`（MySQL 风格） |
| POST | `/api/v1/agent/sessions/create` | 创建会话；body：`session_name` |
| PATCH | `/api/v1/agent/sessions/{sessionCode}` | 重命名；body：`session_name` |
| DELETE | `/api/v1/agent/sessions/{sessionCode}` | 删除会话 |

**列表响应字段（节选）**：`session_code`, `session_name`, `session_content_count`, `created_at`, `updated_at`。

### 2.2 对话流（SSE）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST (fetch SSE) | `/api/v1/agent/agui` | 发送消息 / 流式回复；body：`sessionCode`, `messages` |
| POST (fetch SSE) | `/api/v1/agent/history` | 拉取会话历史；body：`sessionCode` |
| POST | `/api/v1/agent/cancel` | 取消生成；body：`sessionCode` |

协议：AG-UI over SSE（见 `hooks/chatbot/`、`views/index/chatbot/AI_SPEC.md`）。

---

## 3. 权限（非 HTTP 新接口）

| 项 | 说明 |
|----|------|
| IAM 类型 | `agent_assistant` |
| 前端 pageAuth id | `chatbot_access`（`store/common.ts` `pageAuthData`） |
| 本子需求变更 | `path` 从 `/chatbot` 调整为匹配 `/business/index`（或等价规则）；控制顶栏「资源管理」与路由 `meta.auth.view`（coding 阶段落地） |

视图无权限：走路由守卫 + `views/status/permission.vue`（或业务视角等价页），**无单独 Agent API**。

---

## 4. 业务 ID（`bizs`）与接口演进

### 4.1 当前：页面 query，Agent API 不变

| 层级 | 约定 |
|------|------|
| **前端全局业务 ID** | URL query 参数名 **`bizs`**（常量 `GLOBAL_BIZS_KEY = 'bizs'`），与业务视角其它页一致 |
| **本子需求 Agent 调用** | **保持现网路径与 body 不变**（`/api/v1/agent/...`），**不**向 Session/AG-UI 接口追加 `bizs` 或按业务拆请求 |
| **原因** | 后端尚未提供按业务 ID 分隔会话/对话的能力；避免影响当前展示与联调 |

切换顶栏业务选择器时，仅更新路由 query `?bizs={id}`；会话列表与对话数据仍为用户维度全局列表（与迁移前 `/chatbot` 一致）。

### 4.2 产品目标 vs 本迭代（会话隔离）

| 阶段 | 行为 |
|------|------|
| **PRD 长期目标** | 会话按业务隔离 |
| **本子需求（路由与布局）** | **不**因 `bizs` 切换而改 Agent API；不在前端伪造按 biz 分桶的会话数据（避免与后端不一致） |
| **后续子需求 / 后端就绪后** | 见 §4.3 |

### 4.3 后续：路径参数 `/bizs/{bizId}/`（待后端更新）

后端支持按业务隔离后，Agent 相关接口应改为与其它业务 API 一致的路径前缀（项目内已有 `resolveApiPathByBusinessId` 模式）：

```
/api/v1/agent/bizs/{bizId}/sessions/list
/api/v1/agent/bizs/{bizId}/sessions/create
/api/v1/agent/bizs/{bizId}/agui
/api/v1/agent/bizs/{bizId}/history
...
```

（具体资源路径以后端契约为准；原则是 **`/bizs/{bizId}/` 出现在 API path**，而非仅依赖 query `bizs`。）

**coding 预留**：`store/chatbot/*` 集中封装 base path，本迭代写死无前缀；后续仅改封装层即可切换至 `/bizs/{bizId}/`。

### 4.4 前端路由参数

| 前端路由 | 说明 |
|----------|------|
| `/business/index/:sessionCode?` | `sessionCode` 仍仅用于页面 deep link |
| `?bizs=` | 系统业务 ID（query），与 Agent API **解耦**（本迭代） |

---

## 5. 本子需求 API 验收

- [ ] 无新增 `/api/v1/**` 路径；Agent 请求 **未** 携带 `/bizs/{id}/` 前缀（本迭代）
- [ ] `/business/index?bizs=...` 页可调通 `sessions/*`、`agui`、`history`、`cancel`（与迁移前 `/chatbot` 行为一致）
- [ ] 切换 query `bizs` 不改变 Agent 请求 URL（仅页面/布局上下文变化）
- [ ] 权限失败仍为 HTTP 403 + 平台权限弹窗/申请页，不依赖新错误码

---

## 6. 参考

- 前端实现说明：`front/src/views/index/chatbot/AI_SPEC.md`
- 后端文档（如有）：`<DOC_HOST>` 下 ai-blueking / agent chatbot 章节
