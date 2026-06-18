<script setup lang="ts">
import { EditLine } from 'bkui-vue/lib/icon';

import { type HostApplySuborder } from '@/hooks/chatbot/types';
import { getSpecFieldText } from '@/hooks/chatbot/host-apply-display';

interface Props {
  suborders: HostApplySuborder[];
  readonly?: boolean;
  maxHeight?: number;
  // 是否展示系统盘/数据盘列（确认提交卡用，预提单卡保持精简不展示）
  showDisk?: boolean;
}

withDefaults(defineProps<Props>(), { readonly: false, maxHeight: 480, showDisk: false });
const emit = defineEmits<{ edit: [index: number] }>();
</script>

<template>
  <bk-table :data="suborders" :max-height="maxHeight" show-overflow-tooltip>
    <bk-table-column label="机型" min-width="180" fixed="left">
      <template #default="{ row }">{{ row.device_type || '--' }}</template>
    </bk-table-column>
    <bk-table-column label="操作系统" min-width="140">
      <template #default="{ row }">{{ row.image_id || '--' }}</template>
    </bk-table-column>
    <bk-table-column label="申请数量" prop="replicas" sort min-width="100">
      <template #default="{ row }">{{ getSpecFieldText(row, 'replicas') || '--' }}</template>
    </bk-table-column>
    <bk-table-column label="地域" min-width="120">
      <template #default="{ row }">{{ row.region || '--' }}</template>
    </bk-table-column>
    <bk-table-column label="可用区" min-width="120">
      <template #default="{ row }">{{ row.zone || '--' }}</template>
    </bk-table-column>
    <bk-table-column label="计费模式" min-width="140">
      <template #default="{ row }">{{ getSpecFieldText(row, 'charge_type') || '--' }}</template>
    </bk-table-column>
    <bk-table-column label="需求类型" min-width="120">
      <template #default="{ row }">{{ row.require_type ?? '--' }}</template>
    </bk-table-column>
    <bk-table-column v-if="showDisk" label="系统盘" min-width="160">
      <template #default="{ row }">{{ getSpecFieldText(row, 'system_disk') || '--' }}</template>
    </bk-table-column>
    <bk-table-column v-if="showDisk" label="数据盘" min-width="180">
      <template #default="{ row }">{{ getSpecFieldText(row, 'data_disk') || '--' }}</template>
    </bk-table-column>
    <bk-table-column v-if="!readonly" label="操作" width="70" fixed="right">
      <template #default="{ index }">
        <EditLine class="ha-table-edit" @click="emit('edit', index)" />
      </template>
    </bk-table-column>
  </bk-table>
</template>

<style scoped lang="scss">
.ha-table-edit {
  font-size: 14px;
  color: #4d4f56;
  cursor: pointer;
}
</style>
