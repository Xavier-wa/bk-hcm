## 1. 业务会话鉴权

- [x] 1.1 在 `cmd/agent-server/service/session` 抽出业务会话鉴权辅助函数：只 `AuthorizeWithPerm(Biz, Access, BizID=path)`；业务访问失败时错误信息包含 `bk_biz_id`；不叠加平台 `agent_assistant`
- [x] 1.2 修改 `BizCreateSession`：去掉 `AgentAssistant` 鉴权，改为调用 1.1；`bk_biz_id <= 0` 仍返回 `InvalidParameter`
- [x] 1.3 修改 `BizListSessions`：同上，只鉴业务访问
- [x] 1.4 修改 `BizUpdateSession`：同上，只鉴业务访问；保留会话归属与 `bk_biz_id` 一致性校验
- [x] 1.5 修改 `BizDeleteSession`：同上，只鉴业务访问；保留会话归属与 `bk_biz_id` 一致性校验
- [x] 1.6 确认平台 `CreateSession` / `ListSessions` / 平台 update/delete 仍只鉴无 BizID 的 `AgentAssistant`，不叠加业务访问

## 2. auth-server 映射

- [x] 2.1 修改 `genAgentAssistantResource`：删除 `BizID > 0` 分支，始终返回 `sys.AgentAssistant`
- [x] 2.2 删除 `genBizAgentAssistantResource`；确认 `adaptor.go` 仍仅 `meta.AgentAssistant → genAgentAssistantResource`，无其它入口

## 3. IAM 注册下线

- [x] 3.1 从 `pkg/iam/sys/types.go` 移除 `BizAgentAssistant` 常量及 `ActionIDNameMap` 条目
- [x] 3.2 从 `pkg/iam/sys/initial_actions.go` 移除 `genBizAgentAssistantActions` 及其 append
- [x] 3.3 从 `pkg/iam/sys/initial_action_groups.go` 移除业务分组下「智能体助手」/`BizAgentAssistant` 项
- [x] 3.4 全仓检索 `biz_agent_assistant` / `BizAgentAssistant`，清理后端与文档残留（前端文件属另一子单，本变更不改 `front/`）

## 4. 对话运行接口回归（不改行为）

- [x] 4.1 核对 `agentAuthMiddleware` 仍只鉴平台 `AgentAssistant` + Find，路径仍覆盖 `/agui` `/history` `/cancel`
- [x] 4.2 核对上述三个接口请求体仍不要求 `bk_biz_id`，会话归属校验保持现网

## 5. API 文档

- [x] 5.1 更新 `docs/api-docs/web-server/docs/biz/agent/create_session.md` 所需权限与 2000012 说明为「业务访问」
- [x] 5.2 同步更新 `list_session.md` / `update_session.md` / `delete_session.md` 权限描述，禁止再写「业务-智能体助手」或要求平台智能体助手

## 6. 验证

- [x] 6.1 `gofmt` / 项目规范检查改动的 Go 文件；日志含 `rid`，错误后 `return` 用 `Errorf`
- [x] 6.2 有单测则更新鉴权期望；否则至少编译涉及包：`go test`/`go build` agent-server session 与 auth-server auth 相关包
- [x] 6.3 权限矩阵手测或对照清单：有业务访问 → CRUD 成功（不要求平台智能体助手）；无该业务访问 → 拒绝且信息含 `bk_biz_id`；仅旧 `biz_agent_assistant` 且无业务访问 → 拒绝
- [x] 6.4 手测 `/agui` `/history` `/cancel`：有平台权限且会话属于自己则通过，不校验业务访问
- [x] 6.5 发布后在权限中心确认不再出现可授的「业务-智能体助手」（随 auth-server 注册变更）
