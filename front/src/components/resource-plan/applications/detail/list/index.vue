<script setup lang="ts">
import { ref, computed, watchEffect } from 'vue';
import { useI18n } from 'vue-i18n';
import Panel from '@/components/panel/panel.vue';
import type { TicketByIdResult, TicketDemandItem } from '@/typings/resourcePlan';
import { useLegacyTableSettings } from '@/hooks/use-table-settings';
import ChangedText from './changed-text.vue';

interface Props {
  demands: TicketByIdResult['demands'];
  ticketType?: string; // 单据类型（新增，取消，调整） 外部传入，对应 base_info.type
  showCpuCount?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  demands: () => [],
  showCpuCount: true,
});

const { t } = useI18n();
const pagination = ref({ count: 0, current: 1, limit: 10 });
const sort = ref();
const order = ref();
// 单据资源预测详情

/** 机型列：调整单内新增行在此旁挂 NEW 标（Figma 2714:17065） */
const DEVICE_TYPE_FIELD = 'updated_info.cvm.device_type';

/**
 * 调整单内新增的预测：提交时 add 不带 original_info，详情回显同约定。
 * 纯「新增」单据整单都是新预测，不打 NEW；只在调整单里区分混编行。
 */
const isNewAdjustDemand = (row: TicketDemandItem) => props.ticketType === 'adjust' && !row.original_info;

const columns = [
  {
    label: '机型',
    field: DEVICE_TYPE_FIELD,
  },
  {
    label: 'CPU总核数',
    field: 'updated_info.cvm.cpu_core',
    // 增加排序
    sort: true,
  },
  {
    label: '预测用途',
    field: 'demand_class',
  },
  {
    label: '项目类型',
    field: 'updated_info.obs_project',
  },
  {
    label: '地域',
    field: 'updated_info.area_name',
    defaultHidden: true,
    // filter: true,
  },
  {
    label: '城市',
    field: 'updated_info.region_name',
  },
  {
    label: '可用区',
    field: 'updated_info.zone_name',
    defaultHidden: true,
  },
  {
    label: '核心类型',
    field: 'updated_info.cvm.core_type',
  },
  {
    label: '实例数',
    field: 'updated_info.cvm.os',
  },
  {
    label: '机型族',
    field: 'updated_info.cvm.device_family',
    defaultHidden: true,
  },
  {
    label: '机型类型',
    field: 'updated_info.cvm.device_class',
    defaultHidden: true,
  },
  {
    label: '资源池',
    field: 'updated_info.cvm.res_pool',
    defaultHidden: true,
  },
  {
    label: '期望到货时间',
    field: 'updated_info.expect_time',
  },
  {
    label: '单例磁盘IO(MB/s)',
    field: 'updated_info.cbs.disk_io',
    defaultHidden: true,
  },
  {
    label: '云磁盘类型',
    field: 'updated_info.cbs.disk_type_name',
    defaultHidden: true,
  },
  {
    label: '云磁盘大小(G)',
    field: 'updated_info.cbs.disk_size',
    defaultHidden: true,
  },
  // {
  //   label: '内存总量(G)',
  //   field: 'updated_info.cvm.memory',
  //   isDefaultShow: true,
  // },

  // {
  //   label: '资源模式',
  //   field: 'updated_info.cvm.res_mode',
  // },
  {
    label: '备注',
    field: 'updated_info.remark',
  },
];
const { settings } = useLegacyTableSettings(columns);

// 计算当前页数据
const tableData = computed(() => {
  // 复制数据以避免修改原数组
  const sortedData = [...props.demands];

  // 如果有排序字段和方向，则进行排序
  if (sort.value && order.value) {
    sortedData.sort((a, b) => {
      const field = sort.value;
      const valueA = getNestedValue(a, field);
      const valueB = getNestedValue(b, field);

      if (order.value === 'ASC') {
        return valueA > valueB ? 1 : -1;
      }
      return valueA < valueB ? 1 : -1;
    });
  }

  // 分页逻辑保持不变
  const start = (pagination.value.current - 1) * pagination.value.limit;
  const end = start + pagination.value.limit;
  return sortedData.slice(start, end);
});

// 辅助函数：获取嵌套属性值
function getNestedValue(obj: any, path: string) {
  return path.split('.').reduce((o, p) => o?.[p], obj);
}

const cpuCount = computed(() => {
  return props.demands.reduce((sum, item) => {
    return sum + item?.updated_info?.cvm?.cpu_core || 0;
  }, 0);
});

// 页码变化事件
const handlePageChange = (current: number) => {
  pagination.value.current = current;
};

// 每页条数变化事件
const handlePageSizeChange = (limit: number) => {
  pagination.value.limit = limit;
  pagination.value.current = 1;
};

// 排序变化事件
const handleSort = ({ column, type }: { column: { field: string }; type: string }) => {
  pagination.value.current = 1;
  sort.value = column.field;
  order.value = type === 'desc' ? 'DESC' : 'ASC';
};

watchEffect(() => {
  pagination.value.count = props.demands?.length || 0;
});
</script>

<template>
  <Panel class="panel" :title="t('资源预测')">
    <bk-table
      row-hover="auto"
      :settings="settings"
      :pagination="pagination"
      :data="tableData"
      remote-pagination
      @page-limit-change="handlePageSizeChange"
      @page-value-change="handlePageChange"
      @column-sort="handleSort"
    >
      <template v-for="column in columns" :key="column.label">
        <bk-table-column v-bind="column">
          <template #default="{ row, cell }">
            <span v-if="column.field === 'demand_class'">{{ cell }}</span>
            <div v-else-if="column.field === DEVICE_TYPE_FIELD" class="device-type-cell">
              <ChangedText :col-data="row" :field="column.field" :ticket-type="ticketType" />
              <span v-if="isNewAdjustDemand(row)" class="demand-new-tag">NEW</span>
            </div>
            <ChangedText v-else :col-data="row" :field="(column.field as string)" :ticket-type="ticketType" />
          </template>
        </bk-table-column>
      </template>
    </bk-table>
    <template #title-extra>
      <div v-if="showCpuCount" class="cpu-count">
        CPU总数：
        <span>{{ cpuCount }}核</span>
      </div>
    </template>
  </Panel>
</template>

<style lang="scss" scoped>
:deep(.panel-title-container) {
  justify-content: space-between;
}

.cpu-count {
  text-align: right;
  color: #4d4f56;

  span {
    color: #f59500;
    font-weight: 700;
  }
}

.device-type-cell {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
}

/* 对齐 Figma 2714:17065 Tag：高 16 / 字 10 / 主绿 #2caf5e / 圆角 2 */
.demand-new-tag {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  height: 16px;
  padding: 0 4px;
  border-radius: 2px;
  background: #2caf5e;
  color: #fff;
  font-size: 10px;
  line-height: 16px;
}
</style>
