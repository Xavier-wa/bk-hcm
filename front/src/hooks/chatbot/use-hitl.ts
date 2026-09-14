import { type Ref } from 'vue';

import { MessageRole, type Message } from '@blueking/chat-x';

import { extractText } from './use-chatbot';
import { type HitlInterruptMessage, type HitlInterruptValue } from './types';

// useHitl 提供 HITL 中断消息的识别、内容提取与只读态推断，供全页与浮窗的消息列表复用
export const useHitl = (messages: Ref<Message[]>) => {
  // 类型守卫：检查消息是否为 HITL 中断消息
  const isHitlInterruptMessage = (message: Message): boolean => {
    return (message as HitlInterruptMessage).__type === 'hitl.interrupt';
  };

  const getHitlContent = (message: Message): HitlInterruptValue | null => {
    if (!isHitlInterruptMessage(message)) return null;
    return (message as HitlInterruptMessage).content as HitlInterruptValue;
  };

  // 历史消息只读展示：仅当后续 user 文本命中 options 时，才视为有效选择；
  // 未命中则按“其它/未命中”处理（不回填具体文本，避免误判为自定义输入）
  const getHitlReadonlyState = (message: Message): { readonly: boolean; value: string } => {
    if (!isHitlInterruptMessage(message)) return { readonly: false, value: '' };

    const currentIndex = messages.value.findIndex((item) => item.id === message.id);
    if (currentIndex < 0) return { readonly: false, value: '' };

    const nextMessage = messages.value[currentIndex + 1];
    if (!nextMessage || nextMessage.role !== MessageRole.User) return { readonly: false, value: '' };

    const userAnswer = extractText(nextMessage.content).trim();
    const hitlContent = getHitlContent(message);
    const options = hitlContent?.value.options ?? [];

    if (!userAnswer) return { readonly: true, value: '' };
    // 无 options 的默认澄清：自由文本是唯一可能的作答形态，后继 user 消息即答案，直接回显原文；
    // 有 options 却未命中时仍按「其它/未命中」留空，不改既有保守判定
    if (options.length === 0) return { readonly: true, value: userAnswer };
    if (options.includes(userAnswer)) return { readonly: true, value: userAnswer };

    return { readonly: true, value: '' };
  };

  return {
    isHitlInterruptMessage,
    getHitlContent,
    getHitlReadonlyState,
  };
};
