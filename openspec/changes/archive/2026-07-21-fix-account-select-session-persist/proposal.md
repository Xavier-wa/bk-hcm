## Why

主机申领子图（`host_apply`）在账号选择（`account_select`）节点完成选择后，会把选中账号写入 session 后端（数据库），以便后续同一会话内"重新提单"时跳过账号选择、直接复用。但 MR 3191 将 `host_apply` 由主图重构为子图 SubAgent 后，账号记忆链路变得不可靠：完成一次提单后，在同一对话窗口再次说"重新提单"（一次全新的 HTTP Run），`account_select` 读不到已选账号，又弹出账号选择中断，违背"选一次、之后复用"的预期。

根因：`account_select` 读写账号记忆时**依赖子图 invocation 的 Session 传播**——
- 写入侧经 `PersistStateToService` 落到 `inv.SessionService`；
- 读取侧经 `inv.Session.GetState(SessionSelectedAccountIDStateKey)` 读内存态。

我们核对过框架源码：`invocation.go` 的 `Clone` 会复制 `Session`/`SessionService`，MySQL 版 `GetSession` 也会回填 `UpdateSessionState` 写入的 state。这说明早期"子图 `inv.SessionService == nil` 导致写不进库、账号从未落库"的假设并不成立——**问题不在"账号没写进去"，而在于"跨 Run 恢复时依赖子图 invocation 的 Session 传播来读取记忆"这一机制本身脆弱**：任何 checkpoint 加载、session 身份、或传播时机上的偏差，都会让 `account_select` 在重新提单那一轮读不到账号，从而重新触发选择中断。

因此本方案不纠缠于精确定位每次偏差的具体环节，而是**从设计上彻底绕开子图 invocation 的 Session 传播**：由父图运行期持有真正的 `session.Service`，经闭包注入 `account_select` 节点，按 `session.Key{AppName, UserID, SessionID}` 直接读写 session 后端。子图与主图共用同一 session ID，身份天然一致，注入后即与框架传播行为解耦，行为可预期、可测试。

## What Changes

- 让 `account_select` 节点**绕开子图 invocation 的 Session 传播链路**（写入依赖 `inv.SessionService`、读取依赖 `inv.Session.GetState` 内存态，二者在跨 Run 恢复时均不可靠），改为显式持有真正的 session service（经闭包注入），按 `session.Key{AppName, UserID, SessionID}`（子图与主图同一 session ID）直接读写 session 后端：
  - 写入侧：`injectAccountIDToRuntimeState` 不再依赖 `PersistStateToService`，改用注入的 session service 直接 `UpdateSessionState` 落库；
  - 读取侧：`getSelectedAccountIDFromSession` 不再读子图内存态 `inv.Session.GetState`，改为经注入的 session service 读后端。
- 在每轮 run 起点（`service.go` 的 RunOption 解析 / `tryPrepareAutoResume`）把已落库的账号回填进 `inv.RunOptions.RuntimeState[SessionAccountIDTempKey]`，使当轮 instruction_prompt 从一开始就带账号（与现有的 `SessionBkBizIDStateKey` 注入方式对称、一致）；该回填在数据尚未落库时安全跳过，不强制要求。

> 说明：上述"写入/读取侧修复"即方案 1（根治"重新进账号选择"），"run 起点回填"即方案 2（让 prompt 从一开始就有账号、且注入方式对称）。两者组合完整恢复迁移前的体验。

## Capabilities

### New Capabilities
- `account-select-session-persistence`: 主机申领子图账号选择的跨轮记忆能力——选中账号持久化到 session 后端、跨 Run 读取并跳过重复选择、且在每轮 run 起点回填供 instruction_prompt 使用。

### Modified Capabilities
<!-- 无既有 spec 级别的 requirement 变更 -->

## Impact

- 代码：
  - `cmd/agent-server/logics/agent/cvm_apply/account_select.go`：`NewAccountSelectNode` 增加 `session.Service` 闭包参数；改写 `injectAccountIDToRuntimeState` 与 `getSelectedAccountIDFromSession` 走真正的 session service；
  - `cmd/agent-server/logics/agent/cvm_apply/graph_build.go`：`host_apply` 子图构建处为 `account_select` 节点传入 session service；
  - `cmd/agent-server/service/service.go`：run 起点（`makeRunOptionResolver` 或 `tryPrepareAutoResume`）增加从 session 后端回填 `SessionAccountIDTempKey` 的逻辑；
  - `cmd/agent-server/logics/agent/state/session.go`：`PersistStateToService` 仍可保留（其他主图调用方依赖），但 `account_select` 不再使用它。
- API / 协议：无变化（前端 `forwardedProps.resumeValue` 选账号协议不变，`SessionAccountIDTempKey` / `SessionSelectedAccountIDStateKey` 常量不变）。
- 依赖：依赖 trpc-agent-go 子图 `inv.Session.ID` 与主图 sessionID 一致这一前提（实施前需确认）。
- 风险：仅影响主机申领子图账号记忆链路，不涉及 `StateKeyForwardedResumeValue` 等其他 resume key（已在其他变更 `restore-forwarded-resume-consume-and-clear` 中处理）。
