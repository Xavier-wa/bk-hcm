## 1. 依赖与配置

- [x] 1.1 升级 `trpc-agent-go` 相关依赖至 v1.8.1+，确认 `FSRepository.Refresh()` 可用并回归编译
- [x] 1.2 在 `pkg/cc` 扩展 `AgentSkillsConfig`：新增 `archiveDir`、`bkaidev` 子配置（enabled、baseURL、appCode、appSecret、appID、syncInterval、tag、maxParallel）
- [x] 1.3 更新 `cmd/agent-server/etc/agent_server.yaml` 示例配置与注释

## 2. pkg/tools/manifest（Skill / Prompt 共用）

- [x] 2.1 新建 `pkg/tools/manifest`：`Entry`、`Store`、`NewStore(path)`
- [x] 2.2 实现 `Load`/`Save`（原子写）、`Get`/`Set`/`Remove`、`Diff(remote map[string]string)`
- [x] 2.3 编写 manifest 单元测试（临时目录、Diff 三类结果、并发读写）

## 3. BKAIDev Skill Client

- [x] 3.1 新建 `pkg/thirdparty/api-gateway/bkaidevapi/`，实现 `ServerDiscovery` 与 REST Client 构建（对齐 bkchatapi 模式）
- [x] 3.2 定义 list/retrieve/download/versions 请求与响应类型
- [x] 3.3 实现 `ListSkills`（tag 过滤 status=enabled）、`RetrieveSkill`、`GetSkillDownloadURL`、`ListSkillVersions`

## 4. Skill Install

- [x] 4.1 实现 `installer.go`：`Installer` 接口（download zip → 解压到 root → `manifest.Store.Set`，目录原子替换）
- [x] 4.2 实现并行 Install 控制（errgroup + maxParallel 配置）

## 5. Skill Sync 编排

- [x] 5.1 实现 `syncer.go`：`InitialSync`（list → 并行 install → Refresh → MarkSkillReady）
- [x] 5.2 实现 `SyncOnce`：使用 `manifest.Store.Diff` 驱动增量（新增 install / 删除目录 / 版本变更 install）
- [x] 5.3 改造 `BuildSkillRepo`：返回可 Refresh 的 repository 包装；enabled=false 时保持现有行为
- [x] 5.4 在 Runtime/Service 初始化中启动首次同步 goroutine（与 BuildSkillRepo 并行）
- [x] 5.5 注册 `SkillSyncCronTask`（`pkg/cron`），间隔读取 `syncInterval`

## 6. Readiness 与 HTTP 接口

- [x] 6.1 实现 `readiness.go`：Skill/Prompt Ready 状态与 `IsReady()` 方法
- [x] 6.2 新增 `GET /api/v1/agent/healthz` handler，返回 ready/skillReady/promptReady
- [x] 6.3 实现 `readinessMiddleware`，包装 AGUI/cancel/history 路由，未就绪返回 503
- [x] 6.4 确保 agenthealthz 不被 readiness 中间件拦截

## 7. 集成与测试

- [x] 7.1 补充关键路径日志（英文）与错误处理
