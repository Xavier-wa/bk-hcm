## Context

主机申领门禁 `createCvmApplyGate`（`cmd/agent-server/logics/agent/toolgate/create_cvm_apply.go`）在用户确认后调用 woa `CheckBizApplyOrder`。woa 侧用 `AuthorizeWithPerm(Type: meta.Biz, Action: meta.Create, BizID)` 做业务创建/申领鉴权（`cmd/woa-server/service/task/check.go` 第 57–64 行）；无权限返回 `errf.NewWithPerm(2030403, ...)`，走 error 而非 `Pass=false`。门禁把任意 `err` 写成纯文本 `"申领前置校验失败，暂时无法提单："+err.Error()`，对话只能转述「联系管理员」，没有可打开的申请地址。

**已核实的鉴权入口（agent-server）：**

| 入口 | 位置 | 说明 |
|------|------|------|
| `auth.Authorizer` 接口 | `pkg/iam/auth/auth.go` 第 40–61 行 | `Authorize` / `AuthorizeAny` / `AuthorizeWithPerm` / `GetPermissionToApply` / `GetApplyPermUrl` 等 |
| `Authorize` | 同文件第 98–122 行 | 返回 `([]Decision, authorized bool, error)`，一次能区分「有权限 / 无权限 / IAM 调用失败」 |
| `AuthorizeWithPerm` | 同文件第 142–159 行 | 无权限时内部再调 `GetPermissionToApply`，返回 `NewWithPerm(2030403, ..., permission)`；Authorize RPC 失败与 GetPermissionToApply 失败都变成 `DoAuthorizeFailed`，分不清「无权限但申请信息失败」与「IAM 挂了」 |
| `GetPermissionToApply` / `GetApplyPermUrl` | 同文件第 241–268 行 | 申请结构与申请 URL |
| HTTP 层用法 | `cmd/agent-server/service/memory/query.go` 第 42–45 行 | `svc.authorizer.AuthorizeWithPerm(...)` |
| 实例持有 | `service/session/session.go`、`memory.go`：`authorizer auth.Authorizer`；`NewService` 第 103 行 `auth.NewAuthorizer`，第 113 行 `logics.New(apiClientSet)` 未把 authorizer 传入 Runtime |
| 门禁现注入 | `newCreateCvmApplyGate(*client.ClientSet)`；`ClientSet.AuthServer()` 也能调 AuthorizeBatch / GetPermissionToApply / GetApplyPermUrl。HTTP 层与 woa 使用 Authorizer，门禁与之对齐 |

`pkg/client/common.Request` 解包响应时只保留 `code`/`message`，不携带 `permission`。本期申请地址由门禁侧 Authorizer 生成，不从 woa 错误体读取 `permission`。不改 `common.Request`。

利益相关方：业务视角 Agent 主机申领用户。TAPD：1069995598137758730。

## Goals / Non-Goals

**Goals:**

- 用户确认后、调用 woa 预检之前，门禁用与 woa 相同的资源属性判断主机申领权限。
- 无权限：跳过 woa 预检；拒绝并尽量带 `apply_url`；生成失败走降级。
- 有权限：走现网 woa 预检（额度/库存等），放行条件不放宽、不收紧。
- IAM 本身失败：拒绝，不放行。
- 单测可打桩鉴权与申请地址。

**Non-Goals:**

- 不替代、不删除 woa 侧 `AuthorizeWithPerm`。
- 不改 woa-server、`common.Request`、auth-server 实现、前端、小鲸、正式提单、全部 Agent 工具。
- 不强制返回 `permission`（Q-006）。
- 不新增权限点。

## Decisions

### D1. 主动前置鉴权（主路径）

- **Decision**：`onConfirm` 在 `validateApply`（woa `CheckBizApplyOrder`）之前，用当前用户 kit + `meta.ResourceAttribute{Basic: {Type: meta.Biz, Action: meta.Create}, BizID}` 调 `Authorizer.Authorize`。
  - `err != nil`（IAM 报错/超时）→ 拒绝，不调 woa，不放行（见 D5）。
  - `authorized == false` → 无权限主路径：不调 woa 预检，生成 `apply_url` 或降级（见 D2）。
  - `authorized == true` → 再调现网 `validateApply`。
