# API：aiagent - 资源管理首页 chatbot - 默认主内容区

> 本迭代为**前端展示 + 复用现有 agent 会话能力**。卡片为前端固定配置。
> **关键机制（用户确认）**：场景 tag **不在发送消息时携带**，而是在**创建会话时通过 `session_tag` 参数传递**；`session_tag` 在前端做 key→名称映射，**目前先支持 `host_apply` → 主机申领**。

## 1. 结论摘要

| 能力 | 约定 |
|------|------|
| **大卡片点击** | 复用 `sendMessage`：无会话时惰性创建会话（带场景的传 `session_tag`）→ 发送预设消息 → SSE 流式回复 |
| **小卡片点击** | 纯前端：向 `ChatInput` 注入「场景 tag + 默认提示词」；用户点发送后惰性创建会话（带场景的传 `session_tag`）再发送 |
| **场景标识** | 通过**创建会话**的 `session_tag` 入参传递（**非**发消息字段）；前端 key→名称映射，目前仅 `host_apply: 主机申领` |
| **会话分组联动** | 带 `session_tag` 创建的会话，列表返回同字段 → 自动归入对应**标签文件夹**（上一迭代能力） |
| **卡片数据来源** | 前端固定配置（常量），全业务统一（PRD Q5） |
| **后端接口改动** | `create_session` 已支持 **`session_tag` 入参**（后端已就绪）；其余复用，无新增 |

## 2. 复用的现有接口

| 操作 | 方法 / 路径 | 维度 | 本迭代变化 |
|------|------------|------|------------|
| 创建会话 | `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create` | 业务 | **请求体新增可选 `session_tag`**（见 §3.1） |
| 对话流 | `POST /api/v1/agent/agui`（SSE） | — | 不变（发送只发纯文本，不带场景字段） |
| 历史 | `POST /api/v1/agent/history`（SSE） | — | 不变 |
| 取消 | `POST /api/v1/agent/cancel` | — | 不变 |
| 列表 | `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list` | 业务 | 不变；响应 `session_tag` 用于文件夹分组（已实现） |
| 自动命名 | `PATCH /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}` | 业务 | 不变 |

> `bk_biz_id` 来自 `useWhereAmI().getBizsId()`。

## 3. 创建会话扩展：`session_tag` 入参

### 3.1 请求（业务维度）

```http
POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create
```

