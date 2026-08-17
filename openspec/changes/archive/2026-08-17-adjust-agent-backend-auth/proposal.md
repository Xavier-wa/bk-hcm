## Why

业务会话 CRUD 当前通过 `BizID>0` 把 `AgentAssistant` 映射成 IAM「业务-智能体助手」`biz_agent_assistant`，灰度必须按业务逐个授智能体权限，授权成本高。本期业务会话改为只鉴已有「业务访问」`biz_access`（接口只能操作当前用户自己的会话），并下线已无作用的业务智能体权限点。对话运行与平台会话接口仍走平台「智能体助手」`agent_assistant`。功能尚未上线，不做存量授权迁移。

## What Changes

- 业务会话 API `/api/v1/agent/bizs/{bk_biz_id}/sessions/*`（create/list/update/delete）只鉴对该 `bk_biz_id` 的 `biz_access`；不再校验平台 `agent_assistant`，也不再因 `BizID>0` 检查 `biz_agent_assistant`
- auth-server `genAgentAssistantResource` 对 `AgentAssistant` 一律映射平台 `agent_assistant`，删除 `BizID>0` → `biz_agent_assistant` 分支
- IAM 注册下线 `biz_agent_assistant`（types、initial_actions、action groups）；权限中心随 auth-server 注册变更发布
- `/agui` `/history` `/cancel` 维持现有平台 `agent_assistant` + 会话归属校验，**不**叠加业务访问
- 平台 `/sessions/*`、memory、skill 鉴权保持平台 `agent_assistant`，本期不改产品入口
- 无新 HTTP 路径、无请求体变更；**非 BREAKING**（功能未上线，旧权限点不再产生能力）
- 前端菜单/路由/浮窗不在本变更范围，须与前端子单同迭代发布

## Capabilities

### New Capabilities

（无。不引入新的能力面，只调整既有业务会话 API 的鉴权组合并下线旧 IAM Action。）

### Modified Capabilities

- `aiagent-session-biz-api`：业务会话 create/list/update/delete 的鉴权从「业务-智能体助手」改为只鉴「业务访问」；删除「IAM 业务-智能体助手权限点」注册与按 `bk_biz_id` 映射该 Action 的要求；明确 `/agui` `/history` `/cancel` 与平台 `/sessions/*` 继续只鉴平台权限。会话隔离、path 校验、归属校验保持不变。

## Impact

- **agent-server**：`cmd/agent-server/service/session/{create,query,update,delete}.go` 的 `Biz*` handler；`agentAuthMiddleware`（`service.go`）不改
- **auth-server**：`cmd/auth-server/service/auth/gen_id.go` 的 `genAgentAssistantResource` / `genBizAgentAssistantResource`；`adaptor.go` 若仅通过前者分发则随映射调整
- **pkg/iam**：`pkg/iam/sys/types.go`、`initial_actions.go`、`initial_action_groups.go` 移除 `BizAgentAssistant`
- **主规格**：`openspec/specs/aiagent-session-biz-api/spec.md` 中依赖 `biz_agent_assistant` 的 Requirement 必须改写
- **API 文档**：业务会话接口文档补充鉴权组合说明（路径不变）
- **前端**：不改；须与「前端鉴权逻辑调整」同迭代上线
- **权限中心**：下线 Action 后控制台不再可授「业务-智能体助手」；只授旧点、无业务访问的用户无法调用业务会话 CRUD；对话运行仍要求平台 `agent_assistant`
