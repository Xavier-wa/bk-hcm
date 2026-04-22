# create-ticket-apply

## Requirements

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
