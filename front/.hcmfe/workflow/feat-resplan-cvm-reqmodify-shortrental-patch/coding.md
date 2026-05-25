# feat-resplan-cvm-reqmodify-shortrental-patch 实施方案

> 工作流: `feat-resplan-cvm-reqmodify-shortrental-patch`  
> 需求: `req-resource-plan-2027-cvm-budget`  
> 分支: `feat-resplan-cvm-reqmodify-shortrental-patch`

## 背景

资源预测「修改需求」页从单据详情 `GET .../plans/resources/tickets/{id}` 回填表单。`mapDemand` 此前未映射短租字段与 CBS 推导字段，导致侧栏「修改预测需求」中：

- 短租项目：**短租退回日期** 为空
- CVM+CBS：**云磁盘容量/实例** 恒为 0

## 根因

1. `return_plan_time`、`demand_source` 未从 `updated_info` 写入 `IPlanTicketDemand`
2. 接口仅下发 `cbs.disk_size`，无 `disk_per_size` / `disk_num`，需用 `disk_size ÷ cvm.os` 反推（与 `usePlanStore.convertToPlanTicketDemand` 一致）
3. 部分新增驳回单 `updated_info` **无** `demand_res_types`，需按 cvm/cbs 是否有量推断
4. `cvm.os` 可能为字符串（如 `"1"`），需 `Number(os)` 再参与计算
5. `TicketDemands` 类型与真实响应不一致（`original_info` 可为 `null`、字段可选）

## 改动文件

| 文件 | 改动 |
|------|------|
| `src/views/business/resource-plan/modify/utils.ts` | 补映射与 `deriveDiskPerSize` / `deriveDiskNum` / `deriveDemandResTypes` |
| `src/typings/resourcePlan.ts` | 新增 `TicketDemandItem`、`TicketDemandCvm`、`TicketDemandCbs`；`TicketDemands` 与详情响应对齐 |

## 映射规则（摘要）

| 表单字段 | 来源 |
|----------|------|
| `return_plan_time` | `updated_info.return_plan_time` |
| `demand_source` | `updated_info.demand_source`（默认「指标变化」） |
| `demand_res_types` | 接口有则透传；无则按 cvm/cbs 推断 |
| `demand_res_type` | 单类型用该项，多类型默认 `CVM` |
| `cvm.os` | `Number(updated_info.cvm.os)` |
| `cbs.disk_per_size` | `floor(disk_size / os)`（os>0） |
| `cbs.disk_num` | 仅 CBS 单资源时 `floor(disk_size / disk_per_size)` |

## 样例数据核对（脱敏）

单据 `000000do`，`obs_project=短租项目`，`return_plan_time=2026-08-15`，`os="1"`，`disk_size=1` → 表单应为退回日期 **2026-08-15**、云磁盘容量/实例 **1** GB。

## 不在范围

- 修改 `Add` 组件 `isEdit` 调整类型流程
- 后端详情接口补 `demand_res_types` 字段
- 首迭代 `feat-resplan-cvm-reqmodify` 其它模块

## 验证

- 开发自测：修改页打开短租驳回单 → 修改需求侧栏字段回填正确（用户已确认 PASS）
- 合入前：`hcmfe lint` 已通过
