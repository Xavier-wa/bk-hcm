---
name: comp-detail
description: Use when implementing HCM 局部详情、sideslider 详情或基于 display-value 的页内详情展示。
---

# HCM 局部详情（comp-detail）

## 使用时机

- 新建或调整页面区域中的局部详情组件
- 在 sideslider、抽屉或弹窗中展示资源详情
- 使用 GridContainer / GridItem 与 `display-value` 渲染字段

独立详情页的路由、菜单和一级视图组装由 `page-detail` 负责，不在本 Skill 中处理。

## 前置学习（按顺序阅读）

1. **必须先读** `../comp-field-model/SKILL.md` — 展示字段、分组、render 与多类型 Factory 约定
2. `./references/grid-layout-pattern.md` — GridContainer / GridItem 分组布局
3. `./references/display-value-pattern.md` — `display-value` 绑定、外观与路由跳转

## 参考示例

| 文件 | 说明 |
|------|------|
| `./assets/details.vue` | 局部详情组件骨架 |
| `./assets/field-factory.ts` | 多类型字段工厂 |
| `./assets/field-tcloud.ts` | 多类型详情字段定义示例 |

## 实现步骤

1. 确认详情字段、分组、承载容器及关联资源交互。
2. 先按 `comp-field-model` 定义字段：多类型使用 Factory + `field-<type>.ts`，单类型直接使用 `getModel`。
3. 复制并调整 `details.vue`，通过 GridContainer / GridItem 按 group 渲染字段。
4. 默认使用 `display-value`；render 依赖其他字段时传完整 data，无法满足时使用模板覆盖。
5. 在所属页面或组件中接入局部详情，负责 sideslider 状态、数据加载和关闭；不在此处新增整页路由。

## 注意事项

- 字段模型遵循 `comp-field-model`，不要重复定义另一套字段约定。
- 详情中的路由操作使用 `routerAction.redirect` / `routerAction.open`，禁止直接使用 `router.push`。
- 切换 vendor / resourceType 时重新创建字段模型并刷新详情数据。
