## ADDED Requirements

### Requirement: 按类型白名单下发 agent 配置

agent-server SHALL 提供 `GET /api/v1/agent/bizs/{bk_biz_id}/config/list?config_type={config_type}`。系统 MUST 校验对该 `bk_biz_id` 的 IAM「业务访问」。系统 MUST 对 `config_type` 做白名单校验；本期白名单 MUST 仅为 `agent_feedback_tag`。不在白名单的值（含 `auth`）MUST 返回 `InvalidParameter`，MUST NOT 返回 `global_config` 行。标签配置是全局的，MUST NOT 按 `bk_biz_id` 过滤配置行。本接口 MUST NOT 要求平台「智能体助手」权限。

`config_type` 缺失 MUST 返回 `InvalidParameter`。查询失败 MUST 返回 `Aborted`，MUST NOT 500 空响应。

#### Scenario: 下发 like 与 dislike 两份 map

- **GIVEN** `global_config` 有 `config_type=agent_feedback_tag` 且 `config_key` 为 `like` 与 `dislike` 的两行
- **WHEN** `GET /api/v1/agent/bizs/123/config/list?config_type=agent_feedback_tag` 且用户有业务访问
- **THEN** `data.details` MUST 含两项，`config_value` 为英文 key 到中文文案的扁平 map

#### Scenario: 拒绝读取凭据类配置

- **GIVEN** `global_config` 存在 `config_type=auth`、`config_key=access_token`
- **WHEN** `GET .../config/list?config_type=auth`
- **THEN** MUST 返回 `InvalidParameter`
- **THEN** 响应 MUST NOT 包含 access_token 内容

#### Scenario: 配置缺失不报错

- **GIVEN** 无任何 `agent_feedback_tag` 行
- **WHEN** 以 `config_type=agent_feedback_tag` 查询
- **THEN** MUST 返回 `code=0` 且 `details` 为空数组

#### Scenario: 无业务访问拒绝

- **WHEN** 用户对业务 123 无业务访问
- **THEN** 调用本接口 MUST 返回 `PermissionDenied`

### Requirement: 提交 tags 按当前 reaction 的配置闭集校验

commit 时，非空 `tags` 的每个元素 MUST 是 `global_config` 中 `config_type=agent_feedback_tag` 且 `config_key` 等于本次 `reaction` 的那份 map 的 key。跨套或未知值 MUST 返回 `InvalidParameter`，MUST NOT 静默丢弃。对应 map 缺失或为空时，非空 `tags` MUST 返回 `InvalidParameter`；空 `tags` 或不传 MUST 允许。`other` MUST 视为普通标签，MUST NOT 强制 `comment`。`config_value` MUST 为扁平 map，MUST NOT 要求分组结构。

#### Scenario: 点赞不能提交点踩标签

- **GIVEN** like map 含 `accurate`，dislike map 含 `factual_error`
- **WHEN** commit `reaction=like` 且 `tags=["factual_error"]`
- **THEN** MUST 返回 `InvalidParameter`，MUST NOT 写库

#### Scenario: map 缺失时只允许空 tags

- **GIVEN** 不存在 `config_key=like` 的 `agent_feedback_tag` 行
- **WHEN** commit `reaction=like` 且不传 `tags`
- **THEN** MUST 允许写入
- **WHEN** commit `reaction=like` 且 `tags=["accurate"]`
- **THEN** MUST 返回 `InvalidParameter`

### Requirement: 预置 agent_feedback_tag 配置

发布 SQL MUST 包含向 `global_config` 写入标签的 `INSERT` 语句，而不仅是建 `aiagent_run_feedback`。MUST 插入两行：`config_type=agent_feedback_tag` + `config_key=like`，以及 `config_type=agent_feedback_tag` + `config_key=dislike`。`config_value` MUST 为扁平 JSON object（英文 key → 中文文案）。`id` MUST 从 `id_generator` 中 `resource=global_config` 的当前 `max_id` 按 8 位 36 进制递增申请，MUST NOT 手写固定 id；插入完成后 MUST 将该 resource 的 `max_id` 更新为本次申请到的最大值。`config_type` + `config_key` MUST 唯一。like / dislike 的 key 集合以产品草案为准（like：`accurate` / `complete` / `professional` / `solved` / `well_formatted` / `other`；dislike：`factual_error` / `reasoning_error` / `incomplete` / `unprofessional` / `calculation_error` / `harmful` / `format_error` / `garbled` / `duplicated` / `chart_expected` / `other`）。系统 MUST NOT 在 `enumor` 写死这些 tag key 作为提交闭集，MUST NOT 仅靠启动时 API 灌数替代该 INSERT。

#### Scenario: 预置后可按 type 列出两行

- **GIVEN** 含 `INSERT INTO global_config` 的迁移 SQL 已执行
- **WHEN** 按 `config_type=agent_feedback_tag` 列出
- **THEN** MUST 恰好得到 `like` 与 `dislike` 两行
- **THEN** like 行 `config_value` MUST 含 `accurate`，dislike 行 MUST 含 `factual_error`
