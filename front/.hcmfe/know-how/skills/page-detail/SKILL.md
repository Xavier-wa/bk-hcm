---
name: page-detail
description: Use when assembling an HCM 详情页的目录、入口、路由或 sideslider，并组合字段模型与局部详情组件。
---

# HCM 详情页编排（page-detail）

## 使用时机

- 组装独立路由详情页
- 将局部详情接入列表页 sideslider
- 配置详情入口、数据加载和路由

## 前置学习（按顺序阅读）

1. `../comp-field-model/SKILL.md` — 展示字段、分组、render 与多类型 Factory 约定
2. `../comp-detail/SKILL.md` — 局部详情、Grid 布局与 display-value 实现
3. `./references/detail-page-architecture.md` — 详情页目录、入口与承载方式

## 参考示例

| 文件 | 说明 |
|------|------|
| `./references/detail-page-architecture.md` | Sideslider / 独立路由的页面组装 |
| `../comp-detail/SKILL.md` | 局部详情组件与示例入口 |

## 本 skill 只负责

- 规划模块目录、独立详情入口和路由。
- 将 Details 接入页面，管理 sideslider 或独立页面的数据加载状态。
- 调用对应 comp（`comp-detail`）完成局部实现；字段模型和组件内部逻辑不在此重复定义。

## 创建步骤

### Step 1 — 确认需求

| 信息项 | 说明 |
|--------|------|
| 模块名称 | kebab-case，如 `vpc-detail`、`cloud-account` |
| 详情接口 | 详情查询接口地址（独立路由时需要） |
| 是否多类型 | 是否需要支持多云（VendorEnum）或多资源类型（ResourceTypeEnum）？ |
| 承载方式 | Sideslider（列表页弹窗）/ 独立路由页面 |
| 字段列表 | 字段名、类型、分组、是否自定义渲染 |
| 特殊交互 | 关联资源跳转、popover 悬浮、复制按钮等 |

### Step 2 — 定义字段

按 `comp-field-model` 定义展示字段；多类型决策统一参考其 `references/multi-type-pattern.md`。

### Step 3 — 实现局部组件

按 `comp-detail` 实现 Details，本 Skill 不重复 Grid、display-value 和特殊字段渲染细节。

### Step 4 — 组装页面

1. 按架构文档创建目录和承载入口。
2. Sideslider 模式由列表入口管理显示、选中数据和关闭状态。
3. 独立页面根据 route params 加载数据并注册详情路由。
4. 路由操作使用 `routerAction`，最后运行 `bkdevbuddy lint --fix`。
