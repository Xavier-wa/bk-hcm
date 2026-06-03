## 1. BKAIDev Client 扩展（pkg/thirdparty/api-gateway/bkaidev/）

- [x] 1.1 新建 `prompt.go`，定义 `ListPromptsReq`、`RetrievePromptReq`、`PromptListItem`、`PromptDetail`、`ListPromptsResp` 类型及各自的 `Validate()` 和 `QueryParams()` 方法
- [x] 1.2 在 `bkaidev.go` 的 `Client` 接口中新增 `ListPrompts` 和 `RetrievePrompt` 方法签名
- [x] 1.3 在 `skillClient` 上实现 `ListPrompts` 和 `RetrievePrompt`，复用 `BkAIDevApiGatewayGet` 泛型函数

## 2. 配置扩展（pkg/cc/service.go & pkg/criteria/enumor/）

- [x] 2.1 在 `pkg/criteria/enumor/cron_task.go` 新增 `CronTaskSyncAgentPrompts CronTask = "sync_agent_prompts"`
- [x] 2.2 在 `pkg/cc/service.go` 新增 `AgentPromptBKAIDevConfig` 结构体（Enabled、ApiGateway inline、SpaceID、SyncInterval、SystemPromptID、InstructionID、ManifestPath），实现 `TrySetDefault()`、`Validate()`、`BKAIDevSyncEnabled()` 方法
- [x] 2.3 修改 `AgentPromptConfig`：新增 `BKAIDev *AgentPromptBKAIDevConfig` 字段，修改 `trySetDefault()` 在 BKAIDev 模式启用时跳过文件加载

## 3. PromptStore（cmd/agent-server/logics/prompt/store.go）

- [x] 3.1 新建 `cmd/agent-server/logics/prompt/` 包目录
- [x] 3.2 实现 `promptSnapshot` 结构体（SystemPrompt、SystemPromptMD5、Instruction、InstructionMD5）
- [x] 3.3 实现 `PromptStore`，使用 `atomic.Pointer[promptSnapshot]`，暴露 `SystemPrompt()`、`Instruction()`、`SystemPromptMD5()`、`InstructionMD5()`、`Update()` 方法
- [x] 3.4 实现 `NewPromptStore(sp, spMD5, inst, instMD5 string) *PromptStore` 构造函数

## 4. PromptReplaceCallback（cmd/agent-server/logics/prompt/callback.go）

- [x] 4.1 实现 `MakePromptReplaceCallback(store *PromptStore) model.BeforeModelCallbackStructured`：找到或插入 system message，内容为 `store.SystemPrompt()` + `\n\n` + `store.Instruction()`（instruction 为空时只用 systemPrompt）
- [x] 4.2 处理边界：`store.SystemPrompt()` 为空时 callback 直接返回不做修改

## 5. Prompt Syncer（cmd/agent-server/logics/prompt/syncer.go）

- [x] 5.1 定义 `PromptReadinessNotifier` 接口（`MarkPromptReady()`），与 skill 的 `ReadinessNotifier` 对称
- [x] 5.2 实现 `Syncer` 结构体（client、store、manifest、readiness、cfg 字段）
- [x] 5.3 实现 `NewSyncer(...)` 构造函数
- [x] 5.4 实现 `InitialSync(ctx context.Context)`：后台 goroutine 执行 `syncAll`，完成后调用 `MarkPromptReady()`；失败时仅记录错误日志，不 panic
- [x] 5.5 实现 `SyncOnce(kt *kit.Kit) error`：按配置 ID retrieve prompt，计算 content MD5，与 manifest 比对，变更时调用 `store.Update()` 并写入 manifest
- [x] 5.6 实现辅助函数 `contentMD5(content string) string`（使用 `crypto/md5` + hex 编码）

## 6. Prompt Cron 任务（cmd/agent-server/logics/prompt/cron.go）

- [x] 6.1 实现 `SyncCronTask` 结构体，实现 `core.Task` 接口的 `Name()`、`Next()`、`Do()`、`GetURL()` 方法
- [x] 6.2 实现 `RegisterSyncCronTask(syncer *Syncer) (core.Task, error)`：解析 SyncInterval，调用 `cron.Register`，模式与 `skill.RegisterSyncCronTask` 保持一致

## 7. Prompt Manager（cmd/agent-server/logics/prompt/manager.go）

- [x] 7.1 实现 `Manager` 结构体（Store *PromptStore、Syncer *Syncer）
- [x] 7.2 实现 `NewManager(readiness PromptReadinessNotifier) (*Manager, error)`：
  - 若 BKAIDev 未启用，用 cc 配置的文件内容构造 PromptStore，调用 `readiness.MarkPromptReady()`
  - 若 BKAIDev 启用，初始化 bkaidev client + manifest.Store + Syncer，启动 InitialSync

## 8. Readiness 修改（cmd/agent-server/logics/readiness.go）

- [x] 8.1 将 `NewReadiness()` 中的 `r.promptReady.Store(true)` 改为 `r.promptReady.Store(false)`，移除 TODO 注释

## 9. Runtime 集成（cmd/agent-server/logics/runtime.go）

- [x] 9.1 在 `Runtime` 结构体中新增 `promptMgr *prompt.Manager` 字段
- [x] 9.2 在 `New()` 函数中调用 `prompt.NewManager(readiness)` 构建 promptMgr
- [x] 9.3 新增 `PromptSyncer() *prompt.Syncer` 方法，供 service 层注册 cron 任务
- [x] 9.4 新增 `PromptStore() *prompt.PromptStore` 方法，供 agent 构建层注入 callback

## 10. Agent 层注入 PromptReplaceCallback

- [x] 10.1 修改 `cmd/agent-server/logics/agent/graph_build.go`：在 `modelCb.BeforeModel` 中，在 `MakeSkillInjectWithModelCallback` 之前插入 `prompt.MakePromptReplaceCallback(promptStore)`；`BuildGraph` 函数签名新增 `promptStore *prompt.PromptStore` 参数
- [x] 10.2 修改 `cmd/agent-server/logics/agent/agent_llm.go`：在 `modelCb.BeforeModel` 中插入 `prompt.MakePromptReplaceCallback(promptStore)`；`NewLLMAgent` 函数签名新增 `promptStore *prompt.PromptStore` 参数
- [x] 10.3 更新 `cmd/agent-server/logics/runtime.go` 中调用 `BuildGraph` 和 `NewLLMAgent` 的位置，传入 `rt.PromptStore()`

## 11. Service 层集成（cmd/agent-server/service/）

- [x] 11.1 修改 `cmd/agent-server/service/service.go`：在 `initCronTasks()` 中调用 `prompt.RegisterSyncCronTask(s.runTime.PromptSyncer())`，将任务存入 `s.tasks[enumor.CronTaskSyncAgentPrompts]`
- [x] 11.2 新建 `cmd/agent-server/service/prompt/` 目录，新建 `service.go` 注册 `/prompts/sync` 手动触发接口（参照 `service/skill/service.go`）
- [x] 11.3 新建 `cmd/agent-server/service/prompt/sync.go`，实现 `SyncPrompts` handler（参照 `service/skill/sync.go`）
- [x] 11.4 在 `cmd/agent-server/service/service.go` 的 `initService()` 中调用 `promptsvc.InitService(c)`
