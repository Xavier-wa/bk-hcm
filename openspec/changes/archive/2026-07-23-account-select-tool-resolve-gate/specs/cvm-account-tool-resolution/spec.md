## ADDED Requirements

### Requirement: select_account 工具接住模型确定的账号
系统 SHALL 提供一个面向 LLM 的本地声明工具 `select_account(account_id)`（注册于 `buildSkillTools`，与 `human_confirm`/`skill_load` 同类，不经 tool_proxy）。模型确定账号后调用该工具上报结构化 `account_id`；工具执行时 SHALL 校验该 `account_id` 属于当前 `bk_biz_id` 下 `vendor == tcloud-ziyan` 的可用账号（复用 `listAccountsForBiz` + vendor 过滤），校验通过后 SHALL 复用 `injectAccountIDToRuntimeState` 将账号写入当轮 `RuntimeState[SessionAccountIDTempKey]` 并持久化到 session 后端（`SessionSelectedAccountIDStateKey`）。

#### Scenario: 上报合法账号后注入并落库
- **GIVEN** 当前业务下 `ziyan-hcm-test`（`tcloud-ziyan`）为可用账号
- **WHEN** 模型调用 `select_account("0000002b")`
- **THEN** 系统校验通过，写入 `RuntimeState[SessionAccountIDTempKey]="0000002b"` 并落库，返回成功结果

#### Scenario: 上报非法/不可用账号被拒绝
- **WHEN** 模型调用 `select_account` 传入的 `account_id` 不属于本业务可用（`tcloud-ziyan`）账号
- **THEN** 系统 SHALL 返回可读错误、MUST NOT 写入 `RuntimeState`、MUST NOT 落库

#### Scenario: 上报后同会话下一轮复用不再弹选择
- **GIVEN** 上一轮 `select_account` 已成功落库该账号
- **WHEN** 同一会话再次进入 host_apply（新一轮 Run）
- **THEN** `tryReuseAccountID` 从 session 后端命中该账号并路由至 `llm`，不触发 `account_select.interrupt`

### Requirement: 申领类工具的场景级账号前置门禁
系统 SHALL 在 host_apply 场景的 LLM 节点 `BeforeTool` 回调中施加账号前置门禁（不在跨场景公共基础设施 `toolproxy` 内实现）。门禁 SHALL 使用既有 `toolproxy.ResolveToolCall`（或等价解开 `execute_tool` 信封）取真实工具名，并用 `agent.InvocationFromContext` 读取账号：当目标工具属于受控集合（`enumor.ToolName.IsRecommend()` 为真的推荐类，或提单工具 `create_biz_apply`）且 `RuntimeState[SessionAccountIDTempKey]` 为空时，系统 SHALL 返回 `BeforeToolResult.CustomResult` 硬拦截执行并引导先调用 `select_account`。受控集合 MUST NOT 包含只读查询工具。

#### Scenario: 未选账号调用申领工具被拦截
- **GIVEN** 当轮 `RuntimeState[SessionAccountIDTempKey]` 为空
- **WHEN** 模型经 `execute_tool` 调用 `get_biz_apply_recommend_by_static`
- **THEN** 门禁 `BeforeTool` 返回 `CustomResult` 引导先调用 `select_account`，不实际执行该业务工具

#### Scenario: 已选账号调用申领工具放行
- **GIVEN** 当轮 `RuntimeState[SessionAccountIDTempKey]` 非空
- **WHEN** 模型经 `execute_tool` 调用受控申领工具
- **THEN** 门禁放行，工具正常执行（提单工具仍由 `createCvmApplyGate` 负责人工确认）

#### Scenario: 查询类工具不受门禁影响
- **GIVEN** 当轮尚未解析账号
- **WHEN** 模型经 `execute_tool` 调用只读查询工具（如 `list_config_cvm_device`）
- **THEN** 门禁放行，工具正常执行

#### Scenario: 与提单确认门禁不冲突
- **GIVEN** 账号已解析（`RuntimeState[account_id]` 非空）
- **WHEN** 模型调用提单工具 `create_biz_apply`
- **THEN** 账号门禁放行，仅由 `createCvmApplyGate` 触发一次人工确认，不产生双重拦截

#### Scenario: 门禁自愈闭环
- **WHEN** 模型因未选账号被门禁拦截并收到引导错误
- **THEN** 模型据错补调 `select_account` 完成账号解析后，再次调用申领工具即被放行
