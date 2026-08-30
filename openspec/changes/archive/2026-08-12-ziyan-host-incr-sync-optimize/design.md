# Design: ziyan 主机增量同步优化（取消云同步 + 死锁消除）

## Context

海垒（tcloud-ziyan）主机增量同步现状（详见 proposal.md）：

```
cloud-server watch（bkcc/host.go）
 ├─ upsert 组 → upsertTCloudZiyanHost → SyncHostWithRelResByCond（重型完整链路）
 │     └─ hc-service HostWithRelRes: ListBizHost → getCVM → syncCvmRelRes → Host()(再拉一次云) → syncCvmRel
 └─ delete 组 → DeleteHostByCond → RemoveHostFromCC（纯 DB 删，本无云同步）

并发死锁：host / host_relation 两条 watch 流 + 全量 worker
 → 并发 BatchUpdateCvm 同事务内按 Go map 随机序逐行加锁 → 循环等待 → Error 1213 整批失败
```

关键既有事实（已核实代码）：

- `Host()`（`cmd/hc-service/logics/res-sync/ziyan/host.go:46`）已实现 CC 与 DB 三路 diff（增/改/删），其前半段（CC 查询 + DB 查询 + Diff）正是路由所需的全部输入。
- `listHostFromDBForDiff`（host.go:725）按 bk_host_id + cloud_id 双维度查 DB，防 CC 重建（bk_host_id 变、cloud_id 不变）误判新增导致唯一键冲突。
- 其余五个 vendor 的增量路径均为 `SyncCvmCCInfoByCond` 轻链路，ziyan 是唯一走重型链路的异类。
- data-service `UpdateByIDWithTx` 经 `RearrangeSQLDataWithOption` 生成 SET 子句，**仅更新非零值字段**（`pkg/dal/table/utils/column_helper.go:313`）。
- `batchUpdateCvm` 的 extension 用 `json.UpdateMerge`（gjson `@join`，source 全键覆盖 destination），而 `TCloudZiyanHostExtension` 字段无 omitempty，部分构造的 extension 会把云侧字段**冲成零值**。
- `upsertCmdbHosts` 对 `enumor.TCloudZiyan` 直接 skip（`cmd/data-service/service/cloud/cvm/cmdb.go:47`），ziyan 更新无 CMDB 副作用。

## Goals / Non-Goals

**Goals**

1. 新增 ziyan 增量同步轻量入口：CC 与 DB diff 路由，更新/删除零云调用，创建转完整链路。
2. 消除并发 `BatchUpdateCvm` 的 MySQL 死锁。

**Non-Goals**

- 不改 `HostWithRelRes` 及其全量/主动触发链路的任何控制流。
- 不改 watch 删除事件路径（`DeleteHostByCond` 已无云同步）。
- 不做 iWiki 优化部分的另两项（安全组关系空删除、关联资源重复执行去重）——各自独立 change。
- 不做观测能力建设（metrics/日志增强）——独立 change。
- 不引入 tcloud cc-info 的「关联资源 biz 跟随」能力（ziyan 现状没有，非本期目标）。

## Decisions

### D1: 路由函数 = 新入口本身，分类只做存在性判定

新入口逻辑函数（`HostWithCC`）自带轻量分类，f2 是其内联分支，f1 仅承接创建子集。**分类阶段只做存在性判定，不构建 CC 字段期望态**（理由见 D1.1）：

```
HostWithCC(kt, SyncHostParams{AccountID, BizID, HostIDs})     ← hc-service 新入口
 ├─ 1. getHostFromCCByHostIDs(hostIDs, HostFields)   【复用】不带 bizID，防窗口期误删
 ├─ 2. 提取 cloudID：BkCloudInstID ?: BkAssetID，双空为脏数据剔除（纯字段读，零 RPC）
 ├─ 3. listHostFromDBForDiff(hostIDs, ccCloudIDs)    【复用】bk_host_id + cloud_id 双维度
 ├─ 4. 按 cloud_id 配对做存在性分类（纯 map 查，零 RPC、零写库）
 │    ├─ CC有 DB无 → createIDs   → HostWithRelRes(createIDs)  【f1 原样复用，D2】
 │    ├─ CC无 DB有 → delCloudIDs → deleteHost(delCloudIDs)    【复用，取 DB 记录的 cloudID】
 │    ├─ CC有 DB有 → updateSet（ccHost + dbHost 配对）
 │    └─ CC无 DB无 → 跳过
 └─ 5. updateSet 非空才执行 f2（D3）：
      getHostBizID(仅 updateIDs) → convertToHost(仅 updateSet)
      → CC-only comparator 过滤无变化 → BatchUpdateCvm
```

