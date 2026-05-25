<script setup lang="ts">
import { ref, reactive, watch, h, computed, withDirectives } from 'vue';
import { Loading, Table, Sideslider, bkTooltips } from 'bkui-vue';
import { BkRadioGroup, BkRadioButton } from 'bkui-vue/lib/radio';
import { Column } from 'bkui-vue/lib/table/props';
import { ReturnedWay, useZiyanScrStore } from '@/store';
import { useBusinessMapStore } from '@/store/useBusinessMap';
import usePagination from '@/hooks/usePagination';
import useSelection from '@/views/resource/resource-manage/hooks/use-selection';
import { useRollingServerStore } from '@/store/rolling-server';
import { useRollingServerQuotaStore } from '@/store/rolling-server-quota';
import { useWhereAmI } from '@/hooks/useWhereAmI';

import RecycleTypeSelector from './recycle-type-selector.vue';
import RecycleQuotaTips from '@/views/ziyanScr/rolling-server/recycle-quota-tips/index.vue';
import ReturnForecastSelector, { IFormModel } from './return-forecast-selector.vue';
import type { IPreviewRecycleOrderItem } from '../typings';
import { cloneDeep } from 'lodash';
import { TextFile } from 'bkui-vue/lib/icon';

const props = defineProps<{
  ips: string[];
}>();

const emit = defineEmits<(e: 'selectChange', selected: any[]) => void>();

const { getBizsId } = useWhereAmI();
const businessMapStore = useBusinessMapStore();
const scrStore = useZiyanScrStore();
const { selections, handleSelectionChange } = useSelection();

const { pagination, handlePageLimitChange, handlePageValueChange } = usePagination(() => getOrderHost());

const steps = [{ title: '确认回收IP' }, { title: '回收配置' }, { title: '回收提交' }];
const currentStep = ref(1);

const preRecycleList = ref<IPreviewRecycleOrderItem[]>([]);
let originPreRecycleList: IPreviewRecycleOrderItem[] = [];
const preRecycleLoading = ref(false);

const settings = reactive({
  cvm: 'IMMEDIATE',
  pm: 'IMMEDIATE',
  skipConfirm: true,
  recycleType: ['滚服项目', '短租项目'],
});
const recycleType = ref('滚服项目');

const getResourceTypeName = (type: 'QCLOUDCVM' | 'IDCPM' | 'OTHERS') => {
  const resourceTypeMap = {
    QCLOUDCVM: '腾讯云虚拟机',
    IDCPM: 'IDC物理机',
    OTHERS: '其他',
  };
  return resourceTypeMap[type];
};

const getReturnPlanName = (returnPlan: string, resourceType: string) => {
  if (returnPlan === 'IMMEDIATE') {
    return resourceType === 'IDCPM' ? '立即销毁(隔离2小时)' : '立即销毁';
  }
  if (returnPlan === 'DELAY') {
    let label = '延迟销毁';
    if (resourceType === 'IDCPM') {
      label += '(隔离1天)';
    } else if (resourceType === 'QCLOUDCVM') {
      label += '(隔离15天)';
    }
    return label;
  }
  return '';
};

const sidesliderState = reactive({
  show: false,
  title: '',
});
const currentOrder = ref<{ suborder_id: string; bk_biz_id: number }>();
const orderHostList = ref([]);

const handleViewOrderHost = (row: any) => {
  sidesliderState.show = true;
  sidesliderState.title = row.bk_biz_name;
  currentOrder.value = row;
  getOrderHost();
};

