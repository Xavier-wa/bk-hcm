## 1. 常量与配置

- [x] 1.1 在 `pkg/criteria/constant/aiagent.go` 新增常量：`ToolNameCreateBizApply`、`ToolConfirmInterruptKey`（`tool_confirm`）、`StateKeyGateDecision`、决策枚举值 `proceed`/`reject`
- [x] 1.2 在 `pkg/cc/service.go` 的 `AgentToolsConfig` 新增 `ConfirmGate *AgentConfirmGateConfig`（yaml `confirmGate`），定义 `AgentConfirmGateConfig{ Enabled bool; Tools []string }`，并补默认值（缺省 `enabled: true`、`tools` 留空）

## 2. toolgate 通用包

- [x] 2.1 新建 `cmd/agent-server/logics/agent/toolgate/gate.go`：定义 `ConfirmPayload`/`ConfirmField`/`ConfirmResponse` 与 `Gate` 接口（`ToolName`/`BuildConfirm`/`OnConfirm`）
- [x] 2.2 新建 `toolgate/registry.go`：`Registry`（`map[string]Gate`）+ `Register`/`Lookup`/`IsGated`/`Active(tool, cfg)`，`Active` 实现"已注册 AND 配置启用 AND（列表为空或含工具名）"
- [x] 2.3 新建 `toolgate/message_op.go`：自定义 `ReplaceToolCallArgs` 实现 `graph.MessageOp`，按 ToolID 回写 assistant 消息中 tool_call 的 Arguments
- [x] 2.4 新建 `toolgate/node.go`：通用 `tool_gate` 节点——首次进入按 `Gate.BuildConfirm` 构造卡片并 `graph.Interrupt`（key=`tool_confirm:<tool>:<callID>`）；resume 解析 `ConfirmResponse` 调 `Gate.OnConfirm`
- [x] 2.5 在 `node.go` 实现 proceed 路径：解析最终参数（用户改过则写 `ReplaceToolCallArgs` 覆盖），不插用户消息，写 `StateKeyGateDecision=proceed`
- [x] 2.6 在 `node.go` 实现 reject 路径：补 `RoleTool{ToolID, Content:原因}`、清空 `StateKeyUserInput`、写 `StateKeyGateDecision=reject`
- [x] 2.7 新建 `toolgate/routing.go`：`MakeGateRoutingFunc`（`proceed→tool` / `reject→llm`）；并提供从 messages 提取受门禁 tool_call 的辅助函数

## 3. 申领场景 Gate 实现

- [x] 3.1 新建 `toolgate/create_cvm_apply.go`：`createCvmApplyGate` 实现 `Gate`，`ToolName()` 返回 `create_biz_apply`
- [x] 3.2 实现 `BuildConfirm`：剥 `body_param` 包装解析入参，生成只读 `fields`（单据级 + 按 suborder/resource_type 的 spec 字段）与可编辑 `args`
- [x] 3.3 实现 `OnConfirm`：解析 `action`，cancel 返回不放行；confirm 时确定最终参数（含用户修改）并调用 `validateApply` 占位校验
- [x] 3.4 实现 `validateApply` 占位函数：按需只读解析校验视图（resource_type/replicas/region 等），当前直接返回通过，注释 TODO 标注待接入库存/预测真实接口
- [x] 3.5 新建 `toolgate/registry_default.go`（或在 registry 内）`DefaultRegistry()`：注册 `createCvmApplyGate`

## 4. 图与路由集成

- [x] 4.1 修改 `graph_build.go`：构建 `Registry`（结合 `cc` 配置），`AddNode("tool_gate", toolgate.MakeNode(reg))`
- [x] 4.2 修改 `makeRoutingFunc`：tool_calls 含受门禁工具（`reg.Active`）且单独成批 → `tool_gate`；与其他工具/`human_confirm` 混批 → 返回 error；其余分支不变
- [x] 4.3 添加条件边 `llm → tool_gate` 与 `tool_gate → {tool, llm}`（`MakeGateRoutingFunc`）
- [x] 4.4 修改 `tool_confirm.go`：泛化 `confirmToolSet` 跳过受门禁工具（避免双重确认）

## 5. 自定义事件

- [x] 5.1 修改 `agui-event/translator.go`：新增 `buildToolConfirmPayload`，检测中断 key 前缀 `tool_confirm:` 并 emit `CustomEvent("tool.confirm", {value, checkpoint_id, lineage_id})`

## 6. 提示词调整

- [x] 6.1 调整 `cmd/agent-server/etc/prompts/system_prompt.md`：弱化申领相关 `human_confirm` 的强约束（由工程门禁兜底），避免与门禁重复确认

## 7. 单元测试

- [x] 7.1 `toolgate` 路由测试：受门禁/非受门禁/混批/配置开关（enabled、列表过滤、未注册不报错）
- [x] 7.2 `tool_gate` 节点测试：首次中断 key 格式；resume confirm（无改/有改 args 覆盖）；cancel 与校验失败补 tool result + 清空 user_input
- [x] 7.3 `ReplaceToolCallArgs` 测试：按 ToolID 正确覆盖、ToolID 不存在时无副作用
- [x] 7.4 `createCvmApplyGate` 测试：`BuildConfirm` 剥 body_param 与字段抽取；`OnConfirm` confirm/cancel 分支
- [x] 7.5 translator 测试：`tool_confirm:` 前缀 emit `tool.confirm`，非该前缀不 emit

## 8. 验证

- [x] 8.1 `gofmt`/`goimports`/`golangci-lint` 通过，`go build ./cmd/agent-server/...` 通过
- [x] 8.2 `go test ./cmd/agent-server/...` 通过
- [x] 8.3 `openspec verify-change agent-tool-confirm-gate`（或等价校验）通过
