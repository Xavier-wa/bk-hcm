## Why

CVM 主机申领 workflow（graph 模式）在流程开始时没有提前确认用户的云账号权限。当用户在当前业务下无任何账号权限或同时拥有多个账号时，后续申领节点将必然失败或产生歧义。本期在 graph 入口插入账号自动识别节点，提前处理账号获取与选择，确保申领流程可以正常执行。

## What Changes

- 新增 `account_select` Function Node：查询当前用户在指定业务（`bk_biz_id`）下的云账号列表，并按账号数量（0/1/N）执行对应路由逻辑
- 新增 `no_permission_fallback` Function Node：当账号数量为 0 时触发 `no_permission.interrupt` 中断，resume 后回到 `account_select` 节点重试
- `account_select` 节点在账号数量 >= 2 时触发 `account_select.interrupt` 中断，等待用户选择后写入 `account_id` 状态字段，再路由至 `llm`
- 账号数量恰好为 1 时自动选择，直接写入 `account_id` 路由到 `llm`
- 新增 graph State 字段 `account_id`（string），供下游节点消费
- 新增两个 interrupt key 常量：`NoPermissionInterruptKey`、`AccountSelectInterruptKey`
- 修改 graph 拓扑：将 START 入口从 `llm` 改为 `account_select`，账号确认完成后路由到 `llm`；`no_permission_fallback` 循环回 `account_select`

## Capabilities

### New Capabilities

- `cvm-account-select`：CVM 申领账号自动识别能力，包含账号列表查询、0/1/N 分支路由、两种 HITL interrupt 协议（`no_permission.interrupt` / `account_select.interrupt`）以及选中账号写入 graph State 的完整流程

### Modified Capabilities

（无）

## Impact

- **新增文件**：`cmd/agent-server/logics/agent/cvm/account_select.go`
- **修改文件**：`cmd/agent-server/logics/agent/graph_build.go`（注册新节点、修改边配置、增加 `*cloudserver.Client` 参数）
- **修改文件**：`cmd/agent-server/logics/runtime.go`（`newAGUIRunner` 传入 `clientSet.CloudServer()`）
- **修改文件**：`pkg/criteria/constant/aiagent.go`（新增两个 interrupt key 常量）
- **依赖接口**：`GET /api/v1/cloud/accounts/bizs/{bk_biz_id}?account_type=resource`（cloud-server `ListByUsageBizID` handler，已存在）
- **凭据依赖**：从 graph State 读取 `bk_ticket` 并注入 HTTP 请求上下文（复用现有 `auth.BKTicketFromContext` 机制）
- **无破坏性变更**：现有节点（`hitl`、`tool`、`fallback`）行为不变，仅调整 graph 入口节点
