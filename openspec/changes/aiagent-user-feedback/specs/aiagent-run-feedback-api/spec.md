## ADDED Requirements

### Requirement: 提交反馈按 run_id 覆盖写

agent-server SHALL 提供 `POST /api/v1/agent/bizs/{bk_biz_id}/feedback/commit`。系统 MUST 校验调用者对该 `bk_biz_id` 具备 IAM「业务访问」（`biz_access`）。系统 MUST 按 `run_id` upsert：无行则插入，有行则更新同一行，MUST NOT 再插一条。请求体 MUST 含 `session_id`、`run_id`、`reaction`（仅 `like` / `dislike`）；`tags`、`comment` 可选。请求体 MUST NOT 要求 `bk_biz_id`。`session_id` MUST 对应会话属于当前用户且 `bk_biz_id` 等于 path 值，否则 MUST 返回 `RecordNotFound` 或 `PermissionDenied`。响应成功时 `data.id` 为该行主键。

本期 MUST NOT 依赖 `aiagent_run` 终态校验。系统 MAY 在后续补上「run 存在且 status 为 finished/error」的校验，但不是当前交付要求。

`comment` 超过 500 个 Unicode 字符 MUST 返回 `InvalidParameter`。允许只传 `session_id` + `run_id` + `reaction`。

改判（已有行的 `reaction` 与本次不同）MUST 清空旧 `tags` 与 `comment` 后再写入本次字段。同态度再次提交 MUST 保留行 `id`，只更新 `tags` / `comment` / `reaction`（值不变）与审计字段。

入库 MUST 将 `reaction` 原样写入库列 `reaction`（`like` / `dislike`），MUST NOT 再映射为 0/1。

#### Scenario: 首次提交仅态度

- **GIVEN** 用户对业务 123 有业务访问，`session_id=s1` 属于该用户、`bk_biz_id=123`，`run_id=r1` 尚无反馈行
- **WHEN** 调用 `POST /api/v1/agent/bizs/123/feedback/commit`，body `{ "session_id": "s1", "run_id": "r1", "reaction": "like" }`
- **THEN** 系统 MUST 插入一行，`reaction=like`，`tags` 为空，`comment` 为空，并返回该行 `id`

#### Scenario: 已有反馈再提交则覆盖同一行

- **GIVEN** `run_id=r1` 已有反馈 `id=f1`，`reaction=dislike`
- **WHEN** 再次 commit `{ "session_id": "s1", "run_id": "r1", "reaction": "dislike", "tags": ["incomplete"], "comment": "缺字段" }`
- **THEN** 系统 MUST 仍只有一行，`id` 仍为 `f1`，`tags` 含 `incomplete`，`comment` 为该文本

#### Scenario: 改判清空旧归因

- **GIVEN** `run_id=r1` 已有 `reaction=like`、`tags=["accurate"]`、`comment="很好"`
- **WHEN** commit `{ "session_id": "s1", "run_id": "r1", "reaction": "dislike", "tags": ["factual_error"] }`
- **THEN** 该行 MUST `reaction=dislike`，`tags` MUST 仅为 `factual_error`，`comment` MUST 为空
- **THEN** MUST NOT 保留 `accurate`

#### Scenario: 无业务访问拒绝提交

- **GIVEN** 用户对业务 123 无业务访问
- **WHEN** 调用 commit
- **THEN** 系统 MUST 返回 `PermissionDenied`，错误信息 MUST 包含 `bk_biz_id`

#### Scenario: 他人会话拒绝提交

- **GIVEN** `session_id=s1` 属于 bob，调用者为 `alice`
- **WHEN** alice commit `{ "session_id": "s1", "run_id": "r1", "reaction": "like" }`
- **THEN** 系统 MUST 返回 `PermissionDenied`，MUST NOT 写库

### Requirement: 撤销反馈

agent-server SHALL 提供 `DELETE /api/v1/agent/bizs/{bk_biz_id}/feedback/{run_id}?session_id={session_id}`。系统 MUST 校验业务访问。`session_id` 必填，MUST 对应会话属于当前用户、`bk_biz_id` 等于 path 值，否则 MUST 返回 `RecordNotFound` 或 `PermissionDenied`。系统 MUST 只删除当前用户、当前业务、且 `session_id` 与该行一致的反馈。行不存在 MUST 返回 `RecordNotFound`。属于其他用户、其他业务或其他会话 MUST 返回 `PermissionDenied`，MUST NOT 删除。

#### Scenario: 撤销自己的反馈

- **GIVEN** 当前用户在业务 123、会话 `s1` 对 `run_id=r1` 已有反馈
- **WHEN** `DELETE /api/v1/agent/bizs/123/feedback/r1?session_id=s1`
- **THEN** 该行 MUST 被删除，响应 `data` 为 null

#### Scenario: 不能撤销他人反馈

- **GIVEN** `run_id=r1` 的反馈 `user=bob`、`session_id=s1`
- **WHEN** alice 带 `session_id=s1` 删除该 `run_id`
- **THEN** MUST 返回 `PermissionDenied`，行 MUST 仍在

#### Scenario: session_id 与反馈行不一致拒绝删除

- **GIVEN** `run_id=r1` 的反馈属于会话 `s1`，调用者对该会话有权
- **WHEN** 带 `session_id=s2` 删除
- **THEN** MUST 返回 `PermissionDenied`，行 MUST 仍在
