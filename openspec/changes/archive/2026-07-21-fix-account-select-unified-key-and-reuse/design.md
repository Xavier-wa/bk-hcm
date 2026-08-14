## Context

`host_apply` 子图的 entry 是 `account_select`。`makeSubgraphInputMapper` 为避免下一轮命中上一轮子图 `__end__` 完成态，按消息条数设置 `CfgKeyCheckpointNS`（如 `host_apply_25`），因此**每轮新申领都会从 `account_select` 重新执行**——这是预期，不是 bug。

跨轮「选一次、之后复用」依赖 session 后端记忆（`SessionSelectedAccountIDStateKey`）。上一变更 `fix-account-select-session-persist` 已注入 `session.Service` 并在 run 起点回填 `RuntimeState[SessionAccountIDTempKey]`，但线上复现（完成提单后再说「再帮我申请一单一摸一样的」）仍再次弹出 `account_select.interrupt`。

当前代码缺口：

| 环节 | 现状 | 问题 |
|------|------|------|
| 写入 key | `buildSessionKey(inv)` ← `inv.Session.*` | 子图 InputMapper 过滤 `StateKeySession`，`inv.Session` 可能为空 → 空 key / Update 失败 |
| 读 key（run 起点） | `appName + ctx.User + input.ThreadID` | 与写入 key 不一致 |
| 入口跳过 | 只 `getSelectedAccountIDFromSession` | 忽略本轮已回填的 RuntimeState |

## Goals / Non-Goals

**Goals:**

- 写入 / 读取 / run 起点回填使用**同一套** session.Key。
- 同会话再次进入 `host_apply` 时，`account_select` 能复用已选账号并跳过 HITL。
- `inv.Session == nil` 时仍可通过 `CfgKeyLineageID` + ctx 用户 + appName 完成读写。
- 保留现有 checkpoint namespace 设计与前端选账号协议。

**Non-Goals:**

- 不改动 `makeSubgraphInputMapper` / `CfgKeyCheckpointNS` 按消息条数分轮次的策略。
- 不把账号塞入子图 graph.State（避免 checkpoint 脏读）。
- 不改 `StateKeyForwardedResumeValue` 消费/剥离逻辑（独立变更已覆盖）。
- 不改前端 `forwardedProps.resumeValue` 协议。
- 多账号 resume 时 forwarded account_id 为空的文本兜底匹配为可选后续，不在本设计必做项。

## Decisions

### 决策 1：统一 session key —— `BuildAGUISessionKey(ctx, appName, threadID)`

**做法：**

```text
session.Key{
  AppName:   appName,                          // 闭包 / 配置注入
  UserID:    auth.BKUsernameFromContext(ctx),  // 与 AG-UI UserIDResolver 一致
  SessionID: threadID,                         // 优先 RuntimeState[CfgKeyLineageID]，否则 inv.Session.ID
}
```

- `account_select` 的 inject / get 与 `service.loadSelectedAccountID` 全部改用该函数。
- `NewAccountSelectNode(cloudClient, sessSvc, appName)` 增加 `appName`。

**为什么：** threadID 与 lineageId 在 AG-UI 中间件里已对齐为同一会话身份；ctx 用户名与 runner 建 session 时一致。彻底摆脱对子图 `inv.Session` 身份字段完整性的依赖。

**替代方案：**

- 继续用 `inv.Session`：子图路径不可靠，否决。
- 把 key 写进闭包工厂时捕获：缺少 per-request UserID/ThreadID，否决。

### 决策 2：`tryReuseAccountID` 双源跳过

**做法：** `account_select` 入口：

1. 用统一 key 读 session 后端 `SessionSelectedAccountIDStateKey`（权威、跨 Run）。
2. 若为空，读 `inv.RunOptions.RuntimeState[SessionAccountIDTempKey]`（本轮 run 起点可能已回填）。
3. 任一命中 → 写 RuntimeState、路由 LLM，**不** emit 账号选择文案/中断。
4. 都未命中 → 现行 listAccounts / 单选 / 多选 HITL。

**为什么：** 上一变更做了「决策 2：run 起点回填」，但节点从未消费该回填；双源跳过让回填真正参与决策，并对后端偶发读失败形成容错。

**替代方案：**

- 只修 key、不改跳过逻辑：若 run 起点已回填但后端暂时不可读，仍会弹中断；双源更稳。
- 只信 RuntimeState：无跨 Run 权威源，且子图内 RuntimeState 传播也不可 100% 假设；以后端为先、RuntimeState 为辅。

### 决策 3：不碰 namespace / InputMapper

`CfgKeyCheckpointNS = host_apply_{len(messages)}` 必须保留。去掉会导致下一轮子图直接 resume 到 `__end__`、整段申领空跑。重复进 `account_select` 是故意的；本变更只修入口记忆。

### 决策 4：persist 失败可观测性

`UpdateSessionState` 失败时打 **Error** 日志（含完整 key 与 rid），不阻断本轮（RuntimeState 已有账号可继续）。便于对照「写成功 / 读失败」类问题。

## Risks / Trade-offs

- **[Risk] `CfgKeyLineageID` 在子图 Function Node 的 RuntimeState 中缺失** → Mitigation：fallback 到 `inv.Session.ID`；两者皆空则打 Error 并走正常选择（不 panic）。
- **[Risk] UserID 与历史已落库 key 不一致（例如曾用空 Session 写错 key）** → Mitigation：存量无正确记忆的会话首次仍会选一次账号，之后按新 key 落库；无 migration 脚本必要。
- **[Trade-off] 双源跳过可能在「用户想换账号」时仍复用旧账号** → 与产品「同会话复用」一致；换账号需另开会话或后续加显式切换入口（Non-Goal）。
- **[已规避] 不经 graph.State 传账号** → 避免与 checkpoint / forwarded dirty-read 问题耦合。

## Migration Plan

1. 部署 agent-server 含本变更的版本。
2. 存量会话：无正确 `cvm_apply:selected_account_id` 时行为与今天相同（仍会选一次）；选过之后同会话「再来一单」应跳过。
3. 回滚：回退 agent-server 即可；无 DB schema 变更。

## Open Questions

- 无阻塞问题。若实施时发现 `CfgKeyLineageID` 在子图 inv 上缺失频率高，可改为由父图 InputMapper 显式写入非内部 key `session_thread_id`（仍不经 checkpoint 脏读路径优先放 RuntimeState）。
