## ADDED Requirements

### Requirement: Skill Install 流水线

系统 SHALL 将 Skill 的「获取 zip → 下载 → 解压 → 记录版本」定义为 **Install** 操作，由 `Installer` 接口实现，签名语义为：`Install(ctx, skill) error`。

Install 流程 MUST：

1. 通过 BKAIDev Client 获取 skill.zip 下载 URL
2. 将 zip 下载到 `archiveDir`（与 `root` 分离的配置路径）
3. 解压到 `root/{skillName}/`（`root` 为 FSRepository 扫描目录）
4. 通过 `pkg/tools/manifest.Store` 更新 `{root}/manifest.json`，记录 skillName、version、md5、updatedAt

#### Scenario: 成功 Install 单个 Skill

- **WHEN** Installer 对某 enabled Skill 执行 Install
- **THEN** `root/{skillName}/` 目录 MUST 包含 SKILL.md 及附属文件
- **THEN** manifest MUST 记录该 Skill 的 version 与 md5

#### Scenario: Install 原子替换

- **WHEN** Install 覆盖已存在的同名 Skill
- **THEN** 系统 MUST 先解压到临时目录，成功后再原子替换目标目录，避免 FSRepository 读到不完整文件

#### Scenario: zip 存储路径与 root 分离

- **WHEN** 下载 skill.zip
- **THEN** zip 文件 MUST 存储在 `archiveDir` 下，MUST NOT 直接落在 `root` 扫描路径内（除非即为解压后的 skill 子目录）

#### Scenario: 并行 Install

- **WHEN** 首次同步需 Install 多个 Skill
- **THEN** 系统 MUST 使用 goroutine（如 errgroup）并行执行 Install，并发数 MUST 可配置上限

#### Scenario: Install 失败不污染 manifest

- **WHEN** 某 Skill 的下载或解压失败
- **THEN** manifest MUST NOT 更新该 Skill 的版本记录
- **THEN** 错误 MUST 记录日志（英文消息）
