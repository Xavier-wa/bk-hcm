<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, provide, ref, useTemplateRef } from 'vue';

import { useChatbot } from '@/hooks/chatbot/use-chatbot';
import { ChatbotKey } from '@/hooks/chatbot/provide';
import ChatMessageList from '@/components/chatbot/chat-message-list.vue';
import ChatInputBox from '@/components/chatbot/chat-input-box.vue';
import DraggableContainer from './draggable-container.vue';
import AiAssistantHeader from './header.vue';
import AiAssistantNimbus from './nimbus.vue';
import AiAssistantHistoryDropdown from './history-dropdown.vue';
import { isTogglePanelShortcut } from './utils';

import type { AiAssistantExpose } from './types';

defineOptions({
  name: 'AiAssistant',
});

const props = withDefaults(
  defineProps<{
    /** 标题栏标题 */
    title?: string;
    /** 空态提示文案 */
    emptyHint?: string;
    /** 场景标识（如 host_apply）：会话列表按该场景加载，发送时默认按该场景创建会话 */
    sceneTag?: string;
  }>(),
  {
    title: 'AI 小鲸',
    emptyHint: '你好，我是海垒 AI 助手，有什么可以帮你的吗？',
    sceneTag: '',
  },
);

// 浮窗自持有 useChatbot 实例并 provide，与全页相互独立；通过 ChatbotKey 供原子组件 inject。
// 传入 sceneTag 使会话列表按场景加载、新会话归入对应场景。
const chatbot = useChatbot({ sceneTag: props.sceneTag });
provide(ChatbotKey, chatbot);

const { messages, isLoadingHistory, sessions, currentSessionCode, sendMessage, goHome, switchSession, initSessions } =
  chatbot;

const draggableContainerRef = useTemplateRef<InstanceType<typeof DraggableContainer>>('draggableContainerRef');

// 状态机：面板显隐 / 悬浮球锚定 / 高度压缩 / 历史下拉
const panelVisible = ref(false);
const nimbusMinimized = ref(false);
const isCompressed = ref(false);
const historyOpen = ref(false);

const hasMessages = computed(() => messages.value.length > 0);
const isEmpty = computed(() => messages.value.length === 0 && !isLoadingHistory.value);

// 惰性加载：会话列表在首次唤起（或深链初始化）时按场景拉取一次，避免每个挂载页面无谓请求
const sessionsInited = ref(false);
const ensureSessions = () => {
  if (sessionsInited.value) return;
  sessionsInited.value = true;
  initSessions();
};

// 深链初始化：加载并切换到指定会话，同时标记已初始化，避免 show() 再次重复拉取
const initSessionsWithScene = (sessionCode?: string) => {
  sessionsInited.value = true;
  initSessions(sessionCode);
};

const show = () => {
  panelVisible.value = true;
  ensureSessions();
};

const hide = () => {
  panelVisible.value = false;
  historyOpen.value = false;
};

const toggle = () => {
  panelVisible.value ? hide() : show();
};

const handleNimbusClick = () => {
  show();
};

const handleToggleCompression = () => {
  draggableContainerRef.value?.toggleCompression();
};

const handleCompressionChange = (value: boolean) => {
  isCompressed.value = value;
};

const handleNewChat = () => {
  historyOpen.value = false;
  goHome();
};

const handleHistoryClick = () => {
  historyOpen.value = !historyOpen.value;
};

const handleHistorySelect = (sessionCode: string) => {
  historyOpen.value = false;
  switchSession(sessionCode);
};

const handleSend = (text: string) => {
  sendMessage(text);
};

// 快捷键 Cmd/Ctrl + I 切换面板显隐
const handleKeydown = (event: KeyboardEvent) => {
  if (isTogglePanelShortcut(event)) {
    event.preventDefault();
    toggle();
  }
};

onMounted(() => {
  window.addEventListener('keydown', handleKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown);
});

defineExpose<AiAssistantExpose>({
  show,
  hide,
  toggle,
  initSessions: initSessionsWithScene,
  switchSession,
});
</script>

<template>
  <teleport to="body">
    <div class="ai-assistant">
      <!-- 可拖拽 + 可缩放容器 -->
      <DraggableContainer
        ref="draggableContainerRef"
        :default-width="400"
        :default-y="0"
        :max-width-percent="80"
        :visible="panelVisible"
        @compression-change="handleCompressionChange"
      >
        <div class="ai-assistant-panel" :class="{ 'has-messages': hasMessages }">
          <AiAssistantHeader
            :title="props.title"
            :is-compression-height="isCompressed"
            @close="hide"
            @history-click="handleHistoryClick"
            @new-chat="handleNewChat"
            @toggle-compression="handleToggleCompression"
          />
          <div class="ai-assistant-content">
            <div class="ai-assistant-body">
              <div v-if="isLoadingHistory" class="aa-loading">
                <div class="aa-loading-spinner" />
                <span>加载会话历史...</span>
              </div>
              <div v-else-if="isEmpty" class="aa-empty">{{ props.emptyHint }}</div>
              <div v-else class="aa-messages">
                <ChatMessageList />
              </div>
            </div>
            <ChatInputBox @send="handleSend" />
          </div>
          <AiAssistantHistoryDropdown
            :visible="historyOpen"
            :sessions="sessions"
            :current-session-code="currentSessionCode"
            @select="handleHistorySelect"
            @close="historyOpen = false"
          />
        </div>
      </DraggableContainer>

      <!-- 悬浮球 -->
      <AiAssistantNimbus
        v-model:is-minimize="nimbusMinimized"
        :is-panel-show="panelVisible"
        @click="handleNimbusClick"
      />
    </div>
  </teleport>
</template>

<style lang="scss" scoped>
.ai-assistant {
  position: fixed;
  inset: 0;
  z-index: 10000;
  width: 100vw;
  height: 100vh;
  pointer-events: none;
}

.ai-assistant-panel {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background: linear-gradient(
      154deg,
      rgb(163 150 234 / 16%) 0%,
      rgb(187 176 234 / 12%) 8%,
      rgb(95 107 246 / 8%) 13%,
      rgb(35 93 250 / 4%) 35%,
      transparent 50%
    ),
    #fff;
  border-radius: 12px;

  &.has-messages {
    background: #fff;
  }
}

.ai-assistant-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

.ai-assistant-body {
  flex: 1;
  min-height: 0;
}

.aa-messages {
  height: 100%;
  padding: 8px 0 8px 8px;
  overflow-y: auto;
  scrollbar-color: #dcdee5 transparent;
  scrollbar-width: thin;
}

.aa-loading {
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 12px;
  color: #979ba5;

  .aa-loading-spinner {
    width: 24px;
    height: 24px;
    border: 3px solid #eaebf0;
    border-top-color: #3a84ff;
    border-radius: 50%;
    animation: aa-spin 0.8s linear infinite;
  }
}

.aa-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 0 24px;
  font-size: 13px;
  color: #979ba5;
  text-align: center;
}

@keyframes aa-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
