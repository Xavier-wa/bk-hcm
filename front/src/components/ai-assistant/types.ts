// ai-assistant 浮窗组件的公共类型定义（壳层）

/** 位置与尺寸 */
export interface PositionAndSize {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** useDraggable 配置项 */
export interface UseDraggableOptions {
  /** 初始宽度 */
  initWidth?: number;
  /** 最小宽度 */
  minWidth?: number;
  /** 最小高度 */
  minHeight?: number;
  /** 最大宽度占视口百分比 */
  maxWidthPercent?: number;
  /** 压缩态高度 */
  compressedHeight?: number;
  /** 默认高度（不传则占满视口高度） */
  defaultHeight?: number;
  /** 默认 top */
  defaultTop?: number;
  /** 默认 left */
  defaultLeft?: number;
  /** 压缩态边距 */
  compressedPadding?: number;
}

/** 拖拽容器 props */
export interface DraggableContainerProps {
  /** 是否可见 */
  visible?: boolean;
  /** 是否可拖拽 */
  draggable?: boolean;
  /** 是否可缩放 */
  resizable?: boolean;
  /** 默认宽度 */
  defaultWidth?: number;
  /** 默认高度 */
  defaultHeight?: number;
  /** 默认 X */
  defaultX?: number;
  /** 默认 Y */
  defaultY?: number;
  /** 最小宽度 */
  minWidth?: number;
  /** 最小高度 */
  minHeight?: number;
  /** 最大宽度（数值或字符串），不传则按百分比计算 */
  maxWidth?: number | string;
  /** 最大宽度占视口百分比 */
  maxWidthPercent?: number;
  /** 压缩态高度 */
  compressedHeight?: number;
  /** 压缩态边距 */
  compressedPadding?: number;
  /** 拖拽手柄选择器 */
  dragHandle?: string;
  /** 额外类名 */
  className?: string;
}

/** 拖拽容器 emits */
export interface DraggableContainerEmits {
  'drag-stop': [position: PositionAndSize];
  'resize-stop': [position: PositionAndSize];
  dragging: [position: PositionAndSize];
  resizing: [position: PositionAndSize];
  'compression-change': [value: boolean];
}

/** AiAssistant 对外暴露的方法 */
export interface AiAssistantExpose {
  /** 打开面板 */
  show: () => void;
  /** 关闭面板 */
  hide: () => void;
  /** 切换面板显隐 */
  toggle: () => void;
  /** 初始化会话列表（可指定默认选中的 sessionCode） */
  initSessions: (sessionCode?: string) => void;
  /** 切换到指定会话 */
  switchSession: (sessionCode: string) => void;
}
