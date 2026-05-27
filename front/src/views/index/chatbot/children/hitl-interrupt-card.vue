<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { Button, Input } from 'bkui-vue';

import { type HitlInterruptValue } from '@/hooks/chatbot/types';

interface Props {
  content: HitlInterruptValue;
  onConfirm: (value: string) => void;
  readonly?: boolean;
  readonlyValue?: string;
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false,
  readonlyValue: '',
});

const selectedOption = ref('');
const customInput = ref('');
const isConfirmed = ref(false);

const readonlyAnswer = computed(() => props.readonlyValue.trim());
const isReadonlyMode = computed(() => props.readonly || Boolean(readonlyAnswer.value));

watch(
  () => [isReadonlyMode.value, readonlyAnswer.value, props.content.value.options] as const,
  ([readonlyMode, answer, options]) => {
    if (!readonlyMode) return;
    isConfirmed.value = true;
    if (!answer) {
      selectedOption.value = '';
      customInput.value = '';
      return;
    }
    if (options.includes(answer)) {
      selectedOption.value = answer;
      customInput.value = '';
      return;
    }
    selectedOption.value = '';
    customInput.value = '';
  },
  { immediate: true },
);

const showCustomInput = computed(() => !isReadonlyMode.value);
const showReadonlyEmpty = computed(() => isReadonlyMode.value && !selectedOption.value && !customInput.value);

const isButtonDisabled = computed(() => {
  if (isReadonlyMode.value || isConfirmed.value) return true;
  return !selectedOption.value && !customInput.value.trim();
});

const handleSelectOption = (option: string) => {
  if (isReadonlyMode.value || isConfirmed.value) return;
  selectedOption.value = option;
  customInput.value = '';
};

const handleInputChange = (value: string) => {
  if (isReadonlyMode.value || isConfirmed.value) return;
  customInput.value = value;
  selectedOption.value = '';
};

const handleConfirm = () => {
  if (isReadonlyMode.value || isConfirmed.value) return;
  const value = selectedOption.value || customInput.value.trim();
  if (!value) return;
  isConfirmed.value = true;
  props.onConfirm(value);
};
</script>

<template>
  <div class="hitl-interrupt-card">
    <div class="hitl-question">{{ content.value.question }}</div>
    <div class="hitl-options">
      <div
        v-for="option in content.value.options"
        :key="option"
        class="hitl-option"
        :class="{ 'is-selected': selectedOption === option, 'is-disabled': isConfirmed || isReadonlyMode }"
        @click="handleSelectOption(option)"
      >
        <span class="hitl-option-radio">
          <span v-if="selectedOption === option" class="hitl-option-radio-inner" />
        </span>
        <span class="hitl-option-text">{{ option }}</span>
      </div>
    </div>
    <div v-if="showCustomInput" class="hitl-custom">
      <div class="hitl-custom-label">其他选项：</div>
      <Input
        :model-value="customInput"
        :disabled="isConfirmed || isReadonlyMode"
        :readonly="isReadonlyMode"
        :placeholder="isReadonlyMode ? '未填写内容' : '请输入自定义选项内容...'"
        @update:model-value="handleInputChange"
      />
    </div>
    <div v-if="showReadonlyEmpty" class="hitl-readonly-empty">未选择或输入自定义</div>
    <div v-if="!isReadonlyMode" class="hitl-actions">
      <Button theme="primary" :disabled="isButtonDisabled" @click="handleConfirm">确认</Button>
    </div>
  </div>
</template>

<style scoped lang="scss">
.hitl-interrupt-card {
  padding: 16px;
  background: #fff;
  border: 1px solid #dcdee5;
  border-radius: 8px;
}

.hitl-question {
  margin-bottom: 12px;
  font-size: 14px;
  color: #313238;
}

.hitl-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.hitl-option {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  cursor: pointer;
  background: #f5f7fa;
  border: 1px solid #dcdee5;
  border-radius: 4px;
  transition: background 0.2s, border-color 0.2s;

  &:hover:not(.is-disabled) {
    background: #e1ecff;
    border-color: #3a84ff;
  }

  &.is-selected {
    position: relative;
    background: #e1ecff;
    border-color: #3a84ff;

    &::before {
      position: absolute;
      top: 0;
      bottom: 0;
      left: 0;
      width: 2px;
      content: '';
      background: #3a84ff;
      border-radius: 2px 0 0 2px;
    }
  }

  &.is-disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }
}

.hitl-option-radio {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  margin-right: 8px;
  border: 1px solid #979ba5;
  border-radius: 50%;

  .hitl-option.is-selected & {
    border-color: #3a84ff;
  }
}

.hitl-option-radio-inner {
  width: 8px;
  height: 8px;
  background: #3a84ff;
  border-radius: 50%;
}

.hitl-option-text {
  font-size: 13px;
  color: #63656e;
}

.hitl-custom {
  margin-bottom: 16px;

  .hitl-custom-label {
    margin-bottom: 8px;
    font-size: 13px;
    color: #979ba5;
  }
}

.hitl-readonly-empty {
  font-size: 12px;
  color: #979ba5;
}

.hitl-actions {
  display: flex;
  justify-content: flex-end;
}
</style>
