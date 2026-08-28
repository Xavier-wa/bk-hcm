<script setup lang="ts">
import { Info } from 'bkui-vue/lib/icon';
// import { Popover } from 'bkui-vue';
import { getValueByKey } from '@/common/util';
import { computed } from 'vue';

interface Props {
  colData: { updated_info?: object; original_info?: object };
  field?: string;
  ticketType: string; // 单据类型
}
const props = withDefaults(defineProps<Props>(), {
  field: '',
  colData: () => ({}),
});
const specialType = ['transfer', 'delete', 'cancel'];

const purefieldKey = computed(() => {
  return props.field.replaceAll('updated_info.', '').replaceAll('original_info.', '');
});

const originalVal = computed(() => {
  return getValueByKey(props.colData?.original_info, purefieldKey.value);
});

const updatedVal = computed(() => {
  return getValueByKey(props.colData?.updated_info, purefieldKey.value);
});

const isSpecialType = computed(() => {
  return specialType.includes(props.ticketType);
});

const isChanged = computed(() => {
  if (isSpecialType.value) {
    return false;
  }
  // 无 original_info（调整单内新增 / 纯新增）或字段未变 → 不算变更
  return !!props.colData?.original_info && originalVal.value !== updatedVal.value;
});

const text = computed(() => {
  if (isSpecialType.value) {
    return originalVal.value || updatedVal.value;
  }
  return updatedVal.value;
});

const tipContent = computed(() => `修改前: ${originalVal.value ?? '--'}`);
</script>

<template>
  <!-- 仅字段真实变更时出 tip；NEW 行与无变更行不再出现「暂无修改前数据」 -->
  <div
    class="resource-plan-detail-cell"
    :class="{ 'is-changed': isChanged }"
    v-bk-tooltips="{
      content: tipContent,
      disabled: !isChanged,
    }"
  >
    <Info v-if="isChanged" class="resource-plan-detail-info resource-plan-detail-text" />
    <span :class="{ 'resource-plan-detail-text': isChanged }">{{ text || '--' }}</span>
  </div>
</template>

<style lang="scss" scoped>
.resource-plan-detail-cell {
  display: flex;
  align-items: center;

  &.is-changed {
    cursor: pointer;
  }
}

.resource-plan-detail-text {
  color: #e9a24c;
}

.resource-plan-detail-info {
  font-size: 18px;
  margin-right: 4px;
}
</style>
