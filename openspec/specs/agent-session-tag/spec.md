# Capability: agent-session-tag

## Purpose

为 AI Agent 会话提供持久化的场景标签（`session_tag`）能力：在会话创建时可选绑定场景标签，全链路透传落库到 `aiagent_session.session_tag` 列，查询时返回，并支持运行时由意图识别 / 场景切换结果回写，使会话列表与冷启动读到最新标签。

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

系统 SHALL 在 `scene_dispatch` 判定出「本轮要提交的标签与进入该节点时 graph state 中的标签不同」时，**在该节点内同步**把新标签经 data-service 回写到 `aiagent_session.session_tag`。判据 SHALL 为「标签发生变化」，而非「是否构成场景切换」，因此 SHALL 同时覆盖「无标签会话首次识别出受支持场景」与「已有标签会话在轮次边界切换到新场景」两种情形。标签未发生变化时 SHALL NOT 调用 data-service 更新接口。

该回写 SHALL 排在 `scene.switched` 事件 emit **之前**，使 DB 的更新早于前端收到切换事件；系统 SHALL NOT 把该回写放到 goroutine 中与 emit 竞争。

节点内回写 SHALL 以 threadID（`RuntimeState[lineage_id]`）定位会话记录，SHALL NOT 依赖 sessionCode；因此节点内回写 SHALL NOT 负责刷新会话解析缓存，缓存刷新由 Run 结束后的兜底对账负责。缓存滞后 SHALL NOT 改变路由——框架只为 checkpoint 中缺失的 key 注入初始值，checkpoint 中已有的 `StateKeySessionTag` 优先。

节点内回写失败（含 data-service 调用失败、请求上下文被取消、data-service 客户端未注入、threadID 无法解析）SHALL 仅记录日志，SHALL NOT 阻断本轮 Run、SHALL NOT 改变本轮路由决策、SHALL NOT 影响 `scene.switched` 事件的发送。

系统 SHALL 保留 Run 结束后的差异回写作为兜底：在 SSE 响应结束（`next.ServeHTTP` 返回，代表本轮 Run 已完成）之后，把 checkpoint 中的 `StateKeySessionTag` 与该请求进入时会话已有的标签比对，不同即回写并刷新会话解析缓存。兜底回写 SHALL NOT 与 Run 并发执行——并发读取到的是切换前的 checkpoint。兜底回写失败 SHALL 仅记录 Warn 日志且不阻断当前对话。系统 SHALL NOT 因节点内已回写而移除该兜底，并 SHALL 接受一次标签变化产生两次同值写入（写入幂等）。

系统 SHALL NOT 以「请求进入时已有标签」为由跳过回写。

#### Scenario: 首次识别为受支持场景时在判定点即回写

- **GIVEN** 会话无 `session_tag`
- **WHEN** 本轮意图分类得到 `host_apply`，`scene_dispatch` 将其提交为会话标签
- **THEN** 系统在该节点内、进入 `host_apply` 子图之前即把该会话的 `session_tag` 列写为 `host_apply`，不等待本轮 Run 结束

#### Scenario: 场景切换时 DB 更新早于前端收到事件

- **GIVEN** 会话进入本轮时 `session_tag=host_apply`
- **WHEN** 本轮在轮次边界切换到 `resource_query`
- **THEN** `session_tag` 列先被写为 `resource_query`，之后才发出 `scene.switched` 事件；前端在收到事件后无论重新拉取会话列表还是会话详情，读到的都是 `resource_query`

#### Scenario: 切换窗口内切走再切回读到新标签

- **GIVEN** 会话从 `host_apply` 切换到 `resource_query`，新场景子图仍在输出
- **WHEN** 用户切换到其它会话后再切回该会话，前端从 DB 重新拉取会话详情
- **THEN** 返回的 `session_tag` 为 `resource_query`，不再出现一段时间内显示旧标签的情况

#### Scenario: 标签未变更时不回写

- **GIVEN** 会话进入本轮时 `session_tag=host_apply`
- **WHEN** 本轮分类结果仍为 `host_apply`，或分类结果不受支持而保持原标签
- **THEN** 系统既不在节点内回写，Run 结束后的兜底对账也不调用 data-service 更新接口

#### Scenario: 节点内回写失败由兜底对账补齐

- **GIVEN** `scene_dispatch` 判定标签由 `host_apply` 变为 `resource_query`
- **WHEN** 节点内的 data-service 调用失败
- **THEN** 系统记录日志后继续路由到 `resource_query` 子图并照常发出 `scene.switched`；Run 结束后的兜底对账读取最新 checkpoint，把 `session_tag` 补写为 `resource_query` 并刷新会话解析缓存

#### Scenario: 客户端断连时标签已在判定点落库

- **GIVEN** 客户端在 Run 结束前断开 SSE 连接
- **WHEN** `next.ServeHTTP` 提前返回、兜底对账可能读到未完成的 checkpoint
- **THEN** 本轮标签已在 `scene_dispatch` 判定点写入 DB，不因断连而丢失；系统 SHALL NOT 为兜底路径加入重试等待

#### Scenario: 未注入 data-service 客户端时安全跳过

- **GIVEN** `scene_dispatch` 未被注入 data-service 客户端（如单元测试场景）
- **WHEN** 本轮判定出标签变化
- **THEN** 系统跳过节点内回写并记录 Warn 日志，决策矩阵、路由结果与 `scene.switched` 事件行为均不受影响

#### Scenario: 回写失败不阻断对话

- **WHEN** 节点内回写与兜底回写均失败
- **THEN** 系统记录日志，当前会话仍按 checkpoint 中的 `StateKeySessionTag` 继续路由，下一轮 Run 会再次尝试回写
