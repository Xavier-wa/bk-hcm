## ADDED Requirements

### Requirement: CRP 机型映射数据拉取

系统 SHALL 提供从 CRP `queryCvmTypeList` 接口拉取虚拟机机型与物理机机型族映射数据的能力，作为本地映射表的权威数据源。

#### Scenario: 成功拉取 CRP 全量映射快照

- **WHEN** 同步任务以空 `params`（不传 `deptName`）调用 CRP `queryCvmTypeList` 接口且 CRP 返回 N 条记录（N > 0）
- **THEN** 系统 SHALL 提取每条记录的 `cvmInstanceModel` 与 `deviceFamily` 字段构建内存映射快照

#### Scenario: 单条记录字段不完整

- **WHEN** CRP 返回的某条记录的 `cvmInstanceModel` 或 `deviceFamily` 字段为空字符串
- **THEN** 系统 SHALL 跳过该记录、记录 `logs.Warnf` 告警日志，并继续处理其他记录

#### Scenario: CRP HTTP 请求失败

- **WHEN** 调用 CRP 接口出现 HTTP 5xx 错误、连接超时或网络异常
- **THEN** 系统 SHALL 记录 `logs.Errorf` 告警、立即终止本次同步、保留本地表上次成功同步的数据不变

#### Scenario: CRP 返回业务错误码

- **WHEN** CRP 接口返回 HTTP 200 但响应体中 `error.code` 字段非 0
- **THEN** 系统 SHALL 记录错误码与错误信息、立即终止本次同步、保留本地表上次成功同步的数据不变

#### Scenario: CRP 返回空数据集

- **WHEN** CRP 接口成功返回但 `result` 数据集为 0 条（疑似异常）
- **THEN** 系统 SHALL 记录 `logs.Errorf` 告警、不进入 diff 阶段、不发起任何本地表的写操作（保留本地表上次成功同步的数据不变）

### Requirement: 本地映射表增量同步

系统 SHALL 采用两段式 diff 策略（仅新增 + 更新，不删除）将 CRP 数据增量同步到本地 `woa_device_type_physical_rel` 表。本期不对本地存在但 CRP 不再返回的映射执行删除，以保证已用于回收链路的历史机型映射可持续命中。

#### Scenario: CRP 新增机型映射

- **WHEN** CRP 返回的某个 `cvmInstanceModel` 在本地表中不存在
- **THEN** 系统 SHALL 通过 `BatchCreateWoaDeviceTypePhysicalRel` 接口写入新记录，`device_type` 字段值取 CRP 的 `cvmInstanceModel`，`physical_device_family` 字段值取 CRP 的 `deviceFamily`

#### Scenario: CRP 修改机型映射

- **WHEN** CRP 返回的某个 `cvmInstanceModel` 在本地表中已存在，但 `deviceFamily` 与本地 `physical_device_family` 不一致
- **THEN** 系统 SHALL 通过 `BatchUpdateWoaDeviceTypePhysicalRel` 接口更新该记录的 `physical_device_family` 字段为 CRP 最新值

#### Scenario: CRP 不再返回的机型保留本地映射

- **WHEN** 本地表中存在某 `device_type`，但 CRP 本次同步的快照中没有该 `cvmInstanceModel`
- **THEN** 系统 SHALL 在同步过程中跳过该记录（不调用任何删除接口），本地表中的该映射保持不变；如需清理由运维通过 SQL 等外部手段人工处理

#### Scenario: CRP 数据与本地完全一致

- **WHEN** 本地表内容与 CRP 快照中相同的 `device_type` 完全一致且无新增机型
- **THEN** 系统 SHALL 不发起任何写操作并记录成功日志

#### Scenario: BatchCreate / BatchUpdate 执行失败

- **WHEN** 两段式 diff 中 `applyCreate` 或 `applyUpdate` 阶段的 data-service 调用失败
- **THEN** 系统 SHALL 记录 `logs.Errorf` 告警并立即终止本次同步，已成功执行的部分写入保留（不回滚），下一次同步会自然收敛

#### Scenario: 同步成功结束

- **WHEN** 两段式 diff 全部成功执行完毕
- **THEN** 系统 SHALL 记录 `logs.Infof` 成功日志，格式为 `sync device type physical rel done, add: <toCreate 计数>, update: <toUpdate 计数>, rid: <rid>`

### Requirement: CRP 数据格式约束

系统 SHALL 严格按照 CRP 接口契约消费数据，结构体中保留 CRP 完整字段以便后续按需扩展，本期同步链路仅写入 `cvmInstanceModel` 与 `deviceFamily` 两个字段且保留原样字符串不做转换。

#### Scenario: 结构体保留 CRP 完整字段

- **WHEN** CRP 返回的记录包含 `cvmInstanceModel`、`cvmInstanceGroup`、`cvmInstanceType`、`cpuAmount`、`ramAmount`、`diskBlockNum`、`diskBlockSize`、`gpuType`、`gpuCard`、`technicalClass`、`technicalUnit`、`technicalAmount`、`deviceFamily`、`coreTypeName` 等字段
- **THEN** 系统 SHALL 在 `CvmTypeItem` 结构体中按 CRP 字段定义完整接收所有字段，便于后续场景按需消费

#### Scenario: 同步任务仅消费两个字段写入本地表

- **WHEN** 同步任务从 `CvmTypeItem` 列表构建本地表写入数据
- **THEN** 系统 SHALL 仅消费 `cvmInstanceModel` 与 `deviceFamily` 两个字段，其余字段不参与本地表的新增 / 更新

