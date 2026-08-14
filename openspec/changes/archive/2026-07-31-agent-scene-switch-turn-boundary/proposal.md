## Why

会话的场景标签（`session_tag`）一旦被 `intent_recognition` 识别并提交，就永久锁死：`scene_dispatch` 只要看到受支持的 tag 就直达对应子图，`intent_recognition` 此后再也不会被调用，`reconcileSessionTag` 也用 `originalTag != ""` 显式关闭了回写。用户在「主机申领」会话里想查一下已有资源，只能新开会话；在「资源查询」会话里查到库存想直接申领，同样要新开会话。

完整方案（见 `Agent场景切换方案.md`）的目标是「任意时刻可切换」，包含停在 HITL 卡片上的时刻，需要引入 `handoff` 声明工具、子图 `scene_switch` 节点、跨子图边界传递等一整套机制。本提案只做**初版**：仅在一个子图完整跑完、控制权回到主图之后的**轮次边界**上做意图重新识别与场景切换。

这个收窄带来一个决定性的简化：**`scene_dispatch` 只会在轮次边界执行**。当上一轮停在子图内部的 HITL 中断上时，框架 resume 到的是子图的中断节点，主图 `scene_dispatch` 根本不会运行（方案文档 §2.5 已验证）。因此「子图跑完之后」这个条件不需要任何额外的探测——`scene_dispatch` 执行本身就是它的充要条件。初版由此**不需要** `handoff` 工具、不需要子图 `scene_switch` 节点、不需要改子图 output mapper、不需要改 `fallback`。

顺带解决一个长期存在的职责重叠：`StateKeyIntent` 与 `StateKeySessionTag` 在有 tag 的会话里逐字相等（`scene_dispatch` 每轮把前者刷成后者），前者之所以还是一个持久化 state key，只是因为分类结果要跨 `intent_recognition` 和 `scene_dispatch` 两个节点传递。而 `intent_recognition` 本就不是框架意义上的 LLM 节点，只是一个内部调 `mdl.GenerateContent` 的普通 `NodeFunc`。把它降级成函数、并入 `scene_dispatch`，既去掉了主图上唯一一条无中断保护的环，也让 `StateKeyIntent` 可以整体删除。

## What Changes

- **合并 `intent_recognition` 到 `scene_dispatch`**：删除主图 `intent_recognition` 节点及其两条边，`intent` 包的节点构造器降级为纯函数 `intent.Classify(...)`，由 `scene_dispatch` 直接调用。主图上只剩 `fallback → scene_dispatch` 这一条环，且每圈都被 `fallback` 的 interrupt 挡住。**BREAKING（内部）**：`MainGraphAgentNodeIntentRecognition` 枚举值与 `MakeIntentRecognitionNode` 构造器被删除，AG-UI trace 中不再出现 `intent_recognition` 节点。
- **删除 `StateKeyIntent`**：全仓库仅 `BuildFallbackResumeDelta` 与 `unsupportedIntentFallbackMessage` 两处消费，均在 fallback 路径上，且均可逐案等价地改读 `StateKeySessionTag`。`agentstate.ParseIntent`、`intent.intentState` 一并删除。此后主图里「在哪个场景」只有一个真相来源。
- **轮次边界重跑意图识别**：`scene_dispatch` 每次执行都调一次 `intent.Classify`，取代现在「有 tag 就直达子图」的短路。
- **识别结果与 tag 不一致时切换场景**：识别出的意图是受支持场景且不等于当前 `session_tag` 时，提交新 tag 并路由到新场景子图，整个过程在同一个 run 内完成，用户只发一条消息、只看到一个回答。
- **识别结果不受支持时留在原场景**：已有 tag 的会话本轮识别为 `chat`（如「谢谢」「继续」）时保持原 tag、进入原子图，由场景 LLM 回答；不清历史、不清 tag、不出拒识文案。分类失败时 `Classify` 返回 `chat`，因此**失败方向即不切换**。
- **`scene_dispatch` 判定与路由职责分离**：节点做全部判定并把决策写入 `StateKeySceneDispatchNext`，条件边路由函数退化为纯查表，消除节点与路由函数两处重复判定的隐患。
- **DB 标签回写支持变更**：`reconcileSessionTag` 去掉 `originalTag != ""` 短路与配套的「目前会话标签不允许修改」TODO，改为差异回写。触发时机（改到 SSE 结束后，否则读到的是切换前的 checkpoint、回写滞后一轮）已随 `02786079a` 单独上线，本变更不再涉及。
- **新增 `scene.switched` 自定义事件**：切换成功时 `scene_dispatch` 通过 `graph.NewNodeCustomEvent` 发出，框架 AG-UI translator 原生转成 CUSTOM 事件（无需改 `agui-event/translator.go`），前端据此切换会话标签 UI。
- **不做**：`handoff` 声明工具、子图内 `scene_switch` 节点、HITL 中断点上的切换、纯确认词短路省调用、前端显式场景选择器、功能配置开关（理由见 design.md「被删除的决策」）。
- **对外无破坏性**：无接口协议变更，无 DB schema 变更。无标签会话的首轮行为与已有会话的场景内多轮行为均保持不变。

