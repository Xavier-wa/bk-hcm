import { ref } from 'vue';
import { MessageRole, MessageStatus, type Message } from '@blueking/chat-x';

import * as agentApi from '@/store/chatbot/agent';
import {
  EventType,
  HOST_APPLY_CONFIRM_EVENT,
  HOST_APPLY_RECOMMEND_EVENT,
  HOST_APPLY_SUBMIT_EVENT,
  type AccountSelectInterruptValue,
  type AccountSelectOption,
  type HitlInterruptValue,
  type HostApplyRecommendation,
  type HostApplyPreorderValue,
  type HostApplyRecommendValue,
  type HostApplySubmitValue,
  type HostApplySuborder,
} from './types';
import { genId, type MessageModule } from './use-message';
import type { EventModule } from './use-event';

const readSSE = async (
  reader: ReadableStreamDefaultReader<string>,
  onEvent: (e: Record<string, unknown>) => boolean | void,
) => {
  let buffer = '';
  // eslint-disable-next-line no-constant-condition
  while (true) {
    const { value, done } = await reader.read();
    if (done) break;

    buffer += value;
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    let shouldStop = false;
    for (const raw of lines) {
      if (!raw.startsWith('data:')) continue;
      const json = raw.slice(5).trim();
      if (!json) continue;
      try {
        if (onEvent(JSON.parse(json)) === true) {
          shouldStop = true;
          break;
        }
      } catch {
        /* malformed JSON, skip */
      }
    }
    if (shouldStop) {
      reader.cancel();
      break;
    }
  }
};

