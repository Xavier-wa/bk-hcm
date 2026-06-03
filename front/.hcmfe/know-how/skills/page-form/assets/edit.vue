<script setup lang="ts">
import { useTemplateRef } from 'vue';
import Form from './form.vue';

const props = defineProps<{
  data?: Record<string, any>;
}>();

const formRef = useTemplateRef<typeof Form>('formRef');

defineExpose({
  validate: () => formRef.value?.validate(),
  getFormData: () => formRef.value?.getFormData(),
});
</script>

<template>
  <div class="xxx-edit">
    <!-- TODO: 根据业务需要添加提示信息 -->
    <bk-alert theme="info" closable class="alert-info">
      <template #title>
        <div>修改将影响关联资源，请谨慎操作。</div>
      </template>
    </bk-alert>
    <Form ref="formRef" :data="props.data" :is-edit="true" />
  </div>
</template>

<style lang="scss" scoped>
.xxx-edit {
  padding: 24px 24px 0;
}

.alert-info {
  margin-bottom: 16px;
}
</style>
