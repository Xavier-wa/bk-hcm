import { MessageContentType, MessageRole, MessageStatus, type Message, type ToolCall } from '@blueking/chat-x';

import {
  EventType,
  HOST_APPLY_CONFIRM_EVENT,
  HOST_APPLY_RECOMMEND_EVENT,
  HOST_APPLY_SUBMIT_EVENT,
  SCENE_SWITCHED_EVENT,
  type AccountSelectInterruptValue,
  type HitlInterruptValue,
  type HostApplyPreorderValue,
  type HostApplyRecommendValue,
  type HostApplySubmitValue,
  type SceneSwitchedValue,
} from './types';
import { genId, type MessageModule } from './use-message';

// scene.switched 回调：由 useChatbot 在 session 模块就绪后注入（event 早于 session 创建）
export interface EventHandlerDeps {
  onSceneSwitched?: (to: string) => void;
}

// 解析 scene.switched 的 value（兼容 JSON 字符串与已解析对象）
const parseSceneSwitchedPayload = (value: unknown): { to?: string; from?: string } | null => {
  try {
    const root = (typeof value === 'string' ? JSON.parse(value) : value) as SceneSwitchedValue | null;
    if (!root || typeof root !== 'object') return null;
    const { payload } = root;
    if (!payload || typeof payload !== 'object') return null;
    const to = String(payload.to ?? '').trim();
    const from = String(payload.from ?? '').trim();
    return { to: to || undefined, from: from || undefined };
  } catch {
    return null;
  }
};

