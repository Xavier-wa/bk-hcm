## Why

为了支持 OpenClaw 等外部 Agent 通过标准 A2A（Agent2Agent）协议直连 HCM 智能助手并实现连续多轮对话，需要在 `agent-server` 中新增 A2A 协议入口。同时，agent-server 底层 LLM 调用 HCM 工具时，未来将从「经蓝鲸普通网关公开 MCP」改为「直连 api-server 内置 HCM MCP（内部微服务调用）」，因此 MCP ToolSet 也需要新增支持仅以 `bk_username` 作为身份的内部调用类型。

整个改造必须与现有 AG-UI 协议链路（浏览器 → web-server → agent-server，带 `bk_ticket`）完全隔离，不得影响其行为。

## What Changes

- **新增 A2A 协议 HTTP 入口**：在 `agent-server` 中挂载 A2A JSON-RPC 端点 `/api/v1/agent/a2a` 以及 AgentCard 元数据端点 `/api/v1/agent/.well-known/agent.json`、`/api/v1/agent/.well-known/agent-card.json`（与现有 `/api/v1/agent/agui` 保持同一前缀）。
- **新增 A2A Server 构建**：基于 `trpc.group/trpc-go/trpc-agent-go/server/a2a`（底层为 `trpc-a2a-go v0.2.5`）构建 A2A Server，**复用现有 `runner.Runner`（`runTime.AGUIRunner`）**，A2A 与 AG-UI 共用同一底层 Agent / Model / ToolSet / Session / Memory。
- **新增 AgentCard 元数据（静态描述）**：根据**配置文件中的静态描述**构建 AgentCard（包含 Name / Description / Version / URL / Streaming 能力声明 / Skills 列表）。AgentCard 在 A2A 启动时一次性构建后保持不变，不依赖 agent-server 内部的 SkillManager.Repository（避免 A2A 协议层耦合 HCM 内部 skill 同步状态）。当配置未提供 `skills` 时使用一份内置的最小化 skill 占位（A2A v0.2.2 规范要求 `Skills` 字段必填）。
- **新增 contextId → threadId 映射**：A2A 标准协议的 `contextId` 作为连续对话载体，agent-server 将其作为 `threadId` 种子注入 Runner，保证同一 `contextId` 命中同一 session，从而实现多轮连续对话。
- **新增 MCP ToolSet `type: "internal"` 支持**：在 `cmd/agent-server/logics/tool/tool.go` 中新增一种 MCP ToolSet 类型，仅从请求 context 中读取 `bk_username` 注入 `X-Bkapi-User-Name` header，**不注入** `X-Bkapi-Authorization` / `bk_ticket` / `access_token`；用于 agent-server LLM 通过内网直连 api-server 内置 HCM MCP 的内部微服务调用。
- **新增 A2A 配置结构**：在 `pkg/cc/service.go` 中新增 `A2ASetting` / `A2ACardConfig` / `A2ACardSkillConfig` 配置类型，并挂到 `AgentServerSetting.A2A`；其中 `A2ACardConfig.Skills` 用于声明对外暴露的 AgentSkill 列表（ID / Name / Description / Tags 等）；默认 `enable: false`，按需启用。
- **依赖提升**：将 `trpc.group/trpc-go/trpc-a2a-go v0.2.5` 从间接依赖提升为直接依赖（项目已存在该版本，仅 `go.mod` 中字段调整）。

**不在本次范围**（属于整体方案的其它子需求）：
- api-server 北向 MCP ingress（MCP ↔ A2A 协议转换）
- api-server 内置 HCM MCP Server（南向工具 MCP）
- 蓝鲸网关资源配置
- OpenClaw 接入 SKILL 文档

## Capabilities

### New Capabilities

- `agent-a2a-protocol`: 定义 agent-server 对外提供 A2A 协议入口的能力，包括 A2A JSON-RPC 与 AgentCard 端点的挂载、A2A Server 构建、AgentCard 静态描述来源（cc 配置驱动）、连续对话载体（contextId 映射）、与 AG-UI 协议链路的隔离规则。
- `agent-mcp-internal-toolset`: 定义 agent-server MCP ToolSet 中 `type: "internal"` 类型的行为契约，包括身份注入策略、与 `type: "bkaidev"` 的隔离规则、配置项约束。

### Modified Capabilities

<!-- 当前 openspec/specs/ 下无与 agent-server / a2a / mcp toolset 相关的已有 spec，本次为纯新增能力，无需修改已有 spec。 -->

## Impact

- **代码改动**（仅 `agent-server` + 一个 cc 配置文件 + 常量文件）：
  - `pkg/cc/service.go`：新增 `A2ASetting` / `A2ACardConfig` / `A2ACardSkillConfig` 类型，并挂到 `AgentServerSetting.A2A` 字段；`A2ACardConfig.Skills` 字段承载对外 AgentSkill 静态描述。
  - `pkg/criteria/constant/aiagent.go`（或同目录已有 MCP 常量文件）：新增 `MCPTypeInternal` 常量。
  - `cmd/agent-server/logics/tool/tool.go`：在 `buildOneMCPToolSet` 中新增 `type: "internal"` 分支，注入 `bk_username` hook。
  - `cmd/agent-server/service/a2a/`（**新增包**）：A2A Mount / AgentCard 静态构建 / A2A → Runner 桥接 / contextId → threadId 映射。
  - `cmd/agent-server/service/service.go`：在 `ListenAndServeRest()` 中新增 `mountA2A` 调用，与 `mountAGUI` 并列。
  - `cmd/agent-server/etc/agent_server.yaml`：新增 `a2a:` 配置段示例（默认关闭、含 `card.skills` 静态描述）、`tools.mcp[]` 新增 `type: "internal"` 的注释样例。

  > 注：本次**不**修改 `cmd/agent-server/logics/runtime.go`（commit 7be6e551 中暴露的 `SkillManager()` 方法在本方案中无需添加）；AgentCard 完全由 cc 配置驱动，与 Runtime 内部 SkillManager 解耦。
- **依赖**：`go.mod` 将 `trpc.group/trpc-go/trpc-a2a-go v0.2.5` 从间接依赖移到直接依赖块，**不引入新版本**。
- **API 变化**：新增对外端点 `/api/v1/agent/a2a` 与 AgentCard 路径；现有 `/api/v1/agent/agui`、`/cancel`、`/history`、`/sessions/...`、`/memory`、`/skill`、`/prompt`、`/readiness` 全部保持不变。
- **运行时行为**：默认 `a2a.enable: false`，不启用 A2A 时所有 A2A 相关代码路径无效，**对现有 AG-UI 链路零影响**；启用 A2A 时也只新增独立路径，不修改现有 AG-UI 中间件链与 toolset。
- **兼容性**：MCP ToolSet 默认 `type` 仍为 `bkaidev`，已有 `agent_server.yaml` 配置无需调整；`type: "internal"` 仅在新增的内置 MCP 联调阶段使用。
- **测试**：A2A 端点联调（curl `message/stream`）、AG-UI 回归（确保 sessionCode 流程、bk_ticket 注入流程、AGUI MCP 调用不受影响）。
