# ziyan-host-incr-sync-cc-only

## Purpose

ziyan（tcloud-ziyan，自研云，腾讯云 SDK 内部域名）主机增量同步轻量入口。集成点：蓝鲸 CMDB（主机查询 ListHost / 业务关系 FindHostBizRelations / watch 事件）。

## Requirements

### Requirement: 增量入口按 CC 与 DB 比对路由处理类别

hc-service SHALL 提供新的增量同步入口（`SyncHostCCInfoByCond`），接收 cloud-server watch 增量事件下发的 hostIDs（单批 ≤100，含 AccountID/BizID），对每台主机以 CC 当前态与 DB 存量比对（bk_host_id + cloud_id 双维度）确定处理类别，MUST NOT 以事件类型或单看 DB 作为分类依据。

#### Scenario: 更新事件但 DB 无记录（创建事件曾丢失）

- **WHEN** 入口收到主机的更新事件且该主机 CC 有记录、DB 无记录
- **THEN** 该主机被判为创建类别，转完整链路补全云信息后创建

#### Scenario: 更新事件到达时主机已从 CC 删除（事件乱序）

- **WHEN** 入口收到主机的更新事件且该主机 CC 已无记录、DB 有记录
- **THEN** 该主机被判为删除类别，直接删除 DB 记录，不调用云 API

#### Scenario: CC 重建主机（bk_host_id 变更、cloud_id 不变）

- **WHEN** DB 中存在与新 hostID 不同 bk_host_id 但 cloud_id 相同的旧记录
- **THEN** 通过 cloud_id 维度命中旧记录判为更新，MUST NOT 误判为创建（避免 cloud_id+vendor 唯一键冲突）

### Requirement: 更新类别仅刷新 CC 来源字段

对判为更新的主机，入口 SHALL 仅更新 **CC 权威字段**——基表的 BkBizID/BkHostID/BkAssetID/BkCloudID/Region/OsName/MachineType/Private 与 Public IPv4/v6，以及 extension CC 槽位（HostName/SvrSourceTypeID/SrvStatus/SvrDeviceClass/BkDisk/BkCpu/BkOSName/Operator/BkBakOperator）。

入口 MUST NOT 调用云 API，MUST NOT 同步安全组关系与关联资源，且 MUST NOT 修改**云权威字段**——即全量链路中由 `fillCloudFields` 覆盖的 Name/Zone/ImageID/Status/CloudVpcIDs/VpcIDs/CloudSubnetIDs/SubnetIDs/CloudCreatedTime/CloudExpiredTime 及 extension 的 TCloudCvmExtension。

CC 侧主机名的时效性由 extension.HostName 承载；基表 Name 属云权威，由全量同步维护，两条链路不得写同一字段，否则会互相覆盖。

#### Scenario: CC 字段有变化

- **WHEN** 主机判为更新类别且任一 CC 权威字段与 DB 现值不同
- **THEN** 该主机的 CC 权威字段更新落库，云权威字段保持 DB 现值（含 extension 云槽位不被零值覆盖）

#### Scenario: CC 字段无变化

- **WHEN** 主机判为更新类别且全部 CC 权威字段与 DB 现值一致
- **THEN** 该主机不产生任何写库请求

### Requirement: 创建类别转完整链路

对判为创建的主机子集，入口 SHALL 调用既有 `HostWithRelRes` 完整链路（含云信息补全、关联资源与安全组关系同步），完整链路实现 MUST 保持原样不改动。

#### Scenario: 批量中混入创建主机

- **WHEN** 一批 100 台主机中 3 台 CC 有 DB 无、97 台为更新
- **THEN** 3 台走完整链路创建（含云字段），97 台仅刷新 CC 字段，整批零多余云调用

### Requirement: 全量同步与主动触发链路行为不变

全量同步（6h 定时）与用户主动触发的同步 MUST 继续走 `HostWithRelRes` 完整链路，行为与现状完全一致。

#### Scenario: 全量同步执行

- **WHEN** 6h 全量同步任务触发 ziyan 主机同步
- **THEN** 走 `HostWithRelRes` 完整链路，云字段、关联资源、安全组关系全部刷新

### Requirement: 批次失败隔离语义保持

入口对单批处理失败 MUST 保持现有语义：记录错误日志后继续后续批次，不重试、不阻断 cursor 提交。

#### Scenario: 单批处理失败

- **WHEN** 某批次在 CC 查询或写库环节报错
- **THEN** 记录含 hostIDs 与 rid 的错误日志，后续批次照常处理
