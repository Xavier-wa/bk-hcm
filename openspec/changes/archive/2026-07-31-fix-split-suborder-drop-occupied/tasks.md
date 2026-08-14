## 1. 解析层：从工具调用 arguments 还原占用子单

- [x] 1.1 扩展 `recommendToolCall`（`parse.go`），使其携带触发中断的那次工具调用的入参
- [x] 1.2 调整 `findRecommendToolCall` 复用 `toolproxy.ResolveToolCall` 的结果并填充 `args`，确认 by_static / by_plan 两条既有路径的返回值语义不变
- [x] 1.3 新增 `extractOccupiedSuborders`，从解包后的 `body_param.occupied_suborders` 取值（tool-proxy 与直连两种形态经 `ResolveToolCall` 已统一，无需双 schema 尝试），缺失或解析失败时返回 nil 并打 Warn 日志

## 2. 中断层：合并后构造 payload

- [x] 2.1 `handleSplitSuborderRecommend`（`node.go`）在 `parseSuborders` 之外取出占用子单，经 `mergeSuborders` 按「已占用在前、增量在后」拼接（不做去重，过滤 nil 项）
- [x] 2.2 保持中断判定以**增量子单数** `>= 1` 为准，合并结果只用于 payload，不参与判定
- [x] 2.3 将合并后的完整清单传入 `interruptWithRecommendSuborders`，`buildRecommendOptionsMessage` 注入模型上下文的随之也是合并后清单
- [x] 2.4 补充中文注释说明「接口只返回增量、占用子单需从入参还原」这一非显然约束

## 3. 单元测试

- [x] 3.1 增量拆分：`TestMergeSuborders/occupied_first_then_incremental` 断言合并清单已占用在前
- [x] 3.2 非增量拆分：`TestMergeSuborders/no_occupied_keeps_incremental_only` 与 `TestExtractOccupiedSuborders/occupied_absent` 断言与改动前一致
- [x] 3.3 解析退化：`TestExtractOccupiedSuborders` 覆盖空数组 / 非法 JSON / 类型不符 / 空入参四种退化
- [x] 3.4 判定边界：`TestHandleSplitSuborderRecommendNoIncrement` 断言入参带 occupied 但增量为空时不中断、回 `llm`
- [x] 3.5 双形态：`TestFindRecommendToolCallCarriesResolvedArgs` 断言 tool-proxy 信封与直连入参均能取到 occupied
- [x] 3.6 回归：`graph_build_test.go` 路由用例（含 after_tool_hitl）与 `aftertool` 既有用例全部通过

## 4. 验证与收尾

- [x] 4.1 `go build ./cmd/... ./pkg/...`、`gofmt`、`go vet ./cmd/agent-server/logics/agent/aftertool/...` 与 `go test ./cmd/agent-server/logics/agent/aftertool/...` 均通过
- [ ] 4.2 端到端手测：以「已有 S2 子单 → 追加 S3」复现 TAPD 1069995598136531300 场景，确认卡片展示两条配置
- [ ] 4.3 手测确认后建单：点「确认方案」后检查 `create_biz_apply` 入参含全部子单

> 4.2 / 4.3 需在联调环境由人工执行，代码侧改动已就绪。
>
> 验证过程中发现的**既有**问题（与本变更无关，未处理）：
> - `cmd/agent-server/logics/agent/hitl/node_test.go:244` 引用已移除的 `constant.HumanConfirmToolName`，该包 vet/test 无法通过
> - `cmd/agent-server/logics/tool/tool_index_test.go:99` 调用 `extractToolMeta` 参数个数不符，该包构建失败
> - `cmd/agent-server/logics/agent` 的 `TestUnsupportedIntentFallbackMessage` 断言的兜底文案已过期（现文案含「云资源查询」）
> - `cmd/agent-server/service/agui-event` 的 `TestIsToolConfirmInterruptKey` / `TestBuildToolConfirmEvent` 失败
> - `cmd/agent-server/logics/agent/intent` 的 `TestMakeIntentRecognitionNode_ToolMessagesFiltered` 失败
