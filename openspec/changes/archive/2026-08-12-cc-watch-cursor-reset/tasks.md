# Tasks: cc-watch-cursor-reset

## 1. 接口协议定义

- [x] 1.1 在 `pkg/api/cloud-server/cc-sync/` 定义重置接口请求结构（resource，目标租户取 header `X-Bk-Tenant-Id`）并实现 `Validate()`：必填校验 + resource 白名单（host / host_relation）
- [x] 1.2 定义响应结构：回显重置前的 cursor（`pre_cursor`）；`msg` 预留

## 2. Watcher 暴露 cursor 重置能力

- [x] 2.1 `cmd/cloud-server/service/watch/bkcc/watcher.go` 新增导出方法 `ResetEventCursor`：先 `getEventCursor` 读旧值，再写 reset flag key（lease 写 etcd），返回旧 cursor 供审计日志使用；watch 循环每轮迭代开头消费 reset flag 并将 cursor 置空，避免与在途轮次的 cursor 提交竞态

## 3. 主机补偿同步入口

- [x] 3.1 `tcloud-ziyan` / `other` 新增 `SyncHostResource`（仅同步主机）
- [x] 3.2 `cmd/cloud-server/service/sync/sync_host.go` 新增通用入口 `SyncHostsByTenant(kt, vendors, cliSet)`（目标租户取 `kt.TenantID`）：厂商串行、账号页内并发（`errgroup` + `SyncConcurrencyDefaultMaxLimit`）
- [x] 3.3 单账号互斥：以 DB `account_sync_detail`（cvm + syncing）为主判断，抢主机补偿专用锁 `lock.ResKey(accountID, cvm)` 防并发；不抢账号级锁，避免被同账号其他资源同步误伤
- [x] 3.4 `sync/lock` 新增 `ResKey`，生成资源级锁 key（`/hcm/lock/cloud-server/sync/{accountID}/cvm`）

## 4. 装配

- [x] 4.1 `capability.Capability` 增加 watcher 字段，`cmd/cloud-server/service/service.go` 在 `NewWatcher` 处注入
- [x] 4.2 `cmd/cloud-server/service/admin/admin.go` 注册路由 `POST /cc_sync/watch/cursor/reset`

## 5. admin handler 实现

- [x] 5.1 实现 handler（`cc_watch_cursor.go`）：解码 → `Validate()` → 租户存在性校验（`logics/tenant.CheckTenantExist`）→ 调用 watcher 重置方法写 etcd；写入失败返回错误
- [x] 5.2 重置前、后各打一条 Warn 级审计日志（旧 cursor、tenant、resource、rid）；组装响应回显 `pre_cursor`
- [x] 5.3 重置成功后异步触发 `SyncHostsByTenant(kt, [ziyan, other], cliSet)`，不做 master 校验

## 6. 测试与验证

- [x] 6.1 请求结构 `Validate()` 单元测试：必填缺失、非法 resource、合法用例
- [x] 6.2 `sync_host` 单元测试（fake DB/锁/同步副作用）：正常同步、DB syncing 跳过、查 DB 失败跳过、锁占用跳过、同步失败仍解锁、多账号混合、拉账号失败不 panic
- [x] 6.3 `bkcc` reset flag key 拼接单元测试；`consumeResetFlag` 依赖 etcd client，无 mock 基础设施，由集成测试覆盖
- [ ] 6.4 本地自测（需真实环境，待人工执行）：调用接口确认 reset flag 写入、watch 下一轮消费 flag 并将 cursor 置空、从当前时间起消费、审计日志完整；有同步进行中时对应账号被跳过、定时全量不被中断

## 7. 本期不做（已记录方案，后续立项）

- [ ] 7.1 按指定时间重置消费位置（design「后续演进」）
- [ ] 7.2 补偿与定时全量并发写同一条同步状态记录的长期治理
