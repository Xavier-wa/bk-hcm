## Context

当前 agent-server 在 `logics/skill/skill.go` 中通过 `skillpkg.NewFSRepository(roots...)` 从本地目录加载 Skill，配置位于 `tools.skills.root`（默认 `./skills`）。Skill 内容变更需重新部署或手动同步文件，无法利用 BKAIDev 平台的 Skill 托管与版本管理能力。

BKAIDev 提供以下 Skill 相关 API（经 API 网关）：

| API | 用途 |
|-----|------|
| `list_app_v1_skills` | 拉取 App 下 Skill 列表 |
| `retrieve_app_v1_skills` | 查询单个 Skill 详情（SKILL.md、版本号、md5 等） |
| `retrieve_app_v1_skills_download` | 获取 skill.zip 下载 URL |
| `retrieve_app_v1_skills_versions` | 查询 Skill 全部版本 |

项目内已有 `pkg/cron` 定时任务框架（参考 woa-server `initCronTask`）、`pkg/thirdparty/api-gateway` 的 REST Client 构建模式，以及 `tools.bkAIDev` 应用凭证配置（供 MCP bkaidev 类型鉴权复用）。

当前 `trpc-agent-go` 版本为 v1.7.0；本变更需升级至 v1.8.1+ 以使用 `FSRepository.Refresh()` 实现热更新。

## Goals / Non-Goals

**Goals:**

- 通过 BKAIDev API 实现 Skill 的首次同步（启动并行）与定时增量同步
- 将「下载 zip → 解压到 root → 记录版本」抽象为可扩展的 **Install** 流程
- 启动时 Skill Repository 可与首次同步并行构建；对外服务仅在 Readiness 就绪后开放
- 提供 `agenthealthz` 供前端判断 Agent 是否可交互
- 支持配置开关，关闭时回退为纯本地目录模式（与现有行为兼容）

**Non-Goals:**

- 不在本变更中实现 Prompt 的 BKAIDev 远程同步（Readiness 预留 Prompt 检查点，便于后续扩展）
- 不修改 BKAIDev 平台侧 Skill 管理 UI
- 不实现 Skill 的上传/发布（只读同步）
- 不将 manifest 持久化到数据库（使用本地 `manifest.json` 文件）

## Decisions

### 1. Manifest 抽为 pkg/tools 通用工具

**决策**: 在 `pkg/tools/manifest` 实现本地版本清单管理，Skill 与 Prompt（及后续其他 BKAIDev 同步产物）各自持有独立 manifest 文件路径，共用同一套 API。

```
pkg/tools/manifest/
  manifest.go       # Entry、File、Store
  manifest_test.go
```

核心类型与接口：

```go
// Entry 单条资源的本地版本记录
type Entry struct {
    Version   string
    MD5       string    // optional
    UpdatedAt time.Time
}

// Store 绑定一个 manifest.json 路径，线程安全读写
type Store struct { ... }

func NewStore(path string) *Store
func (s *Store) Load() (map[string]Entry, error)
func (s *Store) Save(entries map[string]Entry) error
func (s *Store) Get(name string) (Entry, bool)
func (s *Store) Set(name string, entry Entry) error
func (s *Store) Remove(name string) error
// Diff 对比远端 name→version，返回 added / removed / updated
func (s *Store) Diff(remote map[string]string) (added, removed, updated []string, err error)
```

文件布局示例：

| 产物 | root 目录 | manifest 路径（默认） |
|------|-----------|----------------------|
| Skill | `tools.skills.root` | `{root}/manifest.json` |
| Prompt | `agui.prompt.dir`（后续） | `{promptDir}/manifest.json` |

**理由**: Prompt 远程同步与本变更同期或紧随其后，manifest 逻辑（版本 diff、原子写文件）完全一致，放在 `pkg/tools` 避免 skill/prompt 各写一份；符合项目「通用能力下沉 pkg」惯例。

