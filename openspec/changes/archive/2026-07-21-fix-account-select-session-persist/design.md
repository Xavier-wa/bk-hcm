## Context

主机申领子图（`host_apply`）在 `account_select` 节点完成账号选择后，会调用 `injectAccountIDToRuntimeState`
把选中账号同时写进「当轮 RuntimeState」和「session 后端（经 `PersistStateToService`）」，以便后续同会话内
「重新提单」时跳过账号选择、直接复用，并让 instruction_prompt 带账号。

但 MR 3191 把 `host_apply` 由主图重构为子图 SubAgent 后，账号记忆失效：完成一次提单后，在同一对话窗口再次
说「重新提单」（一次新的 HTTP Run），`account_select` 读不到已选账号，又弹出账号选择中断。

根因在于 `account_select` 节点依赖子图 invocation 的 Session 传播来读写记忆：
- `getSelectedAccountIDFromSession` 读 `inv.Session.GetState(SessionSelectedAccountIDStateKey)`，这是**内存态**，
  跨 Run 重新加载后不可靠；
- `injectAccountIDToRuntimeState` 经 `PersistStateToService` 写后端，而 `PersistStateToService` 依赖
  `inv.SessionService`，该值在子图 invocation 中的可用性取决于框架的 invocation 传播时机。

我们已核对框架源码：`invocation.go` 的 `Clone` 会复制 `Session`/`SessionService`，MySQL 版 `GetSession`
也会回填 `UpdateSessionState` 写入的 state。这说明早期「子图 `inv.SessionService == nil` 导致账号从未落库」
的假设**并不成立**——账号在子图节点处通常能写进后端。**真正脆弱的是读取侧依赖子图 invocation 的 Session
传播在跨 Run 恢复时读回记忆**：任何 checkpoint 加载、session 身份、或传播时机上的偏差，都会让重新提单那一轮
的 `account_select` 读不到账号，从而重新触发选择中断。因此本方案的核心决策是**彻底绕开该传播**（不纠结于
定位每次偏差的具体环节），改为显式注入真正的 `session.Service` 并直接按 `session.Key` 读写后端，行为可预期、
可测试。

约束：
- HITL 每次中断恢复都是一次新的 HTTP Run，`tryPrepareAutoResume` 在每轮重建 `inv.RunOptions.RuntimeState`。
- 账号记忆与 `StateKeyForwardedResumeValue`（其他中断节点的结构化协议通道）相互独立，互不影响。
- `session.Service` 接口（trpc-agent-go@v1.8.1）提供 `GetSession(ctx, Key)` 与
  `UpdateSessionState(ctx, Key, StateMap)`，其中 `Key = {AppName, UserID, SessionID}`。

## Goals / Non-Goals

**Goals:**
- 选中账号可靠持久化到 session 后端（数据库），跨 HTTP Run 可读取。
- 完成提单后同会话「重新提单」时跳过账号选择，直接复用已选账号。
- 每轮 run 起点把已选账号回填进 `inv.RunOptions.RuntimeState[SessionAccountIDTempKey]`，使当轮
  instruction_prompt 从一开始就带账号（与现有的 `SessionBkBizIDStateKey` 注入方式对称、一致）。

**Non-Goals:**
- 不改动 `StateKeyForwardedResumeValue` 相关的中断恢复逻辑（已由独立变更覆盖）。
- 不改动前端协议（`forwardedProps.resumeValue` 选账号字段不变）。
- 不改 `account_select` 的账号查询/路由/HITL 中断交互本身。

## Decisions

### 决策 1：方案 1 —— `account_select` 显式持有 `session.Service`，直接读写后端（不依赖子图 invocation 传播）

**为什么**：子图 invocation 的 `inv.Session` 是内存态、跨 Run 不可靠；`PersistStateToService` 依赖
`inv.SessionService` 的传播也不可靠。把真正的 `session.Service` 经闭包注入 `NewAccountSelectNode`，
节点在读写记忆时直接调用 `GetSession` / `UpdateSessionState`（以 `session.Key{AppName, UserID, SessionID}` 定位），
彻底摆脱对 invocation 内存态与框架传播行为的依赖。

**替代方案**：
- 沿用 `inv.Session.GetState` + `PersistStateToService`：写入侧经框架 `Clone` 通常能落库，但读取侧依赖子图
  invocation 的 Session 传播在跨 Run 恢复时读回记忆并不可靠（会重新触发选择中断），否决。
- 在子图入口 mapper 里把账号塞进子图 state：账号是跨轮记忆而非轮内状态，且子图 state 经 checkpoint 持久化，
  易引入脏读（参考 `StateKeyForwardedResumeValue` 的教训），否决。

**实施要点**：
- `NewAccountSelectNode(cloudClient, sessSvc session.Service)` 增加 `session.Service` 参数；
- `injectAccountIDToRuntimeState`：仍写 `inv.RunOptions.RuntimeState[SessionAccountIDTempKey]`（当轮 prompt 用），
  但「持久化到后端」改为 `sessSvc.UpdateSessionState(ctx, key, StateMap{SessionSelectedAccountIDStateKey: accountID})`；
- `getSelectedAccountIDFromSession`：改为 `sessSvc.GetSession(ctx, key)` 后读 `state[SessionSelectedAccountIDStateKey]`；
- `key` 的来源：`inv.Session` 经 Clone 从父 invocation 复制了 `AppName/UserID/SessionID`，可直接取用；
  若实现时取不到，则在 `service.go` 已知 `sessionSvc` 与 `input.SessionID` 处构造后随闭包传入。
- `state/session.go` 的 `PersistStateToService` 可保留（主图其他调用方仍可能依赖），`account_select` 不再使用它。

### 决策 2：方案 2 —— 每轮 run 起点回填 `SessionAccountIDTempKey`

**为什么**：让当轮 instruction_prompt 在最早期的 LLM/instruction 渲染就带账号，而非等 `account_select` 跑完才填；
且与 `SessionBkBizIDStateKey` 的注入方式对称、一致。该回填仅是「读已存账号补位」，数据尚未落库时安全跳过，
不强制要求。

**实施要点**：在 `service.go` 每轮重建 `inv.RunOptions.RuntimeState` 的位置（与 `makeRunOptionResolver` 注入
`SessionBkBizIDStateKey` 同层，或在 `tryPrepareAutoResume` 内），用已有的 `sessionSvc` 按 `input.SessionID`
读出 `SessionSelectedAccountIDStateKey`，非空则写入 `runtimeState[SessionAccountIDTempKey]`。

## Risks / Trade-offs

- **[Risk] 子图节点取不到正确 `session.Key`（AppName/UserID/SessionID）** → Mitigation：优先取 `inv.Session`
  （Clone 已复制父 invocation 的身份）；若不可靠，则从 `service.go` 已知处构造后随闭包传入（方案 2 已在该处取值，
  可复用同一来源）。
- **[Risk] 存量已固化旧 checkpoint / 旧内存态中无此账号** → Mitigation：仅影响新轮次；旧数据无此 key，读为空时
  走正常账号选择，无副作用。
- **[Trade-off] 方案 2 依赖方案 1 已把账号写进后端**：若方案 1 未完成，方案 2 读到的永远是空、只是无效不报错。
  两方案需一并落地。
- **[已规避] 不通过 `graph.State` 传递账号**：避免子图 state 经 checkpoint 持久化带来的脏读。

## Open Questions

- 实施时在 `account_select` 节点内确认 `inv.Session.AppName/UserID/SessionID` 是否非空；若为空，
  改为由 `service.go` 构造 `session.Key` 随闭包传入（需同步调整 `buildHostApplySubgraph` 签名）。
