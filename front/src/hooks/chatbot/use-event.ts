import { MessageContentType, MessageRole, MessageStatus, type Message, type ToolCall } from '@blueking/chat-x';

import { EventType } from './types';
import { genId, type MessageModule } from './use-message';

export function useEventHandler(msg: MessageModule) {
  const getToolCallMessage = (toolCallId: string) =>
    msg.messages.value.find(
      (m) =>
        m.role === MessageRole.Assistant &&
        (m as { toolCalls?: ToolCall[] }).toolCalls?.some((tc) => tc.id === toolCallId),
    );

  const handleEvent = (event: Record<string, unknown>) => {
    switch (event.type) {
      // ---- 文本消息 ----
      case EventType.TextMessageStart: {
        msg.messages.value.push({
          role: (event.role as string) || MessageRole.Assistant,
          content: '',
          id: genId(),
          messageId: event.messageId as string,
          status: MessageStatus.Streaming,
        } as Message);
        break;
      }
      case EventType.TextMessageContent: {
        msg.appendContent(event.messageId as string, event.delta as string);
        break;
      }
      case EventType.TextMessageEnd: {
        const m = msg.getMessageByMessageId(event.messageId as string);
        if (m) m.status = MessageStatus.Complete;
        break;
      }
      case EventType.TextMessageChunk: {
        const m = msg.getCurrentStreamingMessage();
        if (m) {
          (m as { content: string }).content += (event.delta as string) || '';
        } else {
          msg.messages.value.push({
            role: (event.role as string) || MessageRole.Assistant,
            content: (event.delta as string) || '',
            id: genId(),
            messageId: event.messageId as string,
            status: MessageStatus.Complete,
          } as Message);
        }
        break;
      }

      // ---- 思考 ----
      case EventType.ThinkingStart: {
        msg.messages.value.push({
          role: MessageRole.Reasoning,
          content: [],
          id: genId(),
          messageId: genId(),
          status: MessageStatus.Streaming,
        } as unknown as Message);
        break;
      }
      case EventType.ThinkingTextMessageStart: {
        const m = msg.getCurrentStreamingMessage();
        if (m && m.role === MessageRole.Reasoning) {
          (m.content as string[]).push('');
        }
        break;
      }
      case EventType.ThinkingTextMessageContent: {
        const m = msg.getCurrentStreamingMessage();
        if (m && m.role === MessageRole.Reasoning) {
          const arr = m.content as string[];
          arr[arr.length - 1] += event.delta as string;
        }
        break;
      }
      case EventType.ThinkingTextMessageEnd:
        break;
      case EventType.ThinkingEnd: {
        const m = msg.getCurrentStreamingMessage();
        if (m && m.role === MessageRole.Reasoning) {
          (m as unknown as { duration: number }).duration = (event.duration as number) || 0;
          m.status = MessageStatus.Complete;
        }
        break;
      }

      // ---- 工具调用 ----
      case EventType.ToolCallStart: {
        msg.messages.value.push({
          role: MessageRole.Assistant,
          content: '',
          id: genId(),
          messageId: genId(),
          status: MessageStatus.Streaming,
          toolCalls: [
            {
              id: event.toolCallId as string,
              type: MessageContentType.Function,
              function: {
                mcpName: (event.mcpName as string) || '',
                name: event.toolCallName as string,
                arguments: '',
                description: (event.description as string) || '',
              },
            },
          ],
        } as unknown as Message);
        break;
      }
      case EventType.ToolCallArgs: {
        const m = getToolCallMessage(event.toolCallId as string);
        if (m) {
          const toolCalls = (m as { toolCalls?: ToolCall[] }).toolCalls || [];
          const tc = toolCalls.find((t) => t.id === event.toolCallId);
          if (tc) tc.function.arguments += event.delta as string;
        }
        break;
      }
      case EventType.ToolCallEnd: {
        const m = getToolCallMessage(event.toolCallId as string);
        if (m) m.status = MessageStatus.Complete;
        break;
      }
      case EventType.ToolCallResult: {
        msg.messages.value.push({
          role: MessageRole.Tool,
          content: event.content as string,
          id: genId(),
          messageId: (event.messageId as string) || genId(),
          status: MessageStatus.Complete,
          toolCallId: event.toolCallId as string,
          duration: (event.duration as number) || 0,
        } as unknown as Message);
        break;
      }
      case EventType.ToolCallChunk:
        break;

      // ---- 步骤 ----
      case EventType.StepStarted:
      case EventType.StepFinished:
        break;

      // ---- 运行生命周期 ----
      case EventType.RunStarted:
        break;
      case EventType.RunError: {
        const streaming = msg.getCurrentStreamingMessage();
        if (streaming) streaming.status = MessageStatus.Complete;
        msg.messages.value.push({
          role: MessageRole.Assistant,
          content: (event.message as string) || '请求出错',
          id: genId(),
          messageId: genId(),
          status: MessageStatus.Error,
        } as Message);
        break;
      }
      case EventType.RunFinished: {
        const streaming = msg.getCurrentStreamingMessage();
        if (streaming) streaming.status = MessageStatus.Complete;
        break;
      }

      // ---- 状态/快照/自定义（预留） ----
      case EventType.MessagesSnapshot:
      case EventType.StateDelta:
      case EventType.StateSnapshot:
      case EventType.ActivityDelta:
      case EventType.ActivitySnapshot:
      case EventType.Custom:
      case EventType.Raw:
        break;
      default:
        break;
    }
  };

  return { handleEvent };
}

export type EventModule = ReturnType<typeof useEventHandler>;
