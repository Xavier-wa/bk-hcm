## 1. 一期 — account_select 修复（直接修故障）

- [x] 1.1 `account_select.go` `routeByAccountCount` 保持"按账号总数"决策：`count==1` 自动选中并落库路由 `llm`（注：一期一度改为"按可用账号数"，因与"多账号弹卡片"产品预期冲突已回退）
- [x] 1.2 `count>=2` 一律弹卡片（无论可用账号为 0/1/多个）；可用性判定只影响卡片内选项禁用态，不改变是否弹卡片
- [x] 1.3 `count==0` 路由 `fallback`，由主图兜底回复
- [x] 1.4 default 分支：`resolveSelectedAccountID` 返回空时不再写 `delta[SessionAccountIDTempKey]=""`（一期真正修复的故障点）
- [x] 1.5 一期单测：总数==1→自动选中；总数==0→fallback；自由文本未命中→不下传空账号（多账号弹卡片路径含 interrupt，由集成覆盖）

## 2. 二期 — 常量与枚举

- [x] 2.1 工具名放 `constant`（与 `skill_load`/`human_confirm` 同类，非 MCP 业务工具）：`constant.SelectAccountToolName`；不入 `enumor.ToolName`（语义不符，避免污染 `IsRecommend` 路由枚举）
- [x] 2.2 复用已有 `IsRecommend()`；受账号门禁工具集合判定 `isAccountGatedTool`（推荐类 + `create_biz_apply`）实现在门禁处（`account_gate.go`），避免 `enumor` 反向依赖 `constant`
- [x] 2.3 `pkg/criteria/constant/aiagent.go` 新增 `SelectAccountToolName` / `SelectAccountArgKey` / `SelectAccountRequiredMsg` 常量

## 3. 二期 — select_account 工具（结构化上报 + 校验 + 落库）

- [x] 3.1 新增 `cmd/agent-server/logics/agent/cvm_apply/select_account.go`：工具 `Declaration`（入参 `account_id`，含中文契约说明）
- [x] 3.2 自带执行体 `Call`：从 ctx 取 `inv`/`bk_biz_id`，校验 `account_id` 属于本业务 `tcloud-ziyan` 可用账号（`listAccountsForBiz` + `isEnabledAccountID`）
- [x] 3.3 校验通过：复用 `injectAccountIDToRuntimeState(ctx,inv,id,sessSvc,appName)` 注入 + 落库，返回成功 ack
- [x] 3.4 校验失败：返回可读提示（不写 RuntimeState、不落库、不返回 error），按规范打印含 rid 日志

## 4. 二期 — 依赖穿线与工具注册

- [x] 4.1 `buildSkillTools` 为多子图共享，保持签名不变；改在 `buildHostApplySubgraph` 内注册 `select_account`（用 `clientSet.CloudServer()` / `sessionSvc` / `agentName`），避免 `select_account` 泄漏到 resource_query
- [x] 4.2 `select_account` 注册进 host_apply 的 `skillTools`（同一 map 同时传给 `AddLLMNode` 与 `AddToolsNode`，即 LLM 与 tool 节点均可见）
- [x] 4.3 `select_account` 经 `injectAccountIDToRuntimeState` 写入 `SessionAccountIDTempKey`(RuntimeState) + `SessionSelectedAccountIDStateKey`(session 后端)，与 run 起点回填、account_select 快路径三处一致

## 5. 二期 — 场景级 BeforeTool 账号门禁（不放 toolproxy）

- [x] 5.1 `genLLMNodeOptions` 加可变参 `extraBeforeTool`（共享子图不受影响）；host_apply 传入 `makeAccountGateBeforeTool()`，紧随 `MakeParamFixCallbacks` 追加
- [x] 5.2 门禁用 `toolproxy.ResolveToolCall` 取真实工具名；`agent.InvocationFromContext` 读 `RuntimeState[SessionAccountIDTempKey]`（`accountResolved`）
- [x] 5.3 命中受控集合（`isAccountGatedTool`：`IsRecommend()` / `create_biz_apply`）且账号为空 → 返回 `BeforeToolResult.CustomResult`(`SelectAccountRequiredMsg`)
- [x] 5.4 只读查询/非受控工具放行；账号已解析放行（提单确认仍由 confirm gate 处理，不双重拦截）；`select_account` 自身永不被拦截

## 6. 二期 — 提示词契约

- [x] 6.1 `instruction.md` 增补：账号为空且已确定账号时必须先调 `select_account`；`{{.AccountID}}` 已填充则跳过
- [x] 6.2 命中门禁提示时补调 `select_account` 再重试——写入全局 `instruction.md`（host_apply skill 内容由 bkaidev 远程同步，非本仓管理，故落在对 host_apply 生效的 instruction）

## 7. 二期 — 测试

- [x] 7.1 `select_account` 校验谓词单测 `TestIsEnabledAccountID`（可用/非 ziyan/未知）；`Call` 的 client 依赖路径由集成覆盖
- [x] 7.2 门禁单测（`account_gate_test.go`）：未选账号拦截、已选放行、查询/非受控放行、`select_account` 不拦截、proxy 信封与直连均覆盖
- [x] 7.3 回归单测 `TestRegression_SelectedAccountReusedNextTurn`：选账号落库后新一轮复用不再重复选择

## 8. 验证

- [x] 8.1 `gofmt -l` 全部干净；新增文件无 golangci 告警（`account_select.go`/`graph_build.go` 的 argument-limit 与 magic-number 为改动前既有、未新增）
- [x] 8.2 触及包 `go vet` 通过；`cvm_apply` `go test` 通过；`agent` 包新增门禁单测全部通过（`TestUnsupportedIntentFallbackMessage` 为既有失败、与本次无关；`go build ./...` 全量链接受本机 sqlite-vec CGO 环境限制，非本次代码问题）
- [x] 8.3 `openspec validate account-select-tool-resolve-gate --strict` 通过
