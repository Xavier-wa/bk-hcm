# modify-ticket-apply

## Requirements

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
