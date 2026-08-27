# Design：预付费账单查询结果导出

> 编写前已通过 `blueking-figma-dev` §0–5.5（经 `wf-design-figma-intake` 包装早停）识别稿面。冲突以稿面 + 用户确认为准。**本阶段不出业务代码。**
>
> 导出按钮视觉认列表表格工具条节点；确认弹窗 / loading / 终止导出**无独立稿**，沿用现网 `ExportToExcelBatchButton`。

## 0. 本子需求范围

| 在本迭代内 | 不在本迭代内 |
|------------|--------------|
| 列表表格上方「导出」按钮（接上现网占位） | 列表筛选、列、分页、详情 |
| 按当前筛选导出权限范围内全部命中行（xlsx） | 勾选行导出 |
| 确认弹窗、导出中 loading、可终止 | 详情 / 分摊表 / 账单调整导出 |
| 列与列表 22 列一致、核算不遮蔽 | CSV、后端独立导出接口 |
| 0 条不可点；不设 1000 条业务上限 | 改造 `ExportToExcelBatchButton` 通用逻辑（除非缺 outline 等对齐稿面的最小透传） |

## 1. 设计稿引用

| 状态 | Figma | Node |
|------|-------|------|
| 表格面板（工具条所在） | https://www.figma.com/design/un9Rys4jsWtSSjrW1TJkOO/%E8%B5%84%E6%BA%90%E8%BF%90%E8%90%A5?node-id=708-10421 | `708:10421` |
| **本期主认：导出按钮** | https://www.figma.com/design/un9Rys4jsWtSSjrW1TJkOO/%E8%B5%84%E6%BA%90%E8%BF%90%E8%90%A5?node-id=708-10423 | `708:10423`「导出」 |

fileKey：`un9Rys4jsWtSSjrW1TJkOO`。

Code Connect：`708:10423` 映射为 Button（基础 / 中尺寸 / 左 Icon / 无右 Icon / 常规）。确认弹窗不在该节点上。

## 2. 信息架构 / 布局

仍在预付费列表页，不新开页面：

1. 搜索区、表格、分页不变。
2. 表格面板左上工具条：替换现网禁用占位，落地可点的「导出」。
3. 稿面按钮：白底描边、左下载托盘箭头、文案「导出」、14px。
4. 点击后走现网批量导出确认弹窗（申领主机「导出全部」同类），不是页内新抽屉。

## 3. 关键交互

1. 当前筛选 `count === 0`：按钮 disabled，不弹窗、不下载。
2. `count > 0`：弹出确认（已选择 N 条预付费账单 + 确认提示）。N 用列表接口 `count`，不是当前页条数。
3. 确认后按**当前筛选**分页拉全量（接口单页上限仍 500），本地生成 xlsx 下载。
4. 导出中：弹窗 loading，不可关页逃避；可点「终止导出」。
5. 成功 Toast「导出成功」；失败在弹窗内展示原因，不留下半成品文件。
6. 不设 1000 条业务拦截。组件默认安全上限（约 45 万）与单文件拆分（约 15 万）沿用通用组件，不改成 1000。
7. 超过安全上限时沿用组件现成失败提示（筛选后再导出），不另做业务文案。
8. 无勾选导出、无 CSV。

## 3.x 关键图标语义（Coding 必读）

| 稿面位置 | 语义描述 | Node | 项目候选（类名 / 组件，可空） |
|----------|----------|------|------------------------------|
| 表格工具条「导出」 | 左：向下箭头落入托盘 | `708:10423` | `<i class="hcm-icon bkhcm-icon-download" />`（组件 `showIcon` 已用此类名；`bkhcm-icon-export` 为备选） |

## 3.y 组件候选（Coding 必读）

