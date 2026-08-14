# account-select-session-persistence Specification

## Purpose

主机申领 `account_select` 将选中账号持久化到 session 后端，并在同会话后续 Run 回填 / 复用，避免重复弹出账号选择中断。

## Requirements

### Requirement: 选中账号持久化到 session 后端
主机申领子图在 `account_select` 节点完成账号选择（单选自动选择或多选由用户选定）后，系统 SHALL 将选中的账号 ID
可靠持久化到 session 后端（数据库），跨 HTTP Run 可读取。

#### Scenario: 单选业务自动选中账号后落库
- **WHEN** `account_select` 检测到该业务仅有一个可用云账号并自动选中
- **THEN** 系统将该账号 ID 写入 session 后端（经显式注入的 `session.Service.UpdateSessionState`，key 为 `SessionSelectedAccountIDStateKey`），且当轮 `inv.RunOptions.RuntimeState[SessionAccountIDTempKey]` 被置为该账号 ID

#### Scenario: 多选业务用户选定账号后落库
- **WHEN** `account_select` 触发账号选择中断，用户经前端选定时传入 `forwardedProps.resumeValue`（账号 ID）
- **THEN** 系统将该账号 ID 写入 session 后端，且当轮 `inv.RunOptions.RuntimeState[SessionAccountIDTempKey]` 被置为该账号 ID

### Requirement: 重新提单时复用已选账号并跳过选择中断
在同一会话内完成一次提单、再次进入 `host_apply` 子图时，系统 SHALL 读取已持久化的账号，跳过账号选择中断，直接路由到后续节点。

#### Scenario: 同一会话重新提单不再弹账号选择
- **WHEN** 用户在同一会话（相同 session ID）完成一次提单后，再次发起主机申领意图（如「重新提单」），且 session 后端已存在 `SessionSelectedAccountIDStateKey`
- **THEN** `account_select` 读取到已选账号、将其写入当轮 `RuntimeState[SessionAccountIDTempKey]`，并路由到 LLM 节点，不触发账号选择 HITL 中断

#### Scenario: 后端无账号时仍正常走选择
- **WHEN** session 后端不存在 `SessionSelectedAccountIDStateKey`（首次进入或历史无记忆）
- **THEN** `account_select` 按现状查询账号列表并走单选/多选逻辑，行为不退化

### Requirement: 每轮 run 起点回填账号到 RuntimeState
父图在每轮 run 起点重建 `inv.RunOptions.RuntimeState` 时，系统 SHALL 从 session 后端读取已选账号（若存在）并回填到 `RuntimeState[SessionAccountIDTempKey]`，使当轮 instruction_prompt 从最早一次渲染起即带账号。

#### Scenario: 启动即带账号的 prompt
- **WHEN** 每轮 run 起点 `makeRunOptionResolver`/`tryPrepareAutoResume` 重建 RuntimeState，且 session 后端已存在该账号
- **THEN** `RuntimeState[SessionAccountIDTempKey]` 被置为该账号 ID，供 `{temp:account_id?}` 占位符渲染，与 `SessionBkBizIDStateKey` 注入方式对称

#### Scenario: 后端无账号时安全跳过回填
- **WHEN** 每轮 run 起点 session 后端不存在该账号
- **THEN** 不写入 `SessionAccountIDTempKey`（由 `account_select` 逻辑照常处理），不报错、不强制
