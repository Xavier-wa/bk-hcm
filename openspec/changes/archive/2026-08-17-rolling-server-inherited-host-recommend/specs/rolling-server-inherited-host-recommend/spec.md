## ADDED Requirements

### Requirement: 继承固资推荐接口（业务视角与资源视角）

系统 SHALL 在 woa-server 提供两条继承固资推荐接口，路径归属现有滚服前缀：业务视角 `POST /api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list`（挂 `cmd/woa-server/service/rolling-server/service.go` 的 `bizService`），资源视角 `POST /api/v1/woa/rolling_servers/inherited_hosts/list`（挂同文件的 `initService`）。两者共用同一请求/响应结构。

请求 SHALL 含三个必填参数：`bk_biz_id`（`gt=0`）、`region`（非空，`max=128`，云地域标识如 `ap-guangzhou`）、`device_families`（`min=1,max=100`，机型族中文名如 标准型 / 高IO型 / 大数据型 / 计算型 / GPU型，取值来源为 `device_type` 表 `device_family` 字段，不是枚举 code）。业务视角的 `bk_biz_id` SHALL 以 URL 路径值为准并回填请求体；资源视角 SHALL 从 body 取。`device_families` 中的空白字符串 SHALL 被拒绝，错误信息指明是第几个元素。

响应 SHALL 为按机型族分组的 `info` 列表，分组顺序与入参 `device_families` 一致；每个分组含 `device_family` 与 `hosts`；每条候选含 `bk_asset_id`、`bk_host_innerip`、`bk_cloud_inst_id`、`device_type`、`instance_charge_type`、`billing_start_time`、`billing_expire_time`、`charge_months`、`is_recommended`。

请求结构 MUST NOT 包含任何搜索类字段（`keyword` / `bk_asset_id` 等）；响应结构 MUST NOT 包含分页游标或候选总数字段（`total` / `count`）。

#### Scenario: 业务视角正常返回分组候选

- **GIVEN** 业务 100148 在 `ap-guangzhou` 下有 3 台满足全部筛选条件的标准型通用机型固资
- **WHEN** 调用业务视角推荐接口传 `device_families=["标准型"]`
- **THEN** 响应 `info` 长度为 1，`info[0].device_family == "标准型"`，`hosts` 长度为 3，每条的 `bk_asset_id`、`bk_host_innerip`、`bk_cloud_inst_id`、`device_type`、`instance_charge_type`、`billing_start_time`、`charge_months` 均非空

#### Scenario: 双视角返回内容一致

- **GIVEN** 相同的 `bk_biz_id` / `region` / `device_families`
- **WHEN** 分别调用业务视角与资源视角接口
- **THEN** 两者返回的 `info` 内容完全一致

#### Scenario: 业务视角以路径 bk_biz_id 为准

- **GIVEN** 一个 `path=100148`、`body.bk_biz_id=200000` 的业务视角请求
- **WHEN** 调用推荐接口
- **THEN** 查询以路径值 100148 为准，返回的固资全部归属业务 100148

#### Scenario: 必填参数缺失

- **GIVEN** 请求缺少 `region`，或 `device_families` 为空数组，或 `bk_biz_id ≤ 0`
- **WHEN** 调用推荐接口
- **THEN** 返回 `errf.InvalidParameter`（错误码 2000001），错误信息指明具体缺失的字段名

#### Scenario: 不提供分页、搜索与总数

- **GIVEN** 某机型族下有 8 台满足条件的固资
- **WHEN** 调用推荐接口
- **THEN** `hosts` 长度不超过 5，响应中不含分页游标与候选总数字段，第 6 条及之后的数据不可获取
- **AND** 传入 `keyword` / `bk_asset_id` 等未定义字段时被忽略，既不报错也不生效

### Requirement: 继承固资候选的筛选口径