- **Why**：门禁需要直接知道当前用户对当前业务有没有主机申领权限。woa 无权限走 error 而非 `Pass=false`；解析 2030403 会把权限状态绑在下游错误契约和 `err.Error()` 文本上，契约脆弱。资源属性必须与 `check.go` 第 57–59 行一致，否则会出现门禁放行但 woa 拒绝、或反之。
- **Alternatives**：等 `CheckBizApplyOrder` 返回错误码再倒推权限状态 — 不采纳，因为把权限判定绑在下游错误契约与错误文本上很脆弱，且门禁本就要直接与 IAM 交互。依赖 `err.Error()` / 文案含「权限」— 不采纳，会误伤额度等失败（AC-004）。让 woa 把无权限归一成 `Pass=false` — 不采纳，超出本期范围。

### D2. 无权限时用同一 Authorizer 生成 apply_url

- **Decision**：`Authorize` 判定无权限后，对同一 `ResourceAttribute` 调 `GetPermissionToApply`，再 `GetApplyPermUrl`。门禁主调用使用 `Authorize`，不使用 `AuthorizeWithPerm`：后者把「IAM 挂了」和「无权限但申请信息失败」都收成 `DoAuthorizeFailed`，无法同时满足 F-003 与 D5。`permission` 结构只用于生成 URL，不写入 tool 结果（Q-006）。
- **Why**：`Authorize` 一次能拿到有没有权限；申请地址是无权限之后的附加步骤。HTTP 层的 `AuthorizeWithPerm` 适合「失败即 403」，不适合门禁的三路分支。
- **Alternatives**：只注入 `ClientSet.AuthServer()`、不改图构建签名 — 能调通，但不采纳：与 agent-server 其它鉴权入口及 woa 侧不一致，测试桩更绕，且门禁需要 Authorizer 的三路分支语义。

### D3. 主动鉴权不替代 woa 鉴权（纵深防御）

- **Decision**：woa `CheckBizApplyOrder` 开头的 `AuthorizeWithPerm` 本期保留，不删除、不绕过、不放宽。门禁主动鉴权是前置：无权限时不调用 woa 预检；有权限时仍进入 woa 预检，woa 仍会再鉴权一次。正式提单工具节点本期不改，其自身鉴权也不动。
- **Why**：门禁与 woa 是两次独立调用；只信门禁会在 woa 被其它入口调用、或门禁被关掉时留下缺口。需求写明不放宽鉴权。

### D4. woa 返回 2030403 时的窄兜底（非主路径）

- **Decision**：主动鉴权已通过、随后 woa 预检仍返回 `errf.PermissionDenied`（2030403）时，MUST 按无权限引导处理（生成 `apply_url` 或降级），MUST NOT 显示「暂时无法提单」系统异常文案。识别无权限的主路径是 D1 的 `Authorize == false`。兜底覆盖：两次调用之间权限被回收、或实现漂移导致 woa 仍拒绝。兜底同样用 Authorizer 生成 URL，不解析 woa 错误体里的 `permission`（`common.Request` 不会带上该字段）。
- **Why**：同一鉴权点下，门禁已通过之后再出现 2030403 应极罕见；若完全不处理，该路径会落成现网纯文本，用户仍无法跳转申请。
- **Alternatives**：把解析 woa 2030403 当作识别无权限的唯一手段 — 不采纳，因为把权限判定绑在下游错误契约与错误文本上很脆弱，且门禁本就要直接与 IAM 交互。完全忽略 woa 的 2030403 — 不采纳，TOCTOU 或鉴权点漂移时该路径会落成现网纯文本，用户仍无法跳转申请。

### D5. IAM 本身失败：拒绝，不放行，不继续 woa

- **Decision**：`Authorize` 返回 error（超时、auth-server/IAM 不可用等）时：拒绝放行、`Next` 为 llm、不调用 woa 预检、不带 `apply_url`、文案为权限校验失败/暂时无法提单（系统异常类，不是 F-003 — F-003 的前提是已经判定无权限）。authorizer 未注入（nil）同样 fail closed。
- **Why**：不得因鉴权抖动放行提单。继续走 woa 仍会打同一套 IAM，多一次 RTT 且把故障混进预检错误；若误把 woa 系统错误当可重试库存问题，风险更高。尚未判定是否无权限时不能提供申请跳转（不能走 F-001/F-003）。

### D6. 注入 Authorizer，可打桩

- **Decision**：`newCreateCvmApplyGate(clientSet, authorizer)`。增加可打桩 `authorizeFunc`（返回 authorized bool, error）与 `applyURLFunc`。默认实现调注入的 `auth.Authorizer`。`GetEnabledGateToolNames` / 仅需工具名的路径对 authorizer 传 nil。
- **波及面**（依赖传递，不改其它工具行为）：
  1. `toolgate/create_cvm_apply.go` 构造函数
  2. `toolgate/registry.go`：`builtinGates`、`GetEnabledGateHandlers`
  3. `graph_build.go`：`buildHITLRegistry`、`buildHostApplySubgraph`、`BuildGraph`（resource_query 的 HITL 注册一并改签名，可不使用 authorizer）
  4. `runtime.go`：`New`、`newAGUIRunner`
  5. `service.go`：`logics.New(apiClientSet, authorizer)`（authorizer 在第 103 行已创建，早于第 113 行 Runtime）
  6. 单测：`create_cvm_apply_test.go`、`registry_test.go`、`graph_build_test.go`
