<script setup lang="ts">
import { computed } from 'vue';

import { type ChatSession } from '@/hooks/chatbot/use-chatbot';

defineOptions({
  name: 'AiAssistantHistoryDropdown',
});

const props = withDefaults(
  defineProps<{
    visible?: boolean;
    sessions?: ChatSession[];
    currentSessionCode?: string;
  }>(),
  {
    visible: false,
    sessions: () => [],
    currentSessionCode: '',
  },
);

const emit = defineEmits<{
  select: [sessionCode: string];
  close: [];
}>();

// 按更新时间倒序展示，最近会话置顶
const sortedSessions = computed(() =>
  [...props.sessions].sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()),
);

const handleSelect = (sessionCode: string) => {
  emit('select', sessionCode);
};
</script>

<template>
  <div v-if="props.visible" class="aa-history">
    <!-- 透明遮罩：点击空白处关闭 -->
    <div class="aa-history-mask" @click="emit('close')" />
    <div class="aa-history-panel">
      <div class="aa-history-title">历史会话</div>
      <div v-if="sortedSessions.length" class="aa-history-list">
        <div
          v-for="session in sortedSessions"
          :key="session.sessionCode"
          class="aa-history-item"
          :class="{ 'is-active': session.sessionCode === props.currentSessionCode }"
          :title="session.sessionName"
          @click="handleSelect(session.sessionCode)"
        >
          {{ session.sessionName }}
        </div>
      </div>
      <div v-else class="aa-history-empty">暂无历史会话</div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.aa-history {
  position: absolute;
  inset: 0;
  z-index: 5;
}

.aa-history-mask {
  position: absolute;
  inset: 0;
}

.aa-history-panel {
  position: absolute;
  top: 44px;
  right: 14px;
  display: flex;
  flex-direction: column;
  width: 240px;
  max-height: 320px;
  padding: 6px;
  background: #fff;
  border: 1px solid #eaebf0;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgb(0 0 0 / 12%);
}

.aa-history-title {
  flex-shrink: 0;
  padding: 6px 8px;
  font-size: 12px;
  color: #979ba5;
}

.aa-history-list {
  flex: 1;
  overflow-y: auto;
  scrollbar-color: #dcdee5 transparent;
  scrollbar-width: thin;
}

.aa-history-item {
  padding: 8px;
  overflow: hidden;
  font-size: 13px;
  color: #63656e;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  border-radius: 4px;

  &:hover {
    background: #f0f1f5;
  }

  &.is-active {
    color: #3a84ff;
    background: #e1ecff;
  }
}

.aa-history-empty {
  padding: 16px 8px;
  font-size: 12px;
  color: #979ba5;
  text-align: center;
}
</style>
