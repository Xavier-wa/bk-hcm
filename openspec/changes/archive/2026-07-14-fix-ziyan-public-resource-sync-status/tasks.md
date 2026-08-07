## 1. 同步编排改造

- [x] 1.1 在 `SyncAllResource` 中提前创建 `detail.SyncDetail`，确保公共资源同步可复用同一状态上下文。
- [x] 1.2 修改 `SyncPublicResource` 入参，接收 `sd *detail.SyncDetail`。
- [x] 1.3 调整 `SyncAllResource` 调用 `SyncPublicResource` 的位置和参数，保持公共资源先于私有资源同步。

## 2. 公共资源状态记录

- [x] 2.1 为 `region` 同步增加 `ResSyncStatusSyncing/Success/Failed` 状态记录。
- [x] 2.2 为 `zone` 同步增加 `ResSyncStatusSyncing/Success/Failed` 状态记录。
- [x] 2.3 为 `image` 同步增加 `ResSyncStatusSyncing/Success/Failed` 状态记录。
- [x] 2.4 确保失败时返回原有 `failedRes` 和原始错误，不改变上层错误识别语义。

## 3. 代码质量与兼容性

- [x] 3.1 视重复情况抽取公共资源状态包装 helper，避免三段状态处理逻辑重复过多。
- [x] 3.2 确认 `region -> zone -> image` 同步顺序保持不变。
- [x] 3.3 确认 `syncPublicResource` 仅随首个账号触发一次的既有行为保持不变。
- [x] 3.4 运行 gofmt/goimports，确保新增导入符合项目规范。

## 4. 验证

- [x] 4.1 运行相关 Go 单元测试或最小编译检查，确认 cloud-server 自研云同步包可编译。
- [x] 4.2 手动验证缺失 `zone` 状态记录的账号在下一轮成功同步后会新增 `zone` 状态项。
- [x] 4.3 手动验证已有 `region/image` 历史失败状态在成功同步后更新为成功并刷新时间。
