# 单据/工单

> status: drafted · kind: module
> globs: `src/views/ticket/**`

工单/单据模块，含业务(biz)/服务(srv)/资源多套入口，覆盖主机申领、资源预测等单据的列表、详情、审批与子单流转。

## 职责

- 单据列表与详情展示（如资源预测单据、主机申领单据）。
- 单据类型/状态的展示适配与筛选。
- 子单（sub-ticket）列表与流转、审批交互。

## 关键文件

- `children/resource-plan/list/` — 资源预测单据列表：
  - `children/data-list/column.ts` — 列定义（列头、render、色值语义）。
  - `children/search/condition.ts` — 搜索/筛选项定义。
- `children/resource-plan/sub-ticket/` — 资源预测子单列表/详情。
- 列表/筛选数据源：`@/store/ticket/resource-plan.ts`（`getTicketList` / `getTicketStatusList` / `getTicketTypeList`）。

## 关键流程 / 注意事项

### 资源预测单据「类型」（ticket_type）

- 主单类型枚举：`add`（新增）/ `adjust`（调整）/ `delete`（取消）/ `budget_declare`（预算申报，由 finops 预算同步生成）。
  - 类型定义见 `store/ticket/resource-plan.ts` 的 `IResourcePlanTicketItem.ticket_type`。
- **「类型」列展示与「类型」筛选项均后端驱动**：
  - 列展示后端返回的 `ticket_type_name`（`column.ts` 中 `ticket_type_name` 列无自定义 render）。
  - 筛选下拉选项来自 meta 接口 `/api/v1/woa/metas/ticket_types/list`（`condition.ts` 的 `ticket_types`，`getTicketTypeList`）。
  - 新增类型时前端通常只需补齐 TS 类型联合值与基于类型的渲染逻辑，无需硬编码中文枚举。
- **「CPU核数(已审批数)」列色值语义**（`column.ts`）：
  - 增量类型（`add` / `budget_declare`）→ 绿色 `#299e56`；其余（`adjust` / `delete`）→ 红色 `#ea3636`。
  - 特例：`类型=调整` 且 `状态=部分失败` 时该列显示 `--`。
  - 普通「CPU核数」列为无颜色纯文本。
- 子单类型 `sub_ticket_type`（add/cancel/adjust/transfer）与主单 `ticket_type` 是不同维度，互不影响。