const preRecycleColumns: Column[] = [
  { label: '', type: 'selection', width: 30, minWidth: 30 },
  {
    label: '业务',
    field: 'bk_biz_id',
    render: ({ data }: any) => h('span', businessMapStore.businessMap.get(data.bk_biz_id)),
  },
  {
    label: '资源类型',
    field: 'resource_type',
    render: ({ data }: any) => h('span', getResourceTypeName(data.resource_type)),
  },
  {
    label: '需求类型',
    field: 'recycle_type',
    render: ({ cell, index }: any) => {
      const origin = originPreRecycleList[index];
      const selectionIdx = selections.value.findIndex(
        (item: IPreviewRecycleOrderItem) => item.suborder_id === origin.suborder_id,
      );
      const handleChange = (v: string) => {
        // 修改表格table数据
        preRecycleList.value[index].recycle_type = v;
        // 如果改变的是勾选的行，需要同步更新勾选列表
        if (selectionIdx !== -1) {
          selections.value[selectionIdx].recycle_type = v;
        }
      };
      if (returnedWay.value === ReturnedWay.RESOURCE_POOL && origin.recycle_type !== '滚服项目') {
        return h(RecycleTypeSelector, {
          originValue: origin.recycle_type,
          onChange: handleChange,
        });
      }
      return cell;
    },
  },
  {
    label: '回收类型',
    render: ({ data }: any) => h('span', getReturnPlanName(data.return_plan, data.resource_type)),
  },
  {
    label: '资源总数',
    field: 'total_num',
    render: ({ data }: any) =>
      h(
        'div',
        {
          style: { display: 'flex', alignItems: 'center' },
        },
        [
          data.total_num,
          withDirectives(
            h(TextFile, {
              width: '14px',
              height: '14px',
              fill: '#488dff',
              style: { marginLeft: '6px', cursor: 'pointer' },
              onClick: () => handleViewOrderHost(data),
            }),
            [[bkTooltips, { content: '查看详情' }]],
          ),
        ],
      ),
  },
  {
    label: '回收成本',
    field: 'cost_concerned',
    render: ({ data }: any) => h('span', data.cost_concerned ? '涉及' : '不涉及'),
  },
  {
    label: '预测配置',
    field: 'predict_config',
    minWidth: 350,
    render: ({ data, row }: any) => {
      const selectionIdx = selections.value.findIndex(
        (item: IPreviewRecycleOrderItem) => item.suborder_id === data.suborder_id,
      );
      if (row.recycle_type === '常规项目') {
        return h(ReturnForecastSelector, {
          initValue: {
            returnForecast: data?.return_forecast,
            returnForecastTime: data?.return_forecast_time,
          },
          onConfirm: (model: IFormModel) => {
            // // 修改表格table数据
            row.return_forecast = model.returnForecast;
            row.return_forecast_time = model.returnForecastTime;
            // 如果改变的是勾选的行，需要同步更新勾选列表
            if (selectionIdx !== -1) {
              selections.value[selectionIdx].return_forecast = model.returnForecast;
              selections.value[selectionIdx].return_forecast_time = model.returnForecastTime;
            }
          },
        });
      }
      return '--';
    },
  },
  {
    label: '备注信息',
    field: 'remark',
    render: ({ data }: any) => h('span', data.remark || '--'),
  },
];

const orderHostColumns = [
  {
    label: '固资号',
    field: 'asset_id',
  },
  {
    label: '内网IP',
    field: 'ip',
  },
  {
    label: '机型',
    field: 'device_type',
  },
  {
    label: '园区',
    field: 'sub_zone',
  },
  {
    label: '维护人',
    field: 'operator',
  },
  {
    label: '备份维护人',
    field: 'bak_operator',
  },
];

const handleRecycleTypeChange = (v: string) => {
  if (v === '滚服项目') {
    settings.recycleType = ['滚服项目', '短租项目'];
  } else {
    settings.recycleType = ['短租项目', '滚服项目'];
  }
};

const getPreRecycleList = async () => {
  try {
    preRecycleLoading.value = true;
    const params = {
      ips: props.ips,
      return_plan: {
        cvm: settings.cvm,
        pm: settings.pm,
      },
      skip_confirm: settings.skipConfirm,
      recycle_type_sequence: settings.recycleType,
    };
    const {
      data: { info = [] },
    } = await scrStore.getPreRecycleList(params);
    preRecycleList.value = info;

    originPreRecycleList = cloneDeep(info);
  } finally {
    preRecycleLoading.value = false;
  }
};

