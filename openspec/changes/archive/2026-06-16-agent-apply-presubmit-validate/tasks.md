## 1. woa-server 抽取同源只读校验

- [x] 1.1 在 `cmd/woa-server/types/task` 新增校验结果类型 `CheckApplyOrderResult{Pass bool, Reason string}`（含注释与字段说明）
- [x] 1.2 从 `service/task/scheduler.go` 的 `createApplyOrder` 抽取只读校验聚合方法 `validateApplyOrder(kt, input)` = `verifyAccordingToRequireType` + `verifyResPlanDemand`，并改 `createApplyOrder` 先调该方法（保证创建路径行为零变化）
- [x] 1.3 将底层 `Scheduler().VerifyCvmGPUChargeMonth` 暴露为可在校验链复用的只读校验（确认其无副作用）
- [x] 1.4 提供校验前的机型信息填充入口（复用 `FillCVMAppliedCore`/机型填充），顺序与底层创建路径一致，供 GPU 与容量校验使用

## 2. woa-server 实时容量校验

- [x] 2.1 实现逐 CVM 类子单容量校验：按 `spec` 构造 `GetCapacityParam`（region/zone/device_type/vpc/subnet/charge_type/require_type、`IgnorePrediction = !requireType.NeedVerifyResPlan()`、`DisableUpsertDB = true`），调用 `Capacity().GetCapacity`
- [x] 2.2 实现容量判定口径：`NotNeedVerifyCapacity()` 跳过；单可用区比对该 zone `MaxNum`；分 Campus/多可用区按各候选 zone `MaxNum` 求和；不足生成 `可申领容量不足，需要X台，可用Y台` 原因
- [x] 2.3 组装聚合校验：结构 → 需求类型 → 预测余量 → GPU 计费时长 → 容量；将业务类失败（预测失败、参数非法、容量不足）归一为 `{pass:false, reason}`，系统异常透出 error

## 3. woa-server 校验接口与路由

- [x] 3.1 在 `service/task/scheduler.go` 新增 handler `CheckBizApplyOrder`：`DecodeInto` → path 覆盖 `BkBizId` → `input.Validate()` → `AuthorizeWithPerm(Biz, Create)` → 调用聚合校验 → 返回 `CheckApplyOrderResult`
- [x] 3.2 在 `service/task/service.go` 的 `bizService` 注册路由 `POST /bizs/{bk_biz_id}/task/check/apply`
- [x] 3.3 确认 handler 全程只读：不落库、不建 ITSM、不写库存快照

## 4. woa-server client

- [x] 4.1 在 `pkg/client/woa-server/task.go` 新增 `CheckBizApplyOrder(kt, bizID, req)` 方法，复用 `ApplyReq` 入参，返回 `CheckApplyOrderResult`

## 5. agent-server 依赖透传

- [x] 5.1 改造 `clientSet` 透传链：`runtime.New → newAGUIRunner → agent.BuildGraph → toolgate.EnabledGateHandlers(cfg, clientSet) → newCreateCvmApplyGate(client)`
- [x] 5.2 `createCvmApplyGate` 增加 woa client 字段并经构造函数注入

## 6. agent-server 门禁接入真实校验

- [x] 6.1 在 `toolgate/create_cvm_apply.go` 新增 `extractApplyPathParam(args)`，从 `args["path_param"]["bk_biz_id"]` 提取 `bk_biz_id`，兼容 `float64`/字符串
- [x] 6.2 调整 `OnResume`：将 `bk_biz_id`（path_param）与 `extractApplyBody`（body_param）一起传入 `validateApply`
- [x] 6.3 用真实实现替换 `validateApply` 占位：构造 kit（ctx 取 `rid`/`bk_username`）、构造 `ApplyReq`、调用 `client.WoaServer().Task.CheckBizApplyOrder`，按 `(ok, reason, err)` 三态映射
- [x] 6.4 `bk_biz_id` 取不到时返回系统异常（无法确定业务），不静默放行

## 7. 测试

- [x] 7.1 woa：聚合校验单测（业务/系统异常归一 `businessRejectReason` 全分支覆盖）；聚合链需 DAO/logics/authorizer 依赖，本包无 mock 脚手架，按团队规范不引入大范围 mock，创建与校验同源由代码结构（`validateApplyOrder` 复用）保证
- [x] 7.2 woa：容量原因 `buildCapacityReason` 单测覆盖多可用区拼装与需要/可用展示；GetCapacity/GetZone 依赖下游，留待集成验证
- [x] 7.3 agent：`extractApplyPathParam` 与 `validateApply` 三态映射单测（pass / 业务不通过 / 系统异常 / bk_biz_id 缺失）
- [x] 7.4 运行 `go test ./cmd/woa-server/... ./cmd/agent-server/... ./pkg/client/woa-server/...` 并修复 lint
