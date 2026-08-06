# Capability: agent-scene-dispatch

## MODIFIED Requirements

### Requirement: 有受支持 session_tag 时直达 ReAct 子流程

当 `StateKeySessionTag` 为受支持标签（当前为 `host_apply` / `resource_query`）且**本轮意图分类结果未指向其它受支持场景**时，`scene_dispatch` SHALL 路由到该标签对应的场景子图节点。

`scene_dispatch` SHALL NOT 再因「已有受支持标签」而跳过意图识别：每个轮次边界都先分类一次（详见 `agent-scene-switch`），分类完成后才依据「当前标签 × 分类结果」决定进入哪个子图。

#### Scenario: host_apply 标签会话在分类后进入 host_apply 子图

- **GIVEN** 会话 `session_tag=host_apply`，本轮分类结果为 `host_apply`
- **WHEN** `scene_dispatch` 做路由决策
- **THEN** 路由到 `host_apply` 子图节点

#### Scenario: host_apply 标签会话分类为闲聊时仍进入 host_apply 子图

- **GIVEN** 会话 `session_tag=host_apply`，本轮分类结果为 `chat`
- **WHEN** `scene_dispatch` 做路由决策
- **THEN** 保持 `session_tag=host_apply` 并路由到 `host_apply` 子图，不路由 `fallback`

#### Scenario: 受支持标签集合可扩展

- **WHEN** 判定标签是否受支持由单一受支持标签判定逻辑（`IntentType.IsSupportedScene`）收口
- **THEN** 新增受支持场景时仅需扩展该判定逻辑与标签到子图节点的映射，无需改动 `scene_dispatch` 的路由结构

### Requirement: 意图识别结果回到 scene_dispatch 后由其判定支持性

意图识别 SHALL 由 `scene_dispatch` 节点内部同步调用纯函数完成（`intent.Classify`），SHALL NOT 作为独立的主图节点存在，主图 SHALL NOT 保留 `scene_dispatch → intent_recognition → scene_dispatch` 的往返边。

`scene_dispatch` 取得分类结果后 SHALL 按「当前 `session_tag` × 分类结果」的决策矩阵（定义于 `agent-scene-switch`）判定去向：分类结果受支持时提交为 `StateKeySessionTag` 并路由到对应子图（与原标签不同即构成场景切换）；分类结果不受支持时，已有受支持标签的会话保持原标签并进入原场景子图，无标签会话则路由到 `fallback` 给出未支持提示。

#### Scenario: 本轮分类为 host_apply 进入 host_apply 子图并提交标签

- **WHEN** 无标签会话本轮分类结果为 `host_apply`
- **THEN** `scene_dispatch` 提交 `StateKeySessionTag=host_apply` 并路由到 `host_apply` 子图

#### Scenario: 本轮分类为不支持场景且会话无标签时进入 fallback

- **WHEN** 无标签会话本轮分类结果为 `chat` 或未识别
- **THEN** `scene_dispatch` 路由到 `fallback` 输出未支持提示文案，不进入任何子图

#### Scenario: 主图不存在意图识别节点

- **WHEN** `BuildGraph` 完成构图
- **THEN** 主图节点集合中不含 `intent_recognition`，`scene_dispatch` 的出边目标仅为两个场景子图节点与 `fallback`

#### Scenario: 不存在无中断保护的环

- **WHEN** 检查主图的环
- **THEN** 唯一的环为 `scene_dispatch → 子图/fallback → fallback → scene_dispatch`，其每一圈都被 `fallback` 的 interrupt 阻断，需要新的用户消息才能继续

## ADDED Requirements

### Requirement: scene_dispatch 节点做判定、路由函数只查表

`scene_dispatch` 的全部路由判定 SHALL 集中在节点函数中完成，并把决策结果写入状态键 `StateKeySceneDispatchNext`（取值为目标节点名）。条件边路由函数 SHALL 只读取该键并映射到目标节点，SHALL NOT 重复实现任何 tag/分类结果的判定逻辑。

此约束消除节点与路由函数两处判定不一致的风险：随着判定分支从 2 个增加到 5 个（决策矩阵），重复实现必然发散。该模式与子图内 `account_select` 节点写 `AccountSelectNextNodeKey`、`makeAccountSelectRoutingFunc` 查表的既有做法一致。

#### Scenario: 路由函数不含判定逻辑

- **WHEN** `scene_dispatch` 节点写入 `StateKeySceneDispatchNext=resource_query`
- **THEN** 路由函数读取该键并返回 `resource_query`，不重新读取 `StateKeySessionTag` 做判断

#### Scenario: 决策键缺失时安全兜底

- **WHEN** 路由函数读到的 `StateKeySceneDispatchNext` 为空或非法节点名
- **THEN** 路由到 `fallback` 并记录 Warn 日志，不返回错误中断 run

### Requirement: scene_dispatch 每轮刷新 session_rid

`scene_dispatch` SHALL 在每次执行时把当前请求的 rid 写入 `SessionRidStateKey`。

该键由 `makeRunOptionResolver` 在 run 入口注入 RuntimeState，但框架的 `mergeInitialStateNonInternal` 只补 checkpoint 中缺失的 key，第二轮起注入被 checkpoint 中的旧值压制，导致子图 InputMapper / OutputMapper 的 `[hcm graph trace]` 日志 rid 与本轮请求对不上。由 `scene_dispatch` 每轮强制刷新可修复该问题。

#### Scenario: 第二轮起子图日志 rid 与本轮请求一致

- **GIVEN** 会话已存在 checkpoint，其中 `session_rid` 为上一轮的 rid
- **WHEN** 用户发送新消息，`scene_dispatch` 执行
- **THEN** `SessionRidStateKey` 被刷新为本轮 rid，子图 mapper 日志打印的 rid 与本轮请求一致