按 `cloud_id` 配对与 `common.Diff` 的键一致：CC 重建（bk_host_id 变、cloud_id 不变）经双维度查询命中旧记录后判为更新，BkHostID 作为变化字段之一被更新。cloud_id 变化而 bk_host_id 不变的情形判为创建并遗留旧记录——与现状 `Host()` 行为一致，不在本期范围。

### D1.1: CC 期望态构建延后到更新分支

`convertToHost` + `getHostBizID` **不在分类阶段执行**，仅对更新子集执行。

- **分类不需要它们**：判定 CC有/无只需 cloudID（`BkCloudInstID ?: BkAssetID`，3 行字段读取）；删除分支的 cloudID 取自 DB 记录；创建分支交给 f1 自行构建。
- **性能收益有限但真实**：`getHostBizID` 按 500 分批，100 台桶无论传 100 还是 97 个 ID 都是同一次 RPC，稳态下省 0 次；`convertToHost` 是纯内存映射。**仅在更新集为空时**（全创建批次，如新业务上量）可整体跳过 `FindHostBizRelations`，3 次 RPC 省 1 次。
- **主要收益是失败隔离**：`FindHostBizRelations` 失败或主机无业务关系时，只影响更新分支；创建、删除分支不被牵连（删除本不需 biz，创建的 biz 由 f1 内部查询）。
- **代价**：不再整体复用 `common.Diff`，分类改为手写（约 20 行）。因本就需替换 comparator，复用价值有限。

**替代方案（未采纳）**：构建全量 CC 期望态后交 `common.Diff` 一次性完成分类 + 变化检测。代码更短，但创建子集的期望态被丢弃且浪费，更重要的是把 biz 查询失败的影响面扩大到整桶。

**为什么不按事件类型在 cloud-server 路由**：丢创建事件后更新事件会在 f2 打空（主机缺 6h）；事件乱序、CC 重建均无法处理。CC-vs-DB 路由对上述场景自愈（更新事件 + DB 无记录 → 判为创建 → 转 f1）。

**为什么路由放 hc-service 而非 cloud-server**：diff 需要 CC 当前态 + DB 全量字段，两者都是 hc-service 既有依赖；cloud-server 只持有事件快照（事件时刻，非当前态）。

**为什么不在 `HostWithRelRes` 内插分支**：保持完整链路控制流零改动（全量/主动触发共用），新增旁路比改造共用主干风险低。

**轻量性**：每 100 台桶 = 1×CC ListHost + 1×CC FindHostBizRelations + 1~2×DS List ≈ 3 次 RPC，零云调用、分类零写库——是 `Host()` 前半段既有成本，无法更轻。

### D2: 创建子集回调 f1，不传递路由快照

创建子集（稳态罕见）以 hostIDs 形式调 `HostWithRelRes`，f1 内部重新查 CC/云/DB 独立 diff。路由层的 CC 快照不给 f1 复用——f1 的 CC 查询不带 bizID、云查询按 region 分组等语义自包含，复用快照会污染其正确性假设。代价是创建子集多一次 CC 查询，可忽略。

### D3: f2 借鉴 tcloud `SyncCvmCCInfoByCond` 模式，但用完整 extension 回写规避零值覆盖

tcloud 模式三要点（`cmd/hc-service/logics/res-sync/cc-info/cvm_cc_info.go` + `service/sync/tcloud/cvm_cc_info.go`）：

1. DB 存量记录为输入基准；
2. 变化检测后才写（`if cvm.BkBizID == bizID { continue }`）；
3. 窄字段部分更新（`BatchUpdateCvmCommonInfo`，指针字段只写非 nil）。

ziyan f2 的适配与差异：

| 点 | tcloud | ziyan f2 |
|---|---|---|
| 更新字段集 | 仅 biz 归属 | **CC 权威字段**（见下）：基表（BkBizID/BkHostID/BkAssetID/BkCloudID/Region/OsName/MachineType/四组 IP）+ extension CC 槽位（HostName/SrvSourceTypeID/SrvStatus/SvrDeviceClass/BkDisk/BkCpu/BkOSName/Operator/BkBakOperator） |
| 写库接口 | `BatchUpdateCvmCommonInfo`（不支持 extension） | `BatchUpdateCvm`（支持 extension merge），**不新增 DS 接口** |
| 基表零值风险 | 指针字段天然规避 | DAO 仅更新非零字段（column_helper.go:313），CC-only payload 的零值云字段（Status/ImageID/VpcIDs…）不会落库 |
| extension 零值风险 | 不涉及 | **关键陷阱**：`UpdateMerge` 全键覆盖 → f2 从路由阶段已读出的 dbHosts 取现存 extension，仅覆盖 CC 槽位字段后整体回传，云槽位保持 DB 现值 |
| comparator | bizID 单字段 | `isHostChange` 的 CC 子集：剔除全部云权威字段（见下表），保留 CC 权威字段比较项 |

