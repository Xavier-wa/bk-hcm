## Context

agent-server 已在变更 `2026-04-20-agent-session-management` 中引入 `aiagent_session` 业务会话层：对外暴露 `session_code`，通过 `sessionCodeMiddleware` 解析为框架 `threadId`，平台级 API 使用 IAM `agent_assistant` 权限。

当前会话维度为 `app_name + user + thread_id`，无 `bk_biz_id`。多业务用户在切换业务时无法隔离会话列表；主机申领 Graph workflow 首轮仍需 LLM 向用户询问业务 ID。

**约束**：
- agent-server 不直连 DB，经 data-service 访问 `aiagent_session`
- 业务 API 路径对齐 woa-server：`/bizs/{bk_biz_id}/...`
- `/agui`、`/history`、`/cancel` 请求体不增加 `bk_biz_id`（业务上下文由 `session_code` 关联记录解析）
- `/agui`、`/history`、`/cancel`、`/sessions/{session_code}/context_stats` 解析 `session_code` 后必须显式校验会话 `user` 与当前登录用户一致
- `/agui` 运行入口继续使用平台 `agent_assistant` 权限门槛；业务权限覆盖业务维度 create/list/update/delete
- `session_code` 生成公式不变
- API 文档落点使用现有实际目录：`docs/api-docs/web-server/docs/biz/agent/`
- 本变更仅后端，前端独立需求单对接

**相关方**：agent-server / data-service 后端、IAM 配置、前端 Chatbot（后续）

## Goals / Non-Goals

**Goals:**

- `aiagent_session` 持久化 `bk_biz_id`，支持按业务过滤列表
- 提供业务维度 create/list/update/delete API，IAM「业务-智能体助手」鉴权
- 保留平台维度 create/list API，向后兼容
- 有效业务会话（`bk_biz_id > 0`）在 `/agui` 运行时自动注入 Graph State 与 Prompt，首轮不再询问 `bk_biz_id`
- 存量会话迁移后 `bk_biz_id = -1`（`constant.UnassignedBiz`），行为与改造前一致

**Non-Goals:**

- 前端 Chatbot 业务切换 UI 与路由改造
- 修改 `session_code` 生成算法
- `/agui`、`/history`、`/cancel` 请求体/路径增加 `bk_biz_id`
- 会话跨业务迁移、归档工具
- cloud-server / woa-server 入口（agent-server 直接对外）

## Decisions

### 1. bk_biz_id 作为独立列，不参与 session_code

**决策**：`aiagent_session` 新增 `bk_biz_id BIGINT NOT NULL DEFAULT -1`；`session_code = md5(app_name + user + thread_id) + "-" + YYYYMMDDHH` 公式不变。

**理由**：`session_code` 已在生产使用，变更生成规则会导致 breaking change；业务属性是会话元数据，与框架 thread 映射正交。

**替代方案**：将 `bk_biz_id` 编入 session_code → breaking，且 path 已携带 biz，冗余。

### 2. 索引策略：idx_app_user_biz

**决策**：新增联合索引 `idx_app_user_biz(app_name, user, bk_biz_id)`；保留现有 `idx_app_user`。

**理由**：业务 list API 固定过滤 `app_name + user + bk_biz_id`，联合索引覆盖主查询路径；平台 list 仍走 `user` 过滤，现有索引可用。

### 3. 业务 API 路由注册方式

**决策**：在 `session.InitService` 中参照 woa-server 模式，额外注册 `bizH.Path("/bizs/{bk_biz_id}")` 下的 create/list/update/delete handler。

```
POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create
POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list
PATCH /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}
DELETE /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}
```

**理由**：与项目既有业务 API 风格一致（`cmd/woa-server/service/cvm/service.go`）；path 参数 `bk_biz_id` 为权威来源，body 若含 biz 字段以 path 为准。

**handler 逻辑要点**：
- 解析 path `bk_biz_id`，校验 `> 0`，否则 `InvalidParameter`
- IAM 鉴权：`meta.BizAgentAssistant` + `Action: Create/Find/Update/Delete` + `BizID: bk_biz_id`
- create：写入 `bk_biz_id` 至 data-service
- list：强制 `tools.And(Equal("user", currentUser), Equal("bk_biz_id", pathValue))`，忽略客户端传入的 biz 过滤
- update：解析 `session_code` 后校验会话归属当前用户且记录 `bk_biz_id` 等于 path 值，再更新会话名称
- delete：解析 `session_code` 后校验会话归属当前用户且记录 `bk_biz_id` 等于 path 值，再删除会话

### 4. 平台 API 语义扩展（非 breaking）

**决策**：
- `POST /sessions/create`：创建时 `bk_biz_id = constant.UnassignedBiz (-1)`
- `POST /sessions/list`：服务端不主动追加 `bk_biz_id` 过滤；仍按现有机制合并客户端传入 filter
- 鉴权保持 `meta.AgentAssistant`（平台-智能体助手）

