import { ref } from 'vue';
import { MessageRole, MessageStatus, type Message } from '@blueking/chat-x';

let idCounter = 0;
export const genId = () => {
  idCounter += 1;
  return `msg_${idCounter}`;
};

export function useMessage() {
  const messages = ref<Message[]>([]);

  const getMessageByMessageId = (messageId: string) => messages.value.find((m) => m.messageId === messageId);

  const getCurrentStreamingMessage = () => messages.value.findLast((m) => m.status === MessageStatus.Streaming);

  const addUserMessage = (content: string) => {
    const msg: Message = {
      role: MessageRole.User,
      content,
      id: genId(),
      messageId: genId(),
      status: MessageStatus.Complete,
    } as Message;
    messages.value.push(msg);
    return msg;
  };

  const clearMessages = () => {
    messages.value = [];
  };

  const appendContent = (messageId: string, delta: string) => {
    const msg = getMessageByMessageId(messageId);
    if (msg) {
      (msg as { content: string }).content += delta;
    }
  };

  return {
    messages,
    getMessageByMessageId,
    getCurrentStreamingMessage,
    addUserMessage,
    clearMessages,
    appendContent,
  };
}

export type MessageModule = ReturnType<typeof useMessage>;
