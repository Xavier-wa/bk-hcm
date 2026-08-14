## 1. 配置与常量

- [x] 1.1 在 `pkg/cc/service.go` 中新增 `A2ACardSkillConfig`（含 `id`、`name`、`description`、`tags`、`examples`、`inputModes`、`outputModes` 字段）
- [x] 1.2 在 `pkg/cc/service.go` 中新增 `A2ACardConfig`（含 `name`、`description`、`version`、`url`、`skills []A2ACardSkillConfig`）
- [x] 1.3 在 `pkg/cc/service.go` 中新增 `A2ASetting`（含 `enable`、`basePath`、`enforceCallerOrigin`、`card A2ACardConfig`）
- [x] 1.4 在 `AgentServerSetting` 中新增 `A2A A2ASetting` 字段并补齐 yaml tag、注释
- [x] 1.5 在 `AgentServerSetting.trySetDefault()` 中为 A2A 字段补齐默认值（`enable: false`、`basePath: ""`，由 mount 层使用 `/api/v1/agent` 默认值）
- [x] 1.6 在 `AgentServerSetting.Validate()` 中：当 `A2A.Enable=true` 时校验 `basePath` 合法性，并保证至少有兜底 skill
- [x] 1.7 在 `pkg/criteria/constant/aiagent.go`（或同目录已有 MCP 常量文件）新增 `MCPTypeInternal = "internal"` 常量，注释说明用途
- [x] 1.8 在 cc 层 MCP ToolSet 校验中新增 `type: "internal"` 分支：允许通过且不要求 `tools.bkAIDev`；拒绝未知 `type` 时错误信息包含非法值

## 2. 依赖调整

- [x] 2.1 编辑 `go.mod`：将 `trpc.group/trpc-go/trpc-a2a-go v0.2.5` 从 `// indirect` 块移到直接依赖块（不升级版本）
- [x] 2.2 运行 `go mod tidy` 验证 `go.sum` 不发生预期外变化
- [x] 2.3 运行 `go build ./...` 确认编译通过

## 3. MCP ToolSet `type: "internal"` 支持

- [x] 3.1 在 `cmd/agent-server/logics/tool/tool.go` 的 `buildOneMCPToolSet` 中新增 `if strings.EqualFold(cfg.Type, constant.MCPTypeInternal)` 分支
- [x] 3.2 该分支通过 `mcp.WithMCPOptions(trpcmcp.WithHTTPBeforeRequest(...))` 注入 hook：从 `auth.BKUsernameFromContext(ctx)` 读取 username，非空时设置 `req.Header[constant.UserKey]`
- [x] 3.3 该分支 SHALL NOT 读取 `cc.AgentServer().Tools.BKAIDev`，SHALL NOT 注入 `X-Bkapi-Authorization`，SHALL NOT 读取 `bk_ticket`
- [x] 3.4 启动日志输出 `name`、`type=internal`、`transport`、`serverUrl`，与 bkaidev 类型的日志风格保持一致
- [x] 3.5 在 `cmd/agent-server/logics/tool/tool_test.go` 中新增单元测试：覆盖「`type:internal` 注入 username」「无 username 时不注入」「不注入 X-Bkapi-Authorization」三个场景

## 4. A2A 服务包：基础骨架

- [x] 4.1 新建目录 `cmd/agent-server/service/a2a/`，添加 `doc.go` 或 `service.go` 头部注释，包名 `a2a`
- [x] 4.2 在 `cmd/agent-server/service/a2a/service.go` 中实现 `func Mount(mux *http.ServeMux, rt *logics.Runtime, cfg cc.A2ASetting) error`
- [x] 4.3 `Mount` 第一行检查 `if !cfg.Enable { return nil }`，未启用时直接返回 nil 不挂载任何端点
- [x] 4.4 `Mount` 内部调用 `agoa2a.New(WithRunner(rt.AGUIRunner), WithAgentCard(card), WithExtraA2AOptions(...))` 构建 A2A Server
- [x] 4.5 通过 `a2aserver.WithBasePath` 设置 `basePath`，未配置时默认 `/api/v1/agent`
- [x] 4.6 输出 info 日志记录挂载完成（含 basePath、card.Name、skills 数量）

## 5. AgentCard 静态构建

- [x] 5.1 在 `cmd/agent-server/service/a2a/card.go` 中实现 `buildAgentCard(cfg cc.A2ASetting) a2aserver.AgentCard`
- [x] 5.2 实现 `cardName/cardDescription/cardVersion` 默认值兜底函数（值在 design.md D2 中定义）
- [x] 5.3 实现 `buildSkills(cfg cc.A2ACardConfig) []a2aserver.AgentSkill`：当 `cfg.Skills` 为空时返回内置占位 skill（id=`hcm-agent`、name=`HCM Agent`、tags=`["hcm","agent"]`）
- [x] 5.4 AgentCard 字段填充：`Capabilities.Streaming = ValToPtr(true)`、`DefaultInputModes/OutputModes = []string{"text"}`
- [x] 5.5 单元测试 `card_test.go`：覆盖「完整配置」「未配 name 用默认」「未配 skills 用占位」「skills 中包含 ID/Name/Tags 必填字段」

## 6. A2A 路由挂载

- [x] 6.1 在 `service.go` 中实现 `registerHandlers(mux, handler, basePath)`：挂载 `<basePath>/a2a`（无尾斜杠）作为 JSON-RPC 入口
- [x] 6.2 同时挂载 `<basePath>/.well-known/agent-card.json` 与 `<basePath>/.well-known/agent.json` 两个 AgentCard 路径（同一份内容）
- [x] 6.3 校验路径策略：`basePath` 为空时使用 `/api/v1/agent`；尾斜杠规范化（统一不带尾斜杠拼接）
- [x] 6.4 单元测试 `service_test.go`：通过 `httptest.NewServer` 验证三个端点都返回非 404，且 AgentCard 双路径返回相同 JSON

