## Context

`Agent场景切换方案.md` 完整方案的目标是「任意时刻可切换」，其中最难的部分是「用户停在 HITL 卡片上时也能切」——因为所有 HITL 中断点都在场景子图内部，控制权根本不会回到主图。方案为此设计了 `handoff` 声明工具 + 子图 `scene_switch` 节点 + `RawStateDelta` 白名单透传 + `fallback` 拦截 + 单 run 切换护栏这一整套机制，改动清单 14 项，其中 3 项标为高风险。

本次初版把范围收窄到「一个子图完整跑完之后才切换」。这个收窄不是简单地砍掉几个用例，而是让方案的架构复杂度整体坍缩，原因只有一条：

> **主图** `scene_dispatch` **只会在轮次边界执行。**

主图的边只有四组：`START → scene_dispatch`、`intent_recognition ⇄ scene_dispatch`、`子图 → fallback`、`fallback → scene_dispatch`。当上一轮停在子图内部的 HITL 中断上时，框架 resume 到的是子图的中断节点（方案文档 §2.5 已验证），`scene_dispatch` 不会执行；只有当子图跑完、控制权经 `fallback` 中断等到用户下一条消息时，`fallback` 的 resume 分支才会经静态边把控制权交回 `scene_dispatch`。

也就是说，「子图跑完了吗」这个判断不需要任何探测状态——`scene_dispatch` 正在执行这件事本身就是答案。初版因此不需要 `handoff` 工具、不需要子图新节点、不需要碰 `makeSubgraphOutputMapper`（方案文档标为「漏则静默失效」的高风险项）、不需要碰 `makeFallbackNode`。

**当前实现的关键约束**（决定了下面几个决策）：

1. `intent_recognition` 并不是框架意义上的 LLM 节点。它是一个普通 `graph.NodeFunc`，内部直接调 `mdl.GenerateContent`，把结果写进 `StateKeyIntent` 后靠静态边回到 `scene_dispatch`。这个节点边界几乎没有换来什么，却制造了主图上唯一一条无中断保护的环。
2. `makeSceneDispatchNode` 在 tag 受支持时会把 `StateKeyIntent` 强制刷成 tag。也就是说，在有 tag 的会话里 `StateKeyIntent` 已经完全等价于 `StateKeySessionTag`，不再承载「本轮意图」语义。
3. checkpoint 优先于 runtimeState（框架 `mergeInitialStateNonInternal` 只补缺失的 key），所以 service 层通过 forwardedProps 注入的 `session_tag` 从第二轮起就完全不生效，切换信号不可能走这条路。
4. `reconcileSessionTag` 的触发时机已于 2026-07-23（`02786079a`）修好——从 `asyncIncrContentCount` 尾部拆出，改由 `sessionCodeMiddleware` 在 `next.ServeHTTP` 返回后单独起 goroutine。但函数体内仍有 `originalTag != ""` 短路，其上方还留着一条「目前会话标签不允许修改……未来需要支持修改标签时，需要修改这里」的 TODO：标签一旦写入就不再变更。



## Goals / Non-Goals

**Goals:**

- 用户在一个子图跑完之后的下一条消息里，可以自然地切换到另一个场景，无需新开会话、无需点确认卡片、无额外对话轮次。
- 切换在同一个 run 内完成：用户发一条消息，直接得到新场景的回答。
- 前端能感知切换并同步会话标签 UI。
- 不引入完整方案里那三个高风险改动点（output mapper 白名单、fallback 拦截、子图路由新分支）。
- 借这次改动收敛 `StateKeyIntent` 与 `StateKeySessionTag` 的职责重叠，让主图的场景真相只有一个来源。

**Non-Goals:**

- HITL 中断点上的切换（停在选账号卡片/提单门禁卡片时改主意）。留给二期的 `handoff` 机制。
- 消除每轮的意图识别 LLM 调用开销。初版接受这个成本。
- 切换时的场景私有状态清理与「分桶挂起」。子图隔离 + per-turn namespace 已天然完成隔离（方案文档 §7），不做额外处理。
- 前端显式场景选择器。



## Decisions



