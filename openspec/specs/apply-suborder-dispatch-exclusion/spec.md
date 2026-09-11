# apply-suborder-dispatch-exclusion Specification

## Purpose
TBD - created by archiving change fix-scheduler-queue-premature-release. Update Purpose after archive.
## Requirements
### Requirement: 申请子单在本轮调度处理结束前独占派发

系统 SHALL 使 apply informer 派发出的申请子单，自被某个调度 worker 取出起进入「处理中」状态，直到本轮调度处理结束为止；该子单在处理中期间 MUST NOT 被派发给第二个调度 worker。

处理中期间由 5s 轮询或启动全量 list 触发的**同一子单的重复入队** MUST 只被合并暂存，不得转化为一次新的派发；暂存的入队通知 SHALL 在该子单释放之后才转为下一次派发。

该互斥 MUST 覆盖整轮调度处理，即从子单被取出直到生产下单动作完成或本轮处理以其他方式结束；MUST NOT 出现「取出后立即释放、处理仍在进行中却已可被再次派发」的状态。

追溯：功能点 F-001，业务规则 R-002；验收 AC-001 / AC-006 / AC-P02。

#### Scenario: 处理中期间重复入队不产生第二次派发

- **GIVEN** 子单 `S` 已被某个调度 worker 从 apply informer 取出，且本轮处理尚未结束
- **WHEN** 轮询在此期间把 `S` 再次入队
- **THEN** `S` 不被派发给第二个 worker，`S` 的两次派发时间窗口（以 `apply order S existing device number` 为窗口起点、`Successfully dispatch apply order: S` 为窗口终点）不存在重叠（AC-001 / AC-P02）

#### Scenario: 并发调度 worker 不再读到同一份已调度数量快照

- **GIVEN** 某子单需求 424 台、当前已落库设备 400 台、在途生产记录合计 24 台
- **WHEN** 调度器在这批在途记录回执前再次轮询到该子单
- **THEN** 该子单的下一轮处理串行发生在上一轮结束之后，读到的已调度数量包含上一轮写入的生产记录，不新增任何 `ziyan_cvm_generate_record` 行，并输出 `apply order <id> has been scheduled 424 cvm (existing: 400, generatingCount: 24)`（AC-006）

#### Scenario: 不同子单之间仍并行调度

- **GIVEN** 队列中同时存在多张互不相同的申请子单
- **WHEN** 多个调度 worker 同时取件
- **THEN** 这些子单被并行处理，互斥只作用于同一子单，调度吞吐不因本要求而串行化

### Requirement: 申请子单在本轮处理结束后必须被释放

系统 SHALL 在申请子单的本轮调度处理结束时释放该子单，使其能被后续轮次再次派发。释放 MUST 发生在本轮处理的**全部退出路径**上，包括：生产下单正常完成、因业务条件提前返回、调度处理返回错误、以及处理过程中发生 panic 后的栈展开。

任一子单 MUST NOT 因释放缺失而永久停留在「处理中」状态；该情况会使其在 informer 重建（进程重启）前不再被调度，SHALL 被视为回归缺陷。

同一子单在同一轮处理中 MUST 至多被释放一次；对未处于处理中状态的元素执行释放 MUST NOT 产生副作用。

追溯：功能点 F-001，业务规则 R-004（apply 链路）；验收 AC-002 / AC-P01。

#### Scenario: 处理以错误告终后仍可被再次派发

- **GIVEN** 子单 `S` 的本轮调度处理已结束且以错误告终
- **WHEN** 下一次 5s 轮询把 `S` 再次入队
- **THEN** `S` 被再次派发，日志中出现第 2 次 `S` 的派发记录；不出现「`S` 仅被派发 1 次后直至进程重启再无记录」（AC-002）

#### Scenario: 处理提前返回后仍可被再次派发

- **GIVEN** 子单 `S` 的本轮调度处理因业务条件（如已达需求总数）未执行生产下单即返回
- **WHEN** 下一次轮询把 `S` 再次入队
- **THEN** `S` 被再次派发，不因未走完整生产流程而丢失释放

#### Scenario: 活性不退化

- **GIVEN** 测试环境中注入 20 条处于 `wait_for_match` 状态的子单
- **WHEN** 观察 10 分钟内 woa-server 日志中按 `suborder_id` 统计的 `Successfully dispatch apply order: <suborder_id>` 出现次数
- **THEN** 这 20 条子单中被派发次数 ≥ 2 次的比例为 100%，被派发次数恰为 1 次的条数为 0（AC-P01）

### Requirement: apply 队列的停机与异常元素边界行为

系统 SHALL 在 apply informer 已停止（队列已 shutdown）时，使调度 worker 的取件返回空结果且不报错；该轮 MUST NOT 执行任何释放动作，进程 MUST NOT panic。

系统 SHALL 在 apply 队列中出现非预期类型的元素时，由取件方就地释放该元素并返回错误、记录告警日志；该元素 MUST NOT 在队列的处理中集合中留下占位，后续正常子单的派发 MUST NOT 受其影响。

追溯：功能点 F-001 的边界条件，业务规则 R-004；验收 AC-007 / AC-008。

#### Scenario: 队列已 shutdown 时取件不释放不 panic

- **GIVEN** apply informer 已停止，队列已 shutdown
- **WHEN** 调度 worker 尝试取出元素
- **THEN** 取不到有效子单、进程不 panic，且该轮不执行任何释放动作（AC-008）

#### Scenario: 非预期类型元素被就地释放

- **GIVEN** apply 队列中存在一个非预期类型的元素
- **WHEN** 调度 worker 从队列取出它
- **THEN** 该元素被立即释放并记录告警日志，队列中不留下该元素的占位，后续正常元素的派发不受影响（AC-007）

### Requirement: 独占派发覆盖全部资源类型

系统 SHALL 使申请子单的独占派发与到期释放作用于**通用调度层**，即在按 `order.ResourceType` 分流到 CVM / IDC DVM / QCLOUD DVM / PM / 升降配 CVM 各生产路径**之前**生效，从而天然覆盖走 apply informer 的全部资源类型。

MUST NOT 按资源类型对独占派发行为做差异化处理或特化。

追溯：需求文档「本期包含」第 4 条（缺陷位于与资源类型无关的通用调度层）。

#### Scenario: 非 CVM 资源类型同样受独占派发保护

- **GIVEN** 一张资源类型为 IDC DVM / QCLOUD DVM / PM / 升降配 CVM 的申请子单
- **WHEN** 该子单在本轮调度处理期间被再次入队
- **THEN** 其行为与 CVM 子单完全一致：不被派发给第二个 worker，本轮处理结束后被释放

