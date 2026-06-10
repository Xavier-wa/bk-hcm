<script setup lang="ts">
import { computed, ref, watch } from 'vue';

import VueDraggableResizable from 'vue-draggable-resizable';

import { useDraggable } from './use-draggable';

import type { DraggableContainerEmits, DraggableContainerProps } from './types';

import 'vue-draggable-resizable/style.css';

const props = withDefaults(defineProps<DraggableContainerProps>(), {
  visible: false,
  draggable: true,
  resizable: true,
  defaultWidth: 400,
  defaultHeight: undefined,
  defaultX: undefined,
  defaultY: 0,
  minWidth: 400,
  minHeight: 400,
  maxWidth: undefined,
  maxWidthPercent: 80,
  compressedHeight: 800,
  compressedPadding: 0,
  dragHandle: '.drag-handle',
  className: '',
});

const emit = defineEmits<DraggableContainerEmits>();

// 拖拽/缩放进行中的标记（用于临时屏蔽 iframe 指针事件）
const isDraggingOrResizing = ref(false);

const {
  minWidth,
  minHeight,
  maxWidth,
  top,
  left,
  width,
  height,
  isCompressed,
  handleDragging,
  handleResizing,
  handleDragStop,
  handleResizeStop,
  toggleCompression,
  updatePosition,
  updateSize,
  updatePositionAndSize,
} = useDraggable(
  {
    initWidth: props.defaultWidth,
    minWidth: props.minWidth,
    minHeight: props.minHeight,
    maxWidthPercent: props.maxWidthPercent,
    compressedHeight: props.compressedHeight,
    defaultHeight: props.defaultHeight,
    defaultTop: props.defaultY,
    defaultLeft: props.defaultX,
    compressedPadding: props.compressedPadding,
  },
  {
    onDragStop: (position) => emit('drag-stop', position),
    onResizeStop: (position) => emit('resize-stop', position),
    onDragging: (position) => emit('dragging', position),
    onResizing: (position) => emit('resizing', position),
  },
);

const rootStyle = computed(() => {
  const maxWidthValue =
    typeof props.maxWidth === 'number' ? `${props.maxWidth}px` : props.maxWidth ?? `${maxWidth.value}px`;
  return {
    '--ai-assistant-max-width': maxWidthValue,
  };
});

// 拖拽/缩放时禁用页面内所有 iframe 的指针事件，避免鼠标经过 iframe 导致事件丢失
const disableIframePointerEvents = (): void => {
  document.querySelectorAll('iframe').forEach((iframe) => {
    iframe.style.pointerEvents = 'none';
  });
};

const enableIframePointerEvents = (): void => {
  document.querySelectorAll('iframe').forEach((iframe) => {
    iframe.style.pointerEvents = '';
  });
};

const handleDraggingWithIframe = (x: number, y: number): void => {
  if (!isDraggingOrResizing.value) {
    isDraggingOrResizing.value = true;
    disableIframePointerEvents();
  }
  handleDragging(x, y);
};

const handleResizingWithIframe = (x: number, y: number, w: number, h: number): void => {
  if (!isDraggingOrResizing.value) {
    isDraggingOrResizing.value = true;
    disableIframePointerEvents();
  }
  handleResizing(x, y, w, h);
};

const handleDragStopWithIframe = (x: number, y: number): void => {
  handleDragStop(x, y);
  isDraggingOrResizing.value = false;
  enableIframePointerEvents();
};

const handleResizeStopWithIframe = (x: number, y: number, w: number, h: number): void => {
  handleResizeStop(x, y, w, h);
  isDraggingOrResizing.value = false;
  enableIframePointerEvents();
};

watch(isCompressed, (newValue) => {
  emit('compression-change', newValue);
});

defineExpose({
  updatePosition,
  updateSize,
  updatePositionAndSize,
  toggleCompression,
  isCompressed,
});
</script>

<template>
  <VueDraggableResizable
    v-show="props.visible"
    :active="props.visible"
    :class="['ai-assistant-draggable-wrapper', props.className]"
    class-name="ai-assistant-draggable-inner"
    :drag-handle="props.dragHandle"
    :draggable="props.draggable"
    :h="height"
    :max-width="maxWidth"
    :min-height="minHeight"
    :min-width="minWidth"
    :parent="true"
    :prevent-deactivation="true"
    :resizable="props.resizable"
    :style="rootStyle"
    :w="width"
    :x="left"
    :y="top"
    @drag-stop="handleDragStopWithIframe"
    @dragging="handleDraggingWithIframe"
    @resize-stop="handleResizeStopWithIframe"
    @resizing="handleResizingWithIframe"
  >
    <div class="ai-assistant-draggable-content">
      <slot />
    </div>
  </VueDraggableResizable>
</template>

<style lang="scss" scoped>
.ai-assistant-draggable-wrapper {
  pointer-events: auto;
}

.ai-assistant-draggable-inner {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  pointer-events: auto;
  background: transparent;
  border-radius: 12px;
  box-shadow: 0 2px 12px 0 rgb(0 0 0 / 20%);

  :deep(.handle) {
    background: transparent;
    border: none;

    &.handle-ml,
    &.handle-mr {
      top: 0;
      height: 100%;
      margin-top: 0;
      cursor: ew-resize;
    }

    &.handle-tm,
    &.handle-bm {
      left: 0;
      width: 100%;
      margin-left: 0;
      cursor: ns-resize;
    }

    &.handle-tl,
    &.handle-br {
      cursor: nwse-resize;
    }

    &.handle-tr {
      top: -5px;
      right: -5px;
    }

    &.handle-tl {
      top: -5px;
      left: -5px;
    }

    &.handle-bl {
      bottom: -5px;
      left: -5px;
    }

    &.handle-br {
      right: -5px;
      bottom: -5px;
    }

    &.handle-tr,
    &.handle-bl {
      cursor: nesw-resize;
    }
  }
}

.ai-assistant-draggable-content {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
}

:deep(.vdr) {
  background: transparent;
  border: none;
}
</style>
