## Why

海垒（tcloud-ziyan）主机增量同步链路存在两个已知问题（来源：主机同步排查与优化「优化」章节）：

1. **增量同步做了不必要的云信息同步**：CC watch 事件表达的是「CC 侧数据变了」，对云侧变化无感知。增量真正需要保时效的字段（业务、IP、负责人、运营状态等）全部来自事件自带的 watch 字段，但现状每条事件都走完整五段链路（CC 查询 / 云 ListCvm / 关联资源 / 主机 / 安全组关系），既拖慢消费，又让增量稳定性受云侧限流、网络抖动影响（增量链路云 API 零重试，一超限整批失败）。
2. **多同步流程并发更新同批主机产生 MySQL 死锁**：host 与 host_relation 两条 watch 流（以及 6h 全量 worker）会并发触发对同一批 CVM 行的 `BatchUpdateCvm`；主机集合来自 Go map 遍历，两个事务以随机顺序逐行加锁，形成循环等待，MySQL 牺牲其中一个事务（Error 1213），导致整批 100 台标记失败、不重试、cursor 照样提交，变更只能等下一事件或 6h 后全量补偿。

## What Changes

- **增量同步取消同步云信息**（hc-service ziyan 新增轻量路由入口，对齐其他 vendor 的 `SyncCvmCCInfoByCond` 模式）：
  - 新增一个路由函数作为增量同步入口，自身只做 CC 与 DB 比对（复用现有查询，零云调用、零写库），按比对结果分流：
    - 创建（CC 有、DB 无）→ 转 f1（`HostWithRelRes`，原样复用不改动；从无到有必须补全云信息与关联资源）。
    - 更新（CC 有、DB 有）→ f2（入口内联的 CC-only 逻辑）：仅依据事件 watch 字段刷新 CC 侧字段，跳过 getCVM / 关联资源同步 / 安全组关系同步。
    - 删除（CC 无、DB 有，事件乱序场景）→ 复用现有 deleteHost 删除。
    - CC 无、DB 无 → 跳过。
  - `HostWithRelRes` 不改动，全量同步与主动触发链路维持原样；cloud-server watch 仅切换 upsert 调用处到新入口，删除事件路径（`DeleteHostByCond`，本就不含云同步）维持不变。
- **CVM 批量更新消除死锁**（data-service）：`BatchUpdateCvm` 事务内对更新目标按 ID 升序排序后再逐条加锁更新，所有并发事务以同一全局顺序加锁，等待图必然无环，从必要条件上消除死锁。

## Capabilities

### New Capabilities

- `ziyan-host-incr-sync-cc-only`: ziyan 主机增量同步新增轻量路由入口（对齐其他 vendor 的 `SyncCvmCCInfoByCond` 模式）：入口以 CC 与 DB 比对（bk_host_id + cloud_id 双维度）将每台主机路由到对应处理——创建转 `HostWithRelRes` 完整链路（保持不变，继续承接全量与主动触发），更新仅刷新 CC 字段（零云调用），删除直接删 DB，均无则跳过。覆盖 cloud-server watch upsert 调用处切换，及云字段时效性边界（云字段刷新完全依赖 6h 全量）。
- `cvm-batch-update-ordered-lock`: data-service CVM 批量更新按 ID 升序排序后逐条更新，消除并发同步流程间的行锁循环等待。

### Modified Capabilities

无（`openspec/specs/` 中无 ziyan 主机同步相关既有能力）。

## Impact

- **涉及服务层级（多层）**：
  - Service Layer：`cmd/cloud-server/service/watch/bkcc/host.go`（仅 ziyan upsert 调用处 `upsertTCloudZiyanHost` 切换到新入口；删除路径不动）。
  - Resource Layer - hc-service：`cmd/hc-service/logics/res-sync/ziyan/` 新增路由函数（复用现有 CC/DB 查询与删除函数）；`cmd/hc-service/service/sync/tcloud-ziyan/` 新增接口入口与路由注册；`pkg/client/hc-service/tcloud-ziyan/` 增加对应客户端方法。
  - Resource Layer - data-service：`cmd/data-service/service/cloud/cvm/update.go`（`BatchUpdateCvm` 排序）。
- **API 变更**：hc-service 新增一个内部同步接口（供 cloud-server watch 增量链路调用，不对外暴露）；既有 `SyncHostWithRelResByCond`、`DeleteHostByCond` 与 data-service `BatchUpdateCvm` 协议均不变。
- **行为变更（需周知）**：增量更新后，云侧字段（运行状态、机型配置、到期时间、安全组关系等）刷新时效从「事件级」降为「6h 全量级」；若存在依赖「CC 一动、HCM 云字段随之更新」的使用方，需由主动触发入口承接。
- **无 DB schema 变更，无新增依赖**。
- **预期收益**：增量更新/删除不再依赖云 API（失败来源从五段收窄到两段，云侧限流与网络抖动不再打断增量）；并发同步死锁消除，整批失败率下降。
