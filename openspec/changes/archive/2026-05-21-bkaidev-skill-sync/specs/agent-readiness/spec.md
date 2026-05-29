## ADDED Requirements

### Requirement: agenthealthz 就绪检查接口

agent-server SHALL 提供 `GET /api/v1/agent/healthz`（agenthealthz）接口，供前端判断 Agent 是否可与用户交互。

响应 MUST 包含：

- `ready`: 整体是否就绪（skillReady && promptReady）
- `skillReady`: Skill 首次同步是否完成
- `promptReady`: Prompt 首次同步是否完成（未启用远程 Prompt 同步时默认为 true）

#### Scenario: Skill 首次同步未完成

- **WHEN** `skillReady=false`
- **THEN** `ready` MUST 为 false
- **THEN** HTTP 状态码 SHOULD 为 503（或 200 且 body 中 ready=false，实现时统一一种）

#### Scenario: 全部就绪

- **WHEN** `skillReady=true` 且 `promptReady=true`
- **THEN** `ready` MUST 为 true

#### Scenario: 与 etcd healthz 分离

- **WHEN** 调用现有 `/healthz`
- **THEN** 其行为 MUST 保持 etcd 探活语义，不受 Skill 同步状态影响

### Requirement: Readiness 中间件门禁

系统 MUST 通过 middleware 拦截 AGUI 及相关 Agent 交互路由：仅当 `readiness.IsReady()` 为 true 时才放行请求。

Readiness 判定 MUST 同时考虑 Skill 与 Prompt 首次同步状态（Prompt 未配置远程同步时 `promptReady` 默认为 true）。

#### Scenario: 未就绪时拒绝 AGUI 请求

- **WHEN** 用户请求 `/api/v1/agent/agui` 且 `readiness.IsReady()` 为 false
- **THEN** 系统 MUST 返回 503 及明确错误信息（英文），提示 Agent 尚未就绪

#### Scenario: 就绪后允许交互

- **WHEN** Skill 与 Prompt 首次同步均完成且用户请求 AGUI
- **THEN** 请求 MUST 正常进入 AGUI handler

#### Scenario: agenthealthz 不受中间件拦截

- **WHEN** 请求 `GET /api/v1/agent/healthz`
- **THEN** 该请求 MUST NOT 被 readiness 中间件阻断
