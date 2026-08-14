## ADDED Requirements

### Requirement: scene_dispatch 作为 Graph 入口与单一路由决策中心

`BuildGraph` SHALL 新增纯路由节点 `scene_dispatch` 并将其设为 Graph 入口点。`scene_dispatch` SHALL 不调用 LLM，而是作为唯一的路由决策中心，依据 `StateKeySessionTag`（会话级持久标签）与 `StateKeyIntent`（本轮意图识别结果）决定下一跳。

#### Scenario: scene_dispatch 是新 Run 的第一个节点

- **WHEN** 用户在会话中发送消息触发一次新的 Graph Run（无 interrupted checkpoint）
- **THEN** `scene_dispatch` 节点先于 `intent_recognition` 和 `llm` 执行

### Requirement: 有受支持 session_tag 时直达 ReAct 子流程

当 `StateKeySessionTag` 为受支持标签（当前仅 `host_apply`）时，`scene_dispatch` SHALL 跳过 `intent_recognition`，直接路由到 `llm` 节点。

#### Scenario: host_apply 标签会话首条消息直达 llm

- **WHEN** 会话 `session_tag=host_apply`，用户发送首条消息
- **THEN** `scene_dispatch` 直接路由到 `llm`，不触发意图识别 LLM 调用

#### Scenario: 受支持标签集合可扩展

- **WHEN** 判定标签是否直达由单一受支持标签判定逻辑收口
- **THEN** 新增受支持场景时仅需扩展该判定逻辑，无需改动 `scene_dispatch` 路由结构

### Requirement: 无 session_tag 且本轮无意图时进入意图识别

当 `StateKeySessionTag` 为空且本轮尚无 `StateKeyIntent` 时，`scene_dispatch` SHALL 路由到 `intent_recognition` 节点进行分类。

#### Scenario: 无标签会话首次进入意图识别

- **WHEN** 会话无 `session_tag`，本轮无 `StateKeyIntent`，用户发送消息
- **THEN** `scene_dispatch` 路由到 `intent_recognition`

### Requirement: 意图识别结果回到 scene_dispatch 后由其判定支持性

`intent_recognition` SHALL 仅写入本轮 `StateKeyIntent` 并路由回 `scene_dispatch`，不再直连 `llm` 或 `fallback`。`scene_dispatch` 在本轮已有 `StateKeyIntent` 时 SHALL 判定其是否受支持：受支持（`host_apply`）则回写 `session_tag`、置 `StateKeySessionTag` 并路由到 `llm`；不支持或未识别则预置未支持提示文案并路由到 `fallback`。

#### Scenario: 本轮识别为 host_apply 直达 llm 并回写标签

- **WHEN** 无标签会话本轮 `StateKeyIntent=host_apply`
- **THEN** `scene_dispatch` 回写 `session_tag=host_apply`、置 `StateKeySessionTag=host_apply` 并路由到 `llm`

#### Scenario: 本轮识别为不支持场景进入 fallback

- **WHEN** 无标签会话本轮 `StateKeyIntent` 为 `resource_query` / `chat` 或未识别
- **THEN** `scene_dispatch` 预置未支持提示文案并路由到 `fallback`，不进入 `llm`

#### Scenario: 不发生 scene_dispatch 与 intent_recognition 死循环

- **WHEN** `intent_recognition` 写入 `StateKeyIntent` 后回到 `scene_dispatch`
- **THEN** `scene_dispatch` 因本轮已有 `StateKeyIntent` 而落入判定支持性分支，不再路由回 `intent_recognition`

### Requirement: 未支持场景回复提示后回流 scene_dispatch 并重新识别

当未支持场景经 `fallback` 输出提示并中断后，无标签会话在用户下一条消息 resume 时 SHALL 清空本轮 `StateKeyIntent` 并路由回 `scene_dispatch`，由其重新进入 `intent_recognition`。

#### Scenario: 未支持后下一轮重新意图识别

- **WHEN** 无标签会话上一轮被判未支持并经 `fallback` 中断，用户发送下一条消息
- **THEN** resume 清空 `StateKeyIntent`，路由回 `scene_dispatch`，并重新进入 `intent_recognition`
