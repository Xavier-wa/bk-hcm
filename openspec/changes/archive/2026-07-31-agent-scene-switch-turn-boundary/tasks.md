## 1. 独立修复（不依赖切换功能，可先合入验证）

- [x] 1.1 `makeSceneDispatchNode` 每次执行时把 `rest.RidFromContext(ctx)` 写入 `SessionRidStateKey`，且所有返回分支都带上该 delta（修复 D6：第二轮起子图 mapper 日志 rid 为旧值）
- [x] 1.2 单测：构造带旧 `session_rid` 的 state，断言节点返回的 delta 中 `session_rid` 被刷新为 ctx 中的 rid
- [x] 1.3 `service.go` 把 `reconcileSessionTag` 从 `asyncIncrContentCount` 中拆出，改由 `sessionCodeMiddleware` 在 `next.ServeHTTP` 返回后用独立 goroutine 触发（仅 AGUI 请求）——已随 `02786079a`（2026-07-23）上线
- [x] 1.4 `reconcileSessionTag` 去掉 `originalTag != ""` 短路，以及其上方「目前会话标签不允许修改……未来需要支持修改标签时，需要修改这里」的 TODO，改为「checkpoint 中的 tag 与 `originalTag` 不同才回写 + 刷新 resolver 缓存」；相同则记 Info 直接返回
- [x] 1.5 更新 `reconcileSessionTag` 的函数注释说明差异回写语义；`asyncReconcileSessionTag` 上已有的时机注释保留，补一句「客户端断连时本轮回写落空，由下一轮差异回写自愈，不做重试」

## 2. 结构重构：合并意图识别节点（本步骤要求行为零变化）

- [x] 2.1 `intent/node.go` 把 `MakeIntentRecognitionNode` 改写为纯函数 `Classify(ctx, mdl, promptStore, messages, contextWindowSize) enumor.IntentType`：返回分类结果、不写 state、不返回 error；`promptStore` 为 nil 或 prompt 缺失时返回 `chat` 并记 Error（原实现返回 error，此处改为降级）
- [x] 2.2 保留 `LLMParseIntent`、`extractContextMessages`、`hasUserMessage` 不变；删除 `intentState`
- [x] 2.3 `intent/node_test.go` 全部用例平移到 `Classify`：断言返回值而非 `graph.State`；补一条「promptStore 为 nil 返回 chat 且不 panic」用例
- [x] 2.4 `graph_build.go` 删除 `intent_recognition` 节点注册、`scene_dispatch → intent_recognition` 条件边分支、`intent_recognition → scene_dispatch` 静态边；`makeSceneDispatchNode` 改为接收 `mdl` / `promptStore` / `contextWindowSize`
- [x] 2.5 `pkg/criteria/enumor/aiagent.go` 删除 `MainGraphAgentNodeIntentRecognition` 及其 `Validate` 分支
- [x] 2.6 `message.go` 的 `BuildFallbackResumeDelta` 历史重建判据由 `agentstate.ParseIntent(state)` 改为 `session_tag`；更新其中解释 `StateKeyIntent` 的大段注释
- [x] 2.7 `graph_build.go` 的 `unsupportedIntentFallbackMessage` 改读 `StateKeySessionTag`
- [x] 2.8 删除 `cmd/agent-server/logics/agent/state/intent.go` 与 `constant.StateKeyIntent`；确认全仓库无残留引用
- [x] 2.9 更新 `graph_build.go` 顶部拓扑注释：新的主图形状、「scene_dispatch 只在轮次边界执行」的隐式契约、唯一的环被 `fallback` interrupt 阻断（D2/D3）
- [ ] 2.10 回归：本步骤合入后现有单测应全绿，手工验证无标签会话首轮与有标签会话场景内多轮行为与改动前一致

## 3. 常量

- [x] 3.1 `pkg/criteria/constant/aiagent.go` 新增 `StateKeySceneDispatchNext`（scene_dispatch 决策键，取值为目标节点名）
- [x] 3.2 `pkg/criteria/constant/aiagent.go` 新增 `SceneSwitchedCustomEventName = "scene.switched"`

## 4. scene_dispatch 决策矩阵与路由改造

- [x] 4.1 抽出决策函数：输入 `(tag, classified)`，输出 `(nextNode, newTag, shouldEmitSwitch)`，纯函数、无 ctx 无 I/O，便于表驱动测试
- [x] 4.2 在决策函数中实现 5 个分支（tag 受支持 × 分类切换/相同/不受支持，tag 为空 × 分类受支持/不受支持）
- [x] 4.3 改写 `makeSceneDispatchNode`：调 `intent.Classify` 拿分类结果 → 调决策函数 → 写 delta（`session_tag`、`StateKeySceneDispatchNext`、`session_rid`）→ 需要时发事件
- [x] 4.4 改写 `makeSceneDispatchRoutingFunc` 为纯查表：只读 `StateKeySceneDispatchNext` 并映射到目标节点；空值或非法节点名时路由 `fallback` 并记 Warn，不返回 error
- [x] 4.5 在 `Classify` 调用处记录耗时、分类结果与 rid，使「每轮一次分类」的延迟成本可度量

## 5. scene.switched 事件

