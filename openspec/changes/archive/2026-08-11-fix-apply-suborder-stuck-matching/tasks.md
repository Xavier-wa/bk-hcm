## 1. suspend 覆盖守卫

- [x] 1.1 `matcher.go` 的 suspend 覆盖分支增加 `matchedCnt < TotalNum` 守卫
- [x] 1.2 跳过覆盖时输出 warning 日志，带上挂起记录 generateID 与已交付/需求台数

## 2. 已足额交付时的直接收尾

- [x] 2.1 `retryMatchDevice` 原过滤循环内顺带累加已交付台数，供收尾条件判断
- [x] 2.2 `generator.go` 新增 `finalizeDeliveredOrder`：任取一条 `Success 且 is_matched=true` 记录作 `FinalApplyStep` 入参，挡掉在途/已可捞取两种情况；`Generator` 增加 `SetMatcher` 注入
- [x] 2.3 `retryMatchDevice` 在原过滤为空、已足额交付且无在途批次（`generatingCount` 早退保险 + finalize 内部复查）时直接调 `FinalApplyStep` 收尾，签名改为接收整张 `order`；`GenerateCVM`/`UpgradeCVM`/`MatchPM` 三处调用点同步更新；`scheduler.go` 装配处 `generate.SetMatcher(match)`

## 3. 验证

- [x] 3.1 gopls 无报错；`go build`/`go vet` 受阻于无法拉取的私有模块 `ngate-go`（本机既有环境问题，与本次改动无关），需在可拉取依赖的环境补跑
- [x] 3.2 对照 spec 逐条核对分支逻辑（含 102851-1 场景推演）
