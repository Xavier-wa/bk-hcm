## Why

当前 GraphAgent 每次新 Run 的固定入口是 `intent_recognition` 节点，必须先做一次 LLM 意图分类才能路由到 `host_apply` 子流程，带来额外首包延迟与误分类风险。在「主机申领 workflow」场景下，前端已显式选定业务场景，意图实际已确定，应允许在会话级绑定场景标签（`session_tag`）并在 Run 时直达对应 ReAct 子流程，跳过意图识别。

同时，对于未绑定标签的会话，意图识别一旦识别为受支持场景（当前仅主机申领），应将该结果回写为会话的 `session_tag`，使后续轮次稳定直达，避免重复意图识别与状态漂移。

## What Changes

- **会话标签持久化**：`create_session` 接口支持传入可选 `session_tag`，全链路（agent-server → data-service → DAO）持久化到 `aiagent_session` 表的独立列 `session_tag`；`list_session` 接口返回 `session_tag`。
- **新增场景分发节点（scene_dispatch）作为单一路由决策中心**：作为 Graph 新的入口节点，集中负责路由决策：
  - 会话有受支持 `session_tag`（当前仅 `host_apply`）→ 跳过意图识别，直达 `llm` ReAct 子流程；
  - 无标签且本轮尚无意图 → 进入 `intent_recognition` 意图识别节点；
  - 本轮已有意图（由意图识别返回）→ 判定是否受支持：受支持则回写 `session_tag` 并路由 `llm`，不支持/未识别则预置「暂不支持」提示并进入 `fallback`。
- **意图识别节点只分类、回到 scene_dispatch**：`intent_recognition` 不再直连 `llm`/`fallback`，仅写入本轮意图并返回 `scene_dispatch`，由 `scene_dispatch` 统一判定与路由（含 `host_apply` 命中时回写 `session_tag`）。
- **fallback 路由调整**：会话已绑定受支持 `session_tag` 时，`fallback` 恢复后直达 `llm`，**不再**回到 `intent_recognition` 或 `scene_dispatch`；无标签会话则在 `fallback` 恢复时清空本轮意图并路由回 `scene_dispatch` 重新识别。
- 受支持场景集合当前仅 `host_apply`，设计预留扩展点（未来可加入资源查询等子流程）。

## Capabilities

### New Capabilities

- `agent-session-tag`: 会话场景标签（`session_tag`）持久化能力，覆盖枚举校验、create_session 写入、list_session 返回、agent-server/data-service/DAO 全链路透传与运行时回写。
- `agent-scene-dispatch`: 场景分发节点能力，作为 Graph 入口，按会话 `session_tag` 决定直达 ReAct 或进入意图识别，并定义未支持场景的回复与回流路由。

### Modified Capabilities

- `agent-intent-routing`: Graph 入口由 `intent_recognition` 改为 `scene_dispatch`；意图识别为 `host_apply` 时需回写 `session_tag`；`fallback` 路由规则调整为「有标签直达 llm，无标签回 scene_dispatch」。

## Impact

- **数据库**：在 `aiagent_session` 表新增独立列 `session_tag VARCHAR(64) DEFAULT ''`，可空向后兼容。
- **API 文档**：`docs/api-docs/web-server/docs/service/agent/create_session.md`、`list_session.md` 新增 `session_tag` 字段说明。
- **协议层**：`pkg/api/agent-server/session/session.go`（CreateSessionReq/Resp）、`pkg/api/data-service/aiagent/session.go`（CreateAiagentSessionReq/Result）新增 `session_tag`。
- **枚举层**：复用 `pkg/criteria/enumor/aiagent.go` 的 `IntentType` 作为 `session_tag` 取值校验。
- **agent-server**：`service/session/create.go` 透传标签；`logics/agent/graph_build.go` 新增 `scene_dispatch` 节点与路由函数、调整入口与 fallback 路由；运行时通过会话加载/回写 `session_tag`（`service/service.go` 的 RunOptionResolver、`service/session/middleware.go` Resolver）。
- **data-service**：`cmd/data-service/service/aiagent/session.go` 创建逻辑落库 `session_tag`。
- **不影响**：意图识别节点的分类算法、AG-UI/checkpoint 协议、旧前端不传 `session_tag` 时的兼容行为（保持现网走 `scene_dispatch → intent_recognition`）。
