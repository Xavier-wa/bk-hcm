## Context

技术方案已定稿（iWiki 4031497429，含 D1–D11 与 §4.1–4.4 逐层代码落点），父需求文档已把 Q-001 ～ Q-007 七个问题全部拍板（D-001 ～ D-007），并逐条核对了技术方案引用的 14 处代码出处。本设计记录方案落地时**技术方案与父需求文档都没有给出确定答案、但不定就无法写代码**的那几处，以及若干与技术方案原文的明确偏离。

D-P07 ～ D-P11 是实现落地后回填的决策，其中 D-P07（多族并发）、D-P08（不做超量拉取）、D-P09（业务视角推荐接口进 MCP）三条与技术方案/父需求的原口径**相反**，各自的理由与代价写在对应小节。

现状要点（均已打开代码核实）：

- 现网只有单台校验 `CheckInheritedHost`（`cmd/woa-server/logics/task/scheduler/scheduler.go:3239`），按固资号精确查一台（`Page.Limit: 1`）。
- `calculateMonths` 是 `scheduler` 包私有函数（`scheduler.go:3475`），"不足一月补一月"的兜底在**调用处** `scheduler.go:3269-3272`。
- `scheduler` 包已 import `rollingserver "hcm/cmd/woa-server/logics/rolling-server"`（`scheduler.go:35`），并在 struct 上持有 `rsLogics`。
- `rolling-server` 的 logics 已持有 `cmdbClient` 与 `configLogics`（`logics/rolling-server/service.go:105-113`）。
- `service/task` 的 `service` 已持有 `configLogics config.Logics` 与 `logics taskLogics.Logics`（`service/task/service.go:65-76`），提单链路已在用 `s.logics.Scheduler().CheckApplyQuota(...)`（`service/task/scheduler.go:653`）。
- `CheckApplyQuota`（`scheduler.go:1065`）的签名是 `(kt, bizID, requireType, suborders) error`，额度不足时把 `reason` 包成 `fmt.Errorf("%s", reason)` 返回。
- `ApplyRecommendSuborder`（`pkg/api/woa-server/cvm_apply_recommend.go:192`）当前**没有** `charge_months` 字段；`CheckInheritedHostResp.ChargeMonths` 是 `int`、`BillingStartTime` 是 `time.Time`；提单落库用的 `ResourceSpec.ChargeMonths` 是 `uint`（`int` → `uint` 的负数转换会溢出，见 D-P02 与 spec 的"非正月数不下发"）。
- api-server 内置 MCP 白名单已有 `list_biz_*` pattern（`api_server.yaml:120`），任何 `list_biz_` 前缀的新 operationId 都会被自动放开。
- `docs/support-file/helm/values.yaml` 的 `mcp.internal.servers` 默认是 `[]`，`includeOperationIDs` 只以注释示例形式存在；真实白名单在 `cmd/api-server/etc/api_server.yaml:109-120`。

## Goals / Non-Goals

**Goals:**

- 让页面链路与 AI 链路复用同一份候选查询实现，CMDB 调用次数由调用方传入的机型族列表显式控制（== 去重后的族数）。
- 让推荐链路与现网校验链路的 `charge_months` 出自**同一份代码**，且提取过程不改变现网 `check/apply/order/host` 的返回值。
- 让 `by_static` / `split_suborder` 支持滚服，同时保证六种非滚服 `require_type` 的响应逐字段不变。
- 在"额度不足静默丢弃"与"额度服务故障必须报错"之间给出可实现的区分方式，且不改动提单/审批链路的现有调用点。

**Non-Goals:**

- 不改现网 `check/apply/order/host` 的入参、出参、六条校验规则与 CRP 地域判定（D2、D8）。
- 不让 `by_plan` 为滚服产出推荐方案（D4）——仅把原先的参数校验报错改为返回空 `items`。
- 不做推荐结果落表/缓存（D1）、不做跨请求缓存、不做并发互斥。
- 不实现 `reason` 外露（见下方 DEV-001）。
- 不做前端页面、不做 AI skill 侧对话逻辑、不做 AI 场景 2 的磁盘默认值继承（D-004）。

## Decisions