```json
{
  "session_name": "新对话",
  "session_tag": "host_apply"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `session_name` | string | 是 | 会话名（现有） |
| `session_tag` | string | 否 | **本迭代新增**：场景 key（如 `host_apply`）；不带场景的普通新对话不传/传空 |

- 响应沿用现有 `CreateSessionApiResponse`（已含可选 `session_tag` 回显）。
- 列表接口返回的 `session_tag` 由前端映射为文件夹名（见 §4），归入对应标签文件夹。

> ✅ 后端 `create_session` 已支持 `session_tag` 入参，前端直接传。

### 3.2 前端 `createSession` 需扩展

现有 `store/chatbot/session.ts` 的 `createSession(bkBizId, sessionName)` 只传 `session_name`，coding 阶段需扩展可选 `sessionTag` 参数并透传；上层 `use-session.ts` / `use-chatbot.ts` 的 `createSession` / `sendMessage` 需支持携带场景 tag。

## 4. 场景 `session_tag` key → 名称映射（前端固定）

| session_tag key | 文件夹 / 场景名 | 状态 |
|-----------------|----------------|------|
| `host_apply` | 主机申领 | ✅ 本迭代支持 |
| （其余场景） | — | ⬜ 暂无映射，后续按需补充 |

- 映射表前端固定维护（常量）；列表项 `session_tag` 命中映射 → 显示对应文件夹名；未命中 → 按原始值或未分组处理（沿用上一迭代逻辑）。

## 5. 前端固定配置（卡片清单 + 默认文案）

> 文案为**合理默认**（Q-A1 选 use_default），coding 阶段可调；清单/顺序/响应式见 design §2.2、§2.3。

### 5.1 大卡片（点击直接发送预设消息）

| # | 标题 | 描述 | 预设消息（默认，可调） | session_tag |
|---|------|------|------------------------|-------------|
| 1 | 申领 10 台主机 | 可通过智能推荐快捷申领主机 | 我要申领 10 台主机 | `host_apply` |
| 2 | GPU 库存情况 | 可查看 GPU 库存情况及使用率 | 我想查看 GPU 库存情况 | （无） |
| 3 | 如何提交回收申请 | 可通过智能引导快速回收资源 | 如何提交回收申请 | （无） |
| 4 | 申领 50 台服务器 | 可通过智能推荐快捷申领服务器 | 我要申领 50 台服务器 | `host_apply` |

> GPU 库存描述稿面被截断，默认补全为「…及使用率」，最终以产品/稿面为准（可 coding 微调）。

### 5.2 小卡片（点击注入「场景 tag + 默认提示词」）

| # | chip 文案 | 默认提示词（注入输入框，可调） | session_tag |
|---|-----------|--------------------------------|-------------|
| 1 | 主机申领 | 我要申请主机 | `host_apply` |
| 2 | 主机回收 | 我要回收主机 | （无） |
| 3 | 预测提单 | 我要提交预测单 | （无） |
| 4 | 预测调整 | 我要调整预测 | （无） |
| 5 | CLB申领 | 我要申领 CLB | （无） |
| 6 | CLB删除 | 我要删除 CLB | （无） |
| 7 | CLB批量导入 | 我要批量导入 CLB | （无） |
| 8 | 安全组创建 | 我要创建安全组 | （无） |
| 9 | 安全组规则管理 | 我要管理安全组规则 | （无） |
| 10 | 安全组绑定/解绑 | 我要绑定或解绑安全组 | （无） |

> 「（无）」表示当前无 `session_tag` 映射；该场景创建会话不带 tag，落入未分组。后续补充映射即可联动文件夹。

## 6. 前端数据模型

```typescript
interface BigCard {
  icon: string;          // 图标（iconfont 类名 / svg）
  title: string;
  desc: string;
  prompt: string;        // 点击直接发送的预设消息
  sessionTag?: string;   // 场景 key，创建会话时传 session_tag
}

interface PromptChip {
  tag: string;           // chip 文案（场景名）
  icon: string;
  prompt: string;        // 注入输入框的默认提示词
  sessionTag?: string;   // 场景 key，创建会话时传 session_tag
}

// session_tag key → 名称映射（与上一迭代文件夹分组共用）
const SESSION_TAG_NAME: Record<string, string> = { host_apply: '主机申领' };
```

> 大卡片 / 小卡片为数组，渲染响应式布局；数量由数组长度决定（design 响应式，不写死）。

## 7. 已确认结论

| 编号 | 结论 |
|------|------|
| **Q-A1** | ✅ 后端 `create_session` **已支持 `session_tag` 入参**，前端直接传。 |
| **Q-A2** | ✅ 本迭代**仅做 `host_apply: 主机申领`**，其余卡片不带 `session_tag`（落入未分组），后续按需补充映射。 |
| **Q-A3（文案）** | ✅ 卡片文案用合理默认（§5），coding 阶段可微调。 |

## 8. PRD / Design 映射

| 需求 | 实现 |
|------|------|
| 大卡片点击直发 | 复用 `sendMessage`（带场景传 `session_tag`）（§3、§5.1） |
| 小卡片注入 tag + 默认词待发送 | `ChatInput` v-model 注入 + 发送时按场景创建（§5.2） |
| 场景标识 | 创建会话 `session_tag` 入参 + 前端映射（§3、§4） |
| 卡片前端固定、全业务统一 | 配置常量（§5、§6） |
| 会话按场景分组 | `session_tag` 联动上一迭代文件夹（§1、§4） |
