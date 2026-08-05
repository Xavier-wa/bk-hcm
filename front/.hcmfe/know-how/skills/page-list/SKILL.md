---
name: page-list
description: Use when assembling an HCM 资源列表页的目录、入口、一级视图、路由或 sideslider 状态，并组合局部搜索与表格组件。
---

# HCM 列表页编排（page-list）

## 使用时机

- 组装资源列表页入口
- 在业务、资源或工作台一级视图接入同一组 children
- 管理页面级请求、权限、路由和 sideslider 状态

## 前置学习（按顺序阅读）

1. `../comp-field-model/SKILL.md` — 搜索 condition、表格 column 与多类型 Factory 约定
2. `../comp-data-list/SKILL.md` — 局部 Search + DataList 实现
3. `./references/list-page-architecture.md` — 列表页目录、入口与一级视图职责

## 参考示例

| 文件 | 说明 |
|------|------|
| `./assets/index.vue` | 入口骨架：注入上下文、组装 Search + DataList、管理 sideslider 状态 |
| `../comp-data-list/SKILL.md` | 搜索与表格局部组件及示例入口 |

## 本 skill 只负责

- 规划模块目录、`index.vue` / 一级视图入口和路由。
- 组装 Search + DataList，并管理请求、分页及 create/edit/details sideslider 状态机。
- 调用对应 comp（`comp-data-list`）完成局部实现；字段模型和组件内部逻辑不在此重复定义。

## 创建步骤

### Step 1 — 确认需求

| 信息项 | 说明 |
|--------|------|
| 模块名称 | kebab-case，如 `vpc-list`、`cloud-account` |
| API 接口 | 列表查询接口地址 |
| 是否多类型 | 是否需要支持多云（VendorEnum）或多资源类型（ResourceTypeEnum）？ |
| 搜索字段 | 字段名、类型、选项、特殊查询规则 |
| 表格列 | 列名、类型、是否排序、自定义渲染 |
| 操作按钮 | 新建 / 编辑 / 删除 / 查看详情 / 其他 |
| 所属一级视图 | 业务（businessViews）/ 资源（resourceViews）/ 工作台（serviceViews） |

### Step 2 — 定义字段

按 `comp-field-model` 定义搜索 condition 和表格 column；多类型决策统一参考其 `references/multi-type-pattern.md`。

### Step 3 — 实现局部组件

按 `comp-data-list` 实现 Search + DataList，本 Skill 不重复搜索、表格、分页和操作列细节。

### Step 4 — 组装页面

1. 按架构文档创建目录；可从 `./assets/index.vue` 组装入口。
2. 在入口中接入 Store、URL query、分页、权限和局部组件事件。
3. 管理 create/edit/details sideslider 状态；独立页面通过 `routerAction` 跳转。
4. 注册 route-config、一级视图、菜单和权限，最后运行 `bkdevbuddy lint --fix`。
