## 1. 常量定义

- [x] 1.1 在 `pkg/criteria/constant/aiagent.go` 新增 `NoPermissionInterruptKey = "no_permission.interrupt"` 常量
- [x] 1.2 在 `pkg/criteria/constant/aiagent.go` 新增 `AccountSelectInterruptKey = "account_select.interrupt"` 常量

## 2. account_select 节点实现

- [x] 2.1 创建目录 `cmd/agent-server/logics/agent/cvm/`
- [x] 2.2 新建 `cmd/agent-server/logics/agent/cvm/account_select.go`，实现 `NewAccountSelectNode` 工厂函数（闭包捕获 `cloudClient`）
- [x] 2.3 实现节点内部逻辑：从 `inv.RunOptions.RuntimeState[SessionBkBizIDStateKey].(int64)` 读取 `bk_biz_id`，发起 3 秒超时 HTTP 请求调用 `ListByUsageBizID`（无需 `bk_ticket`，内部 client 调用）
- [x] 2.4 按 vendor 计算 `enabled`/`reason`：`tcloud-ziyan` → enabled=true；其他 vendor → enabled=false，reason="当前仅支持自研云（tcloud-ziyan）账号"
- [x] 2.5 实现 count==0 分支：路由至 `no_permission_fallback`
- [x] 2.6 实现 count==1 分支：写入 `state["account_id"]`，路由至 `llm`
- [x] 2.7 实现 count>=2 分支：构建含 enabled/reason 的 options 列表，调用 `graph.Interrupt(AccountSelectInterruptKey, payload)`，resume 后验证非空并写入 `state["account_id"]`，路由至 `llm`

## 3. no_permission_fallback 节点实现

- [x] 3.1 在 `cmd/agent-server/logics/agent/cvm/account_select.go` 实现 `NewNoPermissionFallbackNode` 工厂函数
- [x] 3.2 实现节点逻辑：调用 `graph.Interrupt(NoPermissionInterruptKey, payload)`，resume 后路由回 `account_select`

## 4. graph 拓扑修改

- [x] 4.1 修改 `cmd/agent-server/logics/agent/graph_build.go`：`BuildGraph` 函数签名增加 `cloudClient *cloudserver.Client` 参数
- [x] 4.2 注册 `account_select` 节点（`graph.AddNode`）
- [x] 4.3 注册 `no_permission_fallback` 节点（`graph.AddNode`）
- [x] 4.4 修改 START 边：将入口从 `llm` 改为 `account_select`
- [x] 4.5 添加 `account_select` → `no_permission_fallback` 条件边（count==0）
- [x] 4.6 添加 `account_select` → `llm` 条件边（count==1 或 resume 完成）
- [x] 4.7 添加 `no_permission_fallback` → `account_select` 回环边（resume 后重试）

## 5. runtime 注入修改

- [x] 5.1 修改 `cmd/agent-server/logics/runtime.go` 的 `newAGUIRunner`：将 `clientSet.CloudServer()` 传入 `BuildGraph` 调用

## 6. graph State 字段扩展

- [x] 6.1 在 graph State 结构体中新增 `account_id` 字段（string 类型）
