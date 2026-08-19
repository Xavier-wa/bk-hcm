## 1. Logics：注入调度同源 CRP 客户端

- [x] 1.1 将 `NewSubnetOp` 注入的客户端由 `thirdCli.OldCVM` 改为 `thirdCli.CVM`
- [x] 1.2 确认 `GetAllSubnet`、`GetSubnetList` 方法体不增加 CRP 调用，也不删除现有 `ListCountIP`；`AvailableIpCount` 改为 `*uint64` 以区分未取到与剩余 0

## 2. Logics：按 left_ip 模式外挂 CRP 左连接

- [x] 2.1 在 `SubnetIf` 增加 `GetAllSubnetWithAvailIP(kt *kit.Kit, req *types.GetAllSubnetReq) (*types.GetSubnetResult, error)`
- [x] 2.2 实现私有方法 `queryLeftIPMap`：参数齐全时调 `s.cvm.QueryRealCvmSubnet`，构建 `cloud_id → leftIpNum`；记录 CRP `TraceId`；错误向上返回（由调用方降级）
- [x] 2.3 实现 `GetAllSubnetWithAvailIP`（对齐 `SyncLeftIP`）：先 `GetAllSubnet` 取清单；region/zone/vpc 不全或 CRP 失败时 Warn 并将 `AvailableIpCount` 置 `nil`；成功则按 `SubnetId` 左连接，把 `int` 的 `leftIpNum` 转为 `*uint64` 写入，未命中写 `0`
- [x] 2.4 `GetSubnetList` 赋值改为 `ValToPtr`，`ListCountIP` 失败仍写数字 `0`，不写 `null`

## 3. Service：下拉入口改调新方法

- [x] 3.1 将 `cmd/woa-server/service/config/subnet.go` 的 `GetSubnet` 从 `GetAllSubnet` 改为 `GetAllSubnetWithAvailIP`；`GetSubnetList` / `UpdateSubnetProperty` 不改调用目标

## 4. 文档

- [x] 4.1 更新 `docs/api-docs/web-server/docs/scr/resource-apply/list_cvm_subnet.md`：补齐 `info` 既有字段；说明 `available_ip_count` 来源 CRP `leftIpNum`，失败或未查询为 `null`，明确剩余 0 时为 `0`
- [x] 4.2 核对 `docs/api-docs/web-server/docs/scr/config-manage/list_config_cvm_subnet.md`：配置列表实现与文档均不变，不改该文件

## 5. 验证

- [x] 5.1 编译 woa-server，确认无未使用 import / 类型不匹配
- [x] 5.2 按 spec 核对：CRP 失败仍返回清单且字段为 `null`；`GetAllSubnet` 方法体无 CRP；下拉响应不被 ListCountIP 残留值覆盖
