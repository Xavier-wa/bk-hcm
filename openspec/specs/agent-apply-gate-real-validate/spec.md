# Capability: agent-apply-gate-real-validate

## Purpose

定义 agent-server 申领门禁（`createCvmApplyGate`）在确认后调用 woa-server 提单前只读校验接口的真实前置校验，替换占位实现，并规范 path_param 提取与 woa 客户端依赖注入。

## Requirements

### Requirement: 申领门禁执行真实前置校验

agent-server 申领门禁（`createCvmApplyGate`，守护工具 `create_biz_apply`）SHALL 在用户在确认卡片点击确认后、放行到 tool 节点执行真实提单之前，调用 woa-server 的提单前只读校验接口执行真实校验，替换原占位实现（直接返回通过）。校验 MUST 只读、无副作用。

`validateApply` SHALL 依据校验结果返回三态 `(ok, reason, err)`，并由门禁映射为：

- `err != nil` → 拒绝放行，向用户提示系统异常（可重试）。
- `!ok`（业务不通过）→ 拒绝放行，向用户回显业务原因并建议调整规格或数量。
- `ok` → 放行到 tool 节点执行真实提单。

#### Scenario: 校验通过放行提单

- **GIVEN** 用户在确认卡片点击确认，且 woa 校验接口返回 `pass=true`
- **WHEN** `createCvmApplyGate.OnResume` 执行 `validateApply`
- **THEN** 门禁路由到 tool 节点，执行真实 `create_biz_apply` 提单

#### Scenario: 业务校验不通过回退

- **GIVEN** woa 校验接口返回 `pass=false` 且携带原因
- **WHEN** `createCvmApplyGate.OnResume` 执行 `validateApply`
- **THEN** 门禁不提单，路由回 llm 节点，向用户回显"申领前置校验未通过：<原因>"并建议调整后重试

#### Scenario: 系统异常不放行

- **GIVEN** woa 校验接口返回 error（系统异常）
- **WHEN** `createCvmApplyGate.OnResume` 执行 `validateApply`
- **THEN** 门禁不提单，向用户提示前置校验失败、暂时无法提单

### Requirement: 从工具调用 path_param 提取 bk_biz_id

MCP 工具调用参数以 `path_param` 与 `body_param` 两个并列对象包裹，门禁原仅解析 `body_param`。系统 SHALL 新增从 `args["path_param"]["bk_biz_id"]` 提取 `bk_biz_id`，并随 `body_param` 一起构造 `ApplyReq` 传入 woa 校验接口。提取 MUST 兼容 JSON number（`float64`）与字符串两种形式。由于 `bk_biz_id` 是真实提单的 path 必填项，到确认后阶段必然存在；若仍取不到，系统 MUST 按系统异常返回明确报错，MUST NOT 静默放行。

#### Scenario: 正常提取 bk_biz_id

- **GIVEN** 工具调用 args 含 `path_param.bk_biz_id`
- **WHEN** 门禁执行前置校验
- **THEN** 使用该 `bk_biz_id` 调用 woa 业务视角校验接口

#### Scenario: bk_biz_id 缺失防御

- **GIVEN** 工具调用 args 中无法解析出 `bk_biz_id`
- **WHEN** 门禁执行前置校验
- **THEN** 返回系统异常错误（无法确定业务），不放行提单

### Requirement: woa 客户端依赖注入

agent-server SHALL 通过依赖注入让 `createCvmApplyGate` 持有 woa-server 客户端：`clientSet` 经 `runtime.New → BuildGraph → toolgate.EnabledGateHandlers → newCreateCvmApplyGate` 透传，门禁经 `client.WoaServer().Task.CheckBizApplyOrder` 调用校验接口。MUST NOT 使用全局/包级客户端。`pkg/client/woa-server/task.go` SHALL 新增 `CheckBizApplyOrder` 客户端方法。

#### Scenario: 门禁经注入的客户端调用校验

- **GIVEN** `clientSet` 已透传至 `createCvmApplyGate`
- **WHEN** 门禁执行 `validateApply`
- **THEN** 经注入的 woa 客户端 `Task.CheckBizApplyOrder` 发起校验请求，请求头透传当前用户用于业务鉴权
