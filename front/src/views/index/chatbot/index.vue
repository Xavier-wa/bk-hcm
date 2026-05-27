<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Plus, CollapseLeft } from 'bkui-vue/lib/icon';
import { InfoBox, OverflowTitle } from 'bkui-vue';

import {
  ChatInput,
  MessageContainer,
  MessageRole,
  MessageStatus,
  useMessageGroup,
  type IToolBtn,
  type Message,
  type TagSchema,
  type UserMessage,
} from '@blueking/chat-x';
import '@blueking/chat-x/dist/index.css';

import { useChatbot, extractText, type ChatSession } from '@/hooks/chatbot/use-chatbot';
import { useUserStore } from '@/store/user';
import HitlInterruptCard from './children/hitl-interrupt-card.vue';
import { type HitlInterruptValue, type HitlInterruptMessage } from '@/hooks/chatbot/types';

const route = useRoute();
const router = useRouter();
const userStore = useUserStore();

const {
  messages,
  isChatting,
  sessions,
  currentSessionCode,
  currentSession,
  isLoadingHistory,
  sendMessage,
  regenerate,
  resendEdited,
  stopGeneration,
  createSession,
  switchSession,
  deleteSession,
  renameSession,
  initSessions,
} = useChatbot();

const readRouteSessionCode = () => {
  const code = route.params.sessionCode;
  return typeof code === 'string' && code ? code : '';
};

const selectedUserMessages = ref<Message[]>();
const { messageGroups } = useMessageGroup({
  messages: computed(() => messages.value),
  selectedUserMessages,
});

const inputValue = shallowRef<string | TagSchema>([[]]);
const editingSessionId = ref('');
const editingTitle = ref('');
const activeMenuId = ref('');
const isCollapsed = ref(false);
const isHovering = ref(false);

const activeAnchorIndex = ref(0);
let navObserver: IntersectionObserver | null = null;

// 会话置顶：本地持久化，按当前登录用户隔离；未登录/未知用户回落到 anonymous 命名空间
const PINNED_STORAGE_PREFIX = 'hcm:chatbot:pinned:';
const pinnedStorageKey = computed(() => `${PINNED_STORAGE_PREFIX}${userStore.username || 'anonymous'}`);
const pinnedSessionCodes = ref<Set<string>>(new Set());

const loadPinned = () => {
  try {
    const raw = localStorage.getItem(pinnedStorageKey.value);
    const parsed = raw ? JSON.parse(raw) : [];
    pinnedSessionCodes.value = new Set(Array.isArray(parsed) ? parsed : []);
  } catch {
    pinnedSessionCodes.value = new Set();
  }
};

const savePinned = () => {
  try {
    localStorage.setItem(pinnedStorageKey.value, JSON.stringify([...pinnedSessionCodes.value]));
  } catch {
    // 存储失败（如隐私模式/配额）时静默降级为仅内存态
  }
};

const isPinned = (code: string) => pinnedSessionCodes.value.has(code);

const handleTogglePin = (code: string) => {
  const next = new Set(pinnedSessionCodes.value);
  if (next.has(code)) next.delete(code);
  else next.add(code);
  pinnedSessionCodes.value = next;
  savePinned();
  activeMenuId.value = '';
};

watch(pinnedStorageKey, loadPinned, { immediate: true });

const messageStatus = computed(() => (isChatting.value ? MessageStatus.Streaming : MessageStatus.Complete));
const currentTitle = computed(() => currentSession.value?.sessionName || '新对话');
const isEmpty = computed(() => messages.value.length === 0 && !isLoadingHistory.value);
const truncateLabel = (text: string, max = 50) => {
  if (!text) return '...';
  return text.length > max ? `${text.slice(0, max)}...` : text;
};

const userAnchors = computed(() =>
  messages.value
    .filter((m) => m.role === MessageRole.User)
    .map((m) => {
      const text = extractText(m.content);
      return { id: m.id, label: truncateLabel(text), fullText: text };
    }),
);

