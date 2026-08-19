# 任务分组说明

四个分组一一对应四张 TAPD 子需求单，按波次分批实现、每批单独评审。组间依赖：组 1 → {组 2, 组 3, 组 4}，组 2 → 组 4（组 4 依赖组 2 的**运行时产物**——继承固资候选查询 logic 的实现，不只是契约）。同波次的组 2 与组 3 相互独立，可并行。

| 组 | 波次 | TAPD 子需求单 | 名称 | 依赖 |
|---|---|---|---|---|
| 组 1 | Wave 1 | `1069995598137047692` | 继承固资推荐接口契约与滚服字段定义 | 无 |
| 组 2 | Wave 2 | `1069995598137047885` | 继承固资候选查询与双视角推荐接口 | 组 1（契约） |
| 组 3 | Wave 2 | `1069995598137048187` | 滚服拆单适配与 MCP 双通道暴露 | 组 1（契约） |
| 组 4 | Wave 3 | `1069995598137048845` | 静态推荐放开滚服与额度预检 | 组 1（契约）+ 组 2（运行时） |

组 1 是 Enabler，交付物是可编译的契约与两份文档，**不要求可演示**；端到端主路径由组 2 首次跑通。

---

## 1. 组 1（Wave 1 · 子单 1069995598137047692）继承固资推荐接口契约与滚服字段定义

- [x] 1.1 新建 `pkg/api/woa-server/rolling_server_inherited_host.go`，定义推荐接口请求结构体：`bk_biz_id`（`int64`，`required,gt=0`）、`region`（`string`，`required,max=128`）、`device_families`（`[]string`，`required,min=1,max=100,dive,required`）；`Validate()` 先调 `validator.Validate.Struct(r)` 再补业务校验。请求结构体 MUST NOT 含任何搜索类字段。
- [x] 1.2 在同文件定义响应结构体：`info` 为分组列表，分组含 `device_family`（`string`）与 `hosts`（候选切片，**不带 `omitempty`**，无候选时初始化为长度 0 的切片以序列化为 `[]`）；候选元素含 `bk_asset_id`、`bk_host_innerip`、`bk_cloud_inst_id`、`device_type`、`instance_charge_type`（`string`）、`billing_start_time` / `billing_expire_time`（`time.Time`）、`charge_months`（`int`，对齐 `CheckInheritedHostResp.ChargeMonths`）、`is_recommended`（`bool`）。响应结构体 MUST NOT 含 `total` / `count` / 分页游标。
  > 交付口径：候选拆成两个类型——`InheritedHost` 只含八个数据字段、作为 logic 出参；`InheritedHostCandidate` 内嵌 `InheritedHost` 再加 `is_recommended`、作为响应元素。两者都落 `pkg/api/woa-server`，logic 直接产出 API 类型，省掉一层同构结构体与转换。
- [x] 1.3 在 `cmd/woa-server/logics/rolling-server` 的 `Logics` 接口上声明继承固资候选查询方法签名：入参 `kt`、业务 ID、`region`、`deviceFamilies []string`；出参以机型族为 key 的 `map[string][]*woaserver.InheritedHost` 与 error。出参元素 **MUST NOT 含 `is_recommended`**（该标记由 Handler 层打，见 design D-P04）。本组只声明签名与类型，实现留给组 2。
  > 交付修正：原条目要求签名接**单个**机型族、由调用方逐族循环。改为接切片、并发在 logic 内部收敛（见 design D-P07）：调用方不再各写一遍循环与并发决策，`by_static` 传单元素切片即可复用同一入口。
