# cvm-account-select Specification

## Purpose

在 graph 模式 CVM 申领 workflow 启动时，自动完成账号识别与选择：查询当前业务下的账号列表，先按**可用账号数**判定是否还有可选空间（可用数为 0 时路由主图 fallback 给出无权限提示），再按**账号总数**分别处理单账号（自动选择并落库）、多账号（一律弹卡片，可用性仅影响选项禁用态）两种场景，并将最终选定的账号 ID 写入 graph State 后进入 `llm` 节点；自由文本未命中时不下传空账号，由 `select_account` 工具兜底。

## Requirements

### Requirement: 账号列表查询
在 graph 模式 CVM 申领 workflow 启动时，系统 SHALL 向 cloud-server 发起账号查询，获取当前业务（`bk_biz_id`）下 `account_type=resource` 的账号列表，超时时间为 3 秒。`bk_biz_id` 从 `inv.RunOptions.RuntimeState[SessionBkBizIDStateKey]` 读取，内部 client 调用**无需 `bk_ticket`**。

#### Scenario: 查询成功
- **WHEN** `account_select` 节点执行，从 RuntimeState 读取到合法的 `bk_biz_id`
- **THEN** 系统向 `GET /api/v1/cloud/accounts/bizs/{bk_biz_id}?account_type=resource` 发起请求，返回账号列表

#### Scenario: 查询超时或失败
- **WHEN** cloud-server 请求超过 3 秒或返回错误
- **THEN** 系统终止当前 graph 执行并向上层写入错误信息，不影响其他节点

### Requirement: 无可用账号时的无权限处理
当当前业务下**可用账号数为 0** 时（账号列表为空，或有账号但无一支持申领），`account_select` SHALL 自行 emit 无权限提示文案（`NoPermissionFallbackMessage`）并将其作为 assistant 消息写入 graph State，同时把 `AccountSelectNextNodeKey` 置为 `fallback`，子图结束并将控制权交还主图 `fallback`。两种情况复用同一条文案。

文案由 `account_select` 而非主图 fallback 产出：子图配置了 output mapper 后，框架只采用 mapper 返回的 delta（`finalizeAgentNodeOutput`），子图 State 中的 `AccountSelectNextNodeKey` 不会进入主图 State，主图无从据此判定文案；而 messages 本就在 mapper 的合并范围内。主图 fallback SHALL 复用消息尾部的 assistant 回复作为本轮 `last_response`，并因尾部已是 assistant 而跳过重复 emit。

系统 MUST NOT 在无可用账号时弹出账号选择卡片：卡片内全部选项均为禁用态，用户既点不动也退不出，只能在 `account_gate` → `select_account` → `human_confirm` 之间空转。

#### Scenario: 无账号路由 fallback
- **WHEN** 账号列表为空（count == 0）
- **THEN** 系统 SHALL emit 无权限提示并路由至 `fallback`（子图 `graph.End` → 主图 fallback），不进入 `llm`

#### Scenario: 有账号但全部不支持申领时路由 fallback
- **GIVEN** 账号列表非空，但其中没有任何 `vendor == tcloud-ziyan` 的账号（唯一账号为非自研云，或多个账号全为非自研云）
- **WHEN** 进入 `account_select` 且未命中已选账号复用
- **THEN** 系统 SHALL emit 无权限提示并路由至 `fallback`，MUST NOT 触发 `account_select.interrupt`，MUST NOT 写入或持久化 `account_id`

#### Scenario: 提示文案经 messages 带回主图
- **WHEN** `account_select` 因无可用账号结束子图
- **THEN** 该提示 SHALL 以 assistant 消息形式经子图 output mapper 合并进主图 `messages`，主图 fallback 据消息尾部复用同一句文案去 interrupt，用户只收到一条提示

### Requirement: 单账号自动选择
账号选择是否触发用户交互 SHALL 先以**可用账号数**为前置判定（为 0 时按「无可用账号时的无权限处理」路由 `fallback`），在存在可用账号的前提下再以**账号总数**为判定基准：账号总数恰为 1 时（此时该账号必然可用），系统 SHALL 自动将该账号 ID 写入 graph State 的 `account_id` 字段并持久化，无需用户干预、无需模型参与，直接路由至 `llm` 节点；账号总数 ≥ 2 时一律弹卡片请用户选择（可用性判定只影响卡片内选项是否禁用，不改变是否弹卡片）。

#### Scenario: 唯一账号且可用时自动写入
- **WHEN** 账号列表包含且仅包含 1 个账号（count == 1）且该账号 `vendor == tcloud-ziyan`
- **THEN** `state["account_id"]` 被设置为该账号 ID 并落库，路由至 `llm` 节点

