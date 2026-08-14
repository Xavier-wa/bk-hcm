## Context

### 背景

agent-server 当前对外仅暴露 AG-UI 协议（路径 `/api/v1/agent/agui` 等），入口来源是 web-server，请求中携带 `bk_ticket` 作为用户态凭证。Runner（`runTime.AGUIRunner`）底层通过 trpc-agent-go 调度 Agent / Model / ToolSet / Session / Memory，工具调用经 MCP ToolSet（`type: "bkaidev"`）走蓝鲸普通网关公开 MCP，注入 `X-Bkapi-Authorization`（包含应用凭证 + `bk_ticket`）。

为对接 OpenClaw 等外部 Agent，HCM 整体方案（iWiki 4021231666）将引入标准 A2A 协议作为内外协议桥接。本变更聚焦 **agent-server 侧**：在不影响现有 AG-UI 链路的前提下，新增 A2A 协议入口，并为后续 LLM 直连 api-server 内置 HCM MCP 铺路（MCP ToolSet 增加 `type: "internal"`）。

参考实现：`jettxiao/hcm@7be6e551`（题为 "a2atemp" 的预研提交）已搭建 A2A Server 挂载骨架，本设计在其基础上修订路径策略与 AgentCard 来源（见 D2、D7），并补齐 contextId 映射、MCP internal toolset、中间件链等遗漏部分。

### 现有约束

- **AG-UI 链路完全不动**：路径、中间件链、`sessionCode → threadId` 解析、`bk_ticket` 注入、AGUI 现有 MCP `bkaidev` toolset 全部保持。
- **Runner 单例**：A2A 与 AG-UI 共用同一 `runTime.AGUIRunner`，禁止重复构造 Runner。
- **不引入新版本依赖**：`trpc-a2a-go v0.2.5` 已在 `go.mod` 间接依赖，本次仅将其提升为直接依赖。
- **A2A v0.2.2 规范合规**：`AgentCard.Skills` 必填，`AgentSkill.ID/Name/Tags` 必填。

## Goals / Non-Goals

**Goals**

- 在 agent-server 提供标准 A2A v0.2.2 入口（JSON-RPC `message/send` / `message/stream` / `tasks/get` / `tasks/cancel` / `tasks/resubscribe`）。
- 暴露符合 A2A 规范的 AgentCard（`/.well-known/agent.json`、`/.well-known/agent-card.json`），描述对外可用能力。
- 支持基于 `contextId` 的连续多轮对话（同一 `contextId` 命中同一 session/threadId）。
- 新增 MCP ToolSet `type: "internal"`：仅注入 `X-Bkapi-User-Name`，用于 LLM 内网直连未来上线的 api-server 内置 HCM MCP。
- A2A 默认关闭，启用后与 AG-UI 完全并行、零干扰、共用 Runner。

**Non-Goals**

- 不实现 api-server 北向 MCP ingress（MCP ↔ A2A 转换），属于 api-server 子需求。
- 不实现 api-server 内置 HCM MCP Server（南向工具 MCP），属于 api-server 子需求。
- 不修改 AG-UI 任何代码路径、中间件、配置默认值。
- 不抽公共「Runner 中间事件模型」（AG-UI translator 与 A2A 事件来源都是 Runner，但本次保持各自独立翻译，待两者翻译逻辑稳定后再评估重构）。
- 不为 A2A 单独实现「Session 持久化层」：复用现有 `aguiSessionSvc` 即可。
- 不实现 A2A AgentCard 签名（`signatures` 字段）与扩展卡（`SupportsAuthenticatedExtendedCard`）。

## Decisions

### D1：A2A Server 框架选型 —— 使用 `trpc-agent-go/server/a2a`，而非直接对接 `trpc-a2a-go`

**决定**：通过 `trpc.group/trpc-go/trpc-agent-go/server/a2a`（下文记为 `agoa2a`）构建 A2A Server，传入 `runner.Runner` 即可。底层 `trpc-a2a-go` 的 `taskmanager.MessageProcessor`、`MemoryTaskManager`、`A2AServer` 由 `agoa2a` 自动封装。

