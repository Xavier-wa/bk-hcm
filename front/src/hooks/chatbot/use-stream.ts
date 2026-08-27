import { ref } from 'vue';
import { MessageRole, MessageStatus, type Message } from '@blueking/chat-x';

import * as agentApi from '@/store/chatbot/agent';
import {
  findRecommendIndexBySuborder,
  isSubmitResumePayload,
  restoreCardClientMeta,
  type CardClientMeta,
} from './card-client-meta';
import {
  EventType,
  HOST_APPLY_CONFIRM_EVENT,
  HOST_APPLY_RECOMMEND_EVENT,
  HOST_APPLY_RESUME_FORWARDED_EVENT,
  HOST_APPLY_SUBMIT_EVENT,
  type AccountSelectInterruptValue,
  type AccountSelectOption,
  type HitlInterruptValue,
  type HostApplyPreorderMessage,
  type HostApplyRecommendation,
  type HostApplyPreorderValue,
  type HostApplyRecommendMessage,
  type HostApplyRecommendValue,
  type HostApplySubmitMessage,
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
  // 标记本次流是否被用户主动停止（点击「停止」），用于收尾时保证展示「已停止生成」提示
  let stoppedByUser = false;

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

  // 用户主动停止后保证「已停止」提示可见（用 Error 态展示与「未响应或被停止」一致的提示图标）：
  //   - 末条为空内容助手气泡（如工具调用后正文未产出即被中断）→ 直接填充提示文案 + 置 Error 态，消除空白气泡；
  //   - 否则（正文已渲染、卡片消息、或末条为 user）→ 追加一条独立提示气泡。
  const ensureStopTip = (content: string) => {
    const last = msg.messages.value.at(-1);
    if (last?.role === MessageRole.Assistant && typeof last.content === 'string' && !last.content.trim()) {
      (last as { content: string; status: MessageStatus }).content = content;
      (last as { content: string; status: MessageStatus }).status = MessageStatus.Error;
      return;
    }
    msg.messages.value.push({
      role: MessageRole.Assistant,
      content,
      id: genId(),
      messageId: genId(),
      status: MessageStatus.Error,
    } as Message);
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

    // 历史思考记录默认折叠：chat-x ReasoningMessage 把 message 整包当 props，collapsed 缺省为 false；
    // history 快照通常不带 duration，无法走组件「有 duration 则自动折叠」的路径。
    const reasoningProps =
      normalizedRole === MessageRole.Reasoning
        ? {
            collapsed: true,
            ...(typeof raw.duration === 'number' && Number.isFinite(raw.duration) && raw.duration > 0
              ? { duration: raw.duration }
              : {}),
          }
        : {};

    return {
      role: normalizedRole,
      content: (raw.content as Message['content']) ?? '',
      ...base,
      ...reasoningProps,
    } as Message;
  };

  // 可渲染为卡片的已知 CUSTOM activity name；其余 activity（ACTIVITY_DELTA、resume_forwarded 等）属图执行内部态，不渲染
  const RENDERABLE_CUSTOM_NAMES = new Set<string>([
    'hitl.interrupt',
    'account_select.interrupt',
    HOST_APPLY_RECOMMEND_EVENT,
    HOST_APPLY_CONFIRM_EVENT,
    HOST_APPLY_SUBMIT_EVENT,
  ]);

  const getActivityName = (raw: Record<string, unknown>): string | undefined =>
    (raw.content as Record<string, unknown> | undefined)?.name as string | undefined;

  // 是否为 resume 转发回执（仅用于回放恢复选择/确认，不渲染）
  const isResumeForwarded = (raw: Record<string, unknown>): boolean =>
    raw.role === 'activity' &&
    raw.activityType === 'CUSTOM' &&
    getActivityName(raw) === HOST_APPLY_RESUME_FORWARDED_EVENT;

  // 是否为不渲染的内部 activity：非 CUSTOM（ACTIVITY_DELTA 等）或未在白名单的 CUSTOM
  const isInternalActivity = (raw: Record<string, unknown>): boolean => {
    if (raw.role !== 'activity') return false;
    if (raw.activityType !== 'CUSTOM') return true;
    return !RENDERABLE_CUSTOM_NAMES.has(getActivityName(raw) ?? '');
  };

  // 解析 resume_forwarded 的 payload.value。兼容历史调试前缀（如「帮我基于此方案进行拆单{...}」），从首个 { 或 [ 起截取 JSON。
  const parseResumeForwardedPayload = (raw: Record<string, unknown>): unknown => {
    const content = raw.content as Record<string, unknown> | undefined;
    const value = (content?.value as Record<string, unknown> | undefined)?.payload as
      | Record<string, unknown>
      | undefined;
    const text = value?.value;
    if (typeof text !== 'string') return null;
    const start = text.search(/[[{]/);
    if (start < 0) return null;
    try {
      return JSON.parse(text.slice(start));
    } catch {
      return null;
    }
  };

  // 用 resume_forwarded 回执回放恢复最近一张待确认卡片的选择/确认：
  //   - 数组 → 模板 B（预提单，可修改）：用确认时的 suborders 覆盖展示内容
  //   - 对象 → 模板 A（方案推荐，不可修改）：匹配候选得到选中下标，未命中兜底首条
  const applyResumeForwarded = (
    raw: Record<string, unknown>,
    lastRecommend: HostApplyRecommendMessage | undefined,
    lastPreorder: HostApplyPreorderMessage | undefined,
    lastSubmit: HostApplySubmitMessage | undefined,
  ): void => {
    const payload = parseResumeForwardedPayload(raw);
    if (!payload) return;

    const suborders = Array.isArray(payload) ? payload.filter((item) => item && typeof item === 'object') : [];
    if (suborders.length > 0) {
      if (lastPreorder) lastPreorder.__confirmedSuborders = suborders as HostApplySuborder[];
      return;
    }

    if (isSubmitResumePayload(payload)) {
      if (lastSubmit) lastSubmit.__submitted = true;
      return;
    }

    if (typeof payload === 'object' && lastRecommend) {
      const recommendations = lastRecommend.content?.value?.recommendations ?? [];
      const idx = findRecommendIndexBySuborder(recommendations, payload);
      if (idx >= 0) lastRecommend.__selectedIndex = idx;
    }
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
        // 用户主动停止才保证「已停止生成」可见（含空白气泡填充）；正常结束不额外插入提示
        if (stoppedByUser) ensureStopTip('已停止生成');
        stoppedByUser = false;
        isChatting.value = false;
        abortController = null;
      }
    }
  };

  // 纯本地中断当前 SSE（不通知后端）——用于会话切换 / 新建会话前的本地清理场景，
  // 避免上一会话的流继续写入共享 messages；让后端 run 自然跑完，结果进入 history 即可。
  // 不依赖 isChatting：纯历史 SNAPSHOT 拉取阶段也可能持有 abortController，需能被打断。
  const abortStream = () => {
    abortController?.abort();
  };

  // 用户主动点击"停止生成"：本地 abort + 通知后端 cancel run。
  // tail 占位 / 状态收尾由当前流（streamChat / fetchHistory）的 finally 统一处理，
  // 文案按场景匹配（/agui → "已停止生成"，/history → "未响应或被停止"）
  const stopGeneration = () => {
    if (!isChatting.value) return;
    // 标记为用户主动停止，收尾时保证展示「已停止生成」（覆盖空白气泡场景）
    stoppedByUser = true;
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
  //
  // isChatting 仅在 SNAPSHOT 之后出现续传实时事件时才置 true：
  // 纯历史加载不是「生成中」，否则切回对话页 / 拉历史时会闪现「停止生成」按钮。
  // opts.clientMeta：在一次性赋值前还原卡片运行态，避免 messages=[] 中间帧闪回可点旧卡。
  const fetchHistory = async (
    code: string,
    onSnapshotLoaded?: () => void,
    opts?: { clientMeta?: Map<string, CardClientMeta> },
  ): Promise<void> => {
    const controller = new AbortController();
    abortController = controller;
    let snapshotDone = false;

    try {
      const response = await agentApi.fetchHistoryStream(code, controller.signal);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);

      const reader = response.body?.pipeThrough(new TextDecoderStream()).getReader();
      if (!reader) throw new Error('Response body is not readable');

      await readSSE(reader, (e) => {
        if (e.type === EventType.MessagesSnapshot) {
          const items = (e.messages as Record<string, unknown>[]) || [];
          // 离屏构建完整列表后再一次赋值：禁止 messages=[] 的可见中间态（获焦闪卡根因）
          const next: Message[] = [];
          // 边遍历边记录最近一张方案推荐卡 / 预提单卡，遇到 resume_forwarded 回执即回填选择/确认内容；
          // 内部 activity（ACTIVITY_DELTA、resume_forwarded 等）不渲染为气泡。
          let lastRecommend: HostApplyRecommendMessage | undefined;
          let lastPreorder: HostApplyPreorderMessage | undefined;
          let lastSubmit: HostApplySubmitMessage | undefined;
          for (const raw of items) {
            if (isResumeForwarded(raw)) {
              applyResumeForwarded(raw, lastRecommend, lastPreorder, lastSubmit);
              continue;
            }
            if (isInternalActivity(raw)) continue;

            const message = toHistoryMessage(raw);
            next.push(message);
            if ((message as HostApplyRecommendMessage).__type === 'host_apply.recommend') {
              lastRecommend = message as HostApplyRecommendMessage;
            } else if ((message as HostApplyPreorderMessage).__type === 'host_apply.preorder') {
              lastPreorder = message as HostApplyPreorderMessage;
            } else if ((message as HostApplySubmitMessage).__type === 'host_apply.submit') {
              lastSubmit = message as HostApplySubmitMessage;
            }
          }
          // 同步帧内完成：赋值 → 补占位 → 还原本地卡片态（Vue 不会在中途刷 DOM）
          msg.messages.value = next;
          fillSnapshotUserGaps('未响应或被停止');
          if (opts?.clientMeta?.size) {
            restoreCardClientMeta(msg.messages.value, opts.clientMeta);
          }
          snapshotDone = true;
          onSnapshotLoaded?.();
          return;
        }
        // SNAPSHOT 后的非 RunFinished 事件 = 断点续传，此时才进入「生成中」
        if (snapshotDone && e.type !== EventType.RunFinished && !isChatting.value) {
          isChatting.value = true;
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
