import { computed, ref, type Ref } from 'vue';
import { MessageRole, MessageStatus, type Message, type ToolCall } from '@blueking/chat-x';

import {
  PROCESS_SUMMARY_TYPE,
  PROCESS_THINKING_TYPE,
  PROCESS_TOOL_TYPE,
  type ProcessSummaryMessage,
  type ProcessThinkingMessage,
  type ProcessToolCall,
  type ProcessToolMessage,
} from './types';
import { getMessageRunId } from './agent-feedback';
import { extractMessageText, splitToolCallArgs } from './tool-intent';

type ToolResultMessage = Message & { toolCallId?: string; duration?: number };

const collectToolResults = (messages: Message[]) => {
  const map = new Map<string, ToolResultMessage>();
  messages.forEach((message) => {
    const { toolCallId } = message as ToolResultMessage;
    if (message.role === MessageRole.Tool && toolCallId) map.set(toolCallId, message as ToolResultMessage);
  });
  return map;
};

const toProcessToolMessage = (
  message: Message,
  results: Map<string, ToolResultMessage>,
  visible: boolean,
  live: boolean,
  streaming: boolean,
  isRowExpanded: (toolCallId: string, defaultExpanded: boolean) => boolean,
): ProcessToolMessage => {
  const rawCalls = ((message as { toolCalls?: ToolCall[] }).toolCalls ?? []) as ProcessToolCall[];
  const toolCalls = rawCalls.map((toolCall) => {
    const { intent, arguments: args } = splitToolCallArgs(toolCall.function?.arguments);
    return {
      ...toolCall,
      function: { ...toolCall.function, arguments: args },
      toolMessage: results.get(toolCall.id) ?? toolCall.toolMessage,
      intent,
    };
  });
  const rowExpanded: Record<string, boolean> = {};
  toolCalls.forEach((toolCall) => {
    // 默认展开态按「这一次调用」算，不按整轮算：拿到结果就即时收起，不等本轮跑完。
    // 判据与行内三态里的 running 完全一致（见 process-tool-row 的 resolveState）——展开的永远只有正在转圈的那行
    rowExpanded[toolCall.id] = isRowExpanded(toolCall.id, streaming && !toolCall.toolMessage);
  });
  const runId = getMessageRunId(message);
  return {
    role: MessageRole.Assistant,
    content: '',
    id: message.id,
    messageId: message.messageId,
    status: message.status,
    __type: PROCESS_TOOL_TYPE,
    __intent: extractMessageText(message.content),
    __toolCalls: toolCalls,
    __visible: visible,
    __live: live,
    __streaming: streaming,
    __rowExpanded: rowExpanded,
    ...(runId ? { __runId: runId } : {}),
  } as ProcessToolMessage;
};

// 无耗时数据时返回 undefined（历史回放），交由展示层省掉耗时段，不退化成 0
const resolveThinkingDuration = (message: Message): number | undefined => {
  const { duration } = message as { duration?: number };
  if (typeof duration === 'number' && Number.isFinite(duration) && duration >= 0) return duration;
  return undefined;
};

const toProcessThinkingMessage = (
  message: Message,
  visible: boolean,
  expanded: boolean,
  live: boolean,
): ProcessThinkingMessage => {
  const runId = getMessageRunId(message);
  return {
    role: MessageRole.Assistant,
    // 思考正文放 __text 而不是 content：这条消息常驻列表，chat-x 的「复制」会拼接组内非 reasoning 消息的 content
    content: '',
    id: message.id,
    messageId: message.messageId,
    status: message.status,
    __type: PROCESS_THINKING_TYPE,
    __text: extractMessageText(message.content),
    __duration: resolveThinkingDuration(message),
    __visible: visible,
    __live: live,
    __expanded: expanded,
    ...(runId ? { __runId: runId } : {}),
  } as ProcessThinkingMessage;
};

const isProcessBody = (message: Message): boolean => {
  if ((message as ProcessSummaryMessage).__type === PROCESS_SUMMARY_TYPE) return false;
  if ((message as ProcessToolMessage).__type === PROCESS_TOOL_TYPE) return true;
  if ((message as ProcessThinkingMessage).__type === PROCESS_THINKING_TYPE) return true;
  if (message.role === MessageRole.Reasoning) return true;
  return message.role === MessageRole.Assistant && !!(message as { toolCalls?: ToolCall[] }).toolCalls?.length;
};

