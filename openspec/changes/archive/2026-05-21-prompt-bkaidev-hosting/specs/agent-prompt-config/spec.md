## MODIFIED Requirements

### Requirement: AgentPromptConfig 支持 BKAIDev 远程 prompt 配置
`AgentPromptConfig` SHALL 新增 `BKAIDev *AgentPromptBKAIDevConfig` 字段。
`AgentPromptBKAIDevConfig` 包含以下字段：
- `Enabled bool`：是否启用 BKAIDev prompt 同步
- `ApiGateway`（inline）：复用 `cc.ApiGateway` 结构，包含 Endpoints、AppCode、AppSecret、User、TLS
- `SpaceID string`：BKAIDev 空间 ID
- `SyncInterval string`：同步间隔，默认 `"5m"`
- `SystemPromptID int`：system prompt 的 prompt_id，0 表示不配置
- `InstructionID int`：instruction 的 prompt_id，0 表示不配置
- `ManifestPath string`：manifest 文件路径，默认为同目录下的 `prompt-manifest.json`

`AgentPromptBKAIDevConfig` SHALL 提供 `TrySetDefault()`（填充默认 SyncInterval）和 `Validate()`（校验 ApiGateway 和 SpaceID）方法。
`BKAIDevSyncEnabled()` 方法返回 `c != nil && c.BKAIDev != nil && c.BKAIDev.Enabled`。

#### Scenario: BKAIDev 模式启用时 trySetDefault 跳过文件加载
- **WHEN** `AgentPromptConfig.BKAIDev.Enabled = true`
- **THEN** `trySetDefault()` 不加载 `systemPromptFile` / `instructionFile`，SystemPrompt 和 Instruction 字段保持为空字符串（由 Syncer 填充）

#### Scenario: 文件模式下行为与现在完全一致
- **WHEN** `AgentPromptConfig.BKAIDev` 为 nil 或 `Enabled=false`
- **THEN** `trySetDefault()` 行为与修改前完全相同，读取文件内容填充 SystemPrompt / Instruction

#### Scenario: 配置校验失败时服务启动失败
- **WHEN** `BKAIDev.Enabled=true` 但 `SpaceID` 为空或 ApiGateway 端点未配置
- **THEN** `Validate()` 返回错误，服务启动失败，输出明确错误信息

### Requirement: 新增 CronTaskSyncAgentPrompts 枚举值
`pkg/criteria/enumor/cron_task.go` SHALL 新增常量 `CronTaskSyncAgentPrompts CronTask = "sync_agent_prompts"`。

#### Scenario: Prompt cron 任务使用唯一的枚举 key
- **WHEN** 注册 prompt sync cron 任务
- **THEN** 任务以 `CronTaskSyncAgentPrompts` 为 key 存入 `service.tasks` map，不与 skill 任务冲突
