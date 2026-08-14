import isEqual from 'lodash/isEqual';
import cloneDeep from 'lodash/cloneDeep';
import { MessageRole, type Message } from '@blueking/chat-x';

import {
  type AccountSelectInterruptMessage,
  type HostApplyPreorderMessage,
  type HostApplyRecommendation,
  type HostApplyRecommendMessage,
  type HostApplySuborder,
  type HostApplySubmitMessage,
} from './types';

/** 卡片运行态字段（不进 MESSAGES_SNAPSHOT），刷新历史前捕获、刷新后按 messageId 写回 */
export interface CardClientMeta {
  selectedIndex?: number;
  initialIndex?: number;
  confirmedSuborders?: HostApplySuborder[];
  submitted?: boolean;
  selectedAccountId?: string;
}

const metaKey = (message: Message): string | undefined => message.messageId || message.id;

const hasFollowingUserMessage = (messages: Message[], message: Message): boolean => {
  const idx = messages.findIndex((item) => item.id === message.id);
  if (idx < 0) return false;
  const next = messages[idx + 1];
  return !!next && next.role === MessageRole.User;
};

/** 本端 refresh 用：捕获全部运行态（含续跑前刚写入、尚未落库的字段） */
export const captureCardClientMeta = (messages: Message[]): Map<string, CardClientMeta> => {
  const map = new Map<string, CardClientMeta>();
  for (const message of messages) {
    const key = metaKey(message);
    if (!key) continue;

    const meta: CardClientMeta = {};
    const rec = message as HostApplyRecommendMessage;
    if (rec.__type === 'host_apply.recommend') {
      if (rec.__selectedIndex !== undefined) meta.selectedIndex = rec.__selectedIndex;
      if (rec.__initialIndex !== undefined) meta.initialIndex = rec.__initialIndex;
    }
    const preorder = message as HostApplyPreorderMessage;
    if (preorder.__type === 'host_apply.preorder' && preorder.__confirmedSuborders) {
      meta.confirmedSuborders = preorder.__confirmedSuborders;
    }
    const submit = message as HostApplySubmitMessage;
    if (submit.__type === 'host_apply.submit' && submit.__submitted) {
      meta.submitted = true;
    }
    const account = message as AccountSelectInterruptMessage;
    if (account.__type === 'account_select.interrupt' && account.__selectedAccountId) {
      meta.selectedAccountId = account.__selectedAccountId;
    }

    if (Object.keys(meta).length > 0) map.set(key, meta);
  }
  return map;
};

/**
 * 跨标签广播用：仅捕获「操作已完成 → 卡片已进入只读态」的最终数据。
 * 未提交的本地编辑（如预提单 C 弹窗改完未点确认）不同步。
 */
export const captureCommittedCardClientMeta = (messages: Message[]): Map<string, CardClientMeta> => {
  const map = new Map<string, CardClientMeta>();
  for (const message of messages) {
    const key = metaKey(message);
    if (!key) continue;
    if (!hasFollowingUserMessage(messages, message)) continue;

    const meta: CardClientMeta = {};
    const rec = message as HostApplyRecommendMessage;
    if (rec.__type === 'host_apply.recommend' && rec.__selectedIndex !== undefined) {
      meta.selectedIndex = rec.__selectedIndex;
    }
    const preorder = message as HostApplyPreorderMessage;
    if (preorder.__type === 'host_apply.preorder' && preorder.__confirmedSuborders) {
      meta.confirmedSuborders = preorder.__confirmedSuborders;
    }
    const submit = message as HostApplySubmitMessage;
    if (submit.__type === 'host_apply.submit' && submit.__submitted) {
      meta.submitted = true;
    }
    const account = message as AccountSelectInterruptMessage;
    if (account.__type === 'account_select.interrupt' && account.__selectedAccountId) {
      meta.selectedAccountId = account.__selectedAccountId;
    }

    if (Object.keys(meta).length > 0) map.set(key, meta);
  }
  return map;
};

export const restoreCardClientMeta = (messages: Message[], map: Map<string, CardClientMeta>): void => {
  if (map.size === 0) return;
  for (const message of messages) {
    const key = metaKey(message);
    if (!key) continue;
    const meta = map.get(key);
    if (!meta) continue;

    const rec = message as HostApplyRecommendMessage;
    if (rec.__type === 'host_apply.recommend') {
      if (meta.selectedIndex !== undefined) rec.__selectedIndex = meta.selectedIndex;
      if (meta.initialIndex !== undefined) rec.__initialIndex = meta.initialIndex;
    }
    const preorder = message as HostApplyPreorderMessage;
    if (preorder.__type === 'host_apply.preorder' && meta.confirmedSuborders) {
      preorder.__confirmedSuborders = cloneDeep(meta.confirmedSuborders);
    }
    const submit = message as HostApplySubmitMessage;
    if (submit.__type === 'host_apply.submit' && meta.submitted) {
      submit.__submitted = true;
    }
    const account = message as AccountSelectInterruptMessage;
    if (account.__type === 'account_select.interrupt' && meta.selectedAccountId) {
      account.__selectedAccountId = meta.selectedAccountId;
    }
  }
};