系统 SHALL 通过 `device_type` 表把机型族展开成该族下的**通用机型**名单，再对每个机型族调用一次 CMDB `ListBizHost`。展开 SHALL 走 `configLogics.Device().ListDistinctDeviceType`，把 `vendor == tcloud_ziyan`、`device_family == 入参机型族`、`device_type_class == CommonType` 三个条件下推到 DB（专用机型 `SpecialType` 由 DB 过滤直接排除，不在内存里筛），并翻页取回该族**全部**通用机型而非首页。

候选 MUST 同时满足五个条件：归属当前业务（`bk_biz_id`）、`dept_name == constant.IEGDeptName`（互动娱乐事业部）、`bk_cloud_region == 所选地域`、`bk_svr_device_cls_name` 属于该族通用机型名单、`instance_charge_type != ""`。

CMDB 查询 MUST 固定使用业务维度接口 `ListBizHost` 而非 `ListHost`，从数据源头保证跨业务隔离。地域判定 SHALL 使用 CMDB 的 `bk_cloud_region`；现网单台校验链路继续使用 CRP 的 `CloudRegion`，两者口径不一致时以校验接口为准（页面选中候选后仍会走一次校验）。

传入的机型族在 `device_type` 表中不存在时，系统 SHALL 视同"该族无通用机型"返回空候选并记 Warn 日志，SHALL NOT 报错。

#### Scenario: 不满足筛选条件的固资被排除

- **GIVEN** 某台固资的 `dept_name` 不是互动娱乐事业部，或 `bk_cloud_region` 不等于请求的地域，或机型的 `DeviceTypeClass` 是 `SpecialType`，或 `instance_charge_type` 为空
- **WHEN** 调用推荐接口
- **THEN** 该台固资不出现在 `hosts` 中

#### Scenario: 跨业务固资被排除

- **GIVEN** 固资 TC260320003234 归属业务 200000
- **WHEN** 用 `bk_biz_id=100148` 调用推荐接口
- **THEN** 响应中不含 TC260320003234

#### Scenario: 机型族在 device_type 表中不存在

- **GIVEN** 请求传入一个 `device_type` 表中不存在的机型族名
- **WHEN** 调用推荐接口
- **THEN** 该族在响应中的 `hosts` 为空数组，接口不报错

### Requirement: 候选排序、条数上限与推荐标记

系统 SHALL 把排序与条数上限一并下推给 CMDB：`BasePage.Sort = billing_start_time` 升序（等价于已使用时长降序）、`BasePage.Limit = constant.RsInheritedHostReturnLimit`（5），SHALL NOT 在本地做排序、翻页或二次截断。单次请求对每个机型族 MUST 只调用一次 CMDB。

每个机型族对外最多返回 5 条候选。候选 SHALL 由 CMDB 的五个筛选条件收敛，SHALL NOT 在 CMDB 返回后再按"已到期""剩余月数 < 1"之类的条件二次剔除——剔除会让某机型族的返回条数少于 5 条却无从回填，且已到期固资在提单环节由现网 `check/apply/order/host` 兜住。`charge_months` 因此 MAY 为 0 或负数（已到期的包年包月固资、无到期时间的按量计费固资），该数值与现网校验接口同源，判断可用性的责任在调用方与校验接口。

候选的 `billing_start_time` 距当前时间已满 `constant.RsInheritedHostRecommendMonths`（36）个月时 SHALL 标记 `is_recommended = true`，否则为 false；同一机型族内满足条件的候选 MAY 有多条，也 MAY 一条都没有。`billing_start_time` 缺失（零值）时无法判断已计费月数，SHALL 标记为 false。`is_recommended` SHALL 由接口层（Handler）在组装分组时打标，候选查询 logic 的出参 MUST NOT 包含该字段——该标记是展示语义，AI 链路复用同一 logic 时不应拿到这个无意义的标记。

#### Scenario: 按 billing_start_time 升序取前 5 条