export const isProcessSummaryMessage = (message: Message): message is ProcessSummaryMessage =>
  (message as ProcessSummaryMessage).__type === PROCESS_SUMMARY_TYPE;

export const isProcessToolMessage = (message: Message): message is ProcessToolMessage =>
  (message as ProcessToolMessage).__type === PROCESS_TOOL_TYPE;

export const isProcessThinkingMessage = (message: Message): message is ProcessThinkingMessage =>
  (message as ProcessThinkingMessage).__type === PROCESS_THINKING_TYPE;

// durationMs 为 undefined 表示本轮拿不到任何思考耗时（历史回放），摘要里省掉「思考耗时」
export const formatProcessSummary = (toolCount: number, durationMs?: number): string => {
  const tools = toolCount > 0 ? `调用${toolCount}个工具` : '';
  if (durationMs === undefined) return tools || '已完成思考';
  const seconds = (Math.max(0, durationMs) / 1000).toFixed(2);
  return tools ? `${tools}，思考耗时${seconds}s` : `思考耗时${seconds}s`;
};

const summarizeProcess = (process: Message[]) => {
  let toolCount = 0;
  let durationMs: number | undefined;
  process.forEach((message) => {
    if (message.role === MessageRole.Reasoning) {
      const duration = resolveThinkingDuration(message);
      if (duration !== undefined) durationMs = (durationMs ?? 0) + duration;
      return;
    }
    if (isProcessToolMessage(message)) {
      toolCount += message.__toolCalls.length;
      return;
    }
    toolCount += (message as { toolCalls?: ToolCall[] }).toolCalls?.length ?? 0;
  });
  return { toolCount, durationMs, text: formatProcessSummary(toolCount, durationMs) };
};

const splitTurns = (messages: Message[]): Message[][] => {
  const turns: Message[][] = [];
  let current: Message[] = [];
  messages.forEach((message) => {
    if (message.role === MessageRole.User) {
      if (current.length) turns.push(current);
      current = [message];
      return;
    }
    current.push(message);
  });
  if (current.length) turns.push(current);
  return turns;
};

