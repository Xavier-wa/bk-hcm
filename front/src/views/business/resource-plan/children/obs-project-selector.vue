<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue';
import { useResourcePlanStore } from '@/store';
import { PopoverPropTypes } from 'bkui-vue/lib/popover';

interface IProps {
  disabled?: boolean;
  clearable?: boolean;
  multiple?: boolean;
  showTips?: boolean;
  popoverOptions?: Partial<PopoverPropTypes>;
  showRollingServerProject?: boolean; // 滚服项目只有931业务可选。注意回填时要考虑是否要回填滚服项目（业务下切换业务的case）
  showShortRentalProject?: boolean; // 控制短租项目
}

const model = defineModel<string | string[]>();
const props = withDefaults(defineProps<IProps>(), {
  disabled: false,
  clearable: true,
  multiple: false,
  showTips: false,
  showRollingServerProject: true,
  showShortRentalProject: true,
});
const emit = defineEmits<{
  change: [value: string | string[]];
}>();

const resourcePlanStore = useResourcePlanStore();

const list = ref<string[]>([]);
const displayList = computed(() => {
  const filterText = [props.showRollingServerProject ? '' : '滚服项目', props.showShortRentalProject ? '' : '短租项目'];
  return list.value.filter((item: string) => !filterText.includes(item));
});
const loading = ref(false);
watchEffect(async () => {
  loading.value = true;
  try {
    const res = await resourcePlanStore.getObsProjects();
    list.value = res.data?.details ?? [];
  } finally {
    loading.value = false;
  }
});

const handleChange = (value: string | string[]) => {
  emit('change', value);
};

const TIP_TRIGGER_VALUES = ['改造复用', '轻量云徙'];
const isTipsShow = computed(() => {
  if (!props.showTips) return false;
  return props.multiple
    ? (model.value as string[]).some((val: string) => TIP_TRIGGER_VALUES.includes(val))
    : TIP_TRIGGER_VALUES.includes(model.value as string);
});
</script>

<template>
  <div class="obs-project-selector-container">
    <bk-select
      v-model="model"
      filterable
      :disabled="disabled"
      :multiple="multiple"
      :clearable="clearable"
      :popover-options="popoverOptions"
      @change="handleChange"
    >
      <bk-option v-for="item in displayList" :key="item" :id="item" :name="item" />
    </bk-select>
    <div v-if="isTipsShow" class="tips">
      <span class="attention">注意：</span>
      所选项目为特殊类型，如需使用该项目类型，请咨询ICR助手
    </div>
  </div>
</template>

<style lang="scss" scoped>
.obs-project-selector-container {
  position: relative;

  .tips {
    position: absolute;
    display: flex;
    align-items: center;
    margin-top: 2px;
    width: 100%;
    font-size: 12px;
    line-height: normal;

    .attention {
      color: #ea3636;
    }
  }
}
</style>
