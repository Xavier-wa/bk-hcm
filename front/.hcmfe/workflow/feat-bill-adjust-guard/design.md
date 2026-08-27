# Design：账单调整列表状态与守卫（无设计稿）

> **本迭代无独立设计稿**。豁免原因：用户确认走豁免；改现网「账单调整」列表，沿用该页表格与 `bk-tag` 形态，不另出视觉稿。**本阶段不出业务代码。**
>
> 用户确认：推送状态、定账状态两列用 **Tag** 展示（对齐现网「调账状态」列）。

## 0. 本子需求范围

| 在本迭代内 | 不在本迭代内 |
|------------|--------------|
| 列表新增「推送状态」「定账状态」两列（Tag） | 预付费管理菜单/列表/详情/导出 |
| 失败 Tag 上展示 `push_fail_reason` | 来源单据 ID 列 / 跳转预付费详情 |
| 行编辑/删除、勾选、批量确认/删除守卫 | 手工新增表单改造 |
| 预付费行禁用说明 tooltip | 审计页、推送状态人工置位 |
| 存量无来源字段时降级为现网「仅未确认」 | 新开页面或改 Header |

## 1. 设计稿引用

| 状态 | 说明 |
|------|------|
| N/A | 无新 Figma 稿 |

## 2. UI 参照

- **现网**：`src/views/bill/bill/adjust/index.tsx`（`/bill/bill-manage/adjust`）。表格、搜索、批量按钮、行内「编辑/删除」文字按钮、调账状态 Tag 均沿用。
- **Tag**：与现网「调账状态」同一套 `bk-tag`（已确认 `success`，未确认无 theme）。
- **PRD 文字约束**：两列必现；守卫矩阵见 PRD；预付费行说明「OFS 预付费调账不可人工修改」。

列位置：紧挨现网「调账状态」之后、「操作」之前，状态列成组。接口未返回时单元格 `--`，不出空 Tag。

### Tag 文案与 theme

**推送状态**（对应 `push_status`：`unpushed` / `pushing` / `pushed` / `failed`）

| 展示 | 枚举 | theme |
|------|------|-------|
| 未推送 | `unpushed` | 无（默认） |
| 推送中 | `pushing` | `info` |
| 已推送 | `pushed` | `success` |
| 失败 | `failed` | `danger` |

无「超时」。`failed` 且 `push_fail_reason` 有值时，Tag 上 tooltip 展示失败原因；非 `failed` 或原因为空则无该说明。

**定账状态**（对应 `settle_state`：`unsettled` / `settled`）

| 展示 | 枚举 | theme |
|------|------|-------|
| 未定账 | `unsettled` | 无（默认） |
| 已定账 | `settled` | `success` |

现网「调账状态」Tag 不改。

### 操作与 tooltip

禁用时仍渲染按钮，不可点；tooltip 优先级：预付费来源 → 推送中 → 已定账 → 现网「已确认无法编辑/删除」（仅来源缺失且已确认）。

预付费来源统一文案：**OFS 预付费调账不可人工修改。**

不可勾选的行：选择列 disabled，无法进入批量确认/删除。

## 3. 关键交互

1. 打开当月账单调整：多两列 Tag，查询仍走现网 list，不加无关请求。
2. 预付费行：编辑/删除禁用 + 上述 tooltip；勾选不可用。
3. 人工 + 非推送中 + 未定账：已确认也可编辑/删除；保存成功后列表刷新（未确认 + 未推送由接口结果体现）。
4. 推送中或已定账：编辑/删除禁用。
5. 批量确认目标仍只能是未确认；已确认人工行不可勾选确认。
6. 后端拒绝：提示接口错误，不乐观改行。

## 3.x 关键图标语义（Coding 必读）

本迭代无新增图标（状态用 Tag，操作沿用文字按钮）。

| 稿面位置 | 语义描述 | Node | 项目候选（类名 / 组件，可空） |
|----------|----------|------|------------------------------|
| — | 无 | N/A | 无 |

