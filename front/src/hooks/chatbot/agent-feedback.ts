import type { Message } from '@blueking/chat-x';

import type { AgentFeedbackReaction } from '@/store/chatbot/feedback';

export const FEEDBACK_COMMENT_MAX_CHARS = 500;

export type AgentFeedbackMessage = Message & { __runId?: string };

export const pickRunId = (source: Record<string, unknown> | undefined): string => {
  if (!source || typeof source !== 'object') return '';
  const meta = source.metadata as Record<string, unknown> | undefined;
  const candidates = [source.runId, source.run_id, meta?.runId, meta?.run_id];
  const id = candidates.find((item) => typeof item === 'string' && item.trim());
  return typeof id === 'string' ? id.trim() : '';
};

/** /history SNAPSHOT 把一轮 run 边界序列化成 activity，content 上带原始 runId。 */
export const isSnapshotRunMarker = (raw: Record<string, unknown>): boolean =>
  raw.role === 'activity' && (raw.activityType === 'RUN_STARTED' || raw.activityType === 'RUN_FINISHED');

export const pickSnapshotActivityRunId = (raw: Record<string, unknown>): string => {
  const { content } = raw;
  if (content && typeof content === 'object' && !Array.isArray(content)) {
    return pickRunId(content as Record<string, unknown>);
  }
  return '';
};

export const getMessageRunId = (message: Message | undefined): string => {
  if (!message) return '';
  const id = (message as AgentFeedbackMessage).__runId;
  return typeof id === 'string' ? id.trim() : '';
};

export const stampMessageRunId = (message: Message | undefined, runId: string) => {
  if (!message || !runId || getMessageRunId(message)) return;
  (message as AgentFeedbackMessage).__runId = runId;
};

export const getGroupRunId = (messages: Message[]): string => {
  for (let i = messages.length - 1; i >= 0; i--) {
    const id = getMessageRunId(messages[i]);
    if (id) return id;
  }
  return '';
};

export interface FeedbackMessageGroup {
  uid: string;
  messages: Message[];
}

/** chat-x 仅 like/unlike 会把 ToolBtn 设为 is-active；取消评价时仍保持该 class 直到下一 tick。 */
export const findActiveFeedbackButton = (target: EventTarget | null): HTMLElement | null => {
  if (!(target instanceof Element)) return null;
  const btn = target.closest('.ai-tool-btn.is-active');
  return btn instanceof HTMLElement ? btn : null;
};

export const findGroupMessagesFromFeedbackButton = (btn: Element, groups: FeedbackMessageGroup[]): Message[] => {
  const groupEl = btn.closest('.message-group');
  if (!groupEl) return [];
  const uid = groupEl.getAttribute('data-message-group-id') || groupEl.id;
  if (!uid) return [];
  return groups.find((group) => group.uid === uid)?.messages ?? [];
};

export const toolIdToReaction = (toolId: string): AgentFeedbackReaction | null => {
  if (toolId === 'like') return 'like';
  if (toolId === 'unlike') return 'dislike';
  return null;
};

export const clipFeedbackComment = (comment: string): string =>
  Array.from(comment ?? '')
    .slice(0, FEEDBACK_COMMENT_MAX_CHARS)
    .join('');

export const invertTagMap = (map: Record<string, string>): Record<string, string> => {
  const inverted: Record<string, string> = {};
  Object.entries(map).forEach(([key, label]) => {
    if (label && inverted[label] === undefined) inverted[label] = key;
  });
  return inverted;
};

export const labelsToTagKeys = (labelToKey: Record<string, string>, labels: string[]): string[] => {
  const keys: string[] = [];
  labels.forEach((label) => {
    const key = labelToKey[label];
    if (key) keys.push(key);
  });
  return keys;
};