const sessionGroups = computed(() => {
  const day = 86400000;
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const todayStart = today.getTime();
  const groups: { label: string; sessions: ChatSession[] }[] = [];
  const buckets: Record<string, ChatSession[]> = {};
  const pinnedList: ChatSession[] = [];

  for (const s of sessions.value) {
    if (pinnedSessionCodes.value.has(s.sessionCode)) {
      pinnedList.push(s);
      continue;
    }

    let label: string;
    const diff = todayStart - new Date(s.updatedAt).getTime();
    if (diff < 0) label = '今天';
    else if (diff < day) label = '昨天';
    else if (diff < 7 * day) label = '7 天内';
    else if (diff < 30 * day) label = '30 天内';
    else label = '更早';

    if (!buckets[label]) buckets[label] = [];
    buckets[label].push(s);
  }

  if (pinnedList.length) {
    groups.push({ label: '置顶', sessions: pinnedList });
  }

  const order = ['今天', '昨天', '7 天内', '30 天内', '更早'];
  for (const label of order) {
    if (buckets[label]?.length) {
      groups.push({ label, sessions: buckets[label] });
    }
  }
  return groups;
});

const handleSendMessage = async (content: UserMessage['content'], _docSchema: TagSchema) => {
  const text = typeof content === 'string' ? content : '';
  if (!text.trim()) return;
  inputValue.value = [[]];
  await sendMessage(text);
};

const handleStopSending = async () => {
  stopGeneration();
};

const handleAgentAction = async (tool: IToolBtn, msgs: Message[]) => {
  if (tool.id === 'rebuild') {
    await regenerate(msgs);
  }
};

// 类型守卫：检查消息是否为 HITL 中断消息
const isHitlInterruptMessage = (message: Message): boolean => {
  return (message as HitlInterruptMessage).__type === 'hitl.interrupt';
};

const getHitlContent = (message: Message): HitlInterruptValue | null => {
  if (!isHitlInterruptMessage(message)) return null;
  return (message as HitlInterruptMessage).content as HitlInterruptValue;
};

// 历史消息只读展示：仅当后续 user 文本命中 options 时，才视为有效选择；
// 未命中则按“其它/未命中”处理（不回填具体文本，避免误判为自定义输入）
const getHitlReadonlyState = (message: Message): { readonly: boolean; value: string } => {
  if (!isHitlInterruptMessage(message)) return { readonly: false, value: '' };

  const currentIndex = messages.value.findIndex((item) => item.id === message.id);
  if (currentIndex < 0) return { readonly: false, value: '' };

  const nextMessage = messages.value[currentIndex + 1];
  if (!nextMessage || nextMessage.role !== MessageRole.User) return { readonly: false, value: '' };

  const userAnswer = extractText(nextMessage.content).trim();
  const hitlContent = getHitlContent(message);
  const options = hitlContent?.value.options ?? [];

  if (!userAnswer) return { readonly: true, value: '' };
  if (options.includes(userAnswer)) return { readonly: true, value: userAnswer };

  return { readonly: true, value: '' };
};

const handleUserInputConfirm = async (message: Message, content: UserMessage['content']) => {
  await resendEdited(message, content);
};

const handleNewSession = () => {
  createSession();
  inputValue.value = [[]];
};

const handleSwitchSession = (code: string) => {
  switchSession(code);
  inputValue.value = [[]];
  activeMenuId.value = '';
};

const handleStartRename = (session: ChatSession) => {
  editingSessionId.value = session.sessionCode;
  editingTitle.value = session.sessionName;
  activeMenuId.value = '';
};

const handleConfirmRename = () => {
  if (editingSessionId.value && editingTitle.value.trim()) {
    renameSession(editingSessionId.value, editingTitle.value);
  }
  editingSessionId.value = '';
  editingTitle.value = '';
};

