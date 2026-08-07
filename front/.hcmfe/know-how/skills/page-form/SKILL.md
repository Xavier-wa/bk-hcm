---
name: page-form
description: Use when assembling an HCM 新建/编辑页面的目录、入口、路由或 sideslider，并组合字段模型与局部表单组件。
---

# HCM 表单页编排（page-form）

## 使用时机

- 组装资源的新建或编辑页面
- 将局部表单接入 sideslider 或独立路由
- 配置页面入口、提交/关闭流程及路由

## 前置学习（按顺序阅读）

1. `../comp-field-model/SKILL.md` — 字段定义与多类型 Factory 约定
2. `../comp-form/SKILL.md` — `form.vue`、`create.vue`、`edit.vue` 的局部实现
3. `./references/form-page-architecture.md` — 表单页目录、入口与承载方式

## 参考示例

| 文件 | 说明 |
|------|------|
| `./references/form-page-architecture.md` | Sideslider / 独立路由的页面组装 |
| `../comp-form/SKILL.md` | 局部表单组件与示例入口 |

## 本 skill 只负责

- 规划模块目录、页面入口和路由。
- 组装 `create.vue` / `edit.vue`，管理 sideslider 或独立页面状态。
- 调用对应 comp（`comp-form`）完成局部实现；字段模型和组件内部逻辑不在此重复定义。

## 创建步骤

### Step 1 — 确认需求

| 信息项 | 说明 |
|--------|------|
| 模块名称 | kebab-case，如 `vpc-form`、`cloud-account` |
| 新建接口 | `POST` 接口地址 |
| 编辑接口 | `PUT/PATCH` 接口地址 |
| 是否多类型 | 是否需要支持多云（VendorEnum）或多资源类型（ResourceTypeEnum）？ |
| 表单字段 | 字段名、类型、是否必填、验证规则、选项、组件配置 |
| 编辑禁用字段 | 编辑时哪些字段不可修改 |
| 字段联动 | 如 A 字段 change 时自动填充 B 字段 |
| 承载方式 | Sideslider（列表页弹窗）/ 独立路由页面 |

### Step 2 — 定义字段

按 `comp-field-model` 定义字段；多类型决策统一参考其 `references/multi-type-pattern.md`。

### Step 3 — 实现局部组件

按 `comp-form` 实现 `form.vue`、`create.vue`、`edit.vue`，本 Skill 不重复组件细节。

### Step 4 — 组装页面

1. 按架构文档创建目录及入口。
2. Sideslider 模式由宿主入口管理显示、提交、关闭和刷新状态。
3. 独立页面使用吸底布局，并注册新建/编辑路由。
4. 页面路由操作使用 `routerAction`，最后运行 `bkdevbuddy lint --fix`。