#### 字段归属的判定依据（实施阶段修正）

CC/云的字段归属不能只看「谁能提供」，而要看**现状全量链路的最终写入者**。`getCloudHost` 先 `convertToHost`（CC 值）再 `fillCloudFields`（云值覆盖），因此凡被 `fillCloudFields` 覆盖的字段，其权威源就是云：

| 归属 | 字段 | f2 行为 |
|---|---|---|
| 云权威（`fillCloudFields` 覆盖） | Name、Zone、ImageID、Status、CloudVpcIDs、VpcIDs、CloudSubnetIDs、SubnetIDs、CloudCreatedTime、CloudExpiredTime、extension.TCloudCvmExtension | 不比较、不下发 |
| CC 权威（`convertToHost` 设置且不被覆盖） | BkBizID、BkHostID、BkAssetID、BkCloudID、AccountID、Region、OsName、MachineType、Private/Public IPv4/v6、extension 的 9 个 CC 槽位 | 比较并下发 |

初稿把 **Name / Zone / CloudVpcIDs / CloudSubnetIDs** 划给了 CC。若按初稿实现，增量写 CC 值、全量写云值，两者会在同一批字段上来回覆盖（flapping）。修正后 CC 侧主机名的时效性由 `extension.HostName`（CC 权威，不被覆盖）承载，基表 `Name` 保持云权威，语义与现状完全一致。

`Status` 是 `CvmBatchUpdate` 上的 `validate:"required"` 字段，虽属云权威但不能留空，f2 回填 DB 现值以通过校验——写回原值不改变数据。

- 复用 `BatchUpdateCvm` 的另一固有行为：`recalcUpdateIsGPU` 以 DB 现存 MachineType/extension 兜底重算 is_gpu，CC-only payload 不改变其结果；CMDB upsert 对 ziyan skip，无副作用。
- 写库顺序：`BatchUpdateCvm` 经 D4 排序后，f1/f2 两路更新同批主机也不再死锁。

### D4: 死锁修复——cvm 批量更新入口在事务前按 ID 升序排序

`cmd/data-service/service/cloud/cvm/update.go` 中**每个开启事务的批量更新入口**在进入 `AutoTxn` 前，按 `ID` 升序重排待更新记录。共两处：`batchUpdateCvm`（vendor 泛型、带 extension）与 `BatchUpdateCvmCommonInfo`（跨 vendor、窄字段）。

- **原理**：所有并发事务以同一全局顺序（ID 升序）逐行加锁，等待图边恒从小 ID 指向大 ID，必然无环——破除死锁四必要条件中的「循环等待」。
- **与切批无关**：hc 侧按 100 切批，每批一个独立事务。只要每个事务内部升序，无论各进程如何切批，全局等待图都无环，因此无需在切批前排序。
- **为什么排序在 data-service 而非 hc-service**：不变量的作用域是事务，而事务开在 DS。放在调用侧则每个构造 payload 的路径都要各自记得——本改动新增的 f2 走 `convToCCOnlyUpdate` 而非 `convToUpdate`，若把排序挂在 `convToUpdate` 上，恰好漏掉「增量与全量并发更新同批主机」这一本期核心场景。
- **影响面声明**：两个入口都是跨 vendor 共用函数，排序对全部 vendor 生效——这是修复的固有属性，非范围蔓延。排序仅改变批内更新顺序，无业务语义（upsertCmdbHosts 对顺序无语义；ziyan 本就 skip）。
- **同时修 `BatchUpdateCvmCommonInfo`（实施阶段修正初稿决策）**：初稿判断「common info 路径无并发写同一批行的实证场景」，核对调用方后该判断不成立。`assign.go:110`「分配主机到业务」与本次新增的 ziyan 增量同步会写**同一批行的同一字段** `bk_biz_id`：用户分配 ziyan 主机到业务时走 `BatchUpdateCvmCommonInfo`，而该动作在 CC 侧产生的 biz 变更事件会触发 f2 走 `BatchUpdateCvm`。两个未排序事务、同一批行、顺序各自随机，正是死锁构成条件。只修一侧留不住不变量。
- **落地形态**：抽 `sortByIDForLockOrder[T any](items []T, idOf func(T) string)` 一个按意图命名的文件内 helper，由两个入口共用。不下沉到 `pkg/tools/slice`——通用 `SortBy` 会丢掉「为什么必须调」的语义，而这正是防止后续新增入口漏调的唯一提示。

