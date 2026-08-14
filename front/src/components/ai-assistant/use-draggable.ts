// 复刻自 ai-blueking containers/use-draggable.ts：浮窗的拖拽、缩放、几何与「缩小高度/恢复默认尺寸」逻辑

import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue';

import type { PositionAndSize, UseDraggableOptions } from './types';

interface UseDraggableCallbacks {
  onDragging?: (position: PositionAndSize) => void;
  onDragStop?: (position: PositionAndSize) => void;
  onResizeStop?: (position: PositionAndSize) => void;
  onResizing?: (position: PositionAndSize) => void;
}

/**
 * 可拖拽容器的逻辑 Hook
 *
 * 封装容器的拖拽、缩放、位置管理与压缩态（缩小高度）切换
 */
export function useDraggable(options: UseDraggableOptions = {}, callbacks?: UseDraggableCallbacks) {
  const initWidth = options.initWidth || 400;
  const minWidth = options.minWidth || 400;
  const minHeight = options.minHeight || 400;
  const maxWidthPercent = options.maxWidthPercent || 80;
  const compressedHeight = options.compressedHeight || 800;
  const compressedPadding = options.compressedPadding !== undefined ? options.compressedPadding : 0;

  // 初始位置：默认右对齐、占满视口高度
  const initialX = ref(options.defaultLeft !== undefined ? options.defaultLeft : window.innerWidth - initWidth);
  const initialTop = ref(options.defaultTop !== undefined ? options.defaultTop : 0);
  const initialHeight = ref(
    options.defaultHeight !== undefined
      ? options.defaultHeight
      : window.innerHeight - (options.defaultTop !== undefined ? options.defaultTop : 0),
  );
  const initialWidth = ref(initWidth);

  const top = ref(initialTop.value);
  const left = ref(initialX.value);
  const width = ref(initialWidth.value);
  const height = ref(initialHeight.value);
  const maxWidth = ref(Math.max(window.innerWidth * (maxWidthPercent / 100), width.value));
  const isCompressed = ref(false);
  const leftDiff = ref(0);

  const getPositionAndSize = (): PositionAndSize => ({
    x: left.value,
    y: top.value,
    width: width.value,
    height: height.value,
  });

  /** 拖拽中 */
  const handleDragging = (x: number, y: number): void => {
    left.value = x;
    top.value = y;
    leftDiff.value = x - (window.innerWidth - width.value);
    callbacks?.onDragging?.(getPositionAndSize());
  };

  /** 缩放中 */
  const handleResizing = (x: number, y: number, w: number, h: number): void => {
    left.value = x;
    top.value = y;
    width.value = Math.min(w, maxWidth.value);
    height.value = h;
    callbacks?.onResizing?.(getPositionAndSize());
  };

  /** 拖拽结束 */
  const handleDragStop = (x: number, y: number): void => {
    left.value = x;
    top.value = y;
    leftDiff.value = x - (window.innerWidth - width.value);
    callbacks?.onDragStop?.(getPositionAndSize());
  };

  /** 缩放结束 */
  const handleResizeStop = (x: number, y: number, w: number, h: number): void => {
    left.value = x;
    top.value = y;
    width.value = Math.min(w, maxWidth.value);
    height.value = h;
    callbacks?.onResizeStop?.(getPositionAndSize());
  };

  /** 视口尺寸变化：保持容器贴右，正常态高度跟随视口 */
  const handleWindowResize = (): void => {
    maxWidth.value = Math.max(window.innerWidth * (maxWidthPercent / 100), width.value);

    nextTick(() => {
      if (isCompressed.value) {
        left.value = window.innerWidth - width.value - compressedPadding;
        top.value = window.innerHeight - compressedHeight - compressedPadding;
      } else {
        const newLeft = window.innerWidth - width.value - leftDiff.value;
        left.value = Math.max(0, newLeft);
        setTimeout(() => {
          height.value = window.innerHeight - top.value;
        }, 0);
      }

      if (width.value > maxWidth.value) {
        width.value = maxWidth.value;
      }
    });
  };

  /** 切换压缩态：缩小高度 ↔ 恢复默认尺寸 */
  const toggleCompression = (): void => {
    if (isCompressed.value) {
      top.value = initialTop.value;
      nextTick(() => {
        height.value = initialHeight.value;
        left.value = initialX.value;
        width.value = initialWidth.value;
      });
    } else {
      top.value = window.innerHeight - compressedHeight - compressedPadding;
      left.value = initialX.value - compressedPadding;
      width.value = initWidth;
      height.value = compressedHeight;
    }
    isCompressed.value = !isCompressed.value;
  };

  /** 编程式更新位置 */
  const updatePosition = (x: number, y: number): void => {
    left.value = x;
    top.value = y;
    leftDiff.value = x - (window.innerWidth - width.value);
  };

  /** 编程式更新尺寸 */
  const updateSize = (w: number, h: number): void => {
    width.value = Math.max(minWidth, Math.min(w, maxWidth.value));
    height.value = Math.max(minHeight, h);
  };

  /** 同时更新位置与尺寸 */
  const updatePositionAndSize = (x: number, y: number, w: number, h: number): void => {
    updatePosition(x, y);
    updateSize(w, h);
  };

  onMounted(() => {
    window.addEventListener('resize', handleWindowResize);
  });

  onBeforeUnmount(() => {
    window.removeEventListener('resize', handleWindowResize);
  });

  return {
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
  };
}
