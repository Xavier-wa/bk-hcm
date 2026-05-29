## Why

当前 agent-server 的 Skill 仓库从本地目录静态加载（`tools.skills.root`），Skill 的发布、版本管理与多实例一致性依赖人工拷贝或镜像打包，无法与 BKAIDev 平台的 Skill 托管能力对齐。需要将 Skill 的拉取、安装与热更新改为通过 BKAIDev 开放 API 托管，使 Skill 可在平台侧统一发布，agent-server 自动同步。

## What Changes

- 新增 BKAIDev Skill API HTTP Client（`list` / `retrieve` / `download` / `versions`），构建方式对齐 `pkg/thirdparty/api-gateway` 现有模式
- 新增 `pkg/tools/manifest` 通用工具：本地 `manifest.json` 读写与 diff（Skill、Prompt 等 BKAIDev 同步产物共用）
- 新增 Skill **Install** 能力：列表拉取 → 获取 zip 下载 URL → 并行下载 → 解压到 `root` 目录，并通过 manifest 工具记录版本
- 服务启动时与 Runtime 构建**并行**执行首次同步；`FSRepository` 可先基于已有/空目录构建，首次同步完成后通过 `Refresh()` 热更新
- 接入 `pkg/cron` 定时任务框架，周期性增量同步（新增 / 删除 / 版本变更才重新 install）
- 新增配置项：BKAIDev 同步开关、App 标识、压缩包缓存目录（与 `root` 解压目标目录分离）、同步间隔等
- 新增 **agenthealthz** 接口与 Readiness 中间件：仅当 Skill（及 Prompt，若启用远程同步）首次同步完成后，才将服务标记为 healthy，AGUI 等对外交互才放行
- 拉取 Skill 时使用 tag 过滤 `status=enabled`
- 依赖升级：`trpc-agent-go` 升级至 v1.8.1+ 以使用 `FSRepository.Refresh()`

## Capabilities

### New Capabilities

- `artifact-manifest`: `pkg/tools/manifest` 本地版本清单工具（Load/Save/Get/Set/Remove/Diff，供 Skill 与 Prompt 同步复用）
- `bkaidev-skill-client`: BKAIDev Skill 开放 API 的 HTTP Client 封装（鉴权、请求/响应类型、错误处理）
- `agent-skill-install`: Skill Install 流水线（下载 zip、解压、manifest 版本记录、并行 install）
- `agent-skill-sync`: 首次同步（启动并行）与定时增量同步（cron + FsRepository.Refresh 热更新）
- `agent-readiness`: agenthealthz 健康检查、Readiness 状态机、AGUI 中间件门禁（Skill + Prompt 就绪后才可交互）

### Modified Capabilities

（无：本次为 agent-server 新增能力，不修改 openspec/specs 下已有 spec）

## Impact

- **agent-server**: `logics/skill`、`logics/runtime`、`service` 路由与中间件、`etc/agent_server.yaml` 配置
- **pkg/tools/manifest**: 新增通用 manifest 工具包
- **pkg/cc**: `AgentSkillsConfig` 及相关 BKAIDev Skill 同步配置结构体
- **pkg/thirdparty/api-gateway**: 新增 `bkaidevapi`（或 `bkaidevv1`）Skill Client 子包
- **pkg/cron**: agent-server 注册 Skill 同步定时任务
- **go.mod**: `trpc-agent-go` 升级至 v1.8.1+
- **前端**: 需调用 `agenthealthz` 判断 Agent 是否可交互（Skill 首次同步完成前不可用）
- **运维**: 需配置 BKAIDev 网关地址、应用凭证、zip 缓存目录与 skill root 目录
