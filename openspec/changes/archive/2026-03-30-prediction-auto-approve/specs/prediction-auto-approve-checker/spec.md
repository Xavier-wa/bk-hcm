## ADDED Requirements

### Requirement: 预测单自动过单条件检查
系统 SHALL 提供统一的条件检查函数 `checkPredictionAutoApprove`，判断预测单是否满足自动过单条件。

**前置条件**：
- 只有"追加"类型的需求单才允许自动过单
- 包含"删除"或"变更"类型需求的单据不走自动过单，直接 break 退出

**三个条件必须全部满足**：
1. 机型全部为标准型（DeviceFamily == "标准型"），空机型或其他机型都不允许
2. 整单 CPU ≤ 1500 核
3. 整单 CBS ≤ 45TB（46080GB）

检查结果返回 `autoApproveCheckResult` 结构体，包含：是否可自动过单（CanAutoApprove）、原因说明（Reason）、整单 CPU 核心数（TotalCPUCores）、整单 CBS 容量（TotalCBSSizeGB）。

#### Scenario: 满足所有条件时允许自动过单
- **GIVEN** 预测单为"追加"类型（Original == nil）
- **AND** 所有 CVM 机型均为"标准型"
- **AND** 整单 CPU 核心数为 1000 核（≤ 1500）
- **AND** 整单 CBS 容量为 30TB（≤ 45TB）
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = true`
- **AND** `Reason` 包含"满足自动过单条件"

#### Scenario: 包含删除或变更类型需求时不允许自动过单
- **GIVEN** 预测单包含"删除"或"变更"类型需求（Original != nil）
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = false`
- **AND** `Reason` 包含"包含删除类型需求"或"包含变更类型需求"
- **AND** 直接退出遍历，不再检查其他需求

#### Scenario: 包含非标准型 CVM 时不允许自动过单
- **GIVEN** 预测单中包含一个机型为"计算型"的 CVM
- **AND** 其他 CVM 均为"标准型"
- **AND** 整单 CPU ≤ 1500 核且 CBS ≤ 45TB
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = false`
- **AND** `Reason` 包含"包含非标准型机型: 计算型"

#### Scenario: 机型为空时不允许自动过单
- **GIVEN** 预测单中包含一个机型为空字符串的 CVM
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = false`
- **AND** `Reason` 包含"包含未指定机型的需求"

#### Scenario: CPU 核心数超出阈值时不允许自动过单
- **GIVEN** 预测单中所有 CVM 机型均为"标准型"
- **AND** 整单 CPU 核心数为 1600 核（> 1500）
- **AND** 整单 CBS ≤ 45TB
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = false`
- **AND** `Reason` 包含"CPU核心数超出阈值"
- **AND** `TotalCPUCores` 为 1600

#### Scenario: CBS 容量超出阈值时不允许自动过单
- **GIVEN** 预测单中所有 CVM 机型均为"标准型"
- **AND** 整单 CPU ≤ 1500 核
- **AND** 整单 CBS 容量为 50TB（> 45TB）
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = false`
- **AND** `Reason` 包含"CBS容量超出阈值"
- **AND** `TotalCBSSizeGB` 为 51200

#### Scenario: 边界值 - CPU 恰好等于阈值
- **GIVEN** 预测单中所有 CVM 机型均为"标准型"
- **AND** 整单 CPU 核心数恰好为 1500 核
- **AND** 整单 CBS ≤ 45TB
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = true`

#### Scenario: 边界值 - CBS 恰好等于阈值
- **GIVEN** 预测单中所有 CVM 机型均为"标准型"
- **AND** 整单 CPU ≤ 1500 核
- **AND** 整单 CBS 容量恰好为 45TB（46080GB）
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = true`

#### Scenario: 多个条件同时不满足
- **GIVEN** 预测单中包含非标准型 CVM
- **AND** 整单 CPU > 1500 核
- **AND** 整单 CBS > 45TB
- **WHEN** 调用 `checkPredictionAutoApprove` 检查该预测单
- **THEN** 返回 `CanAutoApprove = false`
- **AND** `Reason` 包含所有不满足条件的说明

### Requirement: 自动过单阈值常量定义
系统 SHALL 在 `pkg/criteria/constant/res_plan.go` 中定义自动过单阈值常量（与资源规划相关常量放在一起）：
- `AutoApproveCPUCoreThreshold int64 = 1500`：CPU 核心数阈值
- `AutoApproveCBSSizeThreshold int64 = 46080`：CBS 容量阈值（单位：GB）

机型判断使用已有枚举 `enumor.DeviceFamilyStandard`（定义在 `pkg/criteria/enumor/device.go`）。

#### Scenario: 常量值固定
- **WHEN** 系统初始化
- **THEN** `AutoApproveCPUCoreThreshold` 值为 1500
- **AND** `AutoApproveCBSSizeThreshold` 值为 46080
- **AND** `enumor.DeviceFamilyStandard` 值为 "标准型"