- [x] 1.4 `pkg/api/woa-server/cvm_apply_recommend.go` 的 `ApplyRecommendSuborder` 新增五个滚服字段：`charge_months`（`uint`，对齐提单落库的 `ResourceSpec.ChargeMonths`）、`bk_asset_id`（`string`）、`inherit_instance_id`（`string`）、`billing_start_time` / `billing_expire_time`（**`*time.Time`**，见 design D-P02：`omitempty` 对 `time.Time` 值类型不生效），五个字段全部带 `omitempty` 并加注释说明"仅滚服项目（`require_type=6`）填充"。
- [x] 1.5 `ApplyRecommendSplitSubOrderReq` 同步补上述五个字段，类型与 1.4 逐一相同；为 `ChargeType` 字段补注释说明滚服场景取自继承固资的 `instance_charge_type`、不再写死 `PREPAID`（实现归组 3/组 4）。
  > 交付修正：本条只覆盖了子单产物 `ApplyRecommendSuborder` 的 `ChargeType`，遗漏了拆单**请求体** `ApplyRecommendSplitSubOrderReq` 的同名字段，导致 3.3「`charge_type` 取自入参透传」无处取值（spec 明确要求请求可透传 `charge_type`）。该字段已由组 3 补齐为 `ChargeType cvmapi.ChargeType` + `charge_type,omitempty`。**因此对外冻结的拆单入参实际是六个滚服相关字段，而非五个**，通知前端与 AI skill 侧时须按六个字段的口径（见 design D-P10）。
- [x] 1.6 写单测验证 `omitempty` 生效：构造 `require_type=1`、五字段为零值/nil 的 `ApplyRecommendSuborder`，序列化后 JSON 中不含这五个键；构造 `region` 为空与 `device_families` 为空数组的推荐请求，`Validate()` 返回的 error 信息分别包含 `region` 与 `device_families` 字段名。
- [x] 1.7 编写业务视角接口文档 `docs/api-docs/web-server/docs/biz/`（路径 `POST /api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list`），格式参照 `docs/api-docs/web-server/docs/resource/` 下现有文档；版本填 v9.9.9+；含入参表、出参表、请求/响应示例、错误码（2000001 `errf.InvalidParameter`、鉴权失败）；标注 `bk_asset_id` / `bk_host_innerip` 为业务敏感信息。
- [x] 1.8 编写资源视角接口文档 `docs/api-docs/web-server/docs/resource/`（路径 `POST /api/v1/woa/rolling_servers/inherited_hosts/list`），内容与 1.7 一致，仅路径与 `bk_biz_id` 取值来源（body 而非路径）不同。
- [x] 1.9 两份文档的响应示例**另造一组算术自洽的数据**：`charge_months` 必须等于示例编写时点到示例 `billing_expire_time` 的剩余月数。**不得照抄**技术方案 §4.2 中 `billing_start_time=2024-06-10` / `billing_expire_time=2027-06-10` / `charge_months=8` 那组相差 36 个月的数据。
- [ ] 1.10 执行 `go build ./...` 与 `go vet ./pkg/api/woa-server/... ./cmd/woa-server/logics/rolling-server/...` 确认编译通过；契约冻结并通知前端子需求 1069995598136963986 与 AI skill 侧需求 1069995598136239422 可据此开工。通知时须带上两处与初版契约的差异：拆单入参是**六个**滚服字段（含 `charge_type`，见 1.5），`is_recommended` 的语义是"计费满 36 个月"而非"每族第一条"（见 2.9）。

## 2. 组 2（Wave 2 · 子单 1069995598137047885）继承固资候选查询与双视角推荐接口

> 依赖组 1 的 1.1 ～ 1.3。本组完成即首次跑通"选地域 + 机型族 → 拿到可用候选"的端到端主路径。

