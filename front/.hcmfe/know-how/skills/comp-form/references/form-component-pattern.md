# 通用表单组件模式（form.vue）

## 核心结构

```vue
<script setup>
const props = defineProps<{
  data?: Record<string, any>;   // 编辑时的初始数据
  isEdit?: boolean;             // 是否为编辑模式
}>();

// 1. 字段模型
const fieldModel = computed(() => FieldFactory.createModel(currentVendor.value));
const properties = computed(() => fieldModel.value.getProperties<ModelPropertyForm>());
const fields = computed(() => properties.value.filter((field) => !field.apiOnly));

// 2. 表单数据实例
const formData = ref(fieldModel.value.createInstance());

// 3. 编辑时数据回填
watch(() => props.data, (newVal) => {
  formData.value.id = newVal?.id;
  formData.value.name = newVal?.name;
  // ... 逐个字段回填
}, { deep: true, immediate: true });

// 4. 暴露方法供父组件调用
defineExpose({
  getFormData: () => formData.value,
  validate: () => formRef.value.validate(),
});
</script>

<template>
  <bk-form ref="formRef" :model="formData" form-type="vertical">
    <bk-form-item
      v-for="field in fields"
      :key="field.name"
      :label="field.name"
      :property="field.id"
      :required="field.required"
      :rules="field.rules"
    >
      <component
        :is="`hcm-form-${field.type}`"
        v-model="formData[field.id]"
        :option="field.option"
        :display="field.meta?.display"
        v-bind="getFormCompProps(field)"
        v-on="getFormCompEvents(field)"
      />
    </bk-form-item>
  </bk-form>
</template>
```

## 关键要点

### 1. 字段过滤

`apiOnly: true` 的字段不在表单中展示，但会包含在 `formData` 中用于 API 提交：

```typescript
const fields = computed(() => properties.value.filter((field) => !field.apiOnly));
```

### 2. 表单数据实例

`fieldModel.createInstance()` 根据 `@Column` 定义创建一个带初始值的数据对象：

```typescript
const formData = ref(fieldModel.value.createInstance());
// 结果: { id: '', name: '', type: '', ... }
```

### 3. 编辑回填

编辑时必须**逐个字段显式赋值**，不能直接用 `Object.assign`，因为：
- 后端返回的字段名可能与表单字段不完全一致
- 某些字段需要格式转换（如 JSON 格式化）
- 避免引入后端返回的额外字段

```typescript
watch(() => props.data, (newVal) => {
  formData.value.id = newVal?.id;
  formData.value.name = newVal?.name;
  formData.value.policy_document = newVal?.policy_document
    ? formatJSON(newVal.policy_document)
    : '';
}, { deep: true, immediate: true });
```

### 4. 动态组件

表单组件根据 `field.type` 动态渲染：

```vue
<component :is="`hcm-form-${field.type}`" />
```

支持的 `hcm-form-*` 组件由项目内部封装，与 `ModelPropertyType` 对应。

### 5. 组件 Props 增强

通过 `getFormCompProps` 为特定字段注入额外 props：

```typescript
const getFormCompProps = (field: ModelPropertyForm) => {
  const compProps = field.meta?.display?.props || {};
  // 编辑时禁用某些字段
  if (props.isEdit && (field.id === 'account_id' || field.id === 'name')) {
    compProps.disabled = true;
  }
  // 为下拉框注入数据生成器
  if (field.id === 'policy_library_id') {
    compProps.listGenerator = policyLibraryListGenerator.value;
  }
  return compProps;
};
```

### 6. 组件事件绑定

通过 `getFormCompEvents` 为特定字段绑定事件：

```typescript
const getFormCompEvents = (field: ModelPropertyForm) => {
  if (field.id === 'policy_library_id') {
    return {
      change: (value: string, item: any) => {
        formData.value.policy_document = item?.policy_document
          ? formatJSON(item.policy_document)
          : '';
      },
    };
  }
};
```

### 7. 暴露方法

父组件通过 `ref` 调用表单方法：

```typescript
defineExpose({
  getFormData: () => formData.value,           // 获取表单数据
  validate: () => formRef.value.validate(),    // 触发表单验证
});
```