**理由**：兼容旧客户端与管理场景；Chatbot 主流程由前端切至业务 API（独立需求单）。

### 5. IAM：新增业务粒度 Action

**决策**：新增 IAM Action `biz_agent_assistant`（显示名「业务-智能体助手」），`RelatedResourceTypes: bizResource`，`RelatedActions: [BizAccess]`，模式对齐 `ZiyanResCreate`。

同步新增：
- `pkg/iam/sys/types.go`：`BizAgentAssistant client.ActionID`
- `pkg/iam/meta/resource.go`：`BizAgentAssistant ResourceType`
- `pkg/iam/sys/initial_actions.go`：Action 注册
- `cmd/auth-server/service/auth/adaptor.go` + `gen_id.go`：资源实例生成（CMDB biz 资源，`BizID` 来自 path）

**理由**：业务 API 需要按 `bk_biz_id` 实例鉴权；平台 `agent_assistant` 无业务资源维度，不可复用。

**替代方案**：复用 `meta.Biz + meta.Access` → 语义过宽，无法独立授权「智能体助手」能力。

### 6. Resolver 扩展：缓存 session 元数据与归属用户

**决策**：将 `Resolver.Resolve` 扩展为返回 `SessionMeta{ThreadID, BkBizID, User}`（或新增 `ResolveMeta` 方法）；LRU 缓存 value 从 `string` 改为 struct，容量/TTL 不变（10000 / 30min）。

**理由**：middleware 与 `makeRunOptionResolver` 均需 `bk_biz_id`；`session_code` 是对外会话标识，`/agui`、`/history`、`/cancel` 也需要校验会话归属，避免其他用户仅凭泄露的 `session_code` 访问会话。Resolver 已查询 data-service List by session_code，一次查询同时获取 thread_id、bk_biz_id 与 user，避免重复 DB 调用。

**替代方案**：middleware 单独查库 → 每条 `/agui` 请求多一次 data-service 调用（缓存 miss 时）。

### 7. 业务上下文注入：Context 透传 + BeforeModel 注入

**决策**：分两层注入：

```
sessionCodeMiddleware
  → ResolveMeta(sessionCode) → threadID + bkBizID + user
  → 校验 meta.User == currentUser，否则拒绝访问
  → 改写 body: sessionCode → threadId + runId
  → 若为 /agui 且 bkBizID > 0，则通过 r.WithContext(ctx) + 私有 context key 透传 bkBizID

makeRunOptionResolver (已有)
  → 从请求 context 读取 bkBizID
  → 若 bkBizID > 0:
       runtimeState["bk_biz_id"] = bkBizID        // Graph Function 节点消费
  → agent.WithRuntimeState(runtimeState)

BeforeModel callback（cmd/agent-server/logics/agent/graph_build.go）
  → 从 invocation/session/request 上下文读取 bkBizID
  → 若 bkBizID > 0:
       写入 session temp 状态 temp:session_bk_biz_id
       在模型调用前注入业务上下文 Prompt 片段
```

**Prompt 注入方式**（推荐）：
- 在 `instruction.md` 增加可选占位段落，例如：`当前会话业务 ID：{temp:session_bk_biz_id?}`
- `/agui` 请求经 middleware 透传业务上下文后，在 `BeforeModel` 阶段写入 session `temp:session_bk_biz_id`（仅当 `bk_biz_id > 0`）；`/history`、`/cancel` 不触发模型调用，不写入 temp 状态
- 当 `bk_biz_id = -1` 时不写入，占位符为空，instruction 保持现网行为

**理由**：
- Graph State 注入满足 `fetch_plans` 等 Function 节点直接读 `bk_biz_id`
- `{temp:*}` 占位符为框架内置能力（见 `examples/graph/retrieval_placeholder`），无需改 Graph 拓扑
- middleware 处于入口层，只负责解析与透传会话元数据，不直接修改 Graph/session state
- 使用私有 context key 并通过 `r.WithContext(ctx)` 向下游透传，可避免客户端伪造内部 header，也不会污染对外 API 协议
- Prompt/state 更新放在 `BeforeModel`，与现有技能上下文、时间上下文等模型前置注入点一致

**替代方案 A**：仅 State 注入、不改 Prompt → LLM 仍可能口头询问 biz（不满足 R-004）

**替代方案 B**：改 Graph 入口节点读 State → 改动面大，非本需求必要

**替代方案 C**：middleware 直接写 session temp 状态 → 入口层职责过重，且与模型前置上下文注入链路分散，放弃。

### 8. middleware 读取失败时的处理策略

**决策**：`ResolveMeta` 失败时返回 400（与现网 invalid session code 一致）；`/agui`、`/history`、`/cancel` 与 `GetContextStats` 解析到的 `SessionMeta.User` 若与当前 `kt.User` 不一致，则返回 `PermissionDenied`；升级时 DDL 默认值会将历史数据统一迁移为 `bk_biz_id = -1`，不考虑字段缺失的运行时兼容分支。

