## 1. 审计公共方法分批改造

- [x] 1.1 在 `ResBizAssignAudit` 中对 resIDs 按 `constant.BatchOperationMaxLimit`(100) 分批，每批构造独立请求调用 data-service。
- [x] 1.2 在 `ResDeliverAudit` 中同样按 100 分批调用 data-service。
- [x] 1.3 在 `ResCloudAreaBindAudit` 中同样按 100 分批调用 data-service。
- [x] 1.4 确认空列表直接返回、≤ 100 正常单次调用、> 100 分批调用的边界条件。

## 2. 拼写错误修正

- [x] 2.1 `CloudResourceAssignAuditReq.Validate()`：`"assign shuold <= %d"` → `"assign should <= %d"`。
- [x] 2.2 `CloudResourceUpdateAuditReq.Validate()`：`"updates shuold <= %d"` → `"updates should <= %d"`。
- [x] 2.3 `CloudResourceOperationAuditReq.Validate()`：`"assign shuold <= %d"` → `"operations should <= %d"`。

## 3. 代码质量与兼容性

- [x] 3.1 运行 `gofmt/goimports`，确保导入符合项目规范。
- [x] 3.2 确认分批逻辑不影响事务一致性，各批次独立调用。
- [x] 3.3 确认错误处理语义不变：任一批次失败返回错误，已成功批次不回滚。
- [x] 3.4 确认所有调用方（cvm/disk/eip/nic assign）无需额外修改。

## 4. 验证

- [x] 4.1 本地编译通过，无 lint 错误。
- [ ] 4.2 使用「主机数 ≤ 100，但关联磁盘 > 100」的数据集验证批量分配可成功完成。
- [ ] 4.3 验证主机、磁盘、EIP、网卡均正确分配到目标业务，审计记录完整。
- [ ] 4.4 验证错误文案无拼写错误，提示信息语义清晰。
- [ ] 4.5 验证「主机数 > 100」等其他批量限制逻辑不受影响。
- [ ] 4.6 验证单台主机分配流程与优化前一致，无额外分批开销。

> 4.2-4.6 依赖真实环境的数据集与人工/QA 回归验证，本次会话无法在本地直接执行，需部署到测试环境后手动验证。
