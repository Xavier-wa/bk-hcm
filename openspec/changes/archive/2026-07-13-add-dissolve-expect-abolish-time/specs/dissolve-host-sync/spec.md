## MODIFIED Requirements

### Requirement: 主机字段一次性补全落库

同步任务 SHALL 在写库前补全主机全部字段：裁撤系统直接提供的 `project_id`、`asset_id`、`inner_ip`、`module`、`abolish_phase`、`project_name`、`cpu_core`（取 `cpuLogicCoreNum`）、`expect_abolish_time`（取 `expectAbolishTime`）直接落库；`region` SHALL 由设备 `availabilityZoneName` 匹配 `woa_zone.zone_name` 得到对应 `region_id`；`bk_biz_id`、`group_id`、`operators` SHALL 按「CC 优先、ES 兜底」规则补全。

#### Scenario: cpu_core 取自裁撤系统

- **WHEN** 同步处理一条设备
- **THEN** 系统 SHALL 将设备 `cpuLogicCoreNum` 作为 `cpu_core` 落库，不调用 CC 获取 `bk_cpu`

#### Scenario: expect_abolish_time 取自裁撤系统

- **WHEN** 同步处理一条设备
- **THEN** 系统 SHALL 将设备 `expectAbolishTime` 作为 `expect_abolish_time` 落库；设备未提供该值时落库为空串

#### Scenario: region 由 woa_zone 映射

- **WHEN** 设备 `availabilityZoneName` 命中 `woa_zone.zone_name`
- **THEN** 系统 SHALL 将对应 `woa_zone.region_id` 写入 `region`
- **WHEN** `availabilityZoneName` 未命中任何 `woa_zone.zone_name`
- **THEN** 系统 SHALL 将 `region` 置空并继续处理该主机

#### Scenario: CC 命中时填充业务字段

- **WHEN** 按 `asset_id` 批量查询 CC 命中主机
- **THEN** 系统 SHALL 用 CC 的业务 ID、组织 ID(`bk_oper_grp_name_id`)、负责人(`operator`+`bk_bak_operator`) 填充 `bk_biz_id`/`group_id`/`operators`，并与 DB 旧值比较，有变化则更新

#### Scenario: CC 未命中且旧值已有时保留

- **WHEN** CC 未命中某主机，且 DB 旧记录的 `bk_biz_id`/`group_id`/`operators` 已有值
- **THEN** 系统 SHALL 跳过该主机的业务字段补全，保留 DB 旧值

#### Scenario: CC 未命中且旧值为空时 ES 兜底

- **WHEN** CC 未命中某主机，且 DB 旧记录这些字段无值
- **THEN** 系统 SHALL 按 `originDate` 定位 ES 快照索引 `app_device_pass_dtl_{originDate}`，补齐 `bk_biz_id`/`group_id`/`operators` 后更新

### Requirement: 按项目与固资号复合键 diff

同步任务 SHALL 以 `(project_id, asset_id)` 复合键比对裁撤系统数据与 DB 现有数据，得出新增/更新/删除集合，允许同一 `asset_id` 在不同项目并存；MUST 移除按 `asset_id` 去重择优的逻辑。变更比对 SHALL 覆盖 `expect_abolish_time`，即裁撤系统截止时间变化时须触发更新。

#### Scenario: 同一固资号多项目并存

- **WHEN** 同一 `asset_id` 出现在两个不同 `project_id` 下
- **THEN** 系统 SHALL 将两条记录分别按各自 `(project_id, asset_id)` 维护，不做去重合并

#### Scenario: 截止时间变化触发更新

- **WHEN** 裁撤系统某设备的 `expectAbolishTime` 与 DB 中该记录的 `expect_abolish_time` 不一致
- **THEN** 系统 SHALL 将该记录纳入更新集合，回写新的 `expect_abolish_time`
