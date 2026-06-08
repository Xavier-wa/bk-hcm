## 1. 移除 finish_intent_task 与任务状态

- [x] 1.1 删除 `cmd/agent-server/logics/agent/intent/finish_intent_task.go` 与 `finish_intent_task_test.go`
- [x] 1.2 从 `graph_build.go` 移除 `finish_intent_task` 工具注册与 `MakeFinishIntentTaskCallback`
- [x] 1.3 从 `system_prompt.md` 删除 `finish_intent_task` 整段规范
- [x] 1.4 清理 `pkg/criteria/constant` 中 `FinishIntentTaskToolName`、`StateKeyIntentTaskStatus`（若无引用）
- [x] 1.5 清理 `pkg/criteria/enumor/aiagent.go` 中 `IntentTaskStatus` 类型（若无引用）

## 2. 简化意图节点 state

- [x] 2.1 修改 `intent/node.go`：`intentState()` 仅写入 `StateKeyIntent`，移除 `intent_task_status` 与 `isBusinessIntent` 分支
- [x] 2.2 更新 `intent/node_test.go`：删除 `IntentTaskStatus` 相关用例，保留意图分类用例

## 3. 调整 graph 路由

- [x] 3.1 修改 `makePostFallbackRoutingFunc`：`StateKeyIntent == host_apply` → `llm`，否则 → `intent_recognition`
- [x] 3.2 更新 `graph_build.go` 顶部注释与 fallback 条件边说明
- [x] 3.3 更新 `graph_build_test.go`：用 `StateKeyIntent` 替换 `StateKeyIntentTaskStatus` 的 fallback 路由用例

## 4. 验证

- [x] 4.1 运行 `go test ./cmd/agent-server/logics/agent/... -count=1`
- [x] 4.2 本地或联调验证：主机申领多轮对话不重复调意图识别；闲聊/资源查询 fallback 后下轮重新识别
