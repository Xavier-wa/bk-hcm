<script setup lang="ts">
import { computed, onMounted, watch } from 'vue';
// formatDuration 走组件库导出（格式 1s851ms），与工具行耗时同一个来源，两处不各写一套
import { formatDuration, MessageStatus } from '@blueking/chat-x';
import { AngleDown, AngleUp } from 'bkui-vue/lib/icon';

import type { ProcessThinkingMessage } from '@/hooks/chatbot/types';
import { useFollowScroll } from '@/hooks/chatbot/use-follow-scroll';

const props = defineProps<{
  message: ProcessThinkingMessage;
}>();

const emit = defineEmits<{
  toggle: [];
}>();

// 展开态由 useProcessZone 持有：要独立于过程区总开关长期保留，放组件内会随重挂载丢失
const expanded = computed(() => props.message.__expanded);

// 思考正文是手写节点，流式增量不会触发 chat-x 的跟随滚动，按正文长度变化补一次贴底。
// 只看正文长度，不看展开态：用户手动展开是为了读过程，不该被拽到底部
const followScroll = useFollowScroll();
watch(
  () => props.message.__text.length,
  () => followScroll(),
  { flush: 'post' },
);
onMounted(() => followScroll());

const title = computed(() => {
  const label = props.message.status === MessageStatus.Streaming ? '思考中' : '已思考完成';
  // AG-UI 的 REASONING_END 只有 messageId，/agui 的耗时是前端从 REASONING_START 掐到 END 的墙上时钟；
  // /history 重放不出这段时钟（快照消息只有 id/role/content），此时不画耗时段，不编 0ms
  if (props.message.__duration === undefined) return label;
  return `${label} (耗时 ：${formatDuration(props.message.__duration)})`;
});
</script>

<template>
  <!-- 过程区收起时仅留一个空壳（外层 .ai-message-item 由 process-hidden 一起收掉），不从消息列表里摘除本条 -->
  <div v-if="!message.__visible" class="process-thinking process-hidden" />
  <div v-else class="process-thinking">
    <div class="process-thinking-title" @click="emit('toggle')">
      <span class="process-thinking-title-text">{{ title }}</span>
      <span class="process-thinking-arrow" aria-hidden="true">
        <AngleUp v-if="expanded" />
        <AngleDown v-else />
      </span>
    </div>
    <div v-if="expanded && message.__text" class="process-thinking-content">{{ message.__text }}</div>
  </div>
</template>

<style scoped lang="scss">
.process-thinking {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.process-thinking-title {
  display: flex;
  align-items: center;
  width: fit-content;
  max-width: 100%;
  height: 28px;
  padding: 0 10px;
  font-size: 12px;
  line-height: 20px;
  color: #4d4f56;
  cursor: pointer;
  user-select: none;
  background: #f0f1f5;
  border-radius: 4px;
}

.process-thinking-title-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

// bkui Angle* 画在 1024 viewBox 居中，左右各有大块空白；裁掉后箭头才与标题贴合。
// AngleUp / AngleDown 的水平边界一致（288→736，宽 448/1024），一条规则通吃两个状态，切换时不跳字。
.process-thinking-arrow {
  display: block;
  flex-shrink: 0;
  width: 7px;
  height: 16px;
  margin-left: 4px;
  overflow: hidden;
  font-size: 16px;
  line-height: 0;
  color: #4d4f56;

  :deep(svg) {
    display: block;
    margin-left: calc(-1em * 288 / 1024);
  }
}

.process-thinking-content {
  padding: 8px 12px;
  font-size: 12px;
  line-height: 20px;
  color: #979ba5;
  word-break: break-word;
  white-space: pre-wrap;
  background: #f5f7fa;
  border-radius: 2px;
}
</style>
