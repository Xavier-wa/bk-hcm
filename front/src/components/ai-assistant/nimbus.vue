<script setup lang="ts">
import { computed } from 'vue';

import VueDraggableResizable from 'vue-draggable-resizable';

import avatar from '@/assets/image/ai-assistant-avatar.png';

import { useNimbus } from './use-nimbus';
import { getTogglePanelShortcut } from './utils';

defineOptions({
  name: 'AiAssistantNimbus',
});

// 是否锚定（最小化为锚点）—— 双向绑定
const isMinimize = defineModel<boolean>('isMinimize', { default: false });

const props = withDefaults(
  defineProps<{
    /** 面板是否展开（展开时悬浮球拖拽失活） */
    isPanelShow: boolean;
    /** 悬浮球尺寸 */
    size?: 'small' | 'normal' | 'large';
  }>(),
  {
    size: 'normal',
  },
);

const emit = defineEmits<{
  click: [];
}>();

const sizeMap = {
  small: {
    container: 40,
    wrapper: 32,
    img: 24,
    mini: { size: 14, fontSize: 10, right: -4 },
  },
  normal: {
    container: 48,
    wrapper: 40,
    img: 32,
    mini: { size: 16, fontSize: 12, right: -6 },
  },
  large: {
    container: 56,
    wrapper: 48,
    img: 40,
    mini: { size: 18, fontSize: 14, right: -6 },
  },
};

const nimbusDimensions = computed(() => sizeMap[props.size]);

const {
  nimbusLeft,
  nimbusTop,
  handleClick,
  handleMinimize,
  handleDragging,
  handleMouseEnter,
  handleMouseLeave,
  handleMouseDown,
  handleMouseUp,
} = useNimbus(isMinimize, () => emit('click'));

const shortcutText = computed(() => getTogglePanelShortcut());
const minimizeTooltip = computed(() => (isMinimize.value ? '恢复默认大小' : '最小化，将缩成锚点'));
</script>

<template>
  <VueDraggableResizable
    :active="!isPanelShow"
    axis="y"
    :draggable="true"
    :h="nimbusDimensions.container"
    :parent="true"
    :prevent-deactivation="true"
    :resizable="false"
    :w="nimbusDimensions.container"
    :x="nimbusLeft"
    :y="nimbusTop"
    @dragging="handleDragging"
  >
    <div
      v-bk-tooltips="{ content: shortcutText, placement: 'left', disabled: isMinimize }"
      class="nimbus-container"
      :class="{ 'is-minimize': isMinimize }"
      @click="handleClick"
      @mousedown="handleMouseDown"
      @mouseenter="handleMouseEnter"
      @mouseleave="handleMouseLeave"
      @mouseup="handleMouseUp"
    >
      <div class="nimbus-avatar-wrapper">
        <img :width="nimbusDimensions.img" :height="nimbusDimensions.img" :src="avatar" alt="nimbus" />
      </div>
      <i
        v-bk-tooltips="{ content: minimizeTooltip, placement: 'top' }"
        class="nimbus-mini hcm-icon"
        :class="isMinimize ? 'bkhcm-icon-undo' : 'bkhcm-icon-minus'"
        @click.stop="handleMinimize"
      ></i>
    </div>
  </VueDraggableResizable>
</template>

<style lang="scss">
// 覆盖 vue-draggable-resizable 的默认虚线边框（全局）
.vdr {
  border: none !important;

  &.active {
    border: none !important;
  }
}
</style>

<style scoped lang="scss">
.nimbus-container {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: v-bind('`${nimbusDimensions.container}px`');
  height: v-bind('`${nimbusDimensions.container}px`');
  pointer-events: auto;
  cursor: pointer;
  background: #fff;
  border-radius: 50%;
  box-shadow: 0 0 4px 0 #1919291f;
  outline: none;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  transform: translateX(0);

  &:focus,
  &:focus-visible {
    outline: none;
  }

  .nimbus-avatar-wrapper {
    display: flex;
    align-items: center;
    justify-content: center;
    width: v-bind('`${nimbusDimensions.wrapper}px`');
    height: v-bind('`${nimbusDimensions.wrapper}px`');
    background: #f0f5ff;
    border-radius: 50%;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .nimbus-mini {
    position: absolute;
    top: 0;
    right: v-bind('`${nimbusDimensions.mini.right}px`');
    display: flex;
    align-items: center;
    justify-content: center;
    width: v-bind('`${nimbusDimensions.mini.size}px`');
    height: v-bind('`${nimbusDimensions.mini.size}px`');
    font-size: v-bind('`${nimbusDimensions.mini.fontSize}px`');
    color: #979ba5;
    pointer-events: none;
    background: #fff;
    border-radius: 50%;
    box-shadow: 0 2px 6px 0 #0000001a;
    opacity: 0;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  &:hover {
    .nimbus-mini {
      pointer-events: auto;
      opacity: 1;
    }
  }

  // 锚定态：基于初始位置（距右 16px）右移半隐，仅露出约 22px
  &.is-minimize {
    transform: translateX(42px);

    // 锚定态 hover：完整露出并贴近右边缘（留约 10px 缝隙），保证右上角恢复按钮也在视口内可点
    &:hover {
      transform: translateX(6px);
    }
  }
}
</style>
