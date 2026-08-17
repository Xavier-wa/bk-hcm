## ADDED Requirements

### Requirement: IAM 下线业务-智能体助手

系统 SHALL 从 IAM 初始 Actions、Action Groups 与 ActionID 常量中移除 `biz_agent_assistant`（显示名「业务-智能体助手」）。auth-server 将 `meta.AgentAssistant` 映射为 IAM Action 时 MUST 始终返回平台 `agent_assistant`，MUST NOT 因 `ResourceAttribute.BizID > 0` 改映射为 `biz_agent_assistant`。系统 SHALL NOT 为存量授权做迁移或双权限点兼容。

#### Scenario: 初始 Actions 不再注册旧权限点

- **WHEN** IAM 初始 Actions 加载
- **THEN** MUST NOT 存在 ID 为 `biz_agent_assistant` 的 Action

#### Scenario: AgentAssistant 一律映射平台权限点

- **WHEN** auth-server 处理 `meta.AgentAssistant` 且 `BizID > 0`
- **THEN** MUST 生成平台 Action `agent_assistant`，MUST NOT 生成 `biz_agent_assistant`

#### Scenario: 仅有旧权限点无接口能力

- **WHEN** 用户只被授予 `biz_agent_assistant`、未被授予「业务访问」
- **THEN** 调用业务会话 CRUD MUST 返回 `PermissionDenied`

- **WHEN** 用户只被授予 `biz_agent_assistant`、未被授予 `agent_assistant`
- **THEN** 调用 `/agui` `/history` `/cancel` MUST 返回 `PermissionDenied`

## MODIFIED Requirements

### Requirement: 业务维度创建会话 API

agent-server SHALL 提供 `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create`。path 参数 `bk_biz_id` MUST 为有效业务 ID（`> 0`）；请求体包含 `session_name`（规则与平台 create 一致）。系统 MUST 校验用户对该 path 中 `bk_biz_id` 具备「业务访问」`biz_access`；MUST 将会话 `bk_biz_id` 写为 path 值。系统 MUST NOT 再校验「业务-智能体助手」，MUST NOT 叠加平台「智能体助手」。

#### Scenario: 成功创建业务会话

- **WHEN** 用户对业务 123 具备「业务访问」，调用 `POST /api/v1/agent/bizs/123/sessions/create`，body `{ "session_name": "申领对话" }`
- **THEN** 响应 MUST 返回合法 `session_code`，且库表记录 `bk_biz_id=123`
- **THEN** 响应 MUST NOT 要求返回 `bk_biz_id`

#### Scenario: 无平台智能体权限仍可创建

- **WHEN** 用户不具备平台「智能体助手」，但对业务 123 有「业务访问」，调用 `POST /api/v1/agent/bizs/123/sessions/create`
- **THEN** 系统 MUST 允许创建（不因缺少平台智能体助手而拒绝）

#### Scenario: 无业务访问拒绝创建

- **WHEN** 用户对业务 123 无「业务访问」，调用 `POST /api/v1/agent/bizs/123/sessions/create`
- **THEN** 系统 MUST 返回 `PermissionDenied`，错误信息 MUST 包含 `bk_biz_id`

#### Scenario: 非法 bk_biz_id 拒绝

- **WHEN** path `bk_biz_id` 为 `0` 或负数
- **THEN** 系统 MUST 返回 `InvalidParameter`

### Requirement: 业务维度会话列表 API

agent-server SHALL 提供 `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list`。系统 MUST 校验用户对该 path 中 `bk_biz_id` 具备「业务访问」；MUST 强制过滤 `user = 当前用户` 且 `bk_biz_id = path 值`，不得返回其他业务的会话。系统 MUST NOT 再校验「业务-智能体助手」，MUST NOT 叠加平台「智能体助手」。

#### Scenario: 仅返回当前业务会话

- **WHEN** 用户对业务 123 有「业务访问」，在同业务下已有 2 条会话、其他业务下有 3 条，调用 `POST /api/v1/agent/bizs/123/sessions/list`
- **THEN** 响应 details MUST 仅包含 `bk_biz_id=123` 的 2 条记录

#### Scenario: 无业务访问拒绝列表

- **WHEN** 用户对业务 123 无「业务访问」，调用 list 接口
- **THEN** 系统 MUST 返回 `PermissionDenied`

### Requirement: 业务维度更新会话 API

agent-server SHALL 提供 `PATCH /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}`。path 参数 `bk_biz_id` MUST 为有效业务 ID（`> 0`）；系统 MUST 校验用户对该 path 中 `bk_biz_id` 具备「业务访问」；MUST 校验 `session_code` 对应会话属于当前用户且会话 `bk_biz_id` 等于 path 值，校验通过后更新会话名称。系统 MUST NOT 再校验「业务-智能体助手」，MUST NOT 叠加平台「智能体助手」。

#### Scenario: 成功更新业务会话

- **WHEN** 用户对业务 123 有「业务访问」，调用 `PATCH /api/v1/agent/bizs/123/sessions/{session_code}`，body `{ "session_name": "新的会话名称" }`
- **THEN** 系统 MUST 更新该会话名称并返回成功

