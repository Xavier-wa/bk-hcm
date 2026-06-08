## 1. CRP 客户端扩展

- [x] 1.1 在 `pkg/thirdparty/cvmapi/constvar.go` 新增常量 `QueryCvmTypeListMethod = "queryCvmTypeList"`
- [x] 1.2 在 `pkg/thirdparty/cvmapi/cvmapi_request.go` 新增 `QueryCvmTypeListReq` 与 `QueryCvmTypeListParams` 结构体（`Params` 含 `DeptName string` 字段）
- [x] 1.3 在 `pkg/thirdparty/cvmapi/cvmapi_response.go` 新增 `QueryCvmTypeListResp` 与 `CvmTypeItem`。`CvmTypeItem` 完整暴露 CRP 接口契约字段（`CvmInstanceModel`、`CvmInstanceGroup`、`CvmInstanceType`、`CpuAmount`、`RamAmount`、`DiskBlockNum`、`DiskBlockSize`、`GpuType`、`GpuCard`、`TechnicalClass`、`TechnicalUnit`、`TechnicalAmount`、`DeviceFamily`、`CoreTypeName`），便于后续场景按需消费；同步任务本期仅使用 `CvmInstanceModel` + `DeviceFamily` 两个字段写入本地表
- [x] 1.4 在 `pkg/thirdparty/cvmapi/cvmapi.go` 的 `CVMClientInterface` 接口中新增 `QueryCvmTypeList(kt *kit.Kit, params *QueryCvmTypeListParams) (*QueryCvmTypeListResp, error)` 方法
- [x] 1.5 在 `pkg/thirdparty/cvmapi/cvmapi.go` 的 `cvmApi` struct 上实现 `QueryCvmTypeList` 方法，参考 `QueryCvmInstanceType` 的模式：`subPath="/yunti-demand/external"` + `WithParam(CvmApiKey, CvmApiKeyVal)` + `WithHeaders(kt.Header())`，并对 HTTP 错误与 `resp.Error.Code != 0` 两种异常分别打日志并返回 error
- [x] 1.6 在 `cmd/woa-server/logics/plan/dispatcher/crp_split_test.go` 的 `mockCRPClient` 上补齐 `QueryCvmTypeList` 方法（`panic("unexpected")` 占位）

## 2. CronTask 枚举值

- [x] 2.1 在 `pkg/criteria/enumor/cron_task.go` 新增常量 `CronTaskSyncDeviceTypePhysicalRel CronTask = "sync_device_type_physical_rel"`

## 3. 同步任务核心实现

- [x] 3.1 创建 `cmd/woa-server/task/sync_device_type_physical_rel.go`，定义 `SyncDeviceTypePhysicalRelTask` struct（字段：`clientSet *client.ClientSet`、`crpCli cvmapi.CVMClientInterface`、`sd serviced.State`）
- [x] 3.2 实现 `NewSyncDeviceTypePhysicalRelTask(clientSet, crpCli, sd) (croncore.Task, error)` 构造函数
- [x] 3.3 实现 `Name() string` 返回 `string(enumor.CronTaskSyncDeviceTypePhysicalRel)`
- [x] 3.4 实现 `Next() (time.Time, error)`，返回 `time.Now().Add(time.Duration(cc.WoaServer().ResourceSync.SyncDeviceTypePhysicalRel.Interval) * time.Minute)`
- [x] 3.5 实现 `GetURL() string` 返回 `"/device_type_physical_rels/sync"`
- [x] 3.6 实现 `Do(kt *kit.Kit) error` 主流程框架：master 检查 → 调用 `fetchCrpSnapshot` → `loadLocalSnapshot` → `diffSnapshots` → 顺序执行 `applyCreate` → `applyUpdate`（本期**不调用 `applyDelete`**，避免误删一度支持生产但被 CRP 下线的机型映射）
- [x] 3.7 实现私有方法 `fetchCrpSnapshot(kt) (map[string]string, error)`：调用 `crpCli.QueryCvmTypeList(kt, &cvmapi.QueryCvmTypeListParams{})`（**不传 `DeptName`**，尽可能查到更多机型）、对单条记录字段为空的过滤并打 warn、对返回 0 条情形打 error 日志并返回 nil map、返回去重后的 map（key=cvmInstanceModel, value=deviceFamily）。空字段过滤抽出为独立函数 `buildSnapshotFromCvmTypeItems(kt, items)` 便于单测
- [x] 3.8 实现私有方法 `loadLocalSnapshot(kt) (map[string]localRecord, error)`：分页调用 `clientSet.DataService().Global.ResourcePlan.ListWoaDeviceTypePhysicalRel` 取出本地表全量，返回 map（key=device_type, value={id, physical_device_family}）
- [x] 3.9 实现私有方法 `diffSnapshots(crpMap, localMap) (toCreate, toUpdate)` 计算两段 diff：`toCreate` 为 CRP 有 / 本地没有；`toUpdate` 为双方都有但 `physical_device_family` 不同（key=本地 id，value=新值）
- [x] 3.10 实现私有方法 `applyCreate(kt, toCreate)`、`applyUpdate(kt, toUpdate)`，各自按 `constant.BatchOperationMaxLimit` 分批调用 data-service 接口，单步失败立即 return（不回滚已写入）
- [x] 3.11 同时实现私有方法 `applyDelete(kt, toDelete []string)`，**但本期不在 `Do` 主流程中调用**，仅作为后续可能的运维清理工具复用入口保留
- [x] 3.12 在所有失败路径加 `logs.Errorf` 告警，在成功结束加 `logs.Infof("sync device type physical rel done, add: %d, update: %d, rid: %s", len(toCreate), len(toUpdate), kt.Rid)`