- **GIVEN** 某机型族下有 8 台满足条件的固资，计费开始时间分别为 2023-01 至 2025-08
- **WHEN** 调用推荐接口
- **THEN** `hosts` 长度为 5，5 条的 `billing_start_time` 严格升序，第 1 条是全部 8 台中 `billing_start_time` 最早的那台
- **AND** 该机型族对 CMDB 的调用次数为 1

#### Scenario: 已到期固资不在服务端被剔除

- **GIVEN** 某机型族下 `billing_start_time` 最早的固资是一台已到期的包年包月固资
- **WHEN** 调用推荐接口
- **THEN** 该固资仍出现在 `hosts` 首位，其 `charge_months` 为 0 或负数，接口不报错

#### Scenario: 计费满 36 个月的候选均被标记为推荐项

- **GIVEN** 某机型族返回 5 条候选，其中 3 条的 `billing_start_time` 距当前时间已满 36 个月
- **WHEN** 检查响应
- **THEN** 这 3 条的 `is_recommended == true`，另外 2 条为 false

#### Scenario: 无候选计费满 36 个月

- **GIVEN** 某机型族返回的候选 `billing_start_time` 均距当前时间不足 36 个月
- **WHEN** 检查响应
- **THEN** 该族的 `hosts` 中不存在任何 `is_recommended == true` 的元素

#### Scenario: 零候选时无推荐标记

- **GIVEN** 某机型族返回 0 条候选
- **WHEN** 检查响应
- **THEN** 该族的 `hosts` 中不存在任何 `is_recommended == true` 的元素

#### Scenario: logic 出参不含 is_recommended

- **GIVEN** 候选查询 logic 的出参元素类型定义
- **WHEN** 检查其字段
- **THEN** 不存在 `is_recommended` 字段；接口响应的候选元素类型 SHALL 在该类型之上组合出 `is_recommended`

### Requirement: 空分组语义与下游失败的整体失败

传入的机型族若查不到任何可继承机器，该族 MUST 仍出现在响应 `info` 中且 `hosts` 为长度 0 的数组（序列化为 `[]`），MUST NOT 被从响应中省略、MUST NOT 序列化为 `null`——前端要靠这个区分"没查到"和"没传这个族"。

分组 SHALL 按入参 `device_families` 的原始顺序逐个产出；入参中重复的机型族 SHALL 在响应中重复出现（内容相同），logic 层的去重只用于收敛 CMDB 调用次数，MUST NOT 改变响应分组的个数与顺序。

任一机型族的 CMDB 查询失败时，系统 MUST 让整个请求返回 error，MUST NOT 返回只含部分机型族的 `info`——部分结果会让前端把"查询失败"误判成"该族无候选"从而隐藏 Tab。`device_type` 表查询失败同样 SHALL 透出 error。

推荐结果 SHALL NOT 落本地表、SHALL NOT 加缓存，每次实时查询 CMDB。

#### Scenario: 空分组保留在响应中

- **GIVEN** 请求传 `device_families=["标准型","GPU型"]` 且 GPU 型下无任何可继承机器
- **WHEN** 调用推荐接口
- **THEN** `info` 长度为 2，其中存在 `device_family == "GPU型"` 且 `hosts == []` 的元素
- **AND** 该分组不被省略，`hosts` 不为 `null`

#### Scenario: CMDB 失败整体报错

- **GIVEN** CMDB `ListBizHost` 调用返回错误
- **WHEN** 调用推荐接口
- **THEN** 整个请求返回 error，不返回只含部分机型族的 `info`

### Requirement: charge_months 与现网校验接口同源同算法

`charge_months` 的语义 SHALL 为**剩余月数**（当前时间到套餐到期时间），与现网 `CheckInheritedHostResp.ChargeMonths` 同源同算法。计算实现 MUST 在推荐链路与现网校验链路之间**共享同一份代码**，SHALL NOT 各写一份；"当前时间 + 月数仍早于套餐到期时间则再加一月"的兜底 MUST 与主体算法一起收进函数体（该兜底原位于校验链路的调用处），两处调用共用同一个 `(from, expire)` 入口。

