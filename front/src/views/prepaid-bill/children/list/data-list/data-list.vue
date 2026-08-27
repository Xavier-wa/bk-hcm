<script setup lang="ts">
import { PaginationType } from '@/typings';
import { ModelPropertyColumn } from '@/model/typings';
import usePage from '@/hooks/use-page';
import useTableSettings from '@/hooks/use-table-settings';
import { MENU_BILL_PREPAID_DETAIL } from '@/constants/menu-symbol';
import routerAction from '@/router/utils/action';
import type { IPrepaidBillItem } from '@/views/prepaid-bill/typings';
import MainAccountValue from '@/views/prepaid-bill/components/main-account-value.vue';
import RootAccountValue from '@/views/prepaid-bill/components/root-account-value.vue';
import OperationProductValue from '@/views/prepaid-bill/components/operation-product-value.vue';

export interface IDataListProps {
  columns: ModelPropertyColumn[];
  list: IPrepaidBillItem[];
  pagination: PaginationType;
}

const props = withDefaults(defineProps<IDataListProps>(), {});

const { handlePageChange, handlePageSizeChange, handleSort } = usePage();
const { settings } = useTableSettings(props.columns);

const handleViewDetails = (row: IPrepaidBillItem) => {
  if (!row?.id) return;
  routerAction.redirect(
    {
      name: MENU_BILL_PREPAID_DETAIL,
      params: { id: String(row.id) },
    },
    { history: true },
  );
};
</script>

<template>
  <bk-table
    row-hover="auto"
    :data="list"
    :pagination="pagination"
    :max-height="`calc(100vh - 420px)`"
    :settings="settings"
    remote-pagination
    show-overflow-tooltip
    @page-limit-change="handlePageSizeChange"
    @page-value-change="handlePageChange"
    @column-sort="handleSort"
    row-key="id"
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
      :fixed="column.fixed"
      :render="column.render"
      :filter="column.filter"
    >
      <template #default="{ row }">
        <template v-if="column.id === 'id'">
          <bk-button v-if="row?.id" class="id-link" theme="primary" text @click="handleViewDetails(row)">
            {{ row.id }}
          </bk-button>
          <span v-else>--</span>
        </template>
        <template v-else-if="column.id === 'main_account_id'">
          <MainAccountValue :value="row.main_account_id" />
        </template>
        <template v-else-if="column.id === 'root_account_id'">
          <RootAccountValue :value="row.root_account_id" />
        </template>
        <template v-else-if="column.id === 'product_id'">
          <OperationProductValue :value="row.product_id" />
        </template>
        <template v-else>
          <display-value
            :property="column"
            :value="column.meta?.display?.render ? row : row[column.id]"
            :display="column?.meta?.display"
          />
        </template>
      </template>
    </bk-table-column>
  </bk-table>
</template>

<style lang="scss" scoped>
.id-link {
  color: #3a84ff;
  cursor: pointer;
}

// 设置齿轮绝对定位盖在最右列表头上；末列又有漏斗，负 margin 会把图标推进遮挡区。
:deep(.bk-table-head.has-settings th:last-child) {
  .cell {
    padding-right: 48px;
  }

  .bk-table-head-action:last-of-type {
    margin-right: 0;
  }
}
</style>