#### Scenario: 字符串大小写不做归一化

- **WHEN** CRP 返回的 `cvmInstanceModel` 字段值为 `SA4t.32XLARGE576`
- **THEN** 系统 SHALL 按原样字符串写入本地表 `device_type` 字段，HCM 侧不做大小写转换

#### Scenario: 部门参数空传

- **WHEN** 同步任务调用 CRP `queryCvmTypeList` 接口
- **THEN** 系统 SHALL 传入 `params=&QueryCvmTypeListParams{}`（即不传 `deptName`），由 CRP 返回全量机型映射，以避免按部门过滤导致回收链路所需的部分机型族信息缺失

### Requirement: 定时同步任务

系统 SHALL 提供按可配置周期自动执行的定时同步任务，仅在 master 节点执行以避免多节点重复写入。

#### Scenario: 定时触发同步

- **WHEN** 距上一次同步任务执行已超过 `cc.WoaServer().ResourceSync.SyncDeviceTypePhysicalRel.Interval` 配置的分钟数
- **THEN** `pkg/cron` 框架 SHALL 触发 `SyncDeviceTypePhysicalRelTask.Do(kt)` 方法

#### Scenario: 非 master 节点跳过执行

- **WHEN** 定时调度触发时当前节点的 `serviced.State.IsMaster()` 返回 false
- **THEN** 任务 SHALL 记录 `logs.V(5).Infof` 日志并直接返回，不执行任何同步动作

#### Scenario: 默认每日 1 次

- **WHEN** 运维未在 `woa_server.yaml` 中显式配置同步周期
- **THEN** 系统 SHALL 使用默认配置 `Interval=1440`（即每日 1 次）

#### Scenario: 任务注册到 cron 调度

- **WHEN** woa-server 启动并执行 `initCronTask`
- **THEN** 系统 SHALL 通过 `cron.Register` 注册 `SyncDeviceTypePhysicalRelTask` 实例，使其参与统一调度

### Requirement: 手动触发同步 API

系统 SHALL 提供 HTTP POST 接口，允许具有相应权限的运维人员立即触发一次同步，复用定时任务的同步逻辑。

#### Scenario: 运维成功触发同步

- **GIVEN** 调用方持有 `meta.GlobalConfig` 资源 `meta.Create` action 权限
- **WHEN** 调用方向 woa-server 发送 `POST /api/v1/woa/res_sync/device_type_physical_rels/sync` 请求
- **THEN** 系统 SHALL 立即执行一次完整同步流程并在同步完成后返回 HTTP 200

#### Scenario: 权限不足拒绝触发

- **GIVEN** 调用方不持有 `meta.GlobalConfig` 资源 `meta.Create` action 权限
- **WHEN** 调用方调用手动触发 API
- **THEN** 系统 SHALL 拒绝请求并返回权限错误，不执行任何同步动作

#### Scenario: 手动 API 路径与 cron 任务绑定

- **WHEN** 注册手动触发 API 路由时
- **THEN** 系统 SHALL 通过 `s.tasks[enumor.CronTaskSyncDeviceTypePhysicalRel].GetURL()` 取得路由子路径，确保路径与 task 实现保持一致

#### Scenario: 手动触发与定时执行无差异

- **WHEN** 运维通过手动 API 触发同步
- **THEN** 同步流程的实现 SHALL 完全等同于定时调度（同 master 节点行为、同三段式 diff、同异常处理策略）

### Requirement: 异步任务的可观测性

系统 SHALL 通过结构化日志暴露同步任务的关键事件，便于运维通过日志平台进行排障和告警。

#### Scenario: 同步开始记录

- **WHEN** 同步任务（定时或手动）开始执行
- **THEN** 系统 SHALL 记录 `logs.Infof` 包含触发来源（cron / manual）与 `rid` 的日志

#### Scenario: 同步成功记录

- **WHEN** 同步任务成功完成
- **THEN** 系统 SHALL 记录 `logs.Infof` 日志，格式为 `sync device type physical rel done, add: <toCreate 计数>, update: <toUpdate 计数>, rid: <rid>`

#### Scenario: 同步失败告警

- **WHEN** 同步任务因任何原因失败（HTTP 错误、业务错误码、0 条保护、data-service 调用失败）
- **THEN** 系统 SHALL 记录 `logs.Errorf` 包含失败原因与 `rid` 的日志，由日志平台基于关键字触发告警通知

### Requirement: 业务读路径不受同步任务影响

系统 SHALL 保证主机回收链路（短租到期、用户退机、主动退机申请）始终从本地表读取映射数据，与 CRP 状态完全解耦。

#### Scenario: CRP 不可用时业务正常

- **GIVEN** 本地表 `woa_device_type_physical_rel` 已有上一次成功同步的数据
- **WHEN** CRP 接口持续不可用且最近一次同步任务失败
- **THEN** 主机回收业务流程 SHALL 仍然能够基于本地表的旧快照正常进行机型族匹配

#### Scenario: 业务读不直接调用 CRP

- **WHEN** 主机回收链路（`short-rental.ListDeviceTypeFamily`）需要查询机型族映射
- **THEN** 系统 SHALL 仅查询本地 `woa_device_type_physical_rel` 表，不直接调用 CRP 接口

#### Scenario: 本地表无匹配走兜底

- **WHEN** 主机回收链路查询某虚拟机机型的物理机机型族时该机型在本地表中不存在
- **THEN** 业务流程 SHALL 按当前已有兜底逻辑处理（不在本期改造范围）