**理由**：session 不存在应快速失败；session 存在但不属于当前用户属于越权访问，应明确拒绝；本变更要求 DB 迁移先完成，数据层响应始终包含明确的 `bk_biz_id`。

### 9. API 响应字段

**决策**：`CreateSessionResp` 不增加 `BkBizID` 字段；`ListSessionsResult.Details` 保持返回 `BkBizID int64` 字段。平台与业务 create 仅返回会话创建结果，业务归属以持久化记录为准；业务 create 的 `bk_biz_id` 可由 path 明确确定。

**理由**：create 响应只需返回 `session_code` 等会话标识；列表响应仍需带回 `bk_biz_id` 供前端展示与调试，避免 create 响应暴露冗余字段。

### 10. data-service 变更范围

**决策**：贯通以下链路：
- `pkg/dal/table/aiagent/session.go`：字段 + ColumnDescriptor
- `pkg/dal/dao/aiagent/session.go`：INSERT/SELECT/UPDATE 列
- `pkg/api/data-service/aiagent/session.go`：Create/List 请求响应
- `cmd/data-service/service/aiagent/session/`：handler 校验（create 时接受 bk_biz_id，默认 -1）
- SQL 迁移脚本（`9999_*_aiagent_session_bk_biz_id.sql`）

Create 接口：`bk_biz_id` 可选，未传时默认 `-1`；若显式传入 `0` 则按 `0` 写入，不做特殊转换。agent-server 业务 API 显式传入 path 值且校验 `> 0`。

### 11. GetContextStats 路径参数

**决策**：`GetContextStats` 使用 `session_code` 作为路径参数，与更新/删除会话等外部接口保持一致。

**理由**：`session_code` 是对外会话标识，`thread_id` 属于框架内部 ID；业务隔离改造不应扩大对外暴露的内部 ID 使用面。

## Risks / Trade-offs

- **[缓存一致性]** Resolver 缓存不含 bk_biz_id 更新场景（创建后 biz 不可变）→ 映射创建后不变，风险极低；会话删除后 TTL 兜底
- **[IAM 未同步]** 新 Action 未注册到 IAM 平台 → 业务 API 全员 PermissionDenied → 迁移脚本与 IAM 注册同版本发布，发布 checklist 包含 IAM 同步
- **[Prompt 注入遗漏]** 仅 State 注入而 Prompt 未生效 → LLM 仍询问 biz → 验收 AC-005 覆盖；instruction 模板增加明确「不得询问业务 ID」约束句
- **[双 API 并存]** 前端误用平台 create 产生 `bk_biz_id=-1` 会话 → 文档说明 + 前端需求单切换；平台 create 保留供兼容
- **[session_code 泄露]** 其他用户持有 `session_code` 访问 `/agui` → Resolver 返回 user，middleware 校验归属后拒绝
- **[性能]** Resolver 缓存 struct 略增内存 → 每条约 +8 字节，10000 条可忽略

## Migration Plan

1. **DB 迁移**（先发 data-service 兼容读写的版本，或同事务发版）：
   ```sql
   ALTER TABLE aiagent_session
     ADD COLUMN bk_biz_id BIGINT NOT NULL DEFAULT -1 COMMENT '会话所属业务；-1=未分配'
     AFTER user;
   CREATE INDEX idx_app_user_biz ON aiagent_session (app_name, user, bk_biz_id);
   ```
   存量行自动 `-1`，无需数据回填脚本。

2. **发布顺序**：
   1. IAM Action 注册（`biz_agent_assistant`）并授权
  2. data-service（先贯通表结构、DAO、API 类型与 handler，支持读写 bk_biz_id）
  3. agent-server（在 data-service 字段可用后实现平台语义扩展、业务 API、Resolver/middleware 与 Graph 注入）
   4. API 文档

3. **回滚**：回滚 agent-server 至旧版即可（新 API 不可用，旧 API 仍可用）；DB 列可保留（旧代码忽略该列）；若需完全回滚 DDL，需确认无业务 API 写入正值后再 DROP COLUMN（一般不建议）。

4. **验证**：
  - 业务 create/list/update/delete + IAM 拒绝
   - 平台 list 跨业务
   - `/agui` AC-005/006 与跨用户 session_code 拒绝
   - session_code 公式不变 AC-008

## Open Questions

| ID | 问题 | 当前倾向 |
|----|------|----------|
| Q-001 | 平台 `/sessions/create` 是否对 Chatbot 长期开放 | 保留，写入 -1 |
| Q-003 | Prompt 占位符 key 命名 | `{temp:session_bk_biz_id?}` |
| Q-004 | 业务 API 是否要求同时具备平台 `agent_assistant` | 否，仅 `biz_agent_assistant` + BizAccess |
| Q-005 | instruction 是否在 bk_biz_id 有效时追加「禁止询问业务 ID」硬约束句 | 是，implement 时在 instruction 模板补充 |
