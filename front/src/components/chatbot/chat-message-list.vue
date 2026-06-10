<script setup lang="ts">
import { computed, ref } from 'vue';

import {
  MessageContainer,
  MessageStatus,
  useMessageGroup,
  type IToolBtn,
  type Message,
  type UserMessage,
} from '@blueking/chat-x';
import '@blueking/chat-x/dist/index.css';

import { useChatbotContext } from '@/hooks/chatbot/provide';
import { useHitl } from '@/hooks/chatbot/use-hitl';
import HitlInterruptCard from './hitl-interrupt-card.vue';

const { messages, isChatting, sendMessage, regenerate, resendEdited, stopGeneration } = useChatbotContext();

const selectedUserMessages = ref<Message[]>();
const { messageGroups } = useMessageGroup({
  messages: computed(() => messages.value),
  selectedUserMessages,
});

const messageStatus = computed(() => (isChatting.value ? MessageStatus.Streaming : MessageStatus.Complete));

const { isHitlInterruptMessage, getHitlContent, getHitlReadonlyState } = useHitl(messages);

const handleAgentAction = async (tool: IToolBtn, msgs: Message[]) => {
  if (tool.id === 'rebuild') {
    await regenerate(msgs);
  }
};

const handleUserInputConfirm = async (message: Message, content: UserMessage['content']) => {
  await resendEdited(message, content);
};

const handleStopSending = () => {
  stopGeneration();
};
</script>

<template>
  <!-- messages 在新版组件中未被消费，仅因类型定义为必填而保留 -->
  <MessageContainer
    class="chat-message-list"
    :messages="messages"
    :message-groups="messageGroups"
    :message-status="messageStatus"
    :on-agent-action="handleAgentAction"
    :on-user-input-confirm="handleUserInputConfirm"
    @stop-streaming="handleStopSending"
  >
    <template #default="{ message }">
      <HitlInterruptCard
        v-if="isHitlInterruptMessage(message)"
        :content="getHitlContent(message)"
        :readonly="getHitlReadonlyState(message).readonly"
        :readonly-value="getHitlReadonlyState(message).value"
        :on-confirm="sendMessage"
      />
    </template>
  </MessageContainer>
</template>

<style scoped lang="scss">
.chat-message-list {
  :deep(.message-group) {
    max-width: 1000px;
    padding-right: 16px;
    margin-right: auto;
    margin-left: auto;
  }
}

:deep(.message-wrapper) {
  .message-tools-hover {
    opacity: 0;
    transition: opacity 0.2s ease-in-out;
  }

  &:hover .message-tools-hover {
    opacity: 1;
  }
}

:deep(.message-tools-container:not(.ai-user-message-tools)) {
  .message-tools > *:has(.ai-cite-icon),
  .message-tools > *:has(.ai-share-icon),
  .ai-divider,
  .message-tools:last-child {
    display: none;
  }
}

:deep(.ai-user-message-tools) {
  .message-tools > *:has(.ai-cite-icon),
  .message-tools > *:has(.ai-delete-icon) {
    display: none;
  }
}
</style>
