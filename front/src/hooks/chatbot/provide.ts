import { inject, type InjectionKey } from 'vue';

import { type useChatbot } from './use-chatbot';

// Chatbot 上下文：useChatbot() 返回的完整实例，供容器 provide、原子组件 inject 复用同一会话内核
export type ChatbotContext = ReturnType<typeof useChatbot>;

export const ChatbotKey: InjectionKey<ChatbotContext> = Symbol('chatbot');

// useChatbotContext 在原子组件内注入 chatbot 实例，未被容器 provide 时抛错以尽早暴露装配问题
export const useChatbotContext = (): ChatbotContext => {
  const ctx = inject(ChatbotKey);
  if (!ctx) {
    throw new Error('useChatbotContext must be used within a component that provides ChatbotKey');
  }
  return ctx;
};