- **Why**：与 `memory/query.go` 同一套 Authorizer；单测不碰真 IAM。

### D7. 无权限路径的 tool 结果使用 JSON；其它失败保持纯文本

无权限且 URL 可用：

```json
{
  "code": 2030403,
  "message": "当前用户 <user> 在业务 <bizID> 下没有主机申领权限",
  "apply_url": "https://..."
}
```

- URL 失败：省略 `apply_url`，`message` 为降级说明，`code` 仍 2030403。
- 非权限失败与 IAM 系统失败：现网风格纯文本，无 `apply_url`。
- 不写 `permission`（Q-006）。

### D8. 已判定无权限后，申请地址失败只降级

- **Decision**：`GetPermissionToApply` / `GetApplyPermUrl` 失败、空串、非 http(s) → `Warnf` + D7 降级 JSON。这与 D5 不同：此时已经 `Authorize == false`。
- **Why**：AC-008 / F-003。

### D9. 不改 woa、common.Request、权限点

- **Decision**：不从 woa 错误体读取 `permission`，因此不修改 `common.Request`。不删除 woa 鉴权，不改权限点。

### D10. 鉴权主体只取请求上下文用户名

- **Decision**：主动鉴权用 `newAuthKit(ctx)` 构造 kit，用户名只取 `authlogic.BKUsernameFromContext`（`X-Bkapi-User-Name`）；取不到即返回 nil，`onConfirm` 按 D5 fail closed 拒绝（不调 authorize / woa / applyURL）。申领 body 里的 `bk_username` 与 `core.NewBackendKit` 预置的 `constant.BackendOperationUserKey` 都不作为判定主体。`callWoaCheck` 继续用 `newGateKit(ctx, req.User)`，woa 侧身份传递保持现网不变。
- **Why**：body 的 `bk_username` 由模型生成、且用户可在确认卡片上编辑，属不可信输入。原先它只是转发给 woa 的一个参数，woa 自己会再鉴权；本期门禁开始拿它当判定主体后，填一个有权限的账号就能过门禁，与 D5 的 fail closed 取向相反。另外放行后的正式提单走 internal MCP hook，注入的是 ctx 用户名（缺失时兜底后端操作用户），判定主体与真实提单主体必须同源，否则出现门禁按 A 判定、实际按 B 提单。`kt.User` 还进入无权限文案与 `apply_url` 生成，主体错会给出误导性的账号与申请页。
- **Alternatives**：沿用 woa 调用侧的 `req.User` 回退以保持两侧口径一致 — 不采纳，理由同上。ctx 缺用户名时回退后端操作用户 — 不采纳，等于用内部账号的权限替真实用户判定，同样是 fail open。
- **代价**：AG-UI 与 A2A 两个入口的中间件都会把用户名注入 ctx，正常链路不受影响；若将来出现无用户名的调用入口（后台任务等），该路径会被门禁拒绝，需要显式补齐身份而不是回退。

## Risks / Trade-offs

- **有权限路径多 1 次 IAM Authorize**（无权限再加 GetPermissionToApply + GetApplyPermUrl）→ 仅主机申领确认后；IAM 失败立即拒绝、不重试。AC-P01：额外耗时 P99 ≤ 200ms。
- **与 woa 鉴权点漂移** → 资源属性写死为 `Biz + Create + 当前 bizID`，与 `check.go` 第 57–59 行一致；单测断言入参；D4 覆盖 TOCTOU / 漂移。
- **Authorize 与 AuthorizeWithPerm 语义差** → 门禁用 `Authorize` 做三路分支，不用 `AuthorizeWithPerm` 当主调用。
- **JSON 被 LLM 念出** → 可接受；`message` 已是中文原因。
- **注入链变长** → 波及文件见 D6；resource_query 的 `buildHITLRegistry` 只改签名。

## Migration Plan

- 随 agent-server 发布，无表结构、无配置、无 helm。
- 回滚该提交后恢复「先 woa、纯文本拒绝」。
- 现网对话消费方忽略新 JSON 字段即可。

## Open Questions

- Q-006（非阻塞）：前端是否还要 `permission`。本期默认不交。
