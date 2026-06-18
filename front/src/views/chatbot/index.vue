<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, provide, ref, useTemplateRef, watch } from 'vue';
import { useRoute } from 'vue-router';
import { MENU_BUSINESS_CHATBOT } from '@/constants/menu-symbol';
import routerAction from '@/router/utils/action';
import { AngleDownLine, AngleLeft, AngleRight, Close, Search } from 'bkui-vue/lib/icon';
import { InfoBox } from 'bkui-vue';

import { MessageRole } from '@blueking/chat-x';

import { useChatbot, extractText, type ChatSession } from '@/hooks/chatbot/use-chatbot';
import { ChatbotKey, ChatbotModeKey } from '@/hooks/chatbot/provide';
import { useUserStore } from '@/store/user';
import ChatMessageList from '@/components/chatbot/chat-message-list.vue';
import ChatInputBox from '@/components/chatbot/chat-input-box.vue';
import AccountSelectEcho from '@/components/chatbot/account-select-echo.vue';
import SessionSidebarItem from './children/session-sidebar-item.vue';
import { GLOBAL_BIZS_KEY } from '@/common/constant';
import { ASSISTANT_CONTACT, BIG_CARDS, PROMPT_CHIPS, type BigCard, type PromptChip } from './constants';
import { resolveSessionTagName } from './utils';
import cloudAssistantSvg from '@/assets/image/cloud-assistant.svg';
import WName from '@/components/w-name';

const route = useRoute();
const userStore = useUserStore();

const chatbot = useChatbot();
// 容器持有 useChatbot 实例并 provide，供 chat-message-list / chat-input-box 等原子 inject 复用同一会话内核
provide(ChatbotKey, chatbot);
provide(ChatbotModeKey, 'fullpage');

const {
  messages,
  isChatting,
  sessions,
  currentSessionCode,
  currentSession,
  isLoadingHistory,
  sendMessage,
  switchSession,
  deleteSession,
  renameSession,
  goHome,
  initSessions,
  reloadSessions,
  selectedAccountEcho,
} = chatbot;

const readRouteSessionCode = () => {
  const code = route.params.sessionCode;
  return typeof code === 'string' && code ? code : '';
};

const editingSessionId = ref('');
const editingTitle = ref('');
const activeMenuId = ref('');
const isCollapsed = ref(false);
const sessionSearchKeyword = ref('');
const expandedFolderTags = ref<Set<string>>(new Set());

// 小卡片点击后的待发送场景（chip 在编辑器首行自渲染；其 sessionTag 在发送时随 create_session 上报）
const pendingChip = ref<PromptChip | null>(null);
const chatInputBoxRef = useTemplateRef<{ setInput: (text: string) => void; focus: () => void }>('chatInputBoxRef');
// chip 绝对定位 + 编辑器首行 text-indent 让位，需实测 chip 宽度
const sceneChipRef = ref<HTMLElement>();
const sceneChipWidth = ref(0);

// 小卡片（提示词）分段翻页：按容器宽度动态分段
const chipsViewportRef = ref<HTMLElement>();
const currentChipSegment = ref(0);
const chipSegmentOffsets = ref<number[]>([0]);
let chipsResizeObserver: ResizeObserver | null = null;

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

const handleTogglePin = (code: string) => {
  const next = new Set(pinnedSessionCodes.value);
  if (next.has(code)) next.delete(code);
  else next.add(code);
  pinnedSessionCodes.value = next;
  savePinned();
  activeMenuId.value = '';
};

watch(pinnedStorageKey, loadPinned, { immediate: true });

const isEmpty = computed(() => messages.value.length === 0 && !isLoadingHistory.value);
// 首页空态：未选中任何会话且无消息 → 展示默认主内容区（欢迎区 + 大卡片 + 小卡片排）。
// 已选中会话但无消息（空会话）保持空白，不展示默认内容。
const isHomeEmpty = computed(() => !currentSessionCode.value && isEmpty.value);
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