## 3.y 组件候选（Coding 必读）

| 稿面区域/语义 | 组件候选 | 体系（bkui / magic / 扩展包） | 文档确认 | 复用层级 | 落码入口 | 项目路径/说明 |
|---------------|----------|------------------------------|----------|----------|----------|----------------|
| 列表页（列 + 守卫） | 现网账单调整 TSX 页 | HCM adjacent | 已读 `adjust/index.tsx` | adjacent | 无 | `src/views/bill/bill/adjust/index.tsx`。不走 page-list（现网 TSX，不新建 Vue 列表） |
| 推送/定账状态 | `bk-tag` | bkui | 已读现网调账状态列 | adjacent | 无 | 与 `state` 列同一写法；theme 见 §2 |
| 行编辑/删除禁用 | 现网文字 Button + tooltip | bkui | 已读操作列 | adjacent | 无 | 扩 `disabled` 条件与 tooltip，不换组件 |
| 勾选 | 现网 `isRowSelectEnable` | HCM adjacent | 已读 | adjacent | 无 | 扩可选中条件，覆盖批量确认/删除 |

## 4. 状态流转

```text
打开账单调整列表
  ├─ 无新字段 → 两列 --；守卫按「来源缺失」
  └─ 有 source / push_status / settle_state
        ├─ prepaid → 不可编辑/删除/勾选
        ├─ pushing 或 settled → 不可编辑/删除
        └─ manual 且非 pushing 未 settled → 可编辑/删除（含已确认）
              └─ 保存成功 → 刷新列表
```

## 5. 异常 / 边界态

| 场景 | 展示 |
|------|------|
| 字段空 | `--`，不是空 Tag |
| 推送失败 | Tag「失败」`danger`；有 `push_fail_reason` 则 tooltip |
| 预付费行 | 操作禁用 + OFS tooltip |
| 写失败 | 接口错误提示，行数据不本地改 |
| 存量无 source | 仅未确认可操作 |

## 6. 与 PRD 差异（如有）

| 项 | PRD | 确认结论 |
|----|-----|----------|
| 两列形态 | 未规定控件 | **Tag**（本轮用户确认） |
| 推送「超时」 | TAPD 草稿有 | **去掉**；最新字段只有四态 |
| 失败原因 | TAPD 未单列 | 失败 Tag tooltip，不新开列 |
| `source_id` | TAPD 未要求列表展示 | **不进列表**、不跳转 |
| 守卫矩阵 | Q-005 B | 跟 PRD；`source=prepaid/manual` |

## 7. 与 PRD 验收映射

| PRD | Design |
|-----|--------|
| AC-021 两列可见、四态、无超时 | 插在调账状态后；Tag |
| AC-021b 失败原因 | `failed` Tag tooltip = `push_fail_reason` |
| AC-022 prepaid 不可改、不可勾选 | 行按钮 + selection 同时禁用 |
| AC-023 编辑后刷新 | 保存成功后 `getListData`，不乐观写 |
| AC-024 pushing/settled 禁用 | 操作列 disabled |
| AC-025 已确认不可批量确认 | 勾选条件含 unconfirmed |
| AC-026 无 source 不误放开 | 缺 source 时只放开 unconfirmed |
| AC-P04 无额外接口 | 只消费现网 list 新字段 |

## 8. 实现边界（design 纪要）

| 项 | 内容 |
|----|------|
| 目标目录意向 | 现网 `src/views/bill/bill/adjust/`；常量可放账单 constants |
| 组件/手写边界 | 继续 TSX + `bk-tag` / 文字 Button；不要用 page-list 重做整页 |
| 明确不做 | 预付费模块、新增表单、独立导出、改 Header、`source_id` 列、超时态 |
| 数据 | 复用现网调账 list 新字段：`source` / `source_id` / `push_status` / `push_fail_reason` / `settle_state`；不新开接口。`source_id` 本期只随行数据存在，列表不渲染 |
