# Capability: agent-scene-switch

## Purpose

定义 AI Agent 会话在**轮次边界**上的场景（意图）切换能力：一个场景子图完整跑完、控制权回到主图之后，系统重新识别用户本轮意图；识别结果与当前会话场景标签不一致时，在同一个 run 内切换到新场景并给出回答，同时向前端发送 `scene.switched` 事件以同步会话标签 UI。子图内部 HITL 中断点上的切换不在本能力范围内。

## Requirements

### Requirement: 轮次边界重跑意图识别

系统 SHALL 在每个轮次边界重新识别用户意图，不再因会话已有受支持 `StateKeySessionTag` 而跳过意图识别。

轮次边界 SHALL 定义为「主图 `scene_dispatch` 节点被执行」这一事实本身：上一轮若停在场景子图内部的 HITL 中断上，框架 resume 到的是子图中断节点，主图 `scene_dispatch` 不会执行，因此不构成轮次边界；上一轮若子图已跑完并停在主图 `fallback` 中断上，用户下一条消息 resume 后经 `fallback → scene_dispatch` 静态边进入 `scene_dispatch`，构成轮次边界。系统 SHALL NOT 为判定轮次边界引入任何额外的探测状态或标记。

意图分类 SHALL 在 `scene_dispatch` 节点内部同步调用完成（见 `agent-scene-dispatch` 的节点合并要求），SHALL NOT 通过与独立节点往返的方式取得。`scene_dispatch` 在一个 run 内 SHALL 只执行一次，因此 SHALL NOT 需要任何「本 run 是否已识别过」的护栏状态。

#### Scenario: 已有标签的会话在轮次边界重新识别意图

- **GIVEN** 会话 `session_tag=host_apply`，上一轮子图已跑完并停在主图 `fallback` 中断
- **WHEN** 用户发送下一条消息
- **THEN** `scene_dispatch` 调用意图分类，而非直达 `host_apply` 子图

#### Scenario: 停在子图 HITL 中断时不触发重新识别

- **GIVEN** 会话 `session_tag=host_apply`，上一轮停在 `host_apply` 子图内的提单门禁卡片中断上
- **WHEN** 用户在卡片上回复
- **THEN** 框架 resume 到子图中断节点，`scene_dispatch` 不执行，不发生意图重新识别，也不发生场景切换

#### Scenario: 无标签会话首轮行为不变

- **GIVEN** 会话无 `session_tag`
- **WHEN** 用户发送首条消息
- **THEN** `scene_dispatch` 调用意图分类，结果受支持时提交 `session_tag` 并进入对应子图，与现网行为一致

#### Scenario: 单个 run 内只分类一次

- **GIVEN** 任意一次 run
- **WHEN** 该 run 执行完毕
- **THEN** `scene_dispatch` 恰好执行一次，意图分类恰好发生一次

### Requirement: 识别结果与会话标签的决策矩阵

`scene_dispatch` SHALL 按「当前 `session_tag` × 本轮分类结果」的组合决定去向。分类结果 SHALL 作为节点内的局部值使用，SHALL NOT 写入 graph state。

| 当前 tag | 本轮分类结果 | 决策 |
|---|---|---|
| 受支持 | 受支持且不等于 tag | 提交新 tag、发 `scene.switched`、路由新场景子图 |
| 受支持 | 等于 tag | 保持 tag，路由原场景子图 |
| 受支持 | 不受支持（`chat` 等） | 保持 tag，路由原场景子图 |
| 空 | 受支持 | 提交 tag，不发 `scene.switched`，路由对应子图 |
| 空 | 不受支持 | 不提交 tag，路由 `fallback` 给出未支持提示 |

#### Scenario: 主机申领会话切换到资源查询

- **GIVEN** 会话 `session_tag=host_apply`，上一轮子图已跑完
- **WHEN** 用户发送「帮我查下我有哪些主机」，本轮分类为 `resource_query`
- **THEN** `scene_dispatch` 将 `StateKeySessionTag` 提交为 `resource_query`，发出 `scene.switched` 事件，并在同一个 run 内路由到 `resource_query` 子图完成回答

#### Scenario: 资源查询会话切换到主机申领