### D-P01 `calculateMonths` 的共享位置：落在 `logics/rolling-server` 并由 `scheduler` 正向调用

`scheduler` 包已经 import 了 `logics/rolling-server`，所以新的 rolling-server logic **不能**反向 import `scheduler`——否则形成包循环依赖。这排除了"把 `calculateMonths` 导出为 `scheduler.CalculateMonths` 供 rolling-server 调用"这个最省事的方案。

决策：把该算法（连同"不足一月补一月"的兜底）放进 `cmd/woa-server/logics/rolling-server`，导出为 `CalcRemainMonths(from, expire time.Time) int`，`scheduler` 沿已有的 import 方向调用它。依赖方向与现状一致，一行 import 都不用新增。

- 兜底必须**一并搬进函数体**：现网兜底写在调用处（`scheduler.go:3269-3272`），若只搬主体不搬兜底，推荐链路会比校验链路少一个月，直接违反 AC-010 / AC-011。
- 现网调用处 `time.Now()` 出现两次（一次算月数、一次做兜底比较）；提取后统一用同一个 `from` 参数。这是**行为等价的归一化**（两次 `time.Now()` 相差微秒级，不会跨越自然日），同时让函数可测。
- 函数不做零值兜底：到期时间缺失（按量计费）或已过期时返回负数，与提取前的现网行为逐值一致。单测把这个负数结果显式钉住，避免后人"顺手改成返回 0"。
- 备选方案一：提到中立公共包 `pkg/tools/times`。否决理由——"剩余套餐月数（含不足一月补一月）"是滚服继承的业务口径而非通用时间计算，放进通用工具包会让它看起来可以被任意模块复用，实际上换个业务语义就是错的。
- 备选方案二：新建 `cmd/woa-server/logics/shared` 之类的中间包。否决理由——只为一个函数新增一层包，且该函数的唯一两个调用方都已经在现有依赖链上。

### D-P02 五个滚服字段的具体类型：`omitempty` 对 `time.Time` 无效，必须用指针

契约子需求文档写"`billing_start_time` / `billing_expire_time` 用 `time.Time` 且带 `omitempty`"，技术方案 §4.3(1) 写的是 `string`。两者都有问题或都不够精确：Go 的 `encoding/json` 对 **struct 类型的 `omitempty` 不生效**，`time.Time` 零值会被序列化成 `"0001-01-01T00:00:00Z"` 出现在响应里，直接违反 AC-026 / AC-C05（六种非滚服 `require_type` 的响应不得含这五个键）。

决策：

| 结构 | 字段 | 类型 | 理由 |
|---|---|---|---|
| 推荐接口响应候选元素 | `charge_months` | `int` | 对齐 `CheckInheritedHostResp.ChargeMonths`（`int`），AC-010 / AC-011 要逐条比对两个接口的数值，同类型才是同口径 |
| 推荐接口响应候选元素 | `billing_start_time` / `billing_expire_time` | `time.Time` | 该响应里这两个字段恒有值，不需要 `omitempty`，用值类型最简单 |
| `ApplyRecommendSuborder` / `ApplyRecommendSplitSubOrderReq` | `charge_months` | `uint` | 该值最终透传进提单 `ResourceSpec.ChargeMonths`（`uint`），同类型免转换；数值零值命中 `omitempty` |
| `ApplyRecommendSuborder` / `ApplyRecommendSplitSubOrderReq` | `billing_start_time` / `billing_expire_time` | `*time.Time` | 指针 nil 才能命中 `omitempty`；值/指针转换统一用 `cvt.ValToPtr` / `cvt.PtrToVal` |

备选方案：两个时间字段在 suborder 上用 `string`（技术方案原文写法）。否决理由——推荐链路内部一路是 `time.Time`，落到 suborder 转字符串再由提单侧解析回来，多两次格式转换且格式约定容易漂移；`*time.Time` 同样满足 `omitempty`，且类型安全。

### D-P03 额度预检用类型化错误区分"额度不足"与"额度服务故障"，不新增出口

