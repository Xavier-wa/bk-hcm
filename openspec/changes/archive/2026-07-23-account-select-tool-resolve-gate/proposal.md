## Why

主机申领子图在多账号场景下，若用户**通过自由文本**（如"选择自研云账号"）选账号，`account_select` 的规则匹配（`matchAccountIDFromUserInput`：精确 ID / 账号名包含）无法把文本映射成 `account_id`，于是既不写 `RuntimeState[account_id]`、也不落库（`account_select.go:178` 甚至把空 `account_id` 继续下传）。模型虽凭上下文里的账号选项 JSON 推断出了账号并继续申领，但**代码侧从未接住这个 `account_id`**，导致同一会话下一轮再申领时 `tryReuseAccountID` 三源皆空，重复弹出账号选择中断（本次线上问题的根因）。

关键前提（已核对代码）：`account_id` **不是任何业务工具的参数**——`pkg/api/woa-server` 的推荐/提单请求体、`woatypes.ApplyReq` 都没有账号字段。因此不能"从业务工具入参里捞账号"（方案 B 天然不可行）；`account_id` 的真实消费方只有提示词 `{{.AccountID}}`（对应 `instruction.md` 的"禁止再次向用户确认账号信息"）与跨轮复用。要让"模型确定的账号"被确定性接住，需要一个**结构化上报**入口，而非对自由文本做脆弱匹配。

## What Changes

本提案分两期，但**在同一个 change 内一起交付**。

### 一期（外科手术式，直接修复线上故障）
- `account_select` 增加**"单一可用（enabled = `vendor==tcloud-ziyan`）账号自动选中"快路径**：当业务下可用账号恰为 1 个时自动选中并落库（即使账号总数 ≥ 2），无需 HITL、无需模型。覆盖本次"2 账号仅 1 可用"的场景。
- 修复多账号自由文本**未匹配时静默下传空 `account_id`** 的问题（`routeByAccountCount` default 分支）：`account_id` 为空时不再写入 `delta[SessionAccountIDTempKey]=""`，避免污染后续复用判定。
- 明确 `count(可用账号)==0 && 账号总数≥1` 的处理：路由到 `fallback`，不再进入只含禁用选项的多账号 HITL 死路。

### 二期（多可用账号场景的通用兜底）
- 新增**面向 LLM 的本地声明工具** `select_account(account_id)`（注册进 `buildSkillTools`，与 `human_confirm` / `skill_load` 同类，不经 tool_proxy）：模型确定账号后上报**结构化 `account_id`**；执行时校验其属于当前业务可用（`tcloud-ziyan`）账号，随后复用 `injectAccountIDToRuntimeState` 同时写当轮 `RuntimeState` 并落库。
- 新增**场景级 `BeforeTool` 门禁**（仅 host_apply）：在 `genLLMNodeOptions` 中挨着 `MakeParamFixCallbacks` 注册；用**现有** `toolproxy.ResolveToolCall` 解开 proxy 拿真实工具名，用 `InvocationFromContext` 读 `RuntimeState[account_id]`，为空且命中推荐类（`enumor.ToolName.IsRecommend()`）/提单（`create_biz_apply`）工具时，返回 `BeforeToolResult.CustomResult` 硬拦截并引导先调 `select_account`。门禁**不放** `toolproxy/execute.go`（该包为跨场景公共基础设施）。
- 更新 host_apply 提示词/skill：确定账号后必须先调 `select_account`；命中门禁错误时补调。

> 说明：因 `account_id` 非业务工具参数，二期门禁的作用是"逼模型调一次 `select_account` 以完成持久化"（记账保证），而非补一个工具必需参数。提单工具 `create_biz_apply` 已被 `createCvmApplyGate` 人工确认门禁守护，二期门禁需与其**分工不冲突**（账号门禁只在账号未解析时拦截，且路由回 llm，不触发确认中断）。

## Capabilities

### New Capabilities
- `cvm-account-tool-resolution`: 主机申领账号的"模型结构化上报 + 场景级前置门禁"能力——`select_account` 工具接住模型确定的 `account_id`（校验+注入+落库），以及基于 `ResolveToolCall` 的 `BeforeTool` 门禁在账号未解析时拦截申领类工具。

### Modified Capabilities
- `cvm-account-select`: 单账号自动选择改为基于"可用账号数"；新增自由文本未匹配时不下传空 `account_id`、以及 0 可用账号路由 fallback 的行为。

## Impact

- 代码（一期）：
  - `cmd/agent-server/logics/agent/cvm_apply/account_select.go`：`routeByAccountCount` 增加"单一可用账号自动选中"分支、修空 `account_id` 下传、0 可用账号路由 fallback。
- 代码（二期）：
  - `pkg/criteria/enumor/aiagent.go`：新增 `ToolName` = `select_account`（及 `Validate`）；复用已有 `IsRecommend()`，如需可加"受账号门禁工具集合"判定（含 `create_biz_apply`）。
  - `pkg/criteria/constant/aiagent.go`：`select_account` 工具名常量、门禁引导文案常量。
  - `cmd/agent-server/logics/agent/cvm_apply/select_account.go`（新增）：`select_account` 工具声明 + 校验（复用 `listAccountsForBiz` + `tcloud-ziyan` 过滤）+ 落库（复用 `injectAccountIDToRuntimeState`）。
  - `cmd/agent-server/logics/agent/graph_build.go`：`buildSkillTools` 增加 `cloudClient` / `sessSvc` / `appName` 依赖并注册 `select_account`；`genLLMNodeOptions` 追加场景级账号门禁 `BeforeTool` 回调。
  - `cmd/agent-server/etc/prompts/instruction.md` 及 host_apply skill：补充 `select_account` 调用契约与门禁说明。
- API / 协议：无对外 restful 接口变化；前端 `forwardedProps.resumeValue` 点选账号协议不变；`SessionAccountIDTempKey` / `SessionSelectedAccountIDStateKey` 常量不变。
- 依赖：无新增第三方依赖；账号校验复用 cloud-server `ListByUsageBizID`；"解开 proxy"复用 `toolproxy.ResolveToolCall`。
- 风险：
  - 门禁误伤只读查询工具（如 `list_config_cvm_device`）→ 受控集合严格限定为 `IsRecommend()` + `create_biz_apply`。
  - 与 `createCvmApplyGate` 叠加 → 账号门禁仅在 `RuntimeState[account_id]` 为空时拦截并路由回 llm，不与人工确认中断冲突。
  - `select_account` 依赖穿线（`buildSkillTools` 需新增 `cloudClient`/`sessSvc`/`appName`）→ 与 `NewAccountSelectNode` 的注入方式保持一致。