const sortSessionsByUpdatedAt = (list: ChatSession[]) =>
  [...list].sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime());

const matchesSessionSearch = (session: ChatSession) => {
  const keyword = sessionSearchKeyword.value.trim().toLowerCase();
  if (!keyword) return true;
  return session.sessionName.toLowerCase().includes(keyword);
};

const filterSessions = (list: ChatSession[]) => list.filter(matchesSessionSearch);

const sidebarSessions = computed(() => {
  const pinned: ChatSession[] = [];
  const history: ChatSession[] = [];

  for (const session of sessions.value) {
    if (pinnedSessionCodes.value.has(session.sessionCode)) {
      pinned.push(session);
    } else {
      history.push(session);
    }
  }

  const ungrouped = filterSessions(sortSessionsByUpdatedAt(history.filter((s) => !s.sessionTag)));
  const folderMap = new Map<string, ChatSession[]>();

  for (const session of history) {
    if (!session.sessionTag) continue;
    const list = folderMap.get(session.sessionTag) || [];
    list.push(session);
    folderMap.set(session.sessionTag, list);
  }

  const folders = [...folderMap.entries()]
    .map(([tag, list]) => {
      const filtered = filterSessions(sortSessionsByUpdatedAt(list));
      return {
        tag,
        sessions: filtered,
        maxUpdatedAt: Math.max(...list.map((s) => new Date(s.updatedAt).getTime())),
      };
    })
    .filter((folder) => folder.sessions.length > 0)
    .sort((a, b) => b.maxUpdatedAt - a.maxUpdatedAt);

  return {
    pinned: filterSessions(sortSessionsByUpdatedAt(pinned)),
    ungrouped,
    folders,
  };
});

const hasSidebarSessions = computed(
  () =>
    sidebarSessions.value.pinned.length > 0 ||
    sidebarSessions.value.ungrouped.length > 0 ||
    sidebarSessions.value.folders.length > 0,
);

const showNoMatchSessions = computed(
  () => !!sessionSearchKeyword.value.trim() && sessions.value.length > 0 && !hasSidebarSessions.value,
);

const pinnedMenuItems = [
  { key: 'unpin', label: '取消置顶' },
  { key: 'rename', label: '重命名' },
  { key: 'delete', label: '删除', danger: true },
];

const historyMenuItems = [
  { key: 'pin', label: '置顶' },
  { key: 'rename', label: '重命名' },
  { key: 'delete', label: '删除', danger: true },
];

const isFolderExpanded = (tag: string) => expandedFolderTags.value.has(tag);

const toggleFolder = (tag: string) => {
  const next = new Set(expandedFolderTags.value);
  if (next.has(tag)) next.delete(tag);
  else next.add(tag);
  expandedFolderTags.value = next;
};

watch(
  () => sidebarSessions.value.folders.map((folder) => folder.tag),
  (tags) => {
    const next = new Set(expandedFolderTags.value);
    for (const tag of tags) {
      next.add(tag);
    }
    expandedFolderTags.value = next;
  },
  { immediate: true },
);

const handleMenuAction = (session: ChatSession, key: string) => {
  if (key === 'pin' || key === 'unpin') {
    handleTogglePin(session.sessionCode);
    return;
  }
  if (key === 'rename') {
    handleStartRename(session);
    return;
  }
  if (key === 'delete') {
    handleDeleteSession(session.sessionCode);
  }
};

const handleSend = async (text: string) => {
  // 小卡片场景的 sessionTag 随本次发送上报后清空；普通输入无 tag
  const sessionTag = pendingChip.value?.sessionTag || '';
  pendingChip.value = null;
  await sendMessage(text, sessionTag);
};

// 大卡片：点击直接发送预设消息（带场景 tag 时随会话创建上报）
const handleBigCardClick = async (card: BigCard) => {
  if (isChatting.value) return;
  pendingChip.value = null;
  await sendMessage(card.prompt, card.sessionTag || '');
};

