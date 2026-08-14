## Context

主机申领子图账号解析链路：`START → account_select → llm → (hitl) → tool → after_tool_hitl → llm …`。`account_select` 是入口节点，每轮子图运行都先跑，负责把账号确定下来并写入两处：当轮 `RuntimeState[SessionAccountIDTempKey]`（供 `instruction.md` 的 `{{.AccountID}}` 渲染）与 session 后端 `SessionSelectedAccountIDStateKey`（供跨 Run `tryReuseAccountID` 复用）。

现有确定性通路：①前端点选卡片（resume 值即 `account_id`，精确匹配）；②总账号数 == 1 自动选中；③跨轮复用命中。**缺口**：多账号 + 用户自由文本选账号时 `matchAccountIDFromUserInput` 的"账号名包含"匹配失败 → 解析出空 `account_id`，`routeByAccountCount` default 分支仍把空值写进 `delta` 并路由 llm（`account_select.go:178`）。模型能从注入的账号选项 JSON 推断账号并继续，但代码没接住 → 下一轮重复弹选择。

已核对的关键事实与既有设施：
- `account_id` **不是任何业务工具的参数**（`pkg/api/woa-server` 推荐/提单请求体、`woatypes.ApplyReq` 均无账号字段）。其消费方仅提示词 `{{.AccountID}}` + 跨轮复用。
- "解开 proxy"已有 canonical 工具 `toolproxy.ResolveToolCall(tc)`（提单门禁 `createCvmApplyGate`、推荐结果解析都在用），可从 `execute_tool` 信封取真实工具名与内层 `parameters`。
- 工具 Call 的 ctx 携带 invocation（`confirmTool.Call`、`MakeDynamicToolFilter` 均用 `agent.InvocationFromContext(ctx)`）。
- `BeforeToolResult.CustomResult`（trpc-agent-go v1.9.1）不为 nil 时"跳过工具执行并返回该结果"——即 `BeforeTool` 可硬拦截。
- 本地声明工具经 `buildSkillTools` map 注册（`skill_load` / `human_confirm`），同时供 LLM 节点与 tool 节点使用。
- 提单工具 `create_biz_apply`（`constant.ToolNameCreateBizApply`）已被 `createCvmApplyGate`（`hitl.Handler`）人工确认门禁守护；推荐类有 `enumor.ToolName.IsRecommend()`。

## Goals / Non-Goals

**Goals:**
- 一期：堵住多账号自由文本未命中时的空 `account_id` 下传（线上"重复弹账号选择"故障的代码侧根因），同时保持"多账号一律弹卡片"的既有产品预期不变。
- 二期：给"多可用账号 + 自由文本"提供确定性接住点（`select_account`），并以场景级门禁强制持久化。
- 全程复用既有设施（`ResolveToolCall` / `injectAccountIDToRuntimeState` / `listAccountsForBiz` / `IsRecommend`），不在公共基础设施层塞业务逻辑。

**Non-Goals:**
- 不给业务工具新增 `account_id` 参数（它本就不需要）。
- 不改前端点选账号协议、不改对外 restful 接口、不改 tool_proxy 的 search/schema/token 机制。
- 不引入对模型自然语言输出的文本解析。
- 不改动 `createCvmApplyGate` 的人工确认流程。

## Decisions

### D1（一期）保持"按账号总数"决策 + 修空值下传
- `routeByAccountCount` 决策基准为**账号总数**（与改动前一致，避免改变"多账号弹卡片"的产品预期）：`count==0 → fallback`；`count==1 → 自动选中并落库、路由 llm`；`count>=2 → 一律弹卡片`（无论其中可用账号为 0/1/多个）。可用性判定（`vendor==tcloud-ziyan`）只决定卡片内选项是否禁用，不改变是否弹卡片。
- default（多账号 HITL）分支：`resolveSelectedAccountID` 返回空时，不再写 `delta[SessionAccountIDTempKey]=""`（保持缺省，不污染复用判定）。这是一期真正修复的故障点。
- 备选一：按"可用账号数"决策（总数≥2 仅 1 可用则自动选中、全不可用则 fallback）。放弃：会让多账号场景不再弹卡片，与产品预期冲突（用户需看到全部账号及禁用原因再选）。
- 备选二：加强 `matchAccountIDFromUserInput` 的模糊匹配。放弃：脆弱、不可控，且不解决多可用账号根因（由 D2/D3 结构化接住）。

