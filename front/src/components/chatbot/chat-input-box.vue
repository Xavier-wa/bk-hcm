<script setup lang="ts">
import { computed, shallowRef, useTemplateRef, watch } from 'vue';

import { ChatInput, MessageStatus, type TagSchema, type UserMessage } from '@blueking/chat-x';
import '@blueking/chat-x/dist/index.css';

import { useChatbotContext } from '@/hooks/chatbot/provide';

interface Props {
  // inputIndent 为编辑器首行左侧让位宽度（px），用于场景 chip 等绝对定位元素的两列布局；0 表示不让位
  inputIndent?: number;
  supportUpload?: boolean;
}

withDefaults(defineProps<Props>(), {
  inputIndent: 0,
  supportUpload: false,
});

const emit = defineEmits<{
  send: [text: string];
}>();

const { isChatting, currentSessionCode, stopGeneration } = useChatbotContext();

const inputValue = shallowRef<string | TagSchema>([[]]);
const chatInputRef = useTemplateRef<{ focus: () => void }>('chatInputRef');

const messageStatus = computed(() => (isChatting.value ? MessageStatus.Streaming : MessageStatus.Complete));

// 切换/新建会话时清空输入框，避免上个会话的草稿串到新会话
watch(currentSessionCode, () => {
  inputValue.value = [[]];
});

const handleSendMessage = async (content: UserMessage['content']) => {
  const text = typeof content === 'string' ? content : '';
  if (!text.trim()) return;
  inputValue.value = [[]];
  emit('send', text);
};

const handleStopSending = async () => {
  stopGeneration();
};

// setInput 注入默认提示词文本（场景 chip 由父级 #input-header 自渲染，v-model 不支持注入 tag 节点）
const setInput = (text: string) => {
  inputValue.value = [[{ type: 'text', text }]] as TagSchema;
};

const focus = () => {
  chatInputRef.value?.focus();
};

defineExpose({ setInput, focus });
</script>

<template>
  <div
    class="chat-input-box"
    :class="{ 'has-indent': inputIndent > 0 }"
    :style="{ '--input-indent': `${inputIndent}px` }"
  >
    <ChatInput
      ref="chatInputRef"
      v-model="inputValue"
      :message-status="messageStatus"
      :support-upload="supportUpload"
      :on-send-message="handleSendMessage"
      :on-stop-sending="handleStopSending"
    >
      <template #input-header>
        <slot name="input-header" />
      </template>
    </ChatInput>
  </div>
</template>

<style scoped lang="scss">
.chat-input-box {
  padding: 16px;

  // 有让位需求时，给编辑器整体让出左列（padding-left 对所有行生效，换行/空态光标都对齐右列）。
  // 左列宽度 = inputIndent 实测宽度 + 间距 16px，配合父级绝对定位的 chip 形成两列布局。
  &.has-indent :deep(.ai-slash-input) {
    padding-left: calc(var(--input-indent, 0px) + 16px);
  }
}
</style>
