## MODIFIED Requirements

### Requirement: 分配审计公共方法内部按批次调用 data-service

cloud-server 侧的 `ResBizAssignAudit`、`ResDeliverAudit`、`ResCloudAreaBindAudit` 三个公共审计方法 SHALL 在方法内部对资源 ID 列表按 `constant.BatchOperationMaxLimit`(100) 分批，每批构造独立的 `CloudResourceAssignAuditReq` 请求调用 data-service 审计接口。

#### Scenario: 资源 ID 列表为空时直接返回

- **GIVEN** 调用 `ResBizAssignAudit` / `ResDeliverAudit` / `ResCloudAreaBindAudit`
- **WHEN** 传入的资源 ID 列表为空
- **THEN** 系统 SHALL 直接返回，不发起任何 data-service 调用

#### Scenario: 资源 ID 列表不超过 100 时单次调用

- **GIVEN** 调用 `ResBizAssignAudit` / `ResDeliverAudit` / `ResCloudAreaBindAudit`
- **WHEN** 传入的资源 ID 列表数量 ≤ 100
- **THEN** 系统 SHALL 构造单个请求调用 data-service，行为与优化前一致

#### Scenario: 资源 ID 列表超过 100 时自动分批

- **GIVEN** 调用 `ResBizAssignAudit` / `ResDeliverAudit` / `ResCloudAreaBindAudit`
- **WHEN** 传入的资源 ID 列表数量 > 100
- **THEN** 系统 SHALL 按 100 条/批将资源 ID 切分为多个批次，逐批构造独立请求调用 data-service

#### Scenario: 批量分配主机时关联磁盘总数超过 100

- **GIVEN** 用户选择 N 台主机（N ≤ 100）执行批量分配
- **AND** 这些主机关联的磁盘总数 > 100
- **WHEN** 执行批量分配操作
- **THEN** 系统 SHALL 将磁盘审计记录按 100 条/批分批提交到 data-service
- **AND** 所有磁盘正确分配到目标业务
- **AND** 审计记录完整无遗漏

#### Scenario: 批量分配主机时关联 EIP 和网卡总数超过 100

- **GIVEN** 用户选择 N 台主机（N ≤ 100）执行批量分配
- **AND** 这些主机关联的 EIP 和网卡总数 > 100
- **WHEN** 执行批量分配操作
- **THEN** 系统 SHALL 将 EIP 和网卡审计记录分别按 100 条/批分批提交到 data-service
- **AND** 所有 EIP 和网卡正确分配到目标业务
- **AND** 审计记录完整无遗漏

### Requirement: 任一批次失败时的错误处理

data-service 审计接口任一批次调用失败时，系统 SHALL 立即返回错误，已成功提交的批次不回滚。

#### Scenario: 某一批次调用失败

- **GIVEN** 资源 ID 列表 > 100，需要分多批调用
- **WHEN** 中间某一批次调用 data-service 返回错误
- **THEN** 系统 SHALL 立即返回该错误给调用方
- **AND** 已成功提交的批次不回滚
- **AND** 错误信息包含失败的批次范围

### Requirement: 审计校验文案拼写修正

data-service 侧审计请求校验的错误文案 SHALL 修正拼写错误，确保语义清晰准确。

#### Scenario: AssignAudit 校验超限提示

- **GIVEN** 向 data-service 发送 `CloudResourceAssignAuditReq` 请求
- **WHEN** `len(req.Assigns) > BatchOperationMaxLimit(100)`
- **THEN** 返回的错误信息 SHALL 为 `"assign should <= 100"`（无拼写错误）

#### Scenario: UpdateAudit 校验超限提示

- **GIVEN** 向 data-service 发送 `CloudResourceUpdateAuditReq` 请求
- **WHEN** `len(req.Updates) > BatchOperationMaxLimit(100)`
- **THEN** 返回的错误信息 SHALL 为 `"updates should <= 100"`（无拼写错误）

#### Scenario: OperationAudit 校验超限提示

- **GIVEN** 向 data-service 发送 `CloudResourceOperationAuditReq` 请求
- **WHEN** `len(req.Assigns) > BatchOperationMaxLimit(100)`
- **THEN** 返回的错误信息 SHALL 为 `"operations should <= 100"`（使用准确的 operations 而非 assign）

### Requirement: 兼容性保证

本次变更 SHALL 完全向后兼容，所有现有调用方无需修改代码即可自动受益。

#### Scenario: 单台主机分配不受影响

- **GIVEN** 选择 1 台主机（仅有系统盘，关联资源 < 100）执行分配
- **WHEN** 执行分配操作
- **THEN** 分配流程与优化前一致，无额外分批开销

#### Scenario: 主机数超限仍触发原有逻辑

- **GIVEN** 选择的主机数 > 100
- **WHEN** 执行批量分配操作
- **THEN** 系统 SHALL 仍然触发原有的主机数限制逻辑，不因本次优化改变
