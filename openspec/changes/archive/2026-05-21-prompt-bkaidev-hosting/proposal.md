## Why

Agent-server 的 system prompt 和 instruction 目前通过本地文件路径加载，无法在运行时动态更新，每次 prompt 变更都需要重新部署服务。通过将 prompt 托管到 BKAIDev 平台，可以实现 prompt 的集中管理和热更新，与已有的 skill 托管机制保持一致。

## What Changes

- **扩展 BKAIDev client**：在 `pkg/thirdparty/api-gateway/bkaidev` 中新增 `ListPrompts` 和 `RetrievePrompt` 两个接口调用，对应 `list_app_v1_prompts` 和 `retrieve_app_v1_prompts`
- **新增 Prompt 配置**：在 `AgentPromptConfig` 中新增 `BKAIDev` 配置块，支持通过 `prompt_id` 指定 system prompt 和 instruction 的来源；原有的 `systemPromptFile` / `instructionFile` 文件配置保留，两种方式互斥
- **新增 Prompt Store**：实现线程安全的 `PromptStore`，持有 system prompt 和 instruction 的当前内容及其 MD5 版本，支持原子更新
- **新增 Prompt Syncer**：实现定时同步逻辑，按配置的 `prompt_id` 调用 `retrieve_app_v1_prompts`，以 content MD5 作为版本标识，与本地 manifest 对比后按需更新，并注册 cron 任务
- **Readiness 联动**：首次同步完成后调用 `readiness.MarkPromptReady()`，移除当前的 `promptReady = true` 临时初始化；若 BKAIDev 未配置则直接就绪（保持现有行为）
- **BeforeModel 回调注入**：为 GraphAgent 和 LLMAgent 均添加 `MakePromptReplaceCallback`，在每次 LLM 调用前将 `msgs[0]`（system message）替换为 `PromptStore` 中的最新内容，仅在版本变化时触发实际替换
- **新增枚举值**：在 `pkg/criteria/enumor/cron_task.go` 中增加 `CronTaskSyncAgentPrompts`

## Capabilities

### New Capabilities

- `bkaidev-prompt-client`: BKAIDev prompt API 调用封装（ListPrompts / RetrievePrompt 接口类型定义与实现）
- `prompt-store`: 线程安全 PromptStore，持有并原子更新 system prompt / instruction 内容
- `prompt-syncer`: 基于 prompt_id + MD5 版本管理的首次同步与定时增量同步，manifest 持久化
- `prompt-replace-callback`: BeforeModel 回调，在每次 LLM 调用前用 PromptStore 最新内容替换系统消息

### Modified Capabilities

- `agent-prompt-config`: `AgentPromptConfig` 新增 `BKAIDev` 子配置块（prompt entries），同时 `trySetDefault` 跳过 BKAIDev 模式下的文件加载逻辑

## Impact

- `pkg/thirdparty/api-gateway/bkaidev/`：新增 prompt 相关类型和 Client 方法
- `pkg/cc/service.go`：`AgentPromptConfig` 新增字段，`trySetDefault` 调整
- `pkg/criteria/enumor/cron_task.go`：新增 `CronTaskSyncAgentPrompts`
- `cmd/agent-server/logics/prompt/`（新目录）：PromptStore、Syncer、BeforeModel callback
- `cmd/agent-server/service/prompt/`（新目录）：HTTP 手动触发同步接口（对齐 skill/sync.go 模式）
- `cmd/agent-server/logics/runtime.go`：集成 PromptManager，注册 BeforeModel callback
- `cmd/agent-server/service/service.go`：注册 prompt cron 任务
- `cmd/agent-server/logics/agent/agent_llm.go`、`graph_build.go`：注入 prompt replace callback
- 无 DB/API 外部接口变更，无 breaking change
