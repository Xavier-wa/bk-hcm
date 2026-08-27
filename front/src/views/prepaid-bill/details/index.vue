<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { Message } from 'bkui-vue';
import { getModel } from '@/model/manager';
import type { ModelProperty } from '@/model/typings';
import { usePrepaidBillStore } from '@/store/prepaid-bill';
import { ACCOUNTING_STATE_MAP, SETTLE_STATES } from '@/views/prepaid-bill/constants';
import { formatAmount } from '@/views/prepaid-bill/utils';
import type { IPrepaidBillItem, IPrepaidSplitItem } from '@/views/prepaid-bill/typings';
import Details from './children/basic-info/basic-info.vue';
import SplitList from './children/split-list/data-list.vue';
import { TableColumn } from '../children/list/data-list/column';

const route = useRoute();
const prepaidBillStore = usePrepaidBillStore();
const columnModel = getModel(TableColumn);

const detail = ref<IPrepaidBillItem | null>(null);
const splitList = ref<IPrepaidSplitItem[]>([]);
const notFound = ref(false);
const splitFailed = ref(false);

const settleField = computed(() =>
  columnModel.getProperties<ModelProperty>().find((field) => field.id === 'settle_state'),
);
const isSettled = computed(() => detail.value?.settle_state === SETTLE_STATES.SETTLED);

const summaryAccounting = computed(() => {
  if (!isSettled.value) return '--';
  const state = detail.value?.accounting_state;
  return (state && ACCOUNTING_STATE_MAP[state]) || state || '--';
});

const summaryAmount = computed(() => {
  if (!isSettled.value) return '--';
  return formatAmount(detail.value?.accounted_cost);
});

const summaryTagTheme = computed(() => {
  if (detail.value?.accounting_state === 'accounted') return 'success';
  if (detail.value?.accounting_state === 'accounting') return 'info';
  return '';
});

const fetchDetail = async () => {
  const id = String(route.params.id || '');
  notFound.value = false;
  splitFailed.value = false;
  detail.value = null;
  splitList.value = [];

  if (!id) {
    notFound.value = true;
    return;
  }

  try {
    const [item, splits] = await Promise.all([
      prepaidBillStore.getPrepaidItem(id),
      prepaidBillStore.listSplitItems(id).catch((error: any) => {
        splitFailed.value = true;
        Message({ theme: 'error', message: error?.message || '分摊明细查询失败' });
        return [] as IPrepaidSplitItem[];
      }),
    ]);

    if (!item) {
      notFound.value = true;
      return;
    }

    detail.value = item;
    splitList.value = splits;
  } catch (error: any) {
    notFound.value = true;
    Message({ theme: 'error', message: error?.message || '查询失败，请重试' });
  }
};

watch(() => route.params.id, fetchDetail, { immediate: true });
</script>

<template>
  <div
    class="prepaid-bill-detail-page"
    v-bkloading="{ loading: prepaidBillStore.detailLoading || prepaidBillStore.splitLoading }"
  >
    <Teleport defer to="#breadcrumbHead">
      <div v-if="detail" class="breadcrumb-head-info">
        <span class="breadcrumb-separator">|</span>
        <span class="breadcrumb-id">{{ detail.id }}</span>
        <display-value
          v-if="settleField"
          :property="settleField"
          :value="detail.settle_state"
          :display="{
            ...settleField.meta?.display,
            on: 'info',
            appearanceProps: {
              ...settleField.meta?.display?.appearanceProps,
              iconObject: {
                success: 'hcm-icon bkhcm-icon-circle-correct-filled',
              },
            },
          }"
        />
      </div>
    </Teleport>
    <bk-exception
      v-if="notFound && !prepaidBillStore.detailLoading"
      type="empty"
      scene="page"
      description="未找到该预付费账单"
    />
    <template v-else-if="detail">
      <Details :data="detail" />
      <div class="details-panel">
        <div class="panel-title">分摊与调账明细</div>
        <div class="split-summary">
          <div class="summary-item">
            <span class="summary-label">核算状态：</span>
            <bk-tag v-if="isSettled" :theme="summaryTagTheme">{{ summaryAccounting }}</bk-tag>
            <span v-else>{{ summaryAccounting }}</span>
          </div>
          <div class="summary-item">
            <span class="summary-label">累计核算金额：</span>
            <span :class="['summary-amount', { highlight: isSettled }]">{{ summaryAmount }}</span>
          </div>
        </div>
        <bk-alert v-if="splitFailed" theme="error" title="分摊明细查询失败" class="split-alert" />
        <SplitList v-else :list="splitList" />
      </div>
    </template>
  </div>
</template>

<style lang="scss" scoped>
.prepaid-bill-detail-page {
  height: 100%;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.breadcrumb-head-info {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  margin-left: -8px;

  .breadcrumb-separator {
    color: #dcdee5;
  }

  .breadcrumb-id {
    color: #979ba5;
  }
}

.details-panel {
  background: #fff;
  border-radius: 2px;
  box-shadow: 0 2px 4px 0 #1919290d;
  padding: 16px 24px;

  .panel-title {
    font-size: 14px;
    font-weight: 700;
    color: #313238;
    line-height: 22px;
    margin-bottom: 16px;
  }
}

.split-summary {
  display: flex;
  align-items: center;
  gap: 52px;
  margin-bottom: 8px;
}

.summary-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
}

.summary-label {
  font-size: 12px;
  color: #4d4f56;
}

.summary-amount {
  font-size: 14px;
  line-height: 22px;
  color: #313238;

  &.highlight {
    font-weight: 700;
    color: #f59500;
  }
}

.split-alert {
  margin-bottom: 8px;
}
</style>
