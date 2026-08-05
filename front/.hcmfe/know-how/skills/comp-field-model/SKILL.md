---
name: comp-field-model
description: Use when defining or modifying HCM 字段模型（@Model/@Column、getModel、FieldFactory），供表单/详情/搜索/表格列共用；纯 UI 壳见 comp-form / comp-data-list / comp-detail。
---

# HCM 字段模型（comp-field-model）

## 使用时机

- 新建或修改字段装饰器类 / FieldFactory
- `comp-form` / `comp-data-list` / `comp-detail` 的前置必读
- design §3.y **仅当**需求是纯模型变更（无 UI 壳改动）时才可作为落码入口

## 前置学习

1. `./references/column-pattern.md` — `@Column` 参数、rules、apiOnly、meta.display
2. `./references/multi-type-pattern.md` — Factory vs 简化；vendor / resourceType

## 参考示例

| 文件 | 说明 |
|------|------|
| `./assets/field-factory.example.ts` | Factory 骨架；复制后按用途改名 |

## 步骤摘要

1. 判断多类型 → Factory + `field-<type>.ts`；否则单类 `getModel`
2. 按用途选择属性集（form rules / detail group+render / search / column）
3. 在对应 comp skill 的组件中 `createModel` / `getModel` 接入

## 不管什么

- form.vue / data-list / details 渲染与事件（各 comp-*）
- 页面路由与一级视图组装（page-*）
