<script setup lang="ts">
import {
  inject, computed, type Ref, ref, watch, useTemplateRef,
} from 'vue';
import { Form } from 'bkui-vue';
import { VendorEnum } from '@/common/constant';
import type { ModelPropertyForm } from '@/model/typings';

// ---- 1. 注入当前云厂商（多类型模块需要，单类型可删除）----
const currentVendor = inject<Ref<VendorEnum>>('currentVendor', ref(VendorEnum.TCLOUD));

// ---- 2. Props ----
const props = defineProps<{
  data?: Record<string, any>;
  isEdit?: boolean;
}>();

// ---- 3. 字段模型（多类型用 Factory，单类型直接 getModel）----
// import { FieldFactory } from './field-factory';
// const fieldModel = computed(() => FieldFactory.createModel(currentVendor.value));
// const properties = computed(() => fieldModel.value.getProperties<ModelPropertyForm>());
// const fields = computed(() => properties.value.filter((field) => !field.apiOnly));
const fields = ref<ModelPropertyForm[]>([]); // 占位

// ---- 4. 表单数据 ----
// const formData = ref(fieldModel.value.createInstance());
const formData = ref<Record<string, any>>({}); // 占位

const formRef = useTemplateRef<typeof Form>('formRef');

// ---- 5. 编辑时数据回填 ----
watch(
  () => props.data,
  (newVal) => {
    if (!newVal) return;
    // TODO: 逐个字段显式赋值
    // formData.value.id = newVal?.id;
    // formData.value.name = newVal?.name;
    // 需要格式转换的字段
    // formData.value.policy_document = newVal?.policy_document
    //   ? formatJSON(newVal.policy_document)
    //   : '';
  },
  { deep: true, immediate: true },
);

// ---- 6. 组件 Props 增强 ----
const getFormCompProps = (field: ModelPropertyForm) => {
  const compProps = field.meta?.display?.props || {};
  // 编辑时禁用不可修改的字段
  if (props.isEdit && ['account_id', 'name'].includes(field.id)) {
    compProps.disabled = true;
  }
  // TODO: 为特定字段注入额外 props
  // if (field.id === 'policy_library_id') {
  //   compProps.listGenerator = policyLibraryListGenerator.value;
  // }
  return compProps;
};

// ---- 7. 组件事件绑定 ----
const getFormCompEvents = (field: ModelPropertyForm) => {
  // TODO: 为特定字段绑定事件
  // if (field.id === 'policy_library_id') {
  //   return {
  //     change: (value: string, item: any) => {
  //       formData.value.policy_document = item?.policy_document
  //         ? formatJSON(item.policy_document)
  //         : '';
  //     },
  //   };
  // }
};

// ---- 8. 暴露方法 ----
defineExpose({
  getFormData: () => formData.value,
  validate: () => formRef.value?.validate(),
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
