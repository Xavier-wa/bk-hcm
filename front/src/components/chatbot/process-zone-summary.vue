<script setup lang="ts">
import { AngleDown, AngleRight } from 'bkui-vue/lib/icon';

defineProps<{
  text: string;
  expanded: boolean;
}>();

const emit = defineEmits<{
  toggle: [];
}>();
</script>

<template>
  <div class="process-zone-summary" :class="{ 'is-expanded': expanded }" @click="emit('toggle')">
    <span class="process-zone-summary-arrow" aria-hidden="true">
      <AngleDown v-if="expanded" />
      <AngleRight v-else />
    </span>
    <span class="process-zone-summary-text">{{ text }}</span>
  </div>
</template>

<style scoped lang="scss">
.process-zone-summary {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  user-select: none;
  line-height: 20px;
}

// bkui Angle* 画在 1024 viewBox 中心，左右各有大块空白；裁掉后箭头左缘与正文对齐。
// 盒宽取两个字形里较宽的 AngleDown（448/1024 × 20px ≈ 9px），展开/收起切换时正文不会左右跳动。
.process-zone-summary-arrow {
  display: block;
  flex-shrink: 0;
  width: 9px;
  height: 20px;
  overflow: hidden;
  font-size: 20px;
  line-height: 0;
  color: #63656e;

  :deep(svg) {
    display: block;

    // AngleRight path 左缘 376/1024
    margin-left: calc(-1em * 376 / 1024);
  }
}

.process-zone-summary.is-expanded .process-zone-summary-arrow :deep(svg) {
  // AngleDown path 左缘 288/1024
  margin-left: calc(-1em * 288 / 1024);
}

.process-zone-summary-text {
  font-size: 12px;
  color: #63656e;
}
</style>
