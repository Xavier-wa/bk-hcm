# Design：资源预测-时间和数量同时调整（前端实现）

> 已通过 `wf-design-figma-intake` 包装执行 `blueking-figma-dev` §0–5.5；本阶段只沉淀设计与落地边界，不修改业务代码。  
> 总览稿：`2659:18103`（资源预测-CVM预测）。主实现帧：`2664:45348`（可编辑表格页）。  
> **本工作流一次交付**：按 TAPD 功能点拆包写完并执行；**期望到货时间的选择组件（F-003 / 稿面新日历）本次不做**，其余 F-001 / F-002 / F-004 及配套体验全部做完。

## 0. 本子需求范围

| 在本工作流内（全部执行） | 不在本工作流内 |
|---|---|
| F-001：同单同时改时间与数量（可编辑表格 + 取消二选一）；表头批量改日期（现有 DatePicker + PopConfirm，非新日历） | F-003：期望到货时间的**新选择组件** / 预测专用日历（稿面二期日历面板） |
| F-002：调整流程内新增 / 复制 / 移除预测行 | 完整页面导航、侧栏等非主内容区视觉重做 |
| F-004：废弃「部分延期」入口 + 迁移引导 | 审批端页面重做（总览稿未覆盖审批态重设计） |
| 字段级修改反馈、调整预览、提交 / 移除未修改 / 取消 | 后端实现（前端按 API 契约适配） |
| 预测 ID / 操作列固定，中间字段横向滚动 | 跨年组合调整；GPU 等其他资源类型页同步改造 |
| 单个调整与批量调整统一交互 | |

**时间字段策略（与 F-003 切割）**：同单「改时间」能力保留（F-001），单元格展示日期并允许沿用项目**现有** `DatePicker` / `DateTimePickerColumn` 最小接入；**不按 Figma 新日历稿视觉/交互还原**，不做表头「批量编辑」新组件。

## 0.1 按 TAPD 功能拆分的交付包（本工作流执行顺序）

| 包 | TAPD | 优先级 | 落地要点 | 主要文件意向 |
|---|---|---|---|---|
| P0-A | F-001 | P0 | 拆除 config/time 互斥；`mod` 改为 Ediatable 行内编辑；同一行可同时改时间字段与机型/数量；单行/批量入口统一 | `resource-manage/mod/**`、`add/type`、`usePlanStore`、`typings/plan` |
| P0-B | F-001 配套 | P0 | 修改格黄底 + 原值 tip；调整预览 CPU/内存/云盘前后值；提交 / 移除未修改 / 取消 | `mod/index.tsx`、`index.scss`、`useModColumn` |
| P1-A | F-002 | P1 | 操作列复制 / 新增 / 移除；新增行绿底、预测 ID 空；与修改混编同一调整单 | `operation-column`、`mod` 行模型 |
| P1-B | F-004 | P1 | 列表更多菜单移除「部分延期」；给出迁移引导（文案/提示） | `list/table/index.tsx`、`batch-postpone-sideslider` 入口切断 |
| 明确跳过 | F-003 | — | 期望到货时间**选择组件**不做 | — |

Coding 阶段按 `P0-A → P0-B → P1-A → P1-B` 顺序落地；API 阶段先确认组合调整提交契约再写提交模型。

## 1. 设计稿引用

