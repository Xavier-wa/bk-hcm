## 1. 统一 session key

- [x] 1.1 在 `cvm_apply` 包实现 `BuildAGUISessionKey(ctx, appName, threadID)`（或等价私有函数），`UserID` 取 `auth.BKUsernameFromContext(ctx)`
- [x] 1.2 实现 `resolveThreadID(inv)`：优先 `RuntimeState[CfgKeyLineageID]`，回退 `inv.Session.ID`；皆空时返回空并打 Error 日志
- [x] 1.3 将 `injectAccountIDToRuntimeState` / `getSelectedAccountIDFromSession` 改为使用统一 key；删除或降级仅依赖 `inv.Session` 的 `buildSessionKey`
- [x] 1.4 `NewAccountSelectNode` 增加 `appName string` 参数；`graph_build.go` 的 `buildHostApplySubgraph` 传入 AGUI AppName
- [x] 1.5 `service.go` 的 `loadSelectedAccountID` 改用同一 key 构造逻辑（可抽到共享包或保持两边一致复制并加注释互指）

## 2. 双源复用跳过

- [x] 2.1 实现 `tryReuseAccountID`：先 session 后端，再 `RuntimeState[SessionAccountIDTempKey]`
- [x] 2.2 `account_select` 入口用 `tryReuseAccountID` 替换单纯的 `getSelectedAccountIDFromSession`；命中则写 RuntimeState、路由 LLM、不 emit 选账号文案/中断
- [x] 2.3 persist 失败日志升级为 Error，并带完整 session.Key 与 rid（当轮不阻断）

## 3. 单测与验收

- [x] 3.1 单测：`inv.Session == nil` + RuntimeState 含 lineageID 时，inject 能落库、get 能读回
- [x] 3.2 单测：后端无账号但 RuntimeState 已有 `SessionAccountIDTempKey` 时，`tryReuseAccountID` 命中并跳过
- [x] 3.3 单测：双源皆空时返回 false（走正常选择）
- [ ] 3.4 手工验收：同 session 完成一单后说「再帮我申请一单」，日志出现 `reuse persisted account_id`，无第二次 `account_select.interrupt`
