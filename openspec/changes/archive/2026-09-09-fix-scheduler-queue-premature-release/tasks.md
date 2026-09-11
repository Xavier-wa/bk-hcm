## 1. apply 链路独占派发的实现状态核对

> 本组任务均为对现状代码的走查核对，执行人可自行完成并给出证据（引用文件与行号 + 结论）。

- [x] 1.1 走查 `cmd/woa-server/logics/task/informer/apply/apply.go` 的 `Pop`：确认成功路径在把 id 返回给调用方之前**不**释放该元素，元素在返回后仍处于 workqueue 的处理中状态
- [x] 1.2 走查同文件的 `Done(id string)` 与 `Interface` 声明：确认 informer 对外暴露一个与 `Pop` 配对的释放入口，语义为「释放 `Pop` 返回的元素，使该子单可被再次派发」
- [x] 1.3 走查 `Pop` 的类型断言失败分支：确认对非预期对象就地释放并返回 error、记录告警日志，不在处理中集合留下占位（覆盖 AC-007）
- [x] 1.4 走查 `Pop` 的队列 shutdown 分支：确认返回空结果且不报错、不执行任何释放动作、不 panic（覆盖 AC-008）
- [x] 1.5 走查 `cmd/woa-server/logics/task/scheduler/dispatcher/dispatcher.go` 的 `runWorker`：确认释放动作以 `defer` 形式登记，且登记位置在「取件返回 error」与「取件返回空串」两个判断**之后**，避免对未取出的元素误释放
- [x] 1.6 确认该 `defer` 覆盖 `runWorker` 的全部退出路径：`dispatchHandler` 返回错误、业务条件提前返回、panic 后栈展开（覆盖 AC-002，对应业务规则 R-004）
- [x] 1.7 确认释放与互斥发生在按 `order.ResourceType` 分流到 `GenerateCVM` / `GenerateDVM` / `MatchPM` / `UpgradeCVM` **之前**的通用调度层，未对任何资源类型做特化（对应「独占派发覆盖全部资源类型」）
- [x] 1.8 确认 `Pop` / `Done` 的配对契约与两个方向的后果（提前释放导致重复派发、遗漏释放导致永不派发）在代码注释中有明确说明，便于后续修改者不破坏该约束

## 2. generate 链路独占派发的实现状态核对

> 同上，本组任务执行人可自行完成。

- [x] 2.1 走查 `cmd/woa-server/logics/task/informer/generate/generate.go` 的 `Pop`：确认成功路径在返回 id 之前不释放该元素
- [x] 2.2 走查同文件的 `Done(id string)` 与 `Interface` 声明：确认存在与 `Pop` 配对的释放入口
- [x] 2.3 走查 `Pop` 的类型断言失败分支与 shutdown 分支：行为与 apply informer 一致（覆盖 AC-007 / AC-008）
- [x] 2.4 走查 `cmd/woa-server/logics/task/scheduler/matcher/matcher.go` 的 `runWorker`：确认释放以 `defer` 登记，且位置在取件的 error 判断与空值判断之后
- [x] 2.5 确认该 `defer` 覆盖 matcher 的全部退出路径，特别是两条提前返回分支——生产记录 `status` 未就绪、`is_matched` 已为 `true`（覆盖 AC-004，对应业务规则 R-004）

## 3. 交付计数与统计口径未被触碰的核对

> 本组为反向核对（确认「没有改什么」），执行人可自行完成代码走查部分；数据比对部分见第 5 组。

- [x] 3.1 确认 `cmd/woa-server/logics/task/scheduler/generator/generator.go` 的 `getScheduledDeviceStats` 与 `GenerateCVM` 入口总闸的 `scheduledCount >= order.TotalNum` 判断逻辑未被改动（设计上本次通过恢复串行前提让总闸重新成立，而非修改总闸）
- [x] 3.2 确认 matcher 的交付数统计链路——按 `suborder_id` 取设备、按 `IsDelivered` 计数、写回子单 `success_num`、`pending_num` 与终态判定——未被改动（覆盖 AC-009 / AC-010 的代码走查部分，对应业务规则 R-005）
- [x] 3.3 确认 `delivered_core` 按设备机型累加 CPU 核数的算法未被改动
- [x] 3.4 确认未新增或修改对外接口、未变更表结构、未变更 `total_num` / `success_num` / `pending_num` 的字段语义

