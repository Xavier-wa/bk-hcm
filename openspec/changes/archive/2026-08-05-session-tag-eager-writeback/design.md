## Context

`session_tag` 有两个存储：graph checkpoint（真相源）与 `aiagent_session.session_tag` 列（前端读取的投影）。现状是投影只在一次 Run 结束后由 `sessionCodeMiddleware` 起 goroutine 对账更新：

```
scene_dispatch 决策 ── emit scene.switched ──┐
                                            │ 新场景子图完整 LLM 推理 / 直到 HITL interrupt
                                            ▼
                                  ServeHTTP 返回 → go reconcileSessionTag → 写 DB
└────────────────── 前端读 DB 拿到旧标签的窗口 ──────────────────┘
```

两条既有事实决定了本次的落点：

1. `scene_dispatch` 的决策在一个 run 内**只做一次且不会被推翻**（`agent-scene-dispatch` 契约：入边仅 `{START, fallback}`），所以决策点提交的标签就是本轮的终值，此刻写 DB 语义上是安全的。
2. 「节点内 mid-run 持久化」在 agent-server 已是既有范式且都是**同步**的：`account_select` 的 `persistSelectedAccount` 直接 `sessSvc.UpdateSessionState`，`skill` 侧的 `state.PersistStateToService` 甚至同步 sleep 300ms 再写 MySQL。本次不引入第三种范式。

## Goals / Non-Goals

**Goals:**
- 把 `session_tag` 投影的滞后从「本轮剩余全程」压到「一次 data-service 调用」
- 保证 DB 更新**早于**前端收到 `scene.switched`，使前端无论走事件还是走重新拉取都读到新标签
- 覆盖首次打标（`"" → 受支持场景`），而非只覆盖场景切换
- 回写失败绝不影响本轮对话的路由与输出

**Non-Goals:**
- 不改决策矩阵、路由结果、`scene.switched` 的事件名与 payload
- 不追求「DB 与 checkpoint 强一致」；两个存储间的最终一致由既有兜底对账保证
- 不改前端。当前对话内标签 UI 的实时刷新需要前端消费 `scene.switched`（`front/src` 目前无消费方），是独立议题
- 不改 `aiagent_session` 表结构、data-service 接口、对外 API

## Decisions

### 决策一：节点内同步写，不用 goroutine

**选择**：在 `scene_dispatch` 节点内同步调用 data-service 完成回写，排在 `EmitSceneSwitched` 之前。

**理由**：
- 延迟增量可忽略——同一个节点里已经同步跑了一次意图分类的 LLM 调用（`intent.Classify`），一次 data-service 调用相对它是噪声
- 同步才能给出「DB 先于事件」的**顺序保证**；goroutine 与 emit 竞争，仍留一个 RTT 的窗口
- 不需要 background ctx、不需要把 kit 复制进 goroutine、不需要处理「run 已结束而 goroutine 还在跑」的单测时序

**代价**：用的是请求 ctx，客户端断连导致 ctx cancel 时这次写会失败。由决策五的兜底覆盖。

**备选**：goroutine + background ctx（断连也能写成功，但放弃顺序保证，且引入并发写与测试时序复杂度）；在 service 层嗅探 SSE 字节流识别 `scene.switched` 再触发（否决：依赖事件流的文本格式，脆弱且与事件 payload 强耦合）。

### 决策二：触发条件是「标签有变化」，不是「发生切换」

判据为 `decision.sessionTag != "" && decision.sessionTag != tag`，与 `decision.switched` 解耦——后者只决定发不发 `scene.switched` 事件。

首次打标（`"" → host_apply`）不发事件但脏读窗口一样长，而且它是每个新会话都会经历的最高频路径。若按 `switched` 判定，最常见的一次错标反而漏掉。

### 决策三：以 threadID 定位会话，不引入 sessionCode