**理由**：
1. `agoa2a.New(WithRunner, WithAgentCard, WithExtraA2AOptions)` 已实现「A2A `message/stream` → `runner.Runner.Run` → A2A event 流」的完整桥接，无需手写 `MessageProcessor`、事件 translator。
2. 与 commit 7be6e551 保持一致，降低与上游版本演进的偏离。
3. trpc-agent-go 是 HCM agent-server 已有的核心依赖（AG-UI 也基于它），不引入新的抽象层。

**备选方案**：直接使用 `trpc-a2a-go` 的 `taskmanager.NewMemoryTaskManager(processor)` 自行实现 `MessageProcessor` 桥接到 `runner.Runner`。**放弃原因**：需要手写 ~200 行的事件翻译代码（Runner 事件 → A2A artifact/status event），且与 AG-UI 已有 translator 结构冗余；首期不必要。

### D2：AgentCard 来源 —— 静态配置驱动，不依赖 SkillManager.Repository

**决定**：
- 在 `pkg/cc/service.go` 新增 `A2ACardConfig` / `A2ACardSkillConfig`，将 AgentCard 的 Name / Description / Version / URL / Skills 全部由配置文件声明；
- 启动时**一次性构建** AgentCard 后挂到 `agoa2a.WithAgentCard()`，运行期不变；
- 配置未提供 `card.skills` 时使用一份内置最小化 skill 占位（id=`hcm-agent`，tags=`["hcm","agent"]`），保证 A2A v0.2.2 合规。

**理由**：
1. A2A 协议层不应耦合 HCM 内部 skill 同步状态（`SkillManager.Repository` 是 BKAIDev skill sync 的产物，与 A2A 对外暴露能力是不同的语义）。
2. AgentCard 是对外契约，由运维/产品在配置中显式声明更可控；动态生成会因 skill sync 失败导致 AgentCard 内容抖动。
3. 解耦后 `cmd/agent-server/logics/runtime.go` 无需新增 `SkillManager()` 暴露方法，减少对 Runtime 的污染。

**备选方案**：commit 7be6e551 中的 `cardBuilder.buildSkills()` 从 `skillpkg.Repository.Summaries()` 拉取。**放弃原因**：见上述理由 1、2。

### D3：contextId → threadId 映射策略 —— 直接以 contextId 作为 threadId

**决定**：A2A 框架 `agoa2a` 在派发 Runner 任务时，将 A2A `message.contextId` 作为 `userId/sessionId` 上下文之一传给 Runner；本设计**直接使用 `contextId` 作为 `threadId`**（保持字符串原值），同一 `contextId` 命中同一 session/thread，自然实现连续对话。

**理由**：
1. A2A `contextId` 默认格式 `ctx-<uuid>`，已是不重复的字符串，可直接作为 thread 标识符；
2. 不需要像 AG-UI 那样查 DB（`sessionCode → threadId` 是 HCM 自定义的解析），A2A 链路没有 sessionCode 概念；
3. 复用 `runTime.SessionSvc()`，由 trpc-agent-go runner 自动按 threadId 加载/写回历史。

**备选方案**：
- **方案 A**：维护 `contextId ↔ threadId` 映射表（DB 落库）。**放弃原因**：A2A 协议已明确 `contextId` 是会话级长生命周期标识，无需再做一次映射；增加复杂度且引入 DB 写入路径。
- **方案 B**：每次请求生成新 `threadId`，仅靠 A2A history 维持上下文。**放弃原因**：违背 A2A `contextId` 设计语义，无法跨进程恢复会话。

**实现要点**：
- 通过 `agoa2a.WithSessionIDResolver` 或等价的 RunOption（如 trpc-agent-go A2A server 提供的 hook）将 `contextId` 直接注入 Runner 的 `threadId`；具体 hook 由 `agoa2a` 版本能力决定，若 hook 不可用，则在 A2A handler 之外的中间件层将 `contextId` 提取后写入 request context，再通过自定义 `RunOptionResolver` 注入。
- 首轮请求若 body 中无 `contextId`，由 trpc-a2a-go 框架自动生成（`protocol.GenerateContextID()`），并在 A2A 响应中回传，由调用方（api-server ingress）负责传回客户端。

