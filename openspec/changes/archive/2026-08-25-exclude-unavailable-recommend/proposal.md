## Why

自研云主机申领的静态推荐来自离线任务写入的用户/业务推荐表。历史上合法、已被写入的「需求类型 + 机型」组合，在用户真正申领时可能已被政策禁止（现网例子：小额绿通当前只允许标准型、16 核及以下，推荐仍打出 ITA5 + 小额绿通）。申请单已有机型组合校验，离线写入没有同一套判定，导致用户按静态推荐走下去会在提单阶段被拒。需要把申请单里的机型组合判定抽成函数，离线写入前调用，使下一轮任务成功后静态推荐不再给出当前申领不了的组合。

## What Changes

- **F-001**：离线聚合写入推荐表之前，剔除当前机型组合校验不通过的历史设备记录。口径 Q1=B：绿通机型政策（机型族白名单 + CPU 上限）+ 绿通/滚服禁止特殊型。
- **F-002**：过滤后某用户/业务分组无候选则本轮不写入、不占位；历史行按既有过期清理删除。
- **F-003**：被剔除记录逐条打 Warn 日志（需求类型、机型、剔除原因、rid）。
- **F-004**：申请单与离线推荐写入共用同一个机型判定方法 `CheckDeviceType`。调用方只传需求类型和机型主数据；配置读取在方法内部完成。申请单保留原有查机型与现网错误文案。
- 下一轮离线任务（或手动 `POST /api/v1/woa/apply_recommend/sync`）成功后，AI Agent 调用的**静态推荐**不再读到这些组合（Q2 经 Q7=A 收窄；Q3=B）。

### 本期包含

- 离线写入推荐表前，按与申请单同源的机型组合判定过滤历史记录（Q1=B）。
- 将申请单内层判定抽成 `CheckDeviceType`，申请单与离线都调用它（F-004）。
- 过滤后无候选的分组不写入、不占位（F-002）。
- 被剔除记录逐条 Warn 日志（F-003）。
- 下一轮任务成功后，静态推荐不再读到这些组合（Q2 经 Q7=A 收窄）。
- 生效等待下一轮离线任务或手动同步（Q3=B）。

### 本期不包含

- **不改 CVM 生产提单**（`cvm.go` / `CvmCreateReq`）。该路径继续用自己的内联校验。
- **不在在线读取/返回路径上过滤**（Q3=B）。
- **不改预测推荐**（Q7=A）。
- **不改申领页 Top-N 接口**。
- **不把完整提单前校验链**（额度、预测余量、实时容量、GPU 时长等）搬进离线任务。
- **不改推荐算法本身**（Top-K、去重键、先人后业务、预测内外回退均不变）。
- **不做机型替换 / 降级兜底**。
- **不清理**历史交付源表中的旧组合。
- **不新增或修改**推荐接口的请求/响应字段，不新增「不可用原因」，不改 Agent/页面空列表话术（Q4 默认）。
- **不改**拆单接口、提单接口的对外契约。
- **不覆盖**其他云厂商。

无 **BREAKING** 变更：对外推荐接口请求/响应结构不变；空列表仍成功返回。

## Capabilities

### New Capabilities

（无。本需求是既有 `apply-recommend` 能力的增量过滤，不引入新 capability。）

### Modified Capabilities

- `apply-recommend`: 申请单与离线写入共用同一个机型判定方法 `CheckDeviceType`（调用方只传需求类型和机型主数据，内部按需读配置）；离线聚合前用该方法过滤不可用组合（绿通机型政策 + 绿通/滚服禁止特殊型）；空分组不占位；被剔除记录逐条 Warn。不改 CVM 提单、不改在线读路径、不改预测推荐、不改算法、不改表结构/接口。

## Impact

- Affected specs: `apply-recommend`
- Affected layer: 仅 woa-server Service Layer / logics；不改 Access Layer、data-service、hc-service、前端。
- Affected code:
  - `cmd/woa-server/logics/task/scheduler`：判定方法 `CheckDeviceType`（内部用已注入的绿通 logics 按需读配置）
  - `cmd/woa-server/service/task/scheduler.go`：保留原机型查询与现网错误文案，一次调用抽出的方法
  - `cmd/woa-server/logics/applyrecommend/logics.go`：每轮按页批量查机型后按需求类型调用同一方法（非绿通/滚服直接通过；主数据未命中按「机型数据不存在」剔除），不允许则 Warn / continue
  - `cmd/woa-server/task/apply_recommend.go` / `service/service.go`：离线任务注入已有 `scheduler.Interface`
- 不改：`cmd/woa-server/service/cvm/cvm.go`、表结构、对外接口契约、读取侧 `recommend.go`、预测推荐、源表、推荐算法参数、IAM。
- 副作用：Top-N 读同一张推荐表，写入过滤后其结果也会变干净；这是写侧治理的自然结果，不是本期接口改动。
