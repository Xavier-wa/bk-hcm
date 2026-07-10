## 1. 修改函数签名

- [x] 1.1 为 `listSubAccountPermissionTemplateIDs` 新增 `accountID string` 参数，使函数签名变为 `(kt *kit.Kit, uin uint64, accountID string, cloudIDToLocalID map[string]string) ([]string, error)`
- [x] 1.2 更新 `SubAccountPermissionTemplate` 中调用 `listSubAccountPermissionTemplateIDs` 的地方，补充传入 `opt.AccountID`

## 2. 收集缺失策略

- [x] 2.1 在 `listSubAccountPermissionTemplateIDs` 的分页 for 循环中，将 `if !ok { return nil, err }` 替换为将 `policy`（`TCloudAttachedPolicy`）追加到 `missingPolicies []typeaccount.TCloudAttachedPolicy` 切片
- [x] 2.2 移除原有的 `logs.Errorf` + `return nil, errf.NewFromErr(...)` 报错逻辑

## 3. 补录缺失策略到 DB

- [x] 3.1 在分页 for 循环结束后，若 `len(missingPolicies) > 0`，遍历 `missingPolicies`，将 `PolicyID`（string）转换为 `uint64`，逐个调用 `cli.cloudCli.GetPolicyDetail` 获取完整策略内容
- [x] 3.2 构造 `[]protocloud.PermissionTemplateCreate[corecloud.TCloudPermissionTemplateExtension]` 列表，填充 `CloudID`、`Name`、`AccountID`、`PolicyDocument`、`Memo`、`Extension.CloudType` 字段
- [x] 3.3 调用 `cli.dbCli.TCloud.PermissionTemplate.BatchCreate` 批量写入，获取返回的 `*core.BatchCreateResult`
- [x] 3.4 将 `BatchCreateResult.IDs` 追加到 `templateIDs`，保证最终返回列表包含所有策略（含新补录）的本地 ID
- [x] 3.5 在补录成功后写 `logs.Infof` 记录补录数量

## 4. 错误处理与日志

- [x] 4.1 `PolicyID` string → uint64 转换失败时，记录 `logs.Errorf` 并返回错误
- [x] 4.2 `GetPolicyDetail` 调用失败时，记录 `logs.Errorf` 并返回错误
- [x] 4.3 `BatchCreate` 调用失败时，记录 `logs.Errorf` 并返回错误
