<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { Button } from 'bkui-vue';
import { Done } from 'bkui-vue/lib/icon';

import { type AccountSelectInterruptValue, type AccountSelectOption } from '@/hooks/chatbot/types';
import { getVendorDisplay } from '@/hooks/chatbot/vendor-display';
import CustomMessageCard from './custom-message-card.vue';

interface Props {
  content: AccountSelectInterruptValue;
  onConfirm: (accountId: string) => void;
  readonly?: boolean;
  readonlyValue?: string;
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false,
  readonlyValue: '',
});

const selectedId = ref('');
const isConfirmed = ref(false);

const options = computed(() => props.content.value.options ?? []);

watch(
  () => [props.readonly, props.readonlyValue] as const,
  ([readonly, value]) => {
    if (!readonly) return;
    isConfirmed.value = true;
    selectedId.value = value || '';
  },
  { immediate: true },
);

// 已确认后整体只读
const isReadonly = computed(() => props.readonly || isConfirmed.value);
const selectedOption = computed<AccountSelectOption | null>(
  () => options.value.find((opt) => opt.account_id === selectedId.value) ?? null,
);
// 已知选中账号 → 外壳可收起为摘要行；只读但未知选中（历史兜底失败）→ 外壳直接只读展示选项
const hasKnownSelection = computed(() => isReadonly.value && !!selectedOption.value);
const isConfirmDisabled = computed(() => !selectedId.value);

const summaryText = computed(() => {
  const opt = selectedOption.value;
  if (!opt) return '';
  return `您已选择【${getVendorDisplay(opt.vendor).name}】- ${opt.account_name}`;
});

const handleSelect = (opt: AccountSelectOption) => {
  if (isReadonly.value || !opt.enabled) return;
  selectedId.value = opt.account_id;
};

const handleConfirm = () => {
  if (isReadonly.value || !selectedId.value) return;
  isConfirmed.value = true;
  props.onConfirm(selectedId.value);
};
</script>

<template>
  <CustomMessageCard
    :readonly="isReadonly"
    :collapsible="hasKnownSelection"
    :summary-text="summaryText"
    actions-align="right"
  >
    <!-- 选择态 / 只读回显态：厂商卡 -->
    <div class="as-options" :class="{ 'is-readonly': isReadonly }">
      <div
        v-for="opt in options"
        :key="opt.account_id"
        v-bk-tooltips="{ content: opt.reason, disabled: opt.enabled || !opt.reason }"
        class="as-option"
        :class="{
          'is-selected': selectedId === opt.account_id,
          'is-disabled': !opt.enabled,
          'is-readonly': isReadonly,
        }"
        @click="handleSelect(opt)"
      >
        <div class="as-option-icon-box">
          <img
            class="as-option-icon"
            :src="getVendorDisplay(opt.vendor).icon"
            :alt="getVendorDisplay(opt.vendor).name"
          />
        </div>
        <div class="as-option-info">
          <span class="as-option-vendor">{{ getVendorDisplay(opt.vendor).name }}</span>
          <span class="as-option-account" :title="opt.account_name">{{ opt.account_name }}</span>
        </div>
        <div v-if="selectedId === opt.account_id" class="as-option-corner">
          <Done class="as-corner-check" />
        </div>
      </div>
    </div>

    <template v-if="!isReadonly" #actions>
      <Button theme="primary" :disabled="isConfirmDisabled" @click="handleConfirm">确认</Button>
    </template>
  </CustomMessageCard>
</template>

<style scoped lang="scss">
// 等宽列自动填充：厂商卡从左往右排列，换行后末行空列留空 → 整体靠左对齐、卡片等宽不拉伸
.as-options {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
}

.as-option {
  position: relative;
  display: flex;
  align-items: center;
  min-width: 0;
  padding: 12px;
  overflow: hidden;
  cursor: pointer;
  background: #f5f7fa;
  border: 1px solid #dcdee5;
  border-radius: 6px;
  transition: background 0.2s, border-color 0.2s;

  &:hover:not(.is-disabled, .is-readonly) {
    background: #e1ecff;
    border-color: #3a84ff;
  }

  &.is-selected {
    background: #e1ecff;
    border-color: #3a84ff;
  }

  &.is-disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  &.is-readonly {
    cursor: default;
  }

  .as-option-icon-box {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    margin-right: 10px;
    background: #fff;
    border-radius: 8px;
  }

  .as-option-icon {
    width: 24px;
    height: 24px;
  }

  .as-option-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .as-option-vendor {
    font-size: 14px;
    font-weight: 700;
    color: #313238;
  }

  .as-option-account {
    overflow: hidden;
    font-size: 12px;
    color: #979ba5;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  // 选中态右上角三角角标 + 白色对勾
  .as-option-corner {
    position: absolute;
    top: 0;
    right: 0;
    width: 36px;
    height: 36px;
    background: #3a84ff;
    clip-path: polygon(100% 0, 0 0, 100% 100%);

    .as-corner-check {
      position: absolute;
      top: 0;
      right: 0;
      font-size: 22px;
      color: #fff;
    }
  }
}

// 窄容器（浮窗）：厂商卡纵向整行铺满，并整体收紧图标与间距使其更紧凑。
// 容器基准为外壳 .custom-msg-card（祖先），@container 命中其后代选项即可。
@container (max-width: 380px) {
  .as-options {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .as-option {
    padding: 8px;

    .as-option-icon-box {
      width: 36px;
      height: 36px;
      margin-right: 8px;
    }

    .as-option-icon {
      width: 24px;
      height: 24px;
    }

    .as-option-corner {
      width: 28px;
      height: 28px;

      .as-corner-check {
        font-size: 16px;
      }
    }
  }
}
</style>
