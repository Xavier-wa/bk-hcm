import { computed, ref, type Ref } from 'vue';
import type { Message } from '@blueking/chat-x';

import * as sessionApi from '@/store/chatbot/session';
import type { SessionApiItem } from '@/store/chatbot/session';
import type { ChatSession } from './types';

interface SessionDeps {
  messages: Ref<Message[]>;
  sessionCode: Ref<string>;
  getBkBizId: () => number;
  // 会话切换 / 新建时仅做本地 abort，不调用后端 /cancel —— 主动取消仅由用户点击"停止"触发
  abortStream: () => void;
  // 注意：fetchHistory 现在采用增量渲染，直接写入 deps.messages，不再返回消息数组
  fetchHistory: (sessionCode: string, onSnapshotLoaded?: () => void) => Promise<void>;
}

const toSession = (item: SessionApiItem): ChatSession => ({
  sessionCode: item.session_code,
  sessionName: item.session_name || '新对话',
  sessionContentCount: item.session_content_count,
  sessionTag: item.session_tag?.trim() || undefined,
  createdAt: item.created_at,
  updatedAt: item.updated_at,
  messages: [],
});

const sortByUpdatedAtDesc = (list: ChatSession[]) =>
  [...list].sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime());

export function useSession(deps: SessionDeps) {
  const sessions = ref<ChatSession[]>([]);
  const currentSessionCode = ref('');
  const isLoadingHistory = ref(false);
  const isLoadingSessions = ref(false);

  const currentSession = computed(() => sessions.value.find((s) => s.sessionCode === currentSessionCode.value));

  const resolveBkBizId = () => {
    const bkBizId = deps.getBkBizId();
    return Number.isFinite(bkBizId) && bkBizId > 0 ? bkBizId : 0;
  };

  const saveCurrentSession = () => {
    const session = currentSession.value;
    if (session) {
      session.messages = [...deps.messages.value];
    }
  };

  const applySession = (code: string) => {
    currentSessionCode.value = code;
    deps.sessionCode.value = code;
  };

  const loadSessions = async () => {
    const bkBizId = resolveBkBizId();
    if (!bkBizId) {
      sessions.value = [];
      return;
    }

    isLoadingSessions.value = true;
    try {
      const res = await sessionApi.listSessions(bkBizId);
      sessions.value = sortByUpdatedAtDesc(res.details.map(toSession));
    } finally {
      isLoadingSessions.value = false;
    }
  };

  const createSession = async (sessionTag = '') => {
    const bkBizId = resolveBkBizId();
    if (!bkBizId) return;

    // 带场景 tag 时不复用无 tag 的空会话，直接新建带 tag 的会话，确保归入对应文件夹
    const emptySession = sessionTag
      ? undefined
      : sessions.value.find((s) => s.messages.length === 0 && s.sessionContentCount === 0);
    if (emptySession) {
      if (emptySession.sessionCode === currentSessionCode.value) return;
      deps.abortStream();
      saveCurrentSession();
      const idx = sessions.value.indexOf(emptySession);
      if (idx > 0) {
        sessions.value.splice(idx, 1);
        sessions.value.unshift(emptySession);
      }
      applySession(emptySession.sessionCode);
      deps.messages.value = [];
      return;
    }

    deps.abortStream();
    saveCurrentSession();

    try {
      const res = await sessionApi.createSession(bkBizId, '新对话', sessionTag);
      const session: ChatSession = {
        sessionCode: res.session_code,
        sessionName: res.session_name || '新对话',
        sessionContentCount: 0,
        sessionTag: res.session_tag?.trim() || undefined,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        messages: [],
      };
      sessions.value.unshift(session);
      applySession(session.sessionCode);
      deps.messages.value = [];
    } catch (err) {
      console.error('[Session] createSession failed:', err);
    }
  };

  const switchSession = async (code: string) => {
    if (code === currentSessionCode.value) return;
    deps.abortStream();
    saveCurrentSession();

    const target = sessions.value.find((s) => s.sessionCode === code);
    if (!target) return;
    applySession(code);
    // 清空后由 fetchHistory 增量写入，配合 SNAPSHOT 到达即关 spinner 实现渐进渲染
    deps.messages.value = [];

    if (target.sessionContentCount === 0 && target.messages.length === 0) {
      return;
    }

    isLoadingHistory.value = true;
    try {
      await deps.fetchHistory(code, () => {
        // SNAPSHOT 到达即关闭全屏 loading，露出历史消息；后续 resume 事件继续追加
        if (currentSessionCode.value === code) isLoadingHistory.value = false;
      });
      // 竞态守卫：快速连续切换时，旧 fetch 的 await 回收时当前会话已被切到新 code，
      // 不要覆盖当前会话的 messages 或写回旧 target
      if (currentSessionCode.value === code) {
        target.messages = [...deps.messages.value];
      }
    } catch (err) {
      if (currentSessionCode.value === code) {
        console.warn('[Session] fetchHistory failed, fallback to local snapshot:', err);
        deps.messages.value = [...target.messages];
      }
    } finally {
      if (currentSessionCode.value === code) {
        isLoadingHistory.value = false;
      }
    }
  };

  const deleteSession = async (code: string) => {
    const bkBizId = resolveBkBizId();
    if (!bkBizId) return;

    try {
      await sessionApi.deleteSession(bkBizId, code);
    } catch (err) {
      console.error('[Session] deleteSession failed:', err);
      return;
    }

    const index = sessions.value.findIndex((s) => s.sessionCode === code);
    if (index === -1) return;
    sessions.value.splice(index, 1);

    if (code === currentSessionCode.value) {
      if (sessions.value.length > 0) {
        await switchSession(sessions.value[0].sessionCode);
      } else {
        await createSession();
      }
    }
  };

  const moveSessionToTop = (code: string) => {
    const idx = sessions.value.findIndex((s) => s.sessionCode === code);
    if (idx > 0) {
      const [session] = sessions.value.splice(idx, 1);
      sessions.value.unshift(session);
    }
  };

  const renameSession = async (code: string, title: string) => {
    const bkBizId = resolveBkBizId();
    if (!bkBizId) return;

    const name = title.trim();
    if (!name) return;
    const session = sessions.value.find((s) => s.sessionCode === code);
    if (!session) return;

    const oldName = session.sessionName;
    session.sessionName = name;

    try {
      await sessionApi.updateSession(bkBizId, code, name);
    } catch (err) {
      console.error('[Session] renameSession failed:', err);
      session.sessionName = oldName;
    }
  };

  // 回到首页空态：清空当前选中会话与消息，不创建/不选中任何会话。
  // 用于「进入页面」「新对话」「业务切换后当前会话失效」等场景统一收敛到默认首页。
  const goHome = () => {
    deps.abortStream();
    saveCurrentSession();
    applySession('');
    deps.messages.value = [];
  };

  const initSessions = async (targetCode?: string) => {
    await loadSessions();

    // 深链直达指定会话时切换；否则默认停留在首页空态，不自动创建/选中会话
    if (targetCode && sessions.value.some((s) => s.sessionCode === targetCode)) {
      await switchSession(targetCode);
      return;
    }
  };

  const reloadSessions = async () => {
    const code = currentSessionCode.value;
    await loadSessions();
    // 当前会话仍存在则保持；否则（业务切换 / 被删等）回到首页空态，不自动选中或新建
    if (code && sessions.value.some((s) => s.sessionCode === code)) {
      return;
    }
    goHome();
  };

  return {
    sessions,
    currentSessionCode,
    currentSession,
    isLoadingHistory,
    isLoadingSessions,
    saveCurrentSession,
    createSession,
    switchSession,
    deleteSession,
    renameSession,
    moveSessionToTop,
    goHome,
    initSessions,
    reloadSessions,
  };
}

export type SessionModule = ReturnType<typeof useSession>;