### D5: 接口与调用处——新增 hc-service 内部接口，复用既有请求协议

新增的是 **hc-service 内部接口**（供 cloud-server watch goroutine 调用，对标既有内部接口 `SyncHostWithRelResByCond`），**不是主动触发接口**，与 cloud-server 的外部 API `sync_by_cond` 分属不同层，无重复：

```
主动触发（外部 API，已存在）
  POST /api/v1/cloud/[bizs/:biz/]vendors/:vendor/accounts/:id/resources/:res/sync_by_cond
   → accountSvc.SyncCloudResourceByCond → ziyanCondSyncRes → condSyncFuncMap[res]

增量 watch（内部调用，无外部 API）
  cloud-server watch goroutine → hc-service 内部接口   ← 本条新增
```

- hc-service 新增路由：`h.Add("SyncHostWithCCByCond", "POST", "/hosts/cc_info/by_condition/sync", v.SyncHostWithCCByCond)`（`cmd/hc-service/service/sync/tcloud-ziyan/ziyan.go`，对齐既有 `/hosts/...` 路径风格与 vendor `cc_info` 命名）。
- service 层 handler 复刻 `SyncHostWithRelResByCond` 模式：decode → validate → `svc.syncCli.TCloudZiyan(kt, req.AccountID)` → `HostWithCC`。
- 请求协议**复用** `sync.TCloudZiyanSyncHostByCondReq`（AccountID/BizID/HostIDs ≤100），零新增协议结构。
- `ziyan.Interface`（`cmd/hc-service/logics/res-sync/ziyan/client.go`）新增 `HostWithCC(kt, *SyncHostParams) (*SyncResult, error)`。
- `pkg/client/hc-service/tcloud-ziyan/cvm.go` 新增 `SyncHostWithCCByCond` 客户端方法。
- cloud-server 改动仅一处：`upsertTCloudZiyanHost`（`bkcc/host.go:336`）批量循环内的调用从 `SyncHostWithRelResByCond` 切换为 `SyncHostWithCCByCond`。

## Risks / Trade-offs

- **云字段时效降级** → 增量更新后 Status/机型配置/到期时间/安全组关系等云字段仅靠 6h 全量与主动触发刷新。已在 proposal「行为变更」声明，需周知使用方。承接方式无需新 API：往 ziyan `condSyncFuncMap`（`cmd/cloud-server/service/sync/tcloud-ziyan/cond_sync_resource.go:48`）注册 `CvmCloudResType` 指向完整链路即可复用既有 `sync_by_cond`——**该 map 目前只有 region/zone/image/lb/sg/deviceType，尚无 CVM**，是否本期补齐见 Open Questions。
- **路由 CC 快照与 f1 执行间的窗口**（路由判为创建、f1 执行时已被他人删除/创建）→ f1 内部独立 diff，最终一致；与现状全量链路窗口语义相同。
- **f2 extension 回写依赖路由阶段 DB 快照**，并发更新交错时为 last-write-wins → 与现状语义一致（现状 update 同样非原子合并）；死锁消除后并发失败率反而下降。
- **更新事件触发的 f2 不感知安全组关系变化** → 安全组关系仅由全量/主动触发同步，属本期目标的既定取舍（iWiki 风险节已列）。
- **排序对性能影响**：100 元素排序为纳秒级，可忽略。

## Migration Plan

1. 部署顺序：data-service（排序，独立无害）→ hc-service（新接口，无调用方时上线）→ cloud-server（切换调用处，生效点）。
2. 回滚：cloud-server 切回原调用即可整体回退增量行为；DS 排序与 hc 新接口可独立保留或回滚，互无依赖。
3. 无 DB schema 变更、无配置变更、无新增依赖。

## Resolved Decisions

- **按需刷新入口不纳入本期**：往 ziyan `condSyncFuncMap` 注册 `CvmCloudResType`（复用既有 `sync_by_cond` 外部 API）虽只需约 20 行，但超出本期两项目标，待使用方提出实时诉求后另立 change。本期「云字段时效降级」以周知方式承接。
- **`listHostFromDBForDiff` 采用抽核心 + 委托**：抽出按 `cloudIDs []string` 入参的核心形态，原函数（`Host()` 在用）委托调用。行为不变的 3 行改动，优于在新文件重复 12 行合并逻辑。

## Open Questions

- 新入口埋点（批次数/分流计数/耗时）是否随本期一并加——观测能力整体属独立 change，本期默认只在关键分流点补必要日志，不加 metrics。
- f2 更新时若主机业务转移（bizID 变化），关联的 disk/eip/NI biz 是否跟随（tcloud cc-info 有该能力，ziyan 现状无）——本期不做，待需求方确认时效要求后另立 change。
