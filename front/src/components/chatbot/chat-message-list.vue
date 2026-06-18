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
import { useAccountSelect } from '@/hooks/chatbot/use-account-select';
import { useHostApply } from '@/hooks/chatbot/use-host-apply';
import {
  type AccountSelectInterruptMessage,
  type HostApplyPreorderMessage,
  type HostApplyRecommendMessage,
  type HostApplySubmitMessage,
  type HostApplySuborder,
} from '@/hooks/chatbot/types';
import routerAction from '@/router/utils/action';
import { MENU_SERVICE_HOST_APPLICATION } from '@/constants/menu-symbol';
import HitlInterruptCard from './hitl-interrupt-card.vue';
import AccountSelectCard from './account-select-card.vue';
import HostApplyRecommendCard from './host-apply-recommend-card.vue';
import HostApplyPreorderCard from './host-apply-preorder-card.vue';
import HostApplySubmitCard from './host-apply-submit-card.vue';

const { messages, isChatting, sendMessage, regenerate, resendEdited, stopGeneration } = useChatbotContext();

const selectedUserMessages = ref<Message[]>();
const { messageGroups } = useMessageGroup({
  messages: computed(() => messages.value),
  selectedUserMessages,
});

const messageStatus = computed(() => (isChatting.value ? MessageStatus.Streaming : MessageStatus.Complete));

const { isHitlInterruptMessage, getHitlContent, getHitlReadonlyState } = useHitl(messages);
const { isAccountSelectMessage, getAccountSelectContent, getAccountSelectReadonlyState } = useAccountSelect(messages);
const {
  isRecommendMessage,
  getRecommendContent,
  getRecommendReadonlyState,
  getSelectedIndex,
  isPreorderMessage,
  getPreorderContent,
  getPreorderReadonlyState,
  getPreorderReadonlySuborders,
  isSubmitMessage,
  getSubmitContent,
  getSubmitReadonlyState,
  getSubmitRows,
} = useHostApply(messages);

const handleAgentAction = async (tool: IToolBtn, msgs: Message[]) => {
  if (tool.id === 'rebuild') {
    await regenerate(msgs);
  }
};

// 账号选择确认：记录所选 account_id 到消息（供只读态/吸顶回显复用），并以 resumeValue 回写 agent
const handleAccountConfirm = (message: Message, accountId: string) => {
  const content = getAccountSelectContent(message);
  const option = content?.value.options.find((opt) => opt.account_id === accountId);
  (message as AccountSelectInterruptMessage).__selectedAccountId = accountId;
  const text = option ? `我选择云账号：${option.account_name}` : accountId;
  sendMessage(text, undefined, accountId);
};

// 模板 A 选择方案：记录所选下标（供只读态复用），以 resumeValue 回写所选 suborder（JSON 串）；
// agent 随后返回预提单（模板 B）。【假设】resume 承载形式待后端确认，见 api.md §6。
const handleSelectPlan = (message: Message, index: number) => {
  (message as HostApplyRecommendMessage).__selectedIndex = index;
  const suborder = getRecommendContent(message)?.value.recommendations[index]?.suborder;
  const resumeValue = suborder ? `帮我基于此方案进行拆单${JSON.stringify(suborder)}` : undefined;
  sendMessage('我选择该申领方案', undefined, resumeValue);
};

// 模板 B 确认方案：以 resumeValue 回传 suborders（JSON 串，含 C 弹窗修改）；content 仅用于对话气泡可读。
// 【假设】resume 承载形式待后端确认，见 api.md §6。
const handleConfirmPreorder = (message: Message, suborders: HostApplySuborder[], edited: boolean) => {
  (message as HostApplyPreorderMessage).__confirmedSuborders = suborders;
  sendMessage(edited ? '确认申领配置（已调整）' : '确认申领配置', undefined, JSON.stringify(suborders));
};

// 「添加到配置清单」：本迭代仅跳转主机申请页面 + 预留入口；回填配置清单留后续「回填」需求实现
const handleAddToList = (_payload: HostApplySuborder | HostApplySuborder[]) => {
  // TODO: 后续「回填」需求接入：携带 _payload 回填配置清单并联动打开 AI 助手
  routerAction.redirect({ name: MENU_SERVICE_HOST_APPLICATION });
};

// 模板 D 确认提交申请单：记录已提交（供只读态复用），以 resumeValue 回传整个 data（含 body_param/path_param）。
// 【假设】resume 承载形式待后端确认，见 api.md §6。
const handleSubmitConfirm = (message: Message) => {
  (message as HostApplySubmitMessage).__submitted = true;
  const data = getSubmitContent(message)?.value.data;
  sendMessage('确认提交', undefined, data ? JSON.stringify(data) : undefined);
};

const handleUserInputConfirm = async (message: Message, content: UserMessage['content']) => {
  await resendEdited(message, content);
};

const handleStopSending = () => {
  stopGeneration();
};
</script>

<template>
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
      <AccountSelectCard
        v-else-if="isAccountSelectMessage(message)"
        :content="getAccountSelectContent(message)"
        :readonly="getAccountSelectReadonlyState(message).readonly"
        :readonly-value="getAccountSelectReadonlyState(message).accountId"
        :on-confirm="(accountId) => handleAccountConfirm(message, accountId)"
      />
      <HostApplyRecommendCard
        v-else-if="isRecommendMessage(message)"
        :content="getRecommendContent(message)"
        :readonly="getRecommendReadonlyState(message).readonly"
        :selected-index="getSelectedIndex(message)"
        :on-select="(index) => handleSelectPlan(message, index)"
        :on-add-to-list="handleAddToList"
      />
      <HostApplyPreorderCard
        v-else-if="isPreorderMessage(message)"
        :content="getPreorderContent(message)"
        :readonly="getPreorderReadonlyState(message).readonly"
        :readonly-suborders="getPreorderReadonlySuborders(message)"
        :on-confirm="(suborders, edited) => handleConfirmPreorder(message, suborders, edited)"
        :on-add-to-list="handleAddToList"
      />
      <HostApplySubmitCard
        v-else-if="isSubmitMessage(message)"
        :rows="getSubmitRows(message)"
        :readonly="getSubmitReadonlyState(message).readonly"
        :on-confirm="() => handleSubmitConfirm(message)"
        :on-add-to-list="() => handleAddToList(getSubmitRows(message))"
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