- **GIVEN** 会话 `session_tag=resource_query`，上一轮子图已跑完
- **WHEN** 用户发送「那帮我申领 3 台」，本轮分类为 `host_apply`
- **THEN** `scene_dispatch` 将 `StateKeySessionTag` 提交为 `host_apply`，发出 `scene.switched` 事件，并路由到 `host_apply` 子图

#### Scenario: 场景内追问不触发切换

- **GIVEN** 会话 `session_tag=host_apply`，上一轮子图已跑完
- **WHEN** 用户发送「改成 16 核」，本轮分类为 `host_apply`
- **THEN** `session_tag` 保持 `host_apply`，不发出 `scene.switched` 事件，路由到 `host_apply` 子图

#### Scenario: 场景内闲聊留在原场景由场景 LLM 回答

- **GIVEN** 会话 `session_tag=host_apply`，上一轮子图已跑完
- **WHEN** 用户发送「谢谢」，本轮分类为 `chat`
- **THEN** `session_tag` 保持 `host_apply`，路由到 `host_apply` 子图由场景 LLM 回答；不清历史、不清 tag、不输出未支持提示文案

#### Scenario: 无标签会话识别为不支持场景仍走兜底提示

- **GIVEN** 会话无 `session_tag`
- **WHEN** 本轮分类为 `chat`
- **THEN** `scene_dispatch` 不提交 `session_tag`，路由到 `fallback` 输出未支持提示文案，与现网行为一致

### Requirement: 分类失败时不切换

意图分类在 LLM 调用失败、响应错误或返回值无法识别时 SHALL 返回 `chat`，使决策矩阵落入「不受支持」分支。对已有受支持标签的会话，这意味着**保持原场景**；系统 SHALL NOT 因分类失败而清空标签、切换场景或中断本轮 run。

#### Scenario: LLM 调用失败时留在原场景

- **GIVEN** 会话 `session_tag=host_apply`
- **WHEN** 意图分类的 LLM 调用返回错误
- **THEN** 分类结果降级为 `chat`，会话保持 `host_apply` 并路由到 `host_apply` 子图，run 正常继续，同时记录日志

### Requirement: 切换后账号选择与子图状态的自然行为

场景切换 SHALL NOT 主动清理任何场景私有状态。子图状态随其 per-turn checkpoint namespace 留在旧 namespace，切回时是新 turn、新 namespace，从子图入口重跑。

`selected_account_id` 持久化在 session 后端且跨 run 保留，切走再切回 `host_apply` 时 SHALL 被 `tryReuseAccountID` 命中，不重复弹出选账号卡片。

#### Scenario: 切走再切回不需要重选账号

- **GIVEN** 会话在 `host_apply` 中已选定账号，随后切换到 `resource_query`
- **WHEN** 用户再次切回 `host_apply`
- **THEN** 已选账号被复用，不再弹出选账号卡片；推荐方案需要重新生成

### Requirement: scene.switched 自定义事件

场景切换成功时，`scene_dispatch` SHALL 发出名为 `scene.switched` 的 AG-UI CUSTOM 事件，payload 至少包含切换前场景 `from` 与切换后场景 `to`。事件 SHALL 通过框架的 node custom event 通道发出，由 AG-UI translator 原生转换，SHALL NOT 依赖中断（interrupt）产生。

仅在真正发生切换（`from` 非空且 `from != to`）时发出；无标签会话首次提交标签 SHALL NOT 发出该事件。

事件发送失败 SHALL 仅记录 Warn 日志，不阻断本轮对话。

#### Scenario: 切换时前端收到 scene.switched

- **WHEN** 会话从 `host_apply` 切换到 `resource_query`
- **THEN** SSE 流中出现一条 CUSTOM 事件，名称为 `scene.switched`，payload 含 `from=host_apply`、`to=resource_query`

#### Scenario: 首次提交标签不发事件

- **GIVEN** 会话无 `session_tag`
- **WHEN** 本轮分类为 `host_apply` 并提交为会话标签
- **THEN** 不发出 `scene.switched` 事件

#### Scenario: 事件发送失败不影响回答

- **WHEN** 事件 emit 返回错误
- **THEN** 系统记录 Warn 日志，切换与回答流程照常继续
