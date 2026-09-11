# generate-record-match-exclusion Specification

## Purpose
TBD - created by archiving change fix-scheduler-queue-premature-release. Update Purpose after archive.
## Requirements
### Requirement: 生产记录在本轮匹配处理结束前独占派发

系统 SHALL 使 generate informer 派发出的生产记录，自被某个匹配 worker 取出起进入「处理中」状态，直到本轮匹配处理结束为止；该生产记录在处理中期间 MUST NOT 被派发给第二个匹配 worker。

处理中期间由 5s 轮询触发的**同一生产记录的重复入队** MUST 只被合并暂存，不得转化为一次新的派发；暂存的入队通知 SHALL 在该记录释放之后才转为下一次派发。

该互斥 MUST 保证同一批设备的初始化、压测与交付动作在一轮中至多各执行一次；MUST NOT 出现「取出后立即释放、匹配仍在进行中却已可被再次派发」的状态。

追溯：功能点 F-002，业务规则 R-003；验收 AC-003。

#### Scenario: 处理中期间重复入队不产生第二次匹配

- **GIVEN** 生产记录 `G` 已被某个 matcher worker 取出，且本轮匹配尚未结束
- **WHEN** `G` 在此期间被再次入队
- **THEN** `G` 不被派发给第二个 worker，`ziyan_cvm_device_info` 中该批设备的 `is_delivered` 由 `false` 置为 `true` 的更新只发生一次（AC-003）

#### Scenario: 并发匹配 worker 不再读到相同的未匹配状态

- **GIVEN** 生产记录 `G` 的 `is_matched` 仍为 `false`，且某个 worker 正在对其执行匹配
- **WHEN** 另一个匹配 worker 在同一时刻尝试取件
- **THEN** 该 worker 取不到 `G`，不会读到与前者相同的 `IsMatched = false` 与相同的设备列表，同一批设备的初始化、压测与交付不被重复执行

#### Scenario: 不同生产记录之间仍并行匹配

- **GIVEN** 队列中同时存在多条互不相同的生产记录
- **WHEN** 多个匹配 worker 同时取件
- **THEN** 这些记录被并行处理，互斥只作用于同一条生产记录

### Requirement: 生产记录在本轮匹配结束后必须被释放

系统 SHALL 在生产记录的本轮匹配处理结束时释放该记录，使其能被后续轮次再次派发。释放 MUST 发生在本轮处理的**全部退出路径**上，包括：匹配正常完成、因生产记录状态未就绪而提前返回、因该记录已匹配而提前返回、匹配返回错误、以及处理过程中发生 panic 后的栈展开。

任一生产记录 MUST NOT 因释放缺失而永久停留在「处理中」状态；该情况会使其在 informer 重建（进程重启）前不再被匹配，SHALL 被视为回归缺陷。

同一生产记录在同一轮处理中 MUST 至多被释放一次；对未处于处理中状态的元素执行释放 MUST NOT 产生副作用。

追溯：功能点 F-002，业务规则 R-004（generate 链路）；验收 AC-004。

#### Scenario: 匹配以错误告终后仍可被重试

- **GIVEN** 生产记录 `G` 的本轮匹配以 error 返回
- **WHEN** 下一轮轮询再次入队 `G`
- **THEN** `G` 被再次派发并重试匹配，日志出现第 2 次 `match done, generate id: G` 或第 2 次匹配失败记录（AC-004）

#### Scenario: 状态未就绪而提前返回后仍可被再次派发

- **GIVEN** 生产记录 `G` 被取出时其 `status` 尚未为 Success，本轮匹配未执行即返回
- **WHEN** 下一轮轮询再次入队 `G`
- **THEN** `G` 被再次派发，不因提前返回而丢失释放

#### Scenario: 已匹配而提前返回后仍被释放

- **GIVEN** 生产记录 `G` 被取出时其 `is_matched` 已为 `true`，本轮不再重复匹配
- **WHEN** 本轮处理返回
- **THEN** `G` 被释放，不在处理中集合中留下占位

### Requirement: generate 队列的停机与异常元素边界行为

系统 SHALL 在 generate informer 已停止（队列已 shutdown）时，使匹配 worker 的取件返回空结果且不报错；该轮 MUST NOT 执行任何释放动作，进程 MUST NOT panic。

系统 SHALL 在 generate 队列中出现非预期类型的元素时，由取件方就地释放该元素并返回错误、记录告警日志；该元素 MUST NOT 在队列的处理中集合中留下占位，后续正常生产记录的派发 MUST NOT 受其影响。

追溯：功能点 F-002 的边界条件，业务规则 R-004；验收 AC-007 / AC-008。

#### Scenario: 队列已 shutdown 时取件不释放不 panic

- **GIVEN** generate informer 已停止，队列已 shutdown
- **WHEN** 匹配 worker 尝试取出元素
- **THEN** 取不到有效生产记录、进程不 panic，且该轮不执行任何释放动作（AC-008）

#### Scenario: 非预期类型元素被就地释放

- **GIVEN** generate 队列中存在一个非预期类型的元素
- **WHEN** 匹配 worker 从队列取出它
- **THEN** 该元素被立即释放并记录告警日志，队列中不留下该元素的占位，后续正常元素的派发不受影响（AC-007）

