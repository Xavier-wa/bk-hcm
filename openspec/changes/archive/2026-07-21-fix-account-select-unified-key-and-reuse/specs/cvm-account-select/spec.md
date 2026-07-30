## ADDED Requirements

### Requirement: 统一 AGUI session key 读写选中账号

系统 SHALL 使用统一的 session.Key 构造规则读写 `SessionSelectedAccountIDStateKey`：`AppName` 取 AGUI 配置应用名，`UserID` 取请求上下文中的蓝鲸用户名（与 AG-UI UserIDResolver 一致），`SessionID` 取当轮 thread / lineage ID（优先 `inv.RunOptions.RuntimeState[CfgKeyLineageID]`，缺失时回退 `inv.Session.ID`）。run 起点回填（`loadSelectedAccountID`）与 `account_select` 节点的 persist / get SHALL 使用同一构造规则，不得仅依赖子图 `inv.Session` 身份字段。

#### Scenario: 写入与读取使用同一 key

- **WHEN** `account_select` 完成账号选择并持久化账号 ID，且后续同一 HTTP 会话的新一轮 Run 在 run 起点或 `account_select` 入口读取已选账号
- **THEN** 两侧定位的 session.Key（AppName/UserID/SessionID）一致，能读到先前写入的 `SessionSelectedAccountIDStateKey`

#### Scenario: 子图 inv.Session 为空仍可定位会话

- **WHEN** 子图节点执行时 `inv.Session` 为 nil，但 `RuntimeState` 含合法 `CfgKeyLineageID`，且上下文含用户名，`appName` 已注入节点
- **THEN** 系统仍能构造有效 session.Key 完成 GetSession / UpdateSessionState，不因 Session 为空而静默跳过落库（空 key 应打错误日志并走无记忆分支）

### Requirement: 双源复用已选账号并跳过选择中断

`account_select` 节点入口 SHALL 通过双源查找已选账号：先读 session 后端的 `SessionSelectedAccountIDStateKey`；若为空再读当轮 `inv.RunOptions.RuntimeState[SessionAccountIDTempKey]`（run 起点可能已回填）。任一非空命中时，系统 SHALL 将该账号写入当轮 RuntimeState，路由至 LLM 节点，且 MUST NOT 触发 `account_select.interrupt` HITL，也 MUST NOT 向用户再次 emit「请先选择云账号」类入口文案。

#### Scenario: 同会话再申领时从后端复用并跳过

- **WHEN** 用户在同一会话完成一次主机申领后再次进入 `host_apply`（例如「再帮我申请一单」），且 session 后端已存在 `SessionSelectedAccountIDStateKey`
- **THEN** `account_select` 复用该账号并路由至 LLM，不出现 `account_select.interrupt` 事件

#### Scenario: 后端暂不可读但 RuntimeState 已回填时仍跳过

- **WHEN** session 后端 GetSession 失败或无该 key，但当轮 `RuntimeState[SessionAccountIDTempKey]` 已被 run 起点回填为非空账号 ID
- **THEN** `account_select` 仍复用该账号并路由至 LLM，不触发账号选择中断

#### Scenario: 双源皆无时走正常选择

- **WHEN** session 后端无已选账号且 RuntimeState 中亦无 `SessionAccountIDTempKey`
- **THEN** `account_select` 按既有逻辑查询业务账号列表，并走零账号 / 单账号自动选 / 多账号 HITL，行为不退化

### Requirement: 选中账号持久化失败可观测

系统在经统一 key 调用 `UpdateSessionState` 持久化选中账号失败时，SHALL 记录 Error 级日志（包含完整 session.Key 与 rid），且 MUST NOT 仅因持久化失败中断当轮申领流程（当轮 RuntimeState 已持有账号时可继续）。

#### Scenario: 持久化失败打 Error 且不阻断当轮

- **WHEN** `UpdateSessionState` 返回错误
- **THEN** 日志为 Error 级别并含 session.Key；本轮 graph 不因该错误单独失败（账号已在 RuntimeState）
