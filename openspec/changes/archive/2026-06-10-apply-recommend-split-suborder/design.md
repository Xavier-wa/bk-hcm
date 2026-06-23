## Context

接口1（`by_static_recommend`）与接口2（`by_plan`）已落在 `cmd/woa-server/service/task/recommend.go`，复用了预测余量能力（`s.planLogics`）与库存静态表 `device_capacity`。两者均产出「单子单、数量取入参」的推荐方案。接口3 是「确定方案 → 真正下单」之间的拆单/试算层：入参即接口1/2 某方案的确定参数，按预测内/预测外余量把申请数量拆成主单 + 多子单，纯试算不落库。

现有可直接复用的能力：
- `planLogics.GetProdResRemainPoolMatch` / `GetPlanTypeAvlDeviceTypesV2` / `GetAllDeviceTypeMap`（接口2 已注入 `s.planLogics`）。
- 库存查询模式 `queryCapacitySatisfied`（但仅返回布尔，需新增取数值版）。
- `RequireType.NeedVerifyResPlan()` / `NotNeedVerifyCapacity()`。
- `woaserver.ApplyRecommendSuborder`（字段齐全，可同时用作输出子单与 occupied 入参结构）。

## Goals / Non-Goals

**Goals:**
- 新增 `POST /bizs/{bk_biz_id}/task/apply/recommend/split_suborder` 接口，按计费模式把确定方案拆成主单 + ≤2 子单。
- 按需求类型差异化预测/库存校验；裁撤单池只算一次。
- 支持 `occupied_suborders` 增量拆分：族级扣预测余量、机型级扣库存。
- 零 DB / 零离线 / 零落库改动，复用既有 logics 与 data-service。

**Non-Goals:**
- 不做提单/落库（由既有提单工具完成）。
- 不做 MCP 工具化、Graph 双中断、Skill 沉淀（属 Agent 接入子需求）。
- 不改动接口1/2 及 `GetBizApplyRecommendTop`。
- 不重写预测逻辑，不新增 data-service 接口。

## Decisions

### D1: 落点 service/task + 路由 /apply/recommend/split_suborder
与接口1/2 同族，复用已注入的 `s.planLogics` 与库存查询模式，无需改 `InitService`。
- 备选：本地 reqs 文档写的 `service/cvm` + `split_sub_order`，已确认弃用（与 iWiki 及现有代码不一致）。

### D2: 响应扁平结构 `{ suborders: []*ApplyRecommendSuborder }`
试算不落库、主单无 ID，主单为隐含概念，扁平结构最简洁。复用 `ApplyRecommendSuborder` 避免新增类型。
- 备选：嵌套 `{ main_order: { suborders } }`，已确认弃用。

### D3: 库存从布尔判断升级为数值封顶
接口1/2 的 `queryCapacitySatisfied` 只回 `capacity ≥ 申请台数` 布尔，接口3 需要 capacity **数值**对拆分总量封顶。新增 `querySplitCapacityLimit` 库存查询函数（按 `(require_type, region, device_type)` 查 `device_capacity`，返回各 zone 的容量上限 map）。同一 zone 若存在多行，取该 zone 的**最大** capacity 作为上限。
- `zone=all` → 查 region 下所有 zone，取各 zone 有效容量之 **sum**（zone=all 不钉死可用区、可跨 zone 分摊下单，各 zone 先扣本 zone 占用并截断 0 后求和）。
- `zone=具体值` → 仅查该 zone，取该 zone 的 capacity。

### D4: 族级 occupiedCore 用 GetAllDeviceTypeMap 补族信息
`GetPlanTypeAvlDeviceTypesV2` 返回的 `DeviceTypeAvailable{DeviceType, Available, RemainCore}` 不含族字段，而预测余量在同族（`TechnicalClass` + `CoreType` 相同）间共享同一份余量。故 occupied 扣减落到族级：
```
occupiedCore[(region, TechnicalClass, CoreType, 内/外)] += replicas × cpu_core(子单 device_type)
有效余量 = RemainCore − occupiedCore[group(本机型)]  （截断 0）
```
族信息（TechnicalClass / CoreType / CpuCore）从 `GetAllDeviceTypeMap()` 的 `DistinctDeviceType` 取。预测内/外由子单 charge_type 决定（PREPAID→内，POSTPAID→外）；裁撤合并到同一族级 key。

计费↔预测内/外的映射统一收敛到新增的 `cvmapi.ChargeType.ToPlanType()`（按量计费→预测外，其余含包年包月/默认→预测内），`normalizePlanType` 在其之上叠加「裁撤忽略内外、统一归到 `PlanTypeCodeIgnore` 单池」的语义；`cmd/woa-server/logics/plan/types.go` 的 `GetPlanTypeByChargeType` 同步重构为复用 `ToPlanType`，消除散落的 switch。

### D5: 库存占用按 (require_type, region, device_type, zone) 聚合，区分浮动/具体 zone
库存占用按机型级聚合台数并**保留 zone 维度**，区分两类占用：`zone=all` 的占用视为**浮动占用**（任何 zone 都可能落，统一扣减），具体 zone 的占用只扣到对应 zone，避免某 zone 超额占用透支其他 zone。
- req 指定具体 zone Z：有效库存 = `capacity(Z) − 占用(Z) − 浮动占用(all)`（截断 0）。
- req=all：各 zone 先扣本 zone 具体占用并截断 0 后求和，再统一扣减浮动占用(all)（截断 0）。

### D6: 实际执行顺序 = 预测分配 → 库存对总量封顶
文档③④⑤是查询罗列顺序；实现按真实依赖：先算 `nPrepaid/nPostpaid` 分配，再用库存上限对总量按「预测内 → 预测外」封顶（封顶天然是最后一步）。

### D7: 数量分配算法
```
takePrepaid  = min(replicas, nPrepaid)
takePostpaid = min(replicas - takePrepaid, nPostpaid)
// 库存封顶：cap = 有效库存上限；按「内→外」顺序裁剪 takePrepaid 再 takePostpaid
// 裁撤：单池 nPool，takePrepaid = min(replicas, nPool)，无 POSTPAID
// 非预测类型(滚服/春保池/绿通): nPrepaid = replicas（无预测约束）
```

## Risks / Trade-offs

- [zone=all 取各 zone 之和假设可跨 zone 分摊；若某次落单被钉死单 zone 可能高估] → 试算本就不保证最终落单，申领时按实时资源决定；sum 与「zone=all 可跨 zone 分摊下单」语义一致，风险可接受。
- [族级扣减依赖 GetAllDeviceTypeMap 与 GetPlanTypeAvlDeviceTypesV2 的族判定口径一致] → 二者均以 TechnicalClass+CoreType 判定同族（`IsDeviceMatched`），口径统一；occupied 子单含未知/已禁用机型（cpu_core 取不到）时**直接返回错误**（不再按 0 处理），避免占用扣减口径不明导致结果失真。
- [主请求机型本身未知/已禁用] → `splitApplyOrder` 取 `deviceTypeMap[device_type].CpuCore`，若 cpu_core ≤ 0 记 Warn 并返回**空子单列表**。
- [occupied 仅扣不校验，可能扣成负] → 统一截断为 0，避免负值穿透。
- [裁撤重复计数] → 显式只算一次（单池），已在 spec 固化。

## Open Questions

- 接口响应时间 P99 上限待产品确认（不阻塞实现）。
- 主单是否需要回带入参回显（如汇总 replicas）—— 当前按纯 `{ suborders }` 实现，如需要可后续追加汇总字段。