const handleCancelRename = () => {
  editingSessionId.value = '';
  editingTitle.value = '';
};

const handleDeleteSession = (code: string) => {
  activeMenuId.value = '';
  InfoBox({
    title: '删除此会话？',
    content: '这条会话将被永久删除，不可恢复及撤销',
    confirmText: '删除',
    cancelText: '取消',
    confirmButtonTheme: 'danger',
    closeIcon: false,
    quickClose: true,
    escClose: true,
    onConfirm() {
      deleteSession(code);
      if (pinnedSessionCodes.value.has(code)) {
        const next = new Set(pinnedSessionCodes.value);
        next.delete(code);
        pinnedSessionCodes.value = next;
        savePinned();
      }
    },
  });
};

const toggleMenu = (id: string, e: Event) => {
  e.stopPropagation();
  activeMenuId.value = activeMenuId.value === id ? '' : id;
};

const closeMenu = () => {
  activeMenuId.value = '';
};

const toggleCollapse = () => {
  isCollapsed.value = !isCollapsed.value;
  isHovering.value = false;
};

const handleTriggerEnter = () => {
  if (isCollapsed.value) {
    isHovering.value = true;
  }
};

const handleSidebarLeave = () => {
  if (isCollapsed.value) {
    isHovering.value = false;
    activeMenuId.value = '';
  }
};

const scrollToAnchor = (index: number) => {
  const container = document.querySelector('.index-chat-messages');
  if (!container) return;
  const userGroups = container.querySelectorAll('.message-group:has(.ai-user-message)');
  userGroups[index]?.scrollIntoView({ behavior: 'smooth', block: 'start' });
};

const setupNavObserver = () => {
  navObserver?.disconnect();
  const container = document.querySelector('.index-chat-messages');
  if (!container) return;
  const userGroups = container.querySelectorAll('.message-group:has(.ai-user-message)');
  if (userGroups.length < 2) return;

  navObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          const idx = Array.from(userGroups).indexOf(entry.target as Element);
          if (idx !== -1) activeAnchorIndex.value = idx;
        }
      }
    },
    { root: container, threshold: 0.1 },
  );
  userGroups.forEach((el) => navObserver!.observe(el));
};

watch(
  () => messages.value.length,
  () => nextTick(setupNavObserver),
);

watch(currentSessionCode, (code) => {
  if (!code || route.params.sessionCode === code) return;
  router.replace({ params: { sessionCode: code }, query: route.query });
});

watch(
  () => route.params.sessionCode,
  (code) => {
    const target = typeof code === 'string' ? code : '';
    if (!target || target === currentSessionCode.value) return;
    if (sessions.value.some((s) => s.sessionCode === target)) {
      switchSession(target);
    }
  },
);

onBeforeUnmount(() => {
  navObserver?.disconnect();
});

onMounted(() => {
  initSessions(readRouteSessionCode() || undefined);
});
</script>

