## Why

HCM 主机回收链路（用户退机、短租到期、主动退机申请）依赖"虚拟机机型 → 物理机机型族"映射来判断退回的资源应归并到哪个物理机库存池。该映射当前**靠人工录入维护**，CRP 侧新增/调整机型时 HCM 无法自动同步，导致退机流程被卡住等待人工补录。CRP 已提供 `queryCvmTypeList` 接口，HCM 需对接并实现自动同步，彻底替换人工录入流程。

## What Changes

- 在 `pkg/thirdparty/cvmapi/` 新增 `QueryCvmTypeList` 接口对接（按现有 cvmapi 模式扩展，无新增鉴权）
- 新增 cron 同步任务 `SyncDeviceTypePhysicalRelTask`，每日 1 次全量拉取 CRP 数据并以"两段式 diff（toCreate / toUpdate）"方式增量更新本地 `woa_device_type_physical_rel` 表；本期**不做删除**（避免某机型一度支持生产又被下线时把映射关系一并删掉，回收链路查不到映射）
- 新增手动触发 API（复用 task `Do()` 方法），运维可绕过定时调度立即触发同步
- 同步过程中任何异常（HTTP 失败 / JSON-RPC 错误 / 返回 0 条）均**保留本地表上次成功的快照不变**，仅 `logs.Errorf` 落告警
- 业务读路径（`short-rental.ListDeviceTypeFamily`）已是查本地表，无需改造

## Capabilities

### New Capabilities

- `crp-device-type-sync`: 从 CRP 同步虚拟机机型到物理机机型族的映射数据，包含定时同步、手动触发、异常保护、两段式增量更新（仅新增/更新，不删除）等能力。

### Modified Capabilities

<!-- 无 -->

## Impact

**新增/修改文件**：

- `pkg/thirdparty/cvmapi/{constvar,cvmapi_request,cvmapi_response,cvmapi}.go` — 新增 `QueryCvmTypeList` 方法
- `pkg/criteria/enumor/cron_task.go` — 新增 `CronTaskSyncDeviceTypePhysicalRel` 枚举值
- `cmd/woa-server/task/sync_device_type_physical_rel.go` — **新增**，承载同步主逻辑（参考 `cmd/woa-server/task/device_capacity.go`）
- `cmd/woa-server/service/res-sync/sync_device_type_physical_rel.go` — **新增**，手动触发 API handler
- `cmd/woa-server/service/res-sync/service.go` — 通过 `s.tasks[XXX].GetURL()` 注册路由
- `cmd/woa-server/service/service.go` — 在 `initCronTask` 注册新 task
- `cmd/woa-server/etc/woa_server.yaml` + `docs/support-file/helm/...` — 新增 `resourceSync.syncDeviceTypePhysicalRel.interval` 配置项
- `cmd/woa-server/logics/plan/dispatcher/crp_split_test.go` — mockCRPClient 补齐新方法

**外部依赖**：

- CRP `queryCvmTypeList` 接口（已由 CRP 侧承诺并提供 curl 示例与字段说明）
- 复用现有 `woa_device_type_physical_rel` 表（SQLVER=0055），无 SQL/DAO 改造
- 复用现有 `cvmapi` HTTP 客户端、`pkg/cron` 调度框架、`pkg/cmsi` 日志告警通道

**风险**：

- 同步窗口（≤ 5 分钟）内业务读到半新半旧状态 → 通过两段式 diff 控制单次写入量；业务读容错（map 没 key 走兜底）
- 多节点重复执行 → `pkg/cron` 框架内置 `sd.IsMaster()` 检查
- CRP 返回 0 条疑似异常 → 显式保护：告警 + 不写表
- CRP 已下线的机型映射本地仍保留 → 业务可用性优先，不做删除；如需清理由运维通过 SQL 人工处理

**不在本期范围**：

- 资源申请/选型链路使用映射、`device_capacity` 库存视图直接消费 CRP 字段
- 短租新购、置换、滚动服务器等非回收场景
- CRP 扩展字段（cpu/ram/gpu/technicalClass 等）入库使用（已在 `CvmTypeItem` 结构体中暴露，但同步任务本期仅消费 `cvmInstanceModel` 与 `deviceFamily` 写入本地表）
- 多部门 / 多业务的 `deptName` 动态切换（本期统一不传部门参数，由 CRP 返回全量）
- 软删除/归档保留 CRP 中已下线的机型映射、自动删除 CRP 已下线的机型映射
