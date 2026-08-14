## 1. 前置确认

- [x] 1.1 确认 D2 强校验不扩大失败面：现状 `newAGUIRunner` 在模式分支前无条件调用 `NewSkillRepos`，后者对 nil repo 已返回错误，故「Repository 为 nil ⇒ 启动失败」是既有行为，采用强校验
- [x] 1.2 确认全仓无 `SkillRepos` / `NewSkillRepos` / `SceneRepo(` 的其它引用方，仅 `graph_build.go` 与 `runtime.go` 两处调用点（仓库内无 agent-server 的 helm/values 配置，无需同步）

## 2. 移除按场景映射

- [x] 2.1 删除 `cmd/agent-server/logics/skill/repos.go`（`SkillRepos`、`NewSkillRepos`、`SceneRepo`）
- [x] 2.2 修改 `agent.BuildGraph` 签名：`skillRepos *skill.SkillRepos` → `skillRepo skillpkg.Repository`
- [x] 2.3 `BuildGraph` 内两个子图 builder 调用点改为直传 `skillRepo`，去掉 `skillRepos.SceneRepo(...)`
- [x] 2.4 更新 `BuildGraph` 的函数注释，说明 skill 仓库为全局单实例、所有子图共享，并解释 toolproxy 为何仍按场景取

## 3. 收敛 runtime 构造流程

- [x] 3.1 `newAGUIRunner` 删除 `skill.NewSkillRepos(...)` 构建块及其错误处理
- [x] 3.2 在按 `aguiCfg.Model.Mode` 分支之前统一做 `skillMgr == nil || skillMgr.Repository == nil` 校验，error 日志遵循项目日志规范
- [x] 3.3 graph 分支改为 `agent.BuildGraph(defaultMdl, skillMgr.Repository, ...)`
- [x] 3.4 default（LLM）分支去掉 `llmSkillRepo` 的局部判空，直接使用 `skillMgr.Repository`
- [x] 3.5 清理导入：`runtime.go` 删除已未使用的 `skillpkg`、新增 `errors`；`enumor` 因 `AgentModeGraph` 等仍在使用而保留；`gofmt -l` 无输出

## 4. 验证

- [x] 4.1 `go build ./cmd/agent-server/logics/...` 通过（`./cmd/agent-server` 主包的最终链接在本机因 sqlite-vec cgo 环境问题失败，与本变更无关，clean tree 上同样复现）
- [x] 4.2 `gofmt -l` 对两个改动文件无输出；改动行未超出 120 列
- [x] 4.3 `go test -count=1 ./cmd/agent-server/logics/agent ./cmd/agent-server/logics/skill` 全绿，`graph_build_test.go` / `account_gate_test.go` / `loaded_state_test.go` 均无需改动；`logics/agent/hitl` 的 `TestInjectCancelUserMessage` 与 `logics/tool` 的构建失败经 `git stash` 核对为改动前既有失败
- [ ] 4.4 人工冒烟：同一会话内先触发 host_apply 场景再切到 resource_query，确认两个子图均能 `skill_load` 成功且 SKILL 正文被注入系统提示词
- [ ] 4.5 人工冒烟：确认 chat 场景（走 fallback）行为与改动前一致，未因移除 `IntentTypeChat` 的注册而变化
