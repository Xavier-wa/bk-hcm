## ADDED Requirements

### Requirement: LB 全量同步预取整台负载均衡的监听器 RS

在 tcloud-ziyan 的负载均衡**全量同步**路径中，系统 MUST 在拿到该 LB 的云上监听器列表之后、按批同步监听器之前，对整台 LB 调用一次云 API `DescribeTargets`（不传 `ListenerIds`），并将返回结果按监听器云 ID 建立内存索引，供后续每批监听器的 RS 同步复用。

该预取 MUST 只发生在 `listenerOfLoadBalancer`（由 `LoadBalancerWithListener` / `listenerByLbBatch` 进入的全量路径）。Vendor MUST 为 tcloud-ziyan。

#### Scenario: 典型 LB 只调一次 DescribeTargets

- **GIVEN** 一台 tcloud-ziyan LB 的云上监听器数量大于 0 且不超过预取阈值
- **WHEN** 全量同步该 LB 的监听器与 RS
- **THEN** 系统对该 LB 只发起 1 次不带 ListenerIds 的 `DescribeTargets`
- **AND** 后续按最多 20 个监听器一批同步 RS 时不再调用 `DescribeTargets`

#### Scenario: 无监听器时不预取

- **GIVEN** 云上该 LB 监听器列表为空
- **WHEN** 全量同步该 LB
- **THEN** 系统 MUST NOT 调用 `DescribeTargets` 做预取

### Requirement: 独立监听器同步入口不预取

指定监听器同步入口（`Listener()`，含 watcher 与变更后同步）MUST 保持优化前行为：不预取整台 LB 的 RS，按请求中的监听器分批调用 `DescribeTargets`。

#### Scenario: 指定监听器同步仍按批拉 RS

- **GIVEN** 调用方通过独立监听器同步入口同步某 LB 下的一组监听器
- **WHEN** 同步这些监听器的 RS
- **THEN** 系统 MUST NOT 用空 ListenerIds 拉取该 LB 全部 RS
- **AND** 仍按监听器 ID 分批调用 `DescribeTargets`

### Requirement: 预取失败回退到按批拉取

预取 `DescribeTargets` 失败时，系统 MUST 打 Warn 日志并回退为按批拉取，MUST NOT 因此失败该 LB 的监听器或规则同步。

#### Scenario: 预取 API 失败后按批继续

- **GIVEN** 全量同步路径准备预取 RS
- **WHEN** 不带 ListenerIds 的 `DescribeTargets` 返回错误
- **THEN** 系统打印 Warn，含 lb 云 ID 与 rid
- **AND** 后续各批监听器按原逻辑分批调用 `DescribeTargets`
- **AND** 监听器与规则同步继续执行

### Requirement: 用可配置的监听器数量阈值保护预取内存

系统 MUST 在预取前用**已经拿到的云上监听器数量**与配置阈值比较：超过阈值则跳过预取，回退按批 `DescribeTargets`。

阈值 MUST 来自 hc-service 配置 `sync.targetsPrefetchMaxListeners`。未配置或值为 0 时 MUST 默认 **10000**。

跳过预取处 MUST 有注释说明：监听器与 RS 一般不超过 1:2 的对应关系，因此用监听器数量近似约束预取内存占用。

系统 MUST NOT 为该护栏额外查询 DB 或云 API 的 RS 数量。

#### Scenario: 监听器数超过阈值则跳过预取

- **GIVEN** `targetsPrefetchMaxListeners` 为 10000
- **AND** 某 LB 云上监听器数量为 10001
- **WHEN** 全量同步该 LB
- **THEN** 系统 MUST NOT 发起不带 ListenerIds 的整台预取
- **AND** 打印 Info 日志，含 lb 云 ID、监听器数量与 rid
- **AND** 后续按最多 20 个监听器一批调用 `DescribeTargets`

#### Scenario: 监听器数等于阈值仍预取

- **GIVEN** `targetsPrefetchMaxListeners` 为 10000
- **AND** 某 LB 云上监听器数量为 10000
- **WHEN** 全量同步该 LB
- **THEN** 系统对该 LB 发起一次整台 RS 预取

#### Scenario: 未配置时默认 10000

- **GIVEN** hc-service 的 `sync.targetsPrefetchMaxListeners` 未配置或为 0
- **WHEN** 服务启动并执行全量 CLB 同步
- **THEN** 预取跳过判断使用的阈值为 10000

### Requirement: 预取缓存部分 miss 不误删 RS

从预取索引按本批监听器 ID 切片时，缺失的监听器 MUST 只打 Warn，MUST NOT 把缺失当作「云上该监听器后端为空」去删除本地 RS。

#### Scenario: 本批部分监听器不在预取结果中

- **GIVEN** 预取成功且索引非空
- **AND** 本批监听器 ID 中有若干不在索引内
- **WHEN** 同步本批 RS
- **THEN** 系统对 miss 的 ID 打印 Warn
- **AND** 仅使用命中的预取结果参与 RS diff
- **AND** MUST NOT 因 miss 删除这些监听器在本地的 RS
