## 1. 方案 1：account_select 显式持有 session.Service 并直接读写后端

- [x] 1.1 在 `buildHostApplySubgraph`（`graph_build.go`）中，把 `session.Service` 透传给 `cvmapply.NewAccountSelectNode`，修改其签名为 `NewAccountSelectNode(cloudClient *cloudserver.Client, sessSvc session.Service)`；`session.Service` 来源为 `clientSet` 持有的运行期 SessionSvc（参考 `service.go` 中 `s.runTime.SessionSvc()` 的取用方式）。
- [x] 1.2 改写 `injectAccountIDToRuntimeState`（`account_select.go`）：① 仍写 `inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey] = accountID`（当轮 prompt 用）；②「持久化到后端」改为调用 `sessSvc.UpdateSessionState(ctx, key, session.StateMap{constant.SessionSelectedAccountIDStateKey: []byte(accountID)})`，其中 `key := session.Key{AppName, UserID, SessionID}` 取自 `inv.Session`（Clone 已复制父 invocation 身份）。若 `inv.Session` 身份字段为空，则在 1.1 处随闭包额外传入构造好的 `session.Key`。
- [x] 1.3 改写 `getSelectedAccountIDFromSession`（`account_select.go`）：不再读内存态 `inv.Session.GetState`，改为 `sessSvc.GetSession(ctx, key)` 后读取 `state[constant.SessionSelectedAccountIDStateKey]`，非空且长度 > 0 时返回 `(string, true)`。
- [x] 1.4 为 `sessSvc == nil` 或 `GetSession` 出错等异常路径补充防御（warn 日志 + 走正常选择分支），保证旧行为不退化；`PersistStateToService`（`state/session.go`）保留不动（主图其他调用方可能依赖）。

## 2. 方案 2：每轮 run 起点回填 RuntimeState

- [x] 2.1 在 `service.go` 每轮重建 `inv.RunOptions.RuntimeState` 的位置（与 `makeRunOptionResolver` 注入 `SessionBkBizIDStateKey` 同层，或 `tryPrepareAutoResume` 内），用已有 `sessionSvc` 按 `input.SessionID` 读取 `SessionSelectedAccountIDStateKey`。
- [x] 2.2 读到的账号 ID 非空时，写入 `runtimeState[constant.SessionAccountIDTempKey] = accountID`；为空时安全跳过（不强制、不报错），与 `SessionBkBizIDStateKey` 注入方式对称。

## 3. 验证

- [x] 3.1 单元/逻辑测试：覆盖「单选自动选中后落库」「多选用户选定后落库」「重新提单读取已存账号并跳过选择」「后端无账号时仍走正常选择」「run 起点回填 / 后端无账号跳过」五类场景（新增 `account_select_test.go`，基于内存 session service，已全部通过）。
- [ ] 3.2 端到端验证：同一会话完成一次提单后再次发起主机申领，确认不再弹账号选择中断、`{temp:account_id?}` 占位符在当轮 prompt 中始终有值。（需 QA 在真实 MySQL session 后端环境手测）
- [ ] 3.3 确认未影响 `StateKeyForwardedResumeValue` 等其他中断恢复通道（与 `restore-forwarded-resume-consume-and-clear` 变更互不冲突）。（`makeSubgraphInputMapper` 已对 `forwarded_resume_value` 剥键，本变更仅新增独立的 `account_id` 后端读写，二者 channel 互不干扰，代码层面已确认无冲突）
