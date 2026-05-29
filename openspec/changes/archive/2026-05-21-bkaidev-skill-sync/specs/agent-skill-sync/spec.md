## ADDED Requirements

### Requirement: 首次同步（启动并行）

当 `skills.bkaidev.enabled=true` 时，系统 MUST 在服务启动阶段启动 goroutine 执行首次同步，且该过程 MUST 与 `BuildSkillRepo` / Runtime 初始化并行，不得阻塞 Repository 构建。

首次同步流程 MUST：

1. 调用 `list_app_v1_skills`（tag 过滤 status=enabled）
2. 对每个 Skill 并行执行 Install
3. 完成后调用 `FSRepository.Refresh()` 热更新
4. 调用 `readiness.MarkSkillReady()`

#### Scenario: 启动时 Repository 可先构建

- **WHEN** agent-server 启动且首次同步尚未完成
- **THEN** `BuildSkillRepo` MUST 成功返回（基于已有 root 内容或空目录）
- **THEN** Runtime MUST 可完成初始化

#### Scenario: 首次同步完成后热更新

- **WHEN** 首次同步全部 Install 成功
- **THEN** 系统 MUST 调用 `FSRepository.Refresh()`
- **THEN** `readiness.skillReady` MUST 变为 true

#### Scenario: BKAIDev 同步开关关闭

- **WHEN** `skills.bkaidev.enabled=false`
- **THEN** 系统 MUST NOT 调用 BKAIDev API
- **THEN** `readiness.MarkSkillReady()` MUST 在 `BuildSkillRepo` 成功后立即调用

### Requirement: 定时增量同步

系统 MUST 使用 `pkg/cron` 框架注册 Skill 同步定时任务，间隔由 `skills.bkaidev.syncInterval` 配置。

每次定时同步 MUST：

1. 拉取远端 enabled Skill 列表
2. 通过 `pkg/tools/manifest.Store.Diff` 与本地 manifest diff
3. 对新增 Skill 执行 Install
4. 对远端已删除的 Skill 删除本地目录与 manifest 条目
5. 对 version 变化的 Skill 执行 Install（version 相同则跳过）
6. 若有变更，调用 `FSRepository.Refresh()`

#### Scenario: 新增 Skill

- **WHEN** 远端列表存在某 Skill 且本地 manifest 无记录
- **THEN** 系统 MUST 执行 Install

#### Scenario: 删除 Skill

- **WHEN** 远端列表不存在某 Skill 且本地 manifest 有记录
- **THEN** 系统 MUST 删除 `root/{skillName}/` 及 manifest 条目

#### Scenario: 版本更新才重新 Install

- **WHEN** 远端 Skill version 与 manifest 中 version 不同
- **THEN** 系统 MUST 执行 Install

#### Scenario: 版本未变跳过

- **WHEN** 远端 Skill version 与 manifest 中 version 相同
- **THEN** 系统 MUST NOT 重新下载或解压

#### Scenario: 定时同步后热更新

- **WHEN** 定时同步产生任意新增、删除或版本更新
- **THEN** 系统 MUST 调用 `FSRepository.Refresh()`