// 小卡片：编辑器内注入默认提示词文本，场景 chip 由 #input-header 插槽自渲染（组件 v-model 不支持注入 tag 节点）
const handlePromptChipClick = (chip: PromptChip) => {
  pendingChip.value = chip;
  chatInputBoxRef.value?.setInput(chip.prompt);
  nextTick(() => chatInputBoxRef.value?.focus());
};

// 移除场景 chip：仅清除场景标识，保留输入框已有文本
const clearPendingChip = () => {
  pendingChip.value = null;
};

// 场景 chip：首页点击小卡片时为可关闭的待发 chip；进入带 session_tag 的会话时回显为只读 chip
const sceneChip = computed<{ label: string; readonly: boolean } | null>(() => {
  if (pendingChip.value) return { label: pendingChip.value.tag, readonly: false };
  const tag = currentSession.value?.sessionTag;
  if (tag) return { label: resolveSessionTagName(tag), readonly: true };
  return null;
});

// chip 渲染/变化后实测其宽度，供编辑器首行 text-indent 让位
watch(sceneChip, () => {
  if (!sceneChip.value) {
    sceneChipWidth.value = 0;
    return;
  }
  nextTick(() => {
    sceneChipWidth.value = sceneChipRef.value?.offsetWidth ?? 0;
  });
});

const handleNewSession = () => {
  // 新对话：回到首页空态，待用户首次发送时再惰性创建会话（输入框随会话切换由 chat-input-box 自行清空）
  goHome();
  pendingChip.value = null;
};

const handleSwitchSession = (code: string) => {
  switchSession(code);
  pendingChip.value = null;
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
};

const expandSidebar = () => {
  isCollapsed.value = false;
};

const scrollToAnchor = (index: number) => {
  const container = document.querySelector('.chatbot-chat-messages');
  if (!container) return;
  const userGroups = container.querySelectorAll('.message-group:has(.ai-user-message)');
  userGroups[index]?.scrollIntoView({ behavior: 'smooth', block: 'start' });
};

