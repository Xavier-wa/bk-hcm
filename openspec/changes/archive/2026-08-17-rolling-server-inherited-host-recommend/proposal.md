## Why

滚服项目的业务本质是"以旧换新、继承套餐"——用一台待退回旧设备的剩余计费周期去申领新机型。现网无论页面还是 AI 链路，继承固资号都要用户自己去别的平台查好再粘贴：购买主机页面只有一个纯文本框加一个"手动校验"按钮（≥ 3 步跨平台操作），AI 提单链路则被三个推荐接口的 `Validate()` 直接拒绝（`require type ... is temporarily not supported`），对滚服的支持度为 0。

两条链路缺的是同一个底层能力：**按「业务 + 地域 + 机型族」筛出可继承的固资，并按已使用时长排序取最久的若干条**。现网只有 `CheckInheritedHost`（`scheduler.go:3239`）按固资号精确查一台（`Page.Limit: 1`），只能校验、不能推荐。不做则前端固资选择器（子需求 1069995598136963986）没有数据来源无法开工，AI 提单链路对滚服继续完全不可用。

## What Changes

- **新增继承固资推荐接口（双视角）**：`POST /api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list` 与 `POST /api/v1/woa/rolling_servers/inherited_hosts/list`，入参 `bk_biz_id` / `region` / `device_families`，出参按机型族分组、每族最多 5 条候选、计费满 36 个月的候选标 `is_recommended`。不提供分页、搜索、候选总数。
- **新增继承固资候选查询 logic**：落在 `cmd/woa-server/logics/rolling-server/`，机型族展开 → CMDB `ListBizHost` 过滤 → `BasePage.Sort` 排序与 `Limit` 一并下推 → 组装候选。签名接**机型族切片**并在内部按 `constant.RsInheritedHostQueryConcurrency` 限流并发逐族查、返回以机型族为 key 的 map；页面链路传全部机型族、AI 链路按候选反查出族后传单元素切片，两侧都不写循环与并发（见 design D-P07，与技术方案"单族串行"的原口径相反）。
- **`calculateMonths` 及其"不足一月补一月"兜底提到共享位置**：该函数现为 `scheduler` 包私有（`scheduler.go:3475`），兜底在调用处（`scheduler.go:3269-3272`）；导出为 `rollingserver.CalcRemainMonths` 放在 `logics/rolling-server`，由 `scheduler` 沿已有 import 方向调用（见 design D-P01）。属纯重构，现网 `check/apply/order/host` 返回值不得变化。
- **`ApplyRecommendSuborder` 补五个滚服字段、`ApplyRecommendSplitSubOrderReq` 补六个**：`charge_months`、`bk_asset_id`、`inherit_instance_id`、`billing_start_time`、`billing_expire_time`，拆单请求体额外补 `charge_type`（滚服计费模式不再由推导得出，必须由入参透传）。全部 `omitempty`，仅滚服（`require_type=6`）填充。
- **`by_static` 放开滚服并在服务端内部补全继承固资**：去掉 `cvm_apply_recommend.go:83` 的拒绝与 `recommend.go:283` 的 `require_type != 6` DB 过滤；在 `filterCandidatesByCapacity` 之后、`assembleStaticPlans` 之前插入候选补全（`device_type` 批量反查机型族 → 调候选查询 logic 取首条 → 填充计费与固资字段），并按 `(region, 机型族)` 缓存、攒够 `limit` 即停；查不到固资的候选丢弃。
- **`by_static` 滚服候选额度预检**：复用 `scheduler` 已暴露的 `CheckApplyQuota`（`scheduler.go:1065`）的四步实现，让它在额度不足时返回类型化错误 `types.QuotaInsufficientError`，推荐链路用 `errors.As` 识别后**静默丢弃**、`reason` 仅记服务端日志；提单与审批链路的调用点不动（见 design D-P03）。
- **`split_suborder` 放开滚服并短路计费字段**：去掉 `cvm_apply_recommend.go:168` 的拒绝，新增三条滚服校验（`inherit_instance_id` 非空、`charge_type` 合法、包年包月时 `charge_months ≥ 1`）；`charge_type` 与 `charge_months` 取自透传的固资信息，不走预测内外推导。拆单算法、库存约束一律不改。
- **MCP 双通道暴露**：两份 APIGW 资源文件（对外 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml` 与内置 MCP `docs/support-file/helm/files/bk_apigw_resources_bk-hcm_internal_mcp.yaml`）新增 `check_biz_apply_order_host` 与业务视角推荐接口 `list_biz_rolling_server_inherited_hosts` 的 openapi 定义、为 `create_biz_apply` 补 `bk_asset_id` 字段声明；`cmd/api-server/etc/api_server.yaml` 与 `docs/support-file/helm/values.yaml` 的 `includeOperationIDs` 成对补 `check_biz_apply_*`。推荐接口的暴露与原口径 D11「推荐接口不进 MCP」相反（见 design D-P09），资源视角仍不进。
- **接口文档**：业务视角落 `docs/api-docs/web-server/docs/biz/`、资源视角落 `docs/api-docs/web-server/docs/resource/`，版本 v9.9.9+；同步更新 `docs/api-docs/api-server/docs/zh/` 下 `by_static` / `by_plan` / `split_suborder` 三份网关文档的滚服口径。
- 非破坏性变更：新增字段全部 `omitempty`，六种非滚服 `require_type`（常规 1 / 春保 2 / 裁撤 3 / 绿通 7 / 春保资源池 8 / 短租 9）的 `by_static` 与 `split_suborder` 响应逐字段不变；`by_plan` 对滚服**不产出方案**（返回 200 + 空 `items`，不再报参数错误）；现网 `check/apply/order/host` 与提单 `checkRollingServer` 一律不改。

## Capabilities

### New Capabilities

- `rolling-server-inherited-host-recommend`: 继承固资推荐能力——双视角推荐接口的契约与行为、候选查询的筛选/排序/条数口径、`charge_months` 剩余月数与校验接口的数值一致性、`is_recommended` 打标规则与打标位置、空分组语义、鉴权与跨业务隔离、外部调用次数与并发口径、两份接口文档与网关注册。
- `rolling-server-ai-mcp-exposure`: 滚服 AI 链路的 MCP 暴露面——两条通道新增 `check_biz_apply_order_host` 与业务视角推荐接口、`create_biz_apply` 补 `bk_asset_id` 声明、api-server 内置 MCP 白名单成对补 `check_biz_apply_*` pattern，以及"资源视角不进 MCP、新增 pattern 与新增 operationId 的命中集合都需显式核对"的暴露面约束。

### Modified Capabilities

- `apply-recommend`: `by_static` 与 `split_suborder` 两个接口对滚服的行为从"直接拒绝"改为支持，且滚服的计费模式与购买时长改为继承自固资而非由预测内外推导；`by_plan` 对滚服从"参数校验报错"改为返回空方案；`ApplyRecommendSuborder` 契约新增五个滚服字段、拆单请求体新增六个。涉及 6 条既有需求的修改（在线静态推荐接口、静态候选查询与 A 类过滤、默认值补全与申请数量配置、拆单试算接口、按计费模式的子单拆分、按需求类型差异化的预测与库存校验）与 4 条新增需求（滚服固资补全、滚服额度预检、滚服字段契约、`by_plan` 对滚服返回空方案）。

## Impact

**代码**

| 层 | 文件 | 改动 |
|---|---|---|
| API 类型 | `pkg/api/woa-server/rolling_server_inherited_host.go`（新增） | 推荐接口请求/响应 + `Validate()`；`InheritedHost`（logic 出参，不含 `is_recommended`）与内嵌它的 `InheritedHostCandidate` |
| API 类型 | `pkg/api/woa-server/cvm_apply_recommend.go` | 子单补五个滚服字段、拆单请求体补六个；去掉 `by_static`(:83)、`split_suborder`(:168) 与 `by_plan`(:119) 的滚服拒绝，`by_plan` 改由 handler 短路返回空 `items`；拆单新增三条滚服校验 |
| Logic | `cmd/woa-server/logics/rolling-server/inherited_host.go`（新增） | 多族并发候选查询 + 单族私有实现 + 导出的 `CalcRemainMonths`（已持有 `cmdbClient` 与 `configLogics`，无需新增注入） |
| Logic | `cmd/woa-server/logics/task/scheduler/scheduler.go` | 删除私有 `calculateMonths` 与调用处兜底，改调 `rollingserver.CalcRemainMonths`；滚服与绿通额度不足改返回 `types.QuotaInsufficientError` 且日志降为 Warn |
| 类型 | `cmd/woa-server/types/task/scheduler.go` | 新增 `QuotaInsufficientError`（承载 `reason`，供 `errors.As` 识别） |
| 常量 | `pkg/criteria/constant/rolling_server.go` | 新增 `RsInheritedHostReturnLimit`(5) / `RsInheritedHostQueryConcurrency`(10) / `RsInheritedHostRecommendMonths`(36) |
| 路由 / Handler | `cmd/woa-server/service/rolling-server/service.go` 与 `inherited_host.go`（新增） | 双视角各注册一条，挂 `/rolling_servers`(:44) 与 `/bizs/{bk_biz_id}/rolling_servers`(:51)；`is_recommended` 在此打标 |
| Handler | `cmd/woa-server/service/task/recommend.go` | 滚服候选补全（含族缓存与 `limit` 提前终止）+ 额度预检 + `assembleStaticPlans` 适配 + 拆单计费短路 + `by_plan` 滚服短路 |
| 依赖注入 | `cmd/woa-server/service/task/service.go:65` | `service` 结构体新增 `rsLogics` 字段（照 `gcLogics`，从 `c.RsLogic` 注入） |

**配置**：`cmd/api-server/etc/api_server.yaml`（`includeOperationIDs` 补 `check_biz_apply_*`）与 `docs/support-file/helm/values.yaml`（同名示例段）成对提交；`docs/support-file/helm/files/bk_apigw_resources_bk-hcm_internal_mcp.yaml` 与 `docs/api-docs/api-server/api/bk_apigw_resources_bk-hcm.yaml` 各新增/补全三处 openapi 定义。

**外部依赖**：CMDB `ListBizHost`（每个去重后的机型族一次，`limit=5`，排序与条数一并下推）、HCM `device_type` 表（族 → 通用机型走 `ListDistinctDeviceType`，机型 → 族走 `ListCvmInstanceInfoByDeviceTypes`）、滚服额度服务（`rsLogics.IsResPoolBiz` / `GetCpuCoreSum` / `CanApplyHost`）。推荐链路**不**引入 CRP 调用。

**数据库**：不新建表、无 DDL、不改 data-service。

**跨需求**：接口契约同时是前端子需求 1069995598136963986 与 AI skill 侧需求 1069995598136239422 的输入，契约冻结后 Wave 2/3 不得重新定义。

**不在本变更范围**：前端页面实现；现网 `check/apply/order/host` 的任何改动；`by_plan` 为滚服产出推荐方案（只做空列表短路）；资源视角推荐接口进 MCP；AI skill 侧对话逻辑；AI 场景 2 的磁盘默认值继承；推荐结果落表/跨请求缓存；并发互斥。