/** BroadcastChannel 序列化：Map → 普通对象（深拷贝，避免引用泄漏） */
export const cardMetaMapToRecord = (map: Map<string, CardClientMeta>): Record<string, CardClientMeta> => {
  const record: Record<string, CardClientMeta> = {};
  map.forEach((meta, key) => {
    record[key] = cloneDeep(meta);
  });
  return record;
};

/** BroadcastChannel 反序列化 */
export const cardMetaRecordToMap = (record?: Record<string, CardClientMeta>): Map<string, CardClientMeta> => {
  if (!record) return new Map();
  return new Map(Object.entries(record).map(([key, meta]) => [key, cloneDeep(meta)]));
};

export const snapshotCommittedCardMetaFromMessages = (messages: Message[]): Record<string, CardClientMeta> =>
  cardMetaMapToRecord(captureCommittedCardClientMeta(messages));

const normalizeSuborderScalars = (suborder: HostApplySuborder): HostApplySuborder => ({
  ...suborder,
  require_type:
    suborder.require_type !== undefined && suborder.require_type !== null ? String(suborder.require_type) : undefined,
  res_assign:
    suborder.res_assign !== undefined && suborder.res_assign !== null ? String(suborder.res_assign) : undefined,
});

/** 宽松匹配 suborder：兼容 require_type 数字/字符串差异、resume payload 字段缺失 */
export const subordersLooselyMatch = (a: HostApplySuborder, b: HostApplySuborder): boolean => {
  if (isEqual(a, b)) return true;
  if (isEqual(normalizeSuborderScalars(a), normalizeSuborderScalars(b))) return true;

  const keyFields: (keyof HostApplySuborder)[] = [
    'device_type',
    'region',
    'zone',
    'image_id',
    'replicas',
    'charge_type',
    'require_type',
  ];
  const definedOnBoth = keyFields.filter(
    (k) => a[k] !== undefined && a[k] !== null && b[k] !== undefined && b[k] !== null,
  );
  if (definedOnBoth.length === 0) return false;
  return definedOnBoth.every((k) => String(a[k]) === String(b[k]));
};

const extractSuborderCandidates = (payload: unknown): HostApplySuborder[] => {
  if (!payload || typeof payload !== 'object') return [];
  const record = payload as Record<string, unknown>;
  if (record.suborder && typeof record.suborder === 'object') {
    return [record.suborder as HostApplySuborder, payload as HostApplySuborder];
  }
  return [payload as HostApplySuborder];
};

/** 在方案推荐候选中定位 payload 对应的下标；未命中返回 -1（不再兜底 0） */
export const findRecommendIndexBySuborder = (recommendations: HostApplyRecommendation[], payload: unknown): number => {
  for (const candidate of extractSuborderCandidates(payload)) {
    const idx = recommendations.findIndex((item) => subordersLooselyMatch(item.suborder, candidate));
    if (idx >= 0) return idx;
  }
  return -1;
};

/** 确认提交 resume payload：含 body_param（整单 data 对象） */
export const isSubmitResumePayload = (payload: unknown): boolean => {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return false;
  const record = payload as Record<string, unknown>;
  if (record.body_param && typeof record.body_param === 'object') return true;
  const { data } = record;
  if (data && typeof data === 'object') {
    const body = (data as Record<string, unknown>).body_param;
    return !!body && typeof body === 'object';
  }
  return false;
};

const parseSubordersArray = (payload: unknown): HostApplySuborder[] => {
  if (!Array.isArray(payload)) return [];
  return payload.filter((item) => item && typeof item === 'object') as HostApplySuborder[];
};

/** F-002 续跑前刷新后：用本次 resumeValue 回填各卡片运行态（方案 / 预提单 / 确认提交） */
export const applyPendingResumeMeta = (messages: Message[], resumeValue: string): void => {
  let payload: unknown;
  try {
    payload = JSON.parse(resumeValue);
  } catch {
    return;
  }

  const suborders = parseSubordersArray(payload);
  if (suborders.length > 0) {
    for (let i = messages.length - 1; i >= 0; i--) {
      const message = messages[i] as HostApplyPreorderMessage;
      if (message.__type !== 'host_apply.preorder') continue;
      if (message.__confirmedSuborders) return;
      message.__confirmedSuborders = cloneDeep(suborders);
      return;
    }
    return;
  }

  if (isSubmitResumePayload(payload)) {
    for (let i = messages.length - 1; i >= 0; i--) {
      const message = messages[i] as HostApplySubmitMessage;
      if (message.__type !== 'host_apply.submit') continue;
      if (message.__submitted) return;
      message.__submitted = true;
      return;
    }
    return;
  }

  if (payload && typeof payload === 'object') {
    for (let i = messages.length - 1; i >= 0; i--) {
      const message = messages[i] as HostApplyRecommendMessage;
      if (message.__type !== 'host_apply.recommend') continue;
      if (message.__selectedIndex !== undefined) return;

      const recommendations = message.content?.value?.recommendations ?? [];
      const idx = findRecommendIndexBySuborder(recommendations, payload);
      if (idx >= 0) {
        message.__selectedIndex = idx;
        return;
      }
    }
  }
};
