## MODIFIED Requirements

### Requirement: Sub-account permission template sync handles missing local templates

当同步子账号关联的权限模板本地 ID 时，若某条云上策略在本地 DB 中不存在，系统 SHALL 自动补录该策略，而不是报错中断。

具体流程：
1. 在分页拉取子账号绑定策略列表时，遇到本地 `cloudIDToLocalID` 映射中不存在的策略，SHALL 将其收集到 `missingPolicies` 切片
2. 完成所有分页后，若 `missingPolicies` 非空，系统 SHALL 逐个调用 `GetPolicyDetail` 拉取完整策略内容
3. 系统 SHALL 将缺失策略通过 `BatchCreate` 批量写入本地 `permission_template` 表
4. 系统 SHALL 将 `BatchCreate` 返回的本地 ID 列表追加到当前子账号的 `templateIDs` 结果中
5. 同步最终结果中，该子账号的 `PermissionTemplateIDs` SHALL 包含所有已补录模板的本地 ID

#### Scenario: Cloud policy not found in local DB during sub-account sync
- **WHEN** `listSubAccountPermissionTemplateIDs` 遍历云上策略列表时，策略 `cloud_id` 在 `cloudIDToLocalID` 中不存在
- **THEN** 该策略被收集到 `missingPolicies`，不报错，继续处理其余策略

#### Scenario: Missing policies are fetched and created in DB after paging loop
- **WHEN** 分页 for 循环结束后 `missingPolicies` 非空
- **THEN** 系统调用 `GetPolicyDetail` 获取每个缺失策略的完整内容，并通过 `BatchCreate` 写入 DB

#### Scenario: Newly created template IDs are included in the result
- **WHEN** `BatchCreate` 成功返回 `IDs`
- **THEN** 这些本地 ID 被追加到 `templateIDs`，函数返回的列表中包含所有策略（含新补录）的本地 ID

#### Scenario: All policies exist in local DB (happy path unchanged)
- **WHEN** 所有云上策略都在 `cloudIDToLocalID` 中存在
- **THEN** 行为与修改前完全相同，不触发任何补录逻辑
