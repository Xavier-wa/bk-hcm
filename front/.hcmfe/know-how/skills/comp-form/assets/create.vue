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
  <div class="xxx-create">
    <Form ref="formRef" :data="props.data" />
  </div>
</template>

<style lang="scss" scoped>
.xxx-create {
  padding: 24px 24px 0;
}
</style>
