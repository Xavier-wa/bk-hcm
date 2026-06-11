# Capability: agent-session-tag

## Purpose

为 AI Agent 会话提供持久化的场景标签（`session_tag`）能力：在会话创建时可选绑定场景标签，全链路透传落库到 `aiagent_session.session_tag` 列，查询时返回，并支持运行时由意图识别结果回写，使后续轮次稳定直达对应场景。

## Requirements

### Requirement: 会话支持持久化场景标签 session_tag

系统 SHALL 在会话创建链路支持可选字段 `session_tag`，并将其持久化到 `aiagent_session` 表的独立列 `session_tag`。`session_tag` 的合法取值 SHALL 与 `enumor.IntentType` 对齐（当前合法直达值为 `host_apply`）。未传 `session_tag` 时，会话 SHALL 视为无标签会话，行为与现网一致。

#### Scenario: 创建会话写入合法 session_tag

- **WHEN** 调用 `POST /api/v1/agent/sessions/create` 传入 `session_tag=host_apply`
- **THEN** 会话记录的 `session_tag` 列持久化为 `host_apply`，且创建响应回显 `session_tag=host_apply`

#### Scenario: 创建会话不传 session_tag 保持兼容

- **WHEN** 调用创建会话接口且不传 `session_tag`
- **THEN** 会话创建成功，`session_tag` 列为空字符串，后续 Run 行为与现网无标签会话一致

#### Scenario: 非法 session_tag 被拒绝

- **WHEN** 创建会话传入不在 `IntentType` 枚举内的 `session_tag`（如 `foo`）
- **THEN** 接口返回 `InvalidParameter` 错误，且不创建会话

### Requirement: 会话查询返回 session_tag

系统 SHALL 在会话列表/详情查询结果中返回 `session_tag` 字段（取自 `aiagent_session.session_tag` 列）。无标签会话 SHALL 返回空值。

#### Scenario: list_session 返回已绑定的 session_tag

- **WHEN** 调用 `POST /api/v1/agent/sessions/list` 查询一个 `session_tag=host_apply` 的会话
- **THEN** 返回的会话明细包含 `session_tag=host_apply`

#### Scenario: list_session 返回无标签会话

- **WHEN** 查询一个未绑定标签的会话
- **THEN** 返回的会话明细中 `session_tag` 为空值

### Requirement: session_tag 在 agent-server/data-service/DAO 全链路透传

系统 SHALL 在 `CreateSessionReq`（agent-server）、`CreateAiagentSessionReq`（data-service）等协议结构体中新增 `session_tag` 字段，并在 agent-server → data-service → DAO 链路中透传落库，禁止在 data-service 以外的服务直接操作 DB。

#### Scenario: 创建标签会话的全链路落库

- **WHEN** agent-server 收到带 `session_tag` 的创建请求并校验通过
- **THEN** agent-server 经 data-service 客户端透传 `session_tag`，data-service 将其写入 `aiagent_session.session_tag` 列后返回

### Requirement: 运行时回写 session_tag

系统 SHALL 支持在 `scene_dispatch` 将无标签会话的本轮意图判定为受支持场景（`host_apply`）后，将 `session_tag` 回写到对应会话的 `session_tag` 列，并同步更新/失效会话解析缓存。回写失败 SHALL 仅记录 Warn 日志且不阻断当前对话。

#### Scenario: 识别为 host_apply 后回写标签

- **WHEN** 无标签会话经意图识别得到 `host_apply`，`scene_dispatch` 判定为受支持
- **THEN** 系统将该会话的 `session_tag` 列回写为 `host_apply`，并更新会话解析缓存，使后续冷启动与 list 查询可读到该标签

#### Scenario: 回写失败不阻断对话

- **WHEN** 回写 `session_tag` 的 data-service 调用失败
- **THEN** 系统记录 Warn 日志，当前 Run 仍按 `host_apply` 继续（由 checkpoint 的 `StateKeySessionTag` 保证路由）
