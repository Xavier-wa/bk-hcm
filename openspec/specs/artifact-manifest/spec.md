## ADDED Requirements

### Requirement: pkg/tools/manifest 本地版本清单

系统 SHALL 在 `pkg/tools/manifest` 提供通用的本地 `manifest.json` 管理能力，供 Skill、Prompt 等 BKAIDev 同步产物复用。每个同步域通过独立的 manifest 文件路径实例化 `Store`（例如 `{skillsRoot}/manifest.json`、`{promptDir}/manifest.json`）。

`Entry` MUST 至少包含：`version`、`updatedAt`；`md5` 为可选字段。

`Store` MUST 提供：

- `NewStore(path string)` — 绑定 manifest 文件路径
- `Load()` — 读取全部条目
- `Save(entries)` — 持久化（原子写：先写临时文件再 rename）
- `Get(name)` / `Set(name, entry)` / `Remove(name)` — 单条读写
- `Diff(remote map[string]string)` — 对比远端 `name→version`，返回 `added`、`removed`、`updated` 三类名称列表

#### Scenario: 原子写入 manifest

- **WHEN** 调用 `Save` 更新 manifest
- **THEN** 系统 MUST 先写入同目录临时文件，成功后再 rename 覆盖目标 `manifest.json`，避免读到半截内容

#### Scenario: Diff 识别版本变更

- **WHEN** 远端存在 `skill-a` version `v2`，本地 manifest 记录 `skill-a` version `v1`
- **THEN** `Diff` MUST 将 `skill-a` 归入 `updated`

#### Scenario: Diff 识别新增与删除

- **WHEN** 远端有 `skill-b` 且本地 manifest 无记录
- **THEN** `Diff` MUST 将 `skill-b` 归入 `added`
- **WHEN** 本地 manifest 有 `skill-c` 且远端列表无 `skill-c`
- **THEN** `Diff` MUST 将 `skill-c` 归入 `removed`

#### Scenario: Skill 与 Prompt 独立 manifest 文件

- **WHEN** Skill Syncer 使用 `NewStore("{skillsRoot}/manifest.json")`
- **AND** Prompt Syncer（后续）使用 `NewStore("{promptDir}/manifest.json")`
- **THEN** 两者 MUST 互不影响，共用同一 `Store` 实现

#### Scenario: 并发安全

- **WHEN** 多个 goroutine 对同一 `Store` 实例调用 `Get`/`Set`/`Save`
- **THEN** `Store` MUST 通过内部锁保证读写安全
