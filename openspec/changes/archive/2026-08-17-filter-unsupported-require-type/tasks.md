## 1. 聚合前过滤不支持的项目类型

- [x] 1.1 在 `cmd/woa-server/logics/applyrecommend/logics.go` 的 `collectCounts` 循环中，于地域/镜像检查之后、构造计数 key 之前，对 `item.RequireType` 调用 `RequireType.Validate()`；失败则跳过该记录，不计入 `userCounts` / `bizCounts`
- [x] 1.2 跳过时输出 Warn 日志，字段至少包含记录 ID、`require_type` 取值与 rid，风格对齐现有 `skip invalid image`
- [x] 1.3 确认滚服 `RequireTypeRollServer=6` 能通过 `Validate()`，不会被本过滤剔除
- [x] 1.4 若 `collectCounts` 加上过滤后超过 80 行，将「是否跳过 + 打日志」抽成同文件私有函数，不重构拉取/补地域/查镜像

## 2. 过滤判定单测

- [x] 2.1 为 `require_type` 跳过判定补充表驱动单测：1/2/3/6/7/8/9 不跳过；0/4/5 及未知值跳过
- [x] 2.2 运行该包定向单测并通过

## 3. 回归与门禁

- [x] 3.1 确认未改读取侧、`writeUserRecommends` / `writeBizRecommends`、源表写入、表结构与对外接口
- [x] 3.2 `go build` 受影响包通过（至少 `./cmd/woa-server/logics/applyrecommend`）
- [x] 3.3 自测清单对齐 AC-001～007：脏类型不入库且无 `unsupported require type`；全合法类型结果不变；滚服写入保留；越界 Warn；整组不占位并走逾期清理；源表不变；`ApplyRecommendTop` / `ApplyRecommendByStatic` 结构不变且读侧仍不返回滚服
- [x] 3.4 `openspec validate filter-unsupported-require-type --strict` 通过
