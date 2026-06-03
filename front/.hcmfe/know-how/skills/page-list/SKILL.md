---
name: page-list
description: 创建 HCM 标准列表页。包含搜索区域、数据表格、分页、操作按钮（新建/编辑/删除/详情）的完整模式。当需要新增一个资源列表页时使用。
---

# 创建 HCM 列表页

## 使用时机

当需要创建一个资源列表页时使用此 Skill，典型场景：
- 新增业务模块的列表页（如 VPC 列表、安全组列表、账号列表）
- 改造老模块的列表页（按新模式重构）
- 在多个一级视图（业务/资源/工作台）下复用同一套列表组件

## 前置学习（按顺序阅读）

1. `./references/list-page-architecture.md` — 列表页整体架构、目录组织、入口职责
2. `./references/search-pattern.md` — 搜索区域模式（通用组件 + condition 定义 + URL 同步）
3. `./references/data-list-pattern.md` — 表格区域模式（通用组件 + column 定义 + 操作列）
4. `./references/multi-type-pattern.md` — 多云/多资源模式（Factory 机制与简化决策）

## 参考示例

| 文件 | 说明 |
|------|------|
| `./assets/index.vue` | 入口页面骨架（注入 vendor、组装 Search + DataList、管理 sideslider 状态） |
| `./assets/search.vue` | 通用搜索组件（接收 fields/condition，emit search/reset） |
| `./assets/search-condition-factory.ts` | 【多类型时】搜索条件工厂 |
| `./assets/search-condition-tcloud.ts` | 【多类型时】腾讯云搜索条件定义示例 |
| `./assets/data-list.vue` | 通用表格组件（接收 columns/list/pagination，emit 操作事件） |
| `./assets/data-list-column-factory.ts` | 【多类型时】表格列工厂 |
| `./assets/data-list-column-tcloud.ts` | 【多类型时】腾讯云表格列定义示例 |

## 创建步骤

### Step 1 — 确认需求信息

| 信息项 | 说明 |
|--------|------|
| 模块名称 | kebab-case，如 `vpc-list`、`cloud-account` |
| API 接口 | 列表查询接口地址 |
| 是否多类型 | 是否需要支持多云（VendorEnum）或多资源类型（ResourceTypeEnum）？ |
| 搜索字段 | 字段名、类型、选项、特殊查询规则 |
| 表格列 | 列名、类型、是否排序、自定义渲染 |
| 操作按钮 | 新建 / 编辑 / 删除 / 查看详情 / 其他 |
| 所属一级视图 | 业务（businessViews）/ 资源（resourceViews）/ 工作台（serviceViews） |

### Step 2 — 判断模式（关键决策）

```
模块是否需要支持多云/多资源类型？
├── 是 → Factory 模式
│   ├── 创建 search/condition-factory.ts + condition-<type>.ts
│   ├── 创建 data-list/column-factory.ts + column-<type>.ts
│   ├── 入口注入 currentVendor / currentResourceType
│   └── 子组件通过 inject 获取当前类型
└── 否 → 简化模式
    ├── 搜索条件直接写在 search/condition.ts（或入口中）
    ├── 表格列直接定义（数组或装饰器）
    └── 入口不需要注入类型
```

### Step 3 — 创建目录结构

```bash
views/<模块>/
├── index.vue
├── typings.ts                  # 模块内闭环类型（如 ISearchCondition）
├── utils.ts                    # 模块工具函数（如状态映射）
└── children/
    └── list/
        ├── search/
        │   ├── search.vue
        │   ├── condition-factory.ts      # 【多类型时】
        │   └── condition-<type>.ts       # 【多类型时】
        └── data-list/
            ├── data-list.vue
            ├── column-factory.ts         # 【多类型时】
            └── column-<type>.ts          # 【多类型时】
```

### Step 4 — 实现搜索区域