## 4. 配置项

- [x] 4.1 在 `pkg/cc/ziyan_types.go`（`ResourceSync` 配置结构所在文件）新增子结构 `SyncDeviceTypePhysicalRel { Interval int }`
- [x] 4.2 在 `cmd/woa-server/etc/woa_server.yaml` 增加配置块 `resourceSync.syncDeviceTypePhysicalRel.interval: 1440`
- [x] 4.3 同步更新 `docs/support-file/helm/values.yaml` 加入同名配置项（configmap 通过 `toYaml` 自动渲染整个 resourceSync 块）
- [x] 4.4 校验配置项加载链路（在 `WoaServerSetting.trySetDefault` 中调用 `SyncDeviceTypePhysicalRel.trySetDefault`，缺省时使用 1440）

## 5. 注册 CronTask

- [x] 5.1 在 `cmd/woa-server/service/service.go::initCronTask` 调用 `crontask.NewSyncDeviceTypePhysicalRelTask(s.client, s.thirdCli.CVM, s.sd)` 创建实例
- [x] 5.2 将实例存入 `s.tasks[enumor.CronTaskSyncDeviceTypePhysicalRel]`
- [x] 5.3 将实例追加到 `cron.Register([...])` 的 task 列表
- [ ] 5.4 验证 woa-server 启动日志显示 task 已被注册（需运行时验证，单元测试不覆盖）

## 6. 手动触发 API

- [x] 6.1 创建 `cmd/woa-server/service/res-sync/sync_device_type_physical_rel.go`，实现 `SyncDeviceTypePhysicalRel(cts *rest.Contexts) (any, error)` handler
- [x] 6.2 在 handler 中执行 IAM 权限校验：`authorizer.AuthorizeWithPerm(cts.Kit, {Type: meta.GlobalConfig, Action: meta.Create})`
- [x] 6.3 在 handler 中通过 `s.tasks[enumor.CronTaskSyncDeviceTypePhysicalRel].Do(cts.Kit)` 调用 task 主流程
- [x] 6.4 在 `cmd/woa-server/service/res-sync/service.go::initService` 新增路由 `h.Add("SyncDeviceTypePhysicalRel", http.MethodPost, s.tasks[enumor.CronTaskSyncDeviceTypePhysicalRel].GetURL(), s.SyncDeviceTypePhysicalRel)`

## 7. 自测与单元测试

- [x] 7.1 为 `SyncDeviceTypePhysicalRelTask` 关键纯逻辑编写单测：`buildSnapshotFromCvmTypeItems` 覆盖单条字段空与去重场景；`diffSnapshots` 覆盖各种 diff 场景；任务元信息（Name/GetURL）；`Do` 中涉及 ClientSet 数据访问层的失败路径需要依赖大量 mock，留作运行时验证（不在本期单元测试覆盖）
- [x] 7.2 为 `diffSnapshots` 编写单测，覆盖：纯新增、纯更新、混合、完全一致、双方空、CRP 缺失本地存在（应被忽略，不删除）
- [ ] 7.3 本地启动 woa-server 验证定时任务在 master 节点触发（observe log）— 待运行时验证
- [ ] 7.4 本地用 `curl` 调用手动 API 触发同步并验证本地表数据：CRP 中存在的所有机型映射都已落库；本地原有的、CRP 中未返回的机型映射保留不变 — 待运行时验证
- [ ] 7.5 验收用例 AC-001：CRP 返回包含 `SA4t.32XLARGE576` 的记录时，本地表存在 `device_type=SA4t.32XLARGE576, physical_device_family=云上计算标准` 的记录 — 待运行时验证
- [ ] 7.6 验收用例 AC-002：用户发起退机/退回主机申请时，主机回收链路成功根据本地映射归还到对应物理机机型族库存池 — 待运行时验证
- [ ] 7.7 验收用例 AC-004：CRP 返回 HTTP 5xx 时，本地表数据保持上次成功同步的快照不变，告警通道收到失败告警 — 待运行时验证

## 8. 文档与发布

- [x] 8.1 新增接口文档 `docs/api-docs/web-server/docs/scr/resource-plan/sync_device_type_physical_rel.md`，标注路径 `POST /api/v1/woa/res_syncs/device_type_physical_rels/sync` 提供版本 `v9.9.9+`
- [x] 8.2 在 `docs/reqs/CRP机型族映射.md` 末尾补充"实现位置"章节，列出全部新增/修改文件链接
- [x] 8.3 运行 `openspec validate add-crp-device-type-sync --strict` 校验通过（输出 `Change 'add-crp-device-type-sync' is valid`）
- [ ] 8.4 代码审查（重点：异常处理路径、master 检查、批次大小、日志规范）— 待人工审查