### D4：复用 `runTime.AGUIRunner` —— 不新建 Runner 实例

**决定**：A2A `agoa2a.WithRunner(rt.AGUIRunner)` 直接传入 AG-UI 同一 Runner 实例。

**理由**：Runner 已封装 Agent / Model / ToolSet / Session / Memory；新建会导致重复初始化（特别是 MCP ToolSet 的连接），且 session 数据隔离会破坏「同一用户在 AG-UI 与 A2A 间的会话连续性预期」（即使本期未使用此特性，也保留可能性）。

**风险**：AG-UI 与 A2A 共用 ToolSet → 若未来引入 A2A 专属 toolset，需要在 Runner 层做请求级 toolset 选择（通过 `RunOption` 注入 `tool.FilterFunc`）。本期无此需求。

### D5：路径策略 —— `basePath: "/api/v1/agent"`，与 AG-UI 同前缀

**决定**：

| 端点 | 最终路径 |
|---|---|
| A2A JSON-RPC 入口 | `/api/v1/agent/a2a` |
| AgentCard（标准） | `/api/v1/agent/.well-known/agent.json` |
| AgentCard（兼容别名） | `/api/v1/agent/.well-known/agent-card.json` |

通过 `agoa2a.WithExtraA2AOptions(a2aserver.WithBasePath("/api/v1/agent"))` 实现。

**理由**：
1. 与现有 `/api/v1/agent/agui`、`/api/v1/agent/cancel`、`/api/v1/agent/history`、`/api/v1/agent/memory` 保持同一前缀，运维网关配置更整齐；
2. 与 iWiki 文档示例路径完全一致（`http://{devhk_host}:19057/agent/a2a` 经网关前缀映射后即此路径）；
3. 双 AgentCard 路径：A2A 0.2.x 规范使用 `/.well-known/agent-card.json`，部分客户端实现仍按 0.1.x 的 `/agent.json` 寻址，同时挂载兼顾兼容。

**备选方案**：使用 `agoa2a` 库的默认 `/a2a/` 前缀（commit 7be6e551 当前实现）。**放弃原因**：与 AG-UI 不同前缀，运维配置零散。

### D6：中间件链编排 —— 复用 AG-UI 既有中间件 + 新增来源校验

**决定**：A2A Handler 套用如下中间件链（从外到内）：

```
mcpCallerOriginMiddleware           ← 仅允许 api-server 调用（X-Bkhcm-Caller-Source=api-server）
  → agentAuthMiddleware             ← 复用 AGUI 的 IAM 校验（AgentAssistant.Find）
    → bkapiContextMiddleware        ← 复用 AGUI 的 BK 用户信息注入（bk_username 写入 context）
      → readinessMiddleware         ← 复用 AGUI 的 skill/prompt 同步就绪门控
        → agoa2a Handler            ← A2A 协议处理
```

**理由**：
- `bkapiContextMiddleware` 已支持 `X-Bkapi-User-Name` 注入，无需修改；本期不读 `X-Bk-Ticket`（A2A 链路无 ticket），现有 middleware 行为对此场景兼容（无 ticket 时跳过 ticket 注入）。
- `agentAuthMiddleware` 现成可用，鉴权对象同 AGUI（`AgentAssistant`）。
- 新增的 `mcpCallerOriginMiddleware` 校验 `X-Bkhcm-Caller-Source` header 等于 `api-server`，防止外部直连绕过 ingress。该 header 在 ingress 实现完成前**先以 warn 日志记录、不拦截**，配置开关 `a2a.enforceCallerOrigin` 控制是否强制拦截（默认 false，待 ingress 上线后开启）。

**备选方案**：mTLS 校验调用方身份。**放弃原因**：HCM 内部微服务未启用 mTLS，单独为 A2A 启用代价过高；header 校验 + 内网网络隔离已足够。

### D7：MCP ToolSet `type: "internal"` 设计

**决定**：在 `cmd/agent-server/logics/tool/tool.go` 的 `buildOneMCPToolSet` 中新增分支：

