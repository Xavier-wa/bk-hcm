## Why

AI 助手主机申领门禁（`create_biz_apply`）在用户确认后调用 `validateApply` 做提单前校验，但当前是**占位实现**（`toolgate/create_cvm_apply.go`，直接返回通过），无法拦截不合规的申领。需要接入与人工提单（`CreateBizApplyOrder`）一致的真实校验——包括需求类型校验、预测内/外余量（预算）、GPU 计费时长，并新增**实时容量/库存**校验——在提单前及早拒绝不可满足的申领，避免无效单据进入审批与异步生产流程。

## What Changes

- **woa-server 新增提单前只读校验接口** `CheckBizApplyOrder`（业务视角 `POST /bizs/{bk_biz_id}/task/check/apply`）：只校验、不落库、不建 ITSM。
  - 复用人工提单同一校验链：`input.Validate()` → 权限 `AuthorizeWithPerm(Biz, Create)` → `verifyAccordingToRequireType`（机型/绿通/DA前缀/裁撤额度）→ `verifyResPlanDemand`（预测内/外余量）。
  - 抽取底层 `Scheduler().CreateApplyOrder` 中的 GPU 计费时长校验（`VerifyCvmGPUChargeMonth`）到只读校验链。
  - **新增实时容量校验**：逐子单调用 `Capacity().GetCapacity`（实时 CRP `QueryCvmCapacity`），按现有生产口径判定（单可用区看该 zone `MaxNum`；分Campus/多可用区按各候选 zone `MaxNum` 求和），与 `replicas` 比对；`NotNeedVerifyCapacity` 的需求类型跳过。
  - 返回结构化结果 `{pass, reason}`：业务不通过返回 `pass=false` + 原因（HTTP 200）；系统异常返回 error。
- **重构（零行为变化）**：从 `createApplyOrder` 抽取只读校验聚合方法，供"创建"与"校验"复用，保证两侧逻辑一致。
- **woa-server client** 新增 `CheckBizApplyOrder` 方法（`pkg/client/woa-server/task.go`）。
- **agent-server 申领门禁接入真实校验**：
  - `validateApply` 由占位改为调用 woa client；`createCvmApplyGate` 持有 woa-server client（`clientSet` 经 `runtime.New → BuildGraph → EnabledGateHandlers → newCreateCvmApplyGate` 透传）。
  - **`bk_biz_id` 从工具调用 `path_param` 提取**：MCP 工具 args 以 `path_param`/`body_param` 并列包裹（`tool_callbacks.go`），现仅取了 `body_param`；新增取 `args["path_param"]["bk_biz_id"]`，随 `ApplyReq` 传给校验接口（path param 是真实提单的必填项，到确认后阶段必然存在）。

## Capabilities

### New Capabilities
- `host-apply-presubmit-validate`: woa-server 提单前只读校验能力，复用提单校验链并新增实时容量校验，输出结构化通过/原因结果。
- `agent-apply-gate-real-validate`: agent-server 申领门禁在用户确认后执行真实前置校验（替换占位），含从工具调用 `path_param` 取 `bk_biz_id` 与 woa 校验接口调用。

### Modified Capabilities
<!-- 申领门禁占位逻辑来自 agent-tool-confirm-gate 变更（尚未归档到 openspec/specs/），本变更以新能力承接其真实校验，不修改已归档 spec。 -->

## Impact

- **woa-server**：`cmd/woa-server/service/task/{service.go,scheduler.go}`（新增 handler + 路由 + 抽取只读校验）、复用 `logics/plan`（预测）、`logics/config/capacity`（容量）、`logics/task/scheduler`（GPU 校验抽取）。
- **client**：`pkg/client/woa-server/task.go`。
- **agent-server**：`cmd/agent-server/logics/agent/toolgate/create_cvm_apply.go`（path_param 取 `bk_biz_id` + 接入校验）、`registry.go`、`graph_build.go`、`logics/runtime.go`（clientSet 透传）。无需改动 session 模型与前端注入。
- **类型**：复用 `cmd/woa-server/types/task` 的 `ApplyReq`，新增校验结果类型。
- **外部依赖**：CRP（云梯）实时容量接口 `QueryCvmCapacity`、预测/ITSM 既有依赖；不新增第三方依赖。
- 校验均为只读、无副作用（不落库、不建单），不改变既有提单与异步生产行为。