- [x] 2.1 把 `calculateMonths`（`cmd/woa-server/logics/task/scheduler/scheduler.go:3475`）提到 `cmd/woa-server/logics/rolling-server`，导出为 `CalcRemainMonths(from, expire time.Time) int`；**把调用处的"不足一月补一月"兜底（`scheduler.go:3269-3272`）一并搬进函数体**，统一用 `from` 参数替代两次 `time.Now()`。函数 MUST NOT 对结果做零值兜底，到期时间缺失/已过期时返回负数，与提取前逐值一致。
  > 交付修正：原条目要求放中立包 `pkg/tools/times`。改为放 `logics/rolling-server` 并由 `scheduler` 沿已有 import 方向正向调用（见 design D-P01）：包循环依赖问题同样解决，且"剩余套餐月数含不足一月补一月"是滚服业务口径而非通用时间计算，放通用包会让它看起来可被任意模块复用。
- [x] 2.2 改造 `scheduler.go:3267-3272` 调用新函数，删除原私有函数与调用处兜底；为新函数补单测覆盖"整月""不足一月补一月（日期差与时刻差两种触发）""按量计费无到期时间""已到期"四类输入，并把负数返回值显式钉在断言里。
- [ ] 2.3 回归验证 2.1 的提取属纯重构：用改动前后同一批固资号分别调现网 `check/apply/order/host`（双视角），逐条比对 `charge_months` 与 `new_billing_expire_time` 数值相同。
- [x] 2.4 在 `cmd/woa-server/logics/rolling-server/inherited_host.go` 实现 1.3 声明的候选查询：入口对 `slice.Unique(deviceFamilies)` 用 `errgroup` 并发逐族查、并发度取 `constant.RsInheritedHostQueryConcurrency`、写结果 map 加锁、`Wait()` 返回首个 error 即整体失败；单族逻辑下沉为私有函数。族 → 通用机型名单走 `configLogics.Device().ListDistinctDeviceType`，把 `vendor = enumor.TCloudZiyan`、`device_family = 入参`、`device_type_class = cvmapi.CommonType` 三个条件下推到 DB 并翻页取全量；机型族在表中不存在（名单为空）时返回长度 0 的切片并记 Warn 日志，不报错。
  > 交付修正：原条目写用 `ListCvmInstanceInfoByDeviceTypes` 展开机型族。该方法是"机型 → 族"方向、按机型批量取回，展开需要的是反方向，故改用 `ListDistinctDeviceType` 并把三个条件下推 DB（专用机型不在内存里筛）。`by_static` 的机型 → 族反查仍用 `ListCvmInstanceInfoByDeviceTypes`，见 4.5。
- [x] 2.5 组装 CMDB `ListBizHost` 查询：`HostPropertyFilter` 过滤 `dept_name` = `constant.IEGDeptName`、`bk_cloud_region` = 所选地域、`bk_svr_device_cls_name IN (通用机型名单)`、`instance_charge_type != ""`，业务维度由 `ListBizHostParams.BizID` 承载；`BasePage.Sort = billing_start_time` 升序、`Limit = constant.RsInheritedHostReturnLimit`（排序与条数一并下推 CMDB，不做本地翻页与二次截断）；Fields 取九个字段（`bk_asset_id`、`bk_host_innerip`、`bk_cloud_inst_id`、`bk_svr_device_cls_name`、`instance_charge_type`、`billing_start_time`、`billing_expire_time`、`bk_cloud_region`、`dept_name`）。固定用 `ListBizHost` 而非 `ListHost`。
  > 交付口径：不做"超量拉取 20 条 + 本地剔除已到期/剩余月数 < 1 + 截断到 5"（见 design D-P08）。可用性由现网 `check/apply/order/host` 判定，推荐接口不另造一套口径；`charge_months` 因此允许为 0 或负数。
