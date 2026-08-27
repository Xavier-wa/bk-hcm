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

import { useChatbotContext, useChatbotMode } from '@/hooks/chatbot/provide';
import { touchCardActionProtect } from '@/hooks/chatbot/card-action-protect';
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
import { Message as bkMessage } from 'bkui-vue';

import routerAction from '@/router/utils/action';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import { useHostApplyBackfillStore } from '@/store/chatbot/host-apply-backfill';
import { GLOBAL_BIZS_KEY } from '@/common/constant';
import HitlInterruptCard from './hitl-interrupt-card.vue';
import AccountSelectCard from './account-select-card.vue';
import HostApplyRecommendCard from './host-apply-recommend-card.vue';
import HostApplyPreorderCard from './host-apply-preorder-card.vue';
import HostApplySubmitCard from './host-apply-submit-card.vue';

const {
  messages,
  isChatting,
  isCurrentSessionRemoteBusy,
  currentSessionCode,
  sendMessage,
  regenerate,
  resendEdited,
  stopGeneration,
} = useChatbotContext();

const { getBizsId } = useWhereAmI();
const chatbotMode = useChatbotMode();
const hostApplyBackfillStore = useHostApplyBackfillStore();

const selectedUserMessages = ref<Message[]>();
const { messageGroups } = useMessageGroup({
  messages: computed(() => messages.value),
  selectedUserMessages,
});

const messageStatus = computed(() => (isChatting.value ? MessageStatus.Streaming : MessageStatus.Complete));

// 组件内置工具按钮里只保留「复制」「重新生成」，其余能力（引用/分享/点赞/不满意/删除）暂未支持
const messageTools: IToolBtn[] = [
  { id: 'cite', hidden: true },
  { id: 'share', hidden: true },
];
const updateTools: IToolBtn[] = [
  { id: 'like', hidden: true },
  { id: 'unlike', hidden: true },
  { id: 'delete', hidden: true },
];

const { isHitlInterruptMessage, getHitlContent, getHitlReadonlyState } = useHitl(messages);
const { isAccountSelectMessage, getAccountSelectContent, getAccountSelectReadonlyState, selectedAccountId } =
  useAccountSelect(messages);
const {
  isRecommendMessage,
  getRecommendContent,
  getRecommendReadonlyState,
  getSelectedIndex,
  getInitialIndex,
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
  touchCardActionProtect();
  const text = option ? `我选择云账号：${option.account_name}` : accountId;
  sendMessage(text, undefined, accountId);
};

// 模板 A 选择方案：记录所选下标（供只读态复用），以 resumeValue 回写所选 suborder（JSON 串）；
// agent 随后返回预提单（模板 B）。
const handleSelectPlan = (message: Message, index: number) => {
  (message as HostApplyRecommendMessage).__selectedIndex = index;
  touchCardActionProtect();
  const suborder = getRecommendContent(message)?.value.recommendations[index]?.suborder;
  const resumeValue = suborder ? JSON.stringify(suborder) : undefined;
  sendMessage('我选择该申领方案', undefined, resumeValue);
};

// 模板 B 确认方案：以 resumeValue 回传 suborders（JSON 串，含 C 弹窗修改）；content 仅用于对话气泡可读。
// 【假设】resume 承载形式待后端确认，见 api.md §6。
const handleConfirmPreorder = (message: Message, suborders: HostApplySuborder[], edited: boolean) => {
  (message as HostApplyPreorderMessage).__confirmedSuborders = suborders;
  touchCardActionProtect();
  sendMessage(edited ? '确认申领配置（已调整）' : '确认申领配置', undefined, JSON.stringify(suborders));
};

