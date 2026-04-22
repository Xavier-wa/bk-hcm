# check-apply-order-host

## Requirements

### Requirement: 校验继承主机信息

系统 SHALL 在校验继承套餐的主机信息时，通过 `require_type` 参数区分"滚服项目"(6)和"机房裁撤"(3)两种场景，分别执行不同的校验规则。请求参数新增 `require_type` 字段（int 类型，必填）。

#### Scenario: 滚服项目校验（保持现有行为）

- **WHEN** 调用 check 接口传入 require_type=6（滚服项目）和 bk_asset_id
- **THEN** 系统 SHALL 执行现有校验逻辑：验证主机所属部门为 IEG、计费信息完整、机型为通用机型（CommonType）、地域与申请一致，返回机型、计费模式、计费时长等信息

#### Scenario: 机房裁撤校验 — 固资号必须在裁撤表中

- **WHEN** 调用 check 接口传入 require_type=3（机房裁撤）和 bk_asset_id
- **THEN** 系统 SHALL 调用 data-service 的 List 接口查询 `recycle_host_info` 表，校验该固资号是否存在于裁撤表中。若不存在，SHALL 返回错误"该固资号不在机房裁撤列表中"

#### Scenario: 机房裁撤校验 — 机型族不能是 GPU 型

- **WHEN** 调用 check 接口传入 require_type=3，且固资号在裁撤表中
- **THEN** 系统 SHALL 查询 BKCC 获取主机的机型信息，再通过 `QueryCvmInstanceType` 或 `ListCvmInstanceInfoByDeviceTypes` 获取机型族，校验机型族不包含 "GPU"（`constant.GpuInstanceClass`）。若为 GPU 机型族，SHALL 返回错误"GPU机型不支持机房裁撤继承"

#### Scenario: 机房裁撤校验 — 返回机型代次

- **WHEN** 调用 check 接口传入 require_type=3，且固资号通过裁撤表和 GPU 校验
- **THEN** 系统 SHALL 通过 `QueryCvmInstanceType` 获取机型详情，并在接口响应中返回 `generation_type`

#### Scenario: 机房裁撤校验 — 返回计费信息

- **WHEN** 机房裁撤校验全部通过
- **THEN** 系统 SHALL 返回与滚服项目相同的响应结构：device_type、device_group、generation_type、instance_charge_type、charge_months、billing_start_time、old_billing_expire_time、new_billing_expire_time、bk_cloud_inst_id

### Requirement: CRP 响应结构新增 generationType 字段

系统 SHALL 在 `QueryCvmInstanceTypeItem` 结构体中新增 `GenerationType`（string）字段，用于接收 CRP 返回的机型代次信息。

#### Scenario: CRP 返回 generationType

- **WHEN** CRP 接口在响应中包含 `generationType` 字段
- **THEN** 系统 SHALL 将其反序列化到 `QueryCvmInstanceTypeItem.GenerationType`，并在校验接口响应中返回

#### Scenario: CRP 未返回 generationType

- **WHEN** CRP 接口在响应中不包含 `generationType` 字段
- **THEN** `QueryCvmInstanceTypeItem.GenerationType` SHALL 为空字符串，校验接口仍可正常返回
