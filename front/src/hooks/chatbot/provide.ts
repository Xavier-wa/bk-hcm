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

// 渲染场景：fullpage 为全页 chatbot，floating 为右侧 AI 助手浮窗
export type ChatbotMode = 'fullpage' | 'floating';

export const ChatbotModeKey: InjectionKey<ChatbotMode> = Symbol('chatbot-mode');

// useChatbotMode 注入当前渲染场景，未 provide 时默认全页
export const useChatbotMode = (): ChatbotMode => inject(ChatbotModeKey, 'fullpage');