```go
if strings.EqualFold(strings.TrimSpace(cfg.Type), constant.MCPTypeInternal) {
    opts = append(opts, mcp.WithMCPOptions(
        trpcmcp.WithHTTPBeforeRequest(func(ctx context.Context, req *http.Request) error {
            if username := auth.BKUsernameFromContext(ctx); username != "" {
                req.Header.Set(constant.UserKey, username) // X-Bkapi-User-Name
            }
            return nil
        }),
    ))
}
```

**与 `type: "bkaidev"` 的隔离**：
- `bkaidev` 类型注入 `X-Bkapi-Authorization`（含 appCode + appSecret + bk_ticket）；
- `internal` 类型**不**注入 `X-Bkapi-Authorization`，**不**读取 `bk_ticket`；
- 配置校验时（`pkg/cc/service.go`）允许 `type: "internal"` 在没有 `tools.bkAIDev` 配置时通过。

**理由**：内置 HCM MCP 是内部微服务调用，依靠内网隔离 + `X-Bkapi-User-Name` 信任，无需走网关 inner-jwt 校验；注入 token 反而引入泄漏面。

**安全约束**（在 D6 中间件层落实）：A2A 链路的 `bk_username` 来自 api-server ingress 解析的网关 JWT；AGUI 链路不会切到 `internal` toolset（AGUI yaml 中 toolset type 保持 `bkaidev`），互不干扰。

### D8：默认关闭 —— `a2a.enable: false`

**决定**：`A2ASetting.Enable` 默认 `false`。`mountA2A` 第一行检查 `if !cfg.Enable { return nil }`，未启用时所有 A2A 代码路径不生效。

**理由**：本期同时上线 agent-server 改动与 api-server 改动不现实（属于不同子需求 + 不同发版节奏）；先合并代码、保持关闭，待 api-server ingress 与内置 MCP 就绪后再灰度打开。

### D9：暂不抽公共「Runner 中间事件模型」

**决定**：本次不抽 AG-UI 与 A2A 共用的事件中间模型。AG-UI 继续使用 `aguievent.NewCustomTranslator`，A2A 由 `agoa2a` 内部翻译。

**理由**：抽公共抽象需要同时改动 AG-UI 与 A2A 两侧，**会触碰现有 AG-UI 链路**，违背「不影响 AGUI」约束。待 A2A 链路稳定运行 + AG-UI 上线后续重构需求时再统一处理。

### D10：A2A AgentCard 双路径兼容

**决定**：同时挂载 `/.well-known/agent.json`（A2A 早期版本）与 `/.well-known/agent-card.json`（A2A v0.2.2+）。两个路径返回同一份 AgentCard。

**理由**：OpenClaw 客户端版本不一，双路径降低客户端发现 AgentCard 失败的风险。`agoa2a` 库已提供 `WithAgentCardHandler` 注入自定义 handler，可同时响应两个路径。

## Risks / Trade-offs

| 风险 | 等级 | 缓解措施 |
|---|---|---|
| 共用 Runner 导致 AG-UI 与 A2A 行为相互影响 | 中 | 中间件层完全独立，仅在 Runner 内部共享；Runner 本身是 stateless 派发器，session 隔离由 threadId 保证 |
| `contextId` 直接作为 `threadId` 可能与 AG-UI 的 threadId 撞库 | 低 | A2A `contextId` 默认 `ctx-<uuid>`，AG-UI threadId 由 `sessionCode` 解析后是 DB 自增 ID / UUID，命名空间不冲突；若客户端传入自定义 `contextId`（如 `session-demo-0056`），仍是字符串隔离，概率可忽略 |
| A2A handler 异常（如 framework bug）影响整个 agent-server 进程 | 低 | A2A handler 与 AGUI handler 是独立 ServeMux 路径，框架内部 panic 由 net/http 默认 recover；启用前先在测试环境联调 |
| `enable: true` 但 `card.skills` 配置为空导致 AgentCard 校验失败 | 低 | 内置占位 skill 兜底（id=`hcm-agent`），保证 AgentCard 合规 |
| `mcpCallerOriginMiddleware` 默认不强制拦截 → 启用 A2A 后外部可绕过 api-server 直连 agent-server | 中 | 上线 api-server ingress 后立即将 `a2a.enforceCallerOrigin: true` 设为默认；网络层依靠 K8s NetworkPolicy / VPC 安全组限制外部直达 |
| A2A 与 AG-UI MCP ToolSet 共享 → 若 A2A 链路无 `bk_ticket`，AG-UI 的 `bkaidev` hook 触发 warn 日志噪音 | 低 | `bkaidev` hook 在无 ticket 时仅 warn + skip 注入，不阻断；A2A 链路 LLM 若切到 `internal` toolset 则完全不走 `bkaidev` 分支 |
| trpc-agent-go A2A 子模块 API 变更（v1.8.1 → 后续版本） | 中 | 锁定 `trpc-a2a-go v0.2.5` + `trpc-agent-go v1.8.1`；升级前在测试环境验证 A2A handler 兼容性 |

