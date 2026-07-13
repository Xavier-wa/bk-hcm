<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { getModel } from '@/model/manager';
import usePage from '@/hooks/use-page';
import useSearchQs from '@/hooks/use-search-qs';
import { useDissolveQuotaStore, type IDissolveOverview } from '@/store/dissolve/quota';
import routerAction from '@/router/utils/action';
import { SearchCondition } from './search/condition';
import { TableColumn } from './data-list/column';
import Search from './search/search.vue';
import DataList from './data-list/data-list.vue';

defineProps<{ refreshKey?: number }>();

const route = useRoute();
const store = useDissolveQuotaStore();

const conditionModel = getModel(SearchCondition);
const conditionProperties = computed(() => conditionModel.getProperties());

const columnModel = getModel(TableColumn);
const columnProperties = computed(() => columnModel.getProperties());

// 将连续同 group 的列折叠为 bk-table 多级表头结构
const groupedColumns = computed(() =>
  columnProperties.value.reduce<any[]>((acc, col) => {
    if (col.group) {
      const last = acc[acc.length - 1];
      if (last?.children) {
        last.children.push(col);
      } else {
        acc.push({ ...col, children: [col] });
      }
    } else {
      acc.push(col);
    }
    return acc;
  }, []),
);

const { pagination } = usePage();

const condition = ref<Record<string, any>>({});
const list = ref<IDissolveOverview[]>([]);
const loading = ref(false);

// URL 查询参数处理
const searchQs = useSearchQs({ key: 'filter', properties: conditionProperties });

const fetchList = async (searchCondition?: Record<string, any>) => {
  const { time_periods, ...cond } = searchCondition ?? condition.value;
  loading.value = true;
  try {
    // 接口直接传参，不是 filter 形式
    const params: Record<string, any> = { ...cond };
    const res = await store.getOverviewList(params);

    // 数据映射：将 API 响应字段映射为前端使用的字段名
    list.value = (res?.items || []).map((item) => {
      const progressStr = String(item.progress || '0.00%');
      const progressNum = parseFloat(progressStr) || 0;
      return {
        ...item,
        progress: progressNum, // 将 "100.00%" 转换为数字
        progress_str: progressStr, // 保留字符串格式用于显示
      } as IDissolveOverview;
    });
  } catch {
    list.value = [];
  } finally {
    loading.value = false;
  }
};

const handleSearch = (vals: Record<string, any>) => {
  searchQs.set(vals);
};

const handleReset = () => {
  searchQs.clear();
};

const handleBizClick = (row: IDissolveOverview) => {
  const filter = searchQs.build({ bk_biz_ids: [row.bk_biz_id] });
  routerAction.redirect({ query: { ...route.query, filter, tab: 'detail' } }, { replace: true });
};

// 监听路由变化，获取列表数据
watch(
  () => route.query,
  (query) => {
    // 从 URL 获取搜索条件
    condition.value = searchQs.get(query, {});
    fetchList();
  },
  { immediate: true },
);
</script>

<template>
  <div class="overview-tab">
    <search :fields="conditionProperties" :condition="condition" @search="handleSearch" @reset="handleReset" />
    <data-list
      v-bkloading="{ loading: loading }"
      :columns="columnProperties"
      :grouped-columns="groupedColumns"
      :list="list"
      :pagination="pagination"
      @biz-click="handleBizClick"
    />
  </div>
</template>

<style lang="scss" scoped>
.overview-tab {
  padding: 0;
}
</style>
