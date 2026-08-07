<script setup lang="ts">
import type { PaginationType } from '@/typings';
import type { ModelPropertyColumn } from '@/model/typings';
import type { IDissolveOverview } from '@/store/dissolve/quota';
import { useBusinessGlobalStore } from '@/store/business-global';
import { ref, computed } from 'vue';
import usePage from '@/hooks/use-page';
import useTableSettings from '@/hooks/use-table-settings';

interface IDataListProps {
  columns: ModelPropertyColumn[];
  groupedColumns: any[];
  list: IDissolveOverview[];
  pagination: PaginationType;
}

const props = withDefaults(defineProps<IDataListProps>(), {});

const emit = defineEmits<{
  'biz-click': [row: IDissolveOverview];
}>();

const businessGlobalStore = useBusinessGlobalStore();

const { handlePageChange, handlePageSizeChange } = usePage();
const { settings } = useTableSettings(props.columns);

// 本地排序状态，默认按裁撤CPU总核数降序
const sortField = ref<string>('current_cpu_core');
const sortOrder = ref<'asc' | 'desc'>('desc');

// 本地排序处理：不修改路由，仅在组件内排序
const handleLocalSort = ({ column, type }: { column: { field: string }; index: number; type: string }) => {
  if (type === 'null') {
    sortField.value = undefined;
    sortOrder.value = undefined;
  } else {
    sortField.value = column?.field;
    sortOrder.value = type as 'asc' | 'desc';
  }
};

// 排序后的数据：汇总行（bk_biz_id === -1）始终保持在第一行
const sortedList = computed(() => {
  const items = [...props.list];

  // 分离汇总行和普通行
  const summaryIndex = items.findIndex((item) => item.bk_biz_id === -1);
  const summary = summaryIndex > -1 ? items.splice(summaryIndex, 1)[0] : null;

  // 对普通行进行本地排序
  if (sortField.value && sortOrder.value) {
    const field = sortField.value;
    const order = sortOrder.value;
    items.sort((a: any, b: any) => {
      const aVal = a[field];
      const bVal = b[field];
      if (aVal === null || aVal === undefined) {
        if (bVal === null || bVal === undefined) return 0;
        return 1;
      }
      if (bVal === null || bVal === undefined) return -1;

      const multiplier = order === 'asc' ? 1 : -1;
      if (typeof aVal === 'string') {
        return multiplier * aVal.localeCompare(bVal);
      }
      return multiplier * (aVal - bVal);
    });
  }

  // 汇总行始终回到第一行
  return summary ? [summary, ...items] : items;
});

const isSummaryRow = (row?: Record<string, any>) => row?.bk_biz_id === -1;

const isBizClickable = (row: IDissolveOverview) => {
  return businessGlobalStore.businessFullList.some((item) => item.id === row.bk_biz_id);
};

const getFieldValue = (row: Record<string, any>, fieldId: string) => {
  if (fieldId.includes('.')) {
    return fieldId.split('.').reduce((obj: any, key: string) => obj?.[key], row) ?? '--';
  }
  return (row as any)[fieldId] ?? '--';
};

// 禁用 bk-table 的自动排序，让排序完全由 sortedList 控制
const noopSortFn = () => 0;
</script>

<template>
  <div class="dissolve-data-list">
    <bk-table
      row-hover="auto"
      :data="sortedList"
      :pagination="pagination"
      :max-height="'calc(100vh - 450px)'"
      :settings="settings"
      remote-pagination
      show-overflow-tooltip
      row-key="bk_biz_id"
      stripe
      @page-value-change="handlePageChange"
      @page-limit-change="handlePageSizeChange"
      @column-sort="handleLocalSort"
    >
      <template v-for="col in props.groupedColumns" :key="col.id ?? col.group">
        <!-- 分组列（多级表头父级） -->
        <bk-table-column v-if="col.children" :label="col.group">
          <bk-table-column
            v-for="child in col.children"
            :key="child.id"
            :prop="child.id"
            :label="child.name"
            :min-width="child.minWidth"
            :sort="child.sort ? { sortFn: noopSortFn } : false"
            :class-name="(row: Record<string, any>) => (isSummaryRow(row) ? 'summary-cell' : '')"
          >
            <template #default="{ row }">
              <display-value :property="child" :value="getFieldValue(row, child.id)" :display="child?.meta?.display" />
            </template>
          </bk-table-column>
        </bk-table-column>

        <!-- 普通列 -->
        <bk-table-column
          v-else
          :prop="col.id"
          :label="col.name"
          :sort="col.sort ? { sortFn: noopSortFn } : false"
          :min-width="col.minWidth"
          :fixed="col.fixed"
          :class-name="(row: Record<string, any>) => (isSummaryRow(row) ? 'summary-cell' : '')"
        >
          <template #default="{ row }">
            <bk-button
              v-if="col.id === 'bk_biz_id' && !isSummaryRow(row)"
              theme="primary"
              text
              :disabled="!isBizClickable(row)"
              @click="emit('biz-click', row)"
            >
              <display-value :property="col" :value="getFieldValue(row, col.id)" :display="col?.meta?.display" />
            </bk-button>
            <span v-else-if="col.id === 'bk_biz_id' && isSummaryRow(row)" class="summary-name">
              <i class="hcm-icon bkhcm-icon-vector"></i>
              汇总
            </span>
            <div v-else-if="col.id === 'progress'" class="progress-cell">
              <span class="progress-text">{{ row.progress_str }}</span>
              <bk-progress color="#699DF4" size="small" :percent="row.progress" :show-text="false" />
            </div>
            <display-value v-else :property="col" :value="getFieldValue(row, col.id)" :display="col?.meta?.display" />
          </template>
        </bk-table-column>
      </template>
    </bk-table>
  </div>
</template>

<style lang="scss" scoped>
.dissolve-data-list {
  background: #fff;
  box-shadow: 0 2px 4px 0 #1919290d;
  border-radius: 2px;
  margin: 24px 24px 0;
  padding: 16px 24px;

  .summary-name {
    font-weight: 600;
    color: #4d4f56;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .progress-cell {
    display: flex;
    flex-direction: column;

    .progress-text {
      color: #4d4f56;
      font-size: 12px;
      line-height: 20px;
    }

    :deep(.bk-progress) {
      flex: 1;
    }
  }
}

:deep(.summary-cell) {
  background-color: #fdf4e8 !important;
  font-weight: 600;
}
</style>