**替代方案**: 留在 `cmd/agent-server/logics/skill/manifest.go` → Prompt 同步时需复制或再抽一层，重复成本高。

### 2. 模块划分：Client / Syncer / Installer / Readiness

```
pkg/tools/manifest/                      # 本地 manifest.json（Skill / Prompt 共用）
pkg/thirdparty/api-gateway/bkaidevapi/   # HTTP Client（list/retrieve/download/versions）
cmd/agent-server/logics/skill/
  installer.go      # Install 接口与实现（Download → Unzip → manifest.Store）
  syncer.go         # 首次同步 + 定时同步编排（diff 调用 manifest.Store）
  repository.go     # BuildSkillRepo + Refresh 封装
  readiness.go      # Readiness 状态（skill / prompt）
cmd/agent-server/logics/prompt/          # 后续 Prompt 同步复用 manifest.Store
```

**理由**: Install 作为独立概念，Syncer 只负责 diff 决策与调度，Client 只负责 API 通信，manifest 由 pkg 工具提供，职责清晰、便于单测与扩展。

**替代方案**: 全部写在 `skill.go` → 文件膨胀，难以测试。

### 3. BKAIDev Client 构建方式

**决策**: 在 `pkg/thirdparty/api-gateway/bkaidevapi/` 新增 Skill Client，使用 `rest.NewClient` + `ServerDiscovery` + `client.Capability`，与 `bkchatapi`、`itsm` 等保持一致。鉴权使用 `X-Bkapi-Authorization`（`bk_app_code` + `bk_app_secret`），复用 `tools.bkAIDev` 或独立 `tools.skills.bkaidev` 配置块。

**理由**: 项目规范要求 third-party HTTP Client 统一模式；避免在 agent-server 内联 http.Client。

### 4. 目录配置分离

**决策**: 扩展 `AgentSkillsConfig`：

```yaml
skills:
  root: "./skills"              # FSRepository 扫描目录（解压目标）
  archiveDir: "./skills-archive" # zip 下载缓存目录（与 root 分离）
  bkaidev:
    enabled: true
    baseURL: "https://xxxxx."
    appCode: "..."
    appSecret: "..."
    appID: "..."                 # BKAIDev App 标识（list API 所需）
    syncInterval: "5m"           # 定时同步间隔
    tag: "status:enabled"        # list 过滤 tag
```

**理由**: zip 缓存与解压目标分离，避免污染 FSRepository 扫描路径；便于清理缓存。

### 5. Install 流程与并行策略

**决策**: `Installer.Install(ctx, skillMeta)` 执行：

1. 调用 `retrieve_app_v1_skills_download` 获取 zip URL
2. 下载到 `archiveDir/{skillName}/{version}.zip`
3. 解压到 `root/{skillName}/`（原子替换：先解压到临时目录再 rename）
4. 通过 `manifest.Store` 更新 `{root}/manifest.json`（`skillName → {version, md5, updatedAt}`）

首次同步：`list_app_v1_skills`（tag 过滤 `status=enabled`）→ `errgroup` 并行 Install。

定时同步 diff 逻辑：

| 远端状态 | 本地 manifest | 动作 |
|---------|--------------|------|
| 新增 | 不存在 | Install |
| 删除 | 存在 | 删除 `root/{name}` + manifest 条目 |
| 存在，version 变化 | version 不同 | Install（覆盖） |
| 存在，version 相同 | version 相同 | 跳过 |

修改判断以 **version** 为准（用户明确要求）；`retrieve_app_v1_skills` 的 md5 可用于 install 后校验。

### 6. 启动并行与 FSRepository 热更新

**决策**:

```
启动流程（伪代码）:
  readiness := NewReadiness()
  repo, _ := BuildSkillRepo()           // 基于已有 root 或空目录，不阻塞
  go syncer.InitialSync(ctx, repo)      // 并行首次同步
  runtime := NewRuntime(repo, readiness)
  ...
  sync 完成后: repo.Refresh() + readiness.MarkSkillReady()
```

