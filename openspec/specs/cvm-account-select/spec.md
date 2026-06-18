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
