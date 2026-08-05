---
name: comp-form
description: Use when implementing HCM 局部表单、抽屉表单、sideslider 表单或页内内嵌表单，包括 form/create/edit 组件。
---

# HCM 局部表单（comp-form）

## 使用时机

- 新建或编辑局部表单组件
- 在抽屉、sideslider、弹窗或页面区域中内嵌表单
- 复用 `form.vue`、`create.vue`、`edit.vue` 的表单结构

整页路由、菜单和一级视图组装由 `page-form` 负责，不在本 Skill 中处理。

## 前置学习（按顺序阅读）

1. **必须先读** `../comp-field-model/SKILL.md` — 字段模型、验证规则与多类型 Factory 约定
2. `./references/form-component-pattern.md` — `form.vue` 的核心结构与关键要点

## 参考示例

| 文件 | 说明 |
|------|------|
| `./assets/form.vue` | 字段模型驱动的通用表单组件 |
| `./assets/create.vue` | 新建表单包装 |
| `./assets/edit.vue` | 编辑表单包装 |
| `./assets/field-factory.ts` | 多类型字段工厂 |
| `./assets/field-tcloud.ts` | 多类型字段定义示例 |

## 实现步骤

1. 确认表单字段、新建/编辑差异、禁用项、联动事件和承载容器。
2. 先按 `comp-field-model` 定义字段：多类型使用 Factory + `field-<type>.ts`，单类型直接使用 `getModel`。
3. 复制并调整 `form.vue`：接入字段模型，过滤 `apiOnly`，创建数据实例，逐字段回填，配置组件 props 与事件。
4. 按需复制 `create.vue` / `edit.vue`，保持包装层轻量并透传 `validate`、`getFormData`。
5. 在所属页面或组件中接入局部表单，负责提交、关闭和刷新；不在此处新增整页路由。

## 注意事项

- 编辑回填必须逐字段显式赋值，避免混入接口额外字段。
- 输入类校验通常使用 `blur`，选择类校验通常使用 `change`。
- 字段定义与 Factory 遵循 `comp-field-model`，不要在本 Skill 重复另一套模型约定。