父需求「给研发的实现提示」第 4 条说额度预检可复用已暴露的 `CheckApplyQuota`（`scheduler.go:1065`），不必重写四步。但该出口只返回 `error`：额度不足返回 `fmt.Errorf("%s", reason)`，`CanApplyHost` / `GetCpuCoreSum` / `IsResPoolBiz` 调用失败也返回 `error`，调用方**无法区分**。而 AC-022 要求额度不足时静默丢弃候选（HTTP 200），AC-R01 要求额度服务故障时整个请求返回 error 且不静默放行——两条 AC 用同一个 `error` 值无法同时满足。

决策：不改签名、不加方法，改为把"额度不足"这件事**类型化**。新增 `types.QuotaInsufficientError{Reason string}`（落 `cmd/woa-server/types/task/scheduler.go`），`checkRollingApplyQuota` 与 `checkGreenChannelApplyQuota` 在 `!canApply` 时返回该类型而非 `fmt.Errorf`；推荐链路用 `errors.As` 识别后静默丢弃并记 Warn，识别不到则透出。

- 四步逻辑（`IsNeedQuotaManage()` → `IsResPoolBiz()` → `GetCpuCoreSum()` → `CanApplyHost()`）仍然只有**一份**实现，符合"不必重写四步"的口径；
- 提单链路与审批建单前置校验的现有调用点**一个字都不用改**：它们只判 `err != nil`，而 `*QuotaInsufficientError` 就是 error；
- 顺带把这两处"额度不足"的日志级别从 `logs.Errorf` 降为 `logs.Warnf`——额度不足对推荐链路是正常过滤条件，是否算错误应由调用方决定，被调方不该抢先定性。
- 绿通分支一并类型化：它与滚服分支形态相同，只改一个会让同一个入口的错误语义分裂。

备选方案一：新增 `PreCheckApplyQuota(kt, bizID, requireType, suborders) (canApply bool, reason string, err error)` 三元组出口，让 `CheckApplyQuota` 委托它。否决理由——接口上多一个只有单一调用方的方法，且两个出口的语义差别只体现在返回值形态上，后人容易调错那个；类型化错误用 `errors.As` 表达同一件事，不扩接口。

备选方案二：推荐链路直接调 `CheckApplyQuota`，把任何非 nil error 都当作"丢弃"。否决理由——额度服务抖动会被误判成"额度不足"，静默放行/静默丢弃都属于安全需求「额度防透支」明确禁止的行为，且直接违反 AC-R01。

### D-P04 `is_recommended` 在 Handler 层打标，logic 出参不含该字段

候选查询 logic 被两个调用方共用：页面链路（要 `is_recommended`）与 `by_static` 的候选补全（只取首条，不关心该标记）。把这个**展示语义**放在 logic 出参里，会让 `by_static` 拿到一个无意义的标记字段。决策：logic 出参类型 `InheritedHost` 只含数据字段，接口响应的候选元素 `InheritedHostCandidate` 内嵌 `InheritedHost` 再加 `is_recommended`，由双视角 Handler 在组装分组时打标。两个类型都落 `pkg/api/woa-server`，logic 直接产出 API 类型，省掉一层同构结构体与来回转换。

### D-P05 AI 侧的机型族反查走 `service/task` 已有的 `configLogics`，候选查询走新增的 `rsLogics`

`by_static` 的补全需要两样东西：把候选的 `device_type` 反查成机型族（`device_group`），以及按机型族查候选。前者 `service/task` 的 `service` 已持有 `configLogics`，直接调 `s.configLogics.Device().ListCvmInstanceInfoByDeviceTypes` 即可，**不需要**为此新增依赖；后者需要 `logics/rolling-server` 的新方法，因此 `service` 结构体新增 `rsLogics` 字段（注入方式照 `gcLogics`）。额度预检走已有的 `s.logics.Scheduler()`，也不需要新增依赖。

注意两侧查 `device_type` 表的方向相反、走的方法也不同：推荐 logic 是"族 → 机型名单"，用 `ListDistinctDeviceType` 把 `vendor` / `device_family` / `device_type_class` 三个条件下推到 DB 并翻页取全量；`by_static` 是"机型 → 族"，用 `ListCvmInstanceInfoByDeviceTypes` 按机型批量取回 `DeviceGroup`。表字段名是 `device_family`，`ListCvmInstanceInfoByDeviceTypes` 出参里对应的字段名是 `DeviceGroup`（`device_group`），同一个东西两个名字，写代码时容易搞混。

