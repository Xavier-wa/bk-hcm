## Why

业务用户在 Agent 主机申领确认后，若没有该业务提单权限，现网门禁先调 woa `CheckBizApplyOrder`、再把 `err.Error()` 写成纯文本（含 2030403），对话只能转述「联系管理员」。Agent 申领已上线，无权限用户卡在「看懂了但去不了申请页」。本期在门禁上做**主动前置鉴权**（与 woa 相同的业务 + 创建/申领），无权限时直接给出可打开的 `apply_url`。

## What Changes

- 主机申领门禁（守护 `create_biz_apply`，用户确认后）MUST 先对当前用户、当前业务做主机申领权限的主动判断；判定无权限时拒绝放行，且不调用 woa 预检。
- 无权限且申请地址可生成时，拒绝结果 MUST 带无权限标识（`code` = 2030403）与非空 `apply_url`；生成失败走降级文案，不得给空串或伪造地址，也不得把这次拒绝变成系统错误（5xx）。
- `apply_url` 必须对应该用户、该业务、主机申领（`meta.Biz` + `meta.Create` + 当前 `bk_biz_id`）；禁止用智能体助手或其它业务顶替。
- 主动鉴权系统失败（IAM 报错 / 超时）MUST 拒绝提单，MUST NOT 放行，MUST NOT 把「鉴权抖动」当成有权限。
- woa `CheckBizApplyOrder` 及其内部 `AuthorizeWithPerm` **保留为纵深防御**，本期不删、不放宽。主动鉴权通过后仍走现网 woa 预检（额度/库存等）。
- 门禁上的非权限失败（参数不合法、额度/库存不足、用户取消确认、缺少业务 ID、woa 非权限系统故障）MUST NOT 附带 `apply_url`。
- 有权限且其它校验通过时，放行行为与现网一致，MUST NOT 返回无权限标识或 `apply_url`。
- `permission` 本期不强制返回（Q-006）；旧消费方忽略新增字段不得报错。
- **非 BREAKING**：现网门禁拒绝本就没有结构化字段；本期只在无权限路径上增加可忽略字段，不改权限点、不放宽鉴权、不改正式提单。

本期不包含：Agent 对话前端渲染、小鲸对齐、正式提单工具节点、全部 Agent 工具、平台「智能体助手」入口权限、无可用云账号提示、传统申领页、资源视角 / 管理员路径、新增或变更主机申领权限点。

## Capabilities

### New Capabilities

无。无权限引导挂在现有主机申领门禁上，不单独成新 capability。

### Modified Capabilities

- `agent-apply-gate-real-validate`：在现有 woa 预检三态之前增加主动鉴权；无权限由门禁 `Authorize` 判定并返回 `apply_url`（或降级）。woa 预检仍负责额度/库存等；非权限失败与放行路径保持现网语义。

## Impact

- **行为只改主机申领门禁**（`create_cvm_apply.go`）：确认后先鉴权，再按结果决定是否调 woa、如何构造拒绝结果。
- **为注入 `auth.Authorizer` 需改依赖传递**（不改其它工具逻辑）：`newCreateCvmApplyGate`、`builtinGates`、`GetEnabledGateHandlers`（`registry.go`）、`buildHITLRegistry` / `BuildGraph` / `buildHostApplySubgraph`（`graph_build.go`）、`newAGUIRunner` / `Runtime.New`（`runtime.go`）、`service.go` 的 `logics.New` 调用；以及对应单测（`create_cvm_apply_test.go`、`registry_test.go`、`graph_build_test.go`）。`GetEnabledGateToolNames` 仍可对 authorizer 传 nil（只取工具名）。
- **复用、不改**：`pkg/iam/auth.Authorizer`（`Authorize` / `GetPermissionToApply` / `GetApplyPermUrl`）；woa `CheckBizApplyOrder` 仍用 `AuthorizeWithPerm(Biz, Create, BizID)`。不改 woa-server、不改 `common.Request`。
- **不改**：auth-server 实现、web-server、正式提单工具、前端、小鲸、传统 HTTP 403 弹窗、权限点注册、数据库 schema、helm / etc yaml。
- **层影响**：仅 agent-server；不新增对外 HTTP API。消费方为 Agent 对话层（本期不改前端）。
