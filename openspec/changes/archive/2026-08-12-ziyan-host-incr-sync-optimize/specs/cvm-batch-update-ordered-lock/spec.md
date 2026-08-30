# cvm-batch-update-ordered-lock

data-service CVM 批量更新按 ID 升序加锁，消除并发写入流程间的 MySQL 行锁循环等待（Error 1213）。

## ADDED Requirements

### Requirement: 批量更新按 ID 升序执行

data-service 中**每个在单个事务内逐行更新 cvm 的批量入口** SHALL 在进入事务前将请求内的更新项按 `id` 升序排序，事务内 MUST 按该顺序逐行执行更新。该约束当前覆盖 `BatchUpdateCvm`（全 vendor 泛型、带 extension）与 `BatchUpdateCvmCommonInfo`（跨 vendor、窄字段），后续新增的同类入口同样 MUST 遵守。

#### Scenario: 乱序请求被排序

- **WHEN** 请求携带的更新项 id 顺序为 [c, a, b]
- **THEN** 事务内实际更新顺序为 a → b → c

#### Scenario: 两个入口产出一致的加锁顺序

- **WHEN** `BatchUpdateCvm` 与 `BatchUpdateCvmCommonInfo` 各自收到同一批 cvm 且批内顺序不同
- **THEN** 两者排序后的加锁顺序完全一致，MUST NOT 因入口不同而形成相反的加锁顺序

### Requirement: 并发更新同批 CVM 不产生死锁

两个及以上并发事务更新存在交集的 CVM 行集合时，MUST 以相同全局顺序获取行锁，等待图无环，MUST NOT 出现 Error 1213（Deadlock found when trying to get lock）。

#### Scenario: 两个并发批量更新同一批主机

- **WHEN** 事务 A 与事务 B 并发更新同一批 CVM（如增量 watch 与全量同步同时到达）
- **THEN** 两者按相同 id 顺序加锁，B 至多等待 A 释放，双方均能完成，无死锁牺牲事务

#### Scenario: 分配业务与增量同步并发写同一批主机

- **WHEN** 用户将一批 ziyan 主机分配到业务（走 `BatchUpdateCvmCommonInfo`），同期 CC 侧 biz 变更事件触发增量同步更新同一批主机（走 `BatchUpdateCvm`）
- **THEN** 两条路径按相同 id 顺序加锁，MUST NOT 出现 Error 1213

### Requirement: 更新业务语义不变

排序 MUST NOT 改变任何单台主机的更新内容：extension 合并（json merge）、is_gpu 重算、非零字段过滤（DAO 仅更新非零值）及审计行为与排序前完全一致。

#### Scenario: 排序前后更新结果等价

- **WHEN** 对同一批主机分别以乱序与升序请求执行批量更新
- **THEN** 两种请求落库后的最终数据完全一致
