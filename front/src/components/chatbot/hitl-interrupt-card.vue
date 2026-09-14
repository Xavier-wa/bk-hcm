<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { Button, Input } from 'bkui-vue';
import { Warn } from 'bkui-vue/lib/icon';

import { type HitlInterruptValue } from '@/hooks/chatbot/types';
import { useFollowScrollOnMount } from '@/hooks/chatbot/use-follow-scroll';

interface Props {
  content: HitlInterruptValue;
  onConfirm: (value: string) => void;
  readonly?: boolean;
  readonlyValue?: string;
  // F-004：他处正在操作该会话，锁定选项与确认（不进只读，仅禁用 + 顶部提示）
  locked?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false,
  readonlyValue: '',
  locked: false,
});

useFollowScrollOnMount();

const selectedOption = ref('');
const customInput = ref('');
const isConfirmed = ref(false);

const readonlyAnswer = computed(() => props.readonlyValue.trim());
const isReadonlyMode = computed(() => props.readonly || Boolean(readonlyAnswer.value));

// options 为可选项，缺字段 / null 时归一为空数组，此时澄清退化为自由文本作答
const options = computed(() => props.content.value.options ?? []);
const hasOptions = computed(() => options.value.length > 0);

watch(
  () => [isReadonlyMode.value, readonlyAnswer.value, options.value] as const,
  ([readonlyMode, answer, currentOptions]) => {
    if (!readonlyMode) return;
    isConfirmed.value = true;
    if (!answer) {
      selectedOption.value = '';
      customInput.value = '';
      return;
    }
    if (currentOptions.includes(answer)) {
      selectedOption.value = answer;
      customInput.value = '';
      return;
    }
    selectedOption.value = '';
    // 无选项时答案只能来自自由输入，只读态回显原文；有选项却未命中仍留空，避免误判为自定义输入
    customInput.value = currentOptions.length === 0 ? answer : '';
  },
  { immediate: true },
);

// 只读态下仍需展示已回显的自由文本答案，否则历史回放只剩一句「未选择或输入自定义」
const showCustomInput = computed(() => !isReadonlyMode.value || Boolean(customInput.value));
// 无选项时输入框是唯一作答区，不再叫「其他选项」
const customPlaceholder = computed(() => {
  if (isReadonlyMode.value) return '未填写内容';
  return hasOptions.value ? '请输入自定义选项内容...' : '请输入您的回复...';
});
const showReadonlyEmpty = computed(() => isReadonlyMode.value && !selectedOption.value && !customInput.value);
// F-004 锁定横幅：仅在仍可交互（未只读）且被他处锁定时展示
const showLockedBanner = computed(() => props.locked && !isReadonlyMode.value);
// 交互是否被禁用：只读 / 已确认 / 他处锁定 任一成立
const isInteractionDisabled = computed(() => isReadonlyMode.value || isConfirmed.value || props.locked);

const isButtonDisabled = computed(() => {
  if (isInteractionDisabled.value) return true;
  return !selectedOption.value && !customInput.value.trim();
});

const handleSelectOption = (option: string) => {
  if (isInteractionDisabled.value) return;
  selectedOption.value = option;
  customInput.value = '';
};

const handleInputChange = (value: string) => {
  if (isInteractionDisabled.value) return;
  customInput.value = value;
  selectedOption.value = '';
};

const handleConfirm = () => {
  if (isInteractionDisabled.value) return;
  const value = selectedOption.value || customInput.value.trim();
  if (!value) return;
  isConfirmed.value = true;
  props.onConfirm(value);
};
</script>

<template>
  <div class="hitl-interrupt-card ai-turn-card">
    <div v-if="showLockedBanner" class="hitl-locked-banner">
      <Warn class="hitl-locked-icon" />
      <span class="hitl-locked-text">该会话正在其它页面操作中，最新状态稍后自动同步…</span>
    </div>
    <div class="hitl-question">{{ content.value.question }}</div>
    <div v-if="hasOptions" class="hitl-options">
      <div
        v-for="option in options"
        :key="option"
        class="hitl-option"
        :class="{ 'is-selected': selectedOption === option, 'is-disabled': isInteractionDisabled }"
        @click="handleSelectOption(option)"
      >
        <span class="hitl-option-radio">
          <span v-if="selectedOption === option" class="hitl-option-radio-inner" />
        </span>
        <span class="hitl-option-text">{{ option }}</span>
      </div>
    </div>
    <div v-if="showCustomInput" class="hitl-custom">
      <div v-if="hasOptions" class="hitl-custom-label">其他选项：</div>
      <Input
        :model-value="customInput"
        :disabled="isInteractionDisabled"
        :readonly="isReadonlyMode"
        :placeholder="customPlaceholder"
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

.hitl-locked-banner {
  display: flex;
  align-items: flex-start;
  padding: 8px 12px;
  margin-bottom: 12px;
  background: #fff4e2;
  border-radius: 4px;

  .hitl-locked-icon {
    flex-shrink: 0;
    margin-top: 2px;
    margin-right: 8px;
    font-size: 16px;
    color: #ff9c01;
  }

  .hitl-locked-text {
    font-size: 12px;
    line-height: 20px;
    color: #63656e;
  }
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
