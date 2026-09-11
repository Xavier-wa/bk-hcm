## 1. 注入 Authorizer 与主动鉴权

- [x] 1.1 扩展 `newCreateCvmApplyGate(clientSet, authorizer)`，增加可打桩 `authorizeFunc`（kit 级资源属性 → authorized bool, error）与 `applyURLFunc`；默认实现调 `auth.Authorizer.Authorize` / `GetPermissionToApply` / `GetApplyPermUrl`；资源属性写死为 `meta.Biz` + `meta.Create` + 当前 bizID（与 `cmd/woa-server/service/task/check.go` 第 57–59 行一致）；单函数 ≤80 行
- [x] 1.2 将 authorizer 沿 `builtinGates` → `GetEnabledGateHandlers` → `buildHITLRegistry` → `BuildGraph` / `buildHostApplySubgraph` → `newAGUIRunner` / `Runtime.New` → `service.go` 的 `logics.New(apiClientSet, authorizer)` 传递；`GetEnabledGateToolNames` 对 authorizer 传 nil；resource_query 的 `buildHITLRegistry` 只改签名
- [x] 1.3 `onConfirm` 在 `validateApply` 之前做主动鉴权：无权限则拒绝且不调 woa 预检；`Authorize` error 或 authorizer 为 nil 则 fail closed（拒绝、不调 woa、不带 `apply_url`、不放行）
- [x] 1.4 鉴权主体只取请求上下文的蓝鲸用户名（`newAuthKit`）：申领参数中的提单人（`bk_username`）与 `core.NewBackendKit` 预置的后端操作用户都不得顶替判定主体；上下文缺用户名时 fail closed；`callWoaCheck` 的现网身份回退（`newGateKit(ctx, req.User)`）保持不变

## 2. 申请地址与拒绝契约

- [x] 2.1 无权限主路径：对同一资源属性生成 `apply_url`；空串 / 非 http(s) / 调用失败记 `Warnf`（bizID 与 rid），走降级，不得当系统异常 return
- [x] 2.2 无权限且 URL 可用：tool `Content` 为 JSON（`code`=2030403、`message` 含账号与业务、非空 `apply_url`），不写 `permission`
- [x] 2.3 无权限且 URL 不可用：同一 JSON，省略 `apply_url`，`message` 为降级说明
- [x] 2.4 窄兜底：主动鉴权已通过但 woa 返回 `errf.PermissionDenied` 时走与主路径相同的无权限 JSON，不得拼「暂时无法提单」；其它 woa error / `Pass=false` / 取消 / 缺 bizID 保持纯文本且无 `apply_url`

## 3. 单测

- [x] 3.1 主动判定无权限（authorize 桩返回 false）且 URL 可生成：`code=2030403`、非空 `apply_url`、`Next != tool`，且 woa check 桩未被调用（AC-001 / AC-007 / AC-S01）
- [x] 3.2 主动判定无权限但 URL 失败或空串：无 `apply_url`、有降级说明、不是系统异常文案、不放行、不调 woa（AC-002 / AC-008）
- [x] 3.3 `Authorize` 超时/报错：拒绝、不调 woa、无 `apply_url`、不标成 2030403 引导、不放行（D5）
- [x] 3.4 主动鉴权通过 + `Pass=false`（额度/参数）、取消确认、文案含「权限」但鉴权为通过：无 `apply_url`、不标成无权限（AC-003 / AC-004）
- [x] 3.5 主动鉴权通过 + woa `Pass=true`：放行且无 `apply_url`（AC-006）；断言鉴权入参为当前 bizID 且 `Biz`+`Create`（AC-005 / AC-S02）
- [x] 3.6 主动鉴权通过 + woa 返回 2030403：走无权限引导兜底，不放行（D4）
- [x] 3.7 更新 `registry_test.go` / `graph_build_test.go` 等因签名变化的调用；`go test -count=1 ./cmd/agent-server/logics/agent/toolgate/` 与受波及的 graph 测试，现有放行 / 取消 / proxy 信封用例保持绿灯
- [x] 3.8 上下文缺蓝鲸用户名（申领参数里带有权限账号）：拒绝放行，authorize / woa check / applyURL 三个桩均未被调用，文案为权限校验失败且不带 `apply_url`（D5）

## 4. 自检

- [x] 4.1 `gofmt` 改动文件；日志符合项目规范（小写开头、`err: %v` 在前、`rid: %s` 在末）
- [x] 4.2 确认未改 woa-server、`pkg/client/common/request.go`、auth-server 实现、前端、helm / etc yaml、权限点；未删除 woa `AuthorizeWithPerm`