### D1. 用「轮次边界重跑意图识别」而非 `handoff` 工具


|              | 轮次边界重跑意图识别（本方案）             | `handoff` 声明工具（完整方案） |
| ------------ | --------------------------- | -------------------- |
| 覆盖范围         | 仅轮次边界                       | 含 HITL 中断点           |
| 不切换轮次的成本     | +1 次意图识别调用（`max_tokens=16`） | 0                    |
| 确定性          | 高，不依赖 LLM 遵循度               | 依赖 prompt 遵循度，有漏切风险  |
| 改动面          | 1 个节点 + 回写 + 1 个事件          | 14 项，含 3 项高风险        |
| 需要改场景 prompt | 否                           | 是，且是漏切的唯一防线          |


初版选前者：改动面小、确定性高、不需要动场景 prompt，用可控的成本换掉了对 LLM 遵循度的依赖。二期若要覆盖中断点，`handoff` 机制可以叠加在本方案之上而不冲突——`handoff` 负责把控制权从子图交回主图，交回之后走的仍然是本方案在 `scene_dispatch` 里建立的这套决策逻辑。

**否决的备选**：在 `fallback` 的 resume 分支里做切换判定。`fallback` 的职责是「投递回复 + 中断等待」，塞进路由判定会让两个路由中心并存；而且它拿不到分类结果，得自己再拉一次 LLM。

### D2. 轮次边界不做显式探测，靠拓扑保证

不引入「上一轮是否在子图内中断」之类的状态位。`scene_dispatch` 执行 ⟺ 轮次边界，这是主图拓扑的直接推论。

**代价**：这是一条隐式契约。如果将来主图拓扑变化（例如给子图加一条绕过 `fallback` 直回 `scene_dispatch` 的边，或把某个 HITL 中断点提到主图上），这个等价关系就会破裂。用 `graph_build.go` 顶部的拓扑注释 + 一条针对性的单测（构图后断言 `scene_dispatch` 的入边集合）来锁住它。

### D3. 合并 `intent_recognition` 到 `scene_dispatch`，主图去掉这条环

`intent_recognition` 独立成节点，换来的只有一件事：AG-UI trace 里多一个节点边界。付出的代价却不小——

- 它是主图上**唯一**一条没有中断保护的环（`scene_dispatch ⇄ intent_recognition`），必须额外设计护栏才敢让 `scene_dispatch` 重复执行；
- 为了跨这两个节点传递分类结果，`StateKeyIntent` 被迫成为持久化的 graph state key，而它一旦进入 state 就会被 checkpoint 持久化、被 `fallback` 消费、被 `scene_dispatch` 覆写，语义随之发散（约束 2）；
- `scene_dispatch` 因此必须处理「我是第一趟还是第二趟」，判定分支凭空翻倍。

而它本来就不是 LLM 节点（约束 1），只是一个内部调 `mdl.GenerateContent` 的普通函数。把它降级成函数、由 `scene_dispatch` 直接调用之后：

```
合并前： START → scene_dispatch ⇄ intent_recognition
                      ↓
              {host_apply, resource_query, fallback} → fallback → scene_dispatch

合并后： START → scene_dispatch → {host_apply, resource_query, fallback}
                      ↑                                    ↓
                      └──────────── fallback ←─────────────┘
```

主图只剩 `fallback → scene_dispatch` 这一条环，而它每一圈都被 `fallback` 的 interrupt 挡住（必须有用户消息才能继续），天然不会失控。`scene_dispatch` 每个 run 只执行一次，「第一趟/第二趟」的问题连同它的护栏一起消失。

具体做法：`intent` 包保留分类逻辑，把节点构造器 `MakeIntentRecognitionNode` 换成纯函数 `intent.Classify(ctx, mdl, promptStore, messages, contextWindowSize) enumor.IntentType`；`LLMParseIntent` 与全部降级逻辑原样保留，现有单测平移到 `Classify` 上。`scene_dispatch` 的构造器相应接收 `mdl` / `promptStore` / `contextWindowSize`。

