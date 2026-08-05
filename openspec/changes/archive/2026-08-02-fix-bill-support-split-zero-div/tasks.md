## 1. 共享分摊计算

- [x] 1.1 新增 `allocateCommonExpense` 纯函数（比例分摊 / 零基数平均分摊 / 双零全 0）
- [x] 1.2 新增单测覆盖：常规比例、零基数平均分摊（含余数）、双零、空列表、部分账号为 0

## 2. 三云调用点接入

- [x] 2.1 改造 `aws_support.go` 的 `splitCommonExpense`：使用共享函数；双零短路；零基数 Warn 日志
- [x] 2.2 改造 `huawei_support.go` 的 `splitCommonExpense`：同上
- [x] 2.3 改造 `gcp_support.go` 的 `Split`：同上，并在空 summary 列表时直接成功返回

## 3. 验证

- [x] 3.1 运行 `monthtask` 包单测通过
