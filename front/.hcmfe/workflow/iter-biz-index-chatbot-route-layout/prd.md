# PRD：aiagent - 资源管理首页 chatbot - 路由与布局

## 0. 需求层级与关联

### 0.1 父需求（Requirement，跨子需求聚合）

| 项 | 内容 |
|----|------|
| **标题** | aiagent - 资源管理首页 chatbot |
| **Requirement ID** | `req-aiagent-biz-index-chatbot` |
| **TAPD** | [父 story](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598134643064)（`1069995598134643064`） |
| **职责** | 聚合 PRD/Design/API 主版本；承载多个 **子需求迭代（Workflow）**；本子需求完成后可 `merge` 入主版本，再开启下一子需求 |

### 0.2 本子需求迭代（Workflow）

| 项 | 内容 |
|----|------|
| **标题** | aiagent - 资源管理首页 chatbot - 路由与布局 |
| **Workflow ID** | `iter-biz-index-chatbot-route-layout` |
| **TAPD** | [子 story](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598134649293)（`1069995598134649293`） |
| **产物目录** | `.hcmfe/workflow/iter-biz-index-chatbot-route-layout/` |

### 0.3 后续子需求（规划说明，非本迭代范围）

同一父需求下，完成本 Workflow（`done`）并 merge 后，可再 `hcmfe_workflow_init` 新建下一子需求 Workflow（独立 ID、独立产物目录），例如能力增强、其它 AI 入口等，均挂接 `req-aiagent-biz-index-chatbot`。

**术语约定**：产品文案「资源管理」= **业务视角（biz）**；实现与路由使用 `business` / `biz` 语义，不与资源运营 `/resource` 混淆。

## 1. 需求背景

父需求计划在业务视角提供带 AI 助手的首页。当前平台将独立「首页」入口指向 `/chatbot`，与目标不一致。

**本子需求目标（范围边界）**：仅完成 **路由与页面布局** 的实现——顶栏入口调整、权限入口、`/business/index` 路由注册、页面壳层与 Figma 布局对齐（含会话侧栏展开/收起两种态）。对话能力、会话 API、HITL 等 **复用已有实现嵌入布局**，不在本子需求内新增业务能力。

## 2. 用户故事

- 作为有 AI 使用权限的用户，我希望通过顶栏 **「资源管理」** 进入 `/business/index`，在符合设计稿的布局中使用 AI 对话与会话管理。
- 作为无 AI 使用权限的用户，我不应看到「资源管理」顶栏入口；若直接访问 `/business/index`，应看到 **权限申请页**。
- 作为有权限用户，我可通过 `/business/index/:sessionCode` 打开指定会话；切换 query `bizs` 后页面上下文切换（本迭代会话列表仍为全局，待后端 `/bizs/{bizId}/` 接口后再按业务分隔）。

## 3. 功能范围

### 3.1 路由与入口（本子需求必须）

| 项 | 要求 |
|----|------|
| **落地路径** | `/business/index`；会话 deep link：**`/business/index/:sessionCode`**（路径参数） |
| **废弃 `/chatbot`** | 不保留 `/chatbot` 产品入口与路由注册；**不做** 旧链接重定向或兼容 |
| **顶栏** | **仅保留**「资源管理」→ `/business/index`；移除原「首页」（原 `/chatbot`） |
| **命名** | 顶栏文案「资源管理」；技术命名倾向 **biz** |

### 3.2 页面布局（本子需求必须）

依据 Figma「业务资源」：

- [主布局（会话侧栏展开）](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/%E4%B8%9A%E5%8A%A1%E8%B5%84%E6%BA%90?node-id=2006-6056)（node `2006-6056`）
- [会话侧栏收起态](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/%E4%B8%9A%E5%8A%A1%E8%B5%84%E6%BA%90?node-id=2031-829)（node `2031-829`）

布局层级（由外到内）：

1. **全局顶栏**：与其它业务页一致
2. **左侧业务菜单**：与 `/business/host` 等页面相同，**始终展示**
3. **内容区（chatbot 双栏）**：会话侧栏（含展开/收起两态）+ 主对话区

