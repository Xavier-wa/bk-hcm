## ADDED Requirements

### Requirement: 注册 hcm_aiagent 指标子系统且无业务开关

系统 SHALL 在 `pkg/metrics` 新增 `AiagentSubSys = "aiagent"` 与 `pkg/metrics/aiagent.go`，按既有 `EnsureXxxMetric` + `ObserveXxx` 写法（与 `clb_submit` / `http_request` 相同）定义下列指标，命名空间 `hcm`：

- `hcm_aiagent_run_total`（Counter）：label `state`、`bkcc_biz_id`、`scene`。`state` 闭集为终态 `finished` / `error` / `unknown`，MUST NOT 包含 `running` 或 `cancel`。进行中的 run 由 `hcm_aiagent_run_inflight` 呈现
- `hcm_aiagent_run_duration_seconds`（Histogram）：label `state`（仅 `finished` / `error`）、`scene`
- `hcm_aiagent_run_inflight`（Gauge）：label `bkcc_biz_id`。MUST NOT 带 `scene`（`RUN_STARTED` 时 scene 常仍为空，终态才分类，+/-1 会对不齐）
- `hcm_aiagent_run_cancel`（Counter）：label `bkcc_biz_id`、`scene`

系统 MUST NOT 定义 `hcm_aiagent_run_observation_write_fail_total` 或等价写失败计数器。

业务 ID MUST 使用 `metrics.LabelBKCCBizID`（`bkcc_biz_id`），MUST NOT 使用 `bk_biz_id`。`scene` 拿不到或 tag 非法时 MUST 留空，MUST NOT 填 `unknown`。MUST NOT 把原始错误文本当 label。

agent-server MUST 在 `InitMetrics` 之后立刻调用 `EnsureAiagentMetric()`。打点函数 MUST 不返回错误、不做 IO。

系统 MUST NOT 为这些指标提供 `enabled` 配置。业务路径命中即打点，与现有 `hcm_clb_*` / `hcm_http_*` 一致。

#### Scenario: 启动后 /metrics 暴露元信息

- **GIVEN** agent-server 已 `InitMetrics` 并 `EnsureAiagentMetric`
- **WHEN** 访问 `/metrics`（尚未发生任何对话）
- **THEN** 响应 MUST 包含 `hcm_aiagent_run_total`、`hcm_aiagent_run_cancel` 等指标的 HELP/TYPE

#### Scenario: 业务 ID 使用 bkcc_biz_id

- **GIVEN** 一轮对话 `bk_biz_id=123`
- **WHEN** 打点 `run_total`
- **THEN** 序列 MUST 带 `bkcc_biz_id="123"`，MUST NOT 带 `bk_biz_id`

#### Scenario: 无 write_fail 指标

- **GIVEN** agent-server 已启动
- **WHEN** 抓取 `/metrics`
- **THEN** 响应 MUST NOT 包含 `write_fail_total` 或 `run_observation_write_fail`

#### Scenario: 未分类 scene 留空

- **GIVEN** 本轮 session_tag 为空、非法或 `unsupported`
- **WHEN** 打点 `run_total` 或 `run_cancel`
- **THEN** 序列 MUST 带空 `scene` 标签，MUST NOT 带 `scene="unknown"`

### Requirement: 事件回调就地打点且不等写库

在确认本轮元数据存在后，系统 MUST 在写库之前就地打点：

- `RUN_STARTED`：`inflight` +1（MUST NOT 打 `run_total`）
- `RUN_FINISHED`：`run_total{state="finished"}` +1，observe 耗时，`inflight` -1
- `RUN_ERROR`：`run_total{state="error"}` +1，observe 耗时，`inflight` -1

打点 MUST 只打一次，MUST NOT 放进异步写库重试循环。写库失败 MUST NOT 回滚已打的点。没有本轮元数据 MUST 不打点。

#### Scenario: STARTED 只加 inflight

- **GIVEN** `/agui` 本轮元数据存在
- **WHEN** 观测到 `RUN_STARTED`
- **THEN** `inflight{bkcc_biz_id=<本轮>}` MUST +1，MUST NOT 增加 `run_total`，且发生在插入 run 行之前
- **THEN** `inflight` MUST NOT 带 `scene` 标签