- `BuildSkillRepo` 同步返回，不等待首次同步
- 首次同步 goroutine 完成后调用 `FSRepository.Refresh()` 并标记 Skill Ready
- 定时任务注册 `SkillSyncTask`，每次执行 diff + Install/Remove + `Refresh()`

**理由**: 满足「启动与首次同步并行、Repository 可先构建」；v1.8.1 `Refresh()` 避免重建整个 Runtime。

### 7. Readiness 与 agenthealthz

**决策**:

- 新增 `GET /api/v1/agent/healthz`（或 `/agenthealthz`，与现有 `/healthz` 区分）：返回 `{ ready: bool, skillReady: bool, promptReady: bool }`
- 现有 `/healthz` 保持 etcd 探活语义不变
- 新增 `readinessMiddleware` 包装 AGUI handler：若 `!readiness.IsReady()` 返回 503 + 明确错误码
- `Readiness.IsReady()` = `skillReady && promptReady`；`promptReady` 默认在 Prompt 未配置远程同步时为 `true`

**理由**: 探活与就绪分离；前端只需轮询 agenthealthz。

### 8. Cron 定时任务

**决策**: agent-server `service` 初始化时 `cron.Init` + 注册 `SkillSyncCronTask`，实现 `core.Task` 接口（`Name()` / `Next()` / `Do()`），间隔由 `syncInterval` 配置。

**理由**: 复用项目内成熟 cron 框架，与 woa-server 模式一致。

### 9. 配置开关

**决策**: `skills.bkaidev.enabled=false` 时：

- 不创建 BKAIDev Client
- 不启动 Syncer / Cron
- `readiness.MarkSkillReady()` 在 `BuildSkillRepo` 成功后立即调用
- 行为与当前纯本地模式一致

### 10. trpc-agent-go 升级

**决策**: 将 `trpc-agent-go` 相关依赖升级至 v1.8.1，确认 `FSRepository` 暴露 `Refresh()` 方法；升级后回归 AGUI、Graph、Skill 工具链。

## Risks / Trade-offs

- **[首次同步耗时]** 大量 Skill 并行下载可能占用带宽与磁盘 → 限制并发数（如 `maxParallel=5`），可配置
- **[部分 Install 失败]** 首次同步中个别 Skill 失败 → 记录错误日志，manifest 不更新该 Skill；Readiness 策略：全部成功才 Ready，或允许 partial（建议：核心 Skill 失败则 not ready，可配置阈值）
- **[磁盘空间]** zip 缓存累积 → 定时清理旧版本 zip（保留当前 version）
- **[API 不可用]** BKAIDev 网关故障 → 首次同步重试 + 超时后保持 not ready；已有本地 manifest 的实例可考虑 fallback（Non-Goal，后续迭代）
- **[trpc-agent-go 升级]** 可能引入 breaking change → 升级前阅读 changelog，跑 agent-server 集成测试

## Migration Plan

1. 升级 `trpc-agent-go` 至 v1.8.1+
2. 部署前在目标环境创建 `root` 与 `archiveDir` 目录，配置 BKAIDev 凭证
3. 灰度：`bkaidev.enabled=true` 的实例从平台拉取；`false` 保持现有行为
4. 前端接入 `agenthealthz` 轮询，Ready 后再展示 Agent 对话入口
5. 回滚：设 `enabled=false` 并重启，使用本地已有 `root` 目录

## Open Questions

- BKAIDev `list_app_v1_skills` 的 tag 过滤参数名与 `status=enabled` 的精确传参格式（实现时对照 API 文档确认）
- 首次同步 partial failure 时是否阻塞 Ready（建议默认：有失败则 not ready，可配置 `allowPartialReady`）
- Prompt 远程同步是否与本变更同批交付（当前 Readiness 预留接口，默认 promptReady=true）
