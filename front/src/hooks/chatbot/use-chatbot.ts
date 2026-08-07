import { computed, ref, onScopeDispose } from 'vue';
import { MessageRole, type Message } from '@blueking/chat-x';
import isEqual from 'lodash/isEqual';

import * as sessionApi from '@/store/chatbot/session';
import { useWhereAmI } from '@/hooks/useWhereAmI';
import { useMessage } from './use-message';
import { useEventHandler } from './use-event';
import { useStream } from './use-stream';
import { useSession } from './use-session';
import { useAccountSelect } from './use-account-select';
import { touchCardActionProtect } from './card-action-protect';
import {
  applyPendingResumeMeta,
  cardMetaRecordToMap,
  type CardClientMeta,
  restoreCardClientMeta,
  snapshotCommittedCardMetaFromMessages,
} from './card-client-meta';
import { type HostApplyRecommendMessage, type HostApplySuborder } from './types';

export type { ChatSession } from './types';
export { touchCardActionProtect } from './card-action-protect';

export const extractText = (content: unknown): string => {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content
    .filter((item) => typeof item === 'object' && item?.type === 'text' && item?.text)
    .map((item) => item.text as string)
    .join('');
};

// useChatbot 选项。sceneTag 为场景标识（如 host_apply）：
// 浮窗按场景挂载时传入，会话列表仅加载该场景，发送时默认按该场景创建会话；全页不传，加载全部。
export interface UseChatbotOptions {
  sceneTag?: string;
}

