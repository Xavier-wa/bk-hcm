<script setup lang="ts">
import { FixedColumn } from '@blueking/ediatable';

const props = withDefaults(
  defineProps<{
    removeable?: boolean;
    showCopy?: boolean;
    showAdd?: boolean;
    showRemove?: boolean;
  }>(),
  {
    removeable: true,
    showCopy: true,
    showAdd: true,
    showRemove: true,
  },
);

const emit = defineEmits<{
  (e: 'copy' | 'add' | 'remove'): void;
}>();

const handleRemove = () => {
  if (!props.removeable) return;
  emit('remove');
};
</script>

<template>
  <FixedColumn fixed="right">
    <div class="adjust-operation-column">
      <div v-if="showCopy" class="adjust-operation-btn" v-bk-tooltips="{ content: '复制' }" @click="emit('copy')">
        <i class="hcm-icon bkhcm-icon-copy"></i>
      </div>
      <div v-if="showAdd" class="adjust-operation-btn" v-bk-tooltips="{ content: '新增' }" @click="emit('add')">
        <i class="hcm-icon bkhcm-icon-plus-circle-shape"></i>
      </div>
      <div
        v-if="showRemove"
        class="adjust-operation-btn"
        :class="{ 'is-disabled': !removeable }"
        v-bk-tooltips="{ content: removeable ? '移除' : '至少保留一行' }"
        @click="handleRemove"
      >
        <i class="hcm-icon bkhcm-icon-minus-circle-shape"></i>
      </div>
    </div>
  </FixedColumn>
</template>

<style scoped lang="scss">
.adjust-operation-column {
  display: flex;
  align-items: center;
  height: 42px;
  padding: 0 16px;
}

.adjust-operation-btn {
  display: flex;
  color: #c4c6cc;
  cursor: pointer;
  font-size: 14px;

  &:hover {
    color: #979ba5;
  }

  &.is-disabled {
    color: #dcdee5;
    cursor: not-allowed;
  }

  & ~ .adjust-operation-btn {
    margin-left: 18px;
  }
}
</style>
