## Why

在同步三级账号关联权限模板时，`listSubAccountPermissionTemplateIDs` 遇到云上策略在本地 DB 不存在的情况会直接报错返回，导致整个子账号同步任务中断。这种情况发生于权限模板同步（`PermissionTemplate` sync）尚未覆盖该策略时，应当容错处理并自动补录缺失模板。

## What Changes

- 修改 `listSubAccountPermissionTemplateIDs`，将本地不存在的策略收集到 `missingPolicies` 切片，而不是直接报错
- 在分页 for 循环结束后，若存在缺失策略，逐个调用 `GetPolicyDetail` 拉取完整策略内容，再通过 `BatchCreate` 批量写入本地 DB
- 批量创建返回 `core.BatchCreateResult.IDs`，将新本地 ID 追加到本次子账号的 `templateIDs` 结果
- `listSubAccountPermissionTemplateIDs` 新增 `accountID` 入参，用于创建模板时填充 `account_id` 字段

## Capabilities

### New Capabilities

无新业务能力，属于缺陷修复 / 容错增强。

### Modified Capabilities

- `sub-account-permission-template-sync`：`listSubAccountPermissionTemplateIDs` 的行为从"缺失即报错"变更为"缺失则自动补录"，改变了同步逻辑的健壮性契约

## Impact

- **文件**：`cmd/hc-service/logics/res-sync/tcloud/sub_account_permission_template.go`
- **函数**：`listSubAccountPermissionTemplateIDs`（修改签名 + 逻辑）、`SubAccountPermissionTemplate`（更新调用传参）
- **外部依赖**：新增调用 `cli.cloudCli.GetPolicyDetail`、`cli.dbCli.TCloud.PermissionTemplate.BatchCreate`（均为已有接口）
- **无 API 变更**，无数据库 schema 变更