#### Scenario: 无业务访问拒绝更新

- **WHEN** 用户对业务 123 无「业务访问」，调用 update 接口
- **THEN** 系统 MUST 返回 `PermissionDenied`，错误信息 MUST 包含 `bk_biz_id`

#### Scenario: 更新其他业务会话被拒绝

- **WHEN** path `bk_biz_id=123`，但 `session_code` 对应会话的 `bk_biz_id=456`
- **THEN** 系统 MUST 拒绝更新，不得修改其他业务会话

#### Scenario: 非法 bk_biz_id 拒绝更新

- **WHEN** path `bk_biz_id` 为 `0` 或负数
- **THEN** 系统 MUST 返回 `InvalidParameter`

### Requirement: 业务维度删除会话 API

agent-server SHALL 提供 `DELETE /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}`。path 参数 `bk_biz_id` MUST 为有效业务 ID（`> 0`）；系统 MUST 校验用户对该 path 中 `bk_biz_id` 具备「业务访问」；MUST 校验 `session_code` 对应会话属于当前用户且会话 `bk_biz_id` 等于 path 值，校验通过后删除会话。系统 MUST NOT 再校验「业务-智能体助手」，MUST NOT 叠加平台「智能体助手」。

#### Scenario: 成功删除业务会话

- **WHEN** 用户对业务 123 有「业务访问」，调用 `DELETE /api/v1/agent/bizs/123/sessions/{session_code}`
- **THEN** 系统 MUST 删除该会话并返回成功

#### Scenario: 无业务访问拒绝删除

- **WHEN** 用户对业务 123 无「业务访问」，调用 delete 接口
- **THEN** 系统 MUST 返回 `PermissionDenied`，错误信息 MUST 包含 `bk_biz_id`

#### Scenario: 删除其他业务会话被拒绝

- **WHEN** path `bk_biz_id=123`，但 `session_code` 对应会话的 `bk_biz_id=456`
- **THEN** 系统 MUST 拒绝删除，不得删除其他业务会话

#### Scenario: 非法 bk_biz_id 拒绝删除

- **WHEN** path `bk_biz_id` 为 `0` 或负数
- **THEN** 系统 MUST 返回 `InvalidParameter`

### Requirement: 未改动接口保持 sessionCode 访问方式

以下平台接口 MUST NOT 在请求体或路径中新增 `bk_biz_id` 参数，仍仅通过 `session_code`（或 path 中的 `session_code`）访问：`POST /agui`、`POST /history`、`POST /cancel`、`PATCH/DELETE /sessions/{session_code}`、`GET /sessions/{session_code}/context_stats`。业务维度更新/删除会话通过 `/bizs/{bk_biz_id}/sessions/{session_code}` 路径访问，不改变平台接口协议。
其中 `POST /agui`、`POST /history`、`POST /cancel` 运行入口鉴权 MUST 保持平台级 `agent_assistant` 权限门槛，MUST NOT 叠加「业务访问」，MUST NOT 切换为业务级 `biz_agent_assistant`。

#### Scenario: agui 请求体格式不变

- **WHEN** 客户端调用 `POST /api/v1/agent/agui`
- **THEN** 请求体 MUST 仍使用 `sessionCode` 字段，MUST NOT 要求 `bk_biz_id` 字段

#### Scenario: agui 平台权限门槛不变

- **WHEN** 客户端调用 `POST /api/v1/agent/agui`
- **THEN** 系统 MUST 继续校验平台级 `agent_assistant` 权限，MUST NOT 校验「业务访问」

#### Scenario: history 与 cancel 不叠加业务访问

- **WHEN** 客户端调用 `POST /api/v1/agent/history` 或 `POST /api/v1/agent/cancel`
- **THEN** 系统 MUST 继续只校验平台级 `agent_assistant` 与会话归属，MUST NOT 新增「业务访问」校验

### Requirement: 业务 API 接口文档

系统 SHALL 在 `docs/api-docs/web-server/docs/biz/agent/` 下更新业务维度会话 create/list/update/delete 接口文档，将所需权限改为「业务访问」；版本占位符遵循仓库接口文档规范。格式对齐现有 agent-server 接口文档。

#### Scenario: 文档覆盖业务 API 鉴权组合

- **WHEN** 查阅 API 文档
- **THEN** MUST 存在上述四个接口的完整说明（参数、响应、错误码），且权限描述 MUST 为业务访问，MUST NOT 再写「业务-智能体助手」或要求平台智能体助手

## REMOVED Requirements

### Requirement: IAM 业务-智能体助手权限点

**Reason**: 业务会话改为只鉴「业务访问」；对话运行仍走平台「智能体助手」。该业务智能体 Action 不再产生任何能力，继续注册会造成误授。

**Migration**: 无存量迁移。业务会话运行时改为校验 `biz_access`；IAM 注册中删除 `biz_agent_assistant`。前端入口由独立变更加 `chatbot_access` / `agent_assistant`，须同迭代发布。
