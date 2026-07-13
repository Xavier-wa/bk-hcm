<script setup lang="ts">
import { computed, ref, watch, useTemplateRef } from 'vue';
import { useRoute } from 'vue-router';
import { getModel } from '@/model/manager';
import usePage from '@/hooks/use-page';
import useSearchQs from '@/hooks/use-search-qs';
import { useDissolveQuotaStore, type IDissolveDetail } from '@/store/dissolve/quota';
import { SearchCondition } from './search/condition';
import { TableColumn } from './data-list/column';
import Search from './search/search.vue';
import DataList from './data-list/data-list.vue';

const route = useRoute();
const store = useDissolveQuotaStore();

const conditionModel = getModel(SearchCondition);
const conditionProperties = computed(() => conditionModel.getProperties());

const columnModel = getModel(TableColumn);
const columnProperties = computed(() => columnModel.getProperties());

const { pagination, getPageParams } = usePage();

const condition = ref<Record<string, any>>({});
const list = ref<IDissolveDetail[]>([]);
const loading = ref(false);

// URL 查询参数处理
const searchQs = useSearchQs({ key: 'filter', properties: conditionProperties });

const fetchList = async (searchCondition?: Record<string, any>) => {
  const { time_periods, ...cond } = searchCondition ?? condition.value;
  loading.value = true;
  try {
    const sort = (route.query.sort as string) || 'inner_ip,id';
    const order = (route.query.order as string) || 'ASC';
    const page = getPageParams(pagination, { sort, order });

    // 将搜索条件转换为接口所需的直接参数
    const apiParams: Record<string, any> = { page, ...cond };
    const { list: dataList, count } = await store.getDetailList(apiParams as any);

    list.value = dataList || [];
    pagination.count = count || 0;
  } catch {
    list.value = [];
    pagination.count = 0;
  } finally {
    loading.value = false;
  }
};

const handleSearch = (vals: Record<string, any>) => {
  pagination.current = 1;
  clearTableSelections();
  searchQs.set(vals);
};

const handleReset = () => {
  pagination.current = 1;
  clearTableSelections();
  searchQs.clear();
};

const dataListRef = useTemplateRef<InstanceType<typeof DataList>>('dataListRef');
const clearTableSelections = () => {
  dataListRef.value?.resetSelections();
};

// 监听路由变化，获取列表数据
watch(
  () => route.query,
  (query) => {
    // 设置分页
    pagination.current = Number(query.page) || 1;
    pagination.limit = Number(query.limit) || pagination.limit;

    // 从 URL 获取搜索条件
    condition.value = searchQs.get(query, {});
    fetchList();
  },
  { immediate: true },
);
</script>

<template>
  <div class="detail-tab">
    <search :fields="conditionProperties" :condition="condition" @search="handleSearch" @reset="handleReset" />
    <data-list
      ref="dataListRef"
      v-bkloading="{ loading }"
      :columns="columnProperties"
      :list="list"
      :pagination="pagination"
      :condition="condition"
    />
  </div>
</template>

<style lang="scss" scoped>
.detail-tab {
  padding: 0;
}
</style>
