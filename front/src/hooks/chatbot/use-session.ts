import { computed, ref, type Ref } from 'vue';
import type { Message } from '@blueking/chat-x';

import * as sessionApi from '@/store/chatbot/session';
import type { SessionApiItem } from '@/store/chatbot/session';
import type { ChatSession } from './types';

interface SessionDeps {
  messages: Ref<Message[]>;
  sessionCode: Ref<string>;
  // 会话切换 / 新建时仅做本地 abort，不调用后端 /cancel —— 主动取消仅由用户点击"停止"触发
  abortStream: () => void;
  // 注意：fetchHistory 现在采用增量渲染，直接写入 deps.messages，不再返回消息数组
  fetchHistory: (sessionCode: string, onSnapshotLoaded?: () => void) => Promise<void>;
}

const toSession = (item: SessionApiItem): ChatSession => ({
  sessionCode: item.session_code,
  sessionName: item.session_name || '新对话',
  sessionContentCount: item.session_content_count,
  createdAt: item.created_at,
  updatedAt: item.updated_at,
  messages: [],
});

export function useSession(deps: SessionDeps) {
  const sessions = ref<ChatSession[]>([]);
  const currentSessionCode = ref('');
  const isLoadingHistory = ref(false);
  const isLoadingSessions = ref(false);

  const currentSession = computed(() => sessions.value.find((s) => s.sessionCode === currentSessionCode.value));

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
    isLoadingSessions.value = true;
    try {
      const res = await sessionApi.listSessions();
      sessions.value = res.details.map(toSession);
    } finally {
      isLoadingSessions.value = false;
    }
  };

  const createSession = async () => {
    const emptySession = sessions.value.find((s) => s.messages.length === 0 && s.sessionContentCount === 0);
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
      const res = await sessionApi.createSession('新对话');
      const session: ChatSession = {
        sessionCode: res.session_code,
        sessionName: res.session_name || '新对话',
        sessionContentCount: 0,
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
    try {
      await sessionApi.deleteSession(code);
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
    const name = title.trim();
    if (!name) return;
    const session = sessions.value.find((s) => s.sessionCode === code);
    if (!session) return;

    const oldName = session.sessionName;
    session.sessionName = name;

    try {
      await sessionApi.updateSession(code, name);
    } catch (err) {
      console.error('[Session] renameSession failed:', err);
      session.sessionName = oldName;
    }
  };

  const initSessions = async (targetCode?: string) => {
    await loadSessions();

    if (targetCode && sessions.value.some((s) => s.sessionCode === targetCode)) {
      await switchSession(targetCode);
      return;
    }

    if (sessions.value.length > 0) {
      const recent = sessions.value[0];
      if (recent.sessionContentCount > 0) {
        await createSession();
      } else {
        await switchSession(recent.sessionCode);
      }
    } else {
      await createSession();
    }
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
    initSessions,
  };
}

export type SessionModule = ReturnType<typeof useSession>;