- [x] 2.6 逐条用 2.1 的共享函数计算 `charge_months` 组装候选；CMDB 或 `device_type` 表查询失败时直接透出 error，不返回部分结果；CMDB 返回空响应时返回长度 0 的切片。`rolling-server` logics 已持有 `cmdbClient` 与 `configLogics`（`logics/rolling-server/service.go:105-113`），**不新增依赖注入**。
- [x] 2.7 在 `cmd/woa-server/service/rolling-server/inherited_host.go` 实现业务视角 Handler：从 URL 路径取 `bk_biz_id` 并校验 > 0（失败 → `errf.InvalidParameter`）→ 校验 `meta.Biz` / `meta.Access` 业务访问权限（对齐 `scheduler.go:1822` 的 `CheckBizInheritedHost`）→ 解码（失败 → `errf.DecodeRequestFailed`）→ 用路径值**回填请求体**（以路径值为准）→ `Validate()`（失败 → `errf.InvalidParameter`）。鉴权提到解码之前：它只依赖路径参数，越权请求不必先解出请求体。
- [x] 2.8 实现资源视角 Handler：`bk_biz_id` 从 body 取；解码 → `Validate()` → 按请求体的 `bk_biz_id` 校验 `meta.ZiYanResource` / `meta.Create`（与现网 `CreateApplyOrder` 同一权限点）。
  > 交付修正：原条目要求"鉴权对齐现网资源视角 `CheckInheritedHost`（当前未做鉴权）、不要顺手收紧"。实际做了鉴权——该接口的 `bk_biz_id` 由调用方指定，无鉴权会被用来按业务 ID 批量枚举固资号与内网 IP，属安全需求「业务敏感信息保护」明确禁止的形态，spec 已同步改为 MUST NOT 无鉴权。
- [x] 2.9 双视角 Handler 共用组装逻辑：把 `device_families` 整个切片交给 2.4 的 logic（并发由 logic 内部收敛，Handler 不写循环）；分组按入参原顺序产出、入参重复的族在响应中重复出现；`billing_start_time` 距当前时间满 `constant.RsInheritedHostRecommendMonths` 个月的候选打 `is_recommended = true`（同族可有多条也可一条都没有），`billing_start_time` 为零值时打 false；无候选的族仍出现在 `info` 中且 `hosts` 为 `[]`（不省略、不为 `null`）；任一族失败整个请求返回 error。
  > 交付修正：原条目写"串行逐族调用""每族第一条打 `is_recommended = true`"。串行改并发见 design D-P07；推荐标记改为按"计费满 36 个月"判定——"第一条恒为推荐项"会把一台刚买两个月的机器也标成推荐，而滚服的推荐语义是"这台用得够久了，该换了"。
- [x] 2.10 路由注册：资源视角挂 `initService`（`cmd/woa-server/service/rolling-server/service.go:44` 的 `/rolling_servers` 前缀），业务视角挂 `bizService`（`:51` 的 `/bizs/{bk_biz_id}/rolling_servers` 前缀），路径均为 `POST .../inherited_hosts/list`。
- [ ] 2.11 功能验证（对应 AC-001 ～ AC-009、AC-012 ～ AC-015、AC-S01/S02）：正例分组返回、筛选五条件反例、跨业务隔离、排序取前 5、计费满 36 个月的候选全部带推荐标记、无满 36 个月的候选时该族无任何标记、零候选无标记、空分组保留为 `[]`、参数缺失（含 `device_families` 含空白串）报 2000001、双视角各自无权限时鉴权失败、CMDB 失败整体报错、双视角内容一致、`path` 与 `body` 的 `bk_biz_id` 冲突时以路径为准、已到期固资仍返回且 `charge_months` 为非正数。
- [ ] 2.12 `charge_months` 一致性验证（AC-010 / AC-011）：对一台 `billing_expire_time = 当前时间 + 8 个月零 3 天` 的固资，推荐接口返回 `charge_months == 9`；对一台按量计费无 `billing_expire_time` 的固资，接口 200 不报错；两种情况的数值均与用同一固资号调现网 `check/apply/order/host` 的返回值相同。
- [ ] 2.13 性能与调用次数验证（AC-016 / AC-P01 / AC-L02）：传 3 个不同机型族时抓 woa-server 日志确认该 rid 下 `ListBizHost` 调用条数为 3；传 `["标准型","标准型","GPU型"]` 时调用条数为 2 且响应 `info` 仍为 3 项；单请求携带 5 个机型族用 `curl -w '%{time_total}'` 连续调 20 次，第 19 分位值 < 3s；传 30 个机型族时确认在途 `ListBizHost` 不超过 `constant.RsInheritedHostQueryConcurrency`，并观察 CMDB 侧有无限流。
- [ ] 2.14 双口径风险验证（AC-017 / AC-018 / G-3）：对一次推荐返回的全部候选，用其 `bk_asset_id` + 同一 `region` + `require_type=6` 逐条调现网 `check/apply/order/host`，失败条数必须为 0；若出现"继承主机的地域与当前所选不匹配"，记录发生次数并反馈（CMDB `bk_cloud_region` 与 CRP `CloudRegion` 口径差异的显式暴露路径），**不得静默忽略**。

