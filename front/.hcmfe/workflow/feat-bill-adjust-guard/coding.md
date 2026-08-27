# Coding — feat-bill-adjust-guard

**TAPD**: [#1069995598137371364](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598137371364)

> 落码入口：无（现网 TSX adjacent）。不走 page-list。HTTP 以 api.md 为准。不要改 Go、`docs/api-docs`、预付费模块、新建表单。

## 执行顺序

1. 常量 + 列表行类型 + 行守卫工具
2. 现网调整列表：两列 Tag + 编辑/删除/勾选守卫
3. lint --fix

## 文件

- `src/constants/bill.ts`
- `src/typings/bill.ts`
- `src/views/bill/bill/adjust/row-guard.ts`
- `src/views/bill/bill/adjust/index.tsx`

## 改动点

### 常量 / 类型

`src/constants/bill.ts` 在现网 `BILL_ADJUSTMENT_STATE__MAP` 旁增加：

- 推送：`unpushed` 未推送 / `pushing` 推送中 / `pushed` 已推送 / `failed` 失败
- 定账：`unsettled` 未定账 / `settled` 已定账
- theme：pushing=`info`，pushed=`success`，failed=`danger`；settled=`success`；其余默认

`src/typings/bill.ts` 增加 **列表行** 可选字段类型（`source` / `source_id` / `push_status` / `push_fail_reason` / `settle_state`）。**不要**把这些塞进新建用的 `AdjustmentItem`。

### 行守卫 `row-guard.ts`

| 函数 | 规则 |
|------|------|
| `canMutateAdjustmentRow` | 无 `source` → 仅 `unconfirmed`；`prepaid` / `pushing` / `settled` → false；`manual` 且非 pushing、未 settled → true（含已确认）；未知 source → 同无 source |
| `canSelectAdjustmentRow` | 必须 `unconfirmed`，且非 prepaid、非 pushing、非 settled |
| `getAdjustmentMutateDisableTip` | 预付费 →「OFS 预付费调账不可人工修改。」→ 推送中 → 已定账 → 现网已确认文案 |

勾选同时卡住批量确认/批量删除（现网 `isCurRowSelectEnable`）。

### 列表页 `index.tsx`

- 在「调账状态」后、「操作」前插入「推送状态」「定账状态」。空/未知枚举 `--`，不出空 Tag。
- `failed` 且 `push_fail_reason` 有值：失败 Tag tooltip；否则无该说明。
- `source_id` 不渲染、不跳转。
- 编辑/删除：`disabled={!canMutate}`，tooltip 在禁用时展示。
- `isCurRowSelectEnable` 改为 `canSelectAdjustmentRow`。
- 仍用 `reqBillsAdjustmentList`；搜索/导出/新建/导入不改。
- 写成功仍 `getListData()`；不乐观改 `state` / `push_status`。
- 列固定：选择列 `fixed: 'left'`；调账状态、推送状态、定账状态、操作 `fixed: 'right'`。币种、备注不钉住。

## 明确不做

- 预付费菜单/列表/详情/导出
- `source` / `source_id` 列
- 推送「超时」、失败原因独立列
- 改 `src/api/bill` 请求封装（只扩类型与页面消费）
- 改手工新增表单、审计页
