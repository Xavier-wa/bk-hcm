<script setup lang="ts">
import { computed, ref, useAttrs, watchEffect } from 'vue';
import { useConfigApplyStageStore, type IApplyStageItem } from '@/store/config/apply-stage';

defineOptions({ name: 'hcm-form-req-stage' });

const model = defineModel<string | string[]>();

const props = withDefaults(defineProps<{ multiple?: boolean; clearable?: boolean; disabled?: boolean }>(), {
  multiple: false,
});

const attrs = useAttrs();

const list = ref<IApplyStageItem[]>([]);

const localModel = computed({
  get() {
    if (props.multiple && model.value && !Array.isArray(model.value)) {
      return [model.value];
    }
    return model.value;
  },
  set(val) {
    model.value = val;
  },
});

const configApplyStageStore = useConfigApplyStageStore();

watchEffect(async () => {
  list.value = await configApplyStageStore.getApplyStage();
});
</script>

<template>
  <bk-select
    v-model="localModel"
    :list="list"
    :clearable="clearable"
    :multiple="multiple"
    :multiple-mode="multiple ? 'tag' : 'default'"
    :id-key="'stage'"
    :display-key="'description'"
    filterable
    v-bind="attrs"
  />
</template>
