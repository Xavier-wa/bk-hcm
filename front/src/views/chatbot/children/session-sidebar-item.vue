<script setup lang="ts">
import { OverflowTitle } from 'bkui-vue';

import type { ChatSession } from '@/hooks/chatbot/use-chatbot';

defineProps<{
  session: ChatSession;
  isActive: boolean;
  isPinnedArea: boolean;
  isEditing: boolean;
  editingTitle: string;
  isMenuOpen: boolean;
  menuItems: { key: string; label: string; danger?: boolean }[];
}>();

const emit = defineEmits<{
  click: [];
  'update:editingTitle': [value: string];
  confirmRename: [];
  cancelRename: [];
  toggleMenu: [event: Event];
  menuAction: [key: string];
}>();
</script>

<template>
  <div
    class="session-item"
    :class="{ active: isActive, 'is-pinned-area': isPinnedArea, editing: isEditing }"
    @click="emit('click')"
  >
    <template v-if="isEditing">
      <input
        :value="editingTitle"
        class="session-rename-input"
        @input="emit('update:editingTitle', ($event.target as HTMLInputElement).value)"
        @click.stop
        @keyup.enter="emit('confirmRename')"
        @keyup.escape="emit('cancelRename')"
        @blur="emit('confirmRename')"
      />
    </template>
    <template v-else>
      <OverflowTitle type="tips" class="session-title" :popover-options="{ maxWidth: 280 }">
        {{ session.sessionName }}
      </OverflowTitle>
      <span v-if="isPinnedArea" class="session-pin-icon" @click.stop>
        <i class="hcm-icon bkhcm-icon-pin" />
      </span>
      <span class="session-actions" @click="emit('toggleMenu', $event)">
        <i class="hcm-icon bkhcm-icon-more-fill" />
      </span>
      <div v-if="isMenuOpen" class="session-menu" @click.stop>
        <div
          v-for="item in menuItems"
          :key="item.key"
          class="session-menu-item"
          :class="{ danger: item.danger }"
          @click="emit('menuAction', item.key)"
        >
          {{ item.label }}
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
.session-item {
  position: relative;
  display: flex;
  align-items: center;
  height: 32px;
  padding: 0 8px;
  margin-bottom: 2px;
  cursor: pointer;
  border-radius: 4px;
  transition: background 0.15s;

  &:hover {
    background: var(--sidebar-item-hover);

    .session-actions {
      display: flex;
    }

    &.is-pinned-area .session-pin-icon {
      display: none;
    }
  }

  &.active {
    background: var(--sidebar-item-hover);
  }

  &.is-pinned-area .session-pin-icon {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    font-size: 14px;
    color: var(--sidebar-text-secondary);
  }

  .session-actions {
    display: none;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    cursor: pointer;
    border-radius: 50%;

    .hcm-icon {
      font-size: 14px;
      color: var(--sidebar-text-secondary);
    }

    &:hover {
      background: var(--sidebar-actions-hover);
    }
  }

  // 非编辑态：标题前的小圆点（纯样式，避免额外 DOM 节点）
  &:not(.editing)::before {
    flex-shrink: 0;
    width: 4px;
    height: 4px;
    margin-right: 8px;
    content: '';
    background: #c4c6cc;
    border-radius: 50%;
  }

  .session-title {
    flex: 1;
    overflow: hidden;
    font-size: 12px;
    line-height: 32px;
    color: var(--sidebar-input-color, #313238);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .session-rename-input {
    flex: 1;
    height: 28px;
    padding: 0 8px;
    font-size: 12px;
    color: var(--sidebar-input-color);
    background: var(--sidebar-input-bg);
    border: 1px solid #3a84ff;
    border-radius: 4px;
    outline: none;
  }

  .session-menu {
    position: absolute;
    top: 32px;
    right: 4px;
    z-index: 100;
    min-width: 100px;
    padding: 4px 0;
    background: var(--sidebar-menu-bg);
    border: 1px solid var(--sidebar-menu-border);
    border-radius: 6px;
    box-shadow: 0 4px 12px var(--sidebar-menu-shadow);
  }

  .session-menu-item {
    padding: 6px 16px;
    font-size: 13px;
    color: var(--sidebar-text);
    cursor: pointer;

    &:hover {
      background: var(--sidebar-menu-hover);
    }

    &.danger {
      color: #ea3636;
    }
  }
}
</style>