## 4. 构建与静态检查

> 本组任务执行人可自行完成，需附命令输出。

- [x] 4.1 编译通过：`go build ./cmd/woa-server/...`
- [x] 4.2 静态检查通过：`go vet ./cmd/woa-server/logics/task/...`
- [x] 4.3 格式检查通过：对改动涉及的文件执行 `gofmt -l` / `goimports -l`，输出为空
- [ ] 4.4 既有单元测试通过：`go test ./cmd/woa-server/...`（本期不新增自动化用例；仓库中未跟踪的 `informer/apply/apply_test.go` 与 `informer/generate/generate_test.go` 不纳入本期，不作为验收依据）

## 5. 需真实环境或人工参与的验证

> 本组任务**执行人无法自行完成**，需要测试/生产环境、并发场景构造、真实日志与数据库数据，或需第二人参与。
> 未取得真实证据前 MUST 据实留空，并在阶段报告中逐条列出未完成原因与所需条件，不得以代码走查结论代替勾选。

- [ ] 5.1 并发场景构造与复现：在测试环境构造「同一子单在本轮处理中被再次入队」，确认不产生第二次派发（AC-001）
- [ ] 5.2 并发场景构造与复现：构造「同一生产记录在本轮匹配中被再次入队」，确认该批设备的 `is_delivered` 由 `false` 置 `true` 的更新只发生一次（AC-003）
- [ ] 5.3 活性观察：注入 20 条 `wait_for_match` 子单并观察 10 分钟，按 `suborder_id` 统计 `Successfully dispatch apply order: <suborder_id>` 出现次数，确认被派发次数 ≥ 2 次的比例为 100%、恰为 1 次的条数为 0（AC-P01）
- [ ] 5.4 调度日志处理窗口相交判定：以 `apply order <id> existing device number` 为窗口起点、`Successfully dispatch apply order: <id>` 为窗口终点，按 `suborder_id` 分组做区间相交判定，确认相交次数为 0（AC-P02）
- [ ] 5.5 失败路径重试观察：确认调度处理以错误告终后，下一轮轮询能再次派发该子单（AC-002）；确认匹配以 error 返回后，下一轮能再次派发该生产记录（AC-004）
- [ ] 5.6 端到端交付数核对：一个分 Campus、需求 424 台、容量充足的 CVM 子单跑完全部轮次后，`success_num = 424`、`pending_num = 0`、状态 DONE，且设备表中该子单 `is_delivered = true` 的行数等于 424（AC-005）
- [ ] 5.7 入口总闸行为核对：构造「需求 424 台 / 已落库 400 台 / 在途 24 台」场景，确认不新增生产记录并输出 `apply order <id> has been scheduled 424 cvm (existing: 400, generatingCount: 24)`（AC-006）
- [ ] 5.8 DB 数据比对：SQL 比对子单 `success_num` 与 `SELECT COUNT(*) ... WHERE is_delivered = 1`，并核对 `delivered_core` 与按设备机型累加的 CPU 核数一致（AC-009 / AC-010 的数据部分）
- [ ] 5.9 上线后观察窗口：连续 30 天每日执行 `SELECT COUNT(*) FROM ziyan_cvm_apply_suborder WHERE success_num > total_num AND created_at >= <上线日期>`，确认结果恒为 0（AC-P03，上线后运营动作）
- [x] 5.10 Code Review：由第二人评审，重点核对释放动作在全部退出路径上的配对完整性（活性风险）与 `defer` 登记位置的正确性