const getOrderHost = async () => {
  const params = (enableCount: boolean) => ({
    suborder_id: [currentOrder.value.suborder_id],
    bk_biz_id: [currentOrder.value.bk_biz_id],
    page: {
      limit: enableCount ? undefined : pagination.limit,
      start: enableCount ? undefined : pagination.start,
      enable_count: enableCount,
    },
  });
  const [
    {
      data: { count },
    },
    {
      data: { info = [] },
    },
  ] = await Promise.all([scrStore.getRecycleOrderHost(params(true)), scrStore.getRecycleOrderHost(params(false))]);

  pagination.count = count;
  orderHostList.value = info;
};

watch(currentStep, (step) => {
  if (step === 3) {
    getPreRecycleList();
  }
});

watch(
  selections,
  (selections) => {
    emit('selectChange', selections);
  },
  { deep: true },
);

const nextStep = () => {
  currentStep.value += 1;
};
const prevStep = () => {
  selections.value = [];
  currentStep.value -= 1;
};

const isLastStep = () => {
  return currentStep.value === steps.length;
};
const isFirstStep = () => {
  return currentStep.value === 1;
};

// rolling-server
const rollingServerStore = useRollingServerStore();
const rollingServerQuotaStore = useRollingServerQuotaStore();
const isSelectionRecycleTypeChange = (submitSelections: any[]) => {
  return submitSelections.some((item) => {
    const originItem = originPreRecycleList.find((originItem) => originItem.suborder_id === item.suborder_id);
    return originItem.recycle_type !== item.recycle_type;
  });
};
const returnedWay = computed(() => {
  // 不会出现多业务的场景
  return !!rollingServerStore.resPollBusinessIds.includes(getBizsId()) ? ReturnedWay.RESOURCE_POOL : ReturnedWay.CRP;
});
watch(
  returnedWay,
  async (v) => {
    if (v === ReturnedWay.RESOURCE_POOL) {
      await rollingServerQuotaStore.getGlobalQuota();
    }
  },
  { immediate: true },
);
// *暂不限制：资源池业务下，选择为“滚服项目”的核数，不能超过全平台应该退还给公司的额度
const isRollingServerCpuCoreExceedByResPool = computed(() => {
  if (ReturnedWay.RESOURCE_POOL === returnedWay.value) {
    const { sum_delivered_core, sum_returned_applied_core } = rollingServerQuotaStore.globalQuotaConfig;
    // 全平台应该退还给公司的额度
    const limit = sum_delivered_core - sum_returned_applied_core;
    // “滚服项目”的核数
    const sum = selections.value.reduce(
      (total, item) => (item.recycle_type === '滚服项目' ? total + item.sum_cpu_core : total),
      0,
    );

    return sum > limit;
  }
  return false;
});

defineExpose({
  nextStep,
  prevStep,
  isLastStep,
  isFirstStep,
  isSelectionRecycleTypeChange,
  isRollingServerCpuCoreExceedByResPool,
});
</script>

