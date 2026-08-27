<script setup lang="ts">
import { computed } from 'vue';
import { getModel } from '@/model/manager';
import { ModelPropertyColumn } from '@/model/typings';
import { SPLIT_ROW_ACCOUNTING } from '@/views/prepaid-bill/constants';
import type { IPrepaidSplitItem } from '@/views/prepaid-bill/typings';
import { SplitColumn } from './column';

export interface ISplitListProps {
  list: IPrepaidSplitItem[];
}

defineProps<ISplitListProps>();

const columnModel = getModel(SplitColumn);
const columns = computed<ModelPropertyColumn[]>(() => columnModel.getProperties());

const cellValue = (row: IPrepaidSplitItem, columnId: string) => {
  if (columnId === 'accounted') {
    return row.accounted ? SPLIT_ROW_ACCOUNTING.YES : SPLIT_ROW_ACCOUNTING.NO;
  }
  if (columnId === 'memo') {
    return row.memo || '--';
  }
  return row[columnId as keyof IPrepaidSplitItem];
};

const BILL_PERIOD_TEXT = /^\d{4}-\d{2}$/;

// 表格取排序值时会把「2026-08」这类账期误判为数字并转成 NaN，导致账期列排不动；先折成 202608 走数值比较。
// 调账金额本身是纯数字字符串，表格会自动转成数字，无需额外处理。
const sortValFormat = [
  (value: unknown) =>
    typeof value === 'string' && BILL_PERIOD_TEXT.test(value) ? Number(value.replace('-', '')) : value,
];
</script>

<template>
  <bk-table
    row-hover="auto"
    :data="list"
    :sort-val-format="sortValFormat"
    show-overflow-tooltip
    row-key="adjustment_id"
  >
    <bk-table-column
      v-for="(column, index) in columns"
      :key="index"
      :prop="column.id"
      :label="column.name as string"
      :sort="column.sort"
      :align="column.align"
      :min-width="column.minWidth"
      :width="column.width"
    >
      <template #default="{ row }">
        <display-value :property="column" :value="cellValue(row, column.id)" :display="column?.meta?.display" />
      </template>
    </bk-table-column>
    <template #empty>
      <bk-exception type="empty" scene="part" description="暂无分摊明细" />
    </template>
  </bk-table>
</template>