// 「添加到配置清单」分两种形态：
// - 浮窗（floating）：chatbot 与申领页 ApplicationForm 同页，经 store 直接追加到当前页配置清单并提示，不开新标签页。
// - 全页（fullpage）：新标签页打开 applyCvm 申领页，方案规格经 URL query 透传（新标签页是独立实例无法用 store），
//   并携带 sessionCode 让申领页唤起对应会话的 AI 助手浮窗（见 service-apply/cvm/index.tsx）。
const handleAddToList = (payload: HostApplySuborder | HostApplySuborder[]) => {
  const suborders = Array.isArray(payload) ? payload : [payload];
  // 申领页账号选择是前置步骤，带上聊天中已选账号让申领页预选同一账号
  const accountId = selectedAccountId.value;
  if (chatbotMode === 'floating') {
    hostApplyBackfillStore.pushBackfill(suborders, accountId);
    bkMessage({ theme: 'success', message: '配置已添加到清单' });
    return;
  }
  // 仅 A 卡（选择方案步骤）传单方案：标记 selectPlan，让申领页打开关联会话后选中该方案（与 backfill 同一规格）
  const fromRecommend = !Array.isArray(payload);
  routerAction.open({
    name: 'applyCvm',
    query: {
      from: 'chatbot',
      [GLOBAL_BIZS_KEY]: getBizsId(),
      backfill: JSON.stringify(suborders),
      sessionCode: currentSessionCode.value ?? '',
      ...(accountId ? { accountId } : {}),
      ...(fromRecommend ? { selectPlan: '1' } : {}),
    },
  });
};

// 模板 D 确认提交申请单：记录已提交（供只读态复用），以 resumeValue 回传整个 data（含 body_param/path_param）。
// 【假设】resume 承载形式待后端确认，见 api.md §6。
const handleSubmitConfirm = (message: Message) => {
  (message as HostApplySubmitMessage).__submitted = true;
  touchCardActionProtect();
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
    :message-tools="messageTools"
    :update-tools="updateTools"
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
        :locked="isCurrentSessionRemoteBusy"
        :on-confirm="sendMessage"
      />
      <AccountSelectCard
        v-else-if="isAccountSelectMessage(message)"
        :content="getAccountSelectContent(message)"
        :readonly="getAccountSelectReadonlyState(message).readonly"
        :readonly-value="getAccountSelectReadonlyState(message).accountId"
        :locked="isCurrentSessionRemoteBusy"
        :on-confirm="(accountId) => handleAccountConfirm(message, accountId)"
      />
      <HostApplyRecommendCard
        v-else-if="isRecommendMessage(message)"
        :content="getRecommendContent(message)"
        :readonly="getRecommendReadonlyState(message).readonly"
        :selected-index="getSelectedIndex(message)"
        :initial-index="getInitialIndex(message)"
        :locked="isCurrentSessionRemoteBusy"
        :on-select="(index) => handleSelectPlan(message, index)"
        :on-add-to-list="handleAddToList"
      />
      <HostApplyPreorderCard
        v-else-if="isPreorderMessage(message)"
        :content="getPreorderContent(message)"
        :readonly="getPreorderReadonlyState(message).readonly"
        :readonly-suborders="getPreorderReadonlySuborders(message)"
        :locked="isCurrentSessionRemoteBusy"
        :on-confirm="(suborders, edited) => handleConfirmPreorder(message, suborders, edited)"
        :on-add-to-list="handleAddToList"
      />
      <HostApplySubmitCard
        v-else-if="isSubmitMessage(message)"
        :rows="getSubmitRows(message)"
        :readonly="getSubmitReadonlyState(message).readonly"
        :locked="isCurrentSessionRemoteBusy"
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

  // 用户消息的工具栏在组件内写死为 CONST_USER_MESSAGE_TOOLS（复制/引用/编辑/删除），未暴露配置项，
  // 且按钮根节点统一为 .ai-tool-btn 无法按 id 区分，只能按位置隐藏未支持的「引用」和「删除」。
  // 组件若调整该列表顺序，此处需同步修改。
  :deep(.ai-user-message-tools .message-tools:first-child) {
    > *:nth-child(2),
    > *:nth-child(4) {
      display: none;
    }
  }
}
</style>
