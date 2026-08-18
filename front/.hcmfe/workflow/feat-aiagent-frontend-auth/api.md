# API：【aiagent-权限点调整】前端鉴权逻辑调整

> 本迭代**不新增、不变更** REST 接口。入口显隐与直访申请沿用现网鉴权与申请 URL 接口，只改前端读取的权限点。

## 1. 范围

| 项 | 说明 |
|----|------|
| 新增 REST | 无 |
| 变更 REST | 无 |
| 新增请求 | 无（PRD AC-P01：不因本需求多拉一次权限） |
| 后端会话鉴权 | 不在本单（弱依赖后端子需求 `1069995598137216850`） |

## 2. 现网鉴权：`POST /api/v1/web/auth/verify`

启动时已批量校验 `commonStore.pageAuthData`，结果写入 `authVerifyData.permissionAction`。本单**继续消费这次结果**，不新增 verify 调用。

| 产品权限 | 请求项 | `permissionAction` / 申请页 key | 本单用法 |
|----------|--------|--------------------------------|----------|
| 平台-智能体助手 | `{ type: 'agent_assistant', action: 'agent_assistant', id: 'agent_assistant', path: /^\/business\/chatbot/ }` | `agent_assistant`（与 IAM Action ID 一致） | **入口能力门**：菜单 / chatbot 页 / 落地 / 浮窗 / 403 申请页 |
| 业务-智能体助手 | 现网 `biz_agent_assistant` | `biz_agent_assistant` | **入口不再读取、不再预鉴权** |
| 业务访问 | 现网业务访问项 | `biz_access` | 无业务访问时仍先申请业务访问 |

现网曾用本地别名 `id: 'chatbot_access'` 映射同一 IAM Action；本单改为 `id: 'agent_assistant'`，与 `urlParams`、申请页 `403/:id` 对齐。

### 2.1 前端消费契约

- `permissionAction.agent_assistant === true`：视为有「平台-智能体助手」
- `permissionAction.biz_access === true`：视为有业务访问
- 菜单 / 浮窗 / 落地 / chatbot 页能力门：**只认** `agent_assistant`，不认 `biz_agent_assistant` / `chatbot_access`
- 缺 key 或 `false`：按无该权限处理

## 3. 现网申请 URL：`POST /api/v1/web/auth/find/apply_perm_url`

直访无权限时复用现网通用申请页。`403` 的 `params.id` 用 **`agent_assistant`**（IAM Action ID，与 `urlParams` 的 key 一致），不再用 `chatbot_access` 或 `biz_agent_assistant`。

无 `biz_access` 时，先走现网业务访问申请；补齐后再按 `agent_assistant` 决定进入页面或申请平台权限。

请求/响应字段沿用现网，本单不改协议。

## 4. 错误码

| 场景 | 行为 |
|------|------|
| verify 失败 / 无结果 | 按无权限：隐藏菜单与浮窗；直访走通用申请页 |
| 会话接口 403（后端子需求未齐） | 本单不改会话接口；入口可能可见但对话被拒，属已知依赖，不在本单处理 |

## 5. 与 PRD 映射

| PRD | API 结论 |
|-----|----------|
| F-001~F-004 能力门改平台权限 | 读 `permissionAction.agent_assistant` |
| F-005 / F-006 业务访问优先 | 读 `permissionAction.biz_access`，申请顺序沿用现网 |
| F-007 / AC-P01 不新增拉取 | 不新增 verify 调用 |
| 不做旧权限兼容 | 入口不读 `biz_agent_assistant` |