**失去的**：trace 里少一个节点边界，意图识别的耗时不再由框架的 node start/complete 事件自动打点。用显式的耗时日志/指标补上（本来就要做，见任务 6.1）。

**保留的降级语义**：`Classify` 在 LLM 调用失败或返回值无法识别时返回 `chat`。在有 tag 的会话里，`chat` 落到「不受支持 → 留在原场景」分支——也就是说**分类失败自动退化为不切换**，这正是我们想要的失败方向。

**否决的备选**：保留两个节点，改用轮次计数器做护栏（见文末「被删除的决策」）。

### D4. 删除 `StateKeyIntent`，`session_tag` 成为唯一的场景真相

`StateKeyIntent` 与 `StateKeySessionTag` 的重叠不是错觉：现网 `scene_dispatch` 在 tag 受支持时会把前者刷成后者（约束 2），两者在有 tag 的会话里逐字相等。它之所以还存在，只是因为分类结果需要跨 `intent_recognition` 和 `scene_dispatch` 两个节点传递（D3 已消除这个需求）。

合并之后，分类结果是 `scene_dispatch` 内的一个局部变量，用完即弃。剩下的问题是 state 里的这个 key 还有谁在读——全仓库只有两处，都在 fallback 路径上：


| 消费方                                | 现在的判断                                                                      | 替换为                                                 |
| ---------------------------------- | -------------------------------------------------------------------------- | --------------------------------------------------- |
| `message.BuildFallbackResumeDelta` | `!ParseIntent(state).IsSupportedScene()` → 清 intent + 重建 assistant/user 历史 | `!parseSessionTag(state).IsSupportedScene()` → 重建历史 |
| `unsupportedIntentFallbackMessage` | 按 intent 是否受支持选兜底文案                                                        | 按 session_tag 是否受支持选兜底文案                            |


两处替换是**逐案等价**的，因为「本轮路由进了某个场景子图」与「`session_tag` 是受支持场景」互为充要：进子图之前 `scene_dispatch` 必然已提交 tag，而 tag 为空时只可能走到 `fallback`。逐一核对：

- 无标签会话本轮识别为 `chat` → 走 fallback：旧 `intent=chat` 不受支持 → 重建；新 `tag=""` 不受支持 → 重建。一致。
- 有标签会话子图跑完 → fallback：旧 `intent=host_apply`、新 `tag=host_apply`，都受支持 → 不重建。一致。
- `account_select` 无可用账号直接 End → fallback：`tag=host_apply` 已在进子图前提交，两者都受支持 → 不重建。一致。

所以 `StateKeyIntent`、`agentstate.ParseIntent`、`intent.intentState` 可以整体删除。删掉之后，「这个会话/这一轮在哪个场景」在主图里只有 `StateKeySessionTag` 一个答案，不再需要在两个 key 之间维护同步不变式（方案文档 §9.1 那一整节的坑随之消失）。

### D5. `scene_dispatch` 节点做判定、路由函数只查表

现在节点和路由函数各自独立地读 tag/intent 判一遍，两处逻辑必须手工保持一致。判定分支从 2 个涨到 5 个（决策矩阵）之后，重复实现必然发散。

改为：节点算出决策，写 `StateKeySceneDispatchNext`（目标节点名）；路由函数读这个键、映射、返回。这个模式在本仓库已有先例——子图里 `account_select` 节点写 `AccountSelectNextNodeKey`，`makeAccountSelectRoutingFunc` 查表。

依赖前提：条件边路由函数看到的是节点 delta 应用之后的 state。这一点由现网行为验证——首轮「无 tag、intent=host_apply」时，节点提交 `session_tag` 后路由函数正是靠读到这个新提交的 tag 才路由到子图的。

路由函数读到空值或非法节点名时路由 `fallback` 并记 Warn，不返回 error 中断 run。

### D6. 顺带修复 `session_rid` 跨轮不刷新

`SessionRidStateKey` 由 `makeRunOptionResolver` 注入 RuntimeState，供只能拿到 `graph.State` 的子图 mapper 打 trace 日志。但框架只补 checkpoint 中缺失的 key，第二轮起注入被旧值压制，子图日志的 rid 从第二轮开始就一直是第一轮的。