| 状态 | Figma | Node |
|---|---|---|
| 总览（CVM） | [资源预测-CVM预测](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/%E4%B8%9A%E5%8A%A1%E8%B5%84%E6%BA%90?node-id=2659-18103&m=dev&focus-id=2659-18103) | `2659:18103` |
| 主实现页 | [调整预测需求—可编辑表格](https://www.figma.com/design/nKX02SsMK8StZYAEEw0MfK/%E4%B8%9A%E5%8A%A1%E8%B5%84%E6%BA%90?node-id=2664-45348&m=dev&focus-id=2659-18103) | `2664:45348` |
| 可编辑表格 | 同上 | `2664:50066` |
| 批量调整说明 | 同上 | `2675:60234` |
| 调整预览 | 同上 | `2672:54071` |
| 底部操作栏 | 同上 | `2672:53614` |
| （参考，不实现）二期日期选择 | 总览旁注「二期将期望到货时间的选择框更改为如下样式」 | `2678:99208` 等 |

## 2. 信息架构 / 布局

主内容区自上而下三层（聚焦主内容区表格，不重做壳）：

1. **可编辑预测表格**：行高 42px；左侧预测相关列 + 右侧操作列宜固定；中间字段横向滚动。
2. **调整预览**：浅黄摘要区，CPU / 内存 / 云盘「调整前 → 调整后」。
3. **底部操作栏**：「提交调整」「移除未修改」「取消」。

总览帧 `2659:18103` 同时包含列表页与调整页，列表页仅作上下文；编码以 `2664:45348` 主内容区为准。

## 3. 关键交互

### 3.1 进入与编辑（F-001）

- 「批量调整」「单个调整」进入同一可编辑表格页；差异仅为初始行数。
- 取消「调整配置 / 调整时间」互斥（含 sideslider Radio）。
- 可编辑字段（除日期选择组件新视觉外）：项目类型、城市、可用区、资源类型、机型类型、机型规格、实例数量、CPU、内存、云盘类型、单实例云盘容量、云盘总量、单实例磁盘 IO；期望到货时间列沿用现有日期能力（见 §0）。
- 机型规格选中后 hover 整格出机型详情 tip；未选不触发。

### 3.2 日期（F-003 新日历不做；表头批量改日期要做）

- **不做（F-003）**：预测专用日历、稿面二期日历面板（周选择视觉、预测内外条等）。
- **要做（F-001）**：行数据模型与提交支持期望到货时间变更；单元格可用现有 DatePicker 最小能力编辑/展示 `YYYY-MM-DD`。
- **要做（稿面 `2664:50073` / `2664:47705`）**：期望到货时间表头批量编辑图标 → PopConfirm + 现有 DatePicker，确认后写入全部行；禁选规则与行内一致。

### 3.3 行操作（F-002）

- 操作列：复制、新增、移除。
- 新增 / 复制行绿底；预测 ID 空，提交后生成。
- 最后一行移除需拦截。

### 3.4 修改反馈（F-001 配套）

- 未改：白底；已改单元格：`#fdf4e8`，hover「调整前为：\<原值\>」；新增：`#ebfaf0`；不适用：`-` / 灰白。

### 3.5 提交与清理

- 「移除未修改」去掉未 diff 的原始行。
- 整单校验后提交；失败保留现场。
- 「取消」带回源；有未保存改动二次确认。

### 3.6 废弃部分延期（F-004）

- 列表「更多」去掉「部分延期」。
- 入口移除后给出迁移引导（提示：时间与数量可在「调整」同一张单完成）。

### 3.7 调整预览

- 实时汇总 CPU / 内存 / 云盘前后值；后值 `#f59500` 强调。

## 3.x 关键图标语义（Coding 必读）

| 稿面位置 | 语义描述 | Node | 项目候选（类名 / 组件，可空） |
|---|---|---|---|
| 日期表头 | 批量编辑当前列 | `2664:47705` | `hcm-icon bkhcm-icon-batch-edit` + `BatchUpdatePopConfirm` |
| 日期单元格 | 日期展示 / 现有选择器 | `I2664:50073;329393:14185;329393:12064` | 优先复用 `DateTimePickerColumn` 内置图标 |
| 操作列 | 复制当前行 | `2664:48055` | `hcm-icon bkhcm-icon-copy` |
| 操作列 | 新增一行 | `2664:48056` | `hcm-icon bkhcm-icon-plus-circle-shape` |
| 操作列 | 移除当前行 | `2664:48057` | `hcm-icon bkhcm-icon-minus-circle-shape` |
| 调整预览 | 前值指向后值 | `2672:54001` | `hcm-icon bkhcm-icon-right-shape` |

## 3.y 组件候选（Coding 必读）

| 稿面区域/语义 | 组件候选 | 体系 | 文档确认 | 复用层级 | 落码入口 | 项目路径/说明 |
|---|---|---|---|---|---|---|
| 主可编辑表格 | `Ediatable` + `HeadColumn` | `@blueking/ediatable` | 无专用 skill；已核对现有用法 | adjacent | 无 | 宿主 `src/components/resource-plan/resource-manage/mod/`；参考 `dissolve/.../time-period-block.vue` |
| 下拉单元格 | `SelectColumn` | `@blueking/ediatable` | 同上 | adjacent | 无 | `dissolve/.../quota-offset/render-row.vue` |
| 数字/文本单元格 | `InputColumn` | `@blueking/ediatable` | 同上 | adjacent | 无 | 同上 |
| 日期单元格（最小） | `DateTimePickerColumn` 或现有 `DatePicker` | ediatable / bkui | 同上 | adjacent | 无 | **仅最小接入，不还原新日历稿** |
| 固定操作列 | `FixedColumn` + 项目操作列 | `@blueking/ediatable` | 同上 | adjacent | 无 | `src/components/ediatable/operation-column.vue` |
| 提交/清理/取消 | `Button` | `bkui-vue` | 编码前复核 | adjacent | 无 | 沿用 `mod/index.tsx` |
| 原值/机型 tip | `v-bk-tooltips` | bkui 指令 | 编码前复核 | adjacent | 无 | 沿用 mod diff 模式 |
| 调整预览 | Panel + 语义 HTML/CSS | 项目封装 | 已核对 | adjacent | 无 | `mod/index.tsx` + `index.scss` |

待确认：

- `@blueking/ediatable@0.0.1-beta.32` 无独立 skill reference；以项目稳定用法为准。
- `bkui-vue@2.1.0-beta.4` 精确版本文档服务曾失败；不得跨版本猜 API。

## 4. 状态流转

```text
列表进入调整
  → 载入原始预测 + 基线快照
  → 编辑 / 复制 / 新增 / 移除
  → 字段 diff 标记 + 调整预览刷新
  → 可选「移除未修改」
  → 整单提交
  → 成功进单据详情 / 失败保留现场
```

状态以字段级 diff 为准；同一行允许时间与配置同时变更。

## 5. 异常 / 边界态

| 场景 | 展示 / 行为 |
|---|---|
| 加载失败 | 全局错误反馈；不可提交 |
| 空数据 | 空态；可取消 |
| 必填缺失 | 单元格校验；整单不提交 |
| 上下游联动失效 | 清空/禁用不适用字段 |
| 资源类型不适用字段 | `-` / 灰白 |
| 最后一行移除 | 拦截 |
| 超宽 | 固定列 + 中间横滚 |
| 离开有改动 | 二次确认 |
| 提交失败 | 保留现场 |

## 6. 与 PRD 差异

| 项 | PRD | 本工作流确认 |
|---|---|---|
| 交付节奏 | 同需求多能力 | **同一工作流内**按 §0.1 拆包全部做完（除 F-003） |
| F-003 专用日历 | 本期能力 | **明确不做**（期望到货时间选择组件） |
| 时间调整 | 同单可改时间 | 保留数据与提交能力；日期 UI 用现有选择器最小接入 |
| 调整方式 | 取消二选一 | 格子表统一单行/批量 |
| 字段摘要 | 提交前摘要 | 格内黄底 + 原值 tip + 底部总量预览 |
| 部分延期 | 下线 + 迁移引导 | 移除入口 + 迁移提示（稿面未给独立样式时按项目 Message/Tooltip 惯例） |
| 总览稿 | 未指定 | 总览改为 `2659:18103` |

## 7. 与 PRD 验收映射

| PRD | Design |
|---|---|
| AC-001 同时改时间与数量 | P0-A：同行可改时间字段 + 配置字段并同单提交 |
| AC-002 仅改时间 | 仅时间 diff；配置保持基线 |
| AC-003 仅改数量 | 仅配置 diff |
| AC-004 字段级摘要 | P0-B：黄底 tip + 调整预览 |
| AC-005 调整内新增 | P1-A |
| AC-006 废弃部分延期 | P1-B |
| AC-007 纯延期/纯修改回归 | 单字段 diff 分别映射原语义 |
| AC-S01 权限不变 | 沿用现入口权限 |
| F-003 / 专用日历 | **本工作流不验收** |

## 8. 实现边界（design 纪要）

| 项 | 内容 |
|---|---|
| 目标目录意向 | 主改 `src/components/resource-plan/resource-manage/mod/`；列表 `list/table/index.tsx`（F-004）；`store/usePlanStore.ts`、`typings/plan.ts`；必要时动 `add/type` 互斥拆除 |
| 技术栈 | Vue 3.5、`bkui-vue@2.1.0-beta.4`、`@blueking/ediatable@0.0.1-beta.32` |
| 组件/手写边界 | Ediatable 列组件 + 项目 `operation-column`；预览 Panel；不新造基础表格 |
| 状态边界 | `originData` / `tableData`；拆除 config/time 互斥与 sideslider 二选一路径 |
| 接口边界 | 现 `adjust_type=update\|delay` 互斥不足以表达组合调整 → **API 阶段必须确认契约**后再编码提交 |
| 权限/路由 | 沿用 `/business/service/resource-plan-mod`、`/service/resource-plan/cvm/mod`；Coding 触点迁 `HcmAuth` + `routerAction` |
| 明确不做 | **期望到货时间新选择组件（F-003）**、页面壳重做、后端实现、GPU/审批页、design 阶段写业务代码 |
| 数据 | 真接口：`list_biz_resource_plan_demand` / `adjust_biz_resource_plan_demand` / `get_demand_available_time`；组合字段以 API 文档为准 |
| 执行计划 | 用户确认 Design → API（契约）→ Coding（§0.1 四包）→ Test → done 收口；全程同一 workflow |