### D-P06 MCP 白名单"成对提交"在 helm 侧的落法

`docs/support-file/helm/values.yaml` 的 `mcp.internal.servers` 默认值是空数组 `[]`，`includeOperationIDs` 只以**注释示例**形式存在；真实生效的白名单在 `cmd/api-server/etc/api_server.yaml:109-120`。因此"成对提交"的具体含义是：`api_server.yaml` 改真实白名单，helm `values.yaml` 改注释示例（让照示例配置的部署方拿到同一条 pattern）。两处 pattern 文本必须一致（AC-M03）。

pattern 落定为 `check_biz_apply_*` 而非 `check_*`：后者会把 openapi spec 中所有 `check_` 前缀 operationId 一次性放开，暴露面不可控。实现时必须先列出 spec 中全部 `check_` 前缀 operationId 再定 pattern（AC-M04 就是这条的验收）。

### D-P07 logic 签名接**多个**机型族并在内部并发，不是单族串行

技术方案与父需求都写"签名接单个机型族，页面链路逐族循环调用"。实测这个形态把并发决策权推给了每一个调用方：页面 Handler 要自己写循环、`by_static` 要自己写循环，将来第三个调用方还要再写一遍，而"要不要并发、并发多少"是查询实现自己的事。

决策：`Logics.ListInheritedHosts(kt, bkBizID, region, deviceFamilies []string) (map[string][]*InheritedHost, error)` 接切片、返回以机型族为 key 的 map，内部用 `errgroup` 并发逐族查，并发度由 `constant.RsInheritedHostQueryConcurrency`（10）封顶，写 map 时加锁。单族查询下沉为包内私有函数 `listFamilyInheritedHosts`。

- 调用方形态统一：页面 Handler 传全部机型族，`by_static` 传单元素切片，两边都不写循环、不写并发。
- 入参 `slice.Unique` 去重，CMDB 调用次数 == 去重后的族数；Handler 仍按入参原顺序（含重复）组装分组，对外语义不变。
- 并发度必须封顶：`device_families` 上限是 100，不封顶就是 100 路并发打 CMDB。10 这个值让 5 族的典型场景一轮打完，同时给 CMDB 留余量。
- 任一族失败整体失败：`errgroup.Wait()` 返回首个 error，直接满足"不返回部分结果"的约束，比串行循环里手写 break 更难写错。
- 这是对原方案「串行、SHALL NOT 并发」的**明确偏离**。原方案的理由是"调用次数由调用方显式控制"，但调用次数由传入的族列表决定，与串行/并发无关——并发只影响墙上时间。P95 < 3s 的指标因此更容易达成，不再需要靠"逐族串行也够快"这个假设。

### D-P08 CMDB `Limit` 直接取对外上限 5，不做超量拉取与本地二次剔除

一版设计曾打算 `Limit` 取 20、拿回后在本地剔除无效候选（已到期、无计费起始时间、剩余月数 < 1）再截断到 5，理由是"最早计费的机器最容易已到期，只拉 5 条可能被剔空"。

决策：不做。`Limit` 直接取 `constant.RsInheritedHostReturnLimit`（5），CMDB 返回什么就返回什么，`charge_months` 允许为 0 或负数。

- 本地剔除会让"每族最多 5 条"变成"每族 0～5 条且条数不可预期"，前端拿到 3 条时无从判断是真的只有 3 台还是被剔掉了 2 台。
- 已到期固资并非"必然不可用"：能不能继承由现网 `check/apply/order/host` 的六条校验判定，推荐接口自己再造一套可用性口径，两套口径迟早对不上（这正是 AC-017 在盯的那个风险）。
- 少拉 15 条 × 每族，对 CMDB 更友好。
- 代价：某机型族的前 5 条可能全是已到期固资，用户点进去校验才发现不可用。这是已接受的取舍——`is_recommended` 标记与 `charge_months` 已经把信息给到前端，由前端决定怎么呈现。

### D-P09 业务视角推荐接口进 MCP，与「推荐接口不进 MCP」的原口径相反