### D2（二期）`select_account` 结构化上报工具
- 注册进 `buildSkillTools`（本地声明工具，不经 proxy，模型直接以工具名调用，检测无需解开 proxy）。
- 携带结构化 `account_id`；执行时校验属于当前 `bk_biz_id` 下 `tcloud-ziyan` 账号（复用 `listAccountsForBiz` + vendor 过滤）；校验通过复用 `injectAccountIDToRuntimeState(ctx,inv,id,sessSvc,appName)` 一次完成注入 + 落库，返回 ack；失败返回可读错误、不落库。
- 落库实现二选一（实现期定）：
  - (a) 自带执行体 Call（闭包捕获 `cloudClient`/`sessSvc`/`appName`，从 ctx 取 inv/bk_biz_id）——内聚、同轮即时注入；
  - (b) 纯声明工具 + `AfterNodeCallback`（照搬 `MakeSkillLoadAfterToolCallback`，在 tool 节点后落库）。
  - 倾向 (a)；无论哪种，`buildSkillTools` 都需新增 `cloudClient`/`sessSvc`/`appName` 依赖穿线（现仅收 `skillRepo`）。

### D3（二期）场景级 `BeforeTool` 账号门禁（不放 toolproxy）
- 在 host_apply 的 `genLLMNodeOptions` 中，挨着 `MakeParamFixCallbacks` 追加账号门禁 `BeforeTool` 回调。
- 逻辑：用 `ResolveToolCall`（或等价解开 `ExecuteToolParams` 信封）拿真实工具名 `name`；若 `name.IsRecommend() || name == create_biz_apply`，且 `InvocationFromContext(ctx)` 的 `RuntimeState[SessionAccountIDTempKey]` 为空 → 返回 `BeforeToolResult{CustomResult: 引导先调 select_account}` 硬拦截。
- 受控集合仅含推荐类 + 提单工具，绝不含只读查询工具。
- 与 `createCvmApplyGate` 分工：账号门禁只在"账号未解析"时拦截并让模型补调 `select_account`（路由回 llm，无用户中断）；账号已解析后放行，仍由 `createCvmApplyGate` 负责提单人工确认。二者条件正交、不双重拦。
- 备选：塞进 `toolproxy/execute.go`。放弃：`toolproxy` 跨场景公共基础设施，注入业务逻辑属分层污染；且 `ResolveToolCall` 已能在场景层解开 proxy。

## Risks / Trade-offs

- [门禁误伤只读工具] → 受控集合严格限定 `IsRecommend()` + `create_biz_apply`，单测覆盖"查询工具不被拦截"。
- [与提单确认门禁叠加] → 账号门禁条件（account_id 为空）与提单门禁（人工确认）正交；账号已解析时账号门禁必放行。
- [模型不调 select_account 直接申领] → D3 门禁拦截并返回引导错误，模型据错补调，形成自愈闭环。
- [模型上报越权/禁用 account_id] → D2 强制校验属于本业务可用账号，失败返回可读错误、不落库。
- [依赖穿线] → `buildSkillTools` 新增依赖需与 `NewAccountSelectNode` 注入方式一致，避免注入不全导致落库静默失败（已有 `sessSvc==nil` 的告警兜底）。
- [同一 assistant 消息内并行调用 select_account 与业务工具] → 门禁读 RuntimeState 可能早于 select_account 写入；提示词要求"先选账号再申领"，且 account_select 入口节点已覆盖绝大多数场景，此为边缘情况，实现期在门禁文案中提示重试即可。

## Migration Plan

- 一期与二期在同一 change 内交付；上线顺序上一期(D1)独立可验证，二期(D2/D3)叠加。
- 门禁可通过受控集合为空快速降级；`select_account` 工具与 D1 快路径互相独立，可分别回退。

## Open Questions

- 提单工具是否需要纳入受控集合，取决于 `create_biz_apply` 在账号未解析时是否可能被直接调用（正常经 llm 需先过 account_select）；默认纳入更安全。
- 受控集合是否配置化（cc 配置）以便运营调整，或先硬编码。
