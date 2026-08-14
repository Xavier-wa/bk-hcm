## Context

GraphAgent（`cmd/agent-server`）当前图拓扑：入口固定为 `intent_recognition`，分类后由条件边直接路由到 `host_apply` 子流程（`llm`）或 `fallback`。Graph state 按 `threadID` 持久化在 **checkpoint**（sqlite/inmemory/redis），`StateKeyIntent` 借此跨轮保留——这是当前 `host_apply` 多轮跳过意图识别的实现基础。

`/agui` 请求经 `sessionCodeMiddleware` 把 `sessionCode → threadID` 解析后下发到 AG-UI runner；解析由 `Resolver` 完成，内置 LRU 缓存（容量 10000，TTL 30min）。`makeRunOptionResolver` 负责构造每次 Run 的 `runtimeState`（initial state），并处理 auto-resume。

本次新增「会话场景标签」(`session_tag`) 与「场景分发节点」(`scene_dispatch`)。`scene_dispatch` 作为**单一路由决策中心**：意图识别节点不再直接进入 `llm`/`fallback`，而是分类后回到 `scene_dispatch`，由其统一判断场景是否受支持并决定下一跳。

## Goals / Non-Goals

**Goals:**

- 会话创建时可绑定 `session_tag`，持久化到 `aiagent_session.extension.session_tag`，`list_session` 返回。
- Graph 入口改为 `scene_dispatch`，由它集中决策：有受支持标签直达 `llm`，无标签先经 `intent_recognition` 再回到 `scene_dispatch` 决策。
- 意图识别仅分类并回到 `scene_dispatch`；由 `scene_dispatch` 判断受支持则回写 `session_tag` 并进 `llm`，不支持则给出提示并进 `fallback`。
- `fallback` 路由：有标签直达 `llm`，无标签回 `scene_dispatch`。
- 稳态下不新增 data-service 读：多轮靠 checkpoint，首轮复用 Resolver 已有查询。

**Non-Goals:**

- 不实现 `resource_query` / `chat` 的 ReAct 子流程（仍走未支持提示）。
- 不支持会话中途切换 / 清除 `session_tag`（仅创建写入 + 意图识别一次性回写）。
- 不改动意图识别节点的分类算法、prompt、模型选择逻辑（仅改其出边路由）。
- 不改 AG-UI/checkpoint 协议。

## Decisions

### D1: session_tag 存储于 aiagent_session 独立列，复用 IntentType 校验

在 `aiagent_session` 表新增独立列 `session_tag VARCHAR(64) DEFAULT ''`，取值与 `enumor.IntentType` 对齐（当前合法直达值仅 `host_apply`）。

- **为何**：查询/索引直接，协议层与 DAO 透传无需 JSON 拆解；列可空，旧数据默认空标签向后兼容。
- **替代方案**：复用既有 `extension` JSON 字段——免 DDL，但查询需 JSON 解析、协议透传需二次封装，且 list 返回需额外映射，可维护性更差。
- **校验**：非法标签在 create 链路返回 `InvalidParameter`；空标签合法（= 无标签会话，走现网行为）。

### D2: 状态模型 —— session_tag 与本轮 intent 分离

引入两个 state key，职责分离以支撑集中式路由：

- `StateKeySessionTag`：**会话级持久标签**。Run 启动时若会话已绑定则注入；意图识别命中受支持场景后由 `scene_dispatch` 写入。跨轮稳定。
- `StateKeyIntent`：**本轮意图识别结果**，仅由 `intent_recognition` 写入，作为 `scene_dispatch` 的临时决策输入。

`scene_dispatch` 优先看 `StateKeySessionTag`，其次看本轮 `StateKeyIntent`，避免「已提交场景」与「本轮识别结果」混淆。

### D3: 运行时获取 session_tag —— checkpoint 扛多轮，Resolver LRU 扛首轮

- **后续轮次**：`StateKeySessionTag` 已在 checkpoint state 中，按 `threadID` 自动恢复，**不读 DB**。
- **会话首轮（无 checkpoint）**：需把 `session_tag` 注入 initial state。复用 `Resolver` 已发生的那次 `Session.List`：将其 LRU 缓存值由 `string`（仅 threadID）改为结构体 `sessionMeta{ ThreadID, SessionTag string }`，让 `session_tag` 搭同一次查询的便车。
  - 命中缓存：0 次 DB 读；未命中：1 次 DB 读（与现状一致，无新增）。
- **注入点**：`sessionCodeMiddleware` 经 `ResolveMeta` 拿到 `session_tag` 后，通过请求体 `forwardedProps` 透传；`makeRunOptionResolver` 读取后写入 `runtimeState[StateKeySessionTag]`（非空才注入；值稳定，resume 重复注入无副作用）。
- **替代方案**：`scene_dispatch` 节点内部直接查 data-service——节点需注入 kt/client，侵入性强且每次冷启动都查，劣于复用缓存。

### D4: 图拓扑改造（scene_dispatch 为单一路由决策中心）

