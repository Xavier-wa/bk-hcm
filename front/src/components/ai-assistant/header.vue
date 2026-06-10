<script setup lang="ts">
import { computed, useTemplateRef } from 'vue';

import { CloseLine } from 'bkui-vue/lib/icon';

import avatar from '@/assets/image/ai-assistant-avatar.png';

defineOptions({
  name: 'AiAssistantHeader',
});

const props = withDefaults(
  defineProps<{
    /** 标题 */
    title?: string;
    /** 是否可拖拽（控制 cursor:move） */
    draggable?: boolean;
    /** 是否压缩态（高度已缩小） */
    isCompressionHeight?: boolean;
    /** 显示新建会话 */
    showNewChatIcon?: boolean;
    /** 显示历史会话 */
    showHistoryIcon?: boolean;
    /** 显示缩小高度/恢复尺寸 */
    showCompressionIcon?: boolean;
  }>(),
  {
    title: 'AI 小鲸',
    draggable: true,
    isCompressionHeight: false,
    showNewChatIcon: true,
    showHistoryIcon: true,
    showCompressionIcon: true,
  },
);

const emit = defineEmits<{
  'new-chat': [];
  'history-click': [event: Event];
  'toggle-compression': [];
  close: [];
}>();

const historyIconRef = useTemplateRef<HTMLElement>('historyIconRef');

const compressionIcon = computed(() => (props.isCompressionHeight ? 'bkhcm-icon-fullscreen' : 'bkhcm-icon-zoomout'));
const compressionTooltip = computed(() => (props.isCompressionHeight ? '恢复默认尺寸' : '缩小高度'));

defineExpose({
  historyIconRef,
});
</script>

<template>
  <div class="ai-assistant-header drag-handle" :class="{ draggable: props.draggable }">
    <div class="left-section">
      <div class="logo">
        <img alt="logo" :src="avatar" />
      </div>
      <div class="title">{{ props.title }}</div>
    </div>
    <div class="right-section">
      <i
        v-if="props.showNewChatIcon"
        v-bk-tooltips="{ content: '新建会话', boundary: 'parent' }"
        class="action-icon hcm-icon bkhcm-icon-chat-plus"
        @click="emit('new-chat')"
      ></i>
      <i
        v-if="props.showHistoryIcon"
        ref="historyIconRef"
        v-bk-tooltips="{ content: '历史会话', boundary: 'parent' }"
        class="action-icon hcm-icon bkhcm-icon-history"
        @click="emit('history-click', $event)"
      ></i>
      <i
        v-if="props.showCompressionIcon"
        v-bk-tooltips="{ content: compressionTooltip, boundary: 'parent' }"
        class="action-icon hcm-icon"
        :class="compressionIcon"
        @click="emit('toggle-compression')"
      ></i>
      <CloseLine v-bk-tooltips="{ content: '关闭', boundary: 'parent' }" class="action-icon" @click="emit('close')" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.ai-assistant-header {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 14px;
  border-bottom: none;

  &.draggable {
    cursor: move;
  }

  .left-section {
    display: flex;
    flex: 1;
    gap: 4px;
    align-items: center;
    min-width: 0;

    .logo {
      width: 32px;
      height: 32px;

      img {
        width: 100%;
        height: 100%;
        object-fit: cover;
      }
    }

    .title {
      max-width: calc(100% - 65px);
      overflow: hidden;
      font-size: 14px;
      font-weight: 600;
      line-height: 20px;
      color: #4d4f56;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .right-section {
    display: flex;
    gap: 12px;
  }

  .action-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    font-size: 14px;
    color: #63656e;
    cursor: pointer;
    border-radius: 2px;

    &.bkhcm-icon-history {
      font-size: 16px;
    }

    &:hover {
      color: #4d4f56;
      background: #eaebf0;
    }

    &.disabled {
      color: #c4c6cc;
      cursor: not-allowed;

      &:hover {
        color: #c4c6cc;
        background: transparent;
      }
    }
  }
}
</style>
