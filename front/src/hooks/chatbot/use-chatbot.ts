import { MessageRole, type Message } from '@blueking/chat-x';

import * as sessionApi from '@/store/chatbot/session';
import { useMessage } from './use-message';
import { useEventHandler } from './use-event';
import { useStream } from './use-stream';
import { useSession } from './use-session';

export type { ChatSession } from './types';

export const extractText = (content: unknown): string => {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content
    .filter((item) => typeof item === 'object' && item?.type === 'text' && item?.text)
    .map((item) => item.text as string)
    .join('');
};

export function useChatbot() {
  const messageModule = useMessage();
  const eventModule = useEventHandler(messageModule);
  const streamModule = useStream(messageModule, eventModule);
  const sessionModule = useSession({
    messages: messageModule.messages,
    sessionCode: streamModule.sessionCode,
    abortStream: streamModule.abortStream,
    fetchHistory: streamModule.fetchHistory,
  });

  const sendMessage = async (content: string) => {
    messageModule.addUserMessage(content);
    const session = sessionModule.currentSession.value;
    if (session && session.sessionName === '新对话' && content.trim()) {
      const newName = content.trim().slice(0, 30);
      session.sessionName = newName;
      sessionApi.updateSession(session.sessionCode, newName).catch(() => {});
    }
    await streamModule.streamChat([{ role: 'user', content }]);
    sessionModule.saveCurrentSession();
    if (session) {
      session.sessionContentCount += 1;
      session.updatedAt = new Date().toISOString();
      sessionModule.moveSessionToTop(session.sessionCode);
    }
  };

  const regenerate = async (aiMessages: Message[]) => {
    if (streamModule.isChatting.value) return;
    const firstAiMsg = aiMessages[0];
    const firstAiIndex = messageModule.messages.value.findIndex((m) => m.id === firstAiMsg.id);
    if (firstAiIndex === -1) return;

    let userContent = '';
    for (let i = firstAiIndex - 1; i >= 0; i--) {
      if (messageModule.messages.value[i].role === MessageRole.User) {
        userContent = extractText(messageModule.messages.value[i].content);
        break;
      }
    }
    if (!userContent.trim()) return;

    messageModule.messages.value.splice(firstAiIndex);
    await streamModule.streamChat([{ role: 'user', content: userContent }]);
    sessionModule.saveCurrentSession();
  };

  const resendEdited = async (message: Message, newContent: unknown) => {
    if (streamModule.isChatting.value) return;
    const content = extractText(newContent);
    if (!content.trim()) return;

    const index = messageModule.messages.value.findIndex((m) => m.id === message.id);
    if (index === -1) return;

    messageModule.messages.value.splice(index);
    messageModule.addUserMessage(content);
    await streamModule.streamChat([{ role: 'user', content }]);
    sessionModule.saveCurrentSession();
  };

  return {
    messages: messageModule.messages,
    isChatting: streamModule.isChatting,
    sessions: sessionModule.sessions,
    currentSessionCode: sessionModule.currentSessionCode,
    currentSession: sessionModule.currentSession,
    isLoadingHistory: sessionModule.isLoadingHistory,
    isLoadingSessions: sessionModule.isLoadingSessions,
    sendMessage,
    regenerate,
    resendEdited,
    stopGeneration: streamModule.stopGeneration,
    createSession: sessionModule.createSession,
    switchSession: sessionModule.switchSession,
    deleteSession: sessionModule.deleteSession,
    renameSession: sessionModule.renameSession,
    initSessions: sessionModule.initSessions,
    clearMessages: messageModule.clearMessages,
  };
}