原口径（D11）是推荐接口一律不进 MCP，理由是 AI 侧的固资由 `by_static` 在服务端内部补全，不需要单独的推荐工具。

决策：把**业务视角**推荐接口（`list_biz_rolling_server_inherited_hosts`）加进 MCP 资源文件，资源视角不加。`by_static` 的服务端内部补全保留，两者互为补充。

- 服务端内部补全只会闷头取首条，用户看不到自己继承的是哪台、也没得选。固资是"以旧换新"里用户真正在意的那个选择项，AI 场景 1（用户不指定固资号）下应当能列出候选并向用户确认。
- 该 operationId 命中白名单里**既有**的 `list_biz_*` pattern，不需要新增 pattern——这也意味着暴露是"命名自动生效"的，必须显式确认而不是默认接受，因此 spec 里补了一条"新增 operationId 的命中确认"约束。
- 资源视角不进：AI 链路一律带业务上下文，资源视角只服务管理端页面。

### D-P10 滚服拆单入参的第六个字段 `charge_type` 与两条附加校验

契约冻结时说的是五个滚服字段。落地时发现拆单**请求体**还缺 `charge_type`：滚服子单的计费模式不再由预测内外推导，而是取自入参，没有这个字段拆单无处取值。因此对外冻结的拆单入参实际是**六个**字段。

同时补两条滚服专属校验（原设计只有 `inherit_instance_id` 非空一条）：

- `charge_type` 必须合法（`PREPAID` / `POSTPAID_BY_HOUR`）：留空会让子单落到空计费模式，一路带到提单才炸。
- `charge_type == PREPAID` 时 `charge_months >= 1`：包年包月子单没有购买时长无法提单。按量计费无套餐时长，不校验。

这两条是"参数校验层拦住必然失败的提单"，而非新增业务规则；缺字段时明确指出缺哪个，不静默降级。

### D-P11 `by_static` 的补全在攒够 `limit` 时提前终止，并按 `(region, 机型族)` 缓存固资

补全环节对每个滚服候选都要查一次固资、做一次额度预检，两者都是外部调用。两条收敛：

- **按 `(region, 机型族)` 缓存**：同一机型族下有多个机型，反查出来的族是同一个，固资候选也就是同一批。单次请求内同一组合只查一次 CMDB。
- **攒够 `limit` 个保留候选即停**：`assembleStaticPlans` 本来就只取前 `limit` 个，后面的候选无论去留都排在 `limit` 名之后，继续查固资与额度不会改变出参，只会白花外部调用。

代价：候选列表靠后的滚服候选不会被查证，因此"某个候选究竟是被丢弃还是没轮到"在日志里看不出区别。这个信息对排障价值有限，不值得为它多打十几次 CMDB。

### DEV-001 不实现 `reason` 外露（与技术方案 §4.3(4) 的偏离）

技术方案 §4.3(4) 原文写额度预检的 `reason`"带进提示语"。但候选被丢弃后，`by_static` 的响应里没有承载 `reason` 的字段——该表述与同一份技术方案的 D6「超额候选不推」自相矛盾。依据用户对父需求 Q-002 的决策（选 A 静默丢弃）：额度预检四步逻辑照做不变，`reason` **只记服务端日志**，响应结构一个字段都不加（不加 `filtered_reasons`、不加 `message`）。代价：滚服额度耗尽时 AI 只能给出"暂无可推荐方案"这类泛化提示，说不出具体原因。

## Risks / Trade-offs

