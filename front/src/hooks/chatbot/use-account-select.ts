import { computed, type Ref } from 'vue';

import { MessageRole, type Message } from '@blueking/chat-x';

import { extractText } from './use-chatbot';
import { getVendorDisplay } from './vendor-display';
import {
  type AccountSelectInterruptMessage,
  type AccountSelectInterruptValue,
  type AccountSelectOption,
} from './types';

export interface AccountSelectReadonlyState {
  readonly: boolean;
  accountId: string;
}

export interface SelectedAccountEcho {
  vendor: string;
  vendorName: string;
  accountName: string;
}

// useAccountSelect 提供云账号选择中断消息的识别、内容提取、只读态推断与吸顶回显派生，供全页与浮窗复用
export const useAccountSelect = (messages: Ref<Message[]>) => {
  const isAccountSelectMessage = (message: Message): boolean => {
    return (message as AccountSelectInterruptMessage).__type === 'account_select.interrupt';
  };

  const getAccountSelectContent = (message: Message): AccountSelectInterruptValue | null => {
    if (!isAccountSelectMessage(message)) return null;
    return (message as AccountSelectInterruptMessage).content as AccountSelectInterruptValue;
  };

  // 命中所选账号：优先用实时流写入的 __selectedAccountId；
  // 历史回放无该字段时，退化为按后续 user 消息文本匹配（account_id 精确 / 文本包含账号名或 id）
  const matchSelectedAccountId = (message: Message): string => {
    const explicit = (message as AccountSelectInterruptMessage).__selectedAccountId;
    if (explicit) return explicit;

    const currentIndex = messages.value.findIndex((item) => item.id === message.id);
    if (currentIndex < 0) return '';

    const nextMessage = messages.value[currentIndex + 1];
    if (!nextMessage || nextMessage.role !== MessageRole.User) return '';

    const userAnswer = extractText(nextMessage.content).trim();
    if (!userAnswer) return '';

    const options = getAccountSelectContent(message)?.value.options ?? [];
    const matched = options.find(
      (opt) =>
        opt.account_id === userAnswer ||
        (opt.account_name && userAnswer.includes(opt.account_name)) ||
        userAnswer.includes(opt.account_id),
    );
    return matched?.account_id ?? '';
  };

  // 只读态：存在后续 user 消息（已提交）或已显式选择 → 只读；否则保持可交互（对齐 HITL）
  const getAccountSelectReadonlyState = (message: Message): AccountSelectReadonlyState => {
    if (!isAccountSelectMessage(message)) return { readonly: false, accountId: '' };

    const explicit = (message as AccountSelectInterruptMessage).__selectedAccountId;
    const currentIndex = messages.value.findIndex((item) => item.id === message.id);
    const nextMessage = currentIndex >= 0 ? messages.value[currentIndex + 1] : undefined;
    const submitted = Boolean(explicit) || (!!nextMessage && nextMessage.role === MessageRole.User);

    if (!submitted) return { readonly: false, accountId: '' };
    return { readonly: true, accountId: matchSelectedAccountId(message) };
  };

  const resolveOption = (message: Message, accountId: string): AccountSelectOption | null => {
    if (!accountId) return null;
    const options = getAccountSelectContent(message)?.value.options ?? [];
    return options.find((opt) => opt.account_id === accountId) ?? null;
  };

  // 吸顶回显：取最后一条已选定账号的云账号选择消息，解析为厂商名 + 账号名
  const selectedAccountEcho = computed<SelectedAccountEcho | null>(() => {
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const message = messages.value[i];
      if (!isAccountSelectMessage(message)) continue;
      const { readonly, accountId } = getAccountSelectReadonlyState(message);
      if (!readonly || !accountId) continue;
      const option = resolveOption(message, accountId);
      if (!option) continue;
      return {
        vendor: option.vendor,
        vendorName: getVendorDisplay(option.vendor).name,
        accountName: option.account_name,
      };
    }
    return null;
  });

  return {
    isAccountSelectMessage,
    getAccountSelectContent,
    getAccountSelectReadonlyState,
    selectedAccountEcho,
  };
};