export function useChatbot(options: UseChatbotOptions = {}) {
  const { sceneTag = '' } = options;
  const { getBizsId } = useWhereAmI();
  const messageModule = useMessage();
  const eventModule = useEventHandler(messageModule);
  const streamModule = useStream(messageModule, eventModule);
  const sessionModule = useSession({
    messages: messageModule.messages,
    sessionCode: streamModule.sessionCode,
    getBkBizId: getBizsId,
    abortStream: streamModule.abortStream,
    fetchHistory: streamModule.fetchHistory,
    isChatting: streamModule.isChatting,
    sceneTag,
  });
  const accountSelectModule = useAccountSelect(messageModule.messages);

  // F-004 in-flight 窗口锁定：记录「他处正在操作中」的会话码集合。
  // 发起端在动作发起瞬间广播 session-busy，本端据此把同会话的 pending 可交互卡片立即锁定，
  // 杜绝「结果返回前（5-10s）其它标签页对过期卡片重复点击」。完成后由 session-updated 解锁。
  const remoteBusySessions = ref<Set<string>>(new Set());
  // 兜底定时器：发起端异常关闭/崩溃未发 session-updated 时，最多 20s 自动解锁，防止长期卡死。
  const busyTimers = new Map<string, ReturnType<typeof setTimeout>>();
  const BUSY_TTL = 20_000;

  const markSessionBusy = (code: string) => {
    if (!remoteBusySessions.value.has(code)) {
      const next = new Set(remoteBusySessions.value);
      next.add(code);
      remoteBusySessions.value = next;
    }
    clearTimeout(busyTimers.get(code));
    busyTimers.set(
      code,
      setTimeout(() => clearSessionBusy(code), BUSY_TTL),
    );
  };
  const clearSessionBusy = (code: string) => {
    clearTimeout(busyTimers.get(code));
    busyTimers.delete(code);
    if (!remoteBusySessions.value.has(code)) return;
    const next = new Set(remoteBusySessions.value);
    next.delete(code);
    remoteBusySessions.value = next;
  };
  // 当前选中会话是否正被他处操作（供 chat-message-list 下发 locked 给交互卡片）
  const isCurrentSessionRemoteBusy = computed(() =>
    remoteBusySessions.value.has(sessionModule.currentSessionCode.value),
  );

  // F-003/F-004 跨标签页/多入口同步：同机不同标签页或浮窗+全屏共享该频道。
  // - session-busy（F-004）：仅广播锁定信号，不同步卡片数据（未提交编辑不跨标签传播）。
  // - session-updated（F-003）：操作完成后广播「已提交只读态」cardMeta，其它实例刷新后合并最终数据。
  type SyncChannelPayload = {
    type?: string;
    sessionCode?: string;
    cardMeta?: Record<string, CardClientMeta>;
  };

  const syncChannel = typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('hcm-chatbot-sync') : null;
  if (syncChannel) {
    syncChannel.onmessage = (ev: MessageEvent) => {
      const data = ev.data as SyncChannelPayload | null;
      if (!data?.sessionCode) return;
      if (data.type === 'session-busy') {
        markSessionBusy(data.sessionCode);
        return;
      }
      if (data.type === 'session-updated') {
        clearSessionBusy(data.sessionCode);
        if (data.sessionCode !== sessionModule.currentSessionCode.value) return;
        const remoteMeta = cardMetaRecordToMap(data.cardMeta);
        void (async () => {
          // 被动刷新：受卡片操作保护窗口约束，且 SNAPSHOT 内原子合并本地 meta
          await sessionModule.refreshCurrentSession({ reason: 'passive' });
          restoreCardClientMeta(messageModule.messages.value, remoteMeta);
          sessionModule.saveCurrentSession();
        })();
      }
    };
  }
  const broadcastSessionBusy = (code: string) => {
    syncChannel?.postMessage({ type: 'session-busy', sessionCode: code });
  };
  const broadcastSessionUpdated = (code: string) => {
    syncChannel?.postMessage({
      type: 'session-updated',
      sessionCode: code,
      cardMeta: snapshotCommittedCardMetaFromMessages(messageModule.messages.value),
    });
  };
  onScopeDispose(() => {
    syncChannel?.close();
    busyTimers.forEach((timer) => clearTimeout(timer));
    busyTimers.clear();
  });

  const sendMessage = async (
    content: string,
    sessionTag = sceneTag,
    resumeValue?: string,
    forwardedProps?: Record<string, unknown>,
  ) => {
    // F-002：续跑/提交（存在 resumeValue）前先刷新当前会话，确保基于最新状态发起。
    // 仅作读一致性的轻量校验：刷新后照常提交，若他处已推进导致 409 仍走现有错误渲染路径，不做专门冲突处理。
    // 普通首轮发送不刷新，避免徒增一次 /history 往返。
    if (resumeValue && sessionModule.currentSession.value) {
      // active：续跑前一致性校验，不因保护窗口跳过；clientMeta 仍原子合并
      await sessionModule.refreshCurrentSession({ reason: 'active' });
      // F-002 刷新会替换 messages 对象，续跑前用 resumeValue 回填方案卡选中下标（与 card-client-meta 捕获互补）
      applyPendingResumeMeta(messageModule.messages.value, resumeValue);
    }

    // 首页空态（无选中会话）发送：先惰性创建/复用会话，避免清空消息时丢失刚加入的用户消息。
    // 创建失败（如无 bizId）时直接返回，不发送。
    // sessionTag 为场景标识（如 host_apply），通过 create_session 的 session_tag 入参传递。
    if (!sessionModule.currentSession.value) {
      await sessionModule.createSession(sessionTag);
      if (!sessionModule.currentSession.value) return;
    }
    messageModule.addUserMessage(content);
    const session = sessionModule.currentSession.value;
    if (session && session.sessionName === '新对话' && content.trim()) {
      const newName = content.trim().slice(0, 30);
      session.sessionName = newName;
      sessionApi.updateSession(getBizsId(), session.sessionCode, newName).catch(() => {});
    }
    // F-004：动作发起即广播 busy，其他持有同会话 pending 卡片的实例在结果返回前立即锁定，防重复点击。
    const busyCode = session?.sessionCode;
    if (busyCode) broadcastSessionBusy(busyCode);
    try {
      await streamModule.streamChat([{ role: 'user', content }], resumeValue, forwardedProps);
      sessionModule.saveCurrentSession();
      if (session) {
        session.sessionContentCount += 1;
        session.updatedAt = new Date().toISOString();
        sessionModule.moveSessionToTop(session.sessionCode);
      }
    } finally {
      // F-003/F-004：无论成功/失败/中断，都广播完成态解锁并触发其他实例刷新
      if (busyCode) broadcastSessionUpdated(busyCode);
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
    const busyCode = sessionModule.currentSessionCode.value;
    if (busyCode) broadcastSessionBusy(busyCode);
    try {
      await streamModule.streamChat([{ role: 'user', content: userContent }]);
      sessionModule.saveCurrentSession();
    } finally {
      if (busyCode) broadcastSessionUpdated(busyCode);
    }
  };

  const resendEdited = async (message: Message, newContent: unknown) => {
    if (streamModule.isChatting.value) return;
    const content = extractText(newContent);
    if (!content.trim()) return;

    const index = messageModule.messages.value.findIndex((m) => m.id === message.id);
    if (index === -1) return;

    messageModule.messages.value.splice(index);
    messageModule.addUserMessage(content);
    const busyCode = sessionModule.currentSessionCode.value;
    if (busyCode) broadcastSessionBusy(busyCode);
    try {
      await streamModule.streamChat([{ role: 'user', content }]);
      sessionModule.saveCurrentSession();
    } finally {
      if (busyCode) broadcastSessionUpdated(busyCode);
    }
  };

  // 「添加到配置清单」从选择方案步骤跳转后，关联会话打开时定位到跳转前所选方案：
  // 按 suborder 在最近一张方案推荐卡的候选中精确匹配（A 卡不可改，理应命中），写入 __initialIndex 定位展示。
  // 注意：仅定位、不设 __selectedIndex，卡片保持可交互（用户仍可选择/切换方案继续添加到清单）。
  const selectRecommendBySuborder = (suborder: HostApplySuborder): boolean => {
    for (let i = messageModule.messages.value.length - 1; i >= 0; i--) {
      const message = messageModule.messages.value[i] as HostApplyRecommendMessage;
      if (message.__type !== 'host_apply.recommend') continue;
      const recommendations = message.content?.value?.recommendations ?? [];
      const idx = recommendations.findIndex((item) => isEqual(item.suborder, suborder));
      if (idx >= 0) {
        message.__initialIndex = idx;
        return true;
      }
    }
    return false;
  };

  return {
    messages: messageModule.messages,
    isChatting: streamModule.isChatting,
    isCurrentSessionRemoteBusy,
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
    refreshCurrentSession: sessionModule.refreshCurrentSession,
    touchCardActionProtect,
    deleteSession: sessionModule.deleteSession,
    renameSession: sessionModule.renameSession,
    goHome: sessionModule.goHome,
    initSessions: sessionModule.initSessions,
    reloadSessions: sessionModule.reloadSessions,
    clearMessages: messageModule.clearMessages,
    selectedAccountEcho: accountSelectModule.selectedAccountEcho,
    selectRecommendBySuborder,
  };
}