#### Scenario: 多账号且存在可用账号时一律弹卡片
- **GIVEN** 账号总数 ≥ 2 且其中至少有 1 个可用账号
- **WHEN** 进入 `account_select` 且未命中已选账号复用
- **THEN** 系统 SHALL 触发 `account_select.interrupt` 弹卡片，卡片展示全部账号，非 `tcloud-ziyan` 账号标记为禁用并附原因

### Requirement: vendor 过滤与 enabled/reason 计算
`ListByUsageBizID` 接口返回的账号列表不含 `enabled`/`reason` 字段。系统 SHALL 在 `account_select` 节点内部根据 vendor 自行计算这两个字段：当前版本仅支持 `vendor = tcloud-ziyan` 的账号，其他 vendor 账号标记为不可用。

账号可用性 SHALL 由**唯一判据** `isAccountEnabled` 给出，路由判定、已选账号复用判定、卡片选项禁用态、`select_account` 工具校验必须共用它；各处若各自判断 vendor，会出现「自动选中 → 下一轮判失效 → 再自动选中」这类互相打架的空转。

#### Scenario: tcloud-ziyan 账号标记为可用
- **WHEN** 账号的 `vendor == "tcloud-ziyan"`
- **THEN** `enabled = true`，`reason = ""`

#### Scenario: 其他 vendor 账号标记为不可用
- **WHEN** 账号的 `vendor != "tcloud-ziyan"`
- **THEN** `enabled = false`，`reason = "当前仅支持自研云（tcloud-ziyan）账号"`

### Requirement: 多账号用户选择
当查询结果包含 2 个及以上账号且其中至少有 1 个可用账号时，系统 SHALL 通过 `account_select.interrupt` 中断请求用户选择，并在 resume 后将所选账号 ID 写入 graph State。`options` 中的 `enabled`/`reason` 由节点内部 vendor 过滤规则计算，不来自接口响应。

#### Scenario: 多账号触发选择中断
- **WHEN** 账号列表包含 2 个或更多账号（count >= 2）且至少有 1 个可用账号
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

### Requirement: graph 拓扑调整
系统 SHALL 将 graph START 入口从 `llm` 改为 `account_select`，确保所有 CVM graph 执行都经过账号识别步骤。

#### Scenario: START 路由至 account_select
- **WHEN** graph 模式 CVM workflow 启动（新对话或初次 invoke）
- **THEN** 第一个执行节点为 `account_select`，而非 `llm`

#### Scenario: 账号确认后进入 llm
- **WHEN** `account_select` 节点成功写入 `account_id`（单账号或用户选择完成）
- **THEN** 下一个执行节点为 `llm`，开始正常 ReAct 循环

### Requirement: interrupt key 常量定义
系统 SHALL 在 `pkg/criteria/constant/aiagent.go` 中定义账号选择相关 interrupt key 常量，与现有 `HITLInterruptKey`、`FallbackInterruptKey` 保持同文件管理。

#### Scenario: 多账号中断使用专属常量
- **WHEN** `account_select` 节点触发多账号选择中断
- **THEN** 使用常量 `AccountSelectInterruptKey = "account_select.interrupt"`

### Requirement: 自由文本未匹配时不下传空账号
在多账号 HITL 场景下，当用户以自由文本 resume（如"选择自研云账号"）且未能解析出 `account_id` 时，系统 MUST NOT 将空 `account_id` 写入 graph State/`delta`（避免污染后续 `tryReuseAccountID` 复用判定）；账号的最终解析与持久化 SHALL 由 `cvm-account-tool-resolution` 能力（`select_account` 工具 + 申领工具门禁）保证。

#### Scenario: 自由文本未命中时不污染状态
- **GIVEN** 多账号 HITL，用户自由文本无法被 `matchAccountIDFromUserInput` 匹配为 `account_id`
- **WHEN** `resolveSelectedAccountID` 返回空
- **THEN** 系统不写入 `delta[account_id]=""`，且账号由 `select_account` 工具与门禁最终结构化接住并落库，使同会话下一轮不再重复弹账号选择

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
- **THEN** `account_select` 按既有逻辑查询业务账号列表，并走无可用账号 fallback / 单账号自动选 / 多账号 HITL，行为不退化

### Requirement: 选中账号持久化失败可观测

系统在经统一 key 调用 `UpdateSessionState` 持久化选中账号失败时，SHALL 记录 Error 级日志（包含完整 session.Key 与 rid），且 MUST NOT 仅因持久化失败中断当轮申领流程（当轮 RuntimeState 已持有账号时可继续）。

#### Scenario: 持久化失败打 Error 且不阻断当轮

- **WHEN** `UpdateSessionState` 返回错误
- **THEN** 日志为 Error 级别并含 session.Key；本轮 graph 不因该错误单独失败（账号已在 RuntimeState）