<template>
  <div class="index-page" @click="closeMenu">
    <div v-if="isCollapsed" class="sidebar-trigger" @mouseenter="handleTriggerEnter" />
    <div
      v-show="!isCollapsed || isHovering"
      class="index-sidebar"
      :class="{ 'is-overlay': isCollapsed && isHovering }"
      @mouseleave="handleSidebarLeave"
    >
      <button class="sidebar-new-btn" @click="handleNewSession">
        <plus style="font-size: 22px" />
        开启新对话
      </button>
      <div class="sidebar-sessions">
        <div v-for="group in sessionGroups" :key="group.label" class="session-group">
          <div class="session-group-label">{{ group.label }}</div>
          <div
            v-for="session in group.sessions"
            :key="session.sessionCode"
            class="session-item"
            :class="{ active: session.sessionCode === currentSessionCode }"
            @click="handleSwitchSession(session.sessionCode)"
          >
            <template v-if="editingSessionId === session.sessionCode">
              <input
                v-model="editingTitle"
                class="session-rename-input"
                @click.stop
                @keyup.enter="handleConfirmRename"
                @keyup.escape="handleCancelRename"
                @blur="handleConfirmRename"
              />
            </template>
            <template v-else>
              <OverflowTitle type="tips" class="session-title" :popover-options="{ maxWidth: 280 }">
                {{ session.sessionName }}
              </OverflowTitle>
              <span class="session-actions" @click="toggleMenu(session.sessionCode, $event)">
                <i class="icon-more" />
              </span>
              <div v-if="activeMenuId === session.sessionCode" class="session-menu" @click.stop>
                <div class="session-menu-item" @click="handleStartRename(session)">重命名</div>
                <div class="session-menu-item" @click="handleTogglePin(session.sessionCode)">
                  {{ isPinned(session.sessionCode) ? '取消置顶' : '置顶' }}
                </div>
                <div class="session-menu-item danger" @click="handleDeleteSession(session.sessionCode)">删除</div>
              </div>
            </template>
          </div>
        </div>
      </div>
      <div class="sidebar-toggle" :title="isCollapsed ? '展开侧栏' : '收起侧栏'" @click="toggleCollapse">
        <CollapseLeft class="toggle-icon" :style="{ transform: isCollapsed ? 'rotate(0deg)' : 'rotate(180deg)' }" />
      </div>
    </div>
    <div class="index-main">
      <div class="index-header">
        <span class="header-title">{{ currentTitle }}</span>
      </div>
      <div class="index-chat">
        <div class="index-chat-messages">
          <div v-if="isLoadingHistory" class="chat-loading">
            <div class="loading-spinner" />
            <span>加载会话历史...</span>
          </div>
          <div v-else-if="isEmpty" class="chat-empty">
            <div class="empty-icon">💬</div>
            <h3 class="empty-title">海垒 AI 助手</h3>
            <p class="empty-desc">有什么可以帮你的？</p>
          </div>
          <!-- messages 在新版组件中未被消费，仅因类型定义为必填而保留 -->
          <MessageContainer
            v-else
            :messages="messages"
            :message-groups="messageGroups"
            :message-status="messageStatus"
            :on-agent-action="handleAgentAction"
            :on-user-input-confirm="handleUserInputConfirm"
            @stop-streaming="handleStopSending"
          >
            <template #default="{ message }">
              <HitlInterruptCard
                v-if="isHitlInterruptMessage(message)"
                :content="getHitlContent(message)"
                :readonly="getHitlReadonlyState(message).readonly"
                :readonly-value="getHitlReadonlyState(message).value"
                :on-confirm="sendMessage"
              />
            </template>
          </MessageContainer>
        </div>
        <div v-if="userAnchors.length >= 2" class="message-nav">
          <div class="nav-track">
            <div
              v-for="(anchor, index) in userAnchors"
              :key="anchor.id"
              class="nav-item"
              :class="{ active: activeAnchorIndex === index }"
              @click="scrollToAnchor(index)"
            >
              <span class="nav-label" :title="anchor.fullText">{{ anchor.label }}</span>
              <span class="nav-dot" />
            </div>
          </div>
        </div>
        <div class="index-chat-input">
          <ChatInput
            v-model="inputValue"
            :message-status="messageStatus"
            :support-upload="false"
            :on-send-message="handleSendMessage"
            :on-stop-sending="handleStopSending"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.index-page {
  --sidebar-bg: #f5f7fa;
  --sidebar-border: #eaebf0;
  --sidebar-overlay-shadow: rgb(0 0 0 / 8%);
  --sidebar-text: #63656e;
  --sidebar-text-secondary: #979ba5;
  --sidebar-btn-bg: #fff;
  --sidebar-btn-border: #dcdee5;
  --sidebar-btn-hover-bg: #eaebf0;
  --sidebar-btn-hover-border: #c4c6cc;
  --sidebar-scroll-thumb: #dcdee5;
  --sidebar-toggle-color: #979ba5;
  --sidebar-toggle-hover-color: #3a3c42;
  --sidebar-toggle-hover-bg: linear-gradient(270deg, #dee0ea 0%, #eaecf2 100%);
  --sidebar-item-hover: #eaebf0;
  --sidebar-item-active-bg: #e1ecff;
  --sidebar-item-active-color: #3a84ff;
  --sidebar-actions-hover: #dcdee5;
  --sidebar-input-bg: #fff;
  --sidebar-input-color: #313238;
  --sidebar-menu-bg: #fff;
  --sidebar-menu-border: #eaebf0;
  --sidebar-menu-shadow: rgb(0 0 0 / 10%);
  --sidebar-menu-hover: #f5f7fa;
  --main-header-border: #eaebf0;
  --main-header-color: #313238;
  --main-bg: #fff;
  --chat-scroll-thumb: #dcdee5;
  --chat-scroll-thumb-hover: #c4c6cc;
  --chat-loading-color: #979ba5;
  --chat-spinner-track: #eaebf0;
  --chat-spinner-color: #3a84ff;
  --chat-empty-title: #313238;
  --chat-empty-desc: #979ba5;
  --message-nav-hover-bg: rgb(255 255 255 / 95%);
  --message-nav-hover-shadow: rgb(0 0 0 / 8%);
  --message-nav-dot-color: #c4c6cc;
  --message-nav-dot-active: #3a84ff;
  --message-nav-label-color: #63656e;
  --message-nav-label-hover: #3a84ff;
  --message-nav-label-active: #3a84ff;

  position: relative;
  display: flex;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

@media (prefers-color-scheme: dark) {
  .index-page {
    --sidebar-bg: #1e1e2e;
    --sidebar-border: #2e2e3e;
    --sidebar-overlay-shadow: rgb(0 0 0 / 40%);
    --sidebar-text: #d0d0d0;
    --sidebar-text-secondary: #888;
    --sidebar-btn-bg: #3a3a4a;
    --sidebar-btn-border: #4a4a5a;
    --sidebar-btn-hover-bg: #4a4a5a;
    --sidebar-btn-hover-border: #5a5a6a;
    --sidebar-scroll-thumb: #3a3a4a;
    --sidebar-toggle-color: #96a2b9;
    --sidebar-toggle-hover-color: #d3d9e4;
    --sidebar-toggle-hover-bg: linear-gradient(270deg, #253047 0%, #263247 100%);
    --sidebar-item-hover: #2a2a3a;
    --sidebar-item-active-bg: #3a3a4a;
    --sidebar-item-active-color: #e1ecff;
    --sidebar-actions-hover: #4a4a5a;
    --sidebar-input-bg: #2a2a3a;
    --sidebar-input-color: #fff;
    --sidebar-menu-bg: #2e2e3e;
    --sidebar-menu-border: #3e3e4e;
    --sidebar-menu-shadow: rgb(0 0 0 / 30%);
    --sidebar-menu-hover: #3a3a4a;
    --message-nav-hover-bg: rgb(40 40 52 / 95%);
    --message-nav-hover-shadow: rgb(0 0 0 / 20%);
    --message-nav-dot-color: #5c5e66;
    --message-nav-dot-active: #3a84ff;
    --message-nav-label-color: #979ba5;
    --message-nav-label-hover: #699df4;
    --message-nav-label-active: #699df4;
  }
}

.sidebar-trigger {
  position: absolute;
  top: 0;
  left: 0;
  z-index: 200;
  width: 8px;
  height: 100%;
}

.index-sidebar {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  width: 240px;
  height: 100%;
  padding: 12px;
  color: var(--sidebar-text);
  background: var(--sidebar-bg);
  border-right: 1px solid var(--sidebar-border);
  transition: width 0.25s ease;

  &.is-overlay {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 150;
    box-shadow: 4px 0 12px var(--sidebar-overlay-shadow);
  }

  .sidebar-new-btn {
    display: flex;
    gap: 6px;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 40px;
    padding: 0 16px;
    font-size: 14px;
    color: var(--sidebar-text);
    cursor: pointer;
    background: var(--sidebar-btn-bg);
    border: 1px solid var(--sidebar-btn-border);
    border-radius: 8px;
    transition: background 0.2s, border-color 0.2s;

    &:hover {
      background: var(--sidebar-btn-hover-bg);
      border-color: var(--sidebar-btn-hover-border);
    }
  }

  .sidebar-sessions {
    flex: 1;
    margin-top: 16px;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: var(--sidebar-scroll-thumb) transparent;
  }

  .sidebar-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    margin-top: 8px;
    font-size: 14px;
    color: var(--sidebar-toggle-color);
    cursor: pointer;
    background: transparent;
    border: none;
    border-radius: 50%;
    transition: color 0.3s, background 0.3s;

    &:hover {
      color: var(--sidebar-toggle-hover-color);
      background: var(--sidebar-toggle-hover-bg);
    }

    .toggle-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 16px;
      height: 16px;
      font-size: 16px;
      transition: transform cubic-bezier(0.4, 0, 0.2, 1) 0.3s;
      transform-origin: center center;
    }
  }

  .session-group {
    margin-bottom: 8px;
  }

  .session-group-label {
    padding: 8px 8px 4px;
    font-size: 11px;
    color: var(--sidebar-text-secondary);
  }

  .session-actions {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    cursor: pointer;
    opacity: 0;
    border-radius: 4px;
    transition: opacity 0.15s;

    .icon-more::before {
      content: '···';
      font-size: 14px;
      font-weight: bold;
      color: var(--sidebar-text-secondary);
    }

    &:hover {
      background: var(--sidebar-actions-hover);
    }
  }

  .session-item {
    position: relative;
    display: flex;
    align-items: center;
    height: 36px;
    padding: 0 8px;
    margin-bottom: 2px;
    cursor: pointer;
    border-radius: 6px;
    transition: background 0.15s;

    &:hover {
      background: var(--sidebar-item-hover);

      .session-actions {
        opacity: 1;
      }
    }

    &.active {
      color: var(--sidebar-item-active-color);
      background: var(--sidebar-item-active-bg);
    }
  }

  .session-title {
    flex: 1;
    overflow: hidden;
    font-size: 13px;
    line-height: 36px;
    color: inherit;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .session-rename-input {
    flex: 1;
    height: 28px;
    padding: 0 8px;
    font-size: 13px;
    color: var(--sidebar-input-color);
    background: var(--sidebar-input-bg);
    border: 1px solid #3a84ff;
    border-radius: 4px;
    outline: none;
  }

  .session-menu {
    position: absolute;
    top: 36px;
    right: 4px;
    z-index: 100;
    min-width: 100px;
    padding: 4px 0;
    background: var(--sidebar-menu-bg);
    border: 1px solid var(--sidebar-menu-border);
    border-radius: 6px;
    box-shadow: 0 4px 12px var(--sidebar-menu-shadow);
  }

  .session-menu-item {
    padding: 6px 16px;
    font-size: 13px;
    color: var(--sidebar-text);
    cursor: pointer;

    &:hover {
      background: var(--sidebar-menu-hover);
    }

    &.danger {
      color: #ea3636;
    }
  }
}

.index-main {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  height: 100%;
  background: var(--main-bg);
}

.index-header {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  height: 48px;
  padding: 0 24px;
  border-bottom: 1px solid var(--main-header-border);

  .header-title {
    font-size: 15px;
    font-weight: 500;
    color: var(--main-header-color);
  }
}

.index-chat {
  position: relative;
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;

  .index-chat-messages {
    flex: 1;
    padding: 16px 0 16px 16px;
    overflow-y: auto;
    scrollbar-color: var(--chat-scroll-thumb) transparent;
    scrollbar-width: thin;

    &::-webkit-scrollbar {
      width: 6px;
    }

    &::-webkit-scrollbar-track {
      background: transparent;
    }

    &::-webkit-scrollbar-thumb {
      background: var(--chat-scroll-thumb);
      border-radius: 3px;
      transition: background 0.2s;

      &:hover {
        background: var(--chat-scroll-thumb-hover);
      }
    }

    :deep(.message-group) {
      max-width: 1000px;
      padding-right: 16px;
      margin-right: auto;
      margin-left: auto;
    }
  }

  :deep(.message-wrapper) {
    .message-tools-hover {
      opacity: 0;
      transition: opacity 0.2s ease-in-out;
    }

    &:hover .message-tools-hover {
      opacity: 1;
    }
  }

  :deep(.message-tools-container:not(.ai-user-message-tools)) {
    .message-tools > *:has(.ai-cite-icon),
    .message-tools > *:has(.ai-share-icon),
    .ai-divider,
    .message-tools:last-child {
      display: none;
    }
  }

  :deep(.ai-user-message-tools) {
    .message-tools > *:has(.ai-cite-icon),
    .message-tools > *:has(.ai-delete-icon) {
      display: none;
    }
  }

  .chat-loading {
    display: flex;
    flex-direction: column;
    gap: 12px;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--chat-loading-color);

    .loading-spinner {
      width: 28px;
      height: 28px;
      border: 3px solid var(--chat-spinner-track);
      border-top-color: var(--chat-spinner-color);
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }
  }

  .chat-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    user-select: none;

    .empty-icon {
      font-size: 48px;
    }

    .empty-title {
      margin-top: 16px;
      font-size: 20px;
      font-weight: 600;
      color: var(--chat-empty-title);
    }

    .empty-desc {
      margin-top: 8px;
      font-size: 14px;
      color: var(--chat-empty-desc);
    }
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .nav-track {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 60vh;
    overflow: hidden auto;
    scrollbar-width: none;

    &::-webkit-scrollbar {
      display: none;
    }
  }

  .nav-dot {
    position: absolute;
    right: 10px;
    top: 50%;
    width: 6px;
    height: 6px;
    background: var(--message-nav-dot-color);
    border-radius: 50%;
    transform: translateY(-50%);
    transition: all 0.2s;
  }

  .nav-label {
    overflow: hidden;
    font-size: 12px;
    line-height: 20px;
    color: var(--message-nav-label-color);
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0;
    transition: opacity 0.2s ease;

    &:hover {
      color: var(--message-nav-label-hover);
    }
  }

  .nav-item {
    position: relative;
    display: flex;
    align-items: center;
    height: 28px;
    padding-right: 34px;
    padding-left: 0;
    border-radius: 4px;
    cursor: pointer;
    transition: padding-left 0.25s ease, background 0.15s;

    &.active .nav-dot {
      width: 10px;
      height: 4px;
      background: var(--message-nav-dot-active);
      border-radius: 2px;
    }

    &.active .nav-label {
      color: var(--message-nav-label-active);
    }
  }

  .message-nav {
    position: absolute;
    top: 50%;
    right: 8px;
    z-index: 10;
    width: 34px;
    padding: 8px 0;
    border-radius: 8px;
    transform: translateY(-50%);
    transition: width 0.25s ease, background 0.25s, box-shadow 0.25s;

    &:hover {
      width: 240px;
      background: var(--message-nav-hover-bg);
      box-shadow: 0 2px 12px var(--message-nav-hover-shadow);
      backdrop-filter: blur(8px);

      .nav-label {
        opacity: 1;
      }

      .nav-item {
        padding-left: 10px;
      }
    }
  }

  .index-chat-input {
    padding: 16px;
  }
}
</style>
