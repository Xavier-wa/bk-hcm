---
name: page-detail
description: 创建 HCM 标准详情页。包含 GridContainer/GridItem 分组布局、字段展示（含自定义渲染）、路由跳转。当需要新增资源详情展示页时使用。
---

# 创建 HCM 详情页

## 使用时机

当需要创建一个资源详情展示页时使用此 Skill，典型场景：
- 列表页中点击"查看详情"展示的详情面板
- 独立路由的详情页（如 `/vpc/:id`）
- 改造老模块的详情展示部分

## 前置学习（按顺序阅读）

1. `./references/detail-page-architecture.md` — 详情页整体架构、目录组织、与列表页的关系
2. `./references/grid-layout-pattern.md` — GridContainer/GridItem 布局模式
3. `./references/display-value-pattern.md` — display-value 组件使用、字段类型映射、路由跳转规范
4. `./references/multi-type-pattern.md` — 多云/多资源字段 Factory 模式与简化决策

## 参考示例

| 文件 | 说明 |
|------|------|
| `./assets/details.vue` | 详情组件骨架（inject vendor、GridContainer 分组、display-value 渲染） |
| `./assets/field-factory.ts` | 【多类型时】字段工厂 |
| `./assets/field-tcloud.ts` | 【多类型时】腾讯云字段定义示例（@Model + @Column + group + render） |

## 创建步骤

### Step 1 — 确认需求信息

| 信息项 | 说明 |
|--------|------|
| 模块名称 | kebab-case，如 `vpc-detail`、`cloud-account` |
| 详情接口 | 详情查询接口地址（独立路由时需要） |
| 是否多类型 | 是否需要支持多云（VendorEnum）或多资源类型（ResourceTypeEnum）？ |
| 承载方式 | Sideslider（列表页弹窗）/ 独立路由页面 |
| 字段列表 | 字段名、类型、分组、是否自定义渲染 |
| 特殊交互 | 关联资源跳转、popover 悬浮、复制按钮等 |

### Step 2 — 判断模式（关键决策）

```
字段是否因云厂商/资源类型而不同？
├── 是 → Factory 模式
│   ├── 创建 details/field-factory.ts + field-<type>.ts
│   ├── 入口 inject currentVendor / currentResourceType
│   └── 通过 FieldFactory.createModel(vendor) 获取字段
└── 否 → 简化模式
    ├── 直接写单个装饰器类（如 fields.ts）
    └── 入口直接 getModel(DetailsFields)
```

### Step 3 — 创建目录结构

```bash
views/<模块>/
├── index.vue
├── children/
│   ├── list/                 # 列表区域（page-list skill）
│   └── details/              # 【本 skill】详情区域
│       ├── details.vue
│       ├── field-factory.ts      # 【多类型时】
│       └── field-<type>.ts       # 【多类型时】
```

### Step 4 — 定义字段模型

1. **多类型**：复制 `./assets/field-factory.ts` + `./assets/field-tcloud.ts`
   - 在 `field-<type>.ts` 中定义 `@Model()` 类
   - 每个字段用 `@Column(type, { name, group, meta })` 装饰
   - `group` 决定面板分组，`meta.display.render` 支持自定义渲染
2. **单类型**：直接写一个装饰器类，去掉 Factory

### Step 5 — 实现详情组件

复制 `./assets/details.vue`，替换以下内容：
1. `FieldFactory` 导入和调用（或替换为直接的 `getModel`）
2. `data` 的类型定义和字段取值逻辑
3. 特殊字段的 `<template>` 覆盖（如关联账号跳转、popover 等）
4. `display-value` 的 `value` 绑定（注意 `extension.xxx` 字段需要传整个 `data`）

### Step 6 — 接入入口

**Sideslider 模式**（列表页内）：
```vue
<!-- index.vue -->
<template>
  <!-- ... 列表 ... -->
  <bk-sideslider v-model:is-show="detailsState.isShow" title="详情">
    <Details :data="detailsState.data" />
  </bk-sideslider>
</template>
```

**独立路由模式**：
```vue
<!-- views/<module>/details-page.vue -->
<script setup>
const route = useRoute();
const data = ref({});
onMounted(async () => {
  data.value = await store.getDetail(route.params.id);
});
</script>
<template>
  <Details :data="data" />
</template>
```

### Step 7 — 注册路由和菜单

按 `fe-menu-route-architecture.mdc` 的 Checklist：
1. 独立路由模式需要注册路由（Sideslider 模式不需要额外路由）
2. `constants/menu-symbol.ts` 添加 Symbol
3. `views/<模块>/route-config.ts` 定义详情路由

### Step 8 — Lint 修复

```bash
hcmfe lint --fix
```

## 注意事项

- **字段分组**：`@Column` 的 `group` 属性决定面板分组，同一 group 的字段在同一个 `details-panel` 中
- **特殊字段**：`display-value` 无法满足时，直接用 `<template>` 覆盖 `<grid-item>` 的内容
- **扩展字段**：字段 ID 如 `extension.cloud_type`，在 `display-value` 的 `value` 中需要传整个 `data` 对象
- **路由跳转**：详情页中所有路由操作必须使用 `routerAction.redirect` / `routerAction.open`，禁止 `router.push`
- **模块内闭环**：字段类型定义、工具函数保持在模块内部
- **改造老模块**：新建文件迁移，老文件加 `@deprecated` 注释
