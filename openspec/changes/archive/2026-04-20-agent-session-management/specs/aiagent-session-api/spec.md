## ADDED Requirements

### Requirement: agent-server 创建会话接口

agent-server SHALL 提供 `POST /api/v1/agent/sessions/create` 接口。请求体包含 `session_name`（可选）。系统从请求头 `X-Bkapi-User-Name` 获取用户信息，调用 data-service 创建记录。

#### Scenario: 成功创建会话

- **WHEN** 已认证用户发送创建请求 `{ "session_name": "我的第一个对话" }`
- **THEN** 系统 MUST 调用 data-service 创建会话，传入 app_name、user、session_name、creator
- **THEN** 响应 MUST 返回 `{ id, session_code, thread_id, session_name }`

#### Scenario: 未提供 session_name

- **WHEN** 用户发送空请求体或不包含 session_name
- **THEN** 系统 MUST 使用默认空字符串作为 session_name 创建会话

### Requirement: agent-server 更新会话接口

agent-server SHALL 提供 `PATCH /api/v1/agent/sessions/{session_code}` 接口，仅允许修改 `session_name`。

#### Scenario: 成功更新会话名称

- **WHEN** 用户发送 `{ "session_name": "新名称" }` 到 `/api/v1/agent/sessions/{session_code}`
- **THEN** 系统 MUST 校验 session 属于当前用户后调用 data-service 更新

#### Scenario: 更新其他用户的会话

- **WHEN** 用户尝试更新不属于自己的 session
- **THEN** 系统 MUST 返回权限错误

### Requirement: agent-server 会话列表接口

agent-server SHALL 提供 `POST /api/v1/agent/sessions/list` 接口，支持 filter + page 分页查询。系统 MUST 自动在 filter 中注入 `user = 当前用户` 条件，确保用户只能查看自己的会话。

#### Scenario: 查询当前用户会话列表

- **WHEN** 用户发送列表查询请求，包含 filter 和 page 参数
- **THEN** 系统 MUST 自动注入 user 过滤条件
- **THEN** 响应 MUST 返回 `{ count, details }` 格式，details 包含会话完整信息

#### Scenario: 支持按 is_temporary 过滤

- **WHEN** filter 包含 `is_temporary = false`
- **THEN** 系统 MUST 仅返回正式会话（非临时）

### Requirement: agent-server 删除会话接口

agent-server SHALL 提供 `DELETE /api/v1/agent/sessions/{session_code}` 接口。系统 MUST 校验 `user == 当前用户` 后调用 data-service 删除。

#### Scenario: 成功删除会话

- **WHEN** 用户删除自己的会话
- **THEN** 系统 MUST 校验所有权后调用 data-service 批量删除接口

#### Scenario: 删除其他用户的会话

- **WHEN** 用户尝试删除不属于自己的 session
- **THEN** 系统 MUST 返回权限错误

### Requirement: data-service client 封装

pkg/client/data-service/aiagent/ 目录 SHALL 提供 `Session` client 接口，封装对 data-service aiagent session 接口的调用，包含 Create、Update、List、Delete、IncrContentCount 方法。client MUST 通过 `pkg/client/common/request.go` 的方法进行 HTTP 请求封装。

#### Scenario: client 正确调用 data-service

- **WHEN** agent-server 通过 client 调用创建会话
- **THEN** client MUST 发送 POST 请求到 `/api/v1/data/aiagent/sessions/create`，包含正确的请求体

### Requirement: data-service client 注册

`pkg/client/data-service/client.go` SHALL 挂载 `Aiagent` 子模块，提供 `client.Aiagent.Session` 访问路径。

#### Scenario: client 模块可访问

- **WHEN** 代码通过 `dataServiceClient.Aiagent.Session.Create(...)` 调用
- **THEN** MUST 正确路由到 data-service aiagent session 创建接口

### Requirement: context-stats 接口改造

agent-server 现有的 `GET /api/v1/agent/sessions/{thread_id}/context-stats` 接口 SHALL 将路径参数从 `thread_id` 改为 `session_code`，变为 `GET /api/v1/agent/sessions/{session_code}/context-stats`。handler 内部通过 `sessionResolver` 完成 sessionCode → threadId 的转换，再用 threadId 查询上下文统计信息。

#### Scenario: 通过 sessionCode 查询 context-stats

- **WHEN** 客户端发送 `GET /api/v1/agent/sessions/{session_code}/context-stats`
- **THEN** handler MUST 通过 sessionResolver 将 session_code 解析为 thread_id
- **THEN** handler MUST 使用解析后的 thread_id 查询上下文统计信息并返回

#### Scenario: sessionCode 无效

- **WHEN** 路径参数中的 session_code 在数据库中不存在
- **THEN** handler MUST 返回错误响应

#### Scenario: 复用 sessionResolver 缓存

- **WHEN** context-stats handler 解析 sessionCode
- **THEN** MUST 复用与 sessionCodeMiddleware 相同的 sessionResolver 实例（含 LRU 缓存），避免重复查询 data-service