export const useProcessZone = (
  messages: Ref<Message[]>,
  isChatting: Ref<boolean>,
  stayProcessExpanded: Ref<boolean>,
) => {
  const userExpanded = ref<Record<string, boolean>>({});
  // 工具行 / 思考正文各自的展开态，key 见 thinkingKey / toolRowKey。
  // 放在 hook 里而不是组件内：折叠态不能随组件重建（流式期间条目类型错位会重挂载）被重置。
  const detailExpanded = ref<Record<string, boolean>>({});

  // key 带上「本轮是否仍在进行」：进行中与结束后各存一份展开态。
  // 于是本轮结束时 key 整体切到 done 段、落回默认收起 —— 思考/工具正文可能很长，
  // 结束后再展开总开关不该糊出一大片；而结束后用户自己点开的展开态稳定保留在 done 段里。
  // 总开关（摘要行）只改 userExpanded，不影响这里的分段与默认值，内部折叠态因此与它解耦。
  const detailKey = (id: string, live: boolean) => `${id}@${live ? 'live' : 'done'}`;
  const thinkingKey = (message: Message, live: boolean) => detailKey(`thinking:${String(message.id)}`, live);
  const toolRowKey = (toolCallId: string, live: boolean) => detailKey(`tool:${toolCallId}`, live);

  // 默认态按「这一条自身是否还在进行」算（见各调用点的 defaultExpanded），不按整轮算：
  // 流式过程中同一时刻只展开正在跑的那条，前一条一结束就即时收起，读的时候不用在长正文里找当前进度。
  // 分段仍是整轮粒度，两者不冲突：结束前的手动展开留在 live 段，整轮结束时随 key 切段一起作废。
  const isDetailExpanded = (key: string, defaultExpanded: boolean) => detailExpanded.value[key] ?? defaultExpanded;

  const isTurnExpanded = (turnKey: string, isLastTurn: boolean) => {
    if (turnKey in userExpanded.value) return userExpanded.value[turnKey];
    if (isLastTurn && (isChatting.value || stayProcessExpanded.value)) return true;
    return false;
  };

  const displayMessages = computed(() => {
    const turns = splitTurns(messages.value);
    const out: Message[] = [];
    turns.forEach((turn, index) => {
      const process = turn.filter(isProcessBody);
      if (!process.length) {
        out.push(...turn.filter((message) => message.role !== MessageRole.Tool));
        return;
      }
      const results = collectToolResults(turn);
      const turnKey = `${index}:${String(process[0].id)}`;
      const isLastTurn = index === turns.length - 1;
      const expanded = isTurnExpanded(turnKey, isLastTurn);
      // 本轮是否仍在进行：流式中、或用户点了停止（stayProcessExpanded）。决定内部折叠态存取哪一段，见 detailKey
      const live = isLastTurn && (isChatting.value || stayProcessExpanded.value);
      // 仅「真的还在流式」：停止后工具不会再返回，未拿到结果的行不该继续转圈
      const streaming = isLastTurn && isChatting.value;
      const { text } = summarizeProcess(process);
      const summaryRunId = process.map(getMessageRunId).find(Boolean) ?? '';
      const summary = {
        role: MessageRole.Assistant,
        content: text,
        id: `process-summary-${turnKey}`,
        messageId: `process-summary-${turnKey}`,
        status: process.some((item) => item.status === MessageStatus.Streaming)
          ? MessageStatus.Streaming
          : MessageStatus.Complete,
        __type: PROCESS_SUMMARY_TYPE,
        __turnKey: turnKey,
        ...(summaryRunId ? { __runId: summaryRunId } : {}),
      } as ProcessSummaryMessage;
      let placedSummary = false;
      turn.forEach((message) => {
        if (message.role === MessageRole.Tool) return;
        if (isProcessBody(message)) {
          if (!placedSummary) {
            out.push(summary);
            placedSummary = true;
          }
          // 收起时不把工具行/思考从列表里摘掉，只标记不可见：
          // chat-x 的消息 v-for 用下标做 key（key: index），中途增删会让插入点之后的每个位置换成另一条消息，
          // 整轮内容随之拆掉重建，表现为展开/收起时正文闪动、组件状态被重置。
          if (message.role === MessageRole.Assistant && (message as { toolCalls?: ToolCall[] }).toolCalls?.length) {
            out.push(
              toProcessToolMessage(message, results, expanded, live, streaming, (toolCallId, defaultExpanded) =>
                isDetailExpanded(toolRowKey(toolCallId, live), defaultExpanded),
              ),
            );
          } else if (message.role === MessageRole.Reasoning) {
            const key = thinkingKey(message, live);
            // 这一段推理自己还在吐字才默认展开；REASONING_END 把 status 置 Complete 后即时收起。
            // 叠一层 streaming：点了停止后 status 可能停在 Streaming，不该继续摊开
            const active = streaming && message.status === MessageStatus.Streaming;
            out.push(toProcessThinkingMessage(message, expanded, isDetailExpanded(key, active), live));
          } else {
            out.push(message);
          }
          return;
        }
        out.push(message);
      });
    });
    return out;
  });

  const toggleTurn = (turnKey: string) => {
    userExpanded.value = { ...userExpanded.value, [turnKey]: !isTurnExpanded(turnKey, false) };
  };

  // 写回渲染这条消息时所用的那一段（__live），避免进行中的点击被记到 done 段、结束后又冒出来
  const toggleThinking = (message: ProcessThinkingMessage) => {
    const key = thinkingKey(message, message.__live);
    detailExpanded.value = { ...detailExpanded.value, [key]: !message.__expanded };
  };

  const toggleToolRow = (message: ProcessToolMessage, toolCallId: string) => {
    const key = toolRowKey(toolCallId, message.__live);
    detailExpanded.value = { ...detailExpanded.value, [key]: !message.__rowExpanded[toolCallId] };
  };

  const isSummaryExpanded = (turnKey: string) => {
    const turns = splitTurns(messages.value);
    const lastKey = (() => {
      const last = turns[turns.length - 1]?.filter(isProcessBody)[0];
      if (!last) return '';
      return `${turns.length - 1}:${String(last.id)}`;
    })();
    return isTurnExpanded(turnKey, turnKey === lastKey);
  };

  return { displayMessages, toggleTurn, isSummaryExpanded, toggleThinking, toggleToolRow };
};