共享函数 SHALL 落在 `cmd/woa-server/logics/rolling-server` 包内并导出，由 `scheduler` 正向调用。`scheduler` 已单向 import 该包，反向让 rolling-server 依赖 `scheduler` 会成环，因此 MUST NOT 采用"导出 `scheduler` 私有函数"的做法。

共享函数 SHALL NOT 对结果做零值兜底：到期时间缺失或已过期时返回 0 或负数，与提取前的现网行为逐值一致。

共享化 MUST NOT 改变现网 `check/apply/order/host` 返回的 `charge_months` 与 `new_billing_expire_time` 数值——该提取属纯重构。

#### Scenario: 剩余月数与校验接口数值相同

- **GIVEN** 一台固资 `billing_expire_time = 当前时间 + 8 个月零 3 天`
- **WHEN** 调用推荐接口
- **THEN** 该条的 `charge_months == 9`，且与用同一固资号调用现网 `check/apply/order/host` 返回的 `charge_months` 数值相同

#### Scenario: 按量计费固资无到期时间

- **GIVEN** 一台按量计费（`POSTPAID_BY_HOUR`）固资无 `billing_expire_time`
- **WHEN** 调用推荐接口
- **THEN** 接口返回 HTTP 200 不报错，且该条的 `charge_months` 与用同一固资号调用现网 `check/apply/order/host` 返回的 `charge_months` 数值相同

#### Scenario: 现网校验接口无回归

- **GIVEN** 计算实现已提到共享位置
- **WHEN** 用改动前后同一批固资号分别调现网 `check/apply/order/host`
- **THEN** 两次返回的 `charge_months` 与 `new_billing_expire_time` 逐条相同

### Requirement: 推荐接口的鉴权与数据保护

业务视角推荐接口 MUST 校验调用方对目标业务的 `meta.Biz` / `meta.Access` 访问权限，与现网 `CheckBizInheritedHost` 一致。

资源视角推荐接口 MUST 按请求体中的 `bk_biz_id` 校验调用方「资源视角下创建主机申请单据」的权限（`meta.ZiYanResource` / `meta.Create`，与现网 `CreateApplyOrder` 同一权限点）。该接口的 `bk_biz_id` 由调用方指定，不按目标业务鉴权会被用来按业务 ID 批量枚举固资号与内网 IP，因此 MUST NOT 保持无鉴权。鉴权失败 MUST 直接返回鉴权 error，MUST NOT 返回任何候选。

响应中的 `bk_asset_id` 与 `bk_host_innerip` 属业务敏感信息，MUST 仅返回给通过上述业务维度鉴权的调用方；查询条件中的 `bk_biz_id` MUST 以服务端确定的业务为准，不允许调用方越权查询其他业务的固资。

#### Scenario: 无业务权限时鉴权失败

- **GIVEN** 调用方对业务 100148 无访问权限
- **WHEN** 调用业务视角推荐接口
- **THEN** 返回鉴权失败，响应体中不含任何 `bk_asset_id` 或 `bk_host_innerip`

#### Scenario: 资源视角无目标业务的建单权限时鉴权失败

- **GIVEN** 调用方对业务 100148 无「资源视角下创建主机申请单据」权限
- **WHEN** 调用资源视角推荐接口传 `bk_biz_id=100148`
- **THEN** 返回鉴权失败，响应体中不含任何 `bk_asset_id` 或 `bk_host_innerip`

### Requirement: 推荐接口的外部调用次数与响应时间口径

推荐接口对 CMDB 的调用次数 MUST 等于**去重后**的机型族数量，SHALL NOT 因候选数量增长而放大。一次查询按时间排序取前 5 条可能全部落在同一个族，所以逐族查询是必需的，调用次数由调用方传入的机型族列表显式控制。

逐族查询 SHALL 并发执行，并发度 MUST 由 `constant.RsInheritedHostQueryConcurrency`（10）封顶，避免机型族数量上限（100）直接打成 100 路并发压垮 CMDB。各族结果写入共享 map 时 MUST 加锁；任一族失败 MUST 让整个请求失败（并发不改变整体失败语义）。