- **[`calculateMonths` 提取动到现网 `scheduler` 包]** → 提取必须是纯重构。用改动前后同一批固资号分别调现网 `check/apply/order/host`，逐条比对 `charge_months` 与 `new_billing_expire_time`（AC-L01）；兜底与主体必须一起搬，不能只搬一半。
- **[地域判定的双口径]** → 推荐链路用 CMDB `bk_cloud_region`，现网校验链路用 CRP `CloudRegion`（D2 不改）。两者理论上可能不一致，届时表现为"推荐出来的候选校验不过"。缓解：AC-017 要求对一次推荐的全部候选逐条调校验接口，失败条数必须为 0；AC-018 要求把不一致的发生次数记录并反馈，不得静默忽略。若实测有差异，需回头调整 D2——这是本设计最大的未验证假设。
- **[改动现网 AI 提单主链路 `recommend.go`]** → `by_static` 与 `split_suborder` 都在跑。缓解：补全与短路逻辑全部走 `require_type == RollServer` 分支，非滚服路径不进任何新代码；AC-026 / AC-M01 逐字段核对六种非滚服 `require_type` 的响应。
- **[`by_plan` 对滚服由报错改为返回空 `items`]** → 三处滚服拒绝分支全部移除后，参数校验层不再区分需求类型，"滚服不产出预测推荐"这条约束只剩 handler 一处短路。缓解：AC-024 / AC-025 / AC-M02 三条 AC 改为核对空列表语义（200 + 空 `items`、不发起预测池与库存查询），并保留 `TestApplyRecommendByPlanReq_ValidateAcceptRollServer` 守参数校验放行。
- **[`assembleStaticPlans` 需要接收补全后的固资信息，签名要改]** → 该函数同时服务非滚服路径。缓解：新增参数走可选/映射形态，非滚服候选取不到固资信息时行为与改动前一致。
- **[额度预检按单候选独立校验，不做跨候选累加]** → 用户拿到的多个方案理论上不能同时提交（父需求假设 A-002：用户最终只会选一个）。这是已接受的口径，AC-R02 就是这条的正向验收。
- **[两条 MCP 通道的验证依赖外部部署]** → APIGW 资源需网关侧发布、内置 MCP 需重启 api-server 才能验证。缓解：两条通道各自独立验收（AC-040 / AC-041），任一未通不阻塞另一条的交付判定。
- **[`by_static` 的滚服候选依赖静态推荐表历史记录]** → 从未提过滚服单的业务在 AI 链路上不产生滚服候选（D-003 已接受）。不做反向生成兜底，降级话术归需求 1069995598136239422。
- **[改动共享额度入口的错误类型]（D-P03）** → `CheckApplyQuota` 的返回值由 `fmt.Errorf` 变成 `*QuotaInsufficientError`，提单预检与审批建单前置校验两处都在跑。缓解：两处只判 `err != nil`，类型变化对它们透明；核对时确认没有任何调用点在比对 error 字符串。
- **[逐族并发查 CMDB]（D-P07）** → 单请求最多同时打 `RsInheritedHostQueryConcurrency` 路 CMDB，机型族多时对 CMDB 的瞬时压力高于串行。缓解：并发度硬编码封顶，不随入参放大；AC-P01 的 P95 测量改为在并发实现上做，同时观察 CMDB 侧有无限流。
- **[不剔除已到期候选]（D-P08）** → 推荐出的候选可能校验不过，用户点进去才知道。缓解：与"地域判定双口径"共用 AC-017 的验收（逐条调校验接口，失败条数为 0）；若实测失败条数不为 0，再决定是补本地剔除还是收紧筛选条件。
- **[业务视角推荐接口进 MCP]（D-P09）** → 该接口返回 `bk_asset_id` / `bk_host_innerip`，进 MCP 意味着这些业务敏感字段会流入 AI 上下文。缓解：接口本身按 `meta.Biz` / `meta.Access` 鉴权，AI 调用同样受该鉴权约束；资源视角不进 MCP。

## Migration Plan

无 DDL、无数据迁移、无配置默认值变更。部署顺序：

1. 代码变更（woa-server）可独立发布——新增字段全部带 `omitempty`，对已上线的 AI skill 与前端是纯增量。
2. `cmd/api-server/etc/api_server.yaml` 与 helm `values.yaml` 的白名单 pattern 需重启 api-server 生效。
3. 两份 APIGW 资源文件（对外 `bk_apigw_resources_bk-hcm.yaml` 与内置 MCP `bk_apigw_resources_bk-hcm_internal_mcp.yaml`）需网关侧发布 / api-server 重启才生效，与 woa-server 发布无先后依赖。

回滚：代码回滚即可，无残留状态（推荐结果不落表、不缓存）。配置回滚为删除新增的 pattern 行与资源定义。

## Open Questions

无。父需求的 Q-001 ～ Q-007 已全部拍板；本设计的 D-P01 ～ D-P11 为落地决策，已在上方给出理由与被否决的备选方案。