`scene_dispatch` 每轮都执行且手里有 ctx rid，顺手刷新即可。一行代码，独立于本需求，可先合入。

### D7. 用框架原生的 node custom event 发 `scene.switched`

现有的 `agui-event/translator.go` 只把**中断**元数据转成 CUSTOM 事件，而切换不产生中断。但框架已经有现成通道：`graph.NewNodeCustomEvent(WithNodeCustomEventEventType("scene.switched"), WithNodeCustomEventPayload(...))` 发出的事件，AG-UI translator 的 `graphNodeCustomEvents → handleCustomEvent` 会无条件地转成名为 `scene.switched` 的 CUSTOM 事件，value 为 `{nodeId, payload, timestamp}`。

发送方式沿用 `message.EmitFallbackMessage` 的既有模式（`graph.GetEventEmitterWithContext(ctx, state)` 拿 emitter 后 `Emit`）。走这条通道 `translator.go` 可以一行不改，但代价是事件 value 的形状与 HCM 现有的 5 个 CUSTOM 事件不一致，是否为此做一次归一化见 Open Questions。

**否决的备选**：`graph.EmitCustomStateDelta`。API 更简洁，但它会把业务 delta 塞进事件的 `StateDelta` 里，存在被下游 session state 同步逻辑消费的风险；`NewNodeCustomEvent` 只携带 `_node_custom_metadata`（框架内部键，被 `isInternalStateKey` 过滤），没有这个副作用。

事件只在 `from != "" && from != to` 时发——无标签会话首次提交标签不算切换，前端不需要提示。emit 失败只记 Warn。

### D8. 标签回写改差异回写

触发时机已随 `02786079a`（2026-07-23）单独上线，本变更只剩函数体内的差异回写：去掉 `originalTag != ""` 短路与配套的 TODO，改为「checkpoint 里的 tag != 请求进入时的 tag 就回写」。

时机为什么必须在 Run 之后，记录在此以免后人改回去。`reconcileSessionTag` 并不知道本轮识别出了什么 tag，它是读最新 checkpoint 反查的。早先它挂在 `go s.asyncIncrContentCount(...)` 尾部、在 `next.ServeHTTP` 之前就拉起，与 SSE 并发，读到的是本轮 Run 开始前的状态。这个缺陷在现网被掩盖了：无标签会话第一轮读不到 checkpoint（还没有）直接返回，第二轮才读到第一轮写的 tag 并回写——标签落库天然滞后一轮，而因为标签只写一次，滞后不可见。切换场景下滞后就会显形：切换发生在第 N 轮，回写却要等第 N+1 轮。

**正常完成路径没有竞态，不需要重试。** 三段顺序把它锁死了：

1. `createCheckpointAndSave` 是同步的——内部直接 `cm.PutFull` 走 saver 事务，不是 fire-and-forget；
2. 它在执行 goroutine 的函数体内被调用，而 `close(eventChan)` 在同一个 goroutine 的 `defer` 里，必然排在其后；
3. SSE 侧 `handleEvents` 要读到 channel 关闭（`ok == false`）才返回。

所以「`ServeHTTP` 返回」蕴含「本轮 checkpoint 已落盘」。

**残余风险（仅断连场景）**：`handleEvents` 另有两个提前返回的分支——`ctx.Done()`（客户端断连）与 `WriteEvent` 失败，两者都会起 `go drainEvents(events)` 在后台吞掉剩余事件，而**图仍在跑**。此时 `ServeHTTP` 已返回，对账会读到跑到半路的 checkpoint。这不是毫秒级竞态，短重试不对症（run 可能还要跑很久）。危害有限：该轮回写落空，下一轮的差异回写读最新 checkpoint 与 `originalTag` 比对时会补上，退化成滞后一轮。**结论：不加预防性重试。**

### 被删除的决策

评审中砍掉的两项，记录原因以免后续重新提起：