export function useStream(msg: MessageModule, event: EventModule) {
  const isChatting = ref(false);
  const sessionCode = ref('');
  let abortController: AbortController | null = null;

  // 防御：useMessageGroup 仅看消息列表最后一条的 role 决定是否注入 LoadingMessage（"请求中..."），
  // 与 messageStatus prop 无关。若 tail 仍是 user，必须补一条 assistant 占位消息阻断注入。
  const ensureAssistantTail = (content: string, status: MessageStatus = MessageStatus.Complete) => {
    const last = msg.messages.value.at(-1);
    if (last?.role === MessageRole.User) {
      msg.messages.value.push({
        role: MessageRole.Assistant,
        content,
        id: genId(),
        messageId: genId(),
        status,
      } as Message);
    }
  };

  // /history SNAPSHOT 里若出现连续 user 消息（上一次对话被中断、没有 assistant 回复就被持久化），
  // 为每个"未响应"的 user 消息补一条占位。不包含 tail —— tail 可能被 resume 的实时事件继续填充，
  // 统一在 fetchHistory 的 finally 里根据最终状态决定是否补占位。
  const fillSnapshotUserGaps = (content: string) => {
    const list = msg.messages.value;
    const next: Message[] = [];
    for (let i = 0; i < list.length; i++) {
      next.push(list[i]);
      const cur = list[i];
      const peek = list[i + 1];
      if (cur.role === MessageRole.User && peek && peek.role === MessageRole.User) {
        next.push({
          role: MessageRole.Assistant,
          content,
          id: genId(),
          messageId: genId(),
          status: MessageStatus.Error,
        } as Message);
      }
    }
    msg.messages.value = next;
  };

  const parseHitlInterruptValue = (input: unknown): HitlInterruptValue | null => {
    let parsed = input;
    if (typeof parsed === 'string') {
      try {
        parsed = JSON.parse(parsed);
      } catch {
        return null;
      }
    }

    if (!parsed || typeof parsed !== 'object') return null;

    const raw = parsed as Partial<HitlInterruptValue>;
    const question = raw.value?.question;
    const options = raw.value?.options;

    if (typeof question !== 'string' || !Array.isArray(options)) return null;

    const normalizedOptions = options.filter((item): item is string => typeof item === 'string');
    if (normalizedOptions.length === 0) return null;

    return {
      checkpoint_id: typeof raw.checkpoint_id === 'string' ? raw.checkpoint_id : '',
      lineage_id: typeof raw.lineage_id === 'string' ? raw.lineage_id : '',
      value: {
        question,
        options: normalizedOptions,
      },
    };
  };

  const parseAccountSelectInterruptValue = (input: unknown): AccountSelectInterruptValue | null => {
    let parsed = input;
    if (typeof parsed === 'string') {
      try {
        parsed = JSON.parse(parsed);
      } catch {
        return null;
      }
    }

    if (!parsed || typeof parsed !== 'object') return null;

    const raw = parsed as Partial<AccountSelectInterruptValue>;
    const options = raw.value?.options;
    if (!Array.isArray(options)) return null;

    const normalizedOptions = options.filter(
      (item): item is AccountSelectOption =>
        !!item && typeof item === 'object' && typeof (item as AccountSelectOption).account_id === 'string',
    );
    if (normalizedOptions.length === 0) return null;

    return {
      checkpoint_id: typeof raw.checkpoint_id === 'string' ? raw.checkpoint_id : '',
      lineage_id: typeof raw.lineage_id === 'string' ? raw.lineage_id : '',
      value: {
        type: 'account_select.interrupt',
        options: normalizedOptions,
      },
    };
  };

  const parseHostApplyRecommendValue = (input: unknown): HostApplyRecommendValue | null => {
    let parsed = input;
    if (typeof parsed === 'string') {
      try {
        parsed = JSON.parse(parsed);
      } catch {
        return null;
      }
    }
    if (!parsed || typeof parsed !== 'object') return null;

    const raw = parsed as Partial<HostApplyRecommendValue>;
    const recommendations = raw.value?.recommendations;
    if (!Array.isArray(recommendations)) return null;

    const normalizedRecommendations = recommendations.filter(
      (item): item is HostApplyRecommendation =>
        !!item && typeof item === 'object' && typeof (item as HostApplyRecommendation).suborder === 'object',
    );
    if (normalizedRecommendations.length === 0) return null;

    return {
      checkpoint_id: typeof raw.checkpoint_id === 'string' ? raw.checkpoint_id : '',
      lineage_id: typeof raw.lineage_id === 'string' ? raw.lineage_id : '',
      value: {
        recommendations: normalizedRecommendations,
      },
    };
  };

  const parseHostApplyPreorderValue = (input: unknown): HostApplyPreorderValue | null => {
    let parsed = input;
    if (typeof parsed === 'string') {
      try {
        parsed = JSON.parse(parsed);
      } catch {
        return null;
      }
    }
    if (!parsed || typeof parsed !== 'object') return null;

    const raw = parsed as Partial<HostApplyPreorderValue>;
    const suborders = raw.value?.suborders;
    if (!Array.isArray(suborders)) return null;

    const normalizedSuborders = suborders.filter(
      (item): item is HostApplySuborder => !!item && typeof item === 'object',
    );
    if (normalizedSuborders.length === 0) return null;

    return {
      checkpoint_id: typeof raw.checkpoint_id === 'string' ? raw.checkpoint_id : '',
      lineage_id: typeof raw.lineage_id === 'string' ? raw.lineage_id : '',
      value: {
        suborders: normalizedSuborders,
      },
    };
  };

  const parseHostApplySubmitValue = (input: unknown): HostApplySubmitValue | null => {
    let parsed = input;
    if (typeof parsed === 'string') {
      try {
        parsed = JSON.parse(parsed);
      } catch {
        return null;
      }
    }
    if (!parsed || typeof parsed !== 'object') return null;

    const raw = parsed as HostApplySubmitValue;
    const suborders = raw.value?.data?.body_param?.suborders;
    if (!Array.isArray(suborders) || suborders.length === 0) return null;

    return {
      checkpoint_id: typeof raw.checkpoint_id === 'string' ? raw.checkpoint_id : '',
      lineage_id: typeof raw.lineage_id === 'string' ? raw.lineage_id : '',
      value: raw.value,
    };
  };

  const toHistoryMessage = (raw: Record<string, unknown>): Message => {
    const id = (raw.id as string) || genId();
    const role = raw.role as string;
    const base = {
      id,
      messageId: (raw.id as string) || genId(),
      status: MessageStatus.Complete,
      ...(raw.toolCalls ? { toolCalls: raw.toolCalls } : {}),
      ...(raw.toolCallId ? { toolCallId: raw.toolCallId, duration: raw.duration ?? 0 } : {}),
    };

    if (role === 'activity' && raw.activityType === 'CUSTOM') {
      const content = raw.content as Record<string, unknown>;
      if (content?.name === 'hitl.interrupt') {
        const parsed = parseHitlInterruptValue(content.value);
        if (parsed) {
          return {
            role: MessageRole.Assistant,
            content: parsed as unknown as Message['content'],
            __type: 'hitl.interrupt',
            ...base,
          } as Message;
        }
      }

      if (content?.name === 'account_select.interrupt') {
        const parsed = parseAccountSelectInterruptValue(content.value);
        if (parsed) {
          return {
            role: MessageRole.Assistant,
            content: parsed as unknown as Message['content'],
            __type: 'account_select.interrupt',
            ...base,
          } as Message;
        }
      }

      if (content?.name === HOST_APPLY_RECOMMEND_EVENT) {
        const parsed = parseHostApplyRecommendValue(content.value);
        if (parsed) {
          return {
            role: MessageRole.Assistant,
            content: parsed as unknown as Message['content'],
            __type: 'host_apply.recommend',
            ...base,
          } as Message;
        }
      }

      if (content?.name === HOST_APPLY_CONFIRM_EVENT) {
        const parsed = parseHostApplyPreorderValue(content.value);
        if (parsed) {
          return {
            role: MessageRole.Assistant,
            content: parsed as unknown as Message['content'],
            __type: 'host_apply.preorder',
            ...base,
          } as Message;
        }
      }

      if (content?.name === HOST_APPLY_SUBMIT_EVENT) {
        const parsed = parseHostApplySubmitValue(content.value);
        if (parsed) {
          return {
            role: MessageRole.Assistant,
            content: parsed as unknown as Message['content'],
            __type: 'host_apply.submit',
            ...base,
          } as Message;
        }
      }

      return {
        role: MessageRole.Assistant,
        content: typeof content?.name === 'string' ? `活动消息：${content.name}` : '活动消息',
        ...base,
      } as Message;
    }

    const normalizedRole = (Object.values(MessageRole) as string[]).includes(role)
      ? (role as MessageRole)
      : MessageRole.Assistant;

    return {
      role: normalizedRole,
      content: (raw.content as Message['content']) ?? '',
      ...base,
    } as Message;
  };

  const streamChat = async (
    userMessages: { role: string; content: string }[],
    resumeValue?: string,
    forwardedProps?: Record<string, unknown>,
  ) => {
    const controller = new AbortController();
    abortController = controller;
    isChatting.value = true;

    try {
      const response = await agentApi.streamChat(
        sessionCode.value,
        userMessages,
        controller.signal,
        resumeValue,
        forwardedProps,
      );

      if (!response.ok) throw new Error(`HTTP ${response.status}`);

      const reader = response.body?.pipeThrough(new TextDecoderStream()).getReader();
      if (!reader) throw new Error('Response body is not readable');

      await readSSE(reader, (e) => event.handleEvent(e));
    } catch (err: unknown) {
      if (err instanceof Error && err.name !== 'AbortError') {
        console.error('[AiChat] stream error:', err);
        msg.messages.value.push({
          role: MessageRole.Assistant,
          content: (err as Error).message || '网络异常，请重试',
          id: genId(),
          messageId: genId(),
          status: MessageStatus.Error,
        } as Message);
      }
    } finally {
      // 只有当前流仍是 active 的才收尾，避免在已被新的流替换后误写状态
      if (abortController === controller) {
        const streaming = msg.getCurrentStreamingMessage();
        if (streaming) streaming.status = MessageStatus.Complete;
        ensureAssistantTail('已停止生成');
        isChatting.value = false;
        abortController = null;
      }
    }
  };

  // 纯本地中断当前 SSE（不通知后端）——用于会话切换 / 新建会话前的本地清理场景，
  // 避免上一会话的流继续写入共享 messages；让后端 run 自然跑完，结果进入 history 即可。
  const abortStream = () => {
    if (!isChatting.value) return;
    abortController?.abort();
  };

  // 用户主动点击"停止生成"：本地 abort + 通知后端 cancel run。
  // tail 占位 / 状态收尾由当前流（streamChat / fetchHistory）的 finally 统一处理，
  // 文案按场景匹配（/agui → "已停止生成"，/history → "未响应或被停止"）
  const stopGeneration = () => {
    if (!isChatting.value) return;
    abortController?.abort();
    if (sessionCode.value) {
      agentApi.cancelRun(sessionCode.value);
    }
  };

  // history 接口同样是 SSE：
  //   1) 先发 RUN_STARTED
  //   2) 发 MESSAGES_SNAPSHOT（历史消息大 JSON）
  //   3) 若存在断点续传，紧跟实时事件流（TEXT_MESSAGE_* / TOOL_CALL_* / THINKING_* …）
  //   4) 最后 RUN_FINISHED（后端发完不会主动关闭 SSE，需要前端 reader.cancel()）
  // 因此这里需要和 streamChat 一样做增量渲染 —— SNAPSHOT 到达立即 push 到 messages，
  // 后续实时事件复用 event.handleEvent 接着填充，而不是攒完整个流再返回。
  const fetchHistory = async (code: string, onSnapshotLoaded?: () => void): Promise<void> => {
    const controller = new AbortController();
    abortController = controller;
    isChatting.value = true;

    try {
      const response = await agentApi.fetchHistoryStream(code, controller.signal);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);

      const reader = response.body?.pipeThrough(new TextDecoderStream()).getReader();
      if (!reader) throw new Error('Response body is not readable');

      await readSSE(reader, (e) => {
        if (e.type === EventType.MessagesSnapshot) {
          const items = (e.messages as Record<string, unknown>[]) || [];
          for (const raw of items) {
            msg.messages.value.push(toHistoryMessage(raw));
          }
          // SNAPSHOT 内连续 user 消息（上次被中断 / 连续停止）每条都补占位
          fillSnapshotUserGaps('未响应或被停止');
          onSnapshotLoaded?.();
          return;
        }
        // 其他事件交给标准分发器，保持与 /agui 流一致的处理路径（增量渲染）
        event.handleEvent(e);
        // 收到 RUN_FINISHED 主动 cancel reader —— 后端发完事件不会关闭 SSE 连接
        if (e.type === EventType.RunFinished) return true;
      });
    } catch (err: unknown) {
      if (err instanceof Error && err.name !== 'AbortError') {
        throw err;
      }
    } finally {
      if (abortController === controller) {
        const streaming = msg.getCurrentStreamingMessage();
        if (streaming) streaming.status = MessageStatus.Complete;
        // 断点续传走完 / abort / 服务端关闭后，若 tail 仍是 user（resume 未产出 assistant 回复），
        // 追加一条错误态占位消息
        ensureAssistantTail('未响应或被停止', MessageStatus.Error);
        isChatting.value = false;
        abortController = null;
      }
    }
  };

  return { isChatting, sessionCode, streamChat, stopGeneration, abortStream, fetchHistory };
}

export type StreamModule = ReturnType<typeof useStream>;
