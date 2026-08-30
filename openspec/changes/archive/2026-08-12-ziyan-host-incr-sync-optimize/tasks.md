# Tasks

## 1. data-service：BatchUpdateCvm 死锁消除（独立，可先行）

- [x] 1.1 `cmd/data-service/service/cloud/cvm/update.go`：抽 `sortByIDForLockOrder[T any](items []T, idOf func(T) string)` 文件内 helper（按意图命名，注释说明「每个开启事务的批量更新入口都必须在进入事务前调用」），置于两个 handler 共用 helper 区
- [x] 1.2 `batchUpdateCvm` 泛型函数在进入 `AutoTxn` 前调用该 helper 排序 `req.Cvms`（排序即全部改动，不动事务内逻辑）
- [x] 1.3 `BatchUpdateCvmCommonInfo` 同样在进入 `AutoTxn` 前排序 `req.Cvms`——它与 `assign.go:110` 分配业务路径会和增量同步写同一批行的 `bk_biz_id`，只修一侧留不住不变量
- [x] 1.4 单测：乱序输入验证两个入口排序后产出同一顺序，且单台更新内容（extension merge、is_gpu 重算、非零字段过滤）与排序前等价

## 2. hc-service 逻辑层：ziyan 路由函数与 CC-only 更新

- [x] 2.1 `listHostFromDBForDiff` 抽出按 `cloudIDs []string` 入参的核心形态，原函数委托调用（行为不变的微重构，供路由在未构建 CC 期望态时使用）
- [x] 2.2 `cmd/hc-service/logics/res-sync/ziyan/` 新增 `host_with_cc.go`：`HostWithCC(kt, *SyncHostParams) (*SyncResult, error)` 分类阶段——`getHostFromCCByHostIDs`（不带 bizID）→ 提取 cloudID（`BkCloudInstID ?: BkAssetID`，双空脏数据剔除）→ `listHostFromDBForDiff` 双维度查 DB → 按 cloud_id 配对做存在性分类（零 RPC、零写库，**不构建 CC 期望态**）
- [x] 2.3 分流落地：创建子集以 hostIDs 回调 `HostWithRelRes`（原样复用）；删除子集用 DB 记录 cloudID 复用 `deleteHost`；CC 无 DB 无跳过
- [x] 2.4 f2 更新分支（仅 updateSet 非空时执行）：`getHostBizID` 仅传更新子集 hostIDs → `convertToHost` 仅转更新子集 → CC-only comparator 过滤 → 写库
- [x] 2.5 CC-only comparator：基于 `isHostChange` 剔除云来源比较项（Status/ImageID/CloudImageID/CloudCreatedTime/CloudExpiredTime/VpcIDs/SubnetIDs/TCloudCvmExtension 全体），仅保留 CC 字段比较
- [x] 2.6 CC-only 更新构造（`convToCCOnlyUpdate`）：基表仅填 CC 字段；extension 取分类阶段读出的 DB 现值、仅覆盖 CC 槽位字段后整体回传（规避 `json.UpdateMerge` 全键覆盖云槽位）；复用 `updateHost` 分批下发
- [x] 2.7 `cmd/hc-service/logics/res-sync/ziyan/client.go`：`ziyan.Interface` 注册 `HostWithCC`
- [x] 2.8 单测：路由分类矩阵（CC有DB无→创建 / CC有DB有→更新 / CC无DB有→删除 / 双无→跳过 / CC 重建 cloud_id 命中旧记录→更新）；全创建批次不触发 `FindHostBizRelations`；extension 云槽位零值覆盖防护；无变化不写库

## 3. hc-service service 层与客户端

- [x] 3.1 `cmd/hc-service/service/sync/tcloud-ziyan/host.go`：新增 `SyncHostWithCCByCond` handler（复刻 `SyncHostWithRelResByCond` 模式：decode → validate → `svc.syncCli.TCloudZiyan` → `HostWithCC`），请求协议复用 `sync.TCloudZiyanSyncHostByCondReq`
- [x] 3.2 `cmd/hc-service/service/sync/tcloud-ziyan/ziyan.go`：注册路由 `h.Add("SyncHostWithCCByCond", "POST", "/hosts/cc_info/by_condition/sync", v.SyncHostWithCCByCond)`
- [x] 3.3 `pkg/client/hc-service/tcloud-ziyan/cvm.go`：新增 `SyncHostWithCCByCond` 客户端方法（走 `pkg/client/common/request.go` 封装）

## 4. cloud-server 调用切换

- [x] 4.1 `cmd/cloud-server/service/watch/bkcc/host.go`：`upsertTCloudZiyanHost` 批量循环内调用从 `SyncHostWithRelResByCond` 切换为 `SyncHostWithCCByCond`（仅 ziyan 一个 case，其余 vendor 与删除路径不动）

## 5. 验证

- [x] 5.1 编译通过，涉及包单测全绿（data-service cvm、res-sync/ziyan、sync/tcloud-ziyan）
- [ ] 5.2 测试环境验证：增量更新批次无 ListCvm/关联资源/SG 关系调用日志；创建主机仍走完整链路（云字段齐全）；构造并发增量+全量更新同批主机无 Error 1213；构造「分配主机到业务」与增量同步并发写同批主机无 Error 1213
