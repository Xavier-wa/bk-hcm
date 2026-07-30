# cvm-account-select Specification

## Purpose

在 graph 模式 CVM 申领 workflow 启动时，自动完成账号识别与选择：查询当前业务下的可用账号，根据账号数量分别处理零账号（无权限中断）、单账号（自动选择）、多账号（用户选择中断）三种场景，并将最终选定的账号 ID 写入 graph State 后进入 `llm` 节点，确保所有 CVM graph 执行都经过账号识别步骤。

## Requirements

### Requirement: 账号列表查询
在 graph 模式 CVM 申领 workflow 启动时，系统 SHALL 向 cloud-server 发起账号查询，获取当前业务（`bk_biz_id`）下 `account_type=resource` 的账号列表，超时时间为 3 秒。`bk_biz_id` 从 `inv.RunOptions.RuntimeState[SessionBkBizIDStateKey]` 读取，内部 client 调用**无需 `bk_ticket`**。

#### Scenario: 查询成功
- **WHEN** `account_select` 节点执行，从 RuntimeState 读取到合法的 `bk_biz_id`
- **THEN** 系统向 `GET /api/v1/cloud/accounts/bizs/{bk_biz_id}?account_type=resource` 发起请求，返回账号列表

#### Scenario: 查询超时或失败
- **WHEN** cloud-server 请求超过 3 秒或返回错误
- **THEN** 系统终止当前 graph 执行并向上层写入错误信息，不影响其他节点

### Requirement: 零账号无权限处理
当查询结果为空（账号数量为 0）时，系统 SHALL 通过 `no_permission_fallback` 节点触发 `no_permission.interrupt` 中断，告知用户当前业务下无可用账号权限。

#### Scenario: 无账号触发中断
- **WHEN** 账号列表为空（count == 0）
- **THEN** 路由至 `no_permission_fallback` 节点，发出如下中断载荷：
  ```json
  {
    "type": "no_permission.interrupt",
    "message": "当前用户在该业务下没有可用的账号权限，无法继续主机申领流程"
  }
  ```

#### Scenario: 用户确认后重试
- **WHEN** 前端回传任意字符串 resume value
- **THEN** graph 从 `no_permission_fallback` 路由回 `account_select` 节点重新查询账号

### Requirement: 单账号自动选择
当查询结果恰好有 1 个账号时，系统 SHALL 自动将该账号 ID 写入 graph State 的 `account_id` 字段，无需用户干预，直接路由至 `llm` 节点。

#### Scenario: 单账号自动写入
- **WHEN** 账号列表包含且仅包含 1 个账号（count == 1）
- **THEN** `state["account_id"]` 被设置为该账号的 ID，并路由至 `llm` 节点继续执行

### Requirement: vendor 过滤与 enabled/reason 计算
`ListByUsageBizID` 接口返回的账号列表不含 `enabled`/`reason` 字段。系统 SHALL 在 `account_select` 节点内部根据 vendor 自行计算这两个字段：当前版本仅支持 `vendor = tcloud-ziyan` 的账号，其他 vendor 账号标记为不可用。

#### Scenario: tcloud-ziyan 账号标记为可用
- **WHEN** 账号的 `vendor == "tcloud-ziyan"`
- **THEN** `enabled = true`，`reason = ""`

#### Scenario: 其他 vendor 账号标记为不可用
- **WHEN** 账号的 `vendor != "tcloud-ziyan"`
- **THEN** `enabled = false`，`reason = "当前仅支持自研云（tcloud-ziyan）账号"`

### Requirement: 多账号用户选择
当查询结果包含 2 个及以上账号时，系统 SHALL 通过 `account_select.interrupt` 中断请求用户选择，并在 resume 后将所选账号 ID 写入 graph State。`options` 中的 `enabled`/`reason` 由节点内部 vendor 过滤规则计算，不来自接口响应。

#### Scenario: 多账号触发选择中断
- **WHEN** 账号列表包含 2 个或更多账号（count >= 2）
- **THEN** 系统发出如下中断载荷：
  ```json
  {
    "type": "account_select.interrupt",
    "message": "检测到多个可用账号，请选择用于本次申领的账号",
    "options": [
      {
        "account_id": "<id>",
        "account_name": "<name>",
        "vendor": "<vendor>",
        "enabled": true,
        "reason": ""
      }
    ]
  }
  ```

#### Scenario: 用户选择账号后写入 State
- **WHEN** 前端回传选中的 `account_id` 字符串（非空）作为 resume value
- **THEN** `state["account_id"]` 被设置为该值，并路由至 `llm` 节点继续执行

#### Scenario: resume value 为空时拒绝
- **WHEN** 前端回传的 resume value 为空字符串
- **THEN** 系统拒绝继续，返回错误，不写入 `account_id`

### Requirement: graph 拓扑调整
系统 SHALL 将 graph START 入口从 `llm` 改为 `account_select`，确保所有 CVM graph 执行都经过账号识别步骤。

#### Scenario: START 路由至 account_select
- **WHEN** graph 模式 CVM workflow 启动（新对话或初次 invoke）
- **THEN** 第一个执行节点为 `account_select`，而非 `llm`