## 7. 中间件链编排

- [x] 7.1 在 `cmd/agent-server/service/service.go` 中新增 `mcpCallerOriginMiddleware(enforce bool) func(http.Handler) http.Handler`：校验 `X-Bkhcm-Caller-Source == "api-server"`
- [x] 7.2 强制模式下不匹配返回 HTTP 403；非强制模式仅 warn 日志（含 `X-Forwarded-For`/`RemoteAddr`、`X-Bkapi-Request-Id`），不拦截
- [x] 7.3 在 `service.go` 新增 `mountA2A(mux)` 方法（与 `mountAGUI(mux)` 并列），内部调用 `svca2a.Mount` 并把返回的 handler 套上中间件链
- [x] 7.4 中间件包装顺序（从外到内）：`mcpCallerOriginMiddleware → agentAuthMiddleware → bkapiContextMiddleware → readinessMiddleware → A2A handler`
- [x] 7.5 在 `ListenAndServeRest()` 中 `mountAGUI(root)` 之后调用 `mountA2A(root)`，错误返回链路与 mountAGUI 一致
- [x] 7.6 设计实现：中间件需同时包裹 JSON-RPC 路径与两个 AgentCard 路径（确保 AgentCard 也受 readiness/auth 保护）

## 8. contextId → threadId 映射

- [x] 8.1 调研 `trpc-agent-go v1.8.1` 的 `server/a2a` 包是否提供 sessionID/threadID 解析 hook（Open Question 1）
- [x] 8.2 若框架提供 hook：直接通过 `agoa2a.WithExtraA2AOptions(...)` 或对应 `With...Resolver` 把 `contextId` 注入为 `threadId`
- [x] 8.3 若框架不提供 hook：在 A2A handler 之前增加一层 `contextIdMiddleware`，从 body 读取 `params.message.contextId`、写入 request context，再通过 RunOptionResolver 注入 `agent.WithRuntimeState(threadId=contextId)`
- [x] 8.4 单元/集成测试：同一 `contextId` 两轮请求 Runner 收到的 `threadId` 相同；不同 `contextId` 收到的 `threadId` 不同

## 9. 配置示例与文档

- [x] 9.1 在 `cmd/agent-server/etc/agent_server.yaml` 末尾追加 `a2a:` 配置段示例（默认 `enable: false`，注释说明各字段用法、含一个 `skills` 示例条目）
- [x] 9.2 在 `cmd/agent-server/etc/agent_server.yaml` 的 `tools.mcp[]` 注释中追加 `type: "internal"` 的示例条目（注释形式，不开启）
- [x] 9.3 在变更目录或 README 中追加最小可用启动配置示例（同时启用 a2a + internal toolset 的最简 yaml 片段）

## 10. AG-UI 链路回归验证

- [x] 10.1 静态审查：`cmd/agent-server/service/service.go` 中 `mountAGUI` 及其调用的方法（`sessionCodeMiddleware`/`agentAuthMiddleware`/`bkapiContextMiddleware`/`readinessMiddleware`）SHALL 一行未改
- [x] 10.2 静态审查：`cmd/agent-server/logics/tool/tool.go` 中 `bkaidev` 类型分支 SHALL 一行未改
- [x] 10.3 静态审查：`cmd/agent-server/logics/runtime.go` 在本次变更中 SHALL 一行未改
- [x] 10.4 在本地或测试环境跑通既有 AG-UI 端到端用例（`/api/v1/agent/agui` + `/cancel` + `/history`），确认无回归
- [x] 10.5 验证 `a2a.enable: false`（默认值）启动时，agent-server 日志中不包含 "a2a server mounted" 类记录

## 11. A2A 端到端联调（基于本期不依赖 api-server 的可独立验证项）

- [x] 11.1 在测试环境（dev/test）配置 `a2a.enable: true` 并启动 agent-server
- [x] 11.2 用 `curl` 请求 `GET /api/v1/agent/.well-known/agent-card.json`，校验返回 200 + 符合 A2A v0.2.2 的 JSON（skills 非空、capabilities.streaming=true）
- [x] 11.3 用 `curl -N` 发送 `POST /api/v1/agent/a2a` 的 `message/stream` 请求（携带 `X-Bkapi-User-Name` + `X-Bkhcm-Caller-Source: api-server`），校验 SSE 流正常返回 A2A 事件
- [x] 11.4 连续两轮请求使用同一 `contextId`，校验第二轮 LLM 能感知第一轮上下文（连续对话生效）
- [x] 11.5 校验 `enforceCallerOrigin: true` 时缺失 `X-Bkhcm-Caller-Source` 返回 403；`false` 时正常处理 + warn 日志
- [x] 11.6 校验关闭 `a2a.enable` 后所有 A2A 端点返回 404，AG-UI 端点行为完全不变

## 12. 验收

- [x] 12.1 运行 `go vet ./...` 与 `golangci-lint run`（如项目配置）确认无新增 lint
- [x] 12.2 运行 `make test`（或对应单元测试目标）确认全部通过
- [x] 12.3 运行 `openspec validate agent-server-a2a-protocol --strict` 通过
- [x] 12.4 PR 描述中引用本变更 ID（`agent-server-a2a-protocol`），列出对 AG-UI 的零修改证明（diff 仅集中在新增包/配置/常量）
