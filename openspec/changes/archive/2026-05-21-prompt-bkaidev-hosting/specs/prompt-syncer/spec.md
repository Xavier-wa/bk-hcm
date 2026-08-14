## ADDED Requirements

### Requirement: Prompt Syncer 首次同步完成后标记 PromptReady
`Syncer.InitialSync(ctx)` SHALL 在后台 goroutine 中执行，完成后调用 `readiness.MarkPromptReady()`。
若 BKAIDev 未配置（`enabled=false`），则在构建时直接调用 `readiness.MarkPromptReady()`，保持与 skill 对称的 readiness 机制。
`Readiness.NewReadiness()` 中的临时 `promptReady.Store(true)` SHALL 改为 `Store(false)`。

#### Scenario: BKAIDev 同步未启用时 promptReady 立即就绪
- **WHEN** 配置中 `prompt.bkaidev.enabled=false`（或未配置 bkaidev 块）
- **THEN** 进程启动后 `promptReady` 立即为 true，不等待远程同步

#### Scenario: 首次同步成功后 promptReady 变为 true
- **WHEN** BKAIDev 同步已启用，且首次 `InitialSync` 成功完成所有配置的 prompt 获取
- **THEN** `readiness.MarkPromptReady()` 被调用，`/api/v1/agent/readiness` 中 `prompt_ready=true`

#### Scenario: 首次同步失败时 promptReady 保持 false
- **WHEN** BKAIDev 接口返回错误，首次同步未成功
- **THEN** `MarkPromptReady` 不被调用，服务 readiness 检查失败

### Requirement: Prompt Syncer 以 content MD5 为版本进行增量同步
`SyncOnce` SHALL 对每个配置的 prompt_id 调用 `RetrievePrompt`，计算 content 的 MD5，
与 `manifest.Store` 中记录的 MD5 对比：若不同则更新 `PromptStore` 并写入 manifest；若相同则跳过。
manifest 文件路径由 `AgentPromptBKAIDevConfig.ManifestPath` 配置，key 使用角色名（"system_prompt" / "instruction"）。

#### Scenario: content 未变化时跳过更新
- **WHEN** 远程 prompt content 的 MD5 与 manifest 中记录的 MD5 相同
- **THEN** 不调用 `PromptStore.Update()`，不写 manifest，日志级别为 debug

#### Scenario: content 发生变化时更新 PromptStore 和 manifest
- **WHEN** 远程 prompt content 的 MD5 与 manifest 中记录的 MD5 不同
- **THEN** 调用 `PromptStore.Update()` 写入新内容，manifest 更新 MD5 和 updated_at，记录 Info 日志

#### Scenario: 仅配置了 systemPromptID 时不同步 instruction
- **WHEN** `instructionID = 0`（未配置）
- **THEN** `SyncOnce` 跳过 instruction 同步，仅处理 system prompt

### Requirement: Prompt Syncer 注册与 skill 对称的 cron 定时任务
`SyncCronTask` SHALL 实现 `core.Task` 接口（Name、Next、Do、GetURL），
注册到相同的 cron 框架（`cron.Register`），cron 任务名为 `CronTaskSyncAgentPrompts`。
手动触发 URL 为 `/prompts/sync`。

#### Scenario: cron 任务按配置间隔触发增量同步
- **WHEN** 距离上次同步已超过 `syncInterval` 时长
- **THEN** `Do` 被调用，触发 `SyncOnce`

#### Scenario: 手动触发同步接口
- **WHEN** 对 `/prompts/sync` 发起 POST 请求
- **THEN** 立即触发一次 `SyncOnce`，响应 HTTP 200

### Requirement: Prompt Manifest 持久化到本地文件
Prompt manifest 文件 SHALL 独立于 skill manifest，使用 `manifest.Store`（现有包）持久化，
key 为角色名（`"system_prompt"` / `"instruction"`），Entry 中仅使用 `MD5` 和 `UpdatedAt` 字段（Version 留空）。

#### Scenario: 进程重启后加载上次同步的 MD5
- **WHEN** 进程重启，manifest 文件已存在
- **THEN** `manifest.Store.Load()` 读取上次的 MD5，跳过内容未变化的 prompt

#### Scenario: 进程首次启动且无 manifest 文件时全量同步
- **WHEN** manifest 文件不存在（首次部署）
- **THEN** 所有配置的 prompt 均被拉取并写入 PromptStore 和 manifest
