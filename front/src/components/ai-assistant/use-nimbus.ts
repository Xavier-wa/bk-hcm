// 复刻自 ai-blueking composables/use-nimbus.ts：悬浮球位置、点击/拖拽区分、锚定半隐逻辑

import { nextTick, onMounted, onUnmounted, ref, type Ref } from 'vue';

/**
 * 悬浮球交互逻辑
 *
 * The isMinimize parameter 由外部（defineModel）传入并双向驱动锚定态。
 * The onClick parameter 在「点击（非拖拽）」时触发。
 */
export function useNimbus(isMinimize: Ref<boolean>, onClick: () => void) {
  const nimbusWidth = 48;
  const nimbusHeight = 48;
  const initNimbusTop = window.innerHeight - nimbusHeight - 40;
  const initNimbusLeft = window.innerWidth - nimbusWidth - 16;
  const nimbusLeft = ref(initNimbusLeft);
  const nimbusTop = ref(initNimbusTop);
  const diffY = ref(0);

  const isHovering = ref(false);
  const isDragging = ref(false);
  let dragStartTime = 0;

  // 视口变化：Y 相对底部保持偏移，X 始终贴右（锚定态的半隐由 CSS transform 控制，不改 left）
  const handleResize = () => {
    nextTick(() => {
      nimbusTop.value = window.innerHeight - nimbusHeight - 40 + diffY.value;
      nimbusLeft.value = window.innerWidth - nimbusWidth - 16;
    });
  };

  // 点击：拖拽后不触发 click
  const handleClick = () => {
    if (isDragging.value) {
      isDragging.value = false;
      return;
    }
    onClick();
  };

  // 切换锚定态（最小化为锚点）；锚定半隐与恢复均由 CSS transform 控制，left 始终保持初始位置，
  // 恢复时移除 is-minimize、transform 归零，必然回到初始完整展示位置
  const handleMinimize = () => {
    isMinimize.value = !isMinimize.value;
  };

  const handleDragging = (x: number, y: number) => {
    nimbusLeft.value = x;
    nimbusTop.value = y;
    diffY.value = y - initNimbusTop;
  };

  const handleMouseEnter = () => {
    isHovering.value = true;
  };

  const handleMouseLeave = () => {
    isHovering.value = false;
  };

  const handleMouseDown = () => {
    dragStartTime = Date.now();
    isDragging.value = false;
  };

  // 按下到抬起超过 200ms 视为拖拽
  const handleMouseUp = () => {
    if (Date.now() - dragStartTime > 200) {
      isDragging.value = true;
    }
  };

  onMounted(() => {
    window.addEventListener('resize', handleResize);
    handleResize();
  });

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize);
  });

  return {
    nimbusLeft,
    nimbusTop,
    isHovering,
    isDragging,
    handleClick,
    handleMinimize,
    handleDragging,
    handleMouseEnter,
    handleMouseLeave,
    handleMouseDown,
    handleMouseUp,
  };
}
