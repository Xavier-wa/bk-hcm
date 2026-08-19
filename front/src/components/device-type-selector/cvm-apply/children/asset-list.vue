<script setup lang="ts">
import { computed, watch } from 'vue';
import { useCvmDeviceStore, type IInheritedHostGroup, type IInheritedHost } from '@/store/cvm/device';
import useCvmChargeType from '@/views/ziyanScr/hooks/use-cvm-charge-type';
import { timeFormatter } from '@/common/util';

const props = defineProps<{
  hostGroups: IInheritedHostGroup[];
  /** 当前激活的机型族 Tab（由父级托管，关闭下拉后状态可保留） */
  activeTab: string;
}>();

const emit = defineEmits<{
  select: [assetId: string];
  'tab-change': [deviceFamily: string];
  'update:activeTab': [tab: string];
}>();

const cvmDeviceStore = useCvmDeviceStore();
const { cvmChargeTypeNames, getMonthName } = useCvmChargeType();

// loading 状态（数据请求由父级触发，loading 仍走 store）
const loading = computed(() => cvmDeviceStore.inheritedHostListLoading);

// Tab 列表（所有分组都展示，包括候选数为 0 的）
const tabList = computed(() =>
  props.hostGroups.map((group) => ({
    name: group.device_family,
    label: group.device_family,
    count: group.hosts.length,
  })),
);

// 当前 Tab 的候选列表
const currentGroup = computed(() => props.hostGroups.find((group) => group.device_family === props.activeTab));
const tableData = computed<IInheritedHost[]>(() => currentGroup.value?.hosts ?? []);

// 当前 Tab 是否无设备（选中了但 hosts 为空）
const isCurrentTabEmpty = computed(() => (currentGroup.value?.hosts.length ?? 0) === 0);

// 计费模式格式化
const formatChargeType = (chargeType: string, chargeMonths: number) => {
  const name = cvmChargeTypeNames[chargeType] ?? chargeType;
  return `${name}(剩余${getMonthName(chargeMonths)})`;
};

// 时间格式化（RFC3339 → YYYY-MM-DD）
const formatTime = (time: string) => timeFormatter(time, 'YYYY-MM-DD');

// 数据变化后，若当前 Tab 已不存在于分组列表则重置为第一个 Tab（同步回父级）
watch(
  () => props.hostGroups,
  () => {
    if (!props.hostGroups.some((g) => g.device_family === props.activeTab)) {
      emit('update:activeTab', props.hostGroups[0]?.device_family ?? '');
    }
  },
  { immediate: true },
);

// Tab 切换处理 — 同步父级并记录切换到的机型族
const handleTabChange = (name: string | number) => {
  emit('update:activeTab', String(name));
  emit('tab-change', String(name));
};

// 行点击选中 — bk-table row-click 回参为 (e: MouseEvent, row, index, rows)
const handleRowClick = (_e: MouseEvent, row: IInheritedHost) => {
  emit('select', row.bk_asset_id);
};
</script>

<template>
  <div @click.stop>
    <!-- loading 过渡态 -->
    <div v-if="loading" class="loading-wrapper">
      <bk-loading theme="primary" size="small" />
    </div>

    <template v-else>
      <!-- 顶部 Tab 筛选区 -->
      <bk-tab v-if="tabList.length > 0" :active="activeTab" type="unborder-card" @change="handleTabChange">
        <bk-tab-panel
          v-for="item in tabList"
          :key="item.name"
          :name="item.name"
          :label="item.label"
          :num="item.count"
          num-display-type="elliptic"
        ></bk-tab-panel>
      </bk-tab>

      <!-- 数据表格 -->
      <bk-table
        v-if="!isCurrentTabEmpty"
        :data="tableData"
        height="auto"
        class="resource-table"
        @row-click="handleRowClick"
      >
        <bk-table-column label="固资号" field="bk_asset_id" min-width="170">
          <template #default="{ row }">
            <span>{{ row.bk_asset_id }}</span>
            <bk-tag v-if="row.is_recommended" theme="warning" size="small" type="filled" class="recommend-dot">
              推荐
            </bk-tag>
          </template>
        </bk-table-column>

        <bk-table-column label="IP" field="bk_host_innerip" min-width="140" />

        <bk-table-column label="机型" field="device_type" min-width="160" />

        <bk-table-column label="计费模式" min-width="180">
          <template #default="{ row }">
            {{ formatChargeType(row.instance_charge_type, row.charge_months) }}
          </template>
        </bk-table-column>

        <bk-table-column label="开始时间" min-width="120">
          <template #default="{ row }">
            {{ formatTime(row.billing_start_time) }}
          </template>
        </bk-table-column>

        <bk-table-column label="结束时间" min-width="120">
          <template #default="{ row }">
            {{ formatTime(row.billing_expire_time) }}
          </template>
        </bk-table-column>
      </bk-table>

      <!-- Tab 无设备文案 -->
      <div v-else-if="tabList.length > 0 && isCurrentTabEmpty" class="empty-text">该机型族没有符合要求的CVM实例</div>

      <!-- 所有分组都为空 -->
      <div v-else class="empty-text">暂无可继承的固资候选</div>
    </template>
  </div>
</template>

<style lang="scss" scoped>
.loading-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
}

.resource-table {
  :deep(.bk-table-body) {
    cursor: pointer;

    .bk-table-row {
      cursor: pointer;
    }
  }
}

.empty-text {
  padding: 24px;
  text-align: center;
  font-size: 12px;
  color: #979ba5;
}
</style>
