## Why

`cmd/agent-server/logics/skill/repos.go` 里的 `SkillRepos` 是一个「为将来可能的按场景隔离 skill 仓库」预留的抽象：它把 `IntentTypeHostApply` / `IntentTypeResourceQuery` / `IntentTypeChat` 三个场景映射到**同一个** `FSRepository` 实例。这个预留至今没有任何使用方，却在 `BuildGraph` 的函数签名、`newAGUIRunner` 的构造流程里持续传播，并且带来两个真实风险：`SceneRepo` 对未注册场景静默返回 `nil`（新增场景时会得到一个没有 skill 能力的子图，且无任何报错），以及后续新增子图时容易漏注册。skill 的实际来源本就只有 BKAIDev 同步下来的一个本地目录，按场景切仓库既无需求也无落地路径。

## What Changes

- 删除 `cmd/agent-server/logics/skill/repos.go`（`SkillRepos` 结构体、`NewSkillRepos`、`SceneRepo`）。
- `agent.BuildGraph` 的 `skillRepos *skill.SkillRepos` 参数改为 `skillRepo skillpkg.Repository`，直接把同一个仓库实例透传给 `buildHostApplySubgraph` 与 `buildResourceQuerySubgraph`。
- `logics.newAGUIRunner` 去掉构建按场景映射的分支，graph 模式与 LLM 模式统一使用 `skillMgr.Repository`；`skillMgr == nil` 的校验保留并前置，nil 仓库在启动期直接失败而不是运行期静默降级。
- 不改变 `toolproxy.ToolProxies` 的按场景映射——工具代理确实存在按场景裁剪工具集的现实需求，与 skill 仓库不同。
- 非 **BREAKING**：纯内部接线重构，对外 API、AGUI 协议、skill 同步行为、会话内 skill 加载状态均不变。

## Capabilities

### New Capabilities

- `agent-skill-repo-wiring`: agent-server 内 skill 仓库的构建与注入契约——全进程单一 `FSRepository` 实例，由所有 subagent（子图）共享，启动期强校验非 nil。

### Modified Capabilities

<!-- 无：本变更不改变任何已有 spec 的需求级行为。agent-skill-sync / agent-skill-install
     描述的是 FSRepository 的来源与热更新，仓库实例数量与注入方式不在其需求范围内。 -->

## Impact

- 代码：`cmd/agent-server/logics/skill/repos.go`（删除）、`cmd/agent-server/logics/agent/graph_build.go`（签名与调用点）、`cmd/agent-server/logics/runtime.go`（构造流程、`enumor`/`skillpkg` 导入清理）。
- 影响面：仅 agent-server（Service Layer 单服务），不涉及 data-service / hc-service 等其它层。
- API/DB/配置：无变更。
- 依赖：无新增依赖。
- 测试：现有单测均不直接调用 `BuildGraph`（`graph_build_test.go` 只断言主图拓扑，`account_gate_test.go` 调的是 `buildSkillTools`），预计无需改测试，仅需回归编译与 `go test ./cmd/agent-server/...`。