## 3. 组 3（Wave 2 · 子单 1069995598137048187）滚服拆单适配与 MCP 双通道暴露

> 依赖组 1 的 1.4 ～ 1.5。与组 2 相互独立，可并行。MCP 部分只依赖现网已存在的 `check_biz_apply_order_host` 接口，与本次任何代码改动无关。

- [x] 3.1 去掉 `pkg/api/woa-server/cvm_apply_recommend.go:168` 处 `ApplyRecommendSplitSubOrderReq.Validate()` 的滚服拒绝分支。**只删这一处**——`:83`（`by_static`，归组 4）与 `:119`（`by_plan`）不在本组范围。（`by_plan` 的拒绝分支后续按「对滚服返回空方案」需求单独移除，改由 handler 短路返回空 `items`。）
- [x] 3.2 在 `ApplyRecommendSplitSubOrderReq.Validate()` 新增滚服专属校验：`require_type == enumor.RequireTypeRollServer` 时 `inherit_instance_id` 不得为空、`charge_type` 必须通过 `Validate()`、`charge_type == PREPAID` 时 `charge_months` 必须 ≥ 1；任一不通过均返回参数校验失败并明确指出是哪个字段，**不静默降级为非滚服**。
  > 交付口径：原条目只有 `inherit_instance_id` 一条。另两条是"参数校验层拦住必然失败的提单"——滚服计费模式取自入参而非推导，留空会让子单落到空计费模式；包年包月子单没有购买时长无法提单。按量计费无套餐时长，不校验 `charge_months`（见 design D-P10）。