## Migration Plan

### 部署步骤

1. **配置准备**（不强制）
   - 已有 `agent_server.yaml` **无需任何调整**——`a2a.enable` 默认 false，启用前保持现状即可。
   - 上线 api-server ingress 后，编辑生产 yaml 增加：
     ```yaml
     a2a:
       enable: true
       basePath: "/api/v1/agent"
       enforceCallerOrigin: true
       card:
         name: "HCM Agent"
         description: "BlueKing Hybrid Cloud Management AI Agent"
         version: "1.0.0"
         url: "http://{api-server-public-url}/api/v1/agent"
         skills:
           - id: "hcm-agent"
             name: "HCM Agent"
             description: "云资源查询、变更辅助、故障排查、操作指引"
             tags: ["hcm", "agent"]
     ```

2. **代码发布**
   - 合入本变更后，agent-server 在所有环境（dev/test/staging/prod）以 `a2a.enable: false` 启动；
   - AG-UI 链路完全不受影响，无回归风险。

3. **联调灰度**（待 api-server ingress 就绪后）
   - 在 dev 环境先开启 `a2a.enable: true`，配合 api-server ingress 联调；
   - 在 test 环境用 curl 直发 A2A `message/stream` 验证 AgentCard、`message/send`、`message/stream`、`tasks/cancel`；
   - 验证 AG-UI 链路无回归（用现有 web-server 入口跑端到端测试）；
   - 灰度生产时 `enforceCallerOrigin: true` 强制开启。

### 回滚策略

- **代码回滚**：直接 revert 本变更 commit；由于 `a2a.enable` 默认 false，未启用环境零影响，已启用环境会丢失 A2A 入口（OpenClaw 接入暂停）但不影响 AG-UI 链路。
- **配置回滚**：将 `a2a.enable` 设为 false（或删除整个 `a2a` 段）后重启 agent-server 即可。
- **AG-UI 兜底**：本变更对 AG-UI 链路零修改，任何 A2A 相关故障均不会传导到 AG-UI；最差情况下可直接关闭 A2A 保留 AG-UI 服务用户。

## Open Questions

1. **`agoa2a` 注入 `contextId → threadId` 的 hook 形式**：需要在编码阶段确认 `trpc-agent-go/server/a2a` v1.8.1 是否提供 `WithSessionIDResolver` 或等价 hook；若不提供，则在中间件层提前读取 body 中的 `contextId` 写入 request context，再通过 `WithRunOptionResolver` 注入 `agent.WithRuntimeState(threadId=...)`。
2. **AgentCard.url 字段**：当 A2A 仅通过 api-server ingress 对外暴露时，AgentCard.url 应填 ingress 暴露地址还是 agent-server 内网地址？方案：填 ingress 地址（OpenClaw 看到的对外 URL），由配置 `card.url` 显式指定，不在代码中拼装。
3. **`enforceCallerOrigin` 灰度策略**：上线初期是否需要支持「白名单 IP/UA」作为 caller-source header 的补充？本期不实现，待 api-server ingress 上线后视实际情况决定。
4. **A2A `tasks/cancel` 行为**：trpc-agent-go A2A 子模块对 Runner cancel 的支持完整性需在编码阶段验证；若不支持，cancel 只能在 task 状态层标记，不会中断底层 LLM 调用——本期可接受（AG-UI cancel 也是软取消）。
