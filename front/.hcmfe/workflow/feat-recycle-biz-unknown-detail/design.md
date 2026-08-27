# Design - 机房裁撤总览-业务名称未知数据支持显示及明细跳转

## 设计概述

本需求为机房裁撤总览页面的功能增强，无新设计稿。基于现有 UI 进行局部修改，属于精简豁免版设计文档。

## 交互设计

### 1. 裁撤总览表格 - 业务名称列显示

**现状**：
- `bk_biz_id` 在 `businessFullList` 中找不到时，`display-value` 组件返回 `--`
- `bk_biz_id === -1` 的汇总行显示"汇总"（带 icon）

**目标**：
- `bk_biz_id` 在 `businessFullList` 中找不到时，显示"未知"（替代 `--`）
- 汇总行（`bk_biz_id === -1`）保持不变

**视觉**：文字样式与现有业务名称按钮一致（蓝色文字按钮），无额外样式

### 2. 裁撤总览表格 - 业务名称按钮可点击性

**现状**：
- `bk_biz_id` 在 `businessFullList` 中找不到时，按钮 `disabled`（灰色不可点击）
- `bk_biz_id === -1` 的汇总行不渲染按钮，显示"汇总"文字

**目标**：
- `bk_biz_id` 在 `businessFullList` 中找不到时（非汇总行），按钮**可点击**
- 点击后跳转到裁撤明细 tab，传 `bk_biz_ids=[0]`
- 汇总行保持不变

### 3. 裁撤总览搜索栏 - 业务名称下拉

**现状**：
- 业务名称下拉选项来自 `businessAuthorizedList`（scope: 'auth'）
- 无"未知"选项

**目标**：
- 在业务名称下拉列表中增加"未知"选项
  - 显示文本：`未知`
  - 值：`0`（数字类型）
- 选项位置：列表末尾（或独立分组，视组件能力而定）
- 支持多选：可与已知业务同时选中

### 4. 裁撤明细页面 - 接收 bk_biz_id=0

**现状**：
- 从裁撤总览跳转时，URL query 携带 `bk_biz_ids` 参数
- 裁撤明细页面解析参数并查询

**目标**：
- 当 `bk_biz_ids=[0]` 时，正常查询（后端负责返回未知业务的设备明细）
- 前端无需特殊处理，透传参数即可

## 状态流转

```
裁撤总览（业务名称未知行）
  ↓ 点击"未知"按钮
裁撤明细（bk_biz_ids=[0] 筛选）
  ↓ 展示设备明细
```

## 组件层级

```
overview/index.vue
  ├── overview/search/search.vue（搜索栏）
  │   └── hcm-search-business → business-selector（需增加"未知"选项）
  └── overview/data-list/data-list.vue（表格）
      └── display-value → business-value（需显示"未知"）
```

## 设计豁免说明

- 无新设计稿：本需求是对现有页面的局部功能增强，不涉及页面布局变更
- 无新图标：不引入新的 icon 资源
- 无新组件：复用现有 `display-value`、`business-selector` 组件，仅修改其行为
- post-node icon-intake：skip（无图标变更）
- post-node comp-intake：skip（无新组件复用）