## Capabilities

### New Capabilities

- `agent-scene-switch`: 轮次边界的场景切换能力——何时重跑意图识别、识别结果与当前 tag 的五种组合各自如何决策、`scene.switched` 事件契约、切换后的状态复用行为。

### Modified Capabilities

- `agent-scene-dispatch`: 「有受支持 session_tag 时直达 ReAct 子流程（跳过意图识别）」被取代为「每个轮次边界都先分类再决定去向」；「意图识别结果回到 scene_dispatch 后由其判定支持性」中的节点往返改为节点内直接调用；新增「节点做判定、路由函数查表」与「每轮刷新 session_rid」两条要求。
- `intent-recognition-node`: 意图识别从 `graph.NodeFunc` 降级为纯函数 `intent.Classify`；`StateKeyIntent` 常量被删除；「Graph 入口为 intent_recognition」这条早已被 `agent-scene-dispatch` 取代的要求一并移除。
- `agent-intent-routing`: 涉及 `scene_dispatch → intent_recognition` 节点往返的路由要求改为节点内分类；`fallback` 的历史重建判据从 `StateKeyIntent` 改为 `StateKeySessionTag`。
- `agent-session-tag`: 「运行时回写 session_tag」从「仅无标签会话首次识别时回写」扩展为「tag 发生变更时回写」，并明确回写发生在 Run 结束之后。

## Impact

**受影响服务层**：仅 agent-server（Access/Service 层）。data-service 侧只复用既有的 `UpdateAiagentSessionReq`，无新增接口、无 DAO 改动、无 DB schema 变更。

**代码**：

| 文件 | 改动 |
|---|---|
| `cmd/agent-server/logics/agent/intent/node.go` | `MakeIntentRecognitionNode` → `Classify` 纯函数；删除 `intentState` |
| `cmd/agent-server/logics/agent/graph_build.go` | `scene_dispatch` 节点内联分类 + 决策矩阵 + 发事件；路由函数改查表；删除 `intent_recognition` 节点与两条边；`unsupportedIntentFallbackMessage` 改读 tag |
| `cmd/agent-server/logics/agent/message/message.go` | `BuildFallbackResumeDelta` 的历史重建判据改读 tag |
| `cmd/agent-server/logics/agent/state/intent.go` | 删除（`ParseIntent` 无消费方） |
| `cmd/agent-server/service/service.go` | `reconcileSessionTag` 改差异回写（触发时机已单独上线，不在本变更范围） |
| `pkg/criteria/constant/aiagent.go` | 新增 `StateKeySceneDispatchNext`、`SceneSwitchedCustomEventName`；删除 `StateKeyIntent` |
| `pkg/criteria/enumor/aiagent.go` | 删除 `MainGraphAgentNodeIntentRecognition` 及其 `Validate` 分支 |
| 相关单测 | `intent/node_test.go` 平移到 `Classify`；`graph_build_test.go` 新增决策矩阵/路由查表/主图入边断言，并改写其中 7 个以 `StateKeyIntent` 构造输入的既有测试 |

**前端**：需要消费 `scene.switched` CUSTOM 事件（payload 含 `from`/`to`）以同步会话标签 UI。场景中文名前端已有 `SESSION_TAG_NAME` 映射，后端只发枚举原值、不下发文案；事件的 wire shape 待与前端敲定（见 design.md Open Questions）。属另开的前端任务，本变更只保证事件契约。

**成本与性能**：每个轮次边界固定增加一次意图识别 LLM 调用（`max_tokens=16`，非流式，量级 300~800ms 首字延迟）。这是初版方案的固有代价——完整方案用 `handoff` 把这个成本降为 0，但依赖 LLM 遵循度且必须配套子图改造。上线前在测试环境实测 P95，超预期则先做纯确认词短路。

**残余风险**：跨场景切换后父图 messages 里留有旧场景的工具调用记录，新场景 LLM 可能模仿去调用不存在的工具（方案文档 §10.2）。已有 `MakeHistoricalToolResultFilter` 作为缓解，属可观测的残余风险。
