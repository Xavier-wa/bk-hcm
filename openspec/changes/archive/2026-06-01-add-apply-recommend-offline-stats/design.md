## Context

本变更跨 woa-server / data-service 两层，新增两张 MySQL 表、一个 cron 任务和两个 HTTP 接口（Top-N 查询 + 手动触发）。
数据规模：user 表上限 ~150w 行，biz 表上限 ~3k 行，源数据 `ziyan_cvm_device_info` 90 天内（按 `updated_at` 回溯）可能达数十万到百万级行；部分源记录 `cloud_region` 为空，需在聚合前经 zone 表映射补全。
设计须在内存安全、DAO 接口统一（仅标准 CRUD）、cron 幂等三者之间取得平衡。

## Goals / Non-Goals

**Goals:**
- 确定内存聚合策略，避免 OOM
- 确定 Upsert 编排方案，在只使用标准 CRUD 接口的前提下保证数据正确
- 确定过期清理方案，避免大批量 DELETE 引发锁/binlog 问题
- 确定 cron 编排原子性策略，避免半成品数据

**Non-Goals:**
- 不涉及 MCP Server 封装实现
- 不讨论实时库存校验与综合评分（兄弟需求）
- 不改变 data-service DAO 接口约定（保持标准 CRUD）

## Decisions

### Decision 1：内存聚合策略 — 分页拉取 + 计数 Map

**选型：** 分页拉取 + 在内存中只维护计数 Map，而非全量记录 slice。

**理由：**
全量拉取所有行到 slice 后再聚合（技术方案原描述），在 90 天 × 日均 1w 交付的场景下内存约 450 MB，存在 OOM 风险。
改为分页拉取（`page.Limit` 由配置 `pageSize` 控制，默认 500），每页记录处理后**仅累加到 `map[key]int`**，原始行即可丢弃。

内存上限分析：

```
用户维度 map key 数 ≤ 150w (300biz × 500user × maxRows=5 三元组上限)
每个 key ~120 bytes → 150w × 120B ≈ 180MB（最坏情况，实际远小于此）
biz 维度 map key 数 ≤ 3k，可忽略
```

相比全量 slice 方案（450 MB），计数 Map 峰值内存降低 60% 以上，且不随记录数线性膨胀。

**备选方案：** 全量 slice → 被否，OOM 风险；流式 Goroutine Pipeline → 过度复杂，当前规模不必要。

---

### Decision 2：写入编排 — 按分组 Delete + BatchCreate

**选型：** 用户表以 `(bk_biz_id, bk_username)` 分组、业务表以 `bk_biz_id` 分组；对每个分组先 `BatchDelete` 该分组旧推荐行，再 `BatchCreate` 本轮新 Top-K 行。写入通过 data-service HTTP client（`BatchDelete` / `BatchCreate`）完成。

**理由：**
- DAO 只提供标准 CRUD（用户约束），无法使用 `INSERT ... ON DUPLICATE KEY UPDATE`
- `List + Diff + Create/Update/Delete` 三路操作往返多、耗时不可接受
- 分组批处理：用户表每个 (biz, user) 分组 1 次 BatchDelete + 1 次 BatchCreate；业务表每个 biz 1 次 BatchDelete + 1 次 BatchCreate

**Delete + BatchCreate 两步之间的短暂空窗：** 执行时长约毫秒级，对推荐系统可接受（非实时一致性场景）。

**一致性范围：** 实际实现未使用单事务包裹（写入经 data-service HTTP 接口的 `BatchDelete` + `BatchCreate` 两次独立调用），依赖"先删后建 + 失败即停 + 下轮覆盖 + 过期清理"达成最终一致；由于每个分组写入时 `updated_at` 必然刷新为本轮时间，Decision 3 的过期清理不会误删本轮触达的行。

**备选方案：**
- `ON DUPLICATE KEY UPDATE`：需要专用 DAO 方法，违反约定
- `List + Diff`：O(groups) 往返，性能差
- 单事务（`AutoTxn`）包裹 Delete+Create：data-service 接口为通用 CRUD HTTP，跨调用无共享事务，未采用

---

### Decision 3：过期清理 — 最终兜底 Delete，分批执行

**场景：** 某业务 90 天内无任何交付记录，其旧推荐行不会被 Decision 2 的循环处理，会长期留存。

**方案：** 在所有 biz 的 Delete + BatchCreate 完成后，对 user 表和 biz 表分别执行：

```
DeleteWithTx WHERE updated_at < startTime
```

分批执行（每批 500 行，通过 List 分页拿 id 再按 id 批量删除），避免一次 DELETE 命中数十万行导致长事务。

**startTime 语义：** cron 任务开始执行时记录的时间戳。由于 Decision 2 按 biz 循环写入，新写入行的 `created_at` / `updated_at` 均 ≥ `startTime`，因此该条件能精确区分"本轮触达"与"本轮未触达"。

