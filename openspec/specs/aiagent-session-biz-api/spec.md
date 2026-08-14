## Requirements

### Requirement: IAM 业务-智能体助手权限点

系统 SHALL 注册 IAM Action `biz_agent_assistant`（显示名「业务-智能体助手」），类型为业务资源（`RelatedResourceTypes: bizResource`），关联 `BizAccess`。auth-server MUST 支持按 path 中的 `bk_biz_id` 生成 CMDB 业务资源实例进行鉴权。

#### Scenario: 权限点注册

- **WHEN** IAM 初始 Actions 加载
- **THEN** MUST 存在 ID 为 `biz_agent_assistant`、名称为「业务-智能体助手」的 Action，且关联 CMDB 业务资源类型

#### Scenario: 按 bk_biz_id 实例鉴权

- **WHEN** agent-server 对 `bk_biz_id=123` 发起 `BizAgentAssistant + Create/Find/Update/Delete` 鉴权
- **THEN** auth-server MUST 生成 CMDB biz 资源实例 ID 为 `"123"` 并校验用户权限

### Requirement: 业务维度创建会话 API

agent-server SHALL 提供 `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create`。path 参数 `bk_biz_id` MUST 为有效业务 ID（`> 0`）；请求体包含 `session_name`（规则与平台 create 一致）。系统 MUST 校验用户对 path 中 `bk_biz_id` 具备「业务-智能体助手」Create 权限；MUST 将会话 `bk_biz_id` 写为 path 值。

#### Scenario: 成功创建业务会话

- **WHEN** 用户对业务 123 具备「业务-智能体助手」Create 权限，调用 `POST /api/v1/agent/bizs/123/sessions/create`，body `{ "session_name": "申领对话" }`
- **THEN** 响应 MUST 返回合法 `session_code`，且库表记录 `bk_biz_id=123`
- **THEN** 响应 MUST NOT 要求返回 `bk_biz_id`

#### Scenario: 无业务权限拒绝创建

- **WHEN** 用户对业务 123 无「业务-智能体助手」权限，调用 `POST /api/v1/agent/bizs/123/sessions/create`
- **THEN** 系统 MUST 返回 `PermissionDenied`，错误信息 MUST 包含 `bk_biz_id`

#### Scenario: 非法 bk_biz_id 拒绝

- **WHEN** path `bk_biz_id` 为 `0` 或负数
- **THEN** 系统 MUST 返回 `InvalidParameter`

### Requirement: 业务维度会话列表 API

agent-server SHALL 提供 `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list`。系统 MUST 校验用户对 path 中 `bk_biz_id` 具备「业务-智能体助手」Find 权限；MUST 强制过滤 `user = 当前用户` 且 `bk_biz_id = path 值`，不得返回其他业务的会话。

#### Scenario: 仅返回当前业务会话

- **WHEN** 用户在同业务下已有 2 条会话、其他业务下有 3 条，调用 `POST /api/v1/agent/bizs/123/sessions/list`
- **THEN** 响应 details MUST 仅包含 `bk_biz_id=123` 的 2 条记录

#### Scenario: 无业务权限拒绝列表

- **WHEN** 用户对业务 123 无「业务-智能体助手」Find 权限，调用 list 接口
- **THEN** 系统 MUST 返回 `PermissionDenied`

### Requirement: 业务维度更新会话 API

agent-server SHALL 提供 `PATCH /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}`。path 参数 `bk_biz_id` MUST 为有效业务 ID（`> 0`）；系统 MUST 校验用户对 path 中 `bk_biz_id` 具备「业务-智能体助手」Update 权限；MUST 校验 `session_code` 对应会话属于当前用户且会话 `bk_biz_id` 等于 path 值，校验通过后更新会话名称。

#### Scenario: 成功更新业务会话

- **WHEN** 用户对业务 123 具备「业务-智能体助手」Update 权限，调用 `PATCH /api/v1/agent/bizs/123/sessions/{session_code}`，body `{ "session_name": "新的会话名称" }`
- **THEN** 系统 MUST 更新该会话名称并返回成功

#### Scenario: 无业务权限拒绝更新

- **WHEN** 用户对业务 123 无「业务-智能体助手」Update 权限，调用 update 接口
- **THEN** 系统 MUST 返回 `PermissionDenied`，错误信息 MUST 包含 `bk_biz_id`

#### Scenario: 更新其他业务会话被拒绝

- **WHEN** path `bk_biz_id=123`，但 `session_code` 对应会话的 `bk_biz_id=456`
- **THEN** 系统 MUST 拒绝更新，不得修改其他业务会话

#### Scenario: 非法 bk_biz_id 拒绝更新

- **WHEN** path `bk_biz_id` 为 `0` 或负数
- **THEN** 系统 MUST 返回 `InvalidParameter`

### Requirement: 业务维度删除会话 API

agent-server SHALL 提供 `DELETE /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}`。path 参数 `bk_biz_id` MUST 为有效业务 ID（`> 0`）；系统 MUST 校验用户对 path 中 `bk_biz_id` 具备「业务-智能体助手」Delete 权限；MUST 校验 `session_code` 对应会话属于当前用户且会话 `bk_biz_id` 等于 path 值，校验通过后删除会话。

#### Scenario: 成功删除业务会话

- **WHEN** 用户对业务 123 具备「业务-智能体助手」Delete 权限，调用 `DELETE /api/v1/agent/bizs/123/sessions/{session_code}`
- **THEN** 系统 MUST 删除该会话并返回成功

