# 表单页架构

## 目录组织

```
views/<模块>/
├── index.vue                    # 列表页入口
├── children/
│   ├── list/                    # 列表子组件（page-list skill）
│   ├── details/                 # 详情子组件（page-detail skill）
│   └── form/                    # 【本 skill】表单组件
│       ├── form.vue             # 通用表单组件（核心）
│       ├── create.vue           # 新建表单包装
│       ├── edit.vue             # 编辑表单包装
│       ├── field-factory.ts     # 【多类型时】字段工厂
│       └── field-<type>.ts      # 【多类型时】各类型字段定义
```

## 分层设计

表单采用**三层结构**：

```
入口（index.vue）
├── Sideslider / 独立路由页面
│   ├── create.vue / edit.vue     # 业务层：样式、提示、差异化逻辑
│   │   └── form.vue              # 通用层：字段渲染、验证、数据管理
```

| 层级 | 文件 | 职责 |
|------|------|------|
| 业务层 | `create.vue` / `edit.vue` | padding、提示信息（edit）、透传 props/expose |
| 通用层 | `form.vue` | 字段模型、表单数据、验证、动态组件渲染 |

**为什么要分层？**
- `form.vue` 完全由字段模型驱动，新建和编辑共用同一套渲染逻辑
- `create.vue` / `edit.vue` 只处理差异化（如编辑时的提示 alert）
- 便于维护：字段增删改只需修改 `field-<type>.ts`

## 承载方式

### Sideslider 模式（列表页内弹窗）

```vue
<!-- index.vue -->
<template>
  <!-- ... 列表 ... -->
  <bk-sideslider v-model:is-show="createState.isShow" title="新建">
    <CreateForm ref="createFormRef" />
  </bk-sideslider>
  <bk-sideslider v-model:is-show="editState.isShow" title="编辑">
    <EditForm ref="editFormRef" :data="editState.data" />
  </bk-sideslider>
</template>
```

### 独立路由模式

独立路由页面使用**吸底布局**：表单内容区域可滚动，底部操作栏固定。

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

## 与列表页/详情页的关系

| 维度 | 列表页 | 详情页 | 表单页 |
|------|--------|--------|--------|
| 多类型 | inject currentVendor | inject currentVendor | inject currentVendor |
| Factory | condition-factory + column-factory | field-factory | field-factory |
| 模型方法 | `getProperties()` / `getPropertiesByGroup()` | `getPropertiesByGroup()` | `getProperties()` + `createInstance()` |
| 数据流 | 查询 → 展示 | 查询 → 展示 | 编辑 → 回填 / 新建 → 提交 |
