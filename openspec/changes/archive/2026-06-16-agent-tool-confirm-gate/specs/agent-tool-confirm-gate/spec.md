## ADDED Requirements

### Requirement: 工具调用前强制确认门禁

系统 SHALL 在 Agent 图中提供通用的工具调用前确认门禁节点（`tool_gate`），置于 `llm` 与 `tool` 之间。当 LLM 产生的 tool_calls 中包含被注册为受门禁的工具时，系统 SHALL 强制路由到 `tool_gate` 并触发 `graph.Interrupt` 中断，确认行为不依赖 LLM 自身决策。

#### Scenario: 受门禁工具触发强制中断

- **GIVEN** `create_biz_apply` 已注册为受门禁工具
- **WHEN** LLM 在最近一条 assistant 消息中产生 `create_biz_apply` 的 tool_call
- **THEN** 系统路由到 `tool_gate` 节点并触发中断，等待用户确认，不直接执行 `create_biz_apply`

#### Scenario: 非受门禁工具不进入门禁

- **GIVEN** 某工具未注册为受门禁工具
- **WHEN** LLM 产生该工具的 tool_call
- **THEN** 系统按原路由直接进入 `tool` 节点执行

#### Scenario: 受门禁工具必须单独成批

- **WHEN** LLM 在同一批 tool_calls 中同时调用受门禁工具与其他工具（或 `human_confirm`）
- **THEN** 系统 SHALL 返回错误，不执行任何工具

### Requirement: 结构化确认事件下发

系统 SHALL 在 `tool_gate` 中断时，通过中断 key 前缀 `tool_confirm:` 触发名为 `tool.confirm` 的自定义事件，向前端下发结构化确认内容。确认内容 SHALL 包含工具名、标题、摘要、只读展示字段（`fields`）与可编辑的原始工具入参（`args`）。

#### Scenario: 申领确认卡片下发

- **GIVEN** LLM 调用 `create_biz_apply` 触发门禁
- **WHEN** `tool_gate` 节点构造确认内容并中断
- **THEN** 系统 emit `tool.confirm` 事件，value 含 `tool`、`title`、`summary`、`fields`、`args`，其中 `args` 为剥离 `body_param` 包装后的原始申领入参

#### Scenario: 中断 key 唯一可恢复

- **WHEN** 同一会话先后两次触发受门禁工具
- **THEN** 两次中断 key 包含各自 tool_call ID，互不冲突，可分别正确 resume

### Requirement: 用户确认与参数修改

系统 SHALL 接收前端回传的结构化确认结果 `{action, args?}`。`action` 为 `confirm` 时放行执行，为 `cancel` 时拒绝执行。当回传包含 `args` 时，系统 SHALL 以该参数覆盖待执行 tool_call 的入参后再执行。

#### Scenario: 用户确认且未修改参数

- **WHEN** 前端回传 `{"action":"confirm"}` 且不含 `args`
- **THEN** 系统使用 LLM 原始 tool_call 入参，校验通过后执行 `create_biz_apply`

#### Scenario: 用户修改参数后确认

- **GIVEN** 用户在确认卡片上修改了申领数量
- **WHEN** 前端回传 `{"action":"confirm","args":{...修改后的完整入参...}}`
- **THEN** 系统用修改后的入参覆盖 assistant 消息中对应 tool_call 的 Arguments，并以修改后的入参执行

#### Scenario: 用户取消

- **WHEN** 前端回传 `{"action":"cancel"}`
- **THEN** 系统不执行工具，补一条 tool 结果消息说明已取消，并回到 `llm` 由其向用户解释

### Requirement: 确认后前置校验

系统 SHALL 在用户确认后、执行真实工具前，执行该工具专属的前置校验 hook（`OnConfirm`）。校验通过 SHALL 放行执行；校验不通过或确认被取消 SHALL 不执行工具，补一条 tool 结果消息并回到 `llm` 解释原因。`create_biz_apply` 的库存/预测校验本期 SHALL 以占位函数实现并返回通过，后续接入真实接口。

#### Scenario: 校验通过执行提单

- **GIVEN** 用户确认申领
- **WHEN** `OnConfirm` 的库存/预测校验返回通过
- **THEN** 系统不插入额外用户消息，保留 tool_call 后路由到 `tool` 执行 `create_biz_apply`

#### Scenario: 校验不通过阻断提单

- **GIVEN** 用户确认申领
- **WHEN** `OnConfirm` 校验返回不通过
- **THEN** 系统不执行 `create_biz_apply`，补一条说明原因的 tool 结果消息并回到 `llm`

#### Scenario: 占位校验默认放行

- **GIVEN** 库存/预测校验尚未接入真实接口
- **WHEN** 用户确认申领
- **THEN** 占位校验函数返回通过，系统按确认流程执行

### Requirement: 可扩展的 per-tool 门禁机制

系统 SHALL 通过 `Gate` 接口（`ToolName`/`BuildConfirm`/`OnConfirm`）与注册表提供扩展机制。新增受门禁工具 SHALL 仅需实现该接口并注册，无需修改图结构或通用 `tool_gate` 节点逻辑。

#### Scenario: 注册新受门禁工具

- **WHEN** 为某新工具实现 `Gate` 接口并注册到注册表
- **THEN** 该工具自动受门禁保护，确认卡片内容与确认后逻辑由其 `Gate` 实现决定，通用节点与路由代码无需改动

### Requirement: 门禁配置开关

受门禁工具集 SHALL 以代码注册表为事实源（注册了 `Gate` 实现即受保护）。系统 SHALL 支持可选 yaml 配置 `tools.confirmGate`（`enabled` 与 `tools` 列表）叠加于注册表之上：路由激活判定为"已注册 AND 配置启用 AND（列表为空或含该工具名）"。配置启用但未注册的工具 SHALL 按未启用处理且不报错。

#### Scenario: 配置启用列表为空启用全部已注册门禁

- **GIVEN** `tools.confirmGate.enabled` 为 true 且 `tools` 为空
- **WHEN** 任一已注册受门禁工具被 LLM 调用
- **THEN** 该工具进入门禁

#### Scenario: 配置仅启用列表内工具

- **GIVEN** `tools.confirmGate.enabled` 为 true 且 `tools` 仅含 `create_biz_apply`
- **WHEN** 另一已注册受门禁工具（不在列表内）被调用
- **THEN** 该工具不进入门禁，按原路由执行

#### Scenario: 紧急关闭门禁

- **GIVEN** `tools.confirmGate.enabled` 为 false
- **WHEN** 任一受门禁工具被调用
- **THEN** 门禁不生效，恢复原有路由行为

#### Scenario: 配置启用但未注册不报错

- **GIVEN** `tools` 含一个未注册 `Gate` 实现的工具名
- **WHEN** 系统启动并运行
- **THEN** 系统不报错，该工具按未启用门禁处理

### Requirement: 与泛化确认互斥

受门禁工具 SHALL 从泛化 `confirmToolSet` 的确认拦截中排除，避免对同一工具产生门禁卡片与泛化文本的双重确认。

#### Scenario: 受门禁工具不触发泛化文本确认

- **GIVEN** `create_biz_apply` 受 `tool_gate` 门禁保护
- **WHEN** 该工具被调用
- **THEN** 泛化 `confirmToolSet` 不再对其追加文本确认提示
