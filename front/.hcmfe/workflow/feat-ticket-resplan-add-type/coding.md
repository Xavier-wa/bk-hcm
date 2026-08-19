# Coding — feat-ticket-resplan-add-type

## 需求

资源预测单据列表新增类型 `budget_declare`（预算申报）的前端展示适配与筛选项支持。
TAPD: [#1069995598136446978](https://<TAPD_HOST>/tapd_fe/69995598/story/detail/1069995598136446978)

## 关键结论（改动前调研）

- **"类型"列**：展示后端返回的 `ticket_type_name`（无自定义 render），后端返回新类型名"预算申报"即自动展示 → **前端无需改动**。
- **"类型"筛选项**：`condition.ts` 的 `ticket_types` 选项来自后端 meta 接口 `/api/v1/woa/metas/ticket_types/list`（动态渲染），后端返回 `budget_declare` 即自动出现 → **前端无需硬编码枚举**。
- **需前端手动适配的仅两处**：TS 类型联合值、"CPU核数(已审批数)"列的增量色值语义。
- 子单 `sub_ticket_type`（add/cancel/adjust/transfer）与主单 `ticket_type` 不同维度，本次不涉及。

## 改动点

**文件**: `front/src/store/ticket/resource-plan.ts` `front/src/views/ticket/children/resource-plan/list/children/data-list/column.ts`

1. `store/ticket/resource-plan.ts`
   - `IResourcePlanTicketItem.ticket_type` 联合类型新增 `'budget_declare'`，保证类型安全与基于类型的渲染逻辑正确。

2. `views/.../data-list/column.ts`（"CPU核数(已审批数)"列 render）
   - 将 `budget_declare` 与 `add` 一并视为"资源增量"，显示绿色正向（`#299e56`）；其余（`adjust`/`delete`）红色（`#ea3636`）。
   - 抽出 `isIncrement = type === 'add' || type === 'budget_declare'` 用于色值判断。
   - 普通"CPU核数"列保持现有无颜色纯文本展示（未改）。

## 依赖前提

- 展示与筛选生效依赖后端在列表接口与 meta 接口返回 `budget_declare` 及其中文名"预算申报"（属后端范围，本次不改后端）。

## 验收对应

- F-001 类型列展示 → 后端驱动（`ticket_type_name`）+ TS 类型补齐
- F-002 类型筛选项 → 后端 meta 驱动
- F-003 已审批核数增量色值 → column.ts `isIncrement`
- 无回归：原有 add/adjust/delete 展示与筛选逻辑未改
