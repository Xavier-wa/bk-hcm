## 1. 抽出判定函数

- [x] 1.1 在已有主机申领 `scheduler` 上实现 `CheckDeviceType`（内部用已注入绿通 logics 按需读配置）；调用方不传 `gcLogics`、不直接 `GetConfigs`；不新加包
- [x] 1.2 离线在 `applyrecommend` 内自己批量查机型 map，空列表不打远程
- [x] 1.3 单测：判定表驱动覆盖绿通 ITA5 / 政策内标准型 / 特殊型 / 滚服非特殊型 / 开关关闭 / 常规项目；主数据未命中由离线 `checkDeviceTypes` 标「机型数据不存在」，不进入 `CheckDeviceType`
- [x] 1.4 判定与查询函数均不超过 80 行；不含额度、预测余量、实时容量

## 2. 调用方

- [x] 2.1 `scheduler.go` 的 `validateDeviceTypeForGreenAndRoll`：保留原机型查询与现网错误文案；对查询命中的机型一次调用 `CheckDeviceType`
- [x] 2.2 `applyrecommend` 的 `collectCounts`：对本轮机型按页批量查一次后按需求类型调用判定方法；主数据未命中标为「机型数据不存在」；`skipDeviceForCount` 之后按结果跳过，不允许则 Warn（id / require_type / device_type / reason / rid）并 continue
- [x] 2.3 `cvm.go` 不改
- [x] 2.4 离线循环内不批量查机型、不调用 `GetConfigs`；离线任务注入 `scheduler.Interface` 而非绿通 logics；读取侧 `recommend.go`、表结构、对外接口契约不变

## 3. 回归

- [x] 3.1 `go test ./cmd/woa-server/logics/task/scheduler` 与 `go test ./cmd/woa-server/logics/applyrecommend` 通过（含 `TestSkipDeviceForCount_RequireType`）
- [x] 3.2 `go build` 受影响包通过（task/scheduler、applyrecommend、含 service/task scheduler 的 woa-server）
- [ ] 3.3 自测：申请单现网文案不变；离线 ITA5 绿通不入库、政策内标准型写入、整组不占位、源表不变、配置/机型查询失败则任务失败、Warn 含 reason；`ApplyRecommendByPlan` / Top-N / 拆单契约不变；CVM 提单行为与基线一致
- [x] 3.4 `openspec validate exclude-unavailable-recommend --strict` 通过