**「用 run 维度标记（rid）做死循环护栏」**——原设计中 `scene_dispatch` 一个 run 内要跑两趟（分类前、分类后），需要一个标记来区分。用 `StateKeyIntent` 是否为空判断不可靠（它会被主动清空又回写），所以打算把当前 rid 写进 state、第二趟比对。这个方案是在给一个本不该存在的环打补丁：判据是从 ctx 取的带外值，语义靠一个 string 比较隐式表达，且依赖 rid 一定非空。D3 合并节点后环消失，`scene_dispatch` 每 run 只执行一次，护栏整个不需要了。

若将来出于 trace 粒度的考虑要把 `intent_recognition` 拆回独立节点，护栏应改用**轮次计数器**而非 rid：由 `fallback` 的 resume delta 递增一个主图 turn（resume 每轮恰好发生一次），`scene_dispatch` 比对「上次分类时的 turn」与「当前 turn」。这与子图 `subgraphTurnKey` 的做法同构，单调、自包含在 graph state 内、不依赖 ctx 透传，比 rid 稳。

**「**`sceneSwitchEnable` **配置开关」**——原意是给「每轮多一次 LLM 调用」留一个即时回退口。砍掉的理由：它会在决策矩阵上凭空多出一整个分支，把刚刚删掉的旧路由语义（有 tag 即直达）又长期供养起来，测试面翻倍；而且若默认关闭，功能等于灰度前完全没在生产跑过。延迟风险改用别的办法控制：意图识别是 `max_tokens=16` 的非流式调用，先在测试环境实测 P95 再决定是否上线；真出问题时，本变更足够小，回滚一个版本比翻一个开关只慢一次发布。

## Risks / Trade-offs

**[每个轮次边界固定多一次 LLM 调用]** → 这是初版的固有代价。意图识别用 `max_tokens=16` 且不流式，量级 300~800ms 首字延迟。上线前在测试环境实测 P95；若不可接受，优先做「纯确认词（好的/嗯/继续）短路」——该优化不改架构，只在 `scene_dispatch` 调 `Classify` 之前加一层字符串判定。

**[意图识别误判导致误切]** → 用户在 `host_apply` 里说「这批机器配置多少」，可能被判成 `resource_query` 而切走。缓解：切换不销毁工作（子图状态留在旧 namespace，`selected_account_id` 跨 run 保留），误切代价只是用户多说一句「回到刚才的申领」，切回时账号不用重选、只需重出方案。这也是不弹确认卡片的依据。`intent_recognition_prompt.md` 已有「上下文延续」与「意图切换」两节专门处理这类边界，且分类输入包含最近 5 条 user/assistant 消息（尾部就是上一轮场景的回复），上下文是够的。观测上要给切换次数打点，按误切反馈决定是否收紧 prompt。

**[分类失败时的行为]** → `Classify` 在 LLM 失败/返回值无法识别时返回 `chat`；有 tag 的会话据此留在原场景，无 tag 的会话走兜底文案。失败方向是「不切换」，安全。

**[跨场景工具名不存在]** → 切换后父图 messages 里留着旧场景的 `tool_proxy_execute_tool` 调用记录，新场景 LLM 可能模仿去调用不存在的工具。已有 `MakeHistoricalToolResultFilter` 把最后一条 user 消息之前的 tool 结果替换为占位符，降低模仿诱因。属可观测的残余风险，上线后看工具执行失败日志。

**[「scene_dispatch 执行 ⟺ 轮次边界」是隐式契约]** → 见 D2。合并后 `scene_dispatch` 的入边只剩 `START` 与 `fallback` 两条，断言更简单也更稳。用拓扑注释 + 入边断言单测锁住。

**[删除** `StateKeyIntent` **波及 fallback 路径]** → 替换虽经逐案核对等价，但 `BuildFallbackResumeDelta` 的历史重建分支一旦误触发会重复追加 assistant 消息，属于用户可见的错误。三个案例（无标签 chat、有标签子图跑完、无可用账号）各写一条单测锁住。