#### Scenario: 账号确认后进入 llm
- **WHEN** `account_select` 节点成功写入 `account_id`（单账号或用户选择完成）
- **THEN** 下一个执行节点为 `llm`，开始正常 ReAct 循环

### Requirement: interrupt key 常量定义
系统 SHALL 在 `pkg/criteria/constant/aiagent.go` 中定义两个新的 interrupt key 常量，与现有 `HITLInterruptKey`、`FallbackInterruptKey` 保持同文件管理。

#### Scenario: 常量可被节点引用
- **WHEN** `no_permission_fallback` 节点触发中断
- **THEN** 使用常量 `NoPermissionInterruptKey = "no_permission.interrupt"`

#### Scenario: 多账号中断使用专属常量
- **WHEN** `account_select` 节点触发多账号选择中断
- **THEN** 使用常量 `AccountSelectInterruptKey = "account_select.interrupt"`

### Requirement: 统一 AGUI session key 读写选中账号

系统 SHALL 使用统一的 session.Key 构造规则读写 `SessionSelectedAccountIDStateKey`：`AppName` 取 AGUI 配置应用名，`UserID` 取请求上下文中的蓝鲸用户名（与 AG-UI UserIDResolver 一致，子图 ctx 缺失时可回退 `inv.Session.UserID`），`SessionID` 取当轮 thread / lineage ID（优先 `inv.RunOptions.RuntimeState[CfgKeyLineageID]`，缺失时回退 `inv.Session.ID`）。run 起点回填（`loadSelectedAccountID`）与 `account_select` 节点的 persist / get SHALL 使用同一构造规则（`BuildAGUISessionKey` / `BuildAGUISessionKeyWithUser`），不得仅依赖子图 `inv.Session` 身份字段。

#### Scenario: 写入与读取使用同一 key

- **WHEN** `account_select` 完成账号选择并持久化账号 ID，且后续同一 HTTP 会话的新一轮 Run 在 run 起点或 `account_select` 入口读取已选账号
- **THEN** 两侧定位的 session.Key（AppName/UserID/SessionID）一致，能读到先前写入的 `SessionSelectedAccountIDStateKey`

#### Scenario: 子图 inv.Session 为空仍可定位会话

- **WHEN** 子图节点执行时 `inv.Session` 为 nil，但 `RuntimeState` 含合法 `CfgKeyLineageID`，且上下文含用户名，`appName` 已注入节点
- **THEN** 系统仍能构造有效 session.Key 完成 GetSession / UpdateSessionState，不因 Session 为空而静默跳过落库（空 key 应打错误日志并走无记忆分支）

### Requirement: 三源复用已选账号并跳过选择中断

`account_select` 节点入口 SHALL 通过三源查找已选账号（`tryReuseAccountID`）：先读 session 后端的 `SessionSelectedAccountIDStateKey`；若为空再读当轮 `inv.RunOptions.RuntimeState[SessionAccountIDTempKey]`（run 起点可能已回填）；若仍为空再读 graph `State[SessionAccountIDTempKey]`（同轮 host_apply 子图 InputMapper 从父图拷贝）。任一非空命中时，系统 SHALL 将该账号写入当轮 RuntimeState，路由至 LLM 节点，且 MUST NOT 触发 `account_select.interrupt` HITL，也 MUST NOT 向用户再次 emit「请先选择云账号」类入口文案。

#### Scenario: 同会话再申领时从后端复用并跳过

- **WHEN** 用户在同一会话完成一次主机申领后再次进入 `host_apply`（例如「再帮我申请一单」），且 session 后端已存在 `SessionSelectedAccountIDStateKey`
- **THEN** `account_select` 复用该账号并路由至 LLM，不出现 `account_select.interrupt` 事件

#### Scenario: 后端暂不可读但 RuntimeState 已回填时仍跳过

- **WHEN** session 后端 GetSession 失败或无该 key，但当轮 `RuntimeState[SessionAccountIDTempKey]` 已被 run 起点回填为非空账号 ID
- **THEN** `account_select` 仍复用该账号并路由至 LLM，不触发账号选择中断

#### Scenario: graph State 已有 account_id 时仍跳过

- **WHEN** session 后端与 RuntimeState 皆无账号，但 graph `State[SessionAccountIDTempKey]` 非空（同轮父图已带入）
- **THEN** `account_select` 仍复用该账号并路由至 LLM，不触发账号选择中断

#### Scenario: 三源皆无时走正常选择

- **WHEN** session 后端、RuntimeState 与 graph State 均无已选账号
- **THEN** `account_select` 按既有逻辑查询业务账号列表，并走零账号 / 单账号自动选 / 多账号 HITL，行为不退化

### Requirement: 选中账号持久化失败可观测

系统在经统一 key 调用 `UpdateSessionState` 持久化选中账号失败时，SHALL 记录 Error 级日志（包含完整 session.Key 与 rid），且 MUST NOT 仅因持久化失败中断当轮申领流程（当轮 RuntimeState 已持有账号时可继续）。

#### Scenario: 持久化失败打 Error 且不阻断当轮

- **WHEN** `UpdateSessionState` 返回错误
- **THEN** 日志为 Error 级别并含 session.Key；本轮 graph 不因该错误单独失败（账号已在 RuntimeState）
