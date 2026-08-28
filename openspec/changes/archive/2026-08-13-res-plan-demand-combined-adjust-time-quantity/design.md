## Context

调整预测需求现有实现（`AdjustBizResPlanDemand` → `constructAdjustReq` → `CreateResPlanTicket` → `SplitAdjustTicket`）存在两类硬约束：

1. **入参层**：`AdjustRPDemandReqElem.DemandID` 必填，`Validate()` 对每条目强校验 `original_info`+`updated_info`。纯新增（无 `demand_id`、无 `original_info`）无法进入流程。
2. **提单层**：`ExamineDemandClass` 只能从 `demandIDs`（非空）查 DB 推导 `demand_class`，整批纯新增无可查条目，CVM/CA 无从判定；`constructAdjustReq` 的 lock 遍历 `demand.Original.DemandID`，纯新增会因 `Original==nil` 空指针。
3. **拆单层**：`getDemandsWithoutTransfer` 把「仅改期 + 关键属性未变」判为延期；但「改期且改量」本应走常规 adjust（调增/调减），否则数量变更无法正确落子单。

本提案在不新增 data-service 接口、不改 DB、不改离线统计的前提下，扩展既有 `res-plan-demand-combined-adjust` 能力：以请求级 `demand_class` 打通整批纯新增，以「过滤空 id + 分路构造」打通混批与改期改量，并校正拆单路由。

可直接复用的既有能力：
- `AreAllDemandBelongToBiz` / `ExamineDemandClass`（按非空 `demandIDs` 查 DB）。
- `constructUpdateDemands` / `constructDelayDemands` / `constructOriginalDemandMap`。
- `buildDemandsFromCreateReq`（纯新增构造，本次用于 `Original=nil` 的创建请求）。
- `splitAdjustDemandsToAddAndDelete`（`Original==nil` 已归为追加子单，纯新增追加自动生效）。

## Goals / Non-Goals

**Goals:**
- 支持同一调整批次内「改期+改量同时传」（update 类型）。
- 支持批次内既有需求与纯新增混批，以及整批纯新增。
- 整批纯新增批次通过请求级 `DemandClass` 确定 CVM/CA，并对整批 `demand_class` 一致性做校验（CVM/CA 不可混）。
- 纯新增不参与 lock；lock 仅覆盖含 `Original` 的 demand。
- 拆单路由与 CRP 延期/修改区分一致：仅改期→延期，改期+改量/换机型/仅改量→adjust。
- 纯新增被 `splitAdjustDemandsToAddAndDelete` 正确归为追加子单。

**Non-Goals:**
- 不暴露 `delay_os`（部分延期），页面行为与 CRP 一致。
- 不改 DB schema / data-service 接口 / 离线统计。
- 不改取消（Cancel）链路与其他 ticket 类型。
- 不对纯新增做滚服校验（纯新增无原需求，滚服限制按既有需求保持）。

## Decisions

### D1: 请求级 `DemandClass` 仅用于整批纯新增
`AdjustRPDemandReq.DemandClass` 用 `omitempty`，仅在「整批调整后所有 `demand_id` 均为空」时必填；混批时由有 ID 条目查 DB 推导，请求级 `demand_class` 被忽略（避免与 DB 推导冲突）。整批 `demand_class` 一致性在 logics 层校验。
- 备选：每条目带 `demand_class` —— 弃用，冗余且与 DB 唯一来源冲突。

### D2: 过滤空 `demand_id` 后再做归属/`ExamineDemandClass`
`AdjustBizResPlanDemand` 先 `slice.Filter` 出非空 `demand_id`，仅对该子集调用 `AreAllDemandBelongToBiz` 与 `ExamineDemandClass`；全空则退回请求级 `DemandClass`。归属校验天然只对既有需求生效，纯新增不参与。

### D3: 纯新增经 `buildDemandsFromCreateReq` 构造（`Original=nil`）
`constructAdjustReq` 内对无 `demand_id` 的条目，将其 `updated_info` 包装为 `CreateResPlanDemandReq`，调用 `buildDemandsFromCreateReq()` 产出 `Original==nil && Updated!=nil` 的 `ResPlanDemand`，与既有 update/delay 分支并行 append。lock 遍历时仅取 `demand.Original != nil` 的条目，规避空指针。

### D4: `AdjustRPDemandReqElem.Validate()` 按纯新增/修改已有分流
- 纯新增：`DemandID==""` → 仅 `updated_info` 必填，`original_info` 可空。
- 修改已有：`DemandID!=""` → `DemandID + original_info + updated_info` 必填。
- `update`：允许 `expect_time` 与资源量同时变更（不再互斥）。
- `delay`：仍要求 `expect_time`；`delay_os` 不在此暴露。

### D5: `validateAdjustResPlan` 跳过纯新增的原需求校验
纯新增条目 `original_info==nil`，其滚服等基于原需求的判定无意义，直接跳过；既有需求条目的滚服限制保持不变。

### D6: 拆单路由校正
`getDemandsWithoutTransfer` 对「改期类」进一步区分：仅当**仅改期且数量不变（同机型、非跨年）**才入延期组；「改期且改量」「换机型」「仅改量不改期」均返回走常规 adjust（调增/调减）。`delay` 类型提单（整单延期）仍入延期组。纯新增（`Original==nil`）已在 `splitAdjustDemandsToAddAndDelete` 归为追加，本路由无需改动 `add.go`/`sub_ticket.go`。

### D7: 主单落库校验放纯新增
`CreateResPlanTicketReq.Validate()` 对调整主单允许 `Original==nil && Updated!=nil` 的条目，与纯新增语义对齐；全 `Updated==nil` 等非法组合仍被拒。

## Risks / Trade-offs

- [整批纯新增 `demand_class` 由前端传入，可能被误填] → 由整批一致性校验兜底（混批时仍查 DB），且纯新增落库后 `demand_class` 随 ticket/demand 固化，后续拆分按实际值走。
- [改期+改量走 adjust 而非 delay，跨年延期仍走 adjust 以保证额度计算正确] → 与 `getDemandsWithoutTransfer` 既有跨年判定口径一致，已在代码固化。
- [纯新增不 lock，若并发对同一批纯新增重复提单] → 纯新增无既有需求可锁，依赖业务侧单据幂等/审核流控制，与取消/既有需求 lock 互补。
- [`delay_os` 不暴露，部分延期能力本期关闭] → 与 CRP 页面行为对齐，避免后端能力领先前端造成歧义；后续如需开放再补 `Validate` 与 `constructDelayDemands` 分支。

## Open Questions

- 整批纯新增是否需要额外的「数量上限/预测余量」校验（当前与既有纯新增提单逻辑一致，未新增约束）—— 视产品是否要求调整单内的预测余量校验而定，不阻塞本期实现。
- 纯新增追加子单在 `prepareAddSubTickets` 中是否触发转移池逻辑 —— 复用既有 `canTransfer` 判定，本期不新增。