<template>
  <div class="recycle-flow">
    <div class="recycle-flow-head">
      <div class="recycle-tips">
        <p>1.回收的主机，必须在配置平台的待回收模块中</p>
        <p>2.主负责人或备份负责人，必须为当前的执行人</p>
        <p>3.物理机回收时候，会自动将公网IP回收：一、从公司CMDB将公网IP回收；二、从主机网卡上清除外网IP地址配置。</p>
        <p>4.云主机回收销毁后不可找回</p>
      </div>
      <BkSteps class="recycle-steps" :cur-step="currentStep" :steps="steps" />
    </div>
    <div class="recycle-flow-main">
      <div class="confirm-host" v-if="currentStep === 1">
        <slot></slot>
      </div>
      <div class="confirm-setting" v-else-if="currentStep === 2">
        <dl>
          <div class="setting-item">
            <dt class="setting-item-label">CVM回收类型</dt>
            <dd class="setting-item-content">
              <BkRadioGroup class="radio-group" v-model="settings.cvm" type="card">
                <BkRadioButton label="IMMEDIATE">立即销毁</BkRadioButton>
                <BkRadioButton label="DELAY">延迟销毁(隔离15天)</BkRadioButton>
              </BkRadioGroup>
              <div class="content-tips">CVM非立即销毁隔离15天，隔离期间费用仍由业务承担</div>
            </dd>
          </div>
          <div class="setting-item">
            <dt class="setting-item-label">物理机回收类型</dt>
            <dd class="setting-item-content">
              <BkRadioGroup class="radio-group" v-model="settings.pm" type="card">
                <BkRadioButton label="IMMEDIATE">立即销毁(隔离2小时)</BkRadioButton>
                <BkRadioButton label="DELAY">延迟销毁(隔离1天)</BkRadioButton>
              </BkRadioGroup>
              <div class="content-tips">物理机立即销毁隔离2小时，非立即销毁隔离1天，隔离期间费用仍由业务承担</div>
            </dd>
          </div>
          <div class="setting-item">
            <dt class="setting-item-label">非空负载二次确认</dt>
            <dd class="setting-item-content">
              <BkRadioGroup class="radio-group" v-model="settings.skipConfirm" type="card">
                <BkRadioButton :label="true">跳过确认</BkRadioButton>
                <BkRadioButton :label="false">需要检查确认</BkRadioButton>
              </BkRadioGroup>
              <div class="content-tips">
                <p>
                  公司回收流程会通过检查CPU负载判断设备是否空闲，若检测为非空负载，会暂停回收，并邮件通知维护人再次确认。
                </p>
                <p>若需要检查确认，请留意邮件“非空负载设备退回二次确认”的邮件</p>
              </div>
            </dd>
          </div>

          <div class="setting-item">
            <dt class="setting-item-label">回收项目优先级选择</dt>
            <dd class="setting-item-content">
              <BkRadioGroup class="radio-group" v-model="recycleType" type="card" @change="handleRecycleTypeChange">
                <BkRadioButton label="滚服项目">滚服项目优先</BkRadioButton>
                <BkRadioButton label="短租项目">短租项目优先</BkRadioButton>
              </BkRadioGroup>
              <div class="content-tips">
                <p>如滚服项目和短期项目同时有需要退回额度时，可按需选择短租优先，默认为退回滚服额度优先</p>
              </div>
            </dd>
          </div>
        </dl>
      </div>
      <div class="confirm-submit" v-else-if="currentStep === 3">
        <p class="font-small mb8">
          注意：回收的项目类型，平台将自动分类，例如业务有使用滚服项目的资源，回收将优先退回到滚服项目。
        </p>
        <Loading :loading="preRecycleLoading">
          <Table
            :data="preRecycleList"
            row-key="id"
            :columns="preRecycleColumns"
            remote-pagination
            show-overflow-tooltip
            max-height="calc(100vh - 400px)"
            @selection-change="(selections: any) => handleSelectionChange(selections, () => true)"
            @select-all="(selections: any) => handleSelectionChange(selections, () => true, true)"
          ></Table>
        </Loading>
        <recycle-quota-tips class="mt8" :returned-way="returnedWay" :selections="selections" />
      </div>
    </div>
  </div>
  <Sideslider v-model:is-show="sidesliderState.show" :title="sidesliderState.title" :width="1150">
    <div class="order-host-container">
      <Table
        :data="orderHostList"
        row-key="bk_host_id"
        :columns="orderHostColumns"
        remote-pagination
        :pagination="pagination"
        :on-page-limit-change="handlePageLimitChange"
        :on-page-value-change="handlePageValueChange"
        show-overflow-tooltip
      ></Table>
    </div>
  </Sideslider>
</template>

<style lang="scss" scoped>
.recycle-flow {
  .recycle-tips {
    background-color: #f0f8ff;
    border-color: #c5daff;
    font-size: 12px;
    color: #63656e;
    padding: 6px 4px;
    margin-bottom: 8px;
  }

  .recycle-steps {
    width: 60%;
    margin: 16px 8px;
  }

  .confirm-setting {
    padding: 16px;

    .setting-item {
      display: flex;
      align-items: baseline;
      margin-bottom: 16px;

      .setting-item-label {
        width: 150px;
      }

      .setting-item-content {
        .content-tips {
          color: #979ba5;
          font-size: 12px;
          margin-top: 6px;
        }

        .radio-group {
          width: 360px;
        }
      }
    }
  }
}

.order-host-container {
  padding: 16px 24px;
}
</style>
