<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { cloneDeep } from 'lodash';
import { Button } from 'bkui-vue';
import { FilliscreenLine, UnfullScreen } from 'bkui-vue/lib/icon';

import { type HostApplyPreorderValue, type HostApplySuborder } from '@/hooks/chatbot/types';
import { useChatbotMode } from '@/hooks/chatbot/provide';
import CustomMessageCard from './custom-message-card.vue';
import HostApplyPreorderTable from './host-apply-preorder-table.vue';
import HostApplyAdjustDialog from './host-apply-adjust-dialog.vue';

interface Props {
  content: HostApplyPreorderValue;
  readonly?: boolean;
  readonlySuborders?: HostApplySuborder[];
  onConfirm: (suborders: HostApplySuborder[], edited: boolean) => void;
  onAddToList: (suborders: HostApplySuborder[]) => void;
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false,
  readonlySuborders: () => [],
});

const localSuborders = ref<HostApplySuborder[]>(cloneDeep(props.content.value.suborders ?? []));
watch(
  () => props.content.value.suborders,
  (suborders) => {
    localSuborders.value = cloneDeep(suborders ?? []);
  },
);

const edited = ref(false);
// 应用内「最大化」覆盖层：全页保留顶部导航栏可见，浮窗则整屏盖住不露顶部
const isFloating = useChatbotMode() === 'floating';
const isMaximized = ref(false);
const adjustVisible = ref(false);
const editingIndex = ref(-1);

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

// 只读态展示确认时的配置（含修改）；交互态展示本地可编辑副本
const tableSuborders = computed(() => (props.readonly ? props.readonlySuborders : localSuborders.value));
const editingSuborder = computed<HostApplySuborder | null>(() => localSuborders.value[editingIndex.value] ?? null);

const summaryText = computed(() => `您已确认 ${tableSuborders.value.length} 条申领配置`);

const handleEditRow = (index: number) => {
  if (props.readonly) return;
  editingIndex.value = index;
  adjustVisible.value = true;
};

const handleAdjustSave = (suborder: HostApplySuborder) => {
  if (localSuborders.value[editingIndex.value]) {
    localSuborders.value[editingIndex.value] = suborder;
    edited.value = true;
  }
  adjustVisible.value = false;
};

const handleConfirm = () => {
  if (props.readonly) return;
  // 退出最大化，让用户感知回到聊天流、看到 agent 后续响应
  isMaximized.value = false;
  props.onConfirm(cloneDeep(localSuborders.value), edited.value);
};

const handleAddToList = () => {
  if (props.readonly) return;
  // 跳转前退出最大化，避免遮罩残留
  isMaximized.value = false;
  props.onAddToList(cloneDeep(localSuborders.value));
};
</script>

<template>
  <CustomMessageCard :readonly="readonly" :summary-text="summaryText">
    <!-- 工具栏：全屏按钮靠右 -->
    <div class="po-toolbar">
      <Button class="po-fullscreen" :size="isFloating ? 'small' : ''" @click="isMaximized = true">
        <FilliscreenLine class="po-fullscreen-icon" />
        <span class="po-fullscreen-text">全屏</span>
      </Button>
    </div>

    <HostApplyPreorderTable :suborders="tableSuborders" :readonly="readonly" @edit="handleEditRow" />

    <template #actions>
      <Button theme="primary" :disabled="readonly" @click="handleConfirm">确认方案</Button>
      <Button :disabled="readonly" @click="handleAddToList">添加到配置清单</Button>
    </template>
  </CustomMessageCard>

  <!-- 最大化：全页铺满导航栏以下区域（导航可见），浮窗则整屏盖住；ESC 退出 -->
  <Teleport to="body">
    <div v-if="isMaximized" class="po-fs-overlay" :class="{ 'is-floating': isFloating }">
      <!-- 灰色画布中的白色内容卡片 -->
      <div class="po-fs-panel">
        <div class="po-fs-header">
          <Button class="po-fullscreen" @click="isMaximized = false">
            <UnfullScreen class="po-fullscreen-icon" />
            <span class="po-fullscreen-text">取消全屏</span>
          </Button>
        </div>
        <!-- 操作按钮跟随表格内容区，不固定在视口底部 -->
        <div class="po-fs-body">
          <HostApplyPreorderTable
            :suborders="tableSuborders"
            :readonly="readonly"
            :max-height="fsTableMaxHeight"
            @edit="handleEditRow"
          />
          <div class="po-fs-actions">
            <Button theme="primary" :disabled="readonly" @click="handleConfirm">确认方案</Button>
            <Button :disabled="readonly" @click="handleAddToList">添加到配置清单</Button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>

  <HostApplyAdjustDialog
    v-if="editingSuborder"
    v-model:visible="adjustVisible"
    :suborder="editingSuborder"
    :z-index="isMaximized ? 10010 : undefined"
    @save="handleAdjustSave"
  />
</template>

<style scoped lang="scss">
.po-toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.po-fullscreen {
  flex-shrink: 0;
  margin-left: auto;

  .po-fullscreen-icon {
    margin-right: 4px;
  }

  .po-fullscreen-text {
    font-size: 12px;
  }
}

// 最大化覆盖层：全页 top 为导航栏高度(52px)保留导航可见；浮窗整屏盖住(top:0)避免露出浮窗头部。
// z-index 高于浮窗面板(10000)与 tooltip(10001)
.po-fs-overlay {
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
  .po-fs-panel {
    display: flex;
    flex-direction: column;
    max-height: 100%;
    padding: 24px;
    background: #fff;
    border-radius: 2px;
  }

  .po-fs-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;

    .po-fullscreen {
      flex-shrink: 0;
      margin-left: auto;
    }
  }

  .po-fs-body {
    min-height: 0;
    overflow: auto;
  }

  .po-fs-actions {
    display: flex;
    gap: 8px;
    margin-top: 16px;
  }
}
</style>