---

### Decision 4：cron 编排原子性 — 两阶段顺序提交，失败即停

**编排顺序：**
```
1. 记录 startTime
2. 分页拉取 + 地域补全 + 计数聚合（内存）
3. 循环写 user 表（按 (bk_biz_id, bk_username) 分组，BatchDelete + BatchCreate）→ 任一分组失败即 return error
4. 循环写 biz 表（按 bk_biz_id 分组，BatchDelete + BatchCreate）→ 失败即 return error
5. 过期清理（user 表 + biz 表，分批 Delete WHERE updated_at < startTime）
```

**失败行为：**
- 步骤 3 中途失败：user 表部分分组已写新数据，其余分组保留旧数据；biz 表未写。下次运行时，步骤 3 会覆盖写（BatchDelete + BatchCreate），步骤 5 会清理本次遗留数据。**最终一致，非即时一致。**
- 步骤 4 失败：user 表数据是新的，biz 表是旧的；下次运行会纠正。
- 步骤 5 失败：过期行未清理，不影响正确性（只是多留了一段时间），下次运行会清理。

**AC-S03 对齐：** 任一步骤失败均记录 `logs.Errorf` 并 `return error`，本轮任务终止，不继续写入。不存在"继续写部分数据"的情况，满足"源数据拉取失败时结束本轮任务"的要求。

---

### Decision 5：查询接口"按人补业务"— woa-server 内存去重

查询接口注册在业务路由下（`POST /api/v1/woa/bizs/{bk_biz_id}/task/apply/recommend/top`，Handler 位于 `cmd/woa-server/service/task`，`bizService` 路由前缀为 `/bizs/{bk_biz_id}/task`）：`bk_biz_id` 从路径参数解析，请求体仅含 `bk_username` 与 `limit`，进入业务逻辑前先做业务访问鉴权（`Biz` / `Access`）。

`data-service` 提供通用 `List`（支持 filter + sort + page），woa-server 的推荐查询 Handler 负责：

1. `List(filter: biz+user, sort: count DESC, limit: N)` → user 行
2. 若 `len(userRows) < limit`：`List(filter: biz, sort: count DESC, limit: limit)` → biz 候选
3. 在内存中按 `(require_type, region, device_type)` 三元组排除 user 已有组合，取前 `remain` 条
4. 拼接返回，`source` 字段标注来源

biz 表每业务行数 ≤ `maxRows`（默认 5），全量拉回开销可忽略，无需 NOT IN 下沉到 DAO。

---

### Decision 6：地域补全 — 按页批量查 zone 表回填 cloud_region

**场景：** 源表 `ziyan_cvm_device_info` 部分记录的 `cloud_region` 为空，仅有 `cloud_zone` 或 `zone_name`，而推荐三元组的 region 维度需要稳定的 `cloud_region`。

**方案：** `collectCounts` 先分页将所有源记录拉取至内存，再对全量记录统一调用 `fillOutCloudRegion`：对 `cloud_region` 为空的记录收集其 `cloud_zone` / `zone_name`，按 `DefaultMaxInLimit` 分批构造 IN / JSON-IN 条件，调用 `Zone.ListZoneExt` 查询并构建 zone→region、campusName→region 映射后回填，最后再遍历计数。补全后 `cloud_region` 仍为空的记录记录 Error 日志并跳过，不计入聚合。

**理由：** 先全量拉取再统一补全，避免每页重复发起 zone 查询；批量 IN 查询避免逐行往返；空 region 记录跳过而非估算，保证三元组质量。

**备选方案：** 预先全量加载 zone 映射表 → 内存占用与同步成本更高，批量 IN 查询足够。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| user 表写入期间（Delete 后 BatchCreate 前）该 biz 用户短暂查不到推荐 | 推荐查询退化为返回 biz 维度数据，可接受（非强一致场景） |
| 150w 行 user 表的过期清理 DELETE 耗时较长 | 分批 Delete（每批 500 行），控制单次事务大小 |
| 源数据拉取量超预期（90天交付量激增） | `pageSize` 可配置；计数 Map 内存有界（key 数量由 maxRows 限制） |
| 多副本重启期间 master 切换导致两个节点同时执行 | `sd.IsMaster()` 在每次 Do() 入口判断；即使极端情况下两个节点同时执行，Delete+BatchCreate 是幂等的 |

## Migration Plan

1. 执行 `scripts/sql/0077_20260527_1500_apply_recommend.sql` 建表（无历史数据迁移）
2. 部署 data-service（新增路由）
3. 部署 woa-server（新增 cron 任务 + Top-N 查询接口 + 手动触发接口）
4. 首次 cron 执行后两张表才有数据；HTTP 接口在首次执行前返回空数组，不报错
5. 回滚：删除两张表（无影响其他功能），回滚 data-service / woa-server 二进制即可
