## Why

`session_tag` 目前只在一次 Run 完整结束（`next.ServeHTTP` 返回）之后才回写 DB，而 `scene_dispatch` 在**本轮最开头**就已经定下了新场景并向前端发出了 `scene.switched`。这中间的脏读窗口不是毫秒级，而是**本轮剩余全程**：新场景子图的完整 LLM 推理（十秒量级），若新场景第一步就弹 HITL 卡片则要等到 interrupt 才结束。

前端会话列表与会话详情的 `session_tag` 都读自 DB，用户在这个窗口内切到别的会话再切回来，看到的就是**旧场景标签**——而这个操作的耗时与窗口是同一量级，必然撞上。窗口从「本轮剩余全程」压到「一次 data-service 调用」，这个用户可感知的错标就基本消失了。

## What Changes

- **新增**：`scene_dispatch` 在判定出「本轮要提交的标签 ≠ 进入本节点时 state 中的标签」时，**在节点内同步**经 data-service 把 `session_tag` 回写到 `aiagent_session`，位置排在 emit `scene.switched` **之前**，保证 DB 先于前端事件更新
- 触发范围为**任何标签变化**，不只是场景切换：无标签会话首次打标（`"" → host_apply`）同样有完整的脏读窗口，且是最高频的一次
- 写失败（含请求 ctx 被 cancel）**仅记录日志**，不阻断本轮 run、不改变路由决策、不影响 `scene.switched` 的发送
- **保留** service 层 Run 结束后的差异回写（`reconcileSessionTag`）作为兜底，覆盖节点内写失败、客户端断连与 resolver 缓存刷新；本次接受一次标签变化可能产生两次同值 DB 写（幂等）
- **不改变**：决策矩阵、路由结果、`scene.switched` 事件语义与 payload、DB schema、data-service 接口、对外 API

## Capabilities

### New Capabilities

（无。本次是既有 `session_tag` 回写能力的时机前移，不引入新的能力面）

### Modified Capabilities

- `agent-session-tag`：「运行时回写 session_tag」要求变更。原要求规定回写 SHALL 在 SSE 结束后触发、SHALL NOT 与 Run 并发；现改为**标签变化的判定点即回写**，Run 结束后的差异回写降级为兜底手段。回写触发条件、时序保证（DB 先于 `scene.switched`）、以及双写幂等的约定都需要落到该 spec 上

## Impact

**受影响代码**（全部在 agent-server，不跨服务层）：
- `cmd/agent-server/logics/agent/graph_build.go` — `makeSceneDispatchNode` 增加回写客户端形参（`BuildGraph` 已持有 `clientSet`，无需改其签名），切换/打标分支内新增回写调用；节点头部「本节点⋯不写 session_tag 到 DB」的注释需同步改写
- 新增回写实现（建议落在 `cmd/agent-server/logics/agent/state/` 下，与既有 `PersistStateToService` 同层）：从 ctx 拼 kit、从 `RuntimeState[graph.CfgKeyLineageID]` 取 threadID、调 `DataService().Aiagent.Session.Update`
- `pkg/criteria/constant/aiagent.go` — 新增回写超时常量；`StateKeySessionTag` 的注释（现写「Run 结束后由 service 层对账回写 DB」）需补充判定点回写为主
- `cmd/agent-server/service/service.go` — `reconcileSessionTag` 逻辑保持不动，仅补注释说明它已降级为兜底

**不受影响**：
- data-service 侧：复用现有 `UpdateAiagentSessionReq`，无接口与 DAO 改动
- `pkg/dal` 与 `aiagent_session` 表结构：无变更
- 前端：本次不改前端；「当前对话内标签 UI 实时刷新」仍依赖前端消费 `scene.switched`（目前 `front/src` 尚无消费方），属独立议题

**已知风险**：
- `UpdateAiagentSessionReq.Reviser` 为 `required`，节点内 ctx 缺 `bk_username` 时须有兜底用户名，否则请求校验直接失败
- 节点内写成功但本超步 checkpoint 未落盘（run 中途崩溃）时，DB 会短暂**超前**于 checkpoint；下一轮轮次边界重新分类后自然收敛，方向上比现状（DB 恒定落后）更安全