节点内可从 `inv.RunOptions.RuntimeState[graph.CfgKeyLineageID]` 取到 threadID（`cvm_apply/session_key.go` 的 `resolveThreadID` 已有同款解析），而 `UpdateAiagentSessionReq` 正是以 `ID = threadID` 定位，因此**不需要** sessionCode，也就不必为此在 ctx 上再挂一个请求级值。

代价是节点内无法调用 `resolver.UpdateCachedSessionTag`（它按 sessionCode 索引）。这个代价可接受：resolver 缓存只服务于「下一轮请求把标签注入 forwardedProps」，而框架的 `mergeInitialStateNonInternal` 只补 checkpoint 中缺失的 key，checkpoint 里已有的 `session_tag` 优先级更高，**缓存滞后不会改变路由**。缓存刷新仍由兜底路径负责。

### 决策四：依赖用形参注入，且必须容忍为 nil

`makeSceneDispatchNode` 增加回写客户端形参，由已持有 `clientSet` 的 `BuildGraph` 透传，不从 ctx 里取客户端。

客户端为 nil 时 SHALL 安全跳过回写并打 Warn，不返回错误：既是防御，也让现有的 `makeSceneDispatchNode` 单测（覆盖决策矩阵与事件发送）可以继续传 nil，不必为每个用例架一个假的 data-service。

### 决策五：保留 Run 结束后的兜底对账，接受同值双写

`reconcileSessionTag` 原样保留。它现在覆盖的是：节点内写失败、请求 ctx 被 cancel、以及 resolver 缓存刷新。

代价是一次标签变化通常会产生**两次同值 DB 写**。原因是 `ResolveMeta` 返回的是缓存条目的副本（`meta, ok := r.cache.Get(...)` 后 `return &meta`），中间件传给对账的 `originalTag` 是请求开始时的快照，节点内的写它看不见，因此对账仍会判定「有差异」而再写一次。写是幂等的，本次接受；若后续要去重，方向是在 ctx 上挂一个请求级的原子单元让两侧共享判据，属独立优化。

### 决策六：Reviser 必须有兜底用户名

`UpdateAiagentSessionReq.Reviser` 带 `validate:"required"`，节点内若从 `authlogic.BKUsernameFromContext(ctx)` 拿到空串，请求会直接被校验拒绝。取 `constant.BackendOperationUserKey` 兜底并打 Warn，而不是放弃回写——`logics/tool/tool.go` 的内部 MCP hook 已是同一处理方式。

### 决策七：回写用独立短超时常量

新增回写超时常量（量级 3s，短于既有的 `SessionIncrContentCountTimeout`），从请求 ctx 派生 timeout ctx。回写是同步的，超时上限直接计入本轮 run 的延迟，不能沿用偏长的对账超时。

## Risks / Trade-offs

- **请求 ctx 被 cancel（客户端断连）导致节点内回写失败** → 只打 Warn；`scene_dispatch` 是本轮第一个节点，其超步 checkpoint 基本必然已落盘，Run 结束后的兜底对账能读到新标签补写
- **DB 短暂超前 checkpoint**（写成功后 run 崩溃、本超步 checkpoint 未落盘） → 下一轮 merge 只补缺失 key，state 仍是旧标签，但轮次边界会无条件重新分类，立即收敛。方向上比现状（DB 恒定落后）更安全
- **同值双写** → 幂等，仅多一次 data-service 调用；已在决策五记录去重方向
- **同步写延长 run 延迟** → 独立短超时兜住上限；相对同节点内的 LLM 分类调用可忽略
- **主图节点 ctx 缺 invocation / user** → threadID 解析失败即跳过回写并打 Error（无法定位会话时写入是危险的），user 缺失走兜底用户名
- **与 `unify-skill-repo-drop-scene-map` 的关系** → 本次回写调用点只要求排在 `EmitSceneSwitched` 之前，不依赖同分支内 `ClearLoadedSkills` 是否存在，两个 change 无顺序耦合

## Migration Plan

无 DB schema 变更、无配置项变更、无接口变更，代码发布即生效。回滚为纯代码回退：删掉节点内回写后行为完全退回现状（兜底对账一直在原位）。
