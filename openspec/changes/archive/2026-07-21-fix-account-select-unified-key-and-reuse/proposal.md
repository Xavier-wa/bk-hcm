## Why

上一轮变更 `fix-account-select-session-persist`（commit `f88068e`）已把 `account_select` 改为经注入的 `session.Service` 读写后端，并在 run 起点回填 `RuntimeState[SessionAccountIDTempKey]`。但线上验证证明它**未闭环**：同会话完成一次主机申领后，用户再说「再帮我申请一单」时仍再次弹出 `account_select.interrupt`。

问题根因有两层，且缺一不可：

1. **session key 来源不统一**：写入侧 `buildSessionKey(inv)` 依赖子图 `inv.Session.AppName/UserID/ID`；run 起点读取侧 `loadSelectedAccountID` 用 `appName + ctx.User + input.ThreadID`。子图入口 `makeSubgraphInputMapper` 会过滤 `StateKeySession`，子图节点上 `inv.Session` 可能为空或不完整，导致写入失败或写到错误 key，后端读不到已选账号。
2. **跳过逻辑只信后端、不读本轮已回填的 RuntimeState**：即使 run 起点已成功回填 `SessionAccountIDTempKey`，`account_select` 入口仍只调用 `getSelectedAccountIDFromSession`；后端读不到就立刻走多账号 HITL 中断。

说明：`makeSubgraphInputMapper` 按消息条数分配 `CfgKeyCheckpointNS` 是**有意设计**（避免下一轮命中子图 `__end__` 完成态空跑），每轮重进 `account_select` 是预期路径；本变更不改动该设计，只保证入口处能可靠复用已选账号。

## What Changes

- 抽取统一的 AGUI session key 构造函数：始终用 `appName + BKUsernameFromContext(ctx) + threadID`，threadID 优先取 `RuntimeState[CfgKeyLineageID]`，`inv.Session` 仅作 fallback；写入与读取共用此函数。
- `NewAccountSelectNode` 增加 `appName` 闭包参数，persist / get 均走统一 key；不再单独依赖 `inv.Session` 身份字段是否完整。
- `account_select` 入口改为 `tryReuseAccountID` 双源跳过：先读 session 后端，再读 `RuntimeState[SessionAccountIDTempKey]`；任一命中则注入 RuntimeState 并路由 LLM，不触发账号选择 HITL。
- `service.go` 的 `loadSelectedAccountID` 改用同一 key 构造函数，与节点读写对齐。
- 增强 persist 失败可观测性（日志需带完整 session.Key）；多账号 resume 时 forwarded account_id 为空的兜底可作为可选加固，非本变更必做项。
- **不改动**：`makeSubgraphInputMapper` / checkpoint namespace、前端协议、`StateKeyForwardedResumeValue` 链路。

## Capabilities

### New Capabilities
<!-- 无全新能力域；本变更是对既有账号选择跨轮记忆闭环的补齐 -->

### Modified Capabilities
- `cvm-account-select`: 补充统一 session key 构造、`tryReuseAccountID` 双源跳过（session 后端 + 当轮 RuntimeState），确保同会话重进 `host_apply` 子图时不再重复触发账号选择中断；与上一变更落库能力衔接，补齐「读写 key 一致 + 入口能跳过」闭环。

## Impact

- 代码（仅 agent-server）：
  - `cmd/agent-server/logics/agent/cvm_apply/account_select.go`：统一 key、`tryReuseAccountID`、`appName` 注入
  - `cmd/agent-server/logics/agent/graph_build.go`：`NewAccountSelectNode` 传 `appName`
  - `cmd/agent-server/service/service.go`：`loadSelectedAccountID` 对齐 key
  - 单测：`inv.Session == nil` + 仅有 lineageID 时仍可读写并跳过
- API / 协议：无变化（`forwardedProps.resumeValue` 选账号字段不变）
- 依赖：无新依赖；继续使用已注入的 `session.Service`
- 风险范围：仅主机申领子图账号记忆；不影响 resource-query 等其他子图
