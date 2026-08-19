## 1. 云 API 预取（clb-sync 已落地，apply 时核对应保持）

- [x] 1.1 `listTargetsFromCloud`：`CloudIDs` 为空时不传 ListenerIds，一次拉取整台 LB
- [x] 1.2 `prefetchListenerTargets`：空 CloudIDs 调云，按 listener id 建 map；失败 Warn 并返回 nil
- [x] 1.3 `listenerOfLoadBalancer`：预取成功后各批用 `listTargetsFromCache` 切片，传入 `listener(..., prefetchedTargets)`
- [x] 1.4 `ListenerTargets` / `listTargetRelated`：`prefetchedTargets == nil` 才按批调云；非 nil 直接使用
- [x] 1.5 独立入口 `Listener()` 继续传 nil，不预取
- [x] 1.6 缓存 miss 只 Warn，不把 miss 当成空后端删除

## 2. DB 批量查 RS（clb-sync 已落地，apply 时核对应保持）

- [x] 2.1 `listTargetsFromDB`：rel 去重 TG 后按 `CloudResourceSyncMaxLimit` IN 批量 `ListTarget`，页 500 翻页
- [x] 2.2 无 RS 的在用 TG 在 map 中保留 nil/空切片
- [x] 2.3 现有 `load_balancer_target_group_test.go` 覆盖云拉取、DB 批量、预取与缓存切片

## 3. 监听器数量阈值配置（本轮 apply 的主要工作）

- [x] 3.1 在 `pkg/cc/types.go` 的 `SyncConfig` 增加 `TargetsPrefetchMaxListeners uint`，yaml 字段 `targetsPrefetchMaxListeners`
- [x] 3.2 `trySetDefault`：值为 0 时设为 10000
- [x] 3.3 `cmd/hc-service/etc/hc_service.yaml` 与 `docs/support-file/helm/values.yaml` 的 `sync` 段补该字段及注释（默认 10000；说明监听器与 RS 一般不超过 1:2）
- [x] 3.4 `listenerOfLoadBalancer`：`len(cloudListeners) > 阈值` 时跳过预取，打 Info（lb、listeners、rid）；`prefetchedTargets` 保持 nil 以回退按批拉取
- [x] 3.5 跳过预取处写中文注释：监听器与 RS 一般不超过 1:2，用监听器数量近似约束预取内存；禁止再查 RS 数量
- [x] 3.6 阈值读取 `cc.HCService().SyncConfig.TargetsPrefetchMaxListeners`，只改 `listenerOfLoadBalancer`，不改 `Listener()`，不做顺带重构

## 4. 测试与核对

- [x] 4.1 单测：监听器数超过阈值不调用整台预取；等于阈值仍预取；`Listener()` 路径仍不预取
- [x] 4.2 跑 `go test`：`cmd/hc-service/logics/res-sync/ziyan/` 相关测试通过
- [x] 4.3 自查 diff：核心同步文件以新增行为主；不改 tcloud 非 ziyan、不改对外 API、不改并发配置
