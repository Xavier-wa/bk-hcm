## 1. 枚举与常量

- [x] 1.1 复用 `pkg/criteria/enumor/aiagent.go` 的 `IntentType` 作为 `session_tag` 取值，`Validate()` 已覆盖 `host_apply`/`resource_query`/`chat`
- [x] 1.2 `pkg/criteria/constant/aiagent.go` 新增 `StateKeySessionTag = "session_tag"`、`ForwardedPropSessionTag = "sessionTag"` 常量

## 2. 协议层透传 session_tag

- [x] 2.1 `pkg/api/agent-server/session/session.go`：`CreateSessionReq`/`CreateSessionResp` 新增 `SessionTag`，`Validate()` 对非空值做 `IntentType` 校验；`ListSessionsResult` 直接复用 `core.ListResultT[aiagent.SessionTable]`（`session_tag` 由表字段序列化返回，无需额外包装类型）
- [x] 2.2 采用独立列方案：`pkg/dal/table/aiagent/session.go` `SessionTable` 与 `SessionColumnDescriptor` 新增 `session_tag` 列；`scripts/sql` 同步 DDL
- [x] 2.3 `pkg/api/data-service/aiagent/session.go`：`CreateAiagentSessionReq`/`UpdateAiagentSessionReq` 新增 `SessionTag` 字段（运行时回写复用 Update）；客户端透传无需改动

## 3. data-service 落库与查询（独立列）

- [x] 3.1 创建落库：`cmd/data-service/service/aiagent/session.go` `CreateAiagentSession` 将 `req.SessionTag` 写入 `SessionTable.SessionTag`
- [x] 3.2 查询读取：`session_tag` 为表列，List 直接返回，无需 JSON 解析或辅助方法
- [x] 3.3 回写能力：`UpdateAiagentSession` 透传 `req.SessionTag`（DAO 仅更新非空字段，空值不覆盖，安全）

## 4. agent-server 会话创建

- [x] 4.1 `cmd/agent-server/service/session/create.go`：透传 `req.SessionTag` 给 data-service，响应回显 `session_tag`；`query.go` 直接返回 `result.Details`（`SessionTable` 已含 `session_tag`）
- [x] 4.2 校验失败返回 `InvalidParameter`，不传时与现网兼容

## 5. Resolver 缓存与 initial-state 注入

- [x] 5.1 `cmd/agent-server/service/session/middleware.go`：LRU 缓存值改为 `SessionMeta{ThreadID, SessionTag}`，新增 `ResolveMeta`（保留 `Resolve` 兼容）与 `UpdateCachedSessionTag`
- [x] 5.2 `sessionCodeMiddleware` 经 `ResolveMeta` 拿到 `session_tag`，通过 `forwardedProps` 透传到 Run
- [x] 5.3 `cmd/agent-server/service/service.go` `makeRunOptionResolver`：读取 `forwardedProps.sessionTag`，非空时注入 `runtimeState[StateKeySessionTag]`

## 6. Graph 拓扑改造（scene_dispatch 单一路由中心）

- [x] 6.1 `cmd/agent-server/logics/agent/graph_build.go`：新增 `scene_dispatch` 节点，入口点改为 `scene_dispatch`
- [x] 6.2 实现 `scene_dispatch` 节点：受支持本轮意图且原无标签时把 `StateKeySessionTag` 写入 state（纯 state 提交，不写 DB）
- [x] 6.3 实现 `scene_dispatch` 路由函数：受支持 `StateKeySessionTag` → llm；本轮有 `StateKeyIntent`（不支持）→ fallback；否则 → intent_recognition
- [x] 6.4 `intent_recognition` 出边改为 `intent_recognition → scene_dispatch`（移除直连 llm/fallback 的条件边）
- [x] 6.5 调整 `makePostFallbackRoutingFunc`：受支持 `StateKeySessionTag` → llm；无标签 → 路由回 `scene_dispatch`；`fallback` 节点 resume 时对无标签会话清空本轮 `StateKeyIntent`
- [x] 6.6 受支持标签判定收口为 `isSupportedScene`，预留扩展点

## 6b. 运行时回写 session_tag（middleware 对账）

- [x] 6b.1 `sessionCodeMiddleware` Run 结束异步钩子（`reconcileSessionTag`）：读取该 thread 最新 checkpoint 的 `ChannelValues[StateKeySessionTag]`
- [x] 6b.2 若标签非空且会话原为无标签，经 data-service `Update` 回写 `session_tag` 列并刷新 `Resolver` 缓存，失败仅记 Warn

## 7. 文档

- [x] 7.1 更新 `docs/api-docs/web-server/docs/service/agent/create_session.md` 新增 `session_tag` 入参与回显说明
- [x] 7.2 更新 `docs/api-docs/web-server/docs/service/agent/list_session.md` 新增 `session_tag` 返回字段说明

## 8. 测试

- [x] 8.1 `graph_build` 路由单测：`isSupportedScene`、scene_dispatch 节点提交、节点+路由组合直达 llm、未支持→fallback、无标签→intent_recognition、fallback 直达/回分发
- [x] 8.2 create session 校验单测（合法/非法/空 `session_tag`）+ `SessionTable.SessionTag` 字段单测
- [x] 8.3 Resolver 缓存单测：`UpdateCachedSessionTag` 命中更新保留 ThreadID、未命中不预热