- [x] 5.1 在 `logics/agent/message` 包新增发送函数：用 `graph.GetEventEmitterWithContext(ctx, state)` + `graph.NewNodeCustomEvent`（EventType 取 `SceneSwitchedCustomEventName`，Payload 含 `from`/`to`）发出事件，emit 失败记 Warn 不返回 error
- [x] 5.2 `makeSceneDispatchNode` 在决策结果 `shouldEmitSwitch=true`（`from != "" && from != to`）时调用该函数
- [x] 5.3 从 `graph.StateKeyExecContext` 取 `InvocationID` 填入事件；取不到时留空（对齐 `EmitFallbackMessage` 的既有做法）
- [x] 5.4 确认 `service/agui-event/translator.go` 无需改动：框架 translator 的 `graphNodeCustomEvents → handleCustomEvent` 已无条件把 category=custom 的 node custom event 转成同名 CUSTOM 事件

## 6. 测试

- [x] 6.1 决策函数表驱动单测：覆盖 5 个分支全部组合
- [x] 6.2 `makeSceneDispatchNode` 单测：切换场景下断言 delta 中 `session_tag` 为新场景、`StateKeySceneDispatchNext` 为新子图节点；分类失败（mock LLM 报错）时断言保持原 tag
- [x] 6.3 `makeSceneDispatchRoutingFunc` 单测：正常查表、决策键缺失兜底到 `fallback`、非法节点名兜底到 `fallback`
- [x] 6.4 主图拓扑单测：`BuildGraph` 后断言不存在 `intent_recognition` 节点，且 `scene_dispatch` 的入边集合仅为 `START` 与 `fallback`，锁住 D2/D3 的隐式契约
- [x] 6.5 fallback 判据替换的等价性单测（三个案例各一条）：无标签 + chat → 重建历史；有标签 + 子图跑完 → 不重建；有标签 + `account_select` 无可用账号 → 不重建
- [x] 6.6 改写 `graph_build_test.go` 中 7 个以 `StateKeyIntent` 构造输入的既有测试，输入改用 `session_tag`：`TestMakeSceneDispatchNode`、`TestMakeSceneDispatchRoutingFunc`、`TestSceneDispatchNodeThenRouting`、`TestUnsupportedIntentFallbackMessage`、`TestBuildFallbackResumeDeltaClearsUnsupportedIntentHistory`、`TestBuildFallbackResumeDeltaClearsUserInput`、`TestNoUsableAccountFallbackMessageNotDuplicated`
- [x] 6.7 解决 `agent` 包的 mock model 缺口：现有 `mockModel` 是 `intent` 包内的未导出类型，跨包用不了。优先把需要 LLM 的用例压到最少（决策矩阵全部走 6.1 的纯函数测试），确有需要时在 `agent` 包内自建或抽到共享 testutil
- [x] 6.8 `go build ./... && go test ./cmd/agent-server/... ./pkg/criteria/...` 全部通过
- [x] 6.9 `grep -r StateKeyIntent` 确认无残留

## 7. 手工验证

- [ ] 7.1 `host_apply` 会话子图跑完后输入「帮我查下我有哪些主机」→ 一次交互内返回资源查询结果，SSE 中出现 `scene.switched` 事件，DB `session_tag` 当轮变为 `resource_query`
- [ ] 7.2 `resource_query` 会话中说「那帮我申领 3 台」→ 切到 `host_apply`；若此前选过账号则不再弹选账号卡片
- [ ] 7.3 切换后立刻说「回到刚才的申领」→ 切回 `host_apply`，账号不用重选，方案重新生成
- [ ] 7.4 场景内追问「改成 16 核」→ 不切换，行为与现网一致
- [ ] 7.5 场景内说「谢谢」→ 留在原场景由场景 LLM 回答，不清历史、不清 tag、不出未支持提示文案
- [ ] 7.6 停在提单门禁卡片上输入「帮我查下我有哪些主机」→ 初版**不切换**，仍按现有 cancel 路径由原场景 LLM 处理（确认无异常，不是 bug）
- [ ] 7.7 无标签会话首轮全流程 → 与现网完全一致，不发 `scene.switched`
- [ ] 7.8 测试环境实测意图分类调用的 P95 耗时，评估「每轮多一次调用」是否可接受；超预期则评估纯确认词短路

## 8. 收尾

- [ ] 8.1 与前端确认 `scene.switched` 的 wire shape：接受框架原生形状（value 为对象、业务字段在 `payload` 下、不能 `JSON.parse`），还是在 `customTranslator` 里归一化成 JSON 字符串以对齐现有 5 个 CUSTOM 事件（见 design.md Open Questions）。提示文案已确认不需要后端下发（前端有 `SESSION_TAG_NAME` 映射）
- [ ] 8.2 清理 `agent-intent-routing` / `intent-recognition-node` 两份 spec 中先前重构遗留的陈旧内容（仍在描述 `scene_dispatch → llm` 直连、`resource_query` 不受支持、`AgentIntentConfig.modelName` 等），本变更的 delta 只覆盖了直接相关部分
- [ ] 8.3 在 `Agent场景切换方案.md` 中标注初版已实现范围与二期剩余项（`handoff` 工具、中断点切换、短路优化）