**[删除 `StateKeyIntent` 的测试面比看上去大]** → `graph_build_test.go` 现有 7 个测试函数直接以 `graph.State{constant.StateKeyIntent: ...}` 构造输入（`TestMakeSceneDispatchNode`、`TestMakeSceneDispatchRoutingFunc`、`TestSceneDispatchNodeThenRouting`、`TestUnsupportedIntentFallbackMessage`、`TestBuildFallbackResumeDeltaClearsUnsupportedIntentHistory`、`TestBuildFallbackResumeDeltaClearsUserInput`、`TestNoUsableAccountFallbackMessageNotDuplicated`），全部要改写。更麻烦的是合并节点后 `makeSceneDispatchNode` 要收 `mdl`/`promptStore`，这批原本零依赖的纯函数测试会被迫引入 mock model，而现有的 `mockModel` 是 `intent` 包内的未导出类型，`agent` 包用不了。缓解办法正是 D5 的纯决策函数：决策矩阵全部用零依赖的表驱动测试覆盖，只留一两条带 mock 的用例验证「节点确实调了分类并把结果写进 delta」。

**[被遗弃的子图 checkpoint]** → 切走时旧场景那个 turn 的 checkpoint 不会再被 resume（切回来是新 turn 新 namespace），成为死数据。功能无害，占存储。若后续存储压力明显再考虑清理。

**[主图节点删除影响既有 spec]** → `intent-recognition-node` 与 `agent-intent-routing` 两份 spec 直接描述了 `intent_recognition` 节点及其路由，需要同步出 delta。这两份 spec 本身已有先前重构遗留的陈旧内容（仍在描述 `scene_dispatch → llm` 直连、`resource_query` 不受支持），本次只更新与本变更直接相关的要求，其余陈旧项单独记一条清理任务。

## Migration Plan

1. 先合入 D6（`session_rid` 刷新）与 D8（差异回写）两项独立修复，它们不依赖切换功能，可单独验证。D8 的时机部分已随 `02786079a` 上线。
2. 合入 D3/D4 的结构重构（合并节点、删除 `StateKeyIntent`），此时决策矩阵仍按现网语义：有 tag 就用 tag。这一步应当**行为零变化**，用现有单测 + 手工回归确认。
3. 合入决策矩阵中的切换分支与 `scene.switched` 事件。
4. 测试环境实测意图识别 P95 与误切率，再上生产。
5. 前端接入 `scene.switched` 后再面向用户宣讲该能力。

**回滚**：无配置开关，回滚即回退版本。已被切换过的会话其 `session_tag` 停在切换后的值，不会回滚，但那是一个合法的标签，旧版本读到后照常直达对应子图，不影响可用性。

## Open Questions

- ~~`scene.switched` 是否需要后端下发可直接展示的提示文案~~ **已确认不需要**：前端 `front/src/views/chatbot/constants.ts` 已有 `SESSION_TAG_NAME` 映射（`host_apply` → 主机申领、`resource_query` → 资源查询），`resolveSessionTagName` 未命中时回退原值。后端只发枚举原值即可，措辞留给前端。
- `scene.switched` 的 wire shape 待与前端敲定。框架原生通道产出的 CUSTOM 事件 value 是**对象**且包一层：`{nodeId, payload: {from, to}, timestamp}`，前端需读 `value.payload.from` 且不能 `JSON.parse`。而前端 `hooks/chatbot/use-event.ts` 里现有的 5 个 CUSTOM 事件（`hitl.interrupt`、`account_select.interrupt`、`host_apply.recommend/preorder/submit`）清一色是 `JSON.parse(event.value as string)`——因为 HCM 自己的 `customTranslator` 塞的是 JSON 字符串。两条路：(a) 接受框架形状，前端为该事件单开一个分支；(b) 在 `customTranslator` 里按事件名把 value 归一化成 JSON 字符串，前端写法与其余 5 个完全一致。(b) 多约 15 行后端代码（`aguievents.CustomEvent` 只有 `Name`/`Value` 两个字段，按 name 匹配改写很直接），换契约一致，后续再加场景级事件也不用重复决策。
- 「纯确认词短路」是否要在初版就带上？取决于测试环境实测的 P95，建议先测再定。
- 合并后意图识别的耗时打点用什么承载——普通日志、`NewNodeProgressEvent`，还是接现有 metrics？取决于目前 agent-server 的指标体系口径。

