## Context

AI 助手主机申领走 trpc-agent-go graph 流程。`create_biz_apply` 受 `tool_gate` 门禁保护（变更 `agent-tool-confirm-gate` 已落地），用户在确认卡片点确认后，`createCvmApplyGate.OnResume` 调用 `validateApply` 做提单前校验，再放行到 tool 节点执行真实提单。当前 `validateApply` 为占位（`toolgate/create_cvm_apply.go:165`），直接返回通过。

人工提单链路 `CreateBizApplyOrder`（`woa-server/service/task/scheduler.go:522`）的校验已梳理清楚：

- handler 同步：`input.Validate()`（结构）→ `AuthorizeWithPerm(Biz, Create)`（权限）
- `createApplyOrder`（`:645`）同步前置：`verifyAccordingToRequireType`（机型/绿通/DA前缀 + **裁撤可申请额度**）→ `verifyResPlanDemand`（**预测内/外余量**，`logics/plan/verify.go:VerifyResPlanDemandV2`）
- 底层 `Scheduler().CreateApplyOrder`（`logics/task/scheduler/scheduler.go:1162`）：`processingTicketByRequireType` → `VerifyCvmGPUChargeMonth`（**GPU 计费时长**）→ `FillCVMAppliedCore`（补机型族/核数）→ `createApplyTicket`（落库，stage=待审核）→ ITSM 建单。**这些是有副作用的提单动作**。

**容量/库存**当前**不在提单同步校验内**：真正消费容量在审批通过后的异步生产（`generator.getCapacity` → `Capacity().GetCapacity` → CRP 实时 `QueryCvmCapacity`）。生产判定口径：单可用区看该 zone `MaxNum`；分Campus/多可用区按各候选 zone `MaxNum` 逐 zone 累加分配（有效容量=各 zone `MaxNum` 之和）；`NotNeedVerifyCapacity`（如小额绿通）跳过容量。

约束：校验必须只读、无副作用；agent-server 与 woa-server 跨服务调用走 `ClientSet`。`bk_biz_id` 是 `create_biz_apply` 提单的 path 参数：MCP 工具 args 以 `path_param`/`body_param` 并列包裹（`tool_callbacks.go:62`），当前门禁仅 `extractApplyBody` 拆了 `body_param`，`path_param.bk_biz_id` 未取。

## Goals / Non-Goals

**Goals:**

- woa-server 提供与人工提单**等价**的提单前**只读**校验接口，覆盖：结构、权限、需求类型（机型/绿通/裁撤额度）、预测余量、GPU 计费时长。
- 在提单前**新增实时容量校验**，口径与现有异步生产判定一致。
- agent-server 申领门禁用真实校验替换占位；`bk_biz_id` 从工具调用 `path_param` 取得并传入校验。
- 抽取共享校验函数，保证"创建"与"校验"逻辑同源、零行为漂移。

**Non-Goals:**

- 不改变人工提单 `CreateBizApplyOrder` 的对外行为与异步生产流程。
- 不实现容量"预占/锁定"，仅做即时余量判定（存在 TOCTOU 时间窗，见风险）。
- 不改造 `tool_gate` 节点/路由结构与确认卡片协议（沿用 `agent-tool-confirm-gate`）。
- 不做前端卡片交互改动；不改 session 数据模型与 `forwardedProps`/state 注入链路。

## Decisions

### D1：woa-server 新增只读校验接口 `CheckBizApplyOrder`，复用 `ApplyReq`

业务视角 `POST /api/v1/woa/bizs/{bk_biz_id}/task/check/apply`，挂在 `bizService`（`service/task/service.go:198`）。入参复用 `types.ApplyReq`，`bk_biz_id` 取 path 覆盖，与 `CreateBizApplyOrder` 完全对齐。handler 流程：`DecodeInto` → path 覆盖 `BkBizId` → `input.Validate()` → `AuthorizeWithPerm(Biz, Create)` → 调用聚合校验。

- 备选（agent 直接调管理员视角 `/task/create/apply`）：否，越过业务权限与业务上下文。
- 备选（在 agent 侧本地校验）：否，校验依赖 woa 的预测/容量/CRP 客户端与机型库，必须在 woa 内执行。

### D2：抽取只读校验聚合方法，create 与 check 共用

从 `createApplyOrder` 抽取 `validateApplyOrder(kt, input)` = `verifyAccordingToRequireType` + `verifyResPlanDemand`；`createApplyOrder` 改为先调 `validateApplyOrder` 再走落单。check handler 在此基础上追加：

- GPU 计费时长：把 `Scheduler().VerifyCvmGPUChargeMonth` 暴露为可在 check 链复用的只读校验（它本就无副作用），并在 check 前先 `FillCVMAppliedCore`/机型信息填充（与底层一致，供容量与 GPU 校验用）。
- 实时容量校验（见 D3）。

这样"创建"路径行为完全不变（仅把已有校验提取为函数顺序调用），"校验"路径= 创建的全部前置校验 + GPU + 容量。

- 备选（check 内复制一份校验代码）：否，易与 create 漂移，违背一致性目标。

### D3：实时容量校验复用 `Capacity().GetCapacity`，口径对齐生产

逐子单（CVM 类）按 `spec` 构造 `GetCapacityParam`（region/zone/device_type/vpc/subnet/charge_type/require_type、`IgnorePrediction = !requireType.NeedVerifyResPlan()`、`DisableUpsertDB=true` 避免写库），调 `GetCapacity` 实时拿各 zone `MaxNum`：

