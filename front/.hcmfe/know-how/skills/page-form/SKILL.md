---
name: page-form
description: 创建 HCM 标准表单页（新建/编辑）。包含通用表单组件、字段模型驱动、验证规则、数据回填。当需要新增资源的新建或编辑表单时使用。
---

# 创建 HCM 表单页

## 使用时机

当需要创建资源的新建或编辑表单时使用此 Skill，典型场景：
- 列表页点击"新建"打开的创建表单
- 列表页点击"编辑"打开的编辑表单
- 详情页中的编辑入口

## 前置学习（按顺序阅读）

1. `./references/form-page-architecture.md` — 表单页整体架构、三层分层设计、承载方式
2. `./references/form-component-pattern.md` — 通用表单组件（form.vue）的核心结构与关键要点
3. `./references/form-field-pattern.md` — 字段定义模式（@Column 参数、验证规则、组件映射）
4. `./references/multi-type-pattern.md` — 多云/多资源字段 Factory 模式与简化决策

## 参考示例

| 文件 | 说明 |
|------|------|
| `./assets/form.vue` | 通用表单组件（字段模型驱动、动态组件、验证、数据回填） |
| `./assets/create.vue` | 新建表单包装（薄层，透传 props/expose） |
| `./assets/edit.vue` | 编辑表单包装（薄层，可添加提示 alert） |
| `./assets/field-factory.ts` | 【多类型时】字段工厂 |
| `./assets/field-tcloud.ts` | 【多类型时】腾讯云字段定义示例（apiOnly、required、rules、option、props） |

## 创建步骤

### Step 1 — 确认需求信息

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

### Step 2 — 判断模式（关键决策）

```
字段是否因云厂商/资源类型而不同？
├── 是 → Factory 模式
│   ├── 创建 form/field-factory.ts + field-<type>.ts
│   ├── 入口 inject currentVendor / currentResourceType
│   └── form.vue 中通过 FieldFactory.createModel(vendor) 获取字段
└── 否 → 简化模式
    ├── 直接写单个装饰器类（如 fields.ts）
    └── form.vue 中直接 getModel(FormFields)
```

### Step 3 — 创建目录结构

```bash
views/<模块>/
├── index.vue
├── children/
│   ├── list/                 # 列表区域（page-list skill）
│   ├── details/              # 详情区域（page-detail skill）
│   └── form/                 # 【本 skill】表单区域
│       ├── form.vue
│       ├── create.vue
│       ├── edit.vue
│       ├── field-factory.ts      # 【多类型时】
│       └── field-<type>.ts       # 【多类型时】
```

### Step 4 — 定义字段模型

1. **多类型**：复制 `./assets/field-factory.ts` + `./assets/field-tcloud.ts`
   - `apiOnly: true` 标记仅 API 使用的字段（如 `id`）
   - `required` 标记必填字段
   - `rules` 定义验证规则（validator / message / trigger）
   - `option` 定义枚举选项
   - `meta.display.props` 配置组件属性
2. **单类型**：直接写一个装饰器类，去掉 Factory

### Step 5 — 实现通用表单组件

复制 `./assets/form.vue`，替换以下内容：
1. `FieldFactory` 导入和调用（或替换为直接的 `getModel`）
2. `formData` 的类型定义
3. `watch` 中的数据回填逻辑（逐个字段显式赋值）
4. `getFormCompProps` 中的字段 props 增强（如编辑禁用、下拉框数据源）
5. `getFormCompEvents` 中的字段事件绑定（如联动填充）

### Step 6 — 实现新建/编辑包装

- **create.vue**：复制 `./assets/create.vue`，通常不需要修改
- **edit.vue**：复制 `./assets/edit.vue`，根据业务调整 `bk-alert` 的提示内容

### Step 7 — 接入入口

**Sideslider 模式**：
```vue
<!-- index.vue -->
<script setup>
const createFormRef = useTemplateRef('createFormRef');
const editFormRef = useTemplateRef('editFormRef');

const handleCreateSubmit = async () => {
  const valid = await createFormRef.value?.validate();
  if (!valid) return;
  const formData = createFormRef.value?.getFormData();
  await store.create(formData);
  createState.isShow = false;
  // 刷新列表
};
</script>

<template>
  <bk-sideslider v-model:is-show="createState.isShow" title="新建" width="640">
    <CreateForm ref="createFormRef" />
    <template #footer>
      <bk-button theme="primary" @click="handleCreateSubmit">提交</bk-button>
      <bk-button @click="createState.isShow = false">取消</bk-button>
    </template>
  </bk-sideslider>
</template>
```

**独立路由模式**：

使用吸底布局（表单内容可滚动，底部操作栏固定）：

```vue
<!-- views/<module>/create.vue -->
<script setup>
import routerAction from '@/router/utils/action';

const createFormRef = useTemplateRef('createFormRef');

const handleSubmit = async () => {
  const valid = await createFormRef.value?.validate();
  if (!valid) return;
  const formData = createFormRef.value?.getFormData();
  await store.create(formData);
  routerAction.back();
};
</script>

<template>
  <div class="xxx-create-page">
    <div class="form-content">
      <CreateForm ref="createFormRef" />
    </div>
    <div class="form-footer">
      <bk-button theme="primary" @click="handleSubmit">提交</bk-button>
      <bk-button @click="routerAction.back()">取消</bk-button>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.xxx-create-page {
  display: flex;
  flex-direction: column;
  height: 100%;

  .form-content {
    flex: 1;
    overflow-y: auto;
    padding: 24px 24px 0;
  }

  .form-footer {
    position: sticky;
    bottom: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 24px;
    background: #fff;
    border-top: 1px solid #dcdee5;
  }
}
</style>
```

### Step 8 — 注册路由和菜单

独立路由模式需要按 `fe-menu-route-architecture.mdc` 注册新建/编辑路由。Sideslider 模式不需要额外路由。

### Step 9 — Lint 修复

```bash
hcmfe lint --fix
```

## 注意事项

- **字段过滤**：`form.vue` 中通过 `fields.filter(f => !f.apiOnly)` 过滤掉仅 API 使用的字段
- **数据回填**：编辑时必须逐个字段显式赋值，禁止直接用 `Object.assign(formData.value, props.data)`
- **表单数据实例**：使用 `fieldModel.createInstance()` 创建带初始值的表单数据对象
- **验证触发**：`trigger: 'blur'` 用于输入框失去焦点时验证，`trigger: 'change'` 用于选择类组件
- **编辑禁用**：通过 `getFormCompProps` 根据 `isEdit` 和字段 ID 设置 `disabled`
- **字段联动**：通过 `getFormCompEvents` 绑定 `change` 事件，在回调中修改其它字段值
- **模块内闭环**：字段定义、工具函数保持在模块内部
