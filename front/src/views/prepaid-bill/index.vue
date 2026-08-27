<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { Message } from 'bkui-vue';
import { getModel } from '@/model/manager';
import useSearchQs from '@/hooks/use-search-qs';
import usePage from '@/hooks/use-page';
import { ModelPropertyColumn, ModelPropertySearch } from '@/model/typings';
import type { QueryFilterType } from '@/typings';
import { usePrepaidBillStore } from '@/store/prepaid-bill';
import { transformSimpleCondition } from '@/utils/search';
import { isInvalidDateRange } from '@/views/prepaid-bill/utils';
import ExportToExcelBatchButton from '@/components/export-to-excel-batch-button/index.vue';
import { SearchCondition } from './children/list/search/condition';
import { TableColumn } from './children/list/data-list/column';
import { createPrepaidBillExportColumns } from './children/list/data-list/export-column';
import Search from './children/list/search/search.vue';
import DataList from './children/list/data-list/data-list.vue';
import type { IPrepaidBillItem, ISearchCondition } from './typings';

const route = useRoute();
const prepaidBillStore = usePrepaidBillStore();
const { pagination, getPageParams } = usePage();

const searchModel = getModel(SearchCondition);
const columnModel = getModel(TableColumn);
const searchFields = computed<ModelPropertySearch[]>(() => searchModel.getProperties());
const dataListColumns = computed<ModelPropertyColumn[]>(() => columnModel.getProperties());

const condition = ref<ISearchCondition>({});
const billList = ref<IPrepaidBillItem[]>([]);
const searchQs = useSearchQs({ key: 'filter', properties: searchFields });

const validateCondition = (vals: ISearchCondition) => {
  if (isInvalidDateRange(vals.usage_time)) {
    Message({ theme: 'error', message: '使用时间开始日不能晚于结束日' });
    return false;
  }
  if (isInvalidDateRange(vals.order_month_range)) {
    Message({ theme: 'error', message: '订单月份开始月不能晚于结束月' });
    return false;
  }
  return true;
};

const fetchList = async () => {
  condition.value = searchQs.get(route.query, {});
  pagination.current = Number(route.query.page) || 1;
  pagination.limit = Number(route.query.limit) || pagination.limit;

  if (!validateCondition(condition.value)) {
    return;
  }

  try {
    const { list, count } = await prepaidBillStore.listPrepaidItems(
      transformSimpleCondition(condition.value, searchFields.value) as QueryFilterType,
      getPageParams(pagination, {
        sort: (route.query.sort || 'order_at') as string,
        order: (route.query.order || 'DESC') as string,
      }),
    );
    billList.value = list;
    pagination.count = count;
  } catch (error: any) {
    Message({ theme: 'error', message: error?.message || '查询失败，请重试' });
  }
};

watch(() => route.query, fetchList, { immediate: true });

const handleSearch = (vals: ISearchCondition) => {
  if (!validateCondition(vals)) return;
  searchQs.set(vals);
};

const handleReset = () => {
  searchQs.clear();
};

const exportColumns = createPrepaidBillExportColumns();

const exportAllRequest = (signal: AbortSignal) =>
  prepaidBillStore.listPrepaidItemsForExport(
    transformSimpleCondition(condition.value, searchFields.value) as QueryFilterType,
    {
      sort: (route.query.sort || 'order_at') as string,
      order: (route.query.order || 'DESC') as string,
      total: pagination.count,
    },
    signal,
  );
</script>

<template>
  <div class="prepaid-bill-list">
    <Search :fields="searchFields" :condition="condition" @search="handleSearch" @reset="handleReset" />
    <div class="table-panel">
      <div class="toolbar">
        <ExportToExcelBatchButton
          outline
          show-icon
          show-confirm-dialog
          :request="exportAllRequest"
          :columns="exportColumns"
          filename="预付费账单"
          text="导出"
          name="预付费账单"
          :pick-num="pagination.count"
          :disabled="pagination.count === 0"
        />
      </div>
      <DataList
        v-bkloading="{ loading: prepaidBillStore.listLoading }"
        :columns="dataListColumns"
        :list="billList"
        :pagination="pagination"
      />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.prepaid-bill-list {
  height: 100%;
  padding: 24px;
}

.table-panel {
  background: #fff;
  border-radius: 2px;
  box-shadow: 0 2px 4px 0 #1919290d;
  padding: 16px 24px;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;

  :deep(.bk-button) {
    background-color: #fff;
    border-color: #c4c6cc;
    color: #4d4f56;
  }

  .hcm-icon {
    margin-right: 4px;
    font-size: 14px;
  }
}
</style>
