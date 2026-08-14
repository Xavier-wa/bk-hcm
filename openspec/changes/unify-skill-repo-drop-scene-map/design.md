## Context

agent-server 的 skill 能力有且只有一个来源：`skill.Manager.Repository`，它是 `BuildFSRepo` 基于 `tools.skills.root`（外加 `extraDirs`）构造出的单个 `skillpkg.RefreshableRepository`，由 BKAIDev syncer 通过 `Refresh()` 热更新。

但在这个唯一实例之上，`logics/skill/repos.go` 又套了一层 `SkillRepos`：

```go
type SkillRepos struct {
    Scene map[enumor.IntentType]skillpkg.Repository
}
```

`newAGUIRunner` 用 `NewSkillRepos(skillMgr.Repository, IntentTypeHostApply, IntentTypeResourceQuery, IntentTypeChat)` 把三个场景全部指向同一个 repo，`BuildGraph` 再用 `skillRepos.SceneRepo(scene)` 取回来。整条链路的输入输出恒等于那一个实例，中间的 map 没有产生任何区分。

这个抽象是照着 `toolproxy.ToolProxies` 抄的形状。但两者的现实需求并不对称：`ToolProxy` 里装着 registry、embedding 索引、tool tags、topN 等**按场景裁剪工具集**的状态，不同场景确实需要不同实例；skill 仓库只是一个目录扫描器，`skill_load` / `skill_list_docs` 工具按名字取 skill，场景差异体现在 LLM 选择加载哪个 skill，而非仓库本身可见哪些 skill。

## Goals / Non-Goals

**Goals:**

- 删掉 `SkillRepos` 这层空转抽象，让「全进程一个 `FSRepository`，所有子图共享」成为代码里显式且唯一的事实。
- 把 nil 仓库的失败时机从「运行期某个场景静默拿到 nil」提前到「启动期直接返回错误」。
- 保持 skill 同步、会话内 skill 加载状态、prompt 注入等所有既有行为不变。

**Non-Goals:**

- 不动 `toolproxy.ToolProxies` 的按场景映射（其按场景隔离有真实语义）。
- 不改 `BuildFSRepo` / `Manager` / `Syncer`，仓库的构造与热更新方式原样保留。
- 不引入按场景过滤 skill 的替代机制（如按 tag 过滤可见 skill）——当前无需求，真要做时再单独提案。
- 不调整 `buildHostApplySubgraph` / `buildResourceQuerySubgraph` 内部逻辑，它们的 `skillRepo skillpkg.Repository` 形参本就已是单实例签名。

## Decisions

### D1：直接删除 `SkillRepos`，而不是保留结构体只留一个场景

`BuildGraph` 的形参从 `skillRepos *skill.SkillRepos` 改为 `skillRepo skillpkg.Repository`，两个子图 builder 收到同一个实例：

```go
func BuildGraph(mdl trpcmodel.Model, skillRepo skillpkg.Repository, toolset *agenttool.MCPToolSet, ...)
```

**备选方案**：保留 `SkillRepos` 但只注册一个「default」键。**放弃原因**：仍然要求每个新子图记得去 map 里取，且 `SceneRepo` 的 nil 静默返回问题原样保留，等于把问题换了个位置。

**理由**：真要恢复按场景隔离时，改动量就是把这个形参换回一个 bundle 结构体——和现在删掉它的代价对称。为一个没有落地路径的未来预留接口，换来的是每个新场景都必须记住去注册、忘了就静默降级。

### D2：nil 校验前移到 `newAGUIRunner` 入口，失败即启动失败

现状是双重弱校验：`NewSkillRepos` 对 nil repo 报错，`SceneRepo` 对未注册场景返回 nil（不报错）。改造后 `newAGUIRunner` 在分支之前统一校验一次：

```go
if skillMgr == nil || skillMgr.Repository == nil {
    logs.Errorf("[new agui runner] skill repository is nil")
    return nil, errors.New("[new agui runner] skill repository is nil")
}
```

**理由**：skill 仓库缺失意味着所有子图都拿不到 skill，这是配置/启动错误，不该降级成「能跑但模型没有任何 skill 可用」。当前 `default` 分支里 `if skillMgr != nil` 的软判断（`llmSkillRepo` 可能为 nil）在前置校验后属于死代码，一并去掉。

**这不是新的失败模式**：现状下 `newAGUIRunner` 在模式分支之前就无条件调用 `NewSkillRepos`，而它对 nil repo 已经返回错误，因此「Repository 为 nil ⇒ 启动失败」本就是当前行为，本决策只是把同一条校验搬到更靠前、更直白的位置。`default` 分支里 `if skillMgr != nil` 的软判断在 `skillMgr == nil` 已提前返回的前提下属于死代码，一并去掉。

**已知的配置盲区（不在本次范围内）**：`AgentBKAIDevSyncSkillsConfig.trySetDefault()` 只在 `SkillSyncEnabled()` 为真时被调用，因此 `skills.enabled: false` 且未显式配置 `skills.root` 时，`Root` 保持空串、`BuildFSRepo` 返回 nil、agent-server 启动失败。这是既有行为，本变更不改变它；如需让 `root` 默认值在关闭同步时也生效，应另开变更处理 `pkg/cc` 的默认值时机。

### D3：`enumor.IntentTypeChat` 的注册随之消失，不影响 chat 场景

当前 chat 场景注册了 repo，但主图里没有 chat 子图（`scene_dispatch` 的三个出口是 host_apply / resource_query / fallback），这个注册项从未被 `SceneRepo` 取用。删除后 chat 走 fallback 的行为完全不变。

### D4：`skill` 包的 import 是否保留

`graph_build.go` 去掉 `skillRepos *skill.SkillRepos` 后，仍然通过 `skill.MakeSkillInjectWithModelCallback` / `skill.MakeSkillLoadAfterToolCallback` 等使用 `logics/skill` 包，import 保留；`runtime.go` 侧需检查 `enumor`、`skillpkg` 是否因去掉构建块而变为未使用（`enumor` 另有 `enumor.AgentModeGraph` 等用法，大概率保留；`skillpkg` 仅用于 `llmSkillRepo` 声明，去掉后应删除）。以 `goimports` 结果为准。

## Risks / Trade-offs

- **[nil 仓库导致启动失败]** → 经核对，这与改动前完全一致（`NewSkillRepos` 已对 nil repo 报错并阻断启动），本变更不扩大失败面。真正的隐患在于上述配置盲区（关闭同步时 `root` 无默认值），属既有问题，不在本次范围。
- **[未来真需要按场景隔离 skill 时要改签名]** → 届时的改动集中在 `BuildGraph` 一个形参和两个调用点，范围与本次删除对称，且那时会有真实需求来定义隔离维度（按 scene？按 tag？按 biz？），比现在凭空猜一个 map 更准确。
- **[纯重构无行为变化，回归依赖编译与既有单测]** → 通过 `go build ./cmd/agent-server/...` 加 `go test ./cmd/agent-server/logics/...` 回归；另做一次人工冒烟：同一会话内先触发 host_apply 再触发 resource_query，确认两个子图都能正常 `skill_load` 并注入 SKILL 正文。
