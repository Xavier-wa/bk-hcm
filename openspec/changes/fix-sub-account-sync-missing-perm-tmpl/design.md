## Context

`SubAccountPermissionTemplate` 负责同步三级账号（子账号）关联的权限模板本地 ID。其核心辅助函数 `listSubAccountPermissionTemplateIDs` 通过分页调用云 API `ListAttachedUserAllPolicies` 获取子账号绑定的策略列表，再通过预构建的 `cloudIDToLocalID` 映射查找每条策略对应的本地 ID。

当某条云上策略不在本地 DB 时（映射 miss），当前代码直接 `return nil, errf.NewFromErr(...)` 报错，导致整个同步任务失败。这会在权限模板同步（`PermissionTemplate` sync）覆盖不完整、或两个 sync 任务执行顺序不确定时频繁触发。

## Goals / Non-Goals

**Goals:**
- 将 "映射 miss → 报错" 改为 "映射 miss → 收集缺失策略 → 拉取详情 → 批量补录 → 补充本地 ID"
- 保持对已有策略的处理逻辑不变
- 保持对外接口签名不变（`SubAccountPermissionTemplate`）

**Non-Goals:**
- 不修改 `PermissionTemplate` 全量同步逻辑
- 不修改 DB 表结构
- 不添加新 API 端点

## Decisions

### D1：在 `listSubAccountPermissionTemplateIDs` 内部完成补录

**决定**：在函数内部收集缺失策略，分页循环结束后立即补录，返回值语义不变（仍为 `[]string` 本地 ID 列表）。

**备选方案**：将缺失策略返回给调用方，由 `SubAccountPermissionTemplate` 统一收集所有子账号的缺失策略后再批量补录。

**放弃原因**：统一收集方案需要修改 `listSubAccountPermissionTemplateIDs` 的返回类型，同时需要在外层对缺失策略做去重和分批，复杂度更高。内部补录方案更内聚，改动面更小。

### D2：补录时调用 `GetPolicyDetail` 获取完整策略内容

**决定**：对每个缺失策略，调用 `cli.cloudCli.GetPolicyDetail` 获取 `PolicyDocument`、`PolicyName`、`PolicyType`、`Description` 等完整字段，再构造 `PermissionTemplateCreate` 写入 DB。

**原因**：`TCloudAttachedPolicy`（`ListAttachedUserAllPolicies` 返回项）缺少 `PolicyDocument`，而 `PermissionTemplateCreate.PolicyDocument` 是必填字段（`validate:"required"`），无法跳过。

**代价**：每个缺失策略多一次云 API 调用，但缺失情况属于异常路径，正常运行时不会触发。

### D3：新增 `accountID` 参数传递给 `listSubAccountPermissionTemplateIDs`

**决定**：函数签名从 `(kt, uin, cloudIDToLocalID)` 改为 `(kt, uin, accountID, cloudIDToLocalID)`。

**原因**：`PermissionTemplateCreate` 需要 `AccountID` 字段，当前函数没有此信息，需从调用方传入。

### D4：批量补录使用 `BatchCreate`，通过返回 ID 补充 `templateIDs`

**决定**：使用 `cli.dbCli.TCloud.PermissionTemplate.BatchCreate` 批量写入，返回的 `core.BatchCreateResult.IDs` 按顺序对应新建策略的本地 ID，直接追加到 `templateIDs`。

**前提**：`BatchCreate` 返回的 ID 顺序与请求顺序一致（与现有同类实现一致，如 VPC/主机同步）。

## Risks / Trade-offs

- **并发补录冲突**：多个子账号可能绑定同一缺失策略，若并行执行可能触发 DB 唯一约束冲突（`cloud_id + account_id` 唯一键）。→ 当前实现是串行的（子账号逐一处理），不存在此问题。若未来改为并发，需加去重或 upsert。
- **GetPolicyDetail 限流**：对缺失策略逐一调用云 API，可能触发腾讯云 CAM 限流。→ 与现有 `getPolicyDetails` 在 `PermissionTemplate` sync 中的处理方式一致，缺失策略数量通常极少，可接受。
- **补录后 `cloudIDToLocalID` 不更新**：本次补录后，下次其他子账号处理相同策略时仍会触发补录逻辑（若同一账号内多个子账号绑定了同一缺失策略）。→ 可在补录成功后更新 `cloudIDToLocalID`，但当前实现为避免复杂度暂不处理，后续可优化。

## Migration Plan

- 纯逻辑修改，无数据迁移
- 部署后立即生效，下次 sync 任务运行时采用新逻辑
- 回滚：回退代码即可，已补录的模板数据不影响任何现有功能