- [x] 3.3 在 `cmd/woa-server/service/task/recommend.go` 的子单组装环节为滚服加计费短路分支：`assembleSplitSuborders` 开头判滚服，把预测内 + 预测外的分配结果合并为一个总台数、以入参 `charge_type` 组装唯一子单（总台数 ≤ 0 时返回空列表），不走"预测内 `PREPAID` / 预测外 `POSTPAID_BY_HOUR`"推导；`buildSplitSuborder` 之后把五个滚服字段原样填入产出的子单，非滚服不写入任何滚服字段。**拆单算法本身不改**（`NeedVerifyResPlan()` / `computePlanAvailable` / `allocateSplit` / 库存扣减模型一律不动）。
- [x] 3.4 确认滚服继续受库存约束：`NotNeedVerifyCapacity()` 对滚服仍返回 false（仅绿通返回 true），库存按常规项目口径查（`querySplitCapacityLimit` 固定用 `enumor.RequireTypeRegular`），不改。
- [ ] 3.5 拆单功能验证（AC-030 ～ AC-033）：携带完整滚服字段且库存充足时返回恰好 1 个子单、`charge_type` 与 `charge_months` 等于入参透传值、`replicas` 等于入参；预测余量为 0 时仍返回 1 个子单且 `charge_type` 不被推导成 `POSTPAID_BY_HOUR`；缺 `inherit_instance_id` / 缺 `charge_type` / `PREPAID` 且 `charge_months=0` 三种入参各自参数校验失败且报错指名字段；`POSTPAID_BY_HOUR` 且 `charge_months=0` 校验通过；库存小于 `replicas` 时子单数量被库存封顶。
- [ ] 3.6 拆单无回归验证（AC-M01 / AC-M02）：六种非滚服 `require_type`（常规 1 / 春保 2 / 裁撤 3 / 短租 9 / 绿通 7 / 春保资源池 8）的子单不含五个滚服字段，子单数量与各字段值与改动前逐字段相同；`require_type=6` 调 `by_plan` 返回 HTTP 200 与空 `items`（不报错、不产出方案）。
- [x] 3.7 在 `docs/support-file/helm/files/bk_apigw_resources_bk-hcm_internal_mcp.yaml` 与对外网关资源文件 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml` 各新增 `check_biz_apply_order_host` 的 openapi 定义，照 `create_biz_apply` 与 `get_biz_apply_recommend_by_static` 的形态编写 path / operationId / 请求体 schema / `x-bk-apigateway-resource` 段；入参 schema 含 `bk_asset_id`、`region`、`require_type`、`bk_biz_id`，`region` 标记为 required。**不改现网接口本身**，`region` 保持必填、后端不做地域反查兜底。
- [x] 3.8 在同一批文件为 `create_biz_apply` 补齐缺失的 `bk_asset_id` 字段声明，并同步更新 `docs/api-docs/api-server/docs/zh/` 下 `get_biz_apply_recommend_by_static.md` / `get_biz_apply_recommend_by_plan.md` / `get_biz_apply_recommend_split_suborder.md` 三份网关文档的滚服字段与口径。
- [x] 3.9 先列出 openapi spec 中**全部** `check_` 前缀的 operationId，据此确定一条暴露面可控的 glob pattern（`check_biz_apply_*` 而非宽泛的 `check_*`，见 design D-P06），确认匹配集合仅含预期暴露的接口。
- [x] 3.10 把 3.9 确定的 pattern 加进 `cmd/api-server/etc/api_server.yaml` 的 `mcp.internal.servers[].includeOperationIDs`（:109-121，真实生效的白名单）。
- [x] 3.11 同步把同一条 pattern 加进 `docs/support-file/helm/values.yaml` 的 `mcp.internal.servers[]` 配置示例（:112-130；该处默认是 `servers: []` 加注释示例，改的是示例内容）。两处 pattern 文本必须一致——**只改一处会导致容器化部署白名单不生效、通道半可用**。
- [x] 3.12 在两份 APIGW 资源文件新增业务视角推荐接口 `list_biz_rolling_server_inherited_hosts` 的 openapi 定义（path `/api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list`），入参 schema 含 `bk_biz_id` / `region` / `device_families`；资源视角路径**不加**。该 operationId 命中白名单既有的 `list_biz_*` pattern，不需新增 pattern。
  > 交付修正：原方案 D11 与 spec 均写"推荐接口不进 MCP"。改为业务视角进、资源视角不进（见 design D-P09）：服务端内部补全只会闷头取首条，AI 场景 1 下用户看不到自己继承的是哪台也没得选。注意这条暴露是靠命名命中既有 pattern 自动生效的，评审时须显式确认而非默认接受。
- [ ] 3.13 MCP 双通道验证（AC-040 ～ AC-044、AC-M03 / AC-M04）：APIGW 通道部署后 `tools/list` 含 `check_biz_apply_order_host` 且入参 schema 与 required 标记正确；重启 api-server 后内置 MCP 通道 `tools/list` 同样含该工具；任一通道下 `create_biz_apply` 入参 schema 含 `bk_asset_id`；两条通道的工具列表均含 `list_biz_rolling_server_inherited_hosts` 且**不含**资源视角 `/api/v1/woa/rolling_servers/inherited_hosts/list` 对应的工具；变更文件列表中两个 yaml 同时出现且 pattern 一致；只传 `bk_asset_id` 与 `bk_biz_id` 不传 `region` 调用时参数校验失败；逐个核对本次新增 operationId 命中的既有 pattern 均为有意暴露。

## 4. 组 4（Wave 3 · 子单 1069995598137048845）静态推荐放开滚服与额度预检

> 依赖组 1 的 1.3 ～ 1.5（契约）与组 2 的 2.4 ～ 2.6（继承固资候选查询 logic 的**实现**）。照契约可先编译，但功能验证必须等组 2 落地。

- [x] 4.1 `cmd/woa-server/service/task/service.go:65` 的 `service` 结构体新增 `rsLogics` 字段（`logics/rolling-server` 的 `Logics`），注入方式照现有 `gcLogics`。机型族反查用已有的 `s.configLogics`、额度预检用已有的 `s.logics.Scheduler()`，两者**不需要**新增依赖（见 design D-P05）。
- [x] 4.2 去掉 `pkg/api/woa-server/cvm_apply_recommend.go:83` 处 `ApplyRecommendByStaticReq.Validate()` 的滚服拒绝分支。**只删这一处**——`:119`（`by_plan`）与 `:168`（拆单，归组 3）不在本组范围。
- [x] 4.3 去掉 `buildStaticFilterRules`（`cmd/woa-server/service/task/recommend.go:283`）的 `tools.RuleNotEqual("require_type", enumor.RequireTypeRollServer)` DB 过滤，并同步删掉其上方"滚服项目暂不支持"的注释。
- [x] 4.4 新增 `types.QuotaInsufficientError{Reason string}`（`cmd/woa-server/types/task/scheduler.go`），把 `checkRollingApplyQuota` 与 `checkGreenChannelApplyQuota` 在 `!canApply` 时的 `fmt.Errorf("%s", reason)` 改为返回该类型，并把这两处"额度不足"的日志从 `logs.Errorf` 降为 `logs.Warnf`；在 `CheckApplyQuota`（`scheduler.go:1065`）与接口声明处注明"额度不足返回该类型、额度服务故障返回普通 error"（见 design D-P03）。提单链路与审批建单前置校验的现有调用点（`service/task/scheduler.go:653`、`scheduler.go:941`）**一行都不改**——它们只判 `err != nil`。
  > 交付修正：原条目要求新增 `PreCheckApplyQuota` 三元组出口并让 `CheckApplyQuota` 委托它。改为类型化错误：接口不扩方法、四步逻辑仍只有一份，推荐链路用 `errors.As` 区分即可。核对时确认没有任何调用点在比对 error 字符串。
- [x] 4.5 在 `GetBizApplyRecommendByStatic` 的 `filterCandidatesByCapacity` 之后、`assembleStaticPlans` 之前插入滚服候选补全 `enrichRollServerCandidates`：先扫一遍候选，没有滚服候选就原样返回、连机型族反查都不发起；有则把滚服候选的 `device_type` 去重后**一次批量**调 `s.configLogics.Device().ListCvmInstanceInfoByDeviceTypes` 反查机型族（出参字段 `DeviceGroup`）→ 按 `(region, 机型族)` 在单次请求内缓存、调 `s.rsLogics.ListInheritedHosts` 传单元素切片取**首条**。保留候选数攒够入参 `limit` 时提前终止（见 design D-P11）。只有滚服候选走这条流程，非滚服候选原样保留且不额外发起任何 CMDB 查询。
- [x] 4.6 用取到的固资填充候选的 `charge_type`（取自固资的 `instance_charge_type`，**不再写死** `cvmapi.ChargeTypePrePaid`）、`charge_months`、`bk_asset_id`、`inherit_instance_id`、`billing_start_time`、`billing_expire_time`；`charge_months` 仅在 **> 0** 时写入、两个时间仅在非零值时写入（`int` → `uint` 的负数转换会溢出，按量计费固资本就没有套餐起止时间）；反查不到机型族或该族无固资的候选**丢弃**并记 Warn 日志；机型信息查询与候选查询 logic 调用失败时透出 error，不静默当作"无候选"。
- [x] 4.7 对补全后的滚服候选逐个做额度预检：按候选构造 `[]*types.Suborder`（`Spec.DeviceType` + `Replicas = applyNum`，核数由额度出口内部按 `CPUAmount × Replicas` 计算，与落库 `applied_core` 同口径），调 `s.logics.Scheduler().CheckApplyQuota`；**按单候选独立校验，不做跨候选累加**。用 `errors.As` 命中 `*types.QuotaInsufficientError` 时静默丢弃并把 `Reason` 记入服务端日志（`logs.Warnf`，含 rid）；其他 error 透出，不静默放行。
- [x] 4.8 适配 `assembleStaticPlans`（`recommend.go:388`）接收补全后的固资信息以填入 suborder 的五个滚服字段：新增参数为 `map[*staticRecommendCandidate]*woaserver.InheritedHost`，填充抽成 `fillStaticRollServerFields`，非滚服候选在 map 中取不到值（`host == nil`）时直接返回、不写入任何滚服字段，组装结果与改动前**逐字段一致**；为该函数补单测覆盖 nil host、按量计费零值、包年包月正常值、负数月数四类输入。
- [x] 4.9 确认 DEV-001 已落实：响应结构一个字段都不加——**不新增** `filtered_reasons` / `message`，`reason` 只落日志（技术方案 §4.3(4) 的"`reason` 带进提示语"**不实现**，依据 D-002 静默丢弃）。
- [ ] 4.10 功能验证（AC-020 ～ AC-023、AC-027 / AC-028、AC-R01 / AC-R02）：有历史记录 + 有固资 + 额度充足时 items 含滚服方案且五字段非空、`charge_type` 等于固资的 `instance_charge_type`；无固资的候选被丢弃；机型反查不到机型族的候选被丢弃且不为其发起固资查询；额度不足的候选被丢弃且响应无 `filtered_reasons` / `message`、日志中可查到 `reason`；固资为 `POSTPAID_BY_HOUR` 时方案 `charge_type` 不写死 `PREPAID` 且 `charge_months` / `billing_expire_time` 不出现在响应里；无历史滚服记录时返回空 items 且不触发任何 CMDB 固资查询；额度服务返回 error 时整个请求报错；两个候选各需 X 核而额度剩 1.5X 核时两个都保留；同族多机型的候选只触发 1 次固资查询；`limit=2` 而可用滚服候选有 5 个时只对前 2 个查固资与额度。
- [ ] 4.11 无回归验证（AC-024 ～ AC-026）：`require_type=6` 调 `by_plan` 返回 HTTP 200 与空 `items`；`require_type=1` 调 `by_plan` 正常返回；六种非滚服 `require_type` 调 `by_static` 的 suborder 不含五个滚服字段、其余字段与改动前逐字段相同。
- [ ] 4.12 端到端贯通验证（AC-034 ～ AC-036）：用 `by_static` 推荐出的滚服方案完整走一次 `split_suborder` → `create_biz_apply`，单据创建成功且落库的 `inherit_instance_id` / `bk_asset_id` / `charge_type` / `charge_months` 与推荐结果一致；手工构造机型族不一致的提单请求时由现网 `checkRollingServer`（`scheduler.go:1449`）拦截（**本变更不新增**这条校验）；`charge_type` 与固资 `instance_charge_type` 不同的提单请求不被拦截（D8：计费模式是继承结果而非校验项）。
- [ ] 4.13 性能验证（AC-P02）：`require_type=6`、`limit=10` 的 `by_static` 请求相对放开滚服前的基线，P95 耗时增量 < 2s（同一测试环境同一业务，各测 20 次对比）；非滚服请求耗时与改动前一致。