```
START → scene_dispatch
scene_dispatch（条件路由，纯内存，无 LLM）:
  ① StateKeySessionTag 受支持（host_apply）        → llm
  ② 否则本轮已有 StateKeyIntent（intent 节点刚写）:
       - 受支持（host_apply）→ 回写 session_tag、置 StateKeySessionTag → llm
       - 不支持 / 未识别     → 设置未支持提示文案            → fallback
  ③ 否则（无 tag 且本轮无 intent）                  → intent_recognition

intent_recognition → scene_dispatch        （仅分类写 StateKeyIntent，不再直连 llm/fallback）
llm → ConditionalEdge(tool_calls) → hitl / tool / fallback     （保持不变）
hitl → llm ; tool → llm                                         （保持不变）
fallback（中断等待下一条消息，resume 后）:
  - 受支持 session_tag → llm
  - 无 tag            → 清空本轮 StateKeyIntent → scene_dispatch
```

- 受支持标签判定以单一函数收口（当前仅 `host_apply`），预留未来扩展点。
- **决策集中在 `scene_dispatch`**：意图识别从「分类 + 路由」收敛为「只分类」，路由职责单一化，便于扩展新场景。

### D5: 回写 session_tag 由 middleware 在 Run 结束后对账完成（保持 graph 节点纯净）

graph 节点在启动时构建（`runtime.go`），无法持有 per-request 的 `kit`（含 user/appcode/鉴权），因此**不在节点内直接写 DB**。`scene_dispatch` 节点只负责把命中的标签写入 graph state（`StateKeySessionTag`），DB 回写改由 `sessionCodeMiddleware` 在 `/agui` 的 Run 结束后（已有 IncrContentCount 异步钩子处）对账完成：

- 节点侧：`scene_dispatch` 命中受支持场景且原无标签时，向 graph state 写入 `StateKeySessionTag=host_apply`，并持久化进 checkpoint。当轮及后续轮路由均不依赖 DB。
- middleware 侧（已持有 `kt`/`sessionCode`/`clientSet`/`resolver`/`CheckpointSaver`）：Run 结束后读取该 thread 最新 checkpoint 的 `ChannelValues[StateKeySessionTag]`；若非空且该会话原为无标签（由 `ResolveMeta` 缓存值得知），则经 data-service 回写 `session_tag` 列并刷新 `Resolver` 缓存。
- **为何放 middleware**：此处天然具备完整 `kt` 与会话标识，且复用既有 Run 后异步钩子；避免向启动期构建的 graph 节点注入 client/kit，降低耦合与回归风险。
- 回写失败仅记 Warn，不阻断对话；当轮仍由 checkpoint 的 `StateKeySessionTag` 保证路由正确。

### D6: 未支持场景的提示与回流闭环

- `scene_dispatch` 命中分支②的不支持分支时，预置未支持提示文案（沿用 `unsupportedIntentFallbackMessage` 逻辑），路由到 `fallback` 输出并中断。
- `fallback` resume 后，无标签会话**清空本轮 `StateKeyIntent`** 并路由回 `scene_dispatch`：下一次进入 `scene_dispatch` 因无 tag、无本轮 intent 而落到分支③重新 `intent_recognition`，形成「未识别 → 提示 → 回分发 → 重新识别」闭环，且不会出现 `scene_dispatch ↔ intent_recognition` 死循环（intent 写入后即落到分支②）。

## Risks / Trade-offs

- **[stale intent 导致跳过重新识别]** → `fallback` 对无标签会话 resume 时清空 `StateKeyIntent`，确保下一轮重新意图识别。
- **[initial-state 注入覆盖运行态]** → 仅在无 interrupted checkpoint 的新 Run 注入 `StateKeySessionTag`；resume 路径不注入。
- **[回写失败导致标签不持久]** → 仅记 Warn 不阻断；当轮由 checkpoint 保证路由，下次冷启动退化为重新意图识别（可接受）。
- **[Resolver 缓存结构变更影响面]** → `Resolve` 对外仍返回 threadID，仅内部缓存值结构调整，新增 `ResolveMeta`（或等价）暴露 session_tag，降低回归风险。
- **[scene_dispatch 多次进入引入额外跳数]** → 纯内存路由节点，无 LLM/IO，延迟可忽略；相较跳过意图识别 LLM，净收益为正。

## Migration Plan

1. 后端先行：新增 `session_tag` 列 DDL + 枚举校验 + DAO/data-service/agent-server 协议透传 + create 落库（列可空，向后兼容）。
2. 图路由改造：新增 `scene_dispatch` 单一决策节点、`intent_recognition → scene_dispatch` 改边、调整入口与 `fallback` 路由、`scene_dispatch` 回写、Resolver 缓存与 initial-state 注入、新增 `StateKeySessionTag` 常量。
3. 文档：更新 create_session / list_session API 文档。
4. 回滚：标签为可选字段，旧前端不传即与现网一致；如路由异常可回退至以 `intent_recognition` 为入口、直连 `llm/fallback` 的版本。

## Open Questions

- 回写 `session_tag` 采用同步还是异步（参考现有 `IncrContentCount` 异步模式）？倾向异步，失败仅告警。
- `session_tag` 非法值在 create 时是「拒绝」还是「忽略降级为无标签」？倾向拒绝（`InvalidParameter`）。
- `scene_dispatch` 回写标签复用现有 `UpdateAiagentSession` 还是新增专用写回接口？需结合 data-service 现有 update 能力确认。

> 落地结论：最终采用「新增 `session_tag` 独立列」方案（见 D1），而非早期草案的 `extension.session_tag`；回写复用 `UpdateAiagentSession`（仅更新非空字段，安全）。