本子需求验收以 **结构、尺寸、展开/收起态** 与 Figma 一致为准；对话/HITL 等 **复用既有实现嵌入**，不新增业务能力。

### 3.3 权限（本子需求必须）

| 项 | 要求 |
|----|------|
| **标识** | IAM `agent_assistant` / 前端 `chatbot_access`（重命名可选，行为优先） |
| **范围** | `chatbot_access` 作为 **全 AI 场景入口级** 控制（本迭代至少：顶栏「资源管理」+ `/business/index`） |
| **无权限** | 隐藏顶栏入口；直访 `/business/index` → **权限申请页** |
| **有权限** | 正常使用布局内已上线交互 |

### 3.4 不在本子需求范围

- `/chatbot` 旧链重定向或兼容
- 新 Agent 能力、后端接口变更（除非路由/权限必需）
- `/resource`、`/service` 下 AI 入口实现（权限语义可预留）
- 对话协议 / HITL / 流式消息 **新功能**（仅嵌入既有组件）
- 隐藏左侧业务菜单、全屏独占 chatbot（与 Figma 不符）
- 非布局类体验优化（消息锚点、置顶策略变更等，除非为布局占位所必需）

## 4. 业务规则

1. **单一路径**：业务视角 AI 首页仅 `/business/index`（及可选 `:sessionCode`），无 `/chatbot`。
2. **权限与入口一致**：AI 相关顶栏/菜单入口统一受 `chatbot_access` 控制。
3. **业务 ID（`bizs` query）**：路由携带 query `bizs`（`GLOBAL_BIZS_KEY`）。**本子需求** Agent 接口保持现网不变（后端尚未按业务分隔）；**后续**后端支持后，API 改为路径前缀 `/bizs/{bizId}/`（见 `api.md` §4.3）。
4. **无旧链兼容**：不处理 `/chatbot` 访问；由用户更新书签或接受 404（与移除路由行为一致）。

## 5. 验收标准

### P0

- [ ] 有 `chatbot_access`：顶栏仅 **「资源管理」** → `/business/index`；页面布局与 Figma node `2006-6056` / `2031-829` 一致（含左侧业务菜单常显、会话侧栏展开/收起）。
- [ ] 无 `chatbot_access`：无顶栏「资源管理」；直访 `/business/index` → **权限申请页**。
- [ ] 无「首页」、无 `/chatbot` 顶栏与路由。
- [ ] 路由 `/business/index`、`/business/index/:sessionCode` 可访问并挂载布局壳层。
- [ ] 既有 chatbot 组件嵌入后主对话区可展示（不要求本子需求新增对话能力）。

### P1

- [ ] `/business/index` 下顶栏高亮、侧栏/布局行为正确，无双首页入口。
- [ ] 切换 query `bizs` 后布局与路由正常；本迭代会话 API 行为与切换前一致（见 `api.md` §4.1）。

### P2

- [ ] 文档说明后续子需求可复用 `chatbot_access`（本迭代可不实现其它入口）。

## 6. 设计参考

- Figma 主布局（会话侧栏展开）：[node 2006-6056](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/%E4%B8%9A%E5%8A%A1%E8%B5%84%E6%BA%90?node-id=2006-6056)
- Figma 会话侧栏收起：[node 2031-829](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/%E4%B8%9A%E5%8A%A1%E8%B5%84%E6%BA%90?node-id=2031-829)

## 7. 已确认的产品约束

| 约束 | 内容 |
|------|------|
| 父/子结构 | Requirement = 父 TAPD；Workflow = 子 story「路由与布局」 |
| Workflow ID | `iter-biz-index-chatbot-route-layout` |
| 落地路径 | `/business/index`，不保留 `/chatbot`，**不做旧链兼容** |
| 顶栏 | 仅「资源管理」→ `/business/index` |
| 会话 deep link | `/business/index/:sessionCode` |
| 业务 query | `bizs`；本迭代 Agent API **不变**；后续 `/bizs/{bizId}/` 路径（见 api.md） |
| 无权限直访 | **权限申请页** |
| 权限 | `chatbot_access` → 全 AI 场景入口控制（本迭代覆盖顶栏 + 业务首页） |