- `requireType.NotNeedVerifyCapacity()` → 跳过。
- 单可用区（`spec.zone` 非分Campus）→ 该 zone `MaxNum >= replicas`。
- 分Campus / 多可用区（`zone==CvmSeparateCampus` 或 `len(zones)>1`）→ `Σ(候选 zone MaxNum) >= replicas`。
- 不足 → `pass=false`，reason `可申领容量不足，需要X台，可用Y台`。

复用现成 `GetCapacity`（已封装 CRP 实时 `QueryCvmCapacity` + 子网 IP + 单次最大量取最小值），与 `generator` 同源，避免另起容量算法。

- 备选（读本地库存快照表 device-capacity）：否，用户明确要求实时查 `Query` 接口，快照有滞后。

### D4：返回结构化 `{pass, reason}`，区分业务不通过与系统异常

新增结果类型 `CheckApplyOrderResult{Pass bool, Reason string}`。业务校验不通过 → HTTP 200 + `{pass:false, reason}`；下游/系统异常（DB、CRP、预测服务失败）→ 返回 error。这样 agent gate 的三态契约 `(ok, reason, err)` 能精确映射：

- err != nil → 系统异常文案；`!pass` → 业务原因文案；`pass` → 放行。

实现上：聚合校验内部把"业务校验失败"的 error 归一为 `{pass:false, reason}`（按错误码/错误类型判定，如 `errf.ResPlanVerifyFailed`、`InvalidParameter`、容量不足为业务类），其余 error 透出。

- 备选（任何校验失败都返回 error）：否，agent 无法区分"该重试的系统故障"与"该让用户改参的业务拒绝"。

### D5：agent-server 透传 `clientSet`，gate 持有 woa client

`createCvmApplyGate` 增加 woa client 依赖。`clientSet` 已在 `runtime.New(clientSet)` 持有，但未传到 `BuildGraph`。改造透传链：`runtime.New → newAGUIRunner → agent.BuildGraph → toolgate.EnabledGateHandlers(cfg, clientSet) → newCreateCvmApplyGate(client)`。`validateApply` 改为：从 ctx 取 `rid`/`bk_username` 构造 kit、用 `path_param` 的 `bk_biz_id` + `body_param` 构造 `ApplyReq`，调 `client.WoaServer().Task.CheckBizApplyOrder`，映射结果。

- 备选（gate 内用全局/包级 client）：否，违背依赖注入与可测试性。

### D6：`bk_biz_id` 从工具调用 `path_param` 提取

`create_biz_apply` 是业务视角接口 `/bizs/{bk_biz_id}/task/create/apply`，`bk_biz_id` 是 path 参数。MCP 工具把 args 以 `path_param`/`body_param` 并列包裹（`tool_callbacks.go:62` 已对二者做参数修复），LLM 实际提单时必须把 `bk_biz_id` 填入 `path_param`，否则真实 REST 调用无法路由。因此到 `OnResume`（确认后、即将提单）阶段，`args["path_param"]["bk_biz_id"]` 必然存在且与真实提单一致。

- 新增 `extractApplyPathParam(args)` 取 `path_param`，读 `bk_biz_id`（兼容 JSON number `float64`/字符串）。
- `OnResume` 把 `bk_biz_id` 与 `extractApplyBody(args)` 一起传给 `validateApply`，再随 `ApplyReq` 调 check 接口。
- 取不到 `bk_biz_id` → 按系统异常返回，提示无法确定业务（理论上不该发生，做防御）。

无需改 session 数据模型、`forwardedProps`、前端，也无需扩展 hitl handler 接口拿 graph state——`bk_biz_id` 自包含在工具调用参数里，来源最可靠。

- 备选（graph state 注入，复用 `session_tag` 链路）：否，需改 session 模型/前端/handler 签名，复杂且 `bk_biz_id` 本就在工具 args 内，舍近求远。

## Risks / Trade-offs

- [容量校验 TOCTOU：校验通过到真实提单/生产之间容量可能被他人消耗] → 仅作"提单前及早拒绝"，真实生产仍以 `generator` 实际容量为准；不承诺强一致预占。
- [check 与 create 校验漂移] → 通过 D2 抽取同源函数 + 单测覆盖 create/check 调用同一聚合校验。
- [校验接口耗时：实时 CRP 容量 + 预测多次外部调用，确认卡片确认后同步等待] → 逐子单容量可并发；为 check 调用设置合理超时与日志（含 rid）；必要时对子单数量限制（沿用 `ApplyReq` 上限）。
- [`bk_biz_id` 缺失] → 真实提单 path 必填，确认后阶段必然存在；仍取不到时按系统异常明确报错，不静默放行。
- [GPU/容量校验依赖机型信息填充顺序] → check 链显式先 `FillCVMAppliedCore`/机型填充，再校验，与底层一致。
- [跨服务权限上下文] → check 接口执行 `AuthorizeWithPerm`，kit.User 经 client header 透传，确保以真实用户鉴权。

## Migration Plan

- 纯增量：woa 新增 handler/路由/校验聚合（不改 create 对外行为）；client 新增方法；agent 透传 clientSet + 接入校验 + `path_param` 取 `bk_biz_id`。
- 分步上线：①woa check 接口（可独立验证）→ ②client 方法 → ③agent `validateApply` 接入（path_param 取 biz + 调 check）。
- 回滚：agent 侧 `validateApply` 回退为占位（或配置开关），woa check 接口保留无副作用，互不影响既有流程。

## Open Questions

- 容量 reason 是否需要返回"不足的 zone 明细 / 各 zone 余量"给用户，还是只给总量结论。
- check 接口的子单/并发上限与超时阈值取值。
