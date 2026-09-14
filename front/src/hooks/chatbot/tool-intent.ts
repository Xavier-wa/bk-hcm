import { TOOL_INTENT_ARG, TOOL_INTENT_PREFIX } from './types';

export const isToolIntentId = (id: unknown): id is string =>
  typeof id === 'string' && id.startsWith(TOOL_INTENT_PREFIX);

export const isToolIntentHistoryItem = (raw: Record<string, unknown>): boolean =>
  isToolIntentId(raw.id) || isToolIntentId(raw.messageId);

export const extractMessageText = (content: unknown): string => {
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    return content
      .map((item) => {
        if (typeof item === 'string') return item;
        if (item && typeof item === 'object' && 'text' in item) return String((item as { text?: unknown }).text ?? '');
        return '';
      })
      .join('');
  }
  return '';
};

const asRecord = (value: unknown): Record<string, unknown> | null => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
};

const stringifyArgs = (raw: unknown): string => {
  if (typeof raw === 'string') return raw;
  if (raw == null) return '';
  try {
    return JSON.stringify(raw);
  } catch {
    return String(raw);
  }
};

// /agui 现网 TOOL_CALL_ARGS.delta 一次下发完整 JSON，前后可有空格，例如
// ` {"query":"...","tool_intent":"..."} `；少数通道会把 delta 先 parse 成对象。
export const appendToolCallDelta = (current: string, delta: unknown): string => {
  const piece = typeof delta === 'string' ? delta : delta && typeof delta === 'object' ? stringifyArgs(delta) : '';
  if (!piece) return current;
  return current ? `${current}${piece}` : piece;
};

export const trimToolCallArguments = (raw: unknown): string =>
  typeof raw === 'string' ? raw.trim() : stringifyArgs(raw);

// /history 快照里 arguments 与 /agui delta 同形：带首尾空格的完整 JSON 字符串
export const normalizeToolCallsArguments = (toolCalls: unknown): unknown => {
  if (!Array.isArray(toolCalls)) return toolCalls;
  return toolCalls.map((item) => {
    if (!item || typeof item !== 'object') return item;
    const call = item as { function?: Record<string, unknown> };
    if (!call.function) return item;
    return {
      ...call,
      function: {
        ...call.function,
        arguments: trimToolCallArguments(call.function.arguments),
      },
    };
  });
};

const readIntent = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

// 流式 TOOL_CALL_ARGS 拼完整 JSON 之前，先把已经闭合的 "tool_intent":"..." 抠出来，说明能跟着参数增量出现
const extractStreamedToolIntent = (text: string): string => {
  const match = new RegExp(`"${TOOL_INTENT_ARG}"\\s*:\\s*"((?:\\\\.|[^"\\\\])*)"`).exec(text);
  if (!match) return '';
  try {
    return readIntent(JSON.parse(`"${match[1]}"`));
  } catch {
    return readIntent(match[1]);
  }
};

// 说明文案来自工具参数 tool_intent；从展示用 arguments 里剥掉，避免详情区「参数」再画一遍。
// /agui 的 TOOL_CALL_ARGS.delta 与 /history 的 function.arguments 同形：
// 带首尾空格的完整 JSON 字符串（实测 ` {"skill":"...","tool_intent":"..."} `）。
// 两条路径共用这一套拆分；对象形态只作兜底。
export const splitToolCallArgs = (raw: unknown): { intent: string; arguments: string } => {
  const text = stringifyArgs(raw);
  const trimmed = text.trim();
  if (!trimmed) return { intent: '', arguments: text };

  try {
    const record = asRecord(JSON.parse(trimmed));
    if (record && TOOL_INTENT_ARG in record) {
      const intent = readIntent(record[TOOL_INTENT_ARG]);
      const rest = { ...record };
      delete rest[TOOL_INTENT_ARG];
      return { intent, arguments: Object.keys(rest).length ? JSON.stringify(rest) : '' };
    }
  } catch {
    const streamed = extractStreamedToolIntent(trimmed);
    if (streamed) return { intent: streamed, arguments: text };
  }
  return { intent: '', arguments: text };
};
