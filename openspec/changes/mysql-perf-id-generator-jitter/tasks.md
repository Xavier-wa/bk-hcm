## 1. 修改 JitterUntil 增加启动延迟

- [x] 1.1 在 `pkg/tools/utils/wait/wait.go` 的 `JitterUntil` 入口增加启动延迟逻辑：基准延迟 `min(1min, period)`，`jitterFactor > 0` 时做二次抖动
- [x] 1.2 更新 `JitterUntil` 函数注释，说明启动延迟行为

## 2. 迁移 bill 包 controller 循环

- [x] 2.1 更新 `rootaccount_controller.go`：将 `time.NewTicker` 循环替换为 `wait.JitterUntil`，移除原有立即执行一次的调用
- [x] 2.2 更新 `mainaccount_controller.go`：同上
- [x] 2.3 更新 `summarydaily_controller.go`：同上
- [x] 2.4 更新 `dailysplit_controller.go`：同上

## 3. 验证

- [x] 3.1 执行全量编译，确认无编译错误