const setupNavObserver = () => {
  navObserver?.disconnect();
  const container = document.querySelector('.chatbot-chat-messages');
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

// 计算小卡片分段：从首个 chip 起累计宽度，超出视口即作为新段起点，记录各段相对偏移
const recomputeChipSegments = () => {
  const viewport = chipsViewportRef.value;
  const items = viewport ? Array.from(viewport.querySelectorAll<HTMLElement>('.prompt-chip')) : [];
  if (!viewport || !items.length) {
    chipSegmentOffsets.value = [0];
    currentChipSegment.value = 0;
    return;
  }
  const viewportWidth = viewport.clientWidth;
  const baseLeft = items[0].offsetLeft;
  const offsets = [0];
  let segStartLeft = baseLeft;
  for (const item of items) {
    const itemRight = item.offsetLeft + item.offsetWidth - segStartLeft;
    if (itemRight > viewportWidth && item.offsetLeft > segStartLeft) {
      segStartLeft = item.offsetLeft;
      offsets.push(segStartLeft - baseLeft);
    }
  }
  chipSegmentOffsets.value = offsets;
  if (currentChipSegment.value > offsets.length - 1) {
    currentChipSegment.value = offsets.length - 1;
  }
};

const chipTrackStyle = computed(() => ({
  transform: `translateX(-${chipSegmentOffsets.value[currentChipSegment.value] ?? 0}px)`,
}));
const hasChipPaging = computed(() => chipSegmentOffsets.value.length > 1);
// 单按钮翻页：未到末段显示向右箭头并前进一段；到末段显示向左箭头并回到首段
const isChipAtEnd = computed(() => currentChipSegment.value >= chipSegmentOffsets.value.length - 1);
const toggleChipSegment = () => {
  currentChipSegment.value = isChipAtEnd.value ? 0 : currentChipSegment.value + 1;
};

watch(
  isHomeEmpty,
  (val) => {
    if (!val) {
      chipsResizeObserver?.disconnect();
      return;
    }
    nextTick(() => {
      recomputeChipSegments();
      // v-if 重新挂载会生成新的 viewport 节点，每次进入首页空态重新绑定监听
      if (chipsViewportRef.value) {
        chipsResizeObserver?.disconnect();
        chipsResizeObserver = new ResizeObserver(() => recomputeChipSegments());
        chipsResizeObserver.observe(chipsViewportRef.value);
      }
    });
  },
  { immediate: true },
);

watch(
  () => messages.value.length,
  () => nextTick(setupNavObserver),
);

watch(currentSessionCode, (code) => {
  const routeCode = typeof route.params.sessionCode === 'string' ? route.params.sessionCode : '';
  if (code === routeCode) return;
  // 选中会话 → 写入 sessionCode；回到首页空态（code 为空）→ 去掉 sessionCode 参数
  routerAction.redirect(
    {
      name: MENU_BUSINESS_CHATBOT,
      params: code ? { sessionCode: code } : {},
      query: route.query,
    },
    { replace: true },
  );
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

watch(
  () => route.query[GLOBAL_BIZS_KEY],
  () => {
    reloadSessions();
  },
);

onBeforeUnmount(() => {
  navObserver?.disconnect();
  chipsResizeObserver?.disconnect();
  chipsResizeObserver = null;
});

onMounted(() => {
  initSessions(readRouteSessionCode() || undefined);
});
</script>

<template>
  <div class="chatbot-page" @click="closeMenu">
    <aside v-if="isCollapsed" class="chatbot-sidebar-rail" aria-label="会话侧栏快捷操作">
      <button type="button" class="rail-btn" title="展开侧栏" @click="expandSidebar">
        <AngleDownLine class="rail-expand-icon" />
      </button>
      <button type="button" class="rail-btn" title="新对话" @click="handleNewSession">
        <i class="hcm-icon bkhcm-icon-chat-plus rail-new-chat-icon" />
      </button>
    </aside>
    <aside v-show="!isCollapsed" class="chatbot-sidebar">
      <button type="button" class="sidebar-collapse-btn" title="收起侧栏" @click.stop="toggleCollapse">
        <AngleDownLine class="collapse-icon" />
      </button>
      <div class="sidebar-search">
        <input
          v-model="sessionSearchKeyword"
          class="sidebar-search-input"
          type="text"
          placeholder="搜索对话"
          @click.stop
        />
        <Search class="sidebar-search-icon" />
      </div>
      <button type="button" class="sidebar-new-btn" @click="handleNewSession">
        <i class="hcm-icon bkhcm-icon-chat-plus sidebar-new-btn-icon" />
        新对话
      </button>
      <div class="sidebar-sessions">
        <div v-if="showNoMatchSessions" class="session-empty">无匹配会话</div>
        <template v-else>
          <div v-if="sidebarSessions.pinned.length" class="session-group">
            <div class="session-group-label">置顶</div>
            <SessionSidebarItem
              v-for="session in sidebarSessions.pinned"
              :key="session.sessionCode"
              :session="session"
              :is-active="session.sessionCode === currentSessionCode"
              is-pinned-area
              :is-editing="editingSessionId === session.sessionCode"
              :editing-title="editingTitle"
              :is-menu-open="activeMenuId === session.sessionCode"
              :menu-items="pinnedMenuItems"
              @click="handleSwitchSession(session.sessionCode)"
              @update:editing-title="(v) => (editingTitle = v)"
              @confirm-rename="handleConfirmRename"
              @cancel-rename="handleCancelRename"
              @toggle-menu="toggleMenu(session.sessionCode, $event)"
              @menu-action="(key) => handleMenuAction(session, key)"
            />
          </div>
          <div v-if="sidebarSessions.ungrouped.length || sidebarSessions.folders.length" class="session-group">
            <div class="session-group-label">历史对话</div>
            <SessionSidebarItem
              v-for="session in sidebarSessions.ungrouped"
              :key="session.sessionCode"
              :session="session"
              :is-active="session.sessionCode === currentSessionCode"
              :is-pinned-area="false"
              :is-editing="editingSessionId === session.sessionCode"
              :editing-title="editingTitle"
              :is-menu-open="activeMenuId === session.sessionCode"
              :menu-items="historyMenuItems"
              @click="handleSwitchSession(session.sessionCode)"
              @update:editing-title="(v) => (editingTitle = v)"
              @confirm-rename="handleConfirmRename"
              @cancel-rename="handleCancelRename"
              @toggle-menu="toggleMenu(session.sessionCode, $event)"
              @menu-action="(key) => handleMenuAction(session, key)"
            />
            <div v-for="folder in sidebarSessions.folders" :key="folder.tag" class="session-folder">
              <div class="session-folder-header" @click.stop="toggleFolder(folder.tag)">
                <i
                  class="hcm-icon session-folder-icon"
                  :class="isFolderExpanded(folder.tag) ? 'bkhcm-icon-folder-open' : 'bkhcm-icon-folder-close'"
                />
                <span class="session-folder-name">{{ resolveSessionTagName(folder.tag) }}</span>
                <span class="session-folder-count">{{ folder.sessions.length }}</span>
              </div>
              <div v-if="isFolderExpanded(folder.tag)" class="session-folder-children">
                <SessionSidebarItem
                  v-for="session in folder.sessions"
                  :key="session.sessionCode"
                  :session="session"
                  :is-active="session.sessionCode === currentSessionCode"
                  :is-pinned-area="false"
                  :is-editing="editingSessionId === session.sessionCode"
                  :editing-title="editingTitle"
                  :is-menu-open="activeMenuId === session.sessionCode"
                  :menu-items="historyMenuItems"
                  @click="handleSwitchSession(session.sessionCode)"
                  @update:editing-title="(v) => (editingTitle = v)"
                  @confirm-rename="handleConfirmRename"
                  @cancel-rename="handleCancelRename"
                  @toggle-menu="toggleMenu(session.sessionCode, $event)"
                  @menu-action="(key) => handleMenuAction(session, key)"
                />
              </div>
            </div>
          </div>
        </template>
      </div>
    </aside>
    <div class="chatbot-main">
      <div class="chatbot-chat">
        <!-- 云账号选择吸顶回显：通栏铺满主内容区，固定在消息滚动区上方 -->
        <AccountSelectEcho v-if="selectedAccountEcho" :echo="selectedAccountEcho" class="chatbot-account-echo" />
        <div class="chatbot-chat-messages">
          <div v-if="isLoadingHistory" class="chat-loading">
            <div class="loading-spinner" />
            <span>加载会话历史...</span>
          </div>
          <div v-else-if="isHomeEmpty" class="home-default">
            <div class="welcome">
              <img class="welcome-icon" :src="cloudAssistantSvg" alt="海垒 AI 助手" />
              <h3 class="welcome-title">海垒 AI 助手</h3>
              <p class="welcome-tips">可使用对话进行快捷主机申领，主机回收，预测提单等</p>
            </div>
            <div class="big-cards">
              <button
                v-for="card in BIG_CARDS"
                :key="card.title"
                type="button"
                class="big-card"
                :disabled="isChatting"
                @click="handleBigCardClick(card)"
              >
                <i class="hcm-icon big-card-icon" :class="card.icon" />
                <div class="big-card-body">
                  <span class="big-card-title">{{ card.title }}</span>
                  <span class="big-card-desc">{{ card.desc }}</span>
                </div>
              </button>
            </div>
          </div>
          <!-- 已选中会话但无消息：保持空白，不回退默认内容 -->
          <div v-else-if="isEmpty" class="chat-blank" />
          <ChatMessageList v-else />
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
        <div v-if="isHomeEmpty" class="prompt-chips">
          <div class="prompt-chips-inner">
            <div ref="chipsViewportRef" class="chips-viewport">
              <div class="chips-track" :style="chipTrackStyle">
                <button
                  v-for="chip in PROMPT_CHIPS"
                  :key="chip.tag"
                  type="button"
                  class="prompt-chip"
                  @click="handlePromptChipClick(chip)"
                >
                  <i class="hcm-icon prompt-chip-icon" :class="chip.icon" />
                  <span class="prompt-chip-label">{{ chip.tag }}</span>
                </button>
              </div>
            </div>
            <button v-if="hasChipPaging" type="button" class="chip-pager" @click="toggleChipSegment">
              <component :is="isChipAtEnd ? AngleLeft : AngleRight" />
            </button>
          </div>
        </div>
        <ChatInputBox
          ref="chatInputBoxRef"
          class="chatbot-chat-input"
          :input-indent="sceneChipWidth"
          @send="handleSend"
        >
          <template #input-header>
            <div v-if="sceneChip" ref="sceneChipRef" class="scene-chip">
              <span class="scene-chip-label">{{ sceneChip.label }}</span>
              <Close v-if="!sceneChip.readonly" class="scene-chip-close" @click="clearPendingChip" />
            </div>
          </template>
        </ChatInputBox>
        <p class="chatbot-footer">
          有任何问题可联系
          <WName :name="ASSISTANT_CONTACT.name" :alias="ASSISTANT_CONTACT.alias" />
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.chatbot-page {
  --sidebar-bg: #fff;
  --sidebar-border: #eaebf0;
  --sidebar-text: #63656e;
  --sidebar-text-secondary: #979ba5;
  --sidebar-btn-bg: #fff;
  --sidebar-btn-border: #dcdee5;
  --sidebar-btn-hover-bg: #eaebf0;
  --sidebar-btn-hover-border: #c4c6cc;
  --sidebar-scroll-thumb: #dcdee5;
  --sidebar-item-hover: #f0f1f5;
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
  --main-bg: #f5f7fa;
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

.chatbot-sidebar-rail {
  position: absolute;
  top: 12px;
  left: 12px;
  z-index: 180;
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 64px;
  background: #fff;
  border: 1px solid rgb(0 0 0 / 4%);
  border-radius: 999px;
  box-shadow: 0 2px 12px rgb(0 0 0 / 12%);

  .rail-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    padding: 0;
    font-size: 16px;
    color: #63656e;
    cursor: pointer;
    background: transparent;
    border: none;
    border-radius: 50%;
    transition: background 0.2s, color 0.2s;

    .hcm-icon {
      font-size: 16px;
    }

    &:hover {
      color: #313238;
    }
  }

  .rail-expand-icon {
    font-size: 16px;
    transform: rotate(-90deg);
  }

  .rail-new-chat-icon {
    width: 16px;
    height: 16px;
    font-size: 16px;
  }
}

.chatbot-sidebar {
  position: relative;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  width: 256px;
  height: 100%;
  padding: 12px;
  color: var(--sidebar-text);
  background: var(--sidebar-bg);
  border-right: 1px solid var(--sidebar-border);
  transition: width 0.25s ease;

  .sidebar-search {
    position: relative;
    flex-shrink: 0;
    margin-bottom: 8px;
  }

  // 悬浮贴在侧栏右边缘的收起按钮（跨过分割线，圆形浮起），与搜索行垂直对齐
  .sidebar-collapse-btn {
    position: absolute;
    top: 12px;
    right: -20px;
    z-index: 20;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    color: #63656e;
    cursor: pointer;
    background: #fff;
    border: 1px solid var(--sidebar-border);
    border-radius: 50%;
    box-shadow: 0 2px 6px rgb(0 0 0 / 10%);
    transition: color 0.2s, background 0.2s;

    &:hover {
      color: #313238;
    }

    .collapse-icon {
      font-size: 16px;
      transform: rotate(90deg);
    }
  }

  .sidebar-search-input {
    width: 100%;
    height: 32px;
    padding: 0 32px 0 10px;
    font-size: 12px;
    color: var(--sidebar-input-color);
    background: #f0f1f5;
    border: none;
    border-radius: 4px;
    outline: none;

    &::placeholder {
      color: #979ba5;
    }
  }

  .sidebar-search-icon {
    position: absolute;
    top: 50%;
    right: 10px;
    width: 16px;
    height: 16px;
    font-size: 16px;
    color: #979ba5;
    pointer-events: none;
    transform: translateY(-50%);
  }

  .sidebar-new-btn {
    display: flex;
    gap: 6px;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 36px;
    padding: 0 16px;
    font-size: 14px;
    font-weight: 600;
    color: #fff;
    cursor: pointer;
    background: linear-gradient(90deg, #0061a5 0%, #0d99ff 100%);
    border: none;
    border-radius: 4px;
    box-shadow: 0 2px 6px rgb(58 132 255 / 20%);
    transition: opacity 0.2s;

    .sidebar-new-btn-icon {
      width: 16px;
      height: 16px;
      font-size: 16px;
    }

    &:hover {
      opacity: 0.92;
    }
  }

  .sidebar-sessions {
    flex: 1;
    margin-top: 12px;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: var(--sidebar-scroll-thumb) transparent;
  }

  .session-empty {
    padding: 24px 8px;
    font-size: 12px;
    color: var(--sidebar-text-secondary);
    text-align: center;
  }

  .session-group {
    margin-bottom: 8px;
  }

  .session-group-label {
    padding: 8px 8px 4px;
    font-size: 12px;
    color: var(--sidebar-text-secondary);
  }

  .session-folder-header {
    display: flex;
    gap: 6px;
    align-items: center;
    height: 32px;
    padding: 0 8px;
    cursor: pointer;
    border-radius: 4px;
    margin-bottom: 2px;

    &:hover {
      background: var(--sidebar-item-hover);
    }
  }

  .session-folder-icon {
    flex-shrink: 0;
    font-size: 14px;
    color: var(--sidebar-text-secondary);
  }

  .session-folder-name {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    font-size: 12px;
    color: var(--sidebar-input-color, #313238);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .session-folder-count {
    flex-shrink: 0;
    min-width: 16px;
    height: 16px;
    padding: 0 6px;
    font-size: 10px;
    line-height: 16px;
    color: #4d4f56;
    text-align: center;
    background: #f0f1f5;
    border-radius: 8px;
  }

  .session-folder-header:hover .session-folder-count {
    background: #eaebf0;
  }

  .session-folder-children {
    position: relative;
    padding-left: 24px;

    &::before {
      position: absolute;
      top: 0;
      bottom: 8px;
      left: 14px;
      width: 0;
      content: '';
      border-left: 1px dashed #dcdee5;
    }
  }
}

.chatbot-main {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  height: 100%;
  background: var(--main-bg);
}

.chatbot-chat {
  position: relative;
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;

  .chatbot-account-echo {
    flex-shrink: 0;
  }

  .chatbot-chat-messages {
    flex: 1;
    padding: 16px 0 16px 16px;
    overflow-y: auto;
    scrollbar-color: var(--chat-scroll-thumb) transparent;
    scrollbar-width: thin;

    // 全页态：自定义消息卡用投影、去描边（浮窗态保留卡片自身的 1px 描边）
    :deep(.custom-msg-card) {
      border: none;
      box-shadow: 0 12px 32px 0 rgb(0 0 0 / 4%);
    }

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

  .chat-blank {
    height: 100%;
  }

  .home-default {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;

    // 左右留白随视口收缩（响应式），避免宽屏时留白过大
    padding: 24px clamp(16px, 4vw, 40px);
    user-select: none;

    .welcome {
      display: flex;
      flex-direction: column;
      align-items: center;

      .welcome-icon {
        width: 48px;
        height: 48px;
      }

      .welcome-title {
        margin-top: 12px;
        font-size: 20px;
        font-weight: 600;
        color: var(--chat-empty-title);
      }

      .welcome-tips {
        margin-top: 8px;
        font-size: 14px;
        color: var(--chat-empty-desc);
      }
    }

    .big-cards {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
      gap: 12px;
      width: 100%;
      max-width: 960px;
      margin-top: 20px;
    }

    .big-card {
      display: flex;
      gap: 12px;
      align-items: center;
      padding: 16px;
      text-align: left;
      cursor: pointer;
      background: #fff;
      border: 1px solid var(--sidebar-border);
      border-radius: 8px;
      transition: border-color 0.2s, box-shadow 0.2s;

      &:hover {
        border-color: var(--sidebar-item-active-color);
        box-shadow: 0 2px 8px rgb(0 0 0 / 8%);
      }

      &:disabled {
        cursor: not-allowed;
        opacity: 0.6;
      }

      .big-card-icon {
        flex-shrink: 0;
        font-size: 24px;
        color: var(--sidebar-item-active-color);
      }

      .big-card-body {
        display: flex;
        flex-direction: column;
        gap: 4px;
        min-width: 0;
      }

      .big-card-title {
        font-size: 14px;
        font-weight: 600;
        color: var(--chat-empty-title);
      }

      .big-card-desc {
        overflow: hidden;
        font-size: 12px;
        color: var(--chat-empty-desc);
        text-overflow: ellipsis;
        white-space: nowrap;
      }
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

  .prompt-chips {
    padding: 0 16px;

    // 内层宽度与 chat-x 输入框（.chat-input：width 100% / max-width 1000px 居中）保持一致
    .prompt-chips-inner {
      display: flex;
      gap: 8px;
      align-items: center;
      width: 100%;
      max-width: 1000px;
      margin: 0 auto;
    }

    .chips-viewport {
      flex: 1;
      min-width: 0;
      overflow: hidden;
    }

    .chips-track {
      display: flex;
      gap: 8px;
      width: max-content;
      transition: transform 0.25s ease;
    }

    .prompt-chip {
      display: inline-flex;
      flex-shrink: 0;
      gap: 4px;
      align-items: center;
      height: 28px;
      padding: 0 10px;
      font-size: 12px;
      color: var(--sidebar-text);
      white-space: nowrap;
      cursor: pointer;
      background: #fff;
      border: 1px solid var(--sidebar-border);
      border-radius: 14px;
      transition: background 0.2s, border-color 0.2s;

      &:hover {
        color: var(--sidebar-item-active-color);
        background: var(--sidebar-item-active-bg);
        border-color: var(--sidebar-item-active-color);
      }

      .prompt-chip-icon {
        font-size: 14px;
        color: #699df4;
      }
    }

    // 单按钮翻页：圆角方形浅灰底，箭头方向随当前段切换
    .chip-pager {
      display: inline-flex;
      flex-shrink: 0;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      font-size: 16px;
      color: var(--sidebar-text-secondary);
      cursor: pointer;
      background: #f0f1f5;
      border: none;
      border-radius: 6px;
      transition: background 0.2s, color 0.2s;

      &:hover {
        color: var(--sidebar-text);
        background: var(--sidebar-actions-hover);
      }
    }
  }

  // 场景 chip 绝对定位到编辑器首行左侧（让位逻辑由 chat-input-box 的 input-indent 处理）
  .chatbot-chat-input {
    .scene-chip {
      position: absolute;
      top: 6px;
      left: 8px;
      z-index: 3;
      display: inline-flex;
      gap: 4px;
      align-items: center;
      height: 20px;
      padding: 0 8px;
      font-size: 12px;
      color: var(--sidebar-item-active-color);
      background: var(--sidebar-item-active-bg);
      border: 1px solid var(--sidebar-item-active-color);
      border-radius: 10px;

      .scene-chip-close {
        width: 14px;
        height: 14px;
        cursor: pointer;
      }
    }
  }

  .chatbot-footer {
    flex-shrink: 0;
    padding: 0 16px 12px;
    font-size: 12px;
    line-height: 18px;
    color: var(--chat-empty-desc);
    text-align: center;

    // w-name 渲染为文字按钮，需与说明文案基线对齐、字号一致
    :deep(.bk-button) {
      height: auto;
      padding: 0;
      font-size: 12px;
      line-height: 18px;
      vertical-align: baseline;
    }
  }
}
</style>
