<script setup lang="ts">
import { computed, ref } from 'vue';

import {
  MessageContainer,
  MessageRole,
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
import { useAgentFeedback } from '@/hooks/chatbot/use-agent-feedback';
import {
  isProcessSummaryMessage,
  isProcessThinkingMessage,
  isProcessToolMessage,
  useProcessZone,
} from '@/hooks/chatbot/use-process-zone';
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
import ProcessZoneSummary from './process-zone-summary.vue';
import ProcessThinking from './process-thinking.vue';
import ProcessToolRow from './process-tool-row.vue';

const {
  messages,
  isChatting,
  stayProcessExpanded,
  isCurrentSessionRemoteBusy,
  currentSessionCode,
  currentSession,
  sendMessage,
  regenerate,
  resendEdited,
  stopGeneration,
} = useChatbotContext();

const { getBizsId } = useWhereAmI();
const chatbotMode = useChatbotMode();
const hostApplyBackfillStore = useHostApplyBackfillStore();

const selectedUserMessages = ref<Message[]>();
const { displayMessages, toggleTurn, isSummaryExpanded, toggleThinking, toggleToolRow } = useProcessZone(
  messages,
  isChatting,
  stayProcessExpanded,
);
const { messageGroups } = useMessageGroup({
  messages: displayMessages,
  selectedUserMessages,
});

const messageStatus = computed(() => (isChatting.value ? MessageStatus.Streaming : MessageStatus.Complete));

// 引用/分享/删除仍未支持；点赞/点踩走 chat-x 内置 MessageUserFeedback
const messageTools: IToolBtn[] = [
  { id: 'cite', hidden: true },
  { id: 'share', hidden: true },
];
const updateTools: IToolBtn[] = [{ id: 'delete', hidden: true }];

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

const { handleLikeUnlikeAction, handleLikeUnlikeFeedback, handleFeedbackCancelClick } = useAgentFeedback({
  getBkBizId: getBizsId,
  getSessionId: () => currentSession.value?.sessionId ?? '',
  messageGroups,
});

const handleAgentAction = async (tool: IToolBtn, msgs: Message[]) => {
  if (tool.id === 'rebuild') {
    await regenerate(msgs);
    return;
  }
  if (tool.id === 'like' || tool.id === 'unlike') {
    return handleLikeUnlikeAction(tool, msgs);
  }
};

const handleAgentFeedback = (tool: IToolBtn, msgs: Message[], reasonList: string[], otherReason: string) => {
  handleLikeUnlikeFeedback(tool, msgs, reasonList, otherReason);
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

const summaryTurnKey = (message: Message) => (isProcessSummaryMessage(message) ? message.__turnKey : '');

// 空助手消息：模型开了 TEXT_MESSAGE 却没产出正文，留下一条 content 为空的消息（见 use-event 的 TextMessageStart）。
// 它渲染出来高 0，但 flex 仍在它两侧各留一份 gap，视觉上该处间距翻倍。
// 这里渲染空壳把 chat-x 的默认渲染顶掉，交给样式连外层条目一起收起；正文一到就自然回到默认渲染
const isBlankMessage = (message: Message) =>
  message.role === MessageRole.Assistant &&
  !(message as { __type?: string }).__type &&
  !(message as { toolCalls?: unknown[] }).toolCalls?.length &&
  typeof message.content === 'string' &&
  !message.content.trim();
</script>

<template>
  <!-- 取消评价的 click.capture 必须挂在 MessageContainer 上、不能再套一层 div：chat-x 的
       .ai-message-container 靠 height: 100% 撑满滚动父容器（全页 .chatbot-chat-messages / 浮窗 .aa-messages），
       中间多一层无样式 div 会让百分比高度回落成 auto，滚动就从 chat-x 容器转移到外层父容器。
       此时容器内 position: sticky 的「停止生成 / 返回底部」条只能贴在容器自身底边（已在可视区外），
       流式贴底与过程区收起时被外层 overflow 裁掉一截；chat-x 的 jumpToBottom（改内层 scrollTop）也一并失效。 -->
  <MessageContainer
    class="chat-message-list"
    :messages="displayMessages"
    :message-groups="messageGroups"
    :message-status="messageStatus"
    :message-tools="messageTools"
    :update-tools="updateTools"
    :on-agent-action="handleAgentAction"
    :on-agent-feedback="handleAgentFeedback"
    :on-user-input-confirm="handleUserInputConfirm"
    @click.capture="handleFeedbackCancelClick"
    @stop-streaming="handleStopSending"
  >
    <template #default="{ message }">
      <ProcessZoneSummary
        v-if="isProcessSummaryMessage(message)"
        :text="String(message.content ?? '')"
        :expanded="isSummaryExpanded(summaryTurnKey(message))"
        @toggle="toggleTurn(summaryTurnKey(message))"
      />
      <ProcessToolRow
        v-else-if="isProcessToolMessage(message)"
        :message="message"
        @toggle-row="toggleToolRow(message, $event)"
      />
      <ProcessThinking
        v-else-if="isProcessThinkingMessage(message)"
        :message="message"
        @toggle="toggleThinking(message)"
      />
      <HitlInterruptCard
        v-else-if="isHitlInterruptMessage(message)"
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
      <div v-else-if="isBlankMessage(message)" class="blank-message" />
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

  // 用户气泡对齐稿面 node 2060:20561（chat-x 默认是 #e1ecff / 8px 内边距 / 4px 圆角，四项都不一样）。
  // 右上直角、其余 16px 是稿面给的形状（气泡尖角朝向右侧发言人）。
  // 字号写死 14/22 而不是调 --ai-font-size：后者是 chat-x 的整档尺寸变量（small 12/20、normal 14/24），
  // 动它会连 AI 正文与图标一起放大，超出本次范围。
  // 气泡视觉全在 .ai-user-message-content 上：其内的 .ai-text-content 已被 chat-x 重置成透明无内边距
  :deep(.ai-user-message-content) {
    // 多个文本项纵向堆叠（稿面 flex-col）。chat-x 原值是 row，content 为数组时会把各段并排画
    flex-direction: column;
    padding: 12px 24px;
    font-size: 14px;
    line-height: 22px;
    word-break: normal;
    background-color: #cddffe;
    border-radius: 16px 0 16px 16px;
    box-shadow: 0 1px 1px 0 rgb(0 0 0 / 5%);

    // 稿面为 break-word：优先整词换行，超长不可断 token 仍会断，不会溢出气泡
    overflow-wrap: break-word;
  }

  // 多行态（稿面 node 2229:17681）每行一个 p 且 mb-0：行间只有 22px 行高，没有额外间距。
  // chat-x 把整条 content 渲染成一个纯文本节点（text-content 组件即 toDisplayString，不走 markdown），
  // 全链路又没有 white-space 声明，用户 Shift + Enter 打的 \n 会被 HTML 折叠成空格、多行糊成一段。
  // 这里的文本节点是纯数据、不含模板缩进，开 pre-wrap 不会像工具行详情那样凭空多出空行
  :deep(.ai-user-message-content .ai-text-content) {
    white-space: pre-wrap;
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

  // 助手组工具栏：chat-x 0.0.50 用内联 visibility 跟 isHover（mouseenter/leave .message-group）。
  // 稿面 2060:8335 要求复制/赞/踩/重答常驻。只覆盖组级工具栏（.message-group-messages 的直接子节点），
  // 用户气泡仍走 .ai-user-message-tools 的 hover，避免复制/编辑入口一直占位。
  // 库内 mouseleave 会避开 .ai-user-feedback，常驻后这条路径不再承担显隐，反馈弹层本身不受影响。
  :deep(.message-group-messages > .ai-message-tools-container) {
    visibility: visible !important;
  }

  // 助手一轮套 AI Content Bubble：有过程区，或带 .ai-turn-card（CustomMessageCard / HITL）。
  // 账号选择、默认澄清常无 tool/reasoning；HITL 也不走 .custom-msg-card。只认过程区会漏这些轮次。
  // 新结构化卡把根节点加上 ai-turn-card 即可进气泡，不要再按业务名枚举。
  // :deep 必须写完整后代链，嵌套普通选择器会把 data-v 打到 chat-x 内部节点上导致不生效。
  // 卡底留白必须 >= 投影向下的绘制范围（12px 偏移 + 32px 模糊 ≈ 32px）：
  // chat-x 的 .ai-message-container 带 contain: layout paint + overflow-y: auto，会在容器边界硬裁投影，
  // 留白不足时最后一轮卡片的底部投影被切成一条直边；相邻两轮贴死时后一张的白底也会盖掉前一张的投影。
  // padding-top 沿用 chat-x 的 8px（投影只朝下，顶部不需要额外留白）。
  :deep(.message-group:is(:has(.process-zone-summary), :has(.ai-turn-card))) {
    padding-bottom: 32px;
  }

  // 不设 max-width：稿面标的 900 会比没有过程区的一轮（消息组内容宽 984）窄一截，两种轮次宽度不齐
  :deep(.message-group:is(:has(.process-zone-summary), :has(.ai-turn-card)) .message-group-messages) {
    align-items: stretch;
    width: 100%;
    padding: 16px 24px 24px;
    background: #fff;
    border: 1px solid #f1f5f9;
    border-radius: 16px;
    box-shadow: 0 12px 32px 0 rgb(0 0 0 / 4%);
    gap: 16px;
  }

  // 工具栏收回卡内（稿面在分割线下方）。原先绝对定位到卡外，是因为
  // visibility:hidden 不脱流、留在卡内会恒占 20px + 16px gap；常驻后占位是预期，绝对定位的前提消失。
  // 父级 gap 16 + 这里的 1px 线 + 16px padding-top，对齐稿面「内容 → 线 → 工具栏」的间距。
  :deep(
      .message-group:is(:has(.process-zone-summary), :has(.ai-turn-card))
        .message-group-messages
        > .ai-message-tools-container
    ) {
    padding-top: 16px;
    border-top: 1px solid #eaebf0;
  }

  :deep(.message-group:is(:has(.process-zone-summary), :has(.ai-turn-card)) .ai-message-item),
  :deep(.message-group:is(:has(.process-zone-summary), :has(.ai-turn-card)) .ai-assistant-message),
  :deep(.message-group:is(:has(.process-zone-summary), :has(.ai-turn-card)) .ai-turn-card) {
    width: 100%;
  }

  // 组级工具栏已常驻；单条 .ai-assistant-message-tools 是 visibility:hidden 仍占位（还带 12px 下边距），
  // 账号选择这种「正文 + 卡」轮次会在说明文字和卡片之间多出一截空白。
  :deep(.message-group:is(:has(.process-zone-summary), :has(.ai-turn-card)) .ai-assistant-message-tools) {
    display: none;
  }

  // 过程区收起：工具行/思考仍留在消息列表里（避免下标 key 引发整轮 DOM 重建），
  // 这里把空壳连同 chat-x 的外层容器一起收掉，不占位也不吃 gap
  :deep(.message-group:has(.process-zone-summary) .ai-message-item:has(.process-hidden)) {
    display: none;
  }

  // 空助手消息的空壳：本身高 0，但仍算一个 flex 项、两侧各吃一份 gap（表现为该处间距翻倍），
  // 必须连 chat-x 的外层条目一起收掉。不限过程区：普通轮次同理
  :deep(.ai-message-item:has(.blank-message)) {
    display: none;
  }

  :deep(.message-group:has(.process-zone-summary) .ai-assistant-message),
  :deep(.message-group:has(.process-zone-summary) .ai-assistant-message-content) {
    gap: 0;
  }

  :deep(
      .message-group:has(.process-zone-summary)
        .ai-assistant-message:has(.process-zone-summary)
        .ai-assistant-message-tools
    ),
  :deep(
      .message-group:has(.process-zone-summary) .ai-assistant-message:has(.process-tool-row) .ai-assistant-message-tools
    ),
  :deep(
      .message-group:has(.process-zone-summary) .ai-assistant-message:has(.process-thinking) .ai-assistant-message-tools
    ),
  :deep(.message-group:has(.process-zone-summary) .ai-assistant-message:has(.process-zone-summary) .ai-markdown-body),
  :deep(.message-group:has(.process-zone-summary) .ai-assistant-message:has(.process-tool-row) .ai-markdown-body),
  :deep(.message-group:has(.process-zone-summary) .ai-assistant-message:has(.process-thinking) .ai-markdown-body) {
    display: none;
  }

  // 外层已是气泡，内层申领/账号卡不再套第二层描边和投影（全页 .chatbot-chat-messages 也会给 .custom-msg-card 加阴影）
  :deep(.message-group:is(:has(.process-zone-summary), :has(.ai-turn-card)) .custom-msg-card) {
    padding: 0;
    background: transparent;
    border: none;
    border-radius: 0;
    box-shadow: none;
  }
}
</style>