#### Scenario: 无业务权限拒绝删除

- **WHEN** 用户对业务 123 无「业务-智能体助手」Delete 权限，调用 delete 接口
- **THEN** 系统 MUST 返回 `PermissionDenied`，错误信息 MUST 包含 `bk_biz_id`

#### Scenario: 删除其他业务会话被拒绝

- **WHEN** path `bk_biz_id=123`，但 `session_code` 对应会话的 `bk_biz_id=456`
- **THEN** 系统 MUST 拒绝删除，不得删除其他业务会话

#### Scenario: 非法 bk_biz_id 拒绝删除

- **WHEN** path `bk_biz_id` 为 `0` 或负数
- **THEN** 系统 MUST 返回 `InvalidParameter`

### Requirement: 平台维度创建会话语义扩展

现有 `POST /api/v1/agent/sessions/create` SHALL 保持可用。创建时 MUST 将会话 `bk_biz_id` 写为 `constant.UnassignedBiz`（`-1`）；鉴权 MUST 保持平台级 `agent_assistant` Create 权限。

#### Scenario: 平台 create 写入未分配业务

- **WHEN** 用户调用 `POST /api/v1/agent/sessions/create`
- **THEN** 创建的会话记录 `bk_biz_id` MUST 为 `-1`
- **THEN** 响应 MUST NOT 要求返回 `bk_biz_id`

### Requirement: 平台维度会话列表语义扩展

现有 `POST /api/v1/agent/sessions/list` SHALL 继续使用现有列表语义：服务端 MUST NOT 主动追加 `bk_biz_id` 过滤；鉴权 MUST 保持平台级 `agent_assistant` Find 权限。客户端传入的 filter 仍按现有机制与 `user=当前用户` 合并处理。

#### Scenario: 跨业务返回全部会话

- **WHEN** 用户有多业务会话（含不同 `bk_biz_id`），调用 `POST /api/v1/agent/sessions/list` 且未传入 `bk_biz_id` filter
- **THEN** 响应 MUST 返回该用户全部会话，包含不同 `bk_biz_id` 的记录

### Requirement: 未改动接口保持 sessionCode 访问方式

以下平台接口 MUST NOT 在请求体或路径中新增 `bk_biz_id` 参数，仍仅通过 `session_code`（或 path 中的 `session_code`）访问：`POST /agui`、`POST /history`、`POST /cancel`、`PATCH/DELETE /sessions/{session_code}`、`GET /sessions/{session_code}/context_stats`。业务维度更新/删除会话通过新增的 `/bizs/{bk_biz_id}/sessions/{session_code}` 路径访问，不改变平台接口协议。
其中 `POST /agui` 运行入口鉴权 MUST 保持平台级 `agent_assistant` 权限门槛，不切换为业务级 `biz_agent_assistant` 权限。

#### Scenario: agui 请求体格式不变

- **WHEN** 客户端调用 `POST /api/v1/agent/agui`
- **THEN** 请求体 MUST 仍使用 `sessionCode` 字段，MUST NOT 要求 `bk_biz_id` 字段

#### Scenario: agui 平台权限门槛不变

- **WHEN** 客户端调用 `POST /api/v1/agent/agui`
- **THEN** 系统 MUST 继续校验平台级 `agent_assistant` 权限

### Requirement: session_code 接口会话归属显式校验

`POST /agui`、`POST /history`、`POST /cancel` 与 `GET /sessions/{session_code}/context_stats` 在通过 `session_code` 解析到会话元数据后，MUST 显式校验会话 `user` 与当前登录用户一致；若不一致，MUST 拒绝访问，不得将请求转发至下游 AG-UI runner 或读取该会话上下文统计。

#### Scenario: 当前用户访问自己的会话

- **WHEN** 用户 A 使用属于用户 A 的 `session_code` 调用 `/agui`
- **THEN** middleware MUST 通过归属校验并继续改写请求体

#### Scenario: 当前用户访问他人会话

- **WHEN** 用户 B 使用属于用户 A 的 `session_code` 调用 `/agui`
- **THEN** 系统 MUST 返回 `PermissionDenied` 或等效未授权错误
- **THEN** 请求 MUST NOT 进入下游 AG-UI runner

#### Scenario: 当前用户访问他人 context_stats

- **WHEN** 用户 B 使用属于用户 A 的 `session_code` 调用 `/sessions/{session_code}/context_stats`
- **THEN** 系统 MUST 返回 `PermissionDenied` 或等效未授权错误
- **THEN** 系统 MUST NOT 读取或返回用户 A 的会话上下文统计

### Requirement: 会话更新删除归属校验保持

`PATCH/DELETE /api/v1/agent/sessions/{session_code}` MUST 继续校验会话 `user` 与当前登录用户一致；行为与改造前一致。

#### Scenario: 用户 B 无法操作用户 A 的会话

- **WHEN** 用户 B 使用用户 A 的 `session_code` 调用更新或删除
- **THEN** 系统 MUST 拒绝操作

### Requirement: 业务 API 接口文档

系统 SHALL 在 `docs/api-docs/web-server/docs/biz/agent/` 下新增业务维度会话 create/list/update/delete 接口文档，版本占位符使用 `v9.9.9`，格式对齐现有 agent-server 接口文档。

#### Scenario: 文档覆盖业务 API

- **WHEN** 查阅 API 文档
- **THEN** MUST 存在 `POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create`、`POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list`、`PATCH /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}` 与 `DELETE /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}` 的完整说明（参数、响应、错误码）
