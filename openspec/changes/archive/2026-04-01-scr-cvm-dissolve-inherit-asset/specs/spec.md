# dissolve-host-crud

## ADDED Requirements

### Requirement: 裁撤主机表 CRUD 接口

系统 SHALL 通过 data-service 暴露 `recycle_host_info` 表的批量创建、批量更新、列表查询、批量删除四个 HTTP 接口，供上层服务（woa-server）调用。

#### Scenario: 批量创建裁撤主机记录

- **WHEN** 调用 `POST /dissolve/recycle_hosts/batch/create`，传入包含 asset_id、inner_ip、module、abolish_phase、project_name 的主机列表
- **THEN** 系统 SHALL 为每条记录生成唯一 ID，在事务中批量插入 `recycle_host_info` 表，返回创建的 ID 列表

#### Scenario: 列表查询裁撤主机

- **WHEN** 调用 `POST /dissolve/recycle_hosts/list`，传入 filter 表达式和分页参数
- **THEN** 系统 SHALL 按 filter 条件查询 `recycle_host_info` 表，支持按 asset_id 精确匹配，返回符合条件的记录列表或记录总数（count 模式）

#### Scenario: 批量更新裁撤主机记录

- **WHEN** 调用 `PATCH /dissolve/recycle_hosts/batch`，传入 filter 表达式和更新字段
- **THEN** 系统 SHALL 在事务中按 filter 条件更新 `recycle_host_info` 表中匹配的记录

#### Scenario: 批量删除裁撤主机记录

- **WHEN** 调用 `DELETE /dissolve/recycle_hosts/batch`，传入 filter 表达式
- **THEN** 系统 SHALL 在事务中按 filter 条件删除 `recycle_host_info` 表中匹配的记录

### Requirement: 裁撤主机 client 封装

系统 SHALL 在 `pkg/client/data-service/tcloud-ziyan/` 提供 `DissolveClient`，封装上述四个 data-service 接口的调用，并注册到 `Client` 结构体中，使 woa-server 可通过 `clientSet.DataService().TCloudZiyan.Dissolve` 链式调用。

#### Scenario: woa-server 通过 client 查询裁撤主机

- **WHEN** woa-server 业务逻辑调用 `clientSet.DataService().TCloudZiyan.Dissolve.ListRecycleHost(kt, req)`
- **THEN** client SHALL 发送 HTTP 请求到 data-service 的 `/dissolve/recycle_hosts/list` 端点，并将响应反序列化后返回

---

# check-apply-order-host

## MODIFIED Requirements

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

---

# create-ticket-apply

## MODIFIED Requirements

### Requirement: 创建申领单据时支持机房裁撤固资号继承

系统 SHALL 在创建申领单据时，当 require_type=3（机房裁撤）且 spec 中 bk_asset_id 非空时，执行固资号校验并自动继承计费模式。

#### Scenario: 机房裁撤传入 bk_asset_id 时校验并继承计费模式

- **WHEN** 创建申领单据传入 require_type=3 且 spec.bk_asset_id 非空
- **THEN** 系统 SHALL 调用 `CheckInheritedHost`（require_type=3）校验固资号合法性。校验通过后，SHALL 用返回的 `InstanceChargeType` 覆盖接口传入的 `charge_type`（计费模式以 BKCC 为准），保留接口传入的 `charge_months` 不做覆盖，并将返回的 `CloudInstID` 设置为 `inherit_instance_id`

#### Scenario: 机房裁撤未传入 bk_asset_id 时不执行继承

- **WHEN** 创建申领单据传入 require_type=3 但 spec.bk_asset_id 为空
- **THEN** 系统 SHALL 不执行固资号校验和计费模式继承，按正常流程处理（用户自行选择计费模式）

#### Scenario: 非机房裁撤场景保持原有行为

- **WHEN** 创建申领单据传入 require_type 不为 3（如 1、2、6、7）
- **THEN** 系统 SHALL 保持现有创建逻辑不变，滚服项目(6)使用 inherit_instance_id 的现有校验逻辑

---

# modify-ticket-apply

## MODIFIED Requirements

### Requirement: 修改单据重试接口支持固资号和云实例 ID 存储

系统 SHALL 在修改单据重试流程中，支持 `bk_asset_id` 和 `inherit_instance_id` 字段的完整生命周期：接口接收 → MongoDB 存储 → 审批回调传递 → 写回申领单据。

#### Scenario: 修改单据时存储 bk_asset_id 和 inherit_instance_id

- **WHEN** 调用修改单据接口（资源下或业务下），传入 spec.bk_asset_id
- **THEN** 系统 SHALL 将 `bk_asset_id` 存入 `ModifyRecord.Details.CurData.BkAssetId`。若 require_type 为机房裁撤(3)或滚服项目(6)且 bk_asset_id 非空，SHALL 调用 check 接口获取 `CloudInstID`，将其存入 `ModifyRecord.Details.CurData.InheritInstanceId`

#### Scenario: 审批通过回调时传递 inherit_instance_id

- **WHEN** 审批人通过修改单据审批，触发 `auditApplyModifyCallback`
- **THEN** 系统 SHALL 从 `modifyRecord.Details.CurData.InheritInstanceId` 读取存储的云实例 ID，赋值给 `ModifyApplyReq.Spec.InheritInstanceId`

#### Scenario: 审批通过回调时更新申领单据的固资号和云实例 ID

- **WHEN** `auditApplyModifyCallback` 触发 `modifyOrder` 更新申领单据
- **THEN** `modifyOrder` SHALL 将 `spec.bk_asset_id` 和 `spec.inherit_instance_id` 一并写入 `cr_ApplyOrder` 表（MongoDB），确保申领单据记录完整的继承信息

#### Scenario: 无固资号时保持原有行为

- **WHEN** 调用修改单据接口，spec 中无 bk_asset_id 字段
- **THEN** 系统 SHALL 保持现有修改逻辑不变，`ModifyData` 中 `BkAssetId` 和 `InheritInstanceId` 为空字符串零值
