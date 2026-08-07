---
name: comp-data-list
description: Use when implementing HCM 局部表格、页内嵌列表或 search+data-list 组合，包括搜索条件、数据列、分页和列表操作。
---

# HCM 局部列表（comp-data-list）

## 使用时机

- 新建或调整页面区域中的局部表格、页内嵌列表
- 组合 `search.vue` + `data-list.vue` 实现查询与数据展示
- 复用搜索条件、表格列及多类型 Factory 模式

整页路由、菜单和一级视图组装由 `page-list` 负责，不在本 Skill 中处理。

## 前置学习（按顺序阅读）

1. **必须先读** `../comp-field-model/SKILL.md` — 搜索 condition、表格 column 与多类型 Factory 约定
2. `./references/search-pattern.md` — 搜索区域、条件定义与 URL 同步
3. `./references/data-list-pattern.md` — 表格列、分页、排序与操作事件

## 参考示例

| 文件 | 说明 |
|------|------|
| `./assets/search.vue` | 通用搜索组件 |
| `./assets/search-condition-factory.ts` | 多类型搜索条件工厂 |
| `./assets/search-condition-tcloud.ts` | 搜索条件定义示例 |
| `./assets/data-list.vue` | 通用数据表格组件 |
| `./assets/data-list-column-factory.ts` | 多类型表格列工厂 |
| `./assets/data-list-column-tcloud.ts` | 表格列定义示例 |

## 实现步骤

1. 确认搜索字段、表格列、分页排序、列表操作及承载区域。
2. 先按 `comp-field-model` 定义 condition / column：多类型使用 Factory + 对应类型文件，单类型直接使用 `getModel` 或静态数组。
3. 复制并调整 `search.vue`，接入 fields / condition，处理 search、reset 与 URL query 同步。
4. 复制并调整 `data-list.vue`，接入 columns / list / pagination，配置特殊列渲染、权限与操作事件。
5. 在所属页面或组件中组合 Search + DataList，负责请求、状态和交互；不在此处新增整页路由。

## 注意事项

- 搜索条件与表格列模型遵循 `comp-field-model`，不要重复定义另一套字段约定。
- 操作按钮必须接入权限控制，并根据行数据设置禁用状态。
- 切换 vendor / resourceType 时清空旧搜索条件和列表数据。
