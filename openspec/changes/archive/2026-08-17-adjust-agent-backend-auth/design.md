## Context

业务会话 CRUD（`/api/v1/agent/bizs/{bk_biz_id}/sessions/*`）当前只调一次 `AuthorizeWithPerm`，`ResourceAttribute` 带 `Type: AgentAssistant` 且 `BizID > 0`。auth-server `genAgentAssistantResource` 据此改走 `genBizAgentAssistantResource`，IAM Action 变成 `biz_agent_assistant`（关联 `BizAccess`）。平台 `/sessions/*` 与 `/agui` `/history` `/cancel`（`agentAuthMiddleware`）不带 BizID，映射平台 `agent_assistant`。

本期要把业务会话 CRUD 的能力范围改成只鉴「业务访问」，并删除旧的业务智能体 Action。对话运行与平台 `/sessions/*` 仍走平台 `agent_assistant`。主规格 `openspec/specs/aiagent-session-biz-api/spec.md` 仍要求注册并使用 `biz_agent_assistant`，本变更用 delta 改写。功能未上线，无存量双写。前端入口不在本变更，但必须与前端子单同迭代发布。

## Goals / Non-Goals

**Goals:**

- 业务会话 4 个接口：只鉴对应 `bk_biz_id` 的 `biz_access`（不叠加平台 `agent_assistant`）。
- `genAgentAssistantResource` 不再按 BizID 切换 Action；删除 `biz_agent_assistant` 注册与映射函数。
- `/agui` `/history` `/cancel` 行为与现网一致（平台权限 + 会话归属）。
- 更新业务会话 API 文档中的权限描述。

**Non-Goals:**

- 不改前端菜单/路由/浮窗。
- 不为对话运行接口叠加业务访问。
- 不改平台 `/sessions/*`、memory、skill 的鉴权。
- 不做旧权限点兼容、不做授权迁移。
- 不新增 HTTP 路径或请求字段。

## Decisions

### D1：业务会话只鉴 `biz_access`，不叠加平台智能体助手

业务会话 CRUD 只能操作当前用户自己的会话（list 强制 `user=当前用户`，update/delete 校验会话归属），风险可控，IAM 只校验 path 上 `bk_biz_id` 的「业务访问」。平台 `agent_assistant` 继续作为 `/agui` `/history` `/cancel` 与平台 `/sessions/*` 的能力门。

实现：`Biz*` handler 只调一次 `AuthorizeWithPerm(meta.Biz, meta.Access, BizID=path)`。**禁止**再把 `BizID` 填进 `AgentAssistant`。

**备选**：业务会话同时鉴平台 `agent_assistant` + `biz_access`。**放弃原因**：会话接口只能操作本人数据，叠加平台权限会抬高灰度授权成本，且与「入口/对话运行才需要智能体能力」不一致。

**备选**：只鉴 `agent_assistant`、依赖业务视图网关已拦业务访问。**放弃原因**：业务会话 API 可被直接调用，必须在 agent-server 内校验 `bk_biz_id`。

### D2：`genAgentAssistantResource` 去掉 BizID 分支，删除 `genBizAgentAssistantResource`

`adaptor.go` 只有 `meta.AgentAssistant → genAgentAssistantResource`，没有独立的 `meta.BizAgentAssistant`。业务侧切换完全靠 `BizID > 0`。去掉该分支后，带 BizID 的 `AgentAssistant` 也会落到平台 Action；因此业务 handler **禁止**再对 `AgentAssistant` 填 BizID。业务会话改走单独的 `meta.Biz` + `meta.Access`。

删除 `genBizAgentAssistantResource` 与 `sys.BizAgentAssistant` 常量、`genBizAgentAssistantActions`、action group 中的「智能体助手」分组项。全仓检索残留引用。

### D3：不改 `agentAuthMiddleware`

`/agui` `/history` `/cancel` 已通过 middleware 鉴平台 `AgentAssistant` + Find。需求要求本期不调整。会话归属仍由现有 session-code / resolver 校验。

### D4：缺业务访问时错误信息带 `bk_biz_id`

主规格已要求 `PermissionDenied` 信息包含 `bk_biz_id`。现网部分 handler 只返回 `"permission denied"`。本次在业务访问失败路径补上 `bk_biz_id`（例如 `permission denied, bk_biz_id: %d`）。业务会话不再校验平台权限。

### D5：IAM 下线走现有初始 Actions 注册，不做单独迁移脚本

auth-server 启动/注册流程会把 `initial_actions` 同步到权限中心。从初始列表移除即下线可授 Action。无 SQL、无数据修复。回滚方式是还原代码并重新注册。

### D6：与前端同迭代发布，但代码上不互相阻塞

后端先上、前端未改：入口仍查 `biz_agent_assistant`，有平台权限的人进不了首页。前端先上、后端未改：入口可见，业务会话仍要旧权限点 → 403。发布约束写在任务与风险里，实现上两仓改动无编译依赖。

## Risks / Trade-offs

- **[业务会话可无平台权限调用]** → 只能读写本人会话；真正跑对话仍要过 `/agui` 的平台 `agent_assistant`。入口展示由前端子单控制。
- **[前端/后端发布窗口不一致]** → 与前端子单约定同迭代。
- **[权限中心仍短暂可见旧 Action]** → 注册变更随 auth-server 发布；过渡期即使误授旧点也不产生能力（运行时已停用）。

## Migration Plan

1. 合并并发布 agent-server + auth-server（含 IAM 初始 Actions）。
2. 确认权限中心不再列出「业务-智能体助手」。
3. 与前端变更同窗口：业务会话靠已有 `biz_access`；对话运行与入口仍依赖平台 `agent_assistant`。
4. 回滚：还原本变更提交；必要时重新注册 `biz_agent_assistant`（仅当前端仍检查该点时需要）。

## Open Questions

无。澄清结论（Q-001～Q-004）与拆单评审已关闭：不考虑存量；对话运行不改业务鉴权；IAM 点下线并入本单。
