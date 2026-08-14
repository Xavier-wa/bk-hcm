## 1. 常量与回写实现

- [x] 1.1 在 `pkg/criteria/constant/aiagent.go` 新增节点内回写超时常量（量级 3s），注释说明它同步计入 run 延迟、故短于 `SessionIncrContentCountTimeout`
- [x] 1.2 实现回写函数：入参为 ctx、data-service 会话客户端、新标签；内部从 ctx 取 invocation 解析 threadID（`RuntimeState[graph.CfgKeyLineageID]`）与用户名，构造 kit 与 timeout ctx，调 `Aiagent.Session.Update`
  - 落点调整：写进已有的 `cmd/agent-server/logics/agent/state/session_tag.go`（该文件本就只讲 session_tag 在 state 里的读写，与 `ParseSessionTag` 相邻最好找），未另建 `session_tag_writeback.go`
  - 依赖形态调整：形参取包内定义的窄接口 `SessionTagUpdater`（只含 `Update`）而非 `*dataservice.Client`，`Aiagent.Session` 天然满足；否则该客户端是具体结构体，单测无法打桩
- [x] 1.3 在回写函数中实现三条防御分支：客户端为 nil → Warn 后返回；threadID 解析失败 → Error 后返回（无法定位会话时禁止写入）；用户名为空 → 取 `constant.BackendOperationUserKey` 兜底并打 Warn（`Reviser` 是 required 字段）
  - kit 拼装沿用 `account_select` 的 `kit.New()` 方式，不用 `core.NewBackendKit()`：后者内部 `SetBackendTenantID` 会读全局配置，把配置依赖带进节点路径，且在未加载配置的单测里直接 panic
- [x] 1.4 回写失败仅记录日志、不向调用方返回错误（或返回错误但由调用方吞掉并打日志），确保调用点无法阻断 run

## 2. 接入 scene_dispatch

- [x] 2.1 给 `makeSceneDispatchNode` 增加回写客户端形参，`BuildGraph`（提案里误写为 `BuildMainGraph`）用已持有的 `clientSet` 透传；签名超过 120 列时按项目换行规范折行
  - 透传经 `sessionTagUpdater(clientSet)` 取值：依赖链任一环为 nil 时返回 nil 接口，避免把 nil 指针塞进接口后延迟到调用时 panic（构图单测不注入 client set）
- [x] 2.2 在节点内决策之后、`EmitSceneSwitched` 之前插入回写调用，触发判据为 `decision.sessionTag != "" && decision.sessionTag != tag`（与 `decision.switched` 解耦，因此不能写在 `if decision.switched` 分支内部）
- [x] 2.3 更新 `makeSceneDispatchNode` 的函数注释：删除「本节点只提交 graph state 与会话级 skill 状态，不写 session_tag 到 DB；标签回写由 service 层在 Run 结束后对账完成」的旧描述，改为说明「标签变化时在判定点同步回写、排在事件 emit 之前、失败只打日志」
- [x] 2.4 更新 `pkg/criteria/constant/aiagent.go` 中 `StateKeySessionTag` 的注释（现写的是「Run 结束后由 service 层对账回写 DB」，需补充判定点回写为主、对账为兜底）

## 3. 兜底路径的定位说明

- [x] 3.1 更新 `cmd/agent-server/service/service.go` 中 `reconcileSessionTag` 与 `asyncReconcileSessionTag` 的注释：说明它已降级为兜底（覆盖节点内写失败、请求 ctx 被 cancel、以及 resolver 缓存刷新），并显式记录「会与节点内回写产生同值双写、写入幂等」这一已知取舍
- [x] 3.2 确认不修改 `reconcileSessionTag` 的判定逻辑与调用时机（仍在 `next.ServeHTTP` 返回后），避免两处判据同时变动

## 4. 单元测试

- [x] 4.1 为回写函数补测：客户端为 nil 时不调用 data-service 且不返回错误；threadID 缺失时不调用 data-service；用户名为空时请求里的 `Reviser` 为兜底用户名
- [x] 4.2 为回写函数补测：标签正常时请求的 `ID` 等于 threadID、`SessionTag` 等于新标签（用假的 data-service 客户端或接口打桩捕获请求）
- [x] 4.3 在 `graph_build_test.go` 中补测触发判据：`"" → host_apply`（首次打标，不发事件但要回写）与 `host_apply → resource_query`（切换，既回写又发事件）都触发回写；`host_apply → host_apply` 与「分类不受支持保持原标签」都不触发
- [x] 4.4 补测顺序保证：回写发生在 `scene.switched` emit 之前（打桩客户端在回写时刻观测事件 channel 长度，须为 0）
- [x] 4.5 补测回写失败不影响节点输出：打桩客户端返回错误时，节点仍返回正确的 `StateKeySceneDispatchNext` 与 `StateKeySessionTag`，且事件照常发出
- [x] 4.6 确认现有 `makeSceneDispatchNode` 相关用例在新增形参后仍可传 nil 客户端通过，不需要为每个用例架设假客户端（三处调用点统一收口到 `newTestSceneDispatchNode` 辅助函数）

## 5. 验证

- [x] 5.1 `go test ./cmd/agent-server/...` 中本次涉及的包全绿（`logics/agent`、`logics/agent/state`），`gofmt`/`go vet` 干净
  - `go build ./...` 在本机无法完成 main 包链接：`sqlite-vec` 的 cgo 符号在 macOS 上链不上，属既有环境问题；改以编译全部非 main 包验证类型
  - `logics/agent/hitl`、`logics/tool`、`service/agui-event` 三个包在 HEAD 上即为红（如 `tool_index_test.go` 仍用 `extractToolMeta` 旧签名），本次未触碰这些文件
- [ ] 5.2 人工验证主链路：新会话首条消息识别为 `host_apply` 后，在子图仍在输出期间查询会话列表接口，`session_tag` 已为 `host_apply`
- [ ] 5.3 人工验证切换链路：`host_apply` 会话发送资源查询类消息，在新场景输出期间切走再切回该会话，前端标签直接显示新场景，无旧值停留
- [ ] 5.4 检查日志：判定点回写成功一条 Info（含 tag/thread），Run 结束后的兜底对账日志能与之对应（确认双写为同值）