1. **搜索组件** `search.vue`：复制 `./assets/search.vue`，通常不需要修改（完全通用）
2. **条件定义**：
   - 多类型：复制 `./assets/search-condition-factory.ts` + `./assets/search-condition-tcloud.ts`，添加其他类型的定义文件
   - 单类型：直接写 `condition.ts`，参考 `./assets/search-condition-tcloud.ts` 的结构但去掉 `@Model` 装饰器（或直接写静态数组）
3. **入口中组装**：
   ```typescript
   // 多类型
   const searchModel = computed(() => SearchConditionFactory.createModel(currentVendor.value));
   const searchFields = computed(() => searchModel.value.getProperties());

   // 单类型
   const searchFields = ref<ModelPropertySearch[]>([...]);
   ```

### Step 5 — 实现表格区域

1. **表格组件** `data-list.vue`：复制 `./assets/data-list.vue`，根据业务需求调整：
   - 特殊列的渲染逻辑（如 `column.id === 'name'` 的点击事件）
   - 操作按钮的权限 Symbol 和禁用条件
2. **列定义**：
   - 多类型：复制 `./assets/data-list-column-factory.ts` + `./assets/data-list-column-tcloud.ts`
   - 单类型：直接写静态数组或单个装饰器类
3. **入口中组装**：
   ```typescript
   // 多类型
   const columnModel = TableColumnFactory.createModel(currentVendor.value);
   const dataListColumns = computed(() => columnModel.getProperties());

   // 单类型
   const dataListColumns = ref<ModelPropertyColumn[]>([...]);
   ```

### Step 6 — 实现入口页面

复制 `./assets/index.vue`，替换以下内容：
1. Store 导入和 API 调用
2. 权限 Symbol 导入
3. 搜索条件 / 表格列的获取方式（多类型用 Factory，单类型直接定义）
4. 操作事件的具体逻辑（提交表单、删除确认等）
5. 创建/编辑/详情的交互方式（组件均放在 `children/` 中引入，模式不变）：
   - **Sideslider 模式**：入口通过 `reactive` 管理 `isShow` 状态，在 `<template>` 中直接渲染 `<CreateForm>` / `<EditForm>` / `<Details>` 组件
   - **独立路由模式**：入口通过 `routerAction.redirect()` 跳转到子页面，子页面为独立 `.vue` 文件，内部引入 `<CreateForm>` / `<EditForm>` / `<Details>`。路由跳转禁止直接使用 `router.push`，必须使用 `routerAction`

### Step 7 — 注册路由和菜单

按 `fe-menu-route-architecture.mdc` 的 Checklist：
1. `constants/menu-symbol.ts` 添加 Symbol
2. `views/<模块>/route-config.ts` 定义路由（多视角时创建 entry-biz/entry-rsc/entry-srv）
3. `views/index.ts` 合入对应一级视图数组
4. `common/menu-service.ts` 注册菜单（需要展示的才注册）
5. `constants/auth-symbols.ts` + `common/auth-service.ts` 配置权限（如需要视图鉴权）

### Step 8 — Lint 修复

```bash
hcmfe lint --fix
```

## 注意事项

- **模块内闭环**：`typings.ts`、`utils.ts` 保持在模块内部，不对外暴露。如需跨模块复用，抽到 `common/` 或 `utils/`
- **改造老模块不删老文件**：新建文件迁移，老文件加 `@deprecated` 注释
- **操作按钮权限**：所有操作按钮（新建/编辑/删除）必须用 `hcm-auth` 包裹，并根据行数据控制 `disabled`
- **URL 同步**：搜索条件必须通过 `useSearchQs` 与 URL query 同步，支持刷新后保留筛选状态
- **表格列排序**：需要排序的列在 `@Column` 中标记 `sort: true`，并在入口监听 `route.query.sort/order`
- **多类型初始值**：当切换 vendor/resourceType 时，需要清空当前搜索条件和列表数据，避免旧数据残留