推荐接口响应时间 P95 SHALL < 3s，测量场景为单请求携带 5 个机型族、测试环境正常业务数据量。推荐链路 SHALL NOT 引入 CRP 调用。

#### Scenario: 调用次数等于去重后的机型族数量

- **GIVEN** 请求传 3 个不同机型族
- **WHEN** 调用推荐接口
- **THEN** woa-server 日志中该 rid 下的 `ListBizHost` 调用条数为 3

#### Scenario: 重复机型族只查一次

- **GIVEN** 请求传 `device_families=["标准型","标准型","GPU型"]`
- **WHEN** 调用推荐接口
- **THEN** `ListBizHost` 调用条数为 2，响应 `info` 长度仍为 3，其中两个 `标准型` 分组内容相同

#### Scenario: 机型族查询互不污染

- **GIVEN** logic 的候选查询方法
- **WHEN** 用同一 `region` 传两个不同机型族
- **THEN** 每个族各发起 1 次 CMDB `ListBizHost`，两个族的结果互不污染

#### Scenario: 并发度被封顶

- **GIVEN** 请求传 30 个机型族
- **WHEN** 调用推荐接口
- **THEN** 同一时刻在途的 `ListBizHost` 请求数不超过 `constant.RsInheritedHostQueryConcurrency`

#### Scenario: P95 响应时间

- **GIVEN** 单请求携带 5 个机型族，测试环境正常业务数据量
- **WHEN** 用 `curl -w '%{time_total}'` 连续调用 20 次
- **THEN** 第 19 分位值 < 3s

### Requirement: 推荐接口的双视角接口文档

系统 SHALL 交付两份接口文档作为契约的唯一真相源：业务视角文档落 `docs/api-docs/web-server/docs/biz/`，资源视角文档落 `docs/api-docs/web-server/docs/resource/`，格式参照 resource 目录现有文档，版本标注 v9.9.9+。文档 SHALL 含路径、方法、入参表、出参表、请求示例、响应示例、错误码（`errf.InvalidParameter` 2000001 与鉴权失败），并标注 `bk_asset_id` / `bk_host_innerip` 属业务敏感信息与两个视角的 `bk_biz_id` 取值来源差异。

业务视角接口 SHALL 同时在对外网关资源文件 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml` 注册（operationId 为 `list_biz_rolling_server_inherited_hosts`），使其可经蓝鲸网关调用；资源视角接口 SHALL NOT 注册到网关。

响应示例的数据 MUST 算术自洽：`charge_months` 等于示例编写时点到示例 `billing_expire_time` 的剩余月数。MUST NOT 照抄技术方案 §4.2 中 `billing_start_time=2024-06-10` / `billing_expire_time=2027-06-10` / `charge_months=8` 那组相差 36 个月的不自洽数据。

#### Scenario: 两份文档路径与版本

- **GIVEN** 两份接口文档已提交
- **WHEN** 检查文件路径与内容
- **THEN** 业务视角文档位于 `docs/api-docs/web-server/docs/biz/`、资源视角位于 `docs/api-docs/web-server/docs/resource/`，两份均标注版本 v9.9.9+

#### Scenario: 响应示例算术自洽

- **GIVEN** 接口文档的响应示例
- **WHEN** 用示例中的 `billing_expire_time` 与示例编写时点计算剩余月数
- **THEN** 结果等于示例中的 `charge_months`

#### Scenario: 业务视角接口在网关注册

- **GIVEN** 对外网关资源文件 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml`
- **WHEN** 检索 `inherited_hosts/list` 相关的资源定义
- **THEN** 存在 operationId 为 `list_biz_rolling_server_inherited_hosts` 的业务视角路径定义，且不存在资源视角路径 `/api/v1/woa/rolling_servers/inherited_hosts/list`