export function useEventHandler(msg: MessageModule, deps: EventHandlerDeps = {}) {
  const getToolCallMessage = (toolCallId: string) =>
    msg.messages.value.find(
      (m) =>
        m.role === MessageRole.Assistant &&
        (m as { toolCalls?: ToolCall[] }).toolCalls?.some((tc) => tc.id === toolCallId),
    );

  // 当前进行中的推理消息。一轮对话可能有多个推理阶段，每次 REASONING_START 新建、结算后复位。
  // 不用 getCurrentStreamingMessage 定位：推理与正文可能同时处于 streaming，最后一条不一定是推理。
  let currentReasoning: Message | null = null;
  let reasoningStartedAt = 0;

  const startReasoning = () => {
    const messageId = genId();
    msg.messages.value.push({
      role: MessageRole.Reasoning,
      content: [],
      id: genId(),
      messageId,
      status: MessageStatus.Streaming,
    } as unknown as Message);
    // 数组里存的是原始对象，必须回查取到响应式代理再持有，否则后续改动不触发视图更新
    const message = msg.getMessageByMessageId(messageId) as Message;
    currentReasoning = message;
    reasoningStartedAt = Date.now();
    return message;
  };

  // 结算当前推理消息。耗时优先取事件下发值，缺失或非法时用前端计时兜底。
  // 供 REASONING_END 与 RUN_FINISHED / RUN_ERROR 收尾复用，保证缺 END 事件时不卡在「思考中」。
  const finalizeReasoning = (eventDuration?: unknown) => {
    if (!currentReasoning) return;
    const fromEvent = typeof eventDuration === 'number' && Number.isFinite(eventDuration) && eventDuration >= 0;
    (currentReasoning as unknown as { duration: number }).duration = fromEvent
      ? (eventDuration as number)
      : Date.now() - reasoningStartedAt;
    currentReasoning.status = MessageStatus.Complete;
    currentReasoning = null;
    reasoningStartedAt = 0;
  };

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
        // 推理消息的 content 是数组，不能被当作正文续写；此时另起一条 assistant 消息
        if (m && m.role !== MessageRole.Reasoning) {
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

      // ---- 推理（思维链） ----
      case EventType.ReasoningStart: {
        startReasoning();
        break;
      }
      case EventType.ReasoningMessageStart: {
        // 一次推理阶段内可能有多段正文，每段独立成条 MarkdownContent
        if (currentReasoning) (currentReasoning.content as string[]).push('');
        break;
      }
      case EventType.ReasoningMessageContent: {
        // 事件顺序异常（无进行中推理或未开段）时忽略该增量，不新建记录、不报错
        if (!currentReasoning || typeof event.delta !== 'string') break;
        const segments = currentReasoning.content as string[];
        if (segments.length === 0) break;
        segments[segments.length - 1] += event.delta;
        break;
      }
      case EventType.ReasoningMessageEnd:
        // 段落边界由下一次 REASONING_MESSAGE_START 划分，无需处理
        break;
      case EventType.ReasoningEnd: {
        finalizeReasoning(event.duration);
        break;
      }
      case EventType.ReasoningMessageChunk: {
        // 便捷事件自成一段完整正文，无进行中推理时新建一条承载
        if (typeof event.delta !== 'string') break;
        const message = currentReasoning ?? startReasoning();
        (message.content as string[]).push(event.delta);
        break;
      }
      case EventType.ReasoningEncryptedValue:
        // 加密思维链一律不解密、不展示
        break;

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
        // 已产出的推理正文仍然有效，按完成收尾；错误由随后的 assistant 气泡承载
        finalizeReasoning();
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
        finalizeReasoning();
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
        break;
      case EventType.Custom: {
        const name = event.name as string;
        if (name === 'hitl.interrupt') {
          const rawValue = JSON.parse(event.value as string) as HitlInterruptValue;
          msg.messages.value.push({
            role: MessageRole.Assistant,
            content: rawValue as any,
            id: genId(),
            messageId: genId(),
            status: MessageStatus.Complete,
            __type: 'hitl.interrupt',
          } as Message);
        } else if (name === 'account_select.interrupt') {
          const rawValue = JSON.parse(event.value as string) as AccountSelectInterruptValue;
          msg.messages.value.push({
            role: MessageRole.Assistant,
            content: rawValue as any,
            id: genId(),
            messageId: genId(),
            status: MessageStatus.Complete,
            __type: 'account_select.interrupt',
          } as Message);
        } else if (name === HOST_APPLY_RECOMMEND_EVENT) {
          const rawValue = JSON.parse(event.value as string) as HostApplyRecommendValue;
          msg.messages.value.push({
            role: MessageRole.Assistant,
            content: rawValue as any,
            id: genId(),
            messageId: genId(),
            status: MessageStatus.Complete,
            __type: 'host_apply.recommend',
          } as Message);
        } else if (name === HOST_APPLY_CONFIRM_EVENT) {
          const rawValue = JSON.parse(event.value as string) as HostApplyPreorderValue;
          msg.messages.value.push({
            role: MessageRole.Assistant,
            content: rawValue as any,
            id: genId(),
            messageId: genId(),
            status: MessageStatus.Complete,
            __type: 'host_apply.preorder',
          } as Message);
        } else if (name === HOST_APPLY_SUBMIT_EVENT) {
          const rawValue = JSON.parse(event.value as string) as HostApplySubmitValue;
          msg.messages.value.push({
            role: MessageRole.Assistant,
            content: rawValue as any,
            id: genId(),
            messageId: genId(),
            status: MessageStatus.Complete,
            __type: 'host_apply.submit',
          } as Message);
        } else if (name === SCENE_SWITCHED_EVENT) {
          // 同会话场景切换：更新本地 sessionTag；不推消息气泡、不打断流式
          const parsed = parseSceneSwitchedPayload(event.value);
          if (!parsed?.to) {
            if (parsed === null) {
              console.warn('[Event] scene.switched value parse failed, ignored');
            }
            break;
          }
          deps.onSceneSwitched?.(parsed.to);
        }
        // 其余约定外的 CUSTOM 事件名不处理，保持原生（伴随的 TEXT_MESSAGE 文本气泡）输出
        break;
      }
      case EventType.Raw:
        break;
      default:
        break;
    }
  };

  return { handleEvent };
}

export type EventModule = ReturnType<typeof useEventHandler>;
