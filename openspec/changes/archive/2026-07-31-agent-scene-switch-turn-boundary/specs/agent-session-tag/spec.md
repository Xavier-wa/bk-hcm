# Capability: agent-session-tag

## MODIFIED Requirements

### Requirement: 运行时回写 session_tag

系统 SHALL 在一次 Run 结束后，把 checkpoint 中的 `StateKeySessionTag` 与该请求进入时会话已有的标签做比对：**两者不同即回写**，包括「无标签会话首次识别出受支持场景」与「已有标签会话在轮次边界切换到新场景」两种情形。回写后 SHALL 同步更新会话解析缓存。回写失败 SHALL 仅记录 Warn 日志且不阻断当前对话。

系统 SHALL NOT 再以「请求进入时已有标签」为由跳过回写（原 `reconcileSessionTag` 中 `originalTag != ""` 的短路）。

回写 SHALL 在 SSE 响应结束（`next.ServeHTTP` 返回，代表本轮 Run 已完成）之后触发，SHALL NOT 与 Run 并发执行——并发读取到的是切换前的 checkpoint，会导致标签回写滞后一轮。会话消息计数（`IncrContentCount`）不受此约束，可继续与 Run 并发。

#### Scenario: 识别为 host_apply 后回写标签

- **GIVEN** 会话无 `session_tag`
- **WHEN** 本轮意图识别得到 `host_apply` 且 `scene_dispatch` 提交为会话标签，Run 结束
- **THEN** 系统将该会话的 `session_tag` 列回写为 `host_apply`，并更新会话解析缓存

#### Scenario: 场景切换后回写新标签

- **GIVEN** 会话进入本轮时 `session_tag=host_apply`
- **WHEN** 本轮在轮次边界切换到 `resource_query`，Run 结束
- **THEN** 系统将该会话的 `session_tag` 列回写为 `resource_query`，并更新会话解析缓存，使会话列表与冷启动读到新标签

#### Scenario: 标签未变更时不回写

- **GIVEN** 会话进入本轮时 `session_tag=host_apply`
- **WHEN** 本轮未发生切换，Run 结束后 checkpoint 中仍为 `host_apply`
- **THEN** 系统不调用 data-service 更新接口

#### Scenario: 回写发生在 Run 结束之后

- **WHEN** 一次带场景切换的 Run 正在输出 SSE
- **THEN** 标签回写不与该 Run 并发执行，而是在 SSE 结束后读取最新 checkpoint 再回写，使标签在同一轮内即完成更新

#### Scenario: 客户端断连时本轮回写落空

- **GIVEN** 客户端在 Run 结束前断开 SSE 连接，或 SSE 写入失败
- **WHEN** `next.ServeHTTP` 提前返回而图仍在后台继续执行
- **THEN** 本轮回写读到的是未完成的 checkpoint，可能不回写或回写旧值；系统 SHALL NOT 为此加入重试等待，该差异由下一轮 Run 结束后的差异回写补齐

#### Scenario: 回写失败不阻断对话

- **WHEN** 回写 `session_tag` 的 data-service 调用失败
- **THEN** 系统记录 Warn 日志，当前会话仍按 checkpoint 中的 `StateKeySessionTag` 继续路由，下一轮 Run 结束后会再次尝试回写