| 稿面区域/语义 | 组件候选 | 体系（bkui / magic / 扩展包） | 文档确认（已读 reference / 未读） | 复用层级（page/comp/adjacent/none，可空） | 落码入口（skill 名或「无」，可空） | 项目路径/说明（可空） |
|---------------|----------|------------------------------|----------------------------------|------------------------------------------|-----------------------------------|----------------------|
| 导出按钮 + 确认/loading/终止 | `ExportToExcelBatchButton`（基础按钮 + 左 Icon） | HCM adjacent | 已读组件 props 与申领主机/裁撤用法 | adjacent | 无 | `src/components/export-to-excel-batch-button/index.vue`。对标 `src/views/ticket/children/host-apply/device`「导出全部」 |
| 列表页工具条挂载 | 现网列表 `index.vue` toolbar | HCM page | 已读 page-list 入口组装 | page | page-list | 替换 `src/views/prepaid-bill/index.vue` 占位按钮，不新建页面 |
| 导出列 | 列表 22 列投影为 `ExportColumn` | HCM field-model | 已读 `ExportColumn` | adjacent | 无 | 从 `children/list/data-list/column.ts` 投影；`exportFormatter` 出名称/枚举/金额文案 |

待确认：

- 组件根 `bk-button` 默认非描边；稿面是描边。coding 用最小方式对齐（透传 `outline` 或 toolbar 样式），不要为此重做一套按钮。
- 文件名「预付费账单」（组件会加时间戳）。

## 4. 状态流转

```text
列表有 count
  ├─ 0 → 按钮 disabled
  └─ >0 → 点「导出」
        ├─ 取消 → 关闭弹窗
        ├─ 确认 → 分页拉全量
        │     ├─ 终止 → 中止请求，不下载
        │     ├─ 成功 → 下载 xlsx + Toast
        │     └─ 失败 → 弹窗错误，不下载
        └─ 超过组件安全上限 → 失败提示，不下载
```

## 5. 异常 / 边界态

| 场景 | 展示 |
|------|------|
| 筛选命中 0 | 按钮不可点 |
| 拉取中途失败 / 中止 | 不下载；失败展示原因，中止直接关 |
| 未定账行 | 核算两列导出真实值，与列表一致 |
| 无权限账号 | 文件中不会出现（与 list 同一筛选+鉴权） |
| 超大结果 | 不设 1000 截断；走组件默认上限与分文件 |

## 6. 与 PRD 差异（如有）

| 项 | PRD | 稿面/确认结论 |
|----|-----|---------------|
| 按钮文案 | 「导出」 | 稿面「导出」，跟 PRD |
| 1000 条上限 | 不设业务上限 | 跟 PRD；组件安全上限保留 |
| 核算遮蔽 | 跟列表不遮蔽 | 跟 PRD |
| 确认弹窗 | 导出过程中有进行中状态 | 稿面无弹窗；用现网批量导出确认框 |

## 7. 与 PRD 验收映射

| PRD | Design |
|-----|--------|
| AC-009 多条可导出、列与列表 22 列一致 | 确认后按筛选拉全量；`ExportColumn` 对齐列表列 |
| AC-011 0 条不可导出 | `disabled` when count=0 |
| AC-012 未定账核算跟列表 | 导出 formatter 不按 settle_state 改 `--` |
| AC-013 权限隔离 | 复用当前 list filter，不另拼全量 |
| AC-P02 进行中 / 成功 / 失败 | 组件 InfoBox loading + Toast / 失败态 |

## 8. 实现边界（design 纪要）

| 项 | 内容 |
|----|------|
| 目标目录意向 | 仍在 `src/views/prepaid-bill/`；工具条在列表 `index.vue`；拉数可进 `src/store/prepaid-bill/` |
| 组件/手写边界 | **必须**用 `src/components/export-to-excel-batch-button/index.vue`。对标申领主机「导出全部」：`showConfirmDialog` + `request(signal)` + `pickNum=pagination.count` + `showIcon`。不要用裁撤的 `use-custom-dialog`（无额外表单项）。不要手写另一套 exceljs 流程。 |
| 明确不做 | 勾选导出、CSV、详情导出、1000 条业务拦截、改列表列模型本身 |
| 数据（真接口 / mock） | 复用 `POST /api/v1/account/bills/prepaid_items/list`，当前筛选 + 分页拼装；不新开导出 API |
