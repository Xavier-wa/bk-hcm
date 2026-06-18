<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { Button } from 'bkui-vue';
import { FilliscreenLine, UnfullScreen, Warn } from 'bkui-vue/lib/icon';

import { type HostApplySuborder } from '@/hooks/chatbot/types';
import { useChatbotMode } from '@/hooks/chatbot/provide';
import CustomMessageCard from './custom-message-card.vue';
import HostApplyPreorderTable from './host-apply-preorder-table.vue';

interface Props {
  // 已拍平的表格行（合并 body_param.require_type + 外层 replicas + spec），由父组件提供
  rows: HostApplySuborder[];
  readonly?: boolean;
  onConfirm: () => void;
  onAddToList: () => void;
}

const props = withDefaults(defineProps<Props>(), { readonly: false });

// 应用内「最大化」覆盖层：全页保留顶部导航栏可见，浮窗则整屏盖住不露顶部
const isFloating = useChatbotMode() === 'floating';
const isMaximized = ref(false);

// 最大化时表格可用最大高度（视口高度扣除头部/操作区/内边距，全页还需扣除导航栏 52px）
const fsTableMaxHeight = ref(600);
const updateFsTableMaxHeight = () => {
  const reserved = isFloating ? 168 : 220;
  fsTableMaxHeight.value = Math.max(240, window.innerHeight - reserved);
};

const handleEsc = (e: KeyboardEvent) => {
  if (e.key === 'Escape') isMaximized.value = false;
};

// 最大化时锁定背景滚动、监听 ESC 退出与窗口尺寸变化
watch(isMaximized, (val) => {
  if (val) {
    updateFsTableMaxHeight();
    window.addEventListener('keydown', handleEsc);
    window.addEventListener('resize', updateFsTableMaxHeight);
    document.body.style.overflow = 'hidden';
  } else {
    window.removeEventListener('keydown', handleEsc);
    window.removeEventListener('resize', updateFsTableMaxHeight);
    document.body.style.overflow = '';
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleEsc);
  window.removeEventListener('resize', updateFsTableMaxHeight);
  document.body.style.overflow = '';
});

const banner = computed(() => `已确认 ${props.rows.length} 条申领配置方案，可点击按钮提交申请单`);

const handleConfirm = () => {
  if (props.readonly) return;
  // 退出最大化，让用户感知回到聊天流、看到 agent 后续响应
  isMaximized.value = false;
  props.onConfirm();
};

const handleAddToList = () => {
  if (props.readonly) return;
  // 跳转前退出最大化，避免遮罩残留
  isMaximized.value = false;
  props.onAddToList();
};
</script>

<template>
  <CustomMessageCard :readonly="readonly" summary-text="请确认全部方案信息，并提交申请单">
    <!-- 工具栏：提示横幅 + 全屏按钮（同行，横幅靠左、全屏靠右） -->
    <div class="ps-toolbar">
      <div class="ps-banner">
        <Warn class="ps-banner-icon" />
        <span class="ps-banner-text">{{ banner }}</span>
      </div>
      <Button class="ps-fullscreen" :size="isFloating ? 'small' : ''" @click="isMaximized = true">
        <FilliscreenLine class="ps-fullscreen-icon" />
        <span class="ps-fullscreen-text">全屏</span>
      </Button>
    </div>

    <HostApplyPreorderTable :suborders="rows" readonly show-disk />

    <template #actions>
      <Button theme="primary" :disabled="readonly" @click="handleConfirm">确认提交</Button>
      <Button :disabled="readonly" @click="handleAddToList">添加到配置清单</Button>
    </template>
  </CustomMessageCard>

  <!-- 最大化：全页铺满导航栏以下区域（导航可见），浮窗则整屏盖住；ESC 退出 -->
  <Teleport to="body">
    <div v-if="isMaximized" class="ps-fs-overlay" :class="{ 'is-floating': isFloating }">
      <div class="ps-fs-panel">
        <div class="ps-fs-header">
          <div class="ps-banner">
            <Warn class="ps-banner-icon" />
            <span class="ps-banner-text">{{ banner }}</span>
          </div>
          <Button class="ps-fullscreen" @click="isMaximized = false">
            <UnfullScreen class="ps-fullscreen-icon" />
            <span class="ps-fullscreen-text">取消全屏</span>
          </Button>
        </div>
        <!-- 操作按钮跟随表格内容区，不固定在视口底部 -->
        <div class="ps-fs-body">
          <HostApplyPreorderTable :suborders="rows" readonly show-disk :max-height="fsTableMaxHeight" />
          <div class="ps-fs-actions">
            <Button theme="primary" :disabled="readonly" @click="handleConfirm">确认提交</Button>
            <Button :disabled="readonly" @click="handleAddToList">添加到配置清单</Button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped lang="scss">
.ps-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.ps-banner {
  display: flex;
  flex: 1;
  align-items: flex-start;
  min-width: 0;
  padding: 8px 12px;
  background: #fff4e2;
  border-radius: 4px;

  .ps-banner-icon {
    flex-shrink: 0;
    margin-top: 2px;
    margin-right: 8px;
    font-size: 16px;
    color: #ff9c01;
  }

  .ps-banner-text {
    font-size: 12px;
    line-height: 20px;
    color: #63656e;
  }
}

.ps-fullscreen {
  flex-shrink: 0;

  .ps-fullscreen-icon {
    margin-right: 4px;
  }

  .ps-fullscreen-text {
    font-size: 12px;
  }
}

// 最大化覆盖层：全页 top 为导航栏高度(52px)保留导航可见；浮窗整屏盖住(top:0)避免露出浮窗头部。
// z-index 高于浮窗面板(10000)与 tooltip(10001)
.ps-fs-overlay {
  position: fixed;
  inset: 52px 0 0;
  z-index: 10002;
  display: flex;
  flex-direction: column;
  padding: 24px 40px;
  background: #f5f7fa;

  &.is-floating {
    top: 0;
  }

  // 灰色画布中的白色内容卡片，高度随内容自适应（不强制铺满画布）
  .ps-fs-panel {
    display: flex;
    flex-direction: column;
    max-height: 100%;
    padding: 24px;
    background: #fff;
    border-radius: 2px;
  }

  .ps-fs-header {
    display: flex;
    gap: 12px;
    align-items: center;
    margin-bottom: 12px;
  }

  .ps-fs-body {
    min-height: 0;
    overflow: auto;
  }

  .ps-fs-actions {
    display: flex;
    gap: 8px;
    margin-top: 16px;
  }
}
</style>