#### Scenario: inflight 按 bkcc_biz_id 拆开

- **GIVEN** 业务 11 与业务 22 各有一轮进行中
- **WHEN** 抓取 `hcm_aiagent_run_inflight`
- **THEN** 序列 MUST 分别带 `bkcc_biz_id="11"` 与 `bkcc_biz_id="22"`

#### Scenario: FINISHED 立刻计 finished 并减 inflight

- **GIVEN** 本轮 inflight 已 +1，且 scene_dispatch 已提交 `session_tag`
- **WHEN** 观测到 `RUN_FINISHED`
- **THEN** `run_total{state="finished",scene=<本轮 session_tag>}` MUST +1，耗时直方图 MUST 记一笔 `state="finished"`，`inflight` MUST -1
- **THEN** MUST NOT 使用请求进入时尚未打标的空 scene 作为本轮已分类场景的 `scene` 标签

#### Scenario: 写库失败不回滚指标

- **GIVEN** 事件回调已打 `finished`
- **WHEN** 随后 CAS 因 data-service 故障失败
- **THEN** `run_total{state="finished"}` MUST 保持刚打的值

### Requirement: cancel 只在 /cancel 计

系统 MUST 在 `POST /cancel` 鉴权与解析通过后立刻增加 `hcm_aiagent_run_cancel`，一次请求计一次。MUST NOT 查 `aiagent_run` 做跨副本 run_id 去重。MUST NOT 在 `/cancel` 上减少 `inflight`。MUST NOT 把 `cancel` 作为 `run_total` 的 `state`。

随后框架补发的 `RUN_FINISHED` / `RUN_ERROR` MUST 仍按事件回调计 `finished` / `error`。

#### Scenario: 点停止立刻计 run_cancel

- **GIVEN** 用户对进行中的对话 `POST /cancel` 且鉴权通过
- **WHEN** handler 解析完请求
- **THEN** `run_cancel` MUST +1，且 MUST 发生在写库之前

#### Scenario: 取消后补发 FINISHED 再计 finished

- **GIVEN** 该轮已计 `run_cancel`
- **WHEN** 框架补发 `RUN_FINISHED`
- **THEN** `run_total{state="finished"}` MUST 再 +1；`aiagent_run` 终态仍由写路径保持 `cancel`

#### Scenario: 重复 cancel 各计一次

- **GIVEN** 同一 run 连续两次合法 `/cancel`
- **WHEN** 两次请求都鉴权通过
- **THEN** `run_cancel` MUST +2

### Requirement: unknown 仅由清扫按影响行数计

`run_total{state="unknown"}` MUST 只在孤儿清扫任务内、按 CAS 实际更新行数逐条增加。MUST NOT 在事件回调里打 `unknown`。清扫 MUST NOT 修改 `inflight`。清扫任务自身执行情况 MUST 复用 `hcm_cron_*`，不新增任务级指标。

#### Scenario: 清扫 2 行成功则 unknown +2

- **GIVEN** 清扫扫描到 3 行 running，其中 2 行 CAS 成功、1 行已被别的终态抢先写掉
- **WHEN** 清扫结束
- **THEN** `run_total{state="unknown"}` MUST +2，MUST NOT +3

### Requirement: 耗时直方图只覆盖 finished 与 error

`hcm_aiagent_run_duration_seconds` 的 `state` MUST 只有 `finished` 与 `error`。起点 MUST 来自本轮元数据。`/cancel` 与孤儿清扫 MUST NOT observe 该直方图。

#### Scenario: error 记录耗时

- **GIVEN** 本轮元数据带开始时间
- **WHEN** 观测到 `RUN_ERROR`
- **THEN** 直方图 MUST 以 `state="error"` 记一笔秒级耗时

#### Scenario: cancel 不进耗时直方图

- **GIVEN** 用户 `/cancel`
- **WHEN** 取消打点完成
- **THEN** `run_duration_seconds` MUST NOT 因这次取消增加样本